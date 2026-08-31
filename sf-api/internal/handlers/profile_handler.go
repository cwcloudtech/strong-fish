package handlers

import (
	"context"
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
	// strength scores the profile's badges. Optional: a router built without
	// one simply serves profiles that wear none.
	strength *StrengthHandler
}

func NewProfileHandler(users *store.UserStore, social *store.SocialStore, clubs *store.ClubStore, oneRMs *store.OneRMStore) *ProfileHandler {
	return &ProfileHandler{users: users, social: social, clubs: clubs, oneRMs: oneRMs}
}

// resolveTarget loads the profile being addressed and decides whether the
// caller may read it (see models.CanSeeProfile).
func (h *ProfileHandler) resolveTarget(w http.ResponseWriter, r *http.Request) (models.User, string, bool) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	target, err := h.users.FindByIDOrHandle(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		writeStoreError(w, err)
		return models.User{}, callerID, false
	}

	relation, err := h.RelationTo(r.Context(), target.ID, callerID)
	if err != nil {
		writeStoreError(w, err)
		return models.User{}, callerID, false
	}
	if models.CanSeeProfile(target.ProfileVisibility, relation) {
		return target, callerID, true
	}

	// A profile the caller may not see reads as absent rather than forbidden,
	// so its existence isn't disclosed to somebody guessing handles.
	writeError(w, http.StatusNotFound, "Profile not found", CodeNotFound)
	return models.User{}, callerID, false
}

// RelationTo resolves what a caller is to one profile's owner. It is exported
// because the search returns many profiles at once and needs the same answer
// per row, and because getting this rule right in two places is exactly how it
// ends up wrong in one of them.
func (h *ProfileHandler) RelationTo(ctx context.Context, targetID, callerID string) (models.ViewerRelation, error) {
	if utils.IsBlank(callerID) {
		return models.ViewerRelation{}, nil
	}
	relation, err := h.clubs.RelationTo(ctx, targetID, callerID)
	if err != nil {
		return models.ViewerRelation{}, err
	}
	if caller, err := h.users.FindByID(ctx, callerID); err == nil {
		relation.Superadmin = caller.Role == models.GlobalRoleSuperadmin
	}
	return relation, nil
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

	// The profile is projected through DisplayName like every other surface:
	// an anonymized member is their username here too.
	name, surname := target.DisplayName()
	writeJSON(w, http.StatusOK, models.PublicProfile{
		ID: target.ID, Handle: target.Handle, Name: name, Surname: surname,
		Anonymous: target.Anonymous,
		Role:      target.Role, Bio: target.Bio, Picture: target.Picture,
		PictureX: target.PictureX, PictureY: target.PictureY, Bodyweight: target.Bodyweight,
		Specialty: models.NormalizeSpecialty(target.Specialty),
		Socials:   models.NormalizeSocials(target.Socials),
		Birthdate: target.Birthdate,
		Bests:     bests, Total: total, Followers: followers, Following: following,
		Followed: followed, Clubs: clubs, CreatedAt: target.CreatedAt,
		Strength: h.strengthFor(r, target.ID),
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

// WithStrength gives the profile handler the scorer that computes its badges.
// Set after construction because the two handlers need each other: the profile
// wears the badges, and the calculator resolves the member the same way.
func (h *ProfileHandler) WithStrength(strength *StrengthHandler) *ProfileHandler {
	h.strength = strength
	return h
}

// strengthFor is the member's tier and badges, or nil when this deployment was
// built without a scorer or the member has nothing to score.
func (h *ProfileHandler) strengthFor(r *http.Request, userID string) any {
	if h.strength == nil {
		return nil
	}
	if result := h.strength.ForUser(r, userID); result != nil {
		return result
	}
	return nil
}
