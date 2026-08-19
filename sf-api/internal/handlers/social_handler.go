package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// maxPostLength bounds a post's text. Pictures are capped separately by
// maxImageSize, since they're stored inline in the same payload.
const (
	maxPostLength    = 5000
	maxCommentLength = 2000
	maxPostPictures  = 4
	// maxPostClubs bounds how many clubs one post can be shared with. Nobody
	// coaches twenty clubs, and without a cap the list is a free multiplier on
	// the rows one request writes.
	maxPostClubs = 20
)

type SocialHandler struct {
	social       *store.SocialStore
	messages     *store.MessageStore
	clubs        *store.ClubStore
	users        *store.UserStore
	oneRMs       *store.OneRMStore
	maxImageSize int64
}

func NewSocialHandler(social *store.SocialStore, clubs *store.ClubStore, users *store.UserStore,
	oneRMs *store.OneRMStore, messages *store.MessageStore, maxImageSize int64) *SocialHandler {
	return &SocialHandler{
		social: social, clubs: clubs, users: users, oneRMs: oneRMs,
		messages: messages, maxImageSize: maxImageSize,
	}
}

// decorate fills in the per-caller action flags every post carries: its author
// may always edit and delete it, and a superadmin may moderate anyone's.
func decoratePost(post models.Post, callerID string, superadmin bool) models.Post {
	post.Editable = post.AuthorID == callerID
	post.Deletable = post.AuthorID == callerID || superadmin
	return post
}

// decorateComment marks what the caller may do, and has to agree with what
// authorizeComment actually enforces - a superadmin has always been able to
// edit a comment, but this reported otherwise, so no client ever offered it.
func decorateComment(comment models.Comment, callerID string, superadmin bool) models.Comment {
	writable := comment.AuthorID == callerID || superadmin
	comment.Editable = writable
	comment.Deletable = writable
	return comment
}

// caller resolves who is asking and whether they can moderate. An anonymous
// visitor (public profile, public feed) gets a blank id and no powers.
func (h *SocialHandler) caller(r *http.Request) (id string, superadmin bool, clubIDs []string) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return utils.EMPTY, false, []string{}
	}

	if user, err := h.users.FindByID(r.Context(), id); err == nil {
		superadmin = user.Role == models.GlobalRoleSuperadmin
	}
	if ids, err := h.clubs.ListClubIDsForUser(r.Context(), id); err == nil {
		clubIDs = ids
	} else {
		clubIDs = []string{}
	}
	return id, superadmin, clubIDs
}

type postPayload struct {
	Content  string   `json:"content"`
	Pictures []string `json:"pictures"`
	// Links is derived, never submitted: see validate. The field stays on the
	// payload so a client that still sends one gets it ignored rather than
	// rejected.
	Links      []string `json:"links"`
	Visibility string   `json:"visibility"`
	// ClubIDs are the clubs to share a club-only post with. The singular
	// clubId is still accepted so an older client keeps working; see
	// normalizeClubs.
	ClubIDs []string `json:"clubIds"`
	ClubID  string   `json:"clubId"`
}

