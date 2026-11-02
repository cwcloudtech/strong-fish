package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"strong-fish-api/internal/ai"
	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/healthsheet"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programmd"
	"strong-fish-api/internal/programsheet"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/templates"
)

// How long a window the model is shown.
//
// Ninety days because that is both the provider's own ceiling and the span a
// trend is visible over: a fortnight of steps says what last week was like, a
// quarter says where somebody is going.
const healthFeedbackDays = 90

// How recent a reading has to be to travel with an exported sheet.
//
// A day, because that is how long the sheet it describes stays true: the
// numbers move daily, and a reading from last week printed under this week's
// charts would read as a comment on them. Nothing deletes the older one - it
// is still on screen, with its own date on it - it simply does not go into
// the document.
const healthReadingFresh = 24 * time.Hour

// recentReading is the AI's reading of this member's health data as a section
// of the exported sheet, or nil when there is none worth printing.
//
// Printed rather than generated: an export is not the place to spend
// somebody's tokens, and a reading that appeared only in the PDF would be one
// they never saw on screen.
func (h *DeviceHandler) recentReading(r *http.Request, userID, source, locale string) *programsheet.Section {
	stored, err := h.ai.connections.FindHealthFeedback(r.Context(), userID, source)
	if err != nil || time.Since(stored.UpdatedAt) > healthReadingFresh {
		return nil
	}

	rows := []programsheet.Line{}
	for _, line := range programsheet.PlainLines(stored.Content) {
		rows = append(rows, programsheet.Line{Cells: []string{line}})
	}
	if len(rows) == 0 {
		return nil
	}
	// Which subscription produced it and when, under the reading it belongs
	// to - the same credit the screen shows.
	rows = append(rows, programsheet.Line{Cells: []string{
		programsheet.GeneratedBy(locale, creditName(stored.Connection, stored.Model), stored.UpdatedAt),
	}})

	// The label is the column's header rather than the section's: every
	// renderer here draws a heading *and* a header row, and two lines saying
	// "AI feedback" is one too many.
	return &programsheet.Section{
		Columns: []programsheet.Column{{Header: programsheet.Heading(locale, "aiFeedback"), Weight: 1}},
		Rows:    rows,
	}
}

// HealthFeedback is the stored reading of the member's health data.
func (h *DeviceHandler) HealthFeedback(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	provider, ok := h.provider(w, r)
	if !ok {
		return
	}

	feedback, err := h.ai.connections.FindHealthFeedback(r.Context(), userID, provider.Name())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, feedback)
}

// DeleteHealthFeedback removes it.
func (h *DeviceHandler) DeleteHealthFeedback(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	provider, ok := h.provider(w, r)
	if !ok {
		return
	}
	if err := h.ai.connections.DeleteHealthFeedback(r.Context(), userID, provider.Name()); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GenerateHealthFeedback asks the model to read the last ninety days.
//
// Asked for as often as the member likes, through whichever subscription they
// pick on the button. The reading replaces the one before it rather than
// joining it: the subject is a rolling ninety-day window, so an older reading
// of it is not a second opinion but a description of a window that has
// moved.
//
// What it is shown is the member's own health sheet, rendered as markdown -
// the same document the export produces, from the same tables. One
// description of what a health sheet contains, so the reading cannot be about
// something other than what the member can see.
func (h *DeviceHandler) GenerateHealthFeedback(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	found, ok := h.provider(w, r)
	if !ok {
		return
	}
	provider, ok := found.(devicelink.ActivityProvider)
	if !ok {
		writeError(w, http.StatusNotFound, "This provider has no activity data", CodeNotFound)
		return
	}
	if !provider.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	link, err := h.links.Find(r.Context(), userID, provider.Name())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No account connected", CodeDeviceNotConnected)
			return
		}
		writeStoreError(w, err)
		return
	}

	days := healthFeedbackDays
	if max := provider.MaxRangeDays(); days > max {
		days = max
	}

	steps, heart, err := h.healthSeries(r, provider, link, days)
	if err != nil {
		h.writeProviderError(w, err, provider.Name())
		return
	}

	locale := "en"
	if user, err := h.users.FindByID(r.Context(), userID); err == nil {
		locale = localeOf(user, r)
	}

	sheet := programmd.RenderTables(
		healthsheet.Title(h.memberName(r.Context(), userID), locale),
		healthsheet.Subtitle(steps, heart, locale),
		healthsheet.Sections(steps, heart, h.exportBodyComposition(r, userID), locale),
		"")

	answer, ok := h.ai.ask(w, r, userID, r.URL.Query().Get("connectionId"), "health", ai.Request{
		System: templates.CoachSkill + "\n\n" + templates.HealthSkill +
			"\n\n" + LanguageRule(locale),
		Messages: []models.ChatMessage{{Role: models.ChatRoleUser, Message: sheet}},
	})
	if !ok {
		return
	}

	connection, _ := h.ai.usable(r, userID, r.URL.Query().Get("connectionId"))
	saved, err := h.ai.connections.SaveHealthFeedback(r.Context(), userID, models.HealthFeedback{
		Source: provider.Name(), Content: answer.Message,
		Provider: connection.Provider, Model: answer.Model, Days: days,
		Connection: connection.Name,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

// LastDay is the member's last twenty-four hours of health data, as markdown,
// or "" when nothing can be read.
//
// Handed to the chat so a question asked at nine in the evening is answered
// against the day the member has actually had: "should I train tonight" is a
// different question after twelve thousand steps than after four hundred.
//
// A live read on every question, like every other use of this data - nothing
// here is stored (see DeviceLinks). It is skipped entirely when no account is
// connected, and any failure is silence rather than a failed answer: the chat
// must not stop working because a watch's API is down.
func (h *DeviceHandler) LastDay(r *http.Request, userID, locale string) string {
	connected, err := h.links.ListProviders(r.Context(), userID)
	if err != nil || len(connected) == 0 {
		return ""
	}
	linked := map[string]bool{}
	for _, name := range connected {
		linked[name] = true
	}

	found, ok := devicelink.Find(h.providers, devicelink.ActivitySource(h.providers, linked))
	if !ok {
		return ""
	}
	provider, ok := found.(devicelink.ActivityProvider)
	if !ok || !provider.Enabled() {
		return ""
	}
	link, err := h.links.Find(r.Context(), userID, provider.Name())
	if err != nil {
		return ""
	}

	steps, heart, err := h.healthSeries(r, provider, link, 1)
	if err != nil {
		slog.Warn("chat context: health data unavailable", "provider", provider.Name(), "error", err)
		return ""
	}

	// No title: this goes under a heading the caller writes, beside the
	// member's other sections.
	return programmd.RenderTables("", healthsheet.Subtitle(steps, heart, locale),
		healthsheet.Sections(steps, heart, h.exportBodyComposition(r, userID), locale), "")
}
