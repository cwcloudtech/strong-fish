package handlers

import (
	"context"
	"strings"
	"time"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// birthdayEvents turns the birthdates of a caller's club-mates into calendar
// entries.
//
// They are derived on read rather than stored as rows: a birthday has no
// author, cannot be edited, and has to vanish the moment its owner clears the
// date or narrows their profile - none of which a row in the events table would
// do by itself.
//
// The audience is deliberately narrower than "everyone who may see the
// profile". A birthdate is personal, and a public profile is readable by the
// whole internet; filling one in should not put your date of birth into
// strangers' calendars. So it travels to the people you actually train with -
// your club-mates - and only when they may see your profile at all.
func birthdayEvents(ctx context.Context, users *store.UserStore, clubs *store.ClubStore,
	callerID string, from time.Time) ([]models.Event, error) {
	if utils.IsBlank(callerID) {
		return nil, nil
	}

	mateIDs, err := clubs.ListClubMateIDs(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if len(mateIDs) == 0 {
		return nil, nil
	}

	// Four fields per member, not their whole profile: an avatar is stored
	// inline as base64, and reading 300 of them to draw a birthday list was
	// tens of megabytes off the database per calendar load.
	mates, err := users.ListBirthdays(ctx, mateIDs)
	if err != nil {
		return nil, err
	}

	// A club-mate always shares a club by construction, so the only question
	// left per profile is whether the caller manages one - which "private"
	// needs.
	caller, err := users.FindByID(ctx, callerID)
	if err != nil {
		return nil, err
	}
	superadmin := caller.Role == models.GlobalRoleSuperadmin

	if from.IsZero() {
		from = time.Now().UTC()
	}

	// Asked once for every club-mate at a time, rather than once per mate: the
	// answer is a single join, and doing it in the loop below was 300 round
	// trips on a 300-member club's calendar.
	managed, err := clubs.ManagedMembers(ctx, mateIDs, callerID)
	if err != nil {
		return nil, err
	}

	events := []models.Event{}
	for _, mate := range mates {
		birthdate, err := time.Parse("2006-01-02", mate.Birthdate)
		if err != nil {
			continue
		}

		// A club-mate shares a club by construction - that is where the ids
		// came from - so the only questions left are the two below.
		relation := models.ViewerRelation{
			SharesClub:  true,
			ManagesClub: managed[mate.UserID],
			Superadmin:  superadmin,
		}
		if !models.CanSeeProfile(mate.ProfileVisibility, relation) {
			continue
		}

		next := models.NextOccurrence(birthdate, from)
		name := strings.TrimSpace(mate.Name + " " + mate.Surname)
		if utils.IsBlank(name) {
			name = mate.Handle
		}

		events = append(events, models.Event{
			// Synthetic, and stable across years on purpose: the ICS feed emits
			// this once with a yearly recurrence rule, so a UID carrying the
			// year would make every January look like a brand-new event.
			ID:       "birthday-" + mate.UserID,
			Title:    name,
			Kind:     models.EventKindBirthday,
			StartsAt: next,
			// A whole day, and the ICS renders it as one because of the kind
			// (see models.Event.WholeDay) rather than a flag on the entry.
			EndsAt:     next.AddDate(0, 0, 1),
			Visibility: models.VisibilityClub,
			// No picture: the same reason a stored event does not carry one
			// (see store.eventSelect) - an inline avatar per entry is most of
			// the calendar's weight and nothing draws it.
			Author: models.UserSummary{
				ID: mate.UserID, Handle: mate.Handle, Name: mate.Name, Surname: mate.Surname,
			},
		})
	}
	return events, nil
}