// normalizeClubs folds the two ways a client can name clubs into one list, and
// empties it for a post that is not club-only.
//
// The singular field is what the mobile app sent before a post could go to
// several clubs. Accepting it costs one line and means an un-upgraded phone
// still posts to its club rather than silently posting to none.
func normalizeClubs(p *postPayload) {
	if utils.IsNotBlank(p.ClubID) {
		p.ClubIDs = append(p.ClubIDs, p.ClubID)
	}
	p.ClubID = utils.EMPTY

	if p.Visibility != models.VisibilityClub {
		// A public post belongs to no club, whatever the client sent.
		p.ClubIDs = nil
		return
	}

	seen := map[string]bool{}
	unique := make([]string, 0, len(p.ClubIDs))
	for _, id := range p.ClubIDs {
		if utils.IsBlank(id) || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	p.ClubIDs = unique
}

// validate checks a composed post. A post needs something in it - text or a
// picture - and a club-visibility post needs the club it belongs to.
//
// It is also where a post's link comes from. There is no "add a link" field
// any more: whatever URL the author pasted into the text is the post's link,
// so the two can't disagree and an edit can't leave a stale embed behind. Only
// the first is taken - a post gets one embed, not a wall of them - and the URL
// stays in the text where it was typed, the way every other feed does it.
func (h *SocialHandler) validate(w http.ResponseWriter, p *postPayload) bool {
	p.Content = strings.TrimSpace(p.Content)
	if len(p.Content) > maxPostLength {
		writeError(w, http.StatusBadRequest, "This post is too long", CodeEmptyPost)
		return false
	}

	p.Links = nil
	if link := utils.FirstURL(p.Content); utils.IsNotBlank(link) {
		p.Links = []string{link}
	}

	if utils.IsBlank(p.Content) && len(p.Pictures) == 0 {
		writeError(w, http.StatusBadRequest, "A post needs some text, a picture or a link", CodeEmptyPost)
		return false
	}
	if len(p.Pictures) > maxPostPictures {
		writeError(w, http.StatusBadRequest, "Too many attachments on this post", CodeEmptyPost)
		return false
	}
	for _, picture := range p.Pictures {
		if utils.ImageSizeExceeds(picture, h.maxImageSize) {
			writeError(w, http.StatusBadRequest, "One of the pictures is too large", CodeImageTooLarge)
			return false
		}
	}
	if !models.IsValidVisibility(p.Visibility) {
		writeError(w, http.StatusBadRequest, "Invalid visibility", CodeInvalidVisibility)
		return false
	}
	normalizeClubs(p)
	if p.Visibility == models.VisibilityClub && len(p.ClubIDs) == 0 {
		writeError(w, http.StatusBadRequest, "Please pick a club for a club-only post", CodeClubRequired)
		return false
	}
	if len(p.ClubIDs) > maxPostClubs {
		writeError(w, http.StatusBadRequest, "This post is shared with too many clubs", CodeClubRequired)
		return false
	}
	return true
}

// CreatePost publishes a post to the caller's followers, or to one of their
// clubs.
func (h *SocialHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	callerID, superadmin, _ := h.caller(r)

	var p postPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !h.validate(w, &p) {
		return
	}
	// Every club is checked, not just the first: a caller who belongs to one
	// club could otherwise name it alongside four they do not.
	for _, clubID := range p.ClubIDs {
		if _, err := h.clubs.FindMembership(r.Context(), clubID, callerID); err != nil {
			writeError(w, http.StatusForbidden, "You are not a member of this club", CodeNotAClubMember)
			return
		}
	}

	post, err := h.social.CreatePost(r.Context(), callerID, store.PostFields{
		Content: p.Content, Pictures: p.Pictures, Links: p.Links,
		Visibility: p.Visibility, ClubIDs: p.ClubIDs,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, decoratePost(post, callerID, superadmin))
}

// added returns the club ids in want that were not already in have.
func added(have, want []string) []string {
	existing := make(map[string]bool, len(have))
	for _, id := range have {
		existing[id] = true
	}
	var out []string
	for _, id := range want {
		if !existing[id] {
			out = append(out, id)
		}
	}
	return out
}

// Feed is the newspaper: posts from the people the caller follows, their own,
// and anything posted to a club they're in.
func (h *SocialHandler) Feed(w http.ResponseWriter, r *http.Request) {
	callerID, superadmin, clubIDs := h.caller(r)
	page, size := pagination(r)

	posts, total, err := h.social.ListFeed(r.Context(), callerID, clubIDs, page, size)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.respondPosts(w, posts, total, callerID, superadmin)
}

// Discover is the feed for someone who follows nobody yet: every public post.
func (h *SocialHandler) Discover(w http.ResponseWriter, r *http.Request) {
	callerID, superadmin, _ := h.caller(r)
	page, size := pagination(r)

	posts, total, err := h.social.ListDiscoverFeed(r.Context(), callerID, page, size)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.respondPosts(w, posts, total, callerID, superadmin)
}

// ClubFeed returns one club's posts. The route sits behind ClubMembership, so
// the caller is already known to be a member.
func (h *SocialHandler) ClubFeed(w http.ResponseWriter, r *http.Request) {
	callerID, superadmin, _ := h.caller(r)
	page, size := pagination(r)

	posts, total, err := h.social.ListClubFeed(r.Context(), chi.URLParam(r, "clubId"), callerID, page, size)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.respondPosts(w, posts, total, callerID, superadmin)
}

func (h *SocialHandler) respondPosts(w http.ResponseWriter, posts []models.Post, total int, callerID string, superadmin bool) {
	for i := range posts {
		posts[i] = decoratePost(posts[i], callerID, superadmin)
	}
	writeJSON(w, http.StatusOK, models.Page[models.Post]{Results: posts, TotalResults: total})
}

// authorizePost loads a post the caller may act on. write demands authorship (or
// a superadmin's moderation power); otherwise visibility alone is enough.
func (h *SocialHandler) authorizePost(w http.ResponseWriter, r *http.Request, write bool) (models.Post, string, bool, bool) {
	callerID, superadmin, clubIDs := h.caller(r)
	postID := chi.URLParam(r, "postId")

	visible, err := h.social.CanSeePost(r.Context(), postID, callerID, clubIDs)
	if err != nil {
		writeStoreError(w, err)
		return models.Post{}, callerID, superadmin, false
	}
	if !visible {
		writeError(w, http.StatusNotFound, "Post not found", CodeNotFound)
		return models.Post{}, callerID, superadmin, false
	}

	post, err := h.social.FindPost(r.Context(), postID, callerID)
	if err != nil {
		writeStoreError(w, err)
		return models.Post{}, callerID, superadmin, false
	}
	if write && post.AuthorID != callerID && !superadmin {
		writeError(w, http.StatusForbidden, "You cannot modify this post", CodeForbidden)
		return models.Post{}, callerID, superadmin, false
	}
	return post, callerID, superadmin, true
}

func (h *SocialHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	post, callerID, superadmin, ok := h.authorizePost(w, r, false)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, decoratePost(post, callerID, superadmin))
}

// UpdatePost rewrites a post, including where it is published. A superadmin may
// edit anyone's, which is the moderation power the instruction asks for.
//
// Visibility moves with the rest: an author who published to the wrong place,
// or thought better of it afterwards, can move the post rather than delete it
// and write it again. A client that sends no visibility keeps the current one,
// so an older client still edits content without silently republishing.
func (h *SocialHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	existing, callerID, superadmin, ok := h.authorizePost(w, r, true)
	if !ok {
		return
	}

	var p postPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Visibility) {
		p.Visibility = existing.Visibility
		p.ClubIDs = existing.ClubIDs
	}
	if !h.validate(w, &p) {
		return
	}
	// Clubs are checked against the *author*, not whoever is editing: the post
	// stays theirs, and a superadmin moving it into a club its author does not
	// belong to would publish them somewhere they never joined. Only the clubs
	// being added are checked, so an author who has since left a club can still
	// edit the wording of a post already shared there.
	for _, clubID := range added(existing.ClubIDs, p.ClubIDs) {
		if _, err := h.clubs.FindMembership(r.Context(), clubID, existing.AuthorID); err != nil {
			writeError(w, http.StatusForbidden, "The author is not a member of this club", CodeNotAClubMember)
			return
		}
	}

	post, err := h.social.UpdatePost(r.Context(), existing.ID, callerID, store.PostFields{
		Content: p.Content, Pictures: p.Pictures, Links: p.Links,
		Visibility: p.Visibility, ClubIDs: p.ClubIDs,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, decoratePost(post, callerID, superadmin))
}

// GetPublicPost serves a post its author published to everybody, to a caller
// who may well be anonymous - which is what makes a shared link work when a
// stranger opens it from Instagram.
//
// A club-only post is reported as missing rather than forbidden: the
// difference would confirm the id exists to somebody guessing.
func (h *SocialHandler) GetPublicPost(w http.ResponseWriter, r *http.Request) {
	post, err := h.social.FindPublicPost(r.Context(), chi.URLParam(r, "postId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "Post not found", CodeNotFound)
		return
	}

	// Paged like every other listing; the store applies its own default size
	// when the caller asks for none.
	page, size := pagination(r)
	comments, total, err := h.social.ListComments(r.Context(), post.ID, page, size)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"post": post,
		// Read-only for a visitor: replying needs an account.
		"comments":      comments,
		"totalComments": total,
	})
}

