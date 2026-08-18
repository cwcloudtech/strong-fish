package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

type SocialStore struct {
	pool *pgxpool.Pool
}

func NewSocialStore(pool *pgxpool.Pool) *SocialStore {
	return &SocialStore{pool: pool}
}

// postData is the JSONB payload of the posts table. Pictures are base64 data
// URIs stored inline (see the size cap in config), and links are raw URLs the
// client renders through the <media-player> element.
type postData struct {
	Content    string   `json:"content"`
	Pictures   []string `json:"pictures,omitempty"`
	Links      []string `json:"links,omitempty"`
	Visibility string   `json:"visibility"`
}

// commentData is the JSONB payload of the post_comments table.
type commentData struct {
	Content string `json:"content"`
}

// postSelect resolves everything a feed card needs in one row: the author, the
// club a club-only post belongs to, the like and comment counts, and whether
// the *caller* ($1) liked it.
var postSelect = `
	SELECT p.id, p.author_id, coalesce(p.club_id::text, ''), coalesce(c.data->>'name', ''),
	       p.data, p.created_at, p.updated_at,
	       coalesce(u.data->>'handle', ''), ` + displayName("u") + `,
	       ` + displaySurname("u") + `, coalesce(u.data->>'role', ''),
	       coalesce(u.data->>'picture', ''),
	       coalesce((u.data->>'pictureX')::float, 50), coalesce((u.data->>'pictureY')::float, 50),
	       (SELECT count(*) FROM post_likes WHERE post_id = p.id),
	       (SELECT count(*) FROM post_comments WHERE post_id = p.id),
	       exists(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1)
	FROM posts p
	JOIN users u ON u.id = p.author_id
	LEFT JOIN clubs c ON c.id = p.club_id`

func scanPost(row pgx.Row) (models.Post, error) {
	var p models.Post
	var raw []byte
	if err := row.Scan(&p.ID, &p.AuthorID, &p.ClubID, &p.ClubName, &raw, &p.CreatedAt, &p.UpdatedAt,
		&p.Author.Handle, &p.Author.Name, &p.Author.Surname, &p.Author.Role,
		&p.Author.Picture, &p.Author.PictureX, &p.Author.PictureY,
		&p.Likes, &p.Comments, &p.Liked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Post{}, ErrNotFound
		}
		return models.Post{}, err
	}
	var d postData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Post{}, err
	}
	p.Author.ID = p.AuthorID
	p.Content = d.Content
	p.Pictures = d.Pictures
	p.Links = d.Links
	p.Visibility = d.Visibility
	return p, nil
}

