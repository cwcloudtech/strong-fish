package models

import "time"

// Post visibility.
const (
	// VisibilityPublic is readable by everyone, including logged-out visitors
	// browsing a public profile.
	VisibilityPublic = "public"
	// VisibilityClub restricts a post to the members of the club it was posted
	// to.
	VisibilityClub = "club"
)

// IsValidVisibility reports whether visibility is a known post visibility.
func IsValidVisibility(visibility string) bool {
	return visibility == VisibilityPublic || visibility == VisibilityClub
}

// Post is one entry in the feed. Pictures are base64 data URIs carried inline
// in the JSONB payload; links are raw URLs the client renders through the
// <media-player> web component, which detects YouTube/Vimeo/Dailymotion and
// friends and embeds the right player.
type Post struct {
	ID       string `json:"id"`
	AuthorID string `json:"authorId"`
	// ClubID is set only for a club-visibility post.
	ClubID     string      `json:"clubId,omitempty"`
	ClubName   string      `json:"clubName,omitempty"`
	Author     UserSummary `json:"author"`
	Content    string      `json:"content"`
	Pictures   []string    `json:"pictures,omitempty"`
	Links      []string    `json:"links,omitempty"`
	Visibility string      `json:"visibility"`
	Likes      int         `json:"likes"`
	// Liked reports whether the *caller* liked this post.
	Liked    bool `json:"liked"`
	Comments int  `json:"comments"`
	// Editable/Deletable tell the client which actions to offer: the author
	// always, plus a superadmin on every post and comment.
	Editable  bool      `json:"editable"`
	Deletable bool      `json:"deletable"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Comment is one reply under a post.
type Comment struct {
	ID        string      `json:"id"`
	PostID    string      `json:"postId"`
	AuthorID  string      `json:"authorId"`
	Author    UserSummary `json:"author"`
	Content   string      `json:"content"`
	Editable  bool        `json:"editable"`
	Deletable bool        `json:"deletable"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// Report target types.
const (
	ReportTargetPost    = "post"
	ReportTargetComment = "comment"
	ReportTargetUser    = "user"
)

// IsValidReportTarget reports whether targetType is a known report target.
func IsValidReportTarget(targetType string) bool {
	switch targetType {
	case ReportTargetPost, ReportTargetComment, ReportTargetUser:
		return true
	}
	return false
}

// Report statuses.
const (
	ReportStatusOpen      = "open"
	ReportStatusResolved  = "resolved"
	ReportStatusDismissed = "dismissed"
)

// IsValidReportStatus reports whether status is a known report status.
func IsValidReportStatus(status string) bool {
	switch status {
	case ReportStatusOpen, ReportStatusResolved, ReportStatusDismissed:
		return true
	}
	return false
}

// Report is a piece of content a user denounced, for the superadmin's
// moderation queue.
type Report struct {
	ID         string      `json:"id"`
	ReporterID string      `json:"reporterId"`
	Reporter   UserSummary `json:"reporter"`
	TargetType string      `json:"targetType"`
	TargetID   string      `json:"targetId"`
	Reason     string      `json:"reason"`
	Comment    string      `json:"comment,omitempty"`
	Status     string      `json:"status"`
	// Snapshot is the reported content as it read when the report was filed,
	// so the queue still shows what was denounced after the original is edited
	// or deleted.
	Snapshot   string     `json:"snapshot,omitempty"`
	ResolvedBy string     `json:"resolvedBy,omitempty"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// Page is the envelope every paginated listing returns, mirroring the
// frontend's infinite-scroll expectations (results plus a total to compare
// against).
type Page[T any] struct {
	Results      []T `json:"results"`
	TotalResults int `json:"totalResults"`
}
