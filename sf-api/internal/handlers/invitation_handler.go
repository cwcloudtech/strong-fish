package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/email"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// InvitationHandler is a coach asking somebody to join their club.
//
// It exists alongside "add a member" rather than replacing it: adding puts
// somebody in a club without asking, which is right for a coach entering their
// own athletes, and wrong for reaching out to a stranger. An invitation is the
// second case - it waits, and the person decides.
type InvitationHandler struct {
	invitations *store.InvitationStore
	clubs       *store.ClubStore
	users       *store.UserStore
	mailer      *email.Sender
	uiBaseURL   string
}

func NewInvitationHandler(invitations *store.InvitationStore, clubs *store.ClubStore,
	users *store.UserStore, mailer *email.Sender, uiBaseURL string) *InvitationHandler {
	return &InvitationHandler{
		invitations: invitations, clubs: clubs, users: users,
		mailer: mailer, uiBaseURL: uiBaseURL,
	}
}

type invitePayload struct {
	Email   string      `json:"email"`
	Role    models.Role `json:"role"`
	Message string      `json:"message"`
}

// Create invites one address to the club. The route is already behind
// RequireClubManager, so reaching here means the caller may do this.
func (h *InvitationHandler) Create(w http.ResponseWriter, r *http.Request) {
	inviterID, _ := middleware.UserIDFromContext(r.Context())
	clubID := chi.URLParam(r, "clubId")

	var p invitePayload
	if !decodeJSON(w, r, &p) {
		return
	}
	address := strings.ToLower(strings.TrimSpace(p.Email))
	if !utils.IsValidEmail(address) {
		writeError(w, http.StatusBadRequest, "Please add a valid email", CodeInvalidEmail)
		return
	}
	if !models.IsValidRole(p.Role) {
		p.Role = models.RoleMember
	}

	// Somebody already in the club has nothing to accept. This is checked here
	// rather than left to the accept step so the coach is told now, while they
	// are looking at the member list.
	if existing, err := h.users.FindByEmail(r.Context(), address); err == nil {
		if _, err := h.clubs.FindMembership(r.Context(), clubID, existing.ID); err == nil {
			writeError(w, http.StatusBadRequest, "This user is already a member of the club", CodeAlreadyMember)
			return
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		writeStoreError(w, err)
		return
	}

	invitation, err := h.invitations.Invite(r.Context(), clubID, inviterID, address, p.Role, p.Message)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	// Best-effort, like every other outgoing mail here: an invitation that
	// couldn't be emailed is still waiting in the app, so a mail failure must
	// not fail the request.
	h.mailer.SendClubInvite(r.Context(), address, h.localeFor(r, address),
		invitation.ClubName, invitation.InviterName, invitation.Message, h.uiBaseURL+"/dashboard/invitations")

	writeJSON(w, http.StatusCreated, invitation)
}

// localeFor picks the language to write to an address in: the account's own
// choice when there is an account, otherwise the inviter's browser.
func (h *InvitationHandler) localeFor(r *http.Request, address string) string {
	if user, err := h.users.FindByEmail(r.Context(), address); err == nil {
		return localeOf(user, r)
	}
	if len(r.Header.Get("Accept-Language")) >= 2 {
		return r.Header.Get("Accept-Language")[:2]
	}
	return "en"
}

// ListForClub shows a coach who they have invited and who has not answered.
func (h *InvitationHandler) ListForClub(w http.ResponseWriter, r *http.Request) {
	invitations, err := h.invitations.ListForClub(r.Context(), chi.URLParam(r, "clubId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invitations)
}

// Withdraw cancels an invitation that hasn't been answered.
func (h *InvitationHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	invitationID := chi.URLParam(r, "invitationId")

	invitation, err := h.invitations.FindByID(r.Context(), invitationID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// The route carries the club, so an id from another club must not resolve
	// through a manager's rights over this one.
	if invitation.ClubID != chi.URLParam(r, "clubId") {
		writeError(w, http.StatusNotFound, "Invitation not found", CodeInvitationNotFound)
		return
	}
	if err := h.invitations.Delete(r.Context(), invitationID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": invitationID})
}

// --- the invitee's side ---

// Mine lists the invitations waiting for the connected account's address.
func (h *InvitationHandler) Mine(w http.ResponseWriter, r *http.Request) {
	user, ok := h.caller(w, r)
	if !ok {
		return
	}

	invitations, err := h.invitations.ListPendingForEmail(r.Context(), user.Email)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invitations)
}

func (h *InvitationHandler) caller(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return models.User{}, false
	}
	return user, true
}

// resolveMine loads an invitation and checks it is actually addressed to the
// caller. Matching on the address is the whole authorization here: an id from
// somebody else's invitation resolves to a 404, not to a membership.
func (h *InvitationHandler) resolveMine(w http.ResponseWriter, r *http.Request) (models.Invitation, models.User, bool) {
	user, ok := h.caller(w, r)
	if !ok {
		return models.Invitation{}, models.User{}, false
	}

	invitation, err := h.invitations.FindByID(r.Context(), chi.URLParam(r, "invitationId"))
	if err != nil || !strings.EqualFold(invitation.Email, user.Email) ||
		invitation.Status != models.InvitationStatusPending {
		writeError(w, http.StatusNotFound, "Invitation not found", CodeInvitationNotFound)
		return models.Invitation{}, models.User{}, false
	}
	return invitation, user, true
}

func (h *InvitationHandler) Accept(w http.ResponseWriter, r *http.Request) {
	invitation, user, ok := h.resolveMine(w, r)
	if !ok {
		return
	}

	if err := h.invitations.Accept(r.Context(), invitation, user.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Invitation not found", CodeInvitationNotFound)
			return
		}
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"clubId": invitation.ClubID, "status": models.InvitationStatusAccepted})
}

func (h *InvitationHandler) Decline(w http.ResponseWriter, r *http.Request) {
	invitation, _, ok := h.resolveMine(w, r)
	if !ok {
		return
	}

	if _, err := h.invitations.SetStatus(r.Context(), invitation.ID, models.InvitationStatusDeclined); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": invitation.ID, "status": models.InvitationStatusDeclined})
}
