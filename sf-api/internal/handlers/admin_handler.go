package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"strong-fish-api/internal/email"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// AdminHandler is the superadmin's account management: promoting members to
// coach, activating accounts, banning, and clearing a lost second factor.
type AdminHandler struct {
	users          *store.UserStore
	webauthnCreds  *store.WebAuthnCredentialStore
	social         *store.SocialStore
	mailer         *email.Sender
	activationMode string
	uiBaseURL      string
}

func NewAdminHandler(users *store.UserStore, webauthnCreds *store.WebAuthnCredentialStore,
	social *store.SocialStore, mailer *email.Sender, activationMode, uiBaseURL string) *AdminHandler {
	return &AdminHandler{
		users: users, webauthnCreds: webauthnCreds, social: social, mailer: mailer,
		activationMode: activationMode, uiBaseURL: uiBaseURL,
	}
}

// adminUser is one account as the management screen shows it, with the MFA flag
// the plain profile response hides.
type adminUser struct {
	models.UserMeResponse
	MFAEnabled bool `json:"mfaEnabled"`
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}

	results := make([]adminUser, len(users))
	for i, user := range users {
		results[i] = adminUser{UserMeResponse: meResponse(user, h.activationMode), MFAEnabled: user.MFAEnabled}
	}
	writeJSON(w, http.StatusOK, results)
}

type adminUserPayload struct {
	Email    string            `json:"email"`
	Name     string            `json:"name"`
	Surname  string            `json:"surname"`
	Role     models.GlobalRole `json:"role"`
	Password string            `json:"password"`
}

// UpdateUser edits any account - including promoting a member to coach, which is
// how someone gains the right to create clubs and upload programs.
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	targetID := chi.URLParam(r, "userId")

	var p adminUserPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Email) || !utils.IsValidEmail(p.Email) {
		writeError(w, http.StatusBadRequest, "Please add a valid email", CodeInvalidEmail)
		return
	}
	if !models.IsValidGlobalRole(p.Role) {
		writeError(w, http.StatusBadRequest, "Invalid role", CodeInvalidRole)
		return
	}
	// Changing your own role is refused rather than allowed-and-regretted: a
	// superadmin demoting themselves could leave the instance with none.
	if targetID == callerID {
		if existing, err := h.users.FindByID(r.Context(), targetID); err == nil && existing.Role != p.Role {
			writeError(w, http.StatusBadRequest, "You cannot change your own role", CodeCantEditOwnRole)
			return
		}
	}

	var passwordHash *string
	if utils.IsNotBlank(p.Password) {
		if ok, code := utils.IsPasswordValid(p.Password); !ok {
			writeInvalidPassword(w, code)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
			return
		}
		hashed := string(hash)
		passwordHash = &hashed
	}

	user, err := h.users.AdminUpdate(r.Context(), targetID, store.AdminUserFields{
		Email: p.Email, Name: p.Name, Surname: p.Surname, Role: string(p.Role), PasswordHash: passwordHash,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, adminUser{UserMeResponse: meResponse(user, h.activationMode), MFAEnabled: user.MFAEnabled})
}

// DeleteUser removes an account and everything it owns.
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	targetID := chi.URLParam(r, "userId")

	if targetID == callerID {
		writeError(w, http.StatusBadRequest, "You cannot delete your own account", CodeCantDeleteOwnAccount)
		return
	}
	if err := h.users.Delete(r.Context(), targetID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": targetID})
}

// ClearMFA strips every second factor from an account - the recovery path for a
// user who lost both their phone and their security key.
func (h *AdminHandler) ClearMFA(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "userId")

	if err := h.webauthnCreds.DeleteAllForUser(r.Context(), targetID); err != nil {
		writeStoreError(w, err)
		return
	}
	user, err := h.users.ClearMFA(r.Context(), targetID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, adminUser{UserMeResponse: meResponse(user, h.activationMode), MFAEnabled: user.MFAEnabled})
}

// Stats backs the superadmin's dashboard badges.
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.Count(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	openReports, err := h.social.CountOpenReports(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	coachRequests, err := h.users.CountCoachApplicants(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"users": users, "openReports": openReports, "coachRequests": coachRequests,
	})
}

// --- coach confirmation ---

// ListCoachRequests is the queue of accounts that said "I'm a coach" at signup
// and are waiting to be believed.
func (h *AdminHandler) ListCoachRequests(w http.ResponseWriter, r *http.Request) {
	applicants, err := h.users.ListCoachApplicants(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}

	results := make([]models.CoachApplicant, len(applicants))
	for i, user := range applicants {
		results[i] = models.CoachApplicant{
			UserSummary: summarize(user), Request: user.CoachRequest, CreatedAt: user.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, results)
}

type coachDecisionPayload struct {
	Status string `json:"status"`
	Motive string `json:"motive"`
}

// DecideCoachRequest confirms or turns down one claim.
//
// A rejection must carry a motive: it is emailed to the applicant, and "no"
// with no reason tells them nothing about whether to ask again.
func (h *AdminHandler) DecideCoachRequest(w http.ResponseWriter, r *http.Request) {
	deciderID, _ := middleware.UserIDFromContext(r.Context())
	userID := chi.URLParam(r, "userId")

	var p coachDecisionPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !models.IsValidCoachRequestStatus(p.Status) {
		writeError(w, http.StatusBadRequest, "Invalid decision", CodeInvalidStatus)
		return
	}
	if p.Status == models.CoachRequestRejected && utils.IsBlank(p.Motive) {
		writeError(w, http.StatusBadRequest, "Please say why this request is rejected", CodeRejectMotiveRequired)
		return
	}

	applicant, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !applicant.CoachRequest.Pending() {
		writeError(w, http.StatusBadRequest, "This account has no pending coach request", CodeNoCoachRequest)
		return
	}

	user, err := h.users.DecideCoachRequest(r.Context(), userID, p.Status, p.Motive, deciderID, time.Now().UTC())
	if err != nil {
		writeStoreError(w, err)
		return
	}

	locale := localeOf(user, r)
	if p.Status == models.CoachRequestApproved {
		h.mailer.SendCoachApproved(r.Context(), user.Email, locale, h.uiBaseURL)
	} else {
		h.mailer.SendCoachRejected(r.Context(), user.Email, locale, p.Motive)
	}

	writeJSON(w, http.StatusOK, adminUser{
		UserMeResponse: meResponse(user, h.activationMode), MFAEnabled: user.MFAEnabled,
	})
}
