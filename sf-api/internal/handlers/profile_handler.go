package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// ProfileHandler serves the public profile of a member or coach: their avatar,
// their bests on the three competition lifts, their clubs, and their public
// posts. It's readable without logging in when the owner opted in, so a profile
// link can be shared.
type ProfileHandler struct {
	users  *store.UserStore
	social *store.SocialStore
	clubs  *store.ClubStore
	oneRMs *store.OneRMStore
}

func NewProfileHandler(users *store.UserStore, social *store.SocialStore, clubs *store.ClubStore, oneRMs *store.OneRMStore) *ProfileHandler {
	return &ProfileHandler{users: users, social: social, clubs: clubs, oneRMs: oneRMs}
}

// resolveTarget loads the profile being addressed and decides whether the caller
// may read it: a private profile is visible to its owner and to a superadmin,
// and to nobody else.
func (h *ProfileHandler) resolveTarget(w http.ResponseWriter, r *http.Request) (models.User, string, bool) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	target, err := h.users.FindByIDOrHandle(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		writeStoreError(w, err)
		return models.User{}, callerID, false
	}
	if target.PublicProfile || target.ID == callerID {
		return target, callerID, true
	}

	if utils.IsNotBlank(callerID) {
		if caller, err := h.users.FindByID(r.Context(), callerID); err == nil && caller.Role == models.GlobalRoleSuperadmin {
			return target, callerID, true
		}
	}

	// A private profile reads as absent rather than forbidden, so its existence
	// isn't disclosed.
	writeError(w, http.StatusNotFound, "Profile not found", CodeNotFound)
	return models.User{}, callerID, false
}

// Get returns one public profile.
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	target, callerID, ok := h.resolveTarget(w, r)
	if !ok {
		return
	}

	bests, total, err := h.oneRMs.ListBests(r.Context(), target.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	clubs, err := h.clubs.ListProfileClubs(r.Context(), target.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	followers, following, followed, err := h.social.FollowCounts(r.Context(), target.ID, callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, models.PublicProfile{
		ID: target.ID, Handle: target.Handle, Name: target.Name, Surname: target.Surname,
		Role: target.Role, Bio: target.Bio, Picture: target.Picture,
		PictureX: target.PictureX, PictureY: target.PictureY, Bodyweight: target.Bodyweight,
		Bests: bests, Total: total, Followers: followers, Following: following,
		Followed: followed, Clubs: clubs, CreatedAt: target.CreatedAt,
	})
}

// Posts returns a profile's posts as visible to the caller - only the public
// ones for a logged-out visitor, which is what makes a shared link work.
func (h *ProfileHandler) Posts(w http.ResponseWriter, r *http.Request) {
	target, callerID, ok := h.resolveTarget(w, r)
	if !ok {
		return
	}

	clubIDs := []string{}
	superadmin := false
	if utils.IsNotBlank(callerID) {
		if ids, err := h.clubs.ListClubIDsForUser(r.Context(), callerID); err == nil {
			clubIDs = ids
		}
		if caller, err := h.users.FindByID(r.Context(), callerID); err == nil {
			superadmin = caller.Role == models.GlobalRoleSuperadmin
		}
	}

	page, size := pagination(r)
	posts, total, err := h.social.ListProfilePosts(r.Context(), target.ID, callerID, clubIDs, page, size)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	for i := range posts {
		posts[i] = decoratePost(posts[i], callerID, superadmin)
	}
	writeJSON(w, http.StatusOK, models.Page[models.Post]{Results: posts, TotalResults: total})
}