func scanPosts(rows pgx.Rows) ([]models.Post, error) {
	defer rows.Close()
	posts := []models.Post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

// PostFields is a post as composed by its author.
type PostFields struct {
	Content    string
	Pictures   []string
	Links      []string
	Visibility string
	// ClubID is required for a club-visibility post and empty for a public one.
	ClubID string
}

func (f PostFields) payload() ([]byte, error) {
	return json.Marshal(postData{
		Content: f.Content, Pictures: f.Pictures, Links: f.Links, Visibility: f.Visibility,
	})
}

// clubIDArg turns an empty club id into a NULL, which is how a public post is
// stored.
func clubIDArg(clubID string) any {
	if utils.IsBlank(clubID) {
		return nil
	}
	return clubID
}

func (s *SocialStore) CreatePost(ctx context.Context, authorID string, f PostFields) (models.Post, error) {
	data, err := f.payload()
	if err != nil {
		return models.Post{}, err
	}
	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO posts (author_id, club_id, data) VALUES ($1, $2, $3) RETURNING id
	`, authorID, clubIDArg(f.ClubID), data).Scan(&id); err != nil {
		return models.Post{}, err
	}
	return s.FindPost(ctx, id, authorID)
}

// UpdatePost rewrites a post's content. The visibility and club are settled at
// creation: moving a post between clubs (or from a club to the public feed)
// would retroactively expose it to people who couldn't see it.
func (s *SocialStore) UpdatePost(ctx context.Context, id, callerID string, f PostFields) (models.Post, error) {
	patch, err := json.Marshal(map[string]any{
		"content":  f.Content,
		"pictures": f.Pictures,
		"links":    f.Links,
	})
	if err != nil {
		return models.Post{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE posts SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, id, patch)
	if err != nil {
		return models.Post{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Post{}, ErrNotFound
	}
	return s.FindPost(ctx, id, callerID)
}

func (s *SocialStore) FindPost(ctx context.Context, id, callerID string) (models.Post, error) {
	return scanPost(s.pool.QueryRow(ctx, postSelect+` WHERE p.id = $2`, callerID, id))
}

// FindPublicPost returns a post only when its author published it to
// everybody.
//
// The visibility predicate is in the query rather than in a caller-side check,
// so the unauthenticated path cannot read a club-only post even if it forgets
// to look. The caller id is the empty placeholder: there is nobody to resolve
// "did I like this" against.
func (s *SocialStore) FindPublicPost(ctx context.Context, id string) (models.Post, error) {
	return scanPost(s.pool.QueryRow(ctx,
		postSelect+` WHERE p.id = $2 AND p.data->>'visibility' = $3`,
		anonymousUserID, id, models.VisibilityPublic))
}

func (s *SocialStore) DeletePost(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// visibilityClause is the rule every feed query shares: a post is readable when
// it's public, or when it belongs to a club the caller is in, or when the caller
// wrote it. $1 is the caller's id and $2 their club ids.
const visibilityClause = `
	(p.club_id IS NULL OR p.club_id = ANY($2) OR p.author_id = $1)`

// notBlockedClause drops posts whose author the caller has blocked, or who has
// blocked the caller. $1 is the caller's id.
//
// Both directions: a block is one person's decision, but its effect is mutual -
// the blocker should not have to see the person they blocked, and the blocked
// person should not keep appearing in front of somebody who stopped listening.
//
// It lives in the query rather than filtering the results, for the same reason
// the visibility clause does: a caller-side filter makes the page counts wrong,
// and it is only applied where somebody remembered to apply it.
const notBlockedClause = `
	NOT EXISTS (
		SELECT 1 FROM blocks b
		WHERE (b.blocker_id = $1 AND b.blocked_id = p.author_id)
		   OR (b.blocked_id = $1 AND b.blocker_id = p.author_id)
	)`

// ListFeed is the newspaper: posts from the people the caller follows, plus
// their own, plus anything posted to a club they're in - newest first.
func (s *SocialStore) ListFeed(ctx context.Context, callerID string, clubIDs []string, page, size int) ([]models.Post, int, error) {
	limit, offset := clampPage(page, size, 100)

	const scope = `
		WHERE ` + visibilityClause + `
		  AND ` + notBlockedClause + `
		  AND (p.author_id = $1
		       OR p.author_id IN (SELECT following_id FROM follows WHERE follower_id = $1)
		       OR p.club_id = ANY($2))`

	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM posts p `+scope, callerID, clubIDs).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, postSelect+scope+`
		ORDER BY p.created_at DESC LIMIT $3 OFFSET $4`, callerID, clubIDs, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	posts, err := scanPosts(rows)
	return posts, total, err
}

// ListDiscoverFeed is the feed for someone who follows nobody yet: every public
// post, newest first.
func (s *SocialStore) ListDiscoverFeed(ctx context.Context, callerID string, page, size int) ([]models.Post, int, error) {
	limit, offset := clampPage(page, size, 100)

	// Discover is reachable by a logged-out visitor, who has blocked nobody -
	// the placeholder id keeps the same SQL working for both.
	caller := callerID
	if utils.IsBlank(caller) {
		caller = anonymousUserID
	}

	const scope = ` WHERE p.club_id IS NULL AND ` + notBlockedClause

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM posts p`+scope, caller).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, postSelect+scope+`
		ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`, caller, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	posts, err := scanPosts(rows)
	return posts, total, err
}

// ListClubFeed returns one club's posts. Callers must have already checked the
// caller is a member.
func (s *SocialStore) ListClubFeed(ctx context.Context, clubID, callerID string, page, size int) ([]models.Post, int, error) {
	limit, offset := clampPage(page, size, 100)

	const scope = ` WHERE p.club_id = $2 AND ` + notBlockedClause

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM posts p`+scope, callerID, clubID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, postSelect+scope+`
		ORDER BY p.created_at DESC LIMIT $3 OFFSET $4`, callerID, clubID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	posts, err := scanPosts(rows)
	return posts, total, err
}

