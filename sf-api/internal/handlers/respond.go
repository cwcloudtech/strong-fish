package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

type errorBody struct {
	Message  string `json:"message"`
	I18nCode string `json:"i18n_code,omitempty"`
}

// I18n codes for API errors. The frontend looks these up in its own translation
// dictionaries, falling back to the English Message when a code is absent or
// unrecognized (an older client, or one that doesn't know the code).
const (
	CodeInternal             = "errors.internal"
	CodeNotFound             = "errors.notFound"
	CodeInvalidRequestBody   = "errors.invalidRequestBody"
	CodeAllFieldsRequired    = "errors.allFieldsRequired"
	CodeInvalidCredentials   = "errors.invalidCredentials"
	CodeDuplicateEmail       = "errors.duplicateEmail"
	CodeDuplicateHandle      = "errors.duplicateHandle"
	CodeInvalidEmail         = "errors.invalidEmail"
	CodeInvalidHandle        = "errors.invalidHandle"
	CodeInvalidUsername      = "errors.invalidUsername"
	CodeDuplicateUsername    = "errors.duplicateUsername"
	CodeUsernameRequired     = "errors.usernameRequired"
	CodeUserNotFound         = "errors.userNotFound"
	CodeNoUserWithEmail      = "errors.noUserWithEmail"
	CodePasswordsMismatch    = "errors.passwordsMismatch"
	CodePasswordTooShort     = "errors.passwordTooShort"
	CodePasswordNoUpper      = "errors.passwordNoUpper"
	CodePasswordNoLower      = "errors.passwordNoLower"
	CodePasswordNoSymbol     = "errors.passwordNoSymbol"
	CodeInvalidToken         = "errors.invalidToken"
	CodeInvalidMFACode       = "errors.invalidMfaCode"
	CodeImageTooLarge        = "errors.imageTooLarge"
	CodeInvalidRole          = "errors.invalidRole"
	CodeCantEditOwnRole      = "errors.cantEditOwnRole"
	CodeCantDeleteOwnAccount = "errors.cantDeleteOwnAccount"
	CodeNameRequired         = "errors.nameRequired"
	CodeCantRemoveOwner      = "errors.cantRemoveOwner"
	CodeAlreadyMember        = "errors.alreadyMember"
	CodeNotAClubMember       = "errors.notAClubMember"
	CodeDuplicateExercise    = "errors.duplicateExercise"
	CodeInvalidCategory      = "errors.invalidCategory"
	CodeInvalidOneRM         = "errors.invalidOneRm"
	CodeInvalidSet           = "errors.invalidSet"
	CodeInvalidLoadMode      = "errors.invalidLoadMode"
	CodeNoAssignment         = "errors.noAssignment"
	CodeInvalidStatus        = "errors.invalidStatus"
	CodeUploadTooLarge       = "errors.uploadTooLarge"
	CodeInvalidSpreadsheet   = "errors.invalidSpreadsheet"
	CodeNoProgramInFile      = "errors.noProgramInFile"
	CodeEmptyPost            = "errors.emptyPost"
	CodeInvalidVisibility    = "errors.invalidVisibility"
	CodeClubRequired         = "errors.clubRequired"
	CodeEmptyComment         = "errors.emptyComment"
	CodeCannotLikeOwnPost    = "errors.cannotLikeOwnPost"
	CodeInvalidReportTarget  = "errors.invalidReportTarget"
	CodeReportReasonRequired = "errors.reportReasonRequired"
	CodeForbidden            = "errors.forbidden"
	// API keys, and the CLI/mobile config built from one.
	CodeApiKeyDescription   = "errors.apiKeyDescription"
	CodeInvalidExpiration   = "errors.invalidExpiration"
	CodeConfigTokenRequired = "errors.configTokenRequired"
	// Video uploads, and the member's own storage bucket they go to.
	CodeStorageNotConfigured  = "errors.storageNotConfigured"
	CodeStorageUploadFailed   = "errors.storageUploadFailed"
	CodeVideoTooLarge         = "errors.videoTooLarge"
	CodeUnsupportedVideo      = "errors.unsupportedVideo"
	CodeInvalidStorageType    = "errors.invalidStorageType"
	CodeInvalidServiceAccount = "errors.invalidServiceAccount"
	// The events calendar.
	CodeEventTitleRequired = "errors.eventTitleRequired"
	CodeInvalidEventDate   = "errors.invalidEventDate"
	// Profiles, search, invitations and coach requests.
	CodeInvalidBirthdate         = "errors.invalidBirthdate"
	CodeInvalidProfileVisibility = "errors.invalidProfileVisibility"
	CodeInvitationNotFound       = "errors.invitationNotFound"
	CodeAlreadyInvited           = "errors.alreadyInvited"
	CodeNoCoachRequest           = "errors.noCoachRequest"
	CodeRejectMotiveRequired     = "errors.rejectMotiveRequired"
	// Private messages and the block list.
	CodeCannotMessage     = "errors.cannotMessage"
	CodeCannotMessageSelf = "errors.cannotMessageSelf"
	CodeCannotBlockSelf   = "errors.cannotBlockSelf"
	CodeEmptyMessage      = "errors.emptyMessage"
	// The contact form's own failures, including the ones CWCloud's
	// contact-request API reports back (see cwcloudContactErrors).
	CodeContactFormNotConfigured = "errors.contactFormNotConfigured"
	CodeContactFormFailed        = "errors.contactFormFailed"
	CodeContactRateLimited       = "errors.contactRateLimited"
	CodeContactMessageTooShort   = "errors.contactMessageTooShort"
	CodeContactGibberish         = "errors.contactGibberish"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeError(w http.ResponseWriter, status int, message, i18nCode string) {
	writeJSON(w, status, errorBody{Message: message, I18nCode: i18nCode})
}

// passwordPolicyMessages pairs each i18n code utils.IsPasswordValid can return
// with its English fallback message.
var passwordPolicyMessages = map[string]string{
	CodePasswordTooShort: "Password must be at least 8 characters long",
	CodePasswordNoUpper:  "Password must contain an uppercase letter",
	CodePasswordNoLower:  "Password must contain a lowercase letter",
	CodePasswordNoSymbol: "Password must contain a special character",
}

// writeInvalidPassword rejects a request whose new password failed the policy.
func writeInvalidPassword(w http.ResponseWriter, code string) {
	writeError(w, http.StatusBadRequest, passwordPolicyMessages[code], code)
}

// storeErrorMapping pairs each sentinel error the store layer can return with
// the status and i18n code it should surface as.
var storeErrorMapping = []struct {
	err     error
	status  int
	message string
	code    string
}{
	{store.ErrNotFound, http.StatusNotFound, "Resource not found", CodeNotFound},
	{store.ErrDuplicateEmail, http.StatusBadRequest, "Email already in use", CodeDuplicateEmail},
	{store.ErrDuplicateHandle, http.StatusBadRequest, "This profile name is already taken", CodeDuplicateHandle},
	{store.ErrCannotRemoveOwner, http.StatusBadRequest, "The club owner cannot be removed or demoted", CodeCantRemoveOwner},
	{store.ErrAlreadyMember, http.StatusBadRequest, "This user is already a member of the club", CodeAlreadyMember},
	{store.ErrDuplicateExercise, http.StatusBadRequest, "An exercise with this name already exists", CodeDuplicateExercise},
}

// writeStoreError maps a store error to its HTTP status, falling back to a 500
// for anything unrecognized (which is logged, since it's a bug rather than a
// user error).
func writeStoreError(w http.ResponseWriter, err error) {
	for _, mapping := range storeErrorMapping {
		if errors.Is(err, mapping.err) {
			writeError(w, mapping.status, mapping.message, mapping.code)
			return
		}
	}
	slog.Error("unhandled store error", "error", err)
	writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
}

// decodeJSON reads a JSON request body, reporting a malformed one to the client
// in the shape the frontend expects.
func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", CodeInvalidRequestBody)
		return false
	}
	return true
}

