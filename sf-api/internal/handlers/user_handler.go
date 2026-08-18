package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"strong-fish-api/internal/authtoken"
	"strong-fish-api/internal/email"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// mfaLoginTokenTTL is how long the challenge token returned by Login (when MFA
// is enabled) stays valid for finishing login via /v1/users/login/mfa/*.
const mfaLoginTokenTTL = 5 * time.Minute

type UserHandler struct {
	users                *store.UserStore
	webauthnCreds        *store.WebAuthnCredentialStore
	jwtSecret            string
	maxImageSize         int64
	activationMode       string
	mailer               *email.Sender
	apiBaseURL           string
	uiBaseURL            string
	confirmationTokenTTL time.Duration
}

func NewUserHandler(users *store.UserStore, webauthnCreds *store.WebAuthnCredentialStore, jwtSecret string,
	maxImageSize int64, activationMode string, mailer *email.Sender, apiBaseURL, uiBaseURL string,
	confirmationTokenTTL time.Duration) *UserHandler {
	return &UserHandler{
		users: users, webauthnCreds: webauthnCreds, jwtSecret: jwtSecret, maxImageSize: maxImageSize,
		activationMode: activationMode, mailer: mailer, apiBaseURL: apiBaseURL, uiBaseURL: uiBaseURL,
		confirmationTokenTTL: confirmationTokenTTL,
	}
}

type registerPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Locale   string `json:"locale"`
	// Coach is the "I'm a coach" box on the signup form. It records a claim,
	// never a grant: coaching means writing other people's training, so the
	// role waits on a superadmin (see models.CoachRequest).
	Coach bool `json:"coach"`
}

// Register opens an account. Anyone can subscribe: the very first account ever
// created becomes the superadmin, and every later one starts disabled until it's
// confirmed - by the link emailed to it, or by an administrator, depending on
// the activation mode.
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var p registerPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Email) || utils.IsBlank(p.Password) || utils.IsBlank(p.Name) || utils.IsBlank(p.Surname) {
		writeError(w, http.StatusBadRequest, "Please add all fields", CodeAllFieldsRequired)
		return
	}
	if !utils.IsValidEmail(p.Email) {
		writeError(w, http.StatusBadRequest, "Please add a valid email", CodeInvalidEmail)
		return
	}
	if ok, code := utils.IsPasswordValid(p.Password); !ok {
		writeInvalidPassword(w, code)
		return
	}

	if _, err := h.users.FindByEmail(r.Context(), p.Email); err == nil {
		writeError(w, http.StatusBadRequest, "A user with this email already exists", CodeDuplicateEmail)
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeStoreError(w, err)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}

	user, err := h.users.Create(r.Context(), p.Email, string(hash), p.Name, p.Surname, p.Locale)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	if p.Coach {
		// The very first account is the superadmin and already outranks a
		// coach, so it has nothing to apply for.
		if user.Role != models.GlobalRoleSuperadmin {
			if updated, err := h.users.RequestCoach(r.Context(), user.ID, time.Now().UTC()); err == nil {
				user = updated
				h.notifyCoachRequest(r, user)
			} else {
				// The account exists and works; it just isn't queued for
				// review. Failing the registration over it would be worse.
				slog.Error("failed to record coach request", "userId", user.ID, "error", err)
			}
		}
	}

	if user.Role == models.GlobalRoleDisabled && h.activationMode == models.ActivationModeEmail {
		h.sendConfirmationEmail(r.Context(), user, localeOf(user, r))
	}

	h.respondSession(w, user, http.StatusCreated)
}

// notifyCoachRequest tells every superadmin somebody is waiting. Best-effort,
// like every other outgoing mail here: the request is in the queue either way,
// and the queue - not the email - is what they act on.
func (h *UserHandler) notifyCoachRequest(r *http.Request, applicant models.User) {
	emails, err := h.users.ListSuperadminEmails(r.Context())
	if err != nil {
		slog.Error("failed to list superadmins for a coach request", "error", err)
		return
	}
	name := strings.TrimSpace(applicant.Name + " " + applicant.Surname)
	for _, address := range emails {
		h.mailer.SendCoachRequest(r.Context(), address, localeOf(applicant, r),
			name, applicant.Email, h.uiBaseURL+"/dashboard/admin")
	}
}

// sendConfirmationEmail mints a purpose-scoped confirmation token and emails the
// link (best-effort - see package email).
func (h *UserHandler) sendConfirmationEmail(ctx context.Context, user models.User, locale string) {
	token, err := authtoken.GeneratePurpose(h.jwtSecret, user.ID, authtoken.PurposeConfirmAccount, h.confirmationTokenTTL)
	if err != nil {
		slog.Error("failed to generate confirmation token", "error", err)
		return
	}
	h.mailer.SendConfirmation(ctx, user.Email, locale, h.apiBaseURL+"/v1/user/confirmation?token="+url.QueryEscape(token))
}

