package models

import (
	"regexp"
	"strings"
	"time"
)

// hexColor is the only colour shape the calendar can draw: six digits with a
// leading hash. Anything else - a name, a shorthand, something a client made
// up - is dropped rather than passed through to a stylesheet.
var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// NormalizeHexColor keeps a usable colour and discards anything else. An empty
// result means "no colour of its own", and the client falls back to the kind's.
func NormalizeHexColor(value string) string {
	value = strings.TrimSpace(value)
	if hexColor.MatchString(value) {
		return strings.ToLower(value)
	}
	return ""
}

// EventVisibilityPrivate is an event only its author - and a superadmin, who
// moderates everything - can see.
//
// It is what a member gets when they add something to their own calendar: a
// meet they mean to enter, a session they plan. They have no club to publish
// to and no business publishing to the open calendar, but a personal calendar
// is still worth having.
const EventVisibilityPrivate = "private"

// Event kinds. The kind is a label, not a behaviour - a competition and a
// training camp are stored and served identically - but it's what lets the
// calendar colour-code and filter.
const (
	EventKindCompetition = "competition"
	EventKindTraining    = "training"
	EventKindOther       = "other"
	// EventKindBirthday is never stored: it labels the entries derived from
	// members' birthdates (see handlers.birthdayEvents), so a client can style
	// them and a reader can tell them from a date somebody actually scheduled.
	EventKindBirthday = "birthday"
)

// IsValidEventKind reports whether kind is one this app knows.
// Birthdays are excluded: they are derived, so no payload may claim to be one.
func IsValidEventKind(kind string) bool {
	switch kind {
	case EventKindCompetition, EventKindTraining, EventKindOther:
		return true
	}
	return false
}

// NormalizeEventKind maps anything unrecognized - including the empty string -
// onto "other", so an unknown label can never make an event disappear from a
// filtered view.
func NormalizeEventKind(kind string) string {
	if IsValidEventKind(kind) {
		return kind
	}
	return EventKindOther
}

// Event is a date in the calendar: a meet, a club session, a camp.
//
// Times are absolute instants rather than the app's usual day + time-of-day,
// because an event has a real one - a meet starts at a stated hour in a stated
// place, and somebody subscribing from another timezone still needs to be
// there then. An all-day event is the exception and is emitted as a plain date
// in the ICS feed.
type Event struct {
	ID string `json:"id"`
	// ClubID is empty for an event that belongs to no club - a federation meet
	// anybody can see rather than one club's own date.
	ClubID      string      `json:"clubId,omitempty"`
	ClubName    string      `json:"clubName,omitempty"`
	AuthorID    string      `json:"authorId"`
	Author      UserSummary `json:"author"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Location    string      `json:"location,omitempty"`
	// URL is the meet's own page - the entry form, the federation listing.
	URL  string `json:"url,omitempty"`
	Kind string `json:"kind"`
	// Color is the hex the calendar draws this event in. Chosen per event
	// rather than per kind, because what somebody wants to tell apart at a
	// glance is their own blocks, not the four labels the app knows about.
	// Empty means "no colour of its own", and the client falls back to the
	// kind's.
	Color    string    `json:"color,omitempty"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt,omitempty"`
	AllDay   bool      `json:"allDay"`
	// Visibility reuses the post visibilities: club-only, or public (which is
	// what puts a meet in front of somebody who isn't a member yet).
	Visibility string    `json:"visibility"`
	Editable   bool      `json:"editable"`
	Deletable  bool      `json:"deletable"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// End returns the instant the event finishes, defaulting to an hour after it
// starts when the author gave no end - a calendar entry with no duration
// renders as a zero-length blip in Outlook.
func (e Event) End() time.Time {
	if !e.EndsAt.IsZero() && e.EndsAt.After(e.StartsAt) {
		return e.EndsAt
	}
	return e.StartsAt.Add(time.Hour)
}

// WholeDay reports whether this entry occupies a day rather than a time of day.
//
// It is derived from the kind, not chosen: a birthday is the only whole-day
// thing in this calendar, and it is generated rather than authored. Events
// people create always happen at a stated time - a meet starts when it starts.
func (e Event) WholeDay() bool {
	return e.Kind == EventKindBirthday
}
