package models

import "time"

// Invitation statuses.
const (
	InvitationStatusPending  = "pending"
	InvitationStatusAccepted = "accepted"
	InvitationStatusDeclined = "declined"
)

// Invitation is a coach asking somebody to join their club.
//
// It is addressed by email rather than by user id, because most of the point is
// inviting people who have no account yet. Matching on the address at accept
// time means an account created a week after the invitation still finds it
// waiting - and it means the invitation carries nothing anybody could guess
// their way into.
type Invitation struct {
	ID        string `json:"id"`
	ClubID    string `json:"clubId"`
	ClubName  string `json:"clubName,omitempty"`
	InviterID string `json:"inviterId"`
	// InviterName is denormalized so the invitee sees who asked without a
	// second lookup - and without being able to read the inviter's profile.
	InviterName string `json:"inviterName,omitempty"`
	Email       string `json:"email"`
	// Role is what the invitee joins as, so a coach can invite a co-admin
	// directly rather than adding them and promoting them afterwards.
	Role    Role   `json:"role"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	// MemberCount lets the invitee see how big the club is before accepting.
	MemberCount int       `json:"memberCount,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Coach request statuses. An account that asked to be a coach at signup waits
// on a superadmin: being a coach means creating clubs and writing other
// people's training, which is not a claim the app takes at face value.
const (
	CoachRequestPending  = "pending"
	CoachRequestApproved = "approved"
	CoachRequestRejected = "rejected"
)

// IsValidCoachRequestStatus reports whether status is a decision a superadmin
// may record. "pending" is deliberately absent: it is where a request starts,
// never somewhere it is put back.
func IsValidCoachRequestStatus(status string) bool {
	return status == CoachRequestApproved || status == CoachRequestRejected
}

// CoachRequest is one account's claim to be a coach, and what came of it.
type CoachRequest struct {
	Status string `json:"status,omitempty"`
	// Motive is why a request was rejected. It is shown to the applicant, so it
	// is written for them rather than as an internal note.
	Motive      string     `json:"motive,omitempty"`
	RequestedAt *time.Time `json:"requestedAt,omitempty"`
	DecidedAt   *time.Time `json:"decidedAt,omitempty"`
	DecidedBy   string     `json:"decidedBy,omitempty"`
}

// Pending reports whether this request is still waiting on a decision.
func (c CoachRequest) Pending() bool {
	return c.Status == CoachRequestPending
}

// CoachApplicant is one pending request as the superadmin's queue shows it.
type CoachApplicant struct {
	UserSummary
	Request   CoachRequest `json:"request"`
	CreatedAt time.Time    `json:"createdAt"`
}