// respondSession issues a real session token for a fully authenticated user.
func (h *UserHandler) respondSession(w http.ResponseWriter, user models.User, status int) {
	token, err := authtoken.Generate(h.jwtSecret, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	writeJSON(w, status, models.UserResponse{
		ID: user.ID, Email: user.Email, Name: user.Name, Surname: user.Surname,
		Handle: user.Handle, Role: user.Role, Token: token, Picture: user.Picture,
		PictureX: user.PictureX, PictureY: user.PictureY,
		I18nCode: models.I18nCodeForRole(user.Role, h.activationMode),
	})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if !decodeJSON(w, r, &creds) {
		return
	}

	user, err := h.users.FindByEmail(r.Context(), creds.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials", CodeInvalidCredentials)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(creds.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials", CodeInvalidCredentials)
		return
	}

	if user.MFAEnabled {
		h.respondMFAChallenge(w, r, user)
		return
	}
	h.respondSession(w, user, http.StatusOK)
}

// respondMFAChallenge replaces the normal login response when the account has
// MFA enabled: instead of a usable session token, it mints a short-lived
// PurposeMFALogin token the client exchanges for one once the second factor is
// verified.
func (h *UserHandler) respondMFAChallenge(w http.ResponseWriter, r *http.Request, user models.User) {
	challengeToken, err := authtoken.GeneratePurpose(h.jwtSecret, user.ID, authtoken.PurposeMFALogin, mfaLoginTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	webauthnCount, err := h.webauthnCreds.CountByUser(r.Context(), user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, models.MFAChallengeResponse{
		MFARequired:    true,
		ChallengeToken: challengeToken,
		HasTOTP:        utils.IsNotBlank(user.MFATOTPSecret),
		HasWebAuthn:    webauthnCount > 0,
	})
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found", CodeUserNotFound)
		return
	}
	writeJSON(w, http.StatusOK, meResponse(user, h.activationMode))
}

type updateProfilePayload struct {
	Name              string  `json:"name"`
	Surname           string  `json:"surname"`
	Handle            string  `json:"handle"`
	Bio               string  `json:"bio"`
	Locale            string  `json:"locale"`
	ProfileVisibility string  `json:"profileVisibility"`
	Birthdate         string  `json:"birthdate"`
	Bodyweight        float64 `json:"bodyweight"`
	Password          string  `json:"password"`
	ConfirmPassword   string  `json:"confirmPassword"`
}

