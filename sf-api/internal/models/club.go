package models

import "time"

// Role is a user's role inside one club.
type Role string

const (
	// RoleOwner is the coach who created the club. Exactly one per club, and
	// it can't be removed or demoted.
	RoleOwner Role = "owner"
	// RoleAdmin is a member promoted by the owner: they can add/remove members
	// and upload programs, but not delete the club or touch the owner.
	RoleAdmin Role = "admin"
	// RoleMember is an athlete: they run the programs assigned to them and
	// post to the club feed.
	RoleMember Role = "member"
	// RoleNutritionist is a professional a coach invited to read the club's
	// athletes - their training and their health data - the way the coach does,
	// without the power to add/remove members or write programs. A read-only
	// seat, so it is not a manager (see CanManageClub) but it can view member
	// data (see CanViewMemberData).
	//
	// The seat limits only what they may do to the club and to other members:
	// like any member they can still be assigned programs and run them
	// (ProgramHandler.Assign gates on club membership, not role), so a
	// nutritionist - or a coach invited as a plain member - trains exactly like
	// an athlete. The role is kept distinct for ergonomics and for nutrition
	// features to come, not to restrict training.
	RoleNutritionist Role = "nutritionist"
)

// IsValidRole reports whether role is a club role that can be assigned. Owner
// is excluded: it's set once at creation and transferred, never granted.
func IsValidRole(role Role) bool {
	return role == RoleAdmin || role == RoleMember || role == RoleNutritionist
}

// CanManageClub reports whether role may add/remove members and upload or
// delete programs. A nutritionist cannot: they read, they do not run the club.
func CanManageClub(role Role) bool {
	return role == RoleOwner || role == RoleAdmin
}

// CanViewMemberData reports whether role may read another member's training and
// health data in the club - the coaches who run it and the nutritionist they
// invited to advise. The read counterpart of CanManageClub.
func CanViewMemberData(role Role) bool {
	return role == RoleOwner || role == RoleAdmin || role == RoleNutritionist
}

type Club struct {
	ID          string  `json:"id"`
	OwnerID     string  `json:"ownerId"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	City        string  `json:"city,omitempty"`
	Country     string  `json:"country,omitempty"`
	Picture     string  `json:"picture,omitempty"`
	PictureX    float64 `json:"pictureX"`
	PictureY    float64 `json:"pictureY"`
	// Role is the calling user's own role in this club, filled in by the
	// listing endpoints so the frontend can decide what to show without a
	// second round trip. Empty when the caller isn't a member (a superadmin
	// listing every club).
	Role        Role      `json:"role,omitempty"`
	MemberCount int       `json:"memberCount"`
	OwnerName   string    `json:"ownerName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ClubMember struct {
	ID        string    `json:"id"`
	ClubID    string    `json:"clubId"`
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	Handle    string    `json:"handle,omitempty"`
	Name      string    `json:"name"`
	Surname   string    `json:"surname"`
	Picture   string    `json:"picture,omitempty"`
	PictureX  float64   `json:"pictureX"`
	PictureY  float64   `json:"pictureY"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
