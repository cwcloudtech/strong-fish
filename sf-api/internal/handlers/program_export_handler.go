package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programpdf"
	"strong-fish-api/internal/programsheet"
	"strong-fish-api/internal/programxlsx"
	"strong-fish-api/internal/utils"
)

// ExportPDF renders a program as a printable sheet per week.
//
// Rendered here rather than in each client, so the web app and the phone
// produce the same document - and so the loads on it are the ones the API
// computed, not a second implementation of the RPE chart that could disagree
// with the screen it was printed from.
//
// The loads are resolved for whoever it is printed for, the same rule the
// program screen uses: yourself, or - for a coach - the member named by
// ?memberId. A sheet with somebody else's numbers on it is the point of
// printing one.
func (h *ProgramHandler) ExportPDF(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	program, ok := h.authorizeProgram(w, r)
	if !ok {
		return
	}

	memberID, ok := h.resolveMember(w, r, callerID)
	if !ok {
		return
	}

	days, err := h.programs.ListDays(r.Context(), program.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	sets, err := h.programs.ListSetsForProgram(r.Context(), program.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	// No logs: a printed sheet is what to do, not what was done. Passing an
	// empty map keeps the same resolution path the screen uses.
	resolved, _, err := h.resolveSets(r.Context(), sets, memberID, map[string]models.SetLog{})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	byDay := map[string][]models.ProgramSet{}
	for _, set := range resolved {
		byDay[set.DayID] = append(byDay[set.DayID], set)
	}
	for i := range days {
		days[i].Sets = byDay[days[i].ID]
	}

	writeProgramSheet(w, exportFormat(r), program, days, programsheet.Options{
		MemberName: h.memberName(r, memberID),
		Locale:     sheetLocale(r),
		Footer:     fmt.Sprintf("%s - %s", program.Name, h.uiBaseURL),
	})
}

// ExportPublicPDF renders a published program for anybody holding its link.
//
// The same sheet, without the weights. A load is a property of the reader's own
// maxes, and a reader outside the club has none here - so this prints what was
// prescribed, exactly as the public page shows it, rather than inventing
// numbers or refusing to print at all. The sets go to the renderer unresolved,
// which is what leaves the load column empty.
func (h *ProgramHandler) ExportPublicPDF(w http.ResponseWriter, r *http.Request) {
	programID := chi.URLParam(r, "programId")

	program, err := h.programs.FindPublicByID(r.Context(), programID)
	if err != nil {
		// A private program reads as missing rather than forbidden, the same
		// as GetPublic: the difference would confirm the id to somebody
		// guessing.
		writeError(w, http.StatusNotFound, "Program not found", CodeNotFound)
		return
	}

	days, err := h.programs.ListDays(r.Context(), programID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	sets, err := h.programs.ListSetsForProgram(r.Context(), programID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	byDay := map[string][]models.ProgramSet{}
	for _, set := range sets {
		byDay[set.DayID] = append(byDay[set.DayID], set)
	}
	for i := range days {
		days[i].Sets = byDay[days[i].ID]
	}

	writeProgramSheet(w, exportFormat(r), program, days, programsheet.Options{
		Locale: sheetLocale(r),
		Footer: fmt.Sprintf("%s - %s", program.Name, h.uiBaseURL),
	})
}

// The two formats a program is exported in. Which one is asked for is the
// route's own extension - export.pdf or export.xlsx - so a browser, a phone
// and curl all get a file named the way its contents are.
const (
	formatPDF  = "pdf"
	formatXLSX = "xlsx"
)

const (
	mediaPDF  = "application/pdf"
	mediaXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// writeProgramSheet renders and sends a document, for every route that
// produces one: a club's program, a member's own, a published one, and a
// member's assignment with their feedback on it.
//
// The two renderers are laid out from the same tables (see programsheet), so
// this only has to pick which one and label the response.
func writeProgramSheet(w http.ResponseWriter, format string, program models.Program,
	days []models.ProgramDay, options programsheet.Options) {
	var (
		body  []byte
		media string
		err   error
	)
	if format == formatXLSX {
		body, err = programxlsx.Render(program, days, options)
		media = mediaXLSX
	} else {
		body, err = programpdf.Render(program, days, options)
		media = mediaPDF
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}

	w.Header().Set("Content-Type", media)
	// attachment, not inline: this is a document to keep, print or fill in,
	// and a filename is what makes a folder of them legible later.
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", sheetFileName(program.Name, format)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// exportFormat reads which document was asked for off the route's extension.
// Anything else is a PDF: the routes only register the two, and a format that
// somehow arrives unrecognized should still hand back a readable sheet.
func exportFormat(r *http.Request) string {
	if strings.HasSuffix(r.URL.Path, ".xlsx") {
		return formatXLSX
	}
	return formatPDF
}

// memberName is who the sheet was printed for, or blank when that cannot be
// resolved - a missing name is a line the sheet does without, not a failure.
func (h *ProgramHandler) memberName(r *http.Request, memberID string) string {
	if utils.IsBlank(memberID) {
		return utils.EMPTY
	}
	user, err := h.users.FindByID(r.Context(), memberID)
	if err != nil {
		return utils.EMPTY
	}
	return strings.TrimSpace(user.Name + " " + user.Surname)
}

// sheetLocale reads the language the caller is reading the app in, so the
// sheet's headings match the screen it was printed from. Named apart from
// localeOf, which picks the language somebody's *email* is sent in.
func sheetLocale(r *http.Request) string {
	locale := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("locale")))
	if locale == "fr" {
		return "fr"
	}
	return "en"
}

// sheetFileName turns a program's name into something a filesystem will
// accept, since it arrives as whatever a coach typed.
func sheetFileName(name, format string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		case r == ' ':
			return '-'
		}
		return -1
	}, name)

	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" {
		cleaned = "program"
	}
	return cleaned + "." + format
}