// UpdateProfile lets the connected user edit their own profile and, optionally,
// change their password (left untouched when the field is empty).
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p updateProfilePayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Name) || utils.IsBlank(p.Surname) {
		writeError(w, http.StatusBadRequest, "Please add name and surname fields", CodeAllFieldsRequired)
		return
	}

	// The handle is normalized rather than validated character by character, so
	// a user typing "Jean Dupont" gets "jean-dupont" instead of an error.
	handle := utils.Slugify(p.Handle)
	if utils.IsNotBlank(p.Handle) && utils.IsBlank(handle) {
		writeError(w, http.StatusBadRequest, "This profile name cannot be used", CodeInvalidHandle)
		return
	}
	if utils.IsNotBlank(handle) {
		taken, err := h.users.IsHandleTaken(r.Context(), handle, userID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if taken {
			writeError(w, http.StatusBadRequest, "This profile name is already taken", CodeDuplicateHandle)
			return
		}
	}

	var passwordHash *string
	if utils.IsNotBlank(p.Password) {
		if p.Password != p.ConfirmPassword {
			writeError(w, http.StatusBadRequest, "Passwords do not match", CodePasswordsMismatch)
			return
		}
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

	// A birthdate is optional and, once given, has to be a real date - it drives
	// a calendar entry, and "1990-13-45" would render as one.
	birthdate := strings.TrimSpace(p.Birthdate)
	if utils.IsNotBlank(birthdate) {
		if _, err := time.Parse("2006-01-02", birthdate); err != nil {
			writeError(w, http.StatusBadRequest, "Please enter a valid birthdate", CodeInvalidBirthdate)
			return
		}
	}

	user, err := h.users.UpdateProfile(r.Context(), userID, store.ProfileFields{
		Name: p.Name, Surname: p.Surname, Handle: handle, Bio: p.Bio, Locale: p.Locale,
		ProfileVisibility: p.ProfileVisibility, Birthdate: birthdate,
		Bodyweight: p.Bodyweight, PasswordHash: passwordHash,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meResponse(user, h.activationMode))
}

type updatePicturePayload struct {
	Picture string  `json:"picture"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

// UpdatePicture sets the connected user's avatar (base64, stored uncropped)
// along with the x/y position used to display it.
func (h *UserHandler) UpdatePicture(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p updatePicturePayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.ImageSizeExceeds(p.Picture, h.maxImageSize) {
		writeError(w, http.StatusBadRequest, "Image is too large", CodeImageTooLarge)
		return
	}

	user, err := h.users.UpdatePicture(r.Context(), userID, p.Picture, p.X, p.Y)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meResponse(user, h.activationMode))
}

// Search powers the autocomplete used when adding a member to a club.
func (h *UserHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if len(query) < 2 {
		writeJSON(w, http.StatusOK, []models.UserSummary{})
		return
	}

	users, err := h.users.SearchByEmail(r.Context(), query, 10)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	results := make([]models.UserSummary, len(users))
	for i, u := range users {
		results[i] = summarize(u)
	}
	writeJSON(w, http.StatusOK, results)
}

// Confirm is the endpoint a user's emailed confirmation link points to. It's
// clicked straight from the email, so it redirects to the frontend rather than
// returning JSON.
func (h *UserHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	userID, err := authtoken.ParsePurpose(h.jwtSecret, r.URL.Query().Get("token"), authtoken.PurposeConfirmAccount)
	if err != nil {
		h.redirectToLogin(w, r, "invalid")
		return
	}

	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		h.redirectToLogin(w, r, "invalid")
		return
	}
	// A banned account can never be confirmed this way, even with a valid,
	// unexpired token - being banned overrides a pending confirmation.
	if user.Role == models.GlobalRoleBan {
		h.redirectToLogin(w, r, "banned")
		return
	}

	if user.Role == models.GlobalRoleDisabled {
		if _, err := h.users.Confirm(r.Context(), userID); err != nil {
			h.redirectToLogin(w, r, "invalid")
			return
		}
	}
	h.redirectToLogin(w, r, utils.EMPTY)
}

func (h *UserHandler) redirectToLogin(w http.ResponseWriter, r *http.Request, confirmError string) {
	target := h.uiBaseURL + "/login?confirmed=1"
	if utils.IsNotBlank(confirmError) {
		target = h.uiBaseURL + "/login?confirmed=0&reason=" + url.QueryEscape(confirmError)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

type forgotPasswordPayload struct {
	Email string `json:"email"`
}

// ForgotPassword emails a password-renewal link when the address matches an
// account. It always responds 200 with the same generic message regardless of
// whether the account exists (or is banned, which silently skips sending), so
// the endpoint can't be used to test which emails are registered.
func (h *UserHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var p forgotPasswordPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Email) {
		writeError(w, http.StatusBadRequest, "Please add an email", CodeInvalidEmail)
		return
	}

	user, err := h.users.FindByEmail(r.Context(), p.Email)
	if err == nil && user.Role != models.GlobalRoleBan {
		token, err := authtoken.GeneratePurpose(h.jwtSecret, user.ID, authtoken.PurposeResetPassword, h.confirmationTokenTTL)
		if err != nil {
			slog.Error("failed to generate password reset token", "error", err)
		} else {
			h.mailer.SendPasswordReset(r.Context(), user.Email, localeOf(user, r),
				h.uiBaseURL+"/reset-password?token="+url.QueryEscape(token))
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "If this email is registered, a reset link has been sent."})
}

type resetPasswordPayload struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// ResetPassword sets a new password from a token minted by ForgotPassword.
func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var p resetPasswordPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Token) || utils.IsBlank(p.Password) {
		writeError(w, http.StatusBadRequest, "Please add a token and a password", CodeInvalidRequestBody)
		return
	}
	if p.Password != p.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "Passwords do not match", CodePasswordsMismatch)
		return
	}
	if ok, code := utils.IsPasswordValid(p.Password); !ok {
		writeInvalidPassword(w, code)
		return
	}

	userID, err := authtoken.ParsePurpose(h.jwtSecret, p.Token, authtoken.PurposeResetPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, "This reset link is invalid or has expired", CodeInvalidToken)
		return
	}

	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "This reset link is invalid or has expired", CodeInvalidToken)
		return
	}
	// A banned user can never renew their password, even with a valid,
	// unexpired reset token.
	if user.Role == models.GlobalRoleBan {
		writeError(w, http.StatusForbidden, "Your account has been banned by an administrator.", CodeInvalidToken)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	if _, err := h.users.UpdatePassword(r.Context(), userID, string(hash)); err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password updated."})
}
