package handlers

import (
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

// Autoregulation: within one session, a lift is loaded off what the member has
// just shown it to be worth.
//
// A program written in RPE says how hard a set should feel, and the weight that
// makes it feel that way is not the same on every day. When a member logs the
// RPE a set actually came out at, that set estimates a max (loadcalc.E1RM) -
// and the rest of that session's work on the same lift is resolved against
// *that* rather than against the 1RM on file. A top single at RPE 9 instead of
// the prescribed 8 brings the back-offs down with it.
//
// Two boundaries, both deliberate:
//
//   - It lasts the session. Sets arrive in week/day/position order, so a change
//     of day clears everything: a heavy morning says what the bar felt like
//     that morning, not what the member's max has become. Changing the stored
//     1RM stays a decision they make on the 1RM screen.
//   - It needs a perceived RPE. Without one, E1RM reads a logged set as an
//     all-out effort, and every later load would drop on the say-so of somebody
//     who only ticked the set off.
type sessionMaxes struct {
	day   string
	shown map[string]float64
}

func newSessionMaxes() *sessionMaxes {
	return &sessionMaxes{shown: map[string]float64{}}
}

// enter moves to the set's session, forgetting the previous one's work.
func (s *sessionMaxes) enter(dayID string) {
	if dayID != s.day {
		s.day = dayID
		s.shown = map[string]float64{}
	}
}

// forLift returns the max this session has demonstrated for one lift, if any.
func (s *sessionMaxes) forLift(sourceID string) (float64, bool) {
	if utils.IsBlank(sourceID) {
		return 0, false
	}
	shown, ok := s.shown[sourceID]
	if !ok || shown <= 0 {
		return 0, false
	}
	return shown, true
}

// record notes what a logged set demonstrated, for the sets that follow it.
func (s *sessionMaxes) record(sourceID string, log models.SetLog) {
	if utils.IsBlank(sourceID) || log.ActualRPE == nil || log.E1RM <= 0 {
		return
	}
	s.shown[sourceID] = log.E1RM
}