// ListProfilePosts returns one author's posts as visible to the caller. An
// anonymous visitor (blank callerID, empty clubIDs) sees only the public ones,
// which is what makes a shared profile link work logged out.
func (s *SocialStore) ListProfilePosts(ctx context.Context, authorID, callerID string, clubIDs []string, page, size int) ([]models.Post, int, error) {
	limit, offset := clampPage(page, size, 100)

	// A blank caller id can't be compared against a uuid column, so anonymous
	// visitors are given an id that matches nothing.
	caller := callerID
	if utils.IsBlank(caller) {
		caller = anonymousUserID
	}

	const scope = ` WHERE p.author_id = $3 AND ` + visibilityClause + ` AND ` + notBlockedClause

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM posts p`+scope, caller, clubIDs, authorID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, postSelect+scope+`
		ORDER BY p.created_at DESC LIMIT $4 OFFSET $5`, caller, clubIDs, authorID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	posts, err := scanPosts(rows)
	return posts, total, err
}

// anonymousUserID is a well-formed uuid that belongs to nobody, used as the
// "caller" for logged-out reads so visibility SQL doesn't need a separate
// anonymous variant.
const anonymousUserID = "00000000-0000-0000-0000-000000000000"

// CanSeePost reports whether callerID may read a post - a public post is
// readable by anyone, a club post only by that club's members.
func (s *SocialStore) CanSeePost(ctx context.Context, postID, callerID string, clubIDs []string) (bool, error) {
	caller := callerID
	if utils.IsBlank(caller) {
		caller = anonymousUserID
	}
	var visible bool
	err := s.pool.QueryRow(ctx, `
		SELECT exists(SELECT 1 FROM posts p WHERE p.id = $3 AND `+visibilityClause+`)
	`, caller, clubIDs, postID).Scan(&visible)
	return visible, err
}

// --- likes ---

// Like records a like, ignoring a repeat from the same user.
func (s *SocialStore) Like(ctx context.Context, postID, userID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO post_likes (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
	`, postID, userID)
	return err
}

func (s *SocialStore) Unlike(ctx context.Context, postID, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM post_likes WHERE post_id = $1 AND user_id = $2`, postID, userID)
	return err
}

// --- comments ---

var commentSelect = `
	SELECT c.id, c.post_id, c.author_id, c.data, c.created_at, c.updated_at,
	       coalesce(u.data->>'handle', ''), ` + displayName("u") + `,
	       ` + displaySurname("u") + `, coalesce(u.data->>'role', ''),
	       coalesce(u.data->>'picture', ''),
	       coalesce((u.data->>'pictureX')::float, 50), coalesce((u.data->>'pictureY')::float, 50)
	FROM post_comments c
	JOIN users u ON u.id = c.author_id`

func scanComment(row pgx.Row) (models.Comment, error) {
	var c models.Comment
	var raw []byte
	if err := row.Scan(&c.ID, &c.PostID, &c.AuthorID, &raw, &c.CreatedAt, &c.UpdatedAt,
		&c.Author.Handle, &c.Author.Name, &c.Author.Surname, &c.Author.Role,
		&c.Author.Picture, &c.Author.PictureX, &c.Author.PictureY); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Comment{}, ErrNotFound
		}
		return models.Comment{}, err
	}
	var d commentData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Comment{}, err
	}
	c.Author.ID = c.AuthorID
	c.Content = d.Content
	return c, nil
}

func (s *SocialStore) CreateComment(ctx context.Context, postID, authorID, content string) (models.Comment, error) {
	data, err := json.Marshal(commentData{Content: content})
	if err != nil {
		return models.Comment{}, err
	}
	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO post_comments (post_id, author_id, data) VALUES ($1, $2, $3) RETURNING id
	`, postID, authorID, data).Scan(&id); err != nil {
		return models.Comment{}, err
	}
	return s.FindComment(ctx, id)
}

