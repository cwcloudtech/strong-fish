package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// CalendarHandler publishes a member's events as an ICS feed Outlook and
// Google Calendar can subscribe to.
//
// The feed is addressed by a per-user secret token in the URL rather than by a
// session, because neither provider can send an Authorization header when it
// polls a subscribed calendar. That makes the URL itself the credential - the
// same trust model as any other share-by-link - which is why it can be
// regenerated, and why it is never shown to anybody but its owner.
type CalendarHandler struct {
	users      *store.UserStore
	events     *store.EventStore
	clubs      *store.ClubStore
	apiBaseURL string
}

func NewCalendarHandler(users *store.UserStore, events *store.EventStore, clubs *store.ClubStore, apiBaseURL string) *CalendarHandler {
	return &CalendarHandler{users: users, events: events, clubs: clubs, apiBaseURL: apiBaseURL}
}

type calendarFeedStatus struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
}

func (h *CalendarHandler) status(user models.User) calendarFeedStatus {
	if !user.CalendarFeedEnabled || utils.IsBlank(user.CalendarFeedToken) {
		return calendarFeedStatus{Enabled: user.CalendarFeedEnabled}
	}
	return calendarFeedStatus{
		Enabled: true,
		URL:     h.apiBaseURL + "/v1/calendar/" + user.CalendarFeedToken + ".ics",
	}
}

func (h *CalendarHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.status(user))
}

func (h *CalendarHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true)
}

func (h *CalendarHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *CalendarHandler) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	user, err := h.users.SetCalendarFeedEnabled(r.Context(), userID, enabled)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.status(user))
}

func (h *CalendarHandler) Regenerate(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	user, err := h.users.RegenerateCalendarFeedToken(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.status(user))
}

// Feed is what Outlook and Google Calendar poll. There is no session to check -
// only the token in the URL - and it deliberately answers 404 for an unknown
// or disabled one rather than 401: a calendar client shown a 401 will keep
// prompting its user for credentials that do not exist.
func (h *CalendarHandler) Feed(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSuffix(chi.URLParam(r, "token"), ".ics")

	user, err := h.users.FindByCalendarFeedToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "Calendar not found", CodeNotFound)
		return
	}

	clubs, err := h.clubs.ListForUser(r.Context(), user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	clubIDs := make([]string, 0, len(clubs))
	for _, club := range clubs {
		clubIDs = append(clubIDs, club.ID)
	}

	// The whole calendar, past included: a subscriber's history is part of
	// what they subscribed to, and calendar clients expect to keep what they
	// have already synced rather than watch it disappear.
	events, err := h.events.ListVisible(r.Context(), clubIDs, time.Time{})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	birthdays, err := birthdayEvents(r.Context(), h.users, h.clubs, user.ID, time.Time{})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	events = append(events, birthdays...)

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="strong-fish.ics"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(BuildICS(events)))
}

// BuildICS renders events as an RFC 5545 calendar.
//
// Timed events are emitted in UTC (the "Z" form), which is exactly how they
// are stored, so a subscriber in any timezone sees the meet at the hour it
// actually starts. An all-day event uses VALUE=DATE with an exclusive DTEND,
// as the spec requires - Outlook renders a same-day DTEND as no day at all.
func BuildICS(events []models.Event) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//strong-fish//Events//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString("X-WR-CALNAME:strong-fish\r\n")

	stamp := time.Now().UTC().Format("20060102T150405Z")
	for _, event := range events {
		if event.StartsAt.IsZero() {
			continue
		}

		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&b, "UID:%s@strong-fish\r\n", event.ID)
		fmt.Fprintf(&b, "DTSTAMP:%s\r\n", stamp)

		// A birthday is the same date every year, so the client is given the
		// rule instead of one entry per year.
		if event.Kind == models.EventKindBirthday {
			b.WriteString("RRULE:FREQ=YEARLY\r\n")
		}

		if event.WholeDay() {
			start := event.StartsAt.UTC()
			end := event.End().UTC()
			// A DATE-valued DTEND has to fall on a later *day*, not merely at a
			// later instant: End() defaults to an hour after the start, and an
			// hour later formats to the same date - which Outlook reads as an
			// event occupying no days at all, and hides.
			if !end.Truncate(24 * time.Hour).After(start.Truncate(24 * time.Hour)) {
				end = start.AddDate(0, 0, 1)
			}
			fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\n", start.Format("20060102"))
			fmt.Fprintf(&b, "DTEND;VALUE=DATE:%s\r\n", end.Format("20060102"))
		} else {
			fmt.Fprintf(&b, "DTSTART:%s\r\n", event.StartsAt.UTC().Format("20060102T150405Z"))
			fmt.Fprintf(&b, "DTEND:%s\r\n", event.End().UTC().Format("20060102T150405Z"))
		}

		fmt.Fprintf(&b, "SUMMARY:%s\r\n", icsEscape(icsSummary(event)))
		if utils.IsNotBlank(event.Location) {
			fmt.Fprintf(&b, "LOCATION:%s\r\n", icsEscape(event.Location))
		}
		if description := icsDescription(event); utils.IsNotBlank(description) {
			fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", icsEscape(description))
		}
		if utils.IsNotBlank(event.URL) {
			fmt.Fprintf(&b, "URL:%s\r\n", event.URL)
		}
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// icsSummary prefixes a club event with its club, since a subscriber's feed
// mixes several clubs' calendars with the open one.
func icsSummary(event models.Event) string {
	if utils.IsNotBlank(event.ClubName) {
		return event.ClubName + " - " + event.Title
	}
	return event.Title
}

func icsDescription(event models.Event) string {
	parts := []string{}
	if utils.IsNotBlank(event.Description) {
		parts = append(parts, event.Description)
	}
	if utils.IsNotBlank(event.URL) {
		parts = append(parts, event.URL)
	}
	return strings.Join(parts, "\n\n")
}

// icsEscape applies RFC 5545's TEXT escaping. Backslash, comma and semicolon
// are structural in the format, and a real newline has to become the
// two-character sequence "\n" - an unescaped one ends the content line and
// corrupts everything after it.
func icsEscape(value string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		"\r", utils.EMPTY,
		"\n", `\n`,
		",", `\,`,
		";", `\;`,
	).Replace(value)
}
