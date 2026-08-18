package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// EventHandler is the calendar: meets, club sessions, camps.
//
// An event either belongs to a club - and then only its managers may write it -
// or belongs to none, which is the open calendar a superadmin curates. Reading
// follows the same visibility rule posts use, so a club can keep its own dates
// to itself while still publishing the meets it wants seen.
type EventHandler struct {
	events *store.EventStore
	clubs  *store.ClubStore
	users  *store.UserStore
}

func NewEventHandler(events *store.EventStore, clubs *store.ClubStore, users *store.UserStore) *EventHandler {
	return &EventHandler{events: events, clubs: clubs, users: users}
}

type eventPayload struct {
	ClubID      string `json:"clubId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	URL         string `json:"url"`
	Kind        string `json:"kind"`
	StartsAt    string `json:"startsAt"`
	EndsAt      string `json:"endsAt"`
	AllDay      bool   `json:"allDay"`
	Visibility  string `json:"visibility"`
}

func (h *EventHandler) fields(w http.ResponseWriter, p eventPayload) (store.EventFields, bool) {
	if utils.IsBlank(p.Title) {
		writeError(w, http.StatusBadRequest, "Please give this event a title", CodeEventTitleRequired)
		return store.EventFields{}, false
	}

	startsAt, err := time.Parse(time.RFC3339, p.StartsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Please give this event a start date", CodeInvalidEventDate)
		return store.EventFields{}, false
	}

	var endsAt time.Time
	if utils.IsNotBlank(p.EndsAt) {
		endsAt, err = time.Parse(time.RFC3339, p.EndsAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid end date", CodeInvalidEventDate)
			return store.EventFields{}, false
		}
		if endsAt.Before(startsAt) {
			writeError(w, http.StatusBadRequest, "This event ends before it starts", CodeInvalidEventDate)
			return store.EventFields{}, false
		}
	}

	return store.EventFields{
		ClubID: p.ClubID, Title: p.Title, Description: p.Description,
		Location: p.Location, URL: p.URL, Kind: p.Kind,
		StartsAt: startsAt, EndsAt: endsAt, AllDay: p.AllDay, Visibility: p.Visibility,
	}, true
}

// canWrite decides who may create or change an event: a manager of the club it
// belongs to, or a superadmin (who is also the only one who can put an event
// on the open, club-less calendar).
func (h *EventHandler) canWrite(r *http.Request, userID, clubID string) bool {
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		return false
	}
	if user.Role == models.GlobalRoleSuperadmin {
		return true
	}
	if utils.IsBlank(clubID) {
		return false
	}
	membership, err := h.clubs.FindMembership(r.Context(), clubID, userID)
	if err != nil {
		return false
	}
	return models.CanManageClub(membership.Role)
}

// clubIDsOf returns the clubs a caller belongs to, which is what decides the
// club-only events they may read. An anonymous caller has none, and sees the
// public calendar.
func (h *EventHandler) clubIDsOf(r *http.Request, userID string) []string {
	if utils.IsBlank(userID) {
		return nil
	}
	clubs, err := h.clubs.ListForUser(r.Context(), userID)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(clubs))
	for _, club := range clubs {
		ids = append(ids, club.ID)
	}
	return ids
}

// List returns the calendar as the caller may see it. It is reachable logged
// out - a meet anybody can enter is exactly the thing worth finding before you
// have an account - and ?from= bounds it, defaulting to "from now", since a
// calendar is about what is coming.
func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	from := time.Now().UTC()
	if raw := r.URL.Query().Get("from"); utils.IsNotBlank(raw) {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			from = parsed
		}
	}
	if r.URL.Query().Get("past") == "1" {
		from = time.Time{}
	}

	events, err := h.events.ListVisible(r.Context(), h.clubIDsOf(r, userID), from)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	events = h.decorate(r, events, userID)

	// Birthdays are derived, not stored, so they're merged in here rather than
	// coming out of the query - and they're never editable, which decorate
	// would otherwise have to special-case.
	birthdays, err := birthdayEvents(r.Context(), h.users, h.clubs, userID, from)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	events = append(events, birthdays...)
	sort.SliceStable(events, func(i, j int) bool { return events[i].StartsAt.Before(events[j].StartsAt) })

	writeJSON(w, http.StatusOK, events)
}

func (h *EventHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	event, err := h.events.FindByID(r.Context(), chi.URLParam(r, "eventId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !h.canRead(r, event, userID) {
		// Reported as missing rather than forbidden: the difference would
		// confirm the id exists to somebody guessing.
		writeError(w, http.StatusNotFound, "Event not found", CodeNotFound)
		return
	}
	writeJSON(w, http.StatusOK, h.decorate(r, []models.Event{event}, userID)[0])
}

func (h *EventHandler) canRead(r *http.Request, event models.Event, userID string) bool {
	if event.Visibility == models.VisibilityPublic {
		return true
	}
	if utils.IsBlank(userID) || utils.IsBlank(event.ClubID) {
		return false
	}
	_, err := h.clubs.FindMembership(r.Context(), event.ClubID, userID)
	return err == nil
}

// decorate marks the events the caller may act on, so the client offers the
// edit and delete controls only where they would work.
func (h *EventHandler) decorate(r *http.Request, events []models.Event, userID string) []models.Event {
	writable := map[string]bool{}
	for i, event := range events {
		allowed, known := writable[event.ClubID]
		if !known {
			allowed = utils.IsNotBlank(userID) && h.canWrite(r, userID, event.ClubID)
			writable[event.ClubID] = allowed
		}
		events[i].Editable = allowed
		events[i].Deletable = allowed
	}
	return events
}

func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p eventPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !h.canWrite(r, userID, p.ClubID) {
		writeError(w, http.StatusForbidden, "Only a coach can add an event to this calendar", CodeForbidden)
		return
	}
	fields, ok := h.fields(w, p)
	if !ok {
		return
	}

	event, err := h.events.Create(r.Context(), userID, fields)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.decorate(r, []models.Event{event}, userID)[0])
}

func (h *EventHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	existing, err := h.events.FindByID(r.Context(), chi.URLParam(r, "eventId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !h.canWrite(r, userID, existing.ClubID) {
		writeError(w, http.StatusForbidden, "You cannot edit this event", CodeForbidden)
		return
	}

	var p eventPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	// The club is settled at creation: moving an event between clubs would
	// change who can see it after the fact.
	p.ClubID = existing.ClubID
	fields, ok := h.fields(w, p)
	if !ok {
		return
	}

	event, err := h.events.Update(r.Context(), existing.ID, fields)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.decorate(r, []models.Event{event}, userID)[0])
}

func (h *EventHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	existing, err := h.events.FindByID(r.Context(), chi.URLParam(r, "eventId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !h.canWrite(r, userID, existing.ClubID) {
		writeError(w, http.StatusForbidden, "You cannot delete this event", CodeForbidden)
		return
	}
	if err := h.events.Delete(r.Context(), existing.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": existing.ID})
}