func (h *SocialHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	post, _, _, ok := h.authorizePost(w, r, true)
	if !ok {
		return
	}
	if err := h.social.DeletePost(r.Context(), post.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": post.ID})
}

// Like and Unlike toggle the caller's like on a post.
// Like records that somebody liked a post - somebody other than its author.
//
// Refused at the API rather than only hidden in the client: a like count is a
// signal about how a post landed with other people, and one the author can add
// to themselves is not that. The client hides the control, and this is what
// makes the rule true regardless of which client is asking.
func (h *SocialHandler) Like(w http.ResponseWriter, r *http.Request) {
	post, callerID, superadmin, ok := h.authorizePost(w, r, false)
	if !ok {
		return
	}
	if post.AuthorID == callerID {
		writeError(w, http.StatusForbidden, "You cannot like your own post", CodeCannotLikeOwnPost)
		return
	}
	if err := h.social.Like(r.Context(), post.ID, callerID); err != nil {
		writeStoreError(w, err)
		return
	}
	h.respondPost(w, r, post.ID, callerID, superadmin)
}

func (h *SocialHandler) Unlike(w http.ResponseWriter, r *http.Request) {
	post, callerID, superadmin, ok := h.authorizePost(w, r, false)
	if !ok {
		return
	}
	if err := h.social.Unlike(r.Context(), post.ID, callerID); err != nil {
		writeStoreError(w, err)
		return
	}
	h.respondPost(w, r, post.ID, callerID, superadmin)
}

// respondPost re-reads a post so the client gets the refreshed like count
// straight back rather than having to guess at it.
func (h *SocialHandler) respondPost(w http.ResponseWriter, r *http.Request, postID, callerID string, superadmin bool) {
	post, err := h.social.FindPost(r.Context(), postID, callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, decoratePost(post, callerID, superadmin))
}

// --- comments ---

type commentPayload struct {
	Content string `json:"content"`
}

func (h *SocialHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	post, callerID, superadmin, ok := h.authorizePost(w, r, false)
	if !ok {
		return
	}
	page, size := pagination(r)

	comments, total, err := h.social.ListComments(r.Context(), post.ID, page, size)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	for i := range comments {
		comments[i] = decorateComment(comments[i], callerID, superadmin)
	}
	writeJSON(w, http.StatusOK, models.Page[models.Comment]{Results: comments, TotalResults: total})
}

func (h *SocialHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	post, callerID, superadmin, ok := h.authorizePost(w, r, false)
	if !ok {
		return
	}

	var p commentPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	content := strings.TrimSpace(p.Content)
	if utils.IsBlank(content) || len(content) > maxCommentLength {
		writeError(w, http.StatusBadRequest, "A comment needs some text", CodeEmptyComment)
		return
	}

	comment, err := h.social.CreateComment(r.Context(), post.ID, callerID, content)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, decorateComment(comment, callerID, superadmin))
}

