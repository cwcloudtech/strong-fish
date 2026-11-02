package handlers

import (
	"errors"
	"net/http"
	"time"

	"strong-fish-api/internal/ai"
	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/healthsheet"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programmd"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/templates"
)

// How long a window the model is shown, and how often it may be asked.
//
// Ninety days because that is both the provider's own ceiling and the span a
// trend is visible over: a fortnight of steps says what last week was like, a
// quarter says where somebody is going.
//
// A day between readings because the data underneath moves once a day at
// most. Asking again an hour later spends tokens to be told the same thing,
// and a button that can be pressed forty times is one somebody will press
// forty times.
const (
	healthFeedbackDays = 90
	healthFeedbackWait = 24 * time.Hour
)

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
	writeJSON(w, http.StatusOK, withNextAt(feedback))
}

// DeleteHealthFeedback removes it. Asking for another one still waits out the
// day: the rate limit is on the asking, not on what is stored, or deleting
// would be the way around it.
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

	// The wait, before anything is fetched or spent.
	stored, err := h.ai.connections.FindHealthFeedback(r.Context(), userID, provider.Name())
	switch {
	case err == nil:
		if next := stored.UpdatedAt.Add(healthFeedbackWait); time.Now().Before(next) {
			// The refusal carries the reading it is refusing to replace, so a
			// client that asked too early can simply show what it already has.
			writeJSON(w, http.StatusTooManyRequests, healthFeedbackTooSoon{
				Message:  "A new reading can be asked for once a day",
				I18nCode: CodeHealthFeedbackTooSoon,
				Feedback: withNextAt(stored),
			})
			return
		}
	case !errors.Is(err, store.ErrNotFound):
		writeStoreError(w, err)
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

	answer, ok := h.ai.ask(w, r, userID, "health", ai.Request{
		System: templates.CoachSkill + "\n\n" + templates.HealthSkill +
			"\n\n" + LanguageRule(locale),
		Messages: []models.ChatMessage{{Role: models.ChatRoleUser, Message: sheet}},
	})
	if !ok {
		return
	}

	connection, _ := h.ai.connections.Usable(r.Context(), userID)
	saved, err := h.ai.connections.SaveHealthFeedback(r.Context(), userID, models.HealthFeedback{
		Source: provider.Name(), Content: answer.Message,
		Provider: connection.Provider, Model: answer.Model, Days: days,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, withNextAt(saved))
}

// healthFeedbackTooSoon is the refusal body: an error, plus the reading that
// already exists.
type healthFeedbackTooSoon struct {
	Message  string                `json:"message"`
	I18nCode string                `json:"i18n_code"`
	Feedback models.HealthFeedback `json:"feedback"`
}

// withNextAt fills in when another reading may be asked for, so a client can
// say "tomorrow at six" rather than only discovering the refusal by pressing
// the button.
func withNextAt(feedback models.HealthFeedback) models.HealthFeedback {
	feedback.NextAt = feedback.UpdatedAt.Add(healthFeedbackWait)
	return feedback
}
