package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programpdf"
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

	writeProgramPDF(w, r, program, days, programpdf.Options{
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

	writeProgramPDF(w, r, program, days, programpdf.Options{
		Locale: sheetLocale(r),
		Footer: fmt.Sprintf("%s - %s", program.Name, h.uiBaseURL),
	})
}

// writeProgramPDF renders and sends a sheet, for the two routes that produce
// one - a member's own and a published program's.
func writeProgramPDF(w http.ResponseWriter, r *http.Request, program models.Program,
	days []models.ProgramDay, options programpdf.Options) {
	pdf, err := programpdf.Render(program, days, options)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	// attachment, not inline: this is a document to keep and print, and a
	// filename is what makes a folder of them legible later.
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", pdfFileName(program.Name)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
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

// pdfFileName turns a program's name into something a filesystem will accept,
// since it arrives as whatever a coach typed.
func pdfFileName(name string) string {
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
	return cleaned + ".pdf"
}