// ExportAssignment renders a member's assigned block with their feedback on
// it: what was prescribed, and beside it what they actually did.
//
// This is the document a lifter sends their coach at the end of a week, and
// the one a coach exports to read an athlete's block away from the app - which
// is why it is authorized exactly like the assignment itself: the member, a
// manager of the club the program belongs to, or a superadmin.
//
// ?week=N limits it to one week and ?day=<id> to a single session. A block is
// a dozen pages, a week is one, and a session is what gets discussed the
// evening it was trained - the smallest thing worth sending somebody.
func (h *TrainingHandler) ExportAssignment(w http.ResponseWriter, r *http.Request) {
	assignment, ok := h.authorizeAssignment(w, r)
	if !ok {
		return
	}

	program, err := h.programs.FindByID(r.Context(), assignment.ProgramID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	days, err := h.programs.ListDays(r.Context(), assignment.ProgramID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	sets, err := h.programs.ListSetsForProgram(r.Context(), assignment.ProgramID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	logs, err := h.programs.ListLogsForAssignment(r.Context(), assignment.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	// Resolved against the member the block was assigned to, not whoever is
	// asking: a coach exporting their athlete's week wants the athlete's
	// loads, which is the whole point of sending it.
	resolved, _, err := h.sets.resolveSets(r.Context(), sets, assignment.UserID, logs)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	byDay := map[string][]models.ProgramSet{}
	for _, set := range resolved {
		byDay[set.DayID] = append(byDay[set.DayID], set)
	}
	for i := range days {
		days[i].Sets = byDay[days[i].ID]
	}
	days = narrowDays(days, r.URL.Query().Get("week"), r.URL.Query().Get("day"))

	writeProgramSheet(w, exportFormat(r), program, days, programsheet.Options{
		MemberName: h.sets.memberName(r, assignment.UserID),
		Locale:     sheetLocale(r),
		Feedback:   true,
		Footer:     fmt.Sprintf("%s - %s", program.Name, h.sets.uiBaseURL),
	})
}

// narrowDays limits a block to one session, or to one week, or leaves it whole
// when neither was asked for.
//
// A session wins over a week when both are given: it is the narrower answer,
// and a client sending both means the session inside that week.
//
// A week or a session that has no sets comes back empty rather than falling
// back to the whole block: somebody who asked for week 7 of a six-week block
// should get a sheet saying there is nothing there, not twelve pages they did
// not ask for.
func narrowDays(days []models.ProgramDay, week, dayID string) []models.ProgramDay {
	if utils.IsNotBlank(dayID) {
		wanted := strings.TrimSpace(dayID)
		filtered := make([]models.ProgramDay, 0, 1)
		for _, day := range days {
			if day.ID == wanted {
				filtered = append(filtered, day)
			}
		}
		return filtered
	}

	if utils.IsBlank(week) {
		return days
	}
	number, err := strconv.Atoi(strings.TrimSpace(week))
	if err != nil {
		return days
	}

	filtered := make([]models.ProgramDay, 0, len(days))
	for _, day := range days {
		if day.Week == number {
			filtered = append(filtered, day)
		}
	}
	return filtered
}