// authorizeComment loads a comment the caller may edit or delete: its author, or
// a superadmin moderating.
func (h *SocialHandler) authorizeComment(w http.ResponseWriter, r *http.Request) (models.Comment, string, bool, bool) {
	callerID, superadmin, _ := h.caller(r)

	comment, err := h.social.FindComment(r.Context(), chi.URLParam(r, "commentId"))
	if err != nil {
		writeStoreError(w, err)
		return models.Comment{}, callerID, superadmin, false
	}
	if comment.PostID != chi.URLParam(r, "postId") {
		writeError(w, http.StatusNotFound, "Comment not found", CodeNotFound)
		return models.Comment{}, callerID, superadmin, false
	}
	if comment.AuthorID != callerID && !superadmin {
		writeError(w, http.StatusForbidden, "You cannot modify this comment", CodeForbidden)
		return models.Comment{}, callerID, superadmin, false
	}
	return comment, callerID, superadmin, true
}

func (h *SocialHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	comment, callerID, superadmin, ok := h.authorizeComment(w, r)
	if !ok {
		return
	}

	var p commentPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	content := strings.TrimSpace(p.Content)
	if utils.IsBlank(content) || len(content) > maxCommentLength {
		writeError(w, http.StatusBadRequest, "A comment needs some text", CodeEmptyComment)
		return
	}

	updated, err := h.social.UpdateComment(r.Context(), comment.ID, content)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, decorateComment(updated, callerID, superadmin))
}