func (s *SocialStore) FindComment(ctx context.Context, id string) (models.Comment, error) {
	return scanComment(s.pool.QueryRow(ctx, commentSelect+` WHERE c.id = $1`, id))
}

func (s *SocialStore) UpdateComment(ctx context.Context, id, content string) (models.Comment, error) {
	patch, err := json.Marshal(map[string]any{"content": content})
	if err != nil {
		return models.Comment{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE post_comments SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, id, patch)
	if err != nil {
		return models.Comment{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Comment{}, ErrNotFound
	}
	return s.FindComment(ctx, id)
}

func (s *SocialStore) DeleteComment(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM post_comments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SocialStore) ListComments(ctx context.Context, postID string, page, size int) ([]models.Comment, int, error) {
	limit, offset := clampPage(page, size, 100)

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM post_comments WHERE post_id = $1`, postID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, commentSelect+`
		WHERE c.post_id = $1 ORDER BY c.created_at LIMIT $2 OFFSET $3`, postID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	comments := []models.Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, 0, err
		}
		comments = append(comments, c)
	}
	return comments, total, rows.Err()
}

// --- follows ---

func (s *SocialStore) Follow(ctx context.Context, followerID, followingID string) error {
	if followerID == followingID {
		// The table's CHECK would reject this anyway; catching it here keeps
		// the error a clean no-op rather than a constraint violation.
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO follows (follower_id, following_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
	`, followerID, followingID)
	return err
}

func (s *SocialStore) Unfollow(ctx context.Context, followerID, followingID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`, followerID, followingID)
	return err
}

// FollowCounts returns how many people follow userID, how many they follow, and
// whether callerID is one of them.
func (s *SocialStore) FollowCounts(ctx context.Context, userID, callerID string) (followers, following int, followed bool, err error) {
	caller := callerID
	if utils.IsBlank(caller) {
		caller = anonymousUserID
	}
	err = s.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM follows WHERE following_id = $1),
		       (SELECT count(*) FROM follows WHERE follower_id = $1),
		       exists(SELECT 1 FROM follows WHERE follower_id = $2 AND following_id = $1)
	`, userID, caller).Scan(&followers, &following, &followed)
	return
}

