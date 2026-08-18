package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
	"strong-fish-api/internal/xlsximport"
)

// importResponse reports what an upload produced, including the exercises it had
// to invent and anything it had to guess at - a coach should be able to see that
// their file wasn't perfectly clean without the upload having failed over it.
type importResponse struct {
	Program models.Program `json:"program"`
	// CreatedExercises are movements the file mentioned that weren't in the
	// shared catalog yet. They're now available to every coach's autocomplete,
	// so it's worth telling the importer they were added.
	CreatedExercises []models.Exercise `json:"createdExercises"`
	// ReferenceOneRMs are the maxes the spreadsheet's percentages were authored
	// against, keyed by exercise id. They're offered as a starting point for
	// members who haven't recorded their own - never applied automatically.
	ReferenceOneRMs map[string]float64 `json:"referenceOneRms"`
	Warnings        []string           `json:"warnings"`
}

// Import turns an uploaded program spreadsheet into the data model: one
// program, its sessions, and every prescribed set - with the loads deliberately
// left out, since from here on they're derived per member from that member's own
// 1RMs (see package loadcalc).
func (h *ProgramHandler) Import(w http.ResponseWriter, r *http.Request) {
	clubID := chi.URLParam(r, "clubId")
	authorID, _ := middleware.UserIDFromContext(r.Context())

	// Cap the request before reading it, so an oversized upload is refused
	// rather than buffered.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)
	if err := r.ParseMultipartForm(h.maxUploadSize); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "This file is too large", CodeUploadTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Please attach a spreadsheet in the \"file\" field", CodeInvalidRequestBody)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "This file could not be read", CodeInvalidSpreadsheet)
		return
	}

	parsed, err := xlsximport.Parse(data)
	if errors.Is(err, xlsximport.ErrNoProgramFound) {
		writeError(w, http.StatusBadRequest,
			"No training day was found in this spreadsheet. Each day needs an \"Exercice | Reps | RPE | Percentage | Load\" header row.",
			CodeNoProgramInFile)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), CodeInvalidSpreadsheet)
		return
	}

	catalog, created, err := h.resolveExercises(r.Context(), parsed, authorID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	name := r.FormValue("name")
	if utils.IsBlank(name) {
		name = programNameFromFile(header.Filename)
	}

	program, err := h.programs.Create(r.Context(), store.NewProgram{
		ClubID: clubID, AuthorID: authorID, Name: name,
		Description:    r.FormValue("description"),
		SourceFileName: header.Filename,
		Days:           buildDays(parsed, catalog),
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, importResponse{
		Program:          program,
		CreatedExercises: created,
		ReferenceOneRMs:  referenceOneRMs(parsed, catalog),
		Warnings:         warningsOrEmpty(parsed.Warnings),
	})
}

// resolveExercises maps every movement the workbook mentions onto a catalog
// entry, keyed by the slug the parser produced. Anything the catalog doesn't
// already know (by slug or alias) is added to it, so a coach never has to
// pre-register their exercises before uploading - and every other coach gets the
// new movement in their autocomplete from then on.
func (h *ProgramHandler) resolveExercises(ctx context.Context, parsed *xlsximport.ParsedProgram, authorID string) (map[string]models.Exercise, []models.Exercise, error) {
	catalog := make(map[string]models.Exercise, len(parsed.Exercises))
	created := []models.Exercise{}

	for _, parsedExercise := range parsed.Exercises {
		existing, err := h.exercises.FindBySlug(ctx, parsedExercise.Slug)
		if err == nil {
			catalog[parsedExercise.Slug] = existing
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return nil, nil, err
		}

		exercise, err := h.exercises.Create(ctx, store.ExerciseFields{
			Slug: parsedExercise.Slug,
			// The workbook only has the one spelling, so both locales start
			// from it; a coach can translate it afterwards in the catalog.
			Labels:     map[string]string{"en": parsedExercise.Name, "fr": parsedExercise.Name},
			Category:   categoryFor(parsedExercise),
			OneRMRef:   parsedExercise.OneRMRef,
			Bodyweight: parsedExercise.Bodyweight,
			CreatedBy:  authorID,
		})
		if err != nil {
			return nil, nil, err
		}
		catalog[parsedExercise.Slug] = exercise
		created = append(created, exercise)
	}
	return catalog, created, nil
}

// categoryFor classifies a newly imported movement: one programmed off a
// competition lift's max belongs to that lift's family, anything else is an
// accessory.
func categoryFor(exercise xlsximport.ParsedExercise) string {
	if models.IsValidOneRMRef(exercise.OneRMRef) && utils.IsNotBlank(exercise.OneRMRef) {
		return exercise.OneRMRef
	}
	return models.CategoryAccessory
}

// buildDays turns the parsed sessions into the store's insert shape, dropping
// any set whose exercise couldn't be resolved (which resolveExercises makes
// impossible in practice - this is the guard that keeps a nil id out of the
// insert if it ever changes).
func buildDays(parsed *xlsximport.ParsedProgram, catalog map[string]models.Exercise) []store.NewDay {
	days := make([]store.NewDay, 0, len(parsed.Days))
	for _, parsedDay := range parsed.Days {
		sets := make([]store.NewSet, 0, len(parsedDay.Sets))
		for _, parsedSet := range parsedDay.Sets {
			exercise, ok := catalog[parsedSet.ExerciseSlug]
			if !ok {
				continue
			}
			sets = append(sets, store.NewSet{
				ExerciseID: exercise.ID, Position: parsedSet.Position, Reps: parsedSet.Reps,
				RPE: parsedSet.RPE, Percentage: parsedSet.Percentage,
				AbsoluteLoad: parsedSet.AbsoluteLoad, LoadMode: parsedSet.LoadMode, Part: parsedSet.Part,
				Notes: parsedSet.Notes,
			})
		}
		if len(sets) == 0 {
			continue
		}
		days = append(days, store.NewDay{
			Week: parsedDay.Week, Day: parsedDay.Day, Title: parsedDay.Title,
			Position: parsedDay.Position, Sets: sets,
		})
	}
	return days
}

// referenceOneRMs re-keys the spreadsheet's reference maxes by exercise id, so
// the client can offer them as a starting point ("this file was written for a
// 120kg squat - is that yours?") without the API deciding anyone's max for them.
func referenceOneRMs(parsed *xlsximport.ParsedProgram, catalog map[string]models.Exercise) map[string]float64 {
	byID := map[string]float64{}
	for slug, value := range parsed.RefOneRMs {
		if exercise, ok := catalog[slug]; ok {
			byID[exercise.ID] = value
		}
	}
	return byID
}

// programNameFromFile derives a default program name from the uploaded
// filename, so an import without an explicit name isn't called "program.xlsx".
func programNameFromFile(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	base = strings.NewReplacer("_", " ", "-", " ").Replace(base)
	if base = strings.TrimSpace(base); utils.IsBlank(base) {
		return "Imported program"
	}
	return base
}

// warningsOrEmpty keeps the JSON field an array rather than null when the import
// had nothing to report.
func warningsOrEmpty(warnings []string) []string {
	if warnings == nil {
		return []string{}
	}
	return warnings
}
