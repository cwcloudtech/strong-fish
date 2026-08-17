package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/email"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

type ClubHandler struct {
	clubs        *store.ClubStore
	users        *store.UserStore
	mailer       *email.Sender
	maxImageSize int64
	uiBaseURL    string
}

func NewClubHandler(clubs *store.ClubStore, users *store.UserStore, mailer *email.Sender,
	maxImageSize int64, uiBaseURL string) *ClubHandler {
	return &ClubHandler{clubs: clubs, users: users, mailer: mailer, maxImageSize: maxImageSize, uiBaseURL: uiBaseURL}
}

type clubPayload struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	City        string   `json:"city"`
	Country     string   `json:"country"`
	Picture     *string  `json:"picture"`
	PictureX    *float64 `json:"pictureX"`
	PictureY    *float64 `json:"pictureY"`
}

func (p clubPayload) fields() store.ClubFields {
	return store.ClubFields{
		Name: p.Name, Description: p.Description, City: p.City, Country: p.Country,
		Picture: p.Picture, PictureX: p.PictureX, PictureY: p.PictureY,
	}
}

// List returns the clubs the caller belongs to.
func (h *ClubHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	clubs, err := h.clubs.ListForUser(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, clubs)
}

// ListAll returns every club, for the superadmin's management screen.
func (h *ClubHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	clubs, err := h.clubs.ListAll(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, clubs)
}

// Create opens a club with the calling coach as its owner.
func (h *ClubHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p clubPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Name) {
		writeError(w, http.StatusBadRequest, "Please add a name", CodeNameRequired)
		return
	}
	if p.Picture != nil && utils.ImageSizeExceeds(*p.Picture, h.maxImageSize) {
		writeError(w, http.StatusBadRequest, "Image is too large", CodeImageTooLarge)
		return
	}

	club, err := h.clubs.Create(r.Context(), userID, p.fields())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, club)
}

func (h *ClubHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	club, err := h.clubs.FindByID(r.Context(), chi.URLParam(r, "clubId"), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, club)
}

func (h *ClubHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p clubPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Name) {
		writeError(w, http.StatusBadRequest, "Please add a name", CodeNameRequired)
		return
	}
	if p.Picture != nil && utils.ImageSizeExceeds(*p.Picture, h.maxImageSize) {
		writeError(w, http.StatusBadRequest, "Image is too large", CodeImageTooLarge)
		return
	}

	club, err := h.clubs.Update(r.Context(), chi.URLParam(r, "clubId"), userID, p.fields())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, club)
}

// Delete removes a club. Only its owner (or a superadmin) may: an admin can
// manage the roster but not dissolve the club.
func (h *ClubHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requireOwner(w, r) {
		return
	}
	if err := h.clubs.Delete(r.Context(), chi.URLParam(r, "clubId")); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": chi.URLParam(r, "clubId")})
}

// requireOwner gates the actions only a club's owner may take. A superadmin
// passes too - ClubMembership admits them as an admin, so this re-reads their
// account role rather than trusting the club role alone.
func (h *ClubHandler) requireOwner(w http.ResponseWriter, r *http.Request) bool {
	role, _ := middleware.ClubRoleFromContext(r.Context())
	if role == models.RoleOwner {
		return true
	}

	userID, _ := middleware.UserIDFromContext(r.Context())
	user, err := h.users.FindByID(r.Context(), userID)
	if err == nil && user.Role == models.GlobalRoleSuperadmin {
		return true
	}

	writeError(w, http.StatusForbidden, "Only the club owner can do this", CodeForbidden)
	return false
}

// --- members ---

func (h *ClubHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.clubs.ListMembers(r.Context(), chi.URLParam(r, "clubId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

type addMemberPayload struct {
	Email  string      `json:"email"`
	UserID string      `json:"userId"`
	Role   models.Role `json:"role"`
}

// AddMember enrolls an existing account in the club, by id or by email. The
// account must already exist: strong-fish doesn't create accounts on someone
// else's behalf, so an unknown address is reported rather than invited.
func (h *ClubHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	clubID := chi.URLParam(r, "clubId")
	actorID, _ := middleware.UserIDFromContext(r.Context())

	var p addMemberPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	role := p.Role
	if utils.IsBlank(string(role)) {
		role = models.RoleMember
	}
	if !models.IsValidRole(role) {
		writeError(w, http.StatusBadRequest, "Invalid role", CodeInvalidRole)
		return
	}

	var (
		user models.User
		err  error
	)
	switch {
	case utils.IsNotBlank(p.UserID):
		user, err = h.users.FindByID(r.Context(), p.UserID)
	case utils.IsNotBlank(p.Email):
		user, err = h.users.FindByEmail(r.Context(), p.Email)
	default:
		writeError(w, http.StatusBadRequest, "Please add an email or a user id", CodeAllFieldsRequired)
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "No user is registered with this email", CodeNoUserWithEmail)
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}

	member, err := h.clubs.AddMember(r.Context(), clubID, user.ID, role)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	h.notifyInvitation(r, clubID, actorID, user)
	writeJSON(w, http.StatusCreated, member)
}

// notifyInvitation emails the new member (best-effort - a failure to send must
// not undo the enrollment that already succeeded).
func (h *ClubHandler) notifyInvitation(r *http.Request, clubID, actorID string, member models.User) {
	club, err := h.clubs.FindByID(r.Context(), clubID, actorID)
	if err != nil {
		return
	}
	actor, err := h.users.FindByID(r.Context(), actorID)
	if err != nil {
		return
	}
	h.mailer.SendClubInvitation(r.Context(), member.Email, localeOf(member, r),
		club.Name, actor.Name+" "+actor.Surname, h.uiBaseURL+"/dashboard/clubs/"+clubID)
}

type memberRolePayload struct {
	Role models.Role `json:"role"`
}

// SetMemberRole promotes a member to admin of the club, or demotes them back.
func (h *ClubHandler) SetMemberRole(w http.ResponseWriter, r *http.Request) {
	var p memberRolePayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !models.IsValidRole(p.Role) {
		writeError(w, http.StatusBadRequest, "Invalid role", CodeInvalidRole)
		return
	}

	member, err := h.clubs.SetMemberRole(r.Context(), chi.URLParam(r, "clubId"), chi.URLParam(r, "userId"), p.Role)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

// RemoveMember takes a member out of the club.
func (h *ClubHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if err := h.clubs.RemoveMember(r.Context(), chi.URLParam(r, "clubId"), userID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"userId": userID})
}

// Leave lets a member remove themselves from a club, so they don't have to ask
// an admin to be let out.
func (h *ClubHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	if err := h.clubs.RemoveMember(r.Context(), chi.URLParam(r, "clubId"), userID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"userId": userID})
}

type transferOwnershipPayload struct {
	UserID string `json:"userId"`
}

// TransferOwnership hands the club to another of its members.
func (h *ClubHandler) TransferOwnership(w http.ResponseWriter, r *http.Request) {
	if !h.requireOwner(w, r) {
		return
	}

	var p transferOwnershipPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.UserID) {
		writeError(w, http.StatusBadRequest, "Please add a user id", CodeAllFieldsRequired)
		return
	}

	clubID := chi.URLParam(r, "clubId")
	if err := h.clubs.TransferOwnership(r.Context(), clubID, p.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "This user is not a member of the club", CodeNotAClubMember)
			return
		}
		writeStoreError(w, err)
		return
	}

	callerID, _ := middleware.UserIDFromContext(r.Context())
	club, err := h.clubs.FindByID(r.Context(), clubID, callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, club)
}