func (h *SocialHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	comment, _, _, ok := h.authorizeComment(w, r)
	if !ok {
		return
	}
	if err := h.social.DeleteComment(r.Context(), comment.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": comment.ID})
}

// --- follows ---

func (h *SocialHandler) Follow(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	target, err := h.users.FindByIDOrHandle(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if target.ID == callerID {
		writeError(w, http.StatusBadRequest, "You cannot follow yourself", CodeForbidden)
		return
	}
	if err := h.social.Follow(r.Context(), callerID, target.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"followed": true})
}

func (h *SocialHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	target, err := h.users.FindByIDOrHandle(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := h.social.Unfollow(r.Context(), callerID, target.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"followed": false})
}

// ListFollows returns who a profile follows, or - with ?direction=followers -
// who follows them.
func (h *SocialHandler) ListFollows(w http.ResponseWriter, r *http.Request) {
	target, err := h.users.FindByIDOrHandle(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	users, err := h.social.ListFollows(r.Context(), target.ID, r.URL.Query().Get("direction") == "followers")
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// --- reports ---

type reportPayload struct {
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Reason     string `json:"reason"`
	Comment    string `json:"comment"`
}

// Report files a piece of content for a superadmin to look at.
func (h *SocialHandler) Report(w http.ResponseWriter, r *http.Request) {
	callerID, _, clubIDs := h.caller(r)

	var p reportPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !models.IsValidReportTarget(p.TargetType) {
		writeError(w, http.StatusBadRequest, "Invalid report target", CodeInvalidReportTarget)
		return
	}
	if utils.IsBlank(p.Reason) {
		writeError(w, http.StatusBadRequest, "Please say why you are reporting this", CodeReportReasonRequired)
		return
	}

	snapshot, ok := h.snapshotTarget(r.Context(), p, callerID, clubIDs)
	if !ok {
		writeError(w, http.StatusNotFound, "This content does not exist", CodeNotFound)
		return
	}

	report, err := h.social.CreateReport(r.Context(), callerID, p.TargetType, p.TargetID, p.Reason, p.Comment, snapshot)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, report)
}

// snapshotTarget captures the reported content as it reads now, and doubles as
// the existence/visibility check: you can't report what you can't see.
func (h *SocialHandler) snapshotTarget(ctx context.Context, p reportPayload, callerID string, clubIDs []string) (string, bool) {
	switch p.TargetType {
	case models.ReportTargetPost:
		visible, err := h.social.CanSeePost(ctx, p.TargetID, callerID, clubIDs)
		if err != nil || !visible {
			return utils.EMPTY, false
		}
		post, err := h.social.FindPost(ctx, p.TargetID, callerID)
		if err != nil {
			return utils.EMPTY, false
		}
		return post.Content, true

	case models.ReportTargetComment:
		comment, err := h.social.FindComment(ctx, p.TargetID)
		if err != nil {
			return utils.EMPTY, false
		}
		visible, err := h.social.CanSeePost(ctx, comment.PostID, callerID, clubIDs)
		if err != nil || !visible {
			return utils.EMPTY, false
		}
		return comment.Content, true

	case models.ReportTargetMessage:
		// Only a participant may report a message: nobody else can see the
		// thread, so nobody else has anything to report. The snapshot is what
		// makes the report reviewable at all, since a moderator has no way to
		// open a private conversation.
		message, err := h.messages.FindMessage(ctx, p.TargetID)
		if err != nil {
			return utils.EMPTY, false
		}
		memberA, memberB, err := h.messages.ConversationMembers(ctx, message.ConversationID)
		if err != nil || (callerID != memberA && callerID != memberB) {
			return utils.EMPTY, false
		}
		return message.Content, true

	case models.ReportTargetUser:
		user, err := h.users.FindByID(ctx, p.TargetID)
		if err != nil {
			return utils.EMPTY, false
		}
		return user.Name + " " + user.Surname + " (" + user.Email + ")", true
	}
	return utils.EMPTY, false
}

// ListReports is the superadmin's moderation queue.
func (h *SocialHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if utils.IsNotBlank(status) && !models.IsValidReportStatus(status) {
		writeError(w, http.StatusBadRequest, "Invalid status", CodeInvalidStatus)
		return
	}
	page, size := pagination(r)

	reports, total, err := h.social.ListReports(r.Context(), status, page, size)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, models.Page[models.Report]{Results: reports, TotalResults: total})
}

type resolveReportPayload struct {
	Status string `json:"status"`
	// DeleteTarget removes the reported content as part of resolving, so a
	// superadmin doesn't have to go and find it separately.
	DeleteTarget bool `json:"deleteTarget"`
}

// ResolveReport closes a report as handled or dismissed, optionally deleting the
// content it denounced.
func (h *SocialHandler) ResolveReport(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	reportID := chi.URLParam(r, "reportId")

	var p resolveReportPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !models.IsValidReportStatus(p.Status) || p.Status == models.ReportStatusOpen {
		writeError(w, http.StatusBadRequest, "Invalid status", CodeInvalidStatus)
		return
	}

	report, err := h.social.ResolveReport(r.Context(), reportID, p.Status, callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	if p.DeleteTarget {
		// The content may already be gone (deleted by its author, or by an
		// earlier report on the same thing), which isn't an error here.
		var deleteErr error
		switch report.TargetType {
		case models.ReportTargetPost:
			deleteErr = h.social.DeletePost(r.Context(), report.TargetID)
		case models.ReportTargetComment:
			deleteErr = h.social.DeleteComment(r.Context(), report.TargetID)
		}
		if deleteErr != nil && !errors.Is(deleteErr, store.ErrNotFound) {
			writeStoreError(w, deleteErr)
			return
		}
	}
	writeJSON(w, http.StatusOK, report)
}