// pagination reads the ?page= and ?size= query parameters, both optional.
func pagination(r *http.Request) (page, size int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	size, _ = strconv.Atoi(r.URL.Query().Get("size"))
	return page, size
}

// summarize projects a user onto the shape embedded in posts, comments and
// member lists.
func summarize(user models.User) models.UserSummary {
	name, surname := user.DisplayName()
	return models.UserSummary{
		ID: user.ID, Handle: user.Handle, Name: name, Surname: surname,
		Email: user.Email, Role: user.Role, Picture: user.Picture,
		PictureX: user.PictureX, PictureY: user.PictureY, Anonymous: user.Anonymous,
	}
}

// meResponse projects a user onto their own account view.
func meResponse(user models.User, activationMode string) models.UserMeResponse {
	return models.UserMeResponse{
		ID: user.ID, Email: user.Email, Name: user.Name, Surname: user.Surname,
		Handle: user.Handle, Bio: user.Bio, Role: user.Role, Picture: user.Picture,
		PictureX: user.PictureX, PictureY: user.PictureY, Locale: user.Locale,
		Username: user.Username, Anonymous: user.Anonymous,
		ProfileVisibility: user.ProfileVisibility, Birthdate: user.Birthdate,
		CoachRequest: user.CoachRequest, Bodyweight: user.Bodyweight,
		MFAEnabled: user.MFAEnabled, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
		I18nCode: models.I18nCodeForRole(user.Role, activationMode),
	}
}

// connectionRecorder is the one store method recordConnection needs, kept
// narrow so both handlers can take it without either depending on the other.
type connectionRecorder interface {
	RecordConnection(ctx context.Context, userID, ip string, at time.Time) error
}

// recordConnection notes the address a session was minted from.
//
// It is best-effort and deliberately silent on failure: this is administrative
// data, and losing a counter tick must never fail a login. It runs on session
// creation rather than per request - the alternative is a database write on
// every single call for a number nobody reads that often.
func recordConnection(r *http.Request, users connectionRecorder, userID string) {
	ip := utils.ClientIP(r)
	if utils.IsBlank(ip) || users == nil {
		return
	}
	if err := users.RecordConnection(r.Context(), userID, ip, time.Now().UTC()); err != nil {
		slog.Warn("failed to record a connection address", "userId", userID, "error", err)
	}
}

// localeOf returns the language to send a user's emails in, defaulting to the
// request's Accept-Language when they never picked one.
func localeOf(user models.User, r *http.Request) string {
	if utils.IsNotBlank(user.Locale) {
		return user.Locale
	}
	if len(r.Header.Get("Accept-Language")) >= 2 {
		return r.Header.Get("Accept-Language")[:2]
	}
	return "en"
}