// ListFollows returns the accounts userID follows (or, when followers is true,
// the accounts following them).
func (s *SocialStore) ListFollows(ctx context.Context, userID string, followers bool) ([]models.UserSummary, error) {
	query := `
		SELECT u.id, coalesce(u.data->>'handle', ''), ` + displayName("u") + `,
		       ` + displaySurname("u") + `, coalesce(u.data->>'role', ''),
		       coalesce(u.data->>'picture', ''),
		       coalesce((u.data->>'pictureX')::float, 50), coalesce((u.data->>'pictureY')::float, 50)
		FROM follows f
		JOIN users u ON u.id = f.follower_id
		WHERE f.following_id = $1
		ORDER BY f.created_at DESC`
	if !followers {
		query = `
		SELECT u.id, coalesce(u.data->>'handle', ''), ` + displayName("u") + `,
		       ` + displaySurname("u") + `, coalesce(u.data->>'role', ''),
		       coalesce(u.data->>'picture', ''),
		       coalesce((u.data->>'pictureX')::float, 50), coalesce((u.data->>'pictureY')::float, 50)
		FROM follows f
		JOIN users u ON u.id = f.following_id
		WHERE f.follower_id = $1
		ORDER BY f.created_at DESC`
	}

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []models.UserSummary{}
	for rows.Next() {
		var u models.UserSummary
		if err := rows.Scan(&u.ID, &u.Handle, &u.Name, &u.Surname, &u.Role, &u.Picture, &u.PictureX, &u.PictureY); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// --- reports ---

// reportData is the JSONB payload of the content_reports table.
type reportData struct {
	Reason     string     `json:"reason"`
	Comment    string     `json:"comment,omitempty"`
	Status     string     `json:"status"`
	Snapshot   string     `json:"snapshot,omitempty"`
	ResolvedBy string     `json:"resolvedBy,omitempty"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

var reportSelect = `
	SELECT r.id, r.reporter_id, r.target_type, r.target_id, r.data, r.created_at, r.updated_at,
	       coalesce(u.data->>'handle', ''), ` + displayName("u") + `,
	       ` + displaySurname("u") + `, u.email, coalesce(u.data->>'role', '')
	FROM content_reports r
	JOIN users u ON u.id = r.reporter_id`

func scanReport(row pgx.Row) (models.Report, error) {
	var r models.Report
	var raw []byte
	if err := row.Scan(&r.ID, &r.ReporterID, &r.TargetType, &r.TargetID, &raw, &r.CreatedAt, &r.UpdatedAt,
		&r.Reporter.Handle, &r.Reporter.Name, &r.Reporter.Surname, &r.Reporter.Email, &r.Reporter.Role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Report{}, ErrNotFound
		}
		return models.Report{}, err
	}
	var d reportData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Report{}, err
	}
	r.Reporter.ID = r.ReporterID
	r.Reason = d.Reason
	r.Comment = d.Comment
	r.Status = d.Status
	r.Snapshot = d.Snapshot
	r.ResolvedBy = d.ResolvedBy
	r.ResolvedAt = d.ResolvedAt
	return r, nil
}

// CreateReport files a report. snapshot is the content as it read at the time,
// so the moderation queue still shows what was denounced after the original is
// edited or deleted.
func (s *SocialStore) CreateReport(ctx context.Context, reporterID, targetType, targetID, reason, comment, snapshot string) (models.Report, error) {
	data, err := json.Marshal(reportData{
		Reason: reason, Comment: comment, Status: models.ReportStatusOpen, Snapshot: snapshot,
	})
	if err != nil {
		return models.Report{}, err
	}
	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO content_reports (reporter_id, target_type, target_id, data)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, reporterID, targetType, targetID, data).Scan(&id); err != nil {
		return models.Report{}, err
	}
	return scanReport(s.pool.QueryRow(ctx, reportSelect+` WHERE r.id = $1`, id))
}

// ListReports returns the moderation queue, optionally filtered by status.
func (s *SocialStore) ListReports(ctx context.Context, status string, page, size int) ([]models.Report, int, error) {
	limit, offset := clampPage(page, size, 100)

	// An empty status means "every status"; passing it through as a nullable
	// argument keeps one query rather than two.
	var filter any
	if utils.IsNotBlank(status) {
		filter = status
	}

	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM content_reports WHERE $1::text IS NULL OR data->>'status' = $1
	`, filter).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, reportSelect+`
		WHERE $1::text IS NULL OR r.data->>'status' = $1
		ORDER BY r.created_at DESC LIMIT $2 OFFSET $3`, filter, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	reports := []models.Report{}
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, r)
	}
	return reports, total, rows.Err()
}

// ResolveReport closes a report as handled or dismissed.
func (s *SocialStore) ResolveReport(ctx context.Context, id, status, resolvedBy string) (models.Report, error) {
	now := time.Now().UTC()
	patch, err := json.Marshal(map[string]any{
		"status": status, "resolvedBy": resolvedBy, "resolvedAt": now,
	})
	if err != nil {
		return models.Report{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE content_reports SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, id, patch)
	if err != nil {
		return models.Report{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Report{}, ErrNotFound
	}
	return scanReport(s.pool.QueryRow(ctx, reportSelect+` WHERE r.id = $1`, id))
}

// CountOpenReports drives the moderation badge in the superadmin's navigation.
func (s *SocialStore) CountOpenReports(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM content_reports WHERE data->>'status' = $1
	`, models.ReportStatusOpen).Scan(&count)
	return count, err
}
