package handlers

import (
	"fmt"
	"net/http"
	"strings"

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

	pdf, err := programpdf.Render(program, days, programpdf.Options{
		MemberName: h.memberName(r, memberID),
		Locale:     sheetLocale(r),
		Footer:     fmt.Sprintf("%s - %s", program.Name, h.uiBaseURL),
	})
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
