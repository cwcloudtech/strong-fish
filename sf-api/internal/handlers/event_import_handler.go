package handlers

import (
	"errors"
	"io"
	"net/http"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/pdfcal"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// calendarImportResponse reports what an upload produced.
//
// Skipped and Warnings both matter to whoever uploaded the file: a federation
// calendar is revised through the season and re-uploaded, so most of a second
// import is expected to be already there, and a planner always carries an
// entry or two with no date on it at all.
type calendarImportResponse struct {
	Events  []models.Event `json:"events"`
	Skipped int            `json:"skipped"`
	// Warnings name the competitions that were read but not dated, so they can
	// be added by hand rather than quietly lost.
	Warnings []string `json:"warnings"`
}

// ImportCalendar creates events from an uploaded federation season calendar.
//
// The FFForce publishes its season as a PDF year planner, which a coach would
// otherwise retype forty entries from. Everything imported is a whole-day
// competition keeping the colour it was printed in, because on the page the
// category - federal, European, world - is carried by the shading and by
// nothing else.
func (h *EventHandler) ImportCalendar(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	// Cap the request before reading it, so an oversized upload is refused
	// rather than buffered.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)
	if err := r.ParseMultipartForm(h.maxUploadSize); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "This file is too large", CodeUploadTooLarge)
		return
	}

	clubID := r.FormValue("clubId")
	visibility := r.FormValue("visibility")
	if utils.IsBlank(visibility) {
		visibility = defaultImportVisibility(clubID)
	}

	// Both checks run before anything is written: an import that created half
	// a season and then hit a permission error would leave a mess to undo by
	// hand.
	//
	// Importing is for coaches and superadmins. A plain member may write
	// private events one at a time, but bulk-loading a federation season is a
	// coaching job - and letting anyone do it would put forty entries into an
	// account on the strength of one upload.
	if !h.isCoachOrSuperadmin(r, userID) {
		writeError(w, http.StatusForbidden, "Only a coach can import a calendar", CodeForbidden)
		return
	}
	// And then the ordinary rule for where they are publishing it: a club's
	// managers, or a superadmin for the open calendar.
	if !h.canWrite(r, userID, clubID, visibility) {
		writeError(w, http.StatusForbidden, "You cannot publish events here", CodeForbidden)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Please attach the calendar in the \"file\" field", CodeInvalidRequestBody)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "This file could not be read", CodeInvalidRequestBody)
		return
	}

	parsed, err := pdfcal.Calendar(data)
	if err != nil {
		if errors.Is(err, pdfcal.ErrNotAPDF) {
			writeError(w, http.StatusBadRequest, "This is not a PDF calendar", CodeInvalidCalendarFile)
			return
		}
		writeError(w, http.StatusBadRequest, "This calendar could not be read", CodeInvalidCalendarFile)
		return
	}
	if len(parsed.Events) == 0 {
		writeError(w, http.StatusBadRequest,
			"No competitions were found in this file", CodeInvalidCalendarFile)
		return
	}

	response := calendarImportResponse{Events: []models.Event{}, Warnings: parsed.Warnings}
	for _, competition := range parsed.Events {
		// The season calendar is revised and republished; importing the new
		// one should add what changed, not a second copy of the whole year.
		exists, err := h.events.ExistsLike(r.Context(), userID, competition.Title, competition.Start)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if exists {
			response.Skipped++
			continue
		}

		created, err := h.events.Create(r.Context(), userID, store.EventFields{
			ClubID:      clubID,
			Title:       competition.Title,
			Description: competition.Description,
			Kind:        models.EventKindCompetition,
			Color:       competition.Color,
			// The planner records days, never a time of day.
			AllDay:     true,
			StartsAt:   competition.Start,
			EndsAt:     competition.End,
			Visibility: visibility,
		})
		if err != nil {
			writeStoreError(w, err)
			return
		}
		response.Events = append(response.Events, created)
	}

	if response.Warnings == nil {
		response.Warnings = []string{}
	}
	writeJSON(w, http.StatusCreated, response)
}

// isCoachOrSuperadmin reports whether the caller may import at all.
func (h *EventHandler) isCoachOrSuperadmin(r *http.Request, userID string) bool {
	if utils.IsBlank(userID) {
		return false
	}
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		return false
	}
	return user.Role == models.GlobalRoleCoach || user.Role == models.GlobalRoleSuperadmin
}

// defaultImportVisibility picks where an import lands when the caller did not
// say: onto the club they named, or - with no club - onto the open calendar,
// which canWrite then allows only for a superadmin.
func defaultImportVisibility(clubID string) string {
	if utils.IsBlank(clubID) {
		return models.VisibilityPublic
	}
	return models.VisibilityClub
}
