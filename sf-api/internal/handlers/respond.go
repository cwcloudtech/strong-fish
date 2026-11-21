package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/strength"
	"strong-fish-api/internal/utils"
)

// statusClientClosedRequest is nginx's 499 "Client Closed Request": the caller
// hung up before the response was ready. It is not a server error, so we answer
// with it rather than a 500 - keeping a canceled request out of the error log and
// out of the [ERROR] alerting that fires on 5xx.
const statusClientClosedRequest = 499

type errorBody struct {
	Message  string `json:"message"`
	I18nCode string `json:"i18n_code,omitempty"`
	// Detail is an upstream error - a status and a body - surfaced for
	// troubleshooting when a provider could not be reached. Never localized,
	// never shown as the main message: a client puts it in a console panel.
	Detail string `json:"detail,omitempty"`
	// Provider names which integration the detail is from (garmin, stance,
	// google-health, withings), so a console can say so when several providers
	// can each fail at once.
	Provider string `json:"provider,omitempty"`
}

// I18n codes for API errors. The frontend looks these up in its own translation
// dictionaries, falling back to the English Message when a code is absent or
// unrecognized (an older client, or one that doesn't know the code).
const (
	CodeInternal           = "errors.internal"
	CodeNotFound           = "errors.notFound"
	CodeInvalidRequestBody = "errors.invalidRequestBody"
	CodeAllFieldsRequired  = "errors.allFieldsRequired"
	CodeInvalidCredentials = "errors.invalidCredentials"
	CodeDuplicateEmail     = "errors.duplicateEmail"
	CodeDuplicateHandle    = "errors.duplicateHandle"
	CodeInvalidEmail       = "errors.invalidEmail"
	CodeInvalidHandle      = "errors.invalidHandle"
	CodeInvalidUsername    = "errors.invalidUsername"
	CodeDuplicateUsername  = "errors.duplicateUsername"
	CodeUsernameRequired   = "errors.usernameRequired"
	// CodeNameOrUsername is signing up with neither a name nor a username.
	// Either is enough to be somebody in a club; nothing is not.
	CodeNameOrUsername       = "errors.nameOrUsername"
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
	// The three ways reading somebody's steps can fail in a way they can act
	// on: nothing connected here, nothing linked at the provider, or a grant
	// that has to be given again.
	CodeDeviceNotConnected = "errors.deviceNotConnected"
	// CodeDeviceNotPairable is a pairing request against a provider that is
	// connected by OAuth, not by an account - the wrong path for it.
	CodeDeviceNotPairable = "errors.deviceNotPairable"
	// CodeStancePairFailed is Stance refusing the email/password at pairing.
	CodeStancePairFailed  = "errors.stancePairFailed"
	CodeDeviceNotLinked   = "errors.deviceNotLinked"
	CodeDeviceReauthorize = "errors.deviceReauthorize"
	CodeInvalidRange      = "errors.invalidRange"
	// CodeDeviceUnavailable is an integration the operator has not given
	// credentials to: offered on screen, but switched off here.
	CodeDeviceUnavailable = "errors.deviceUnavailable"
	// What can go wrong with a member's own model connection: a provider this
	// build cannot talk to, an address that is not a URL, a missing key, a
	// role that is not grantable, nothing connected at all, and the provider
	// itself saying no.
	CodeInvalidAIProvider = "errors.invalidAiProvider"
	CodeInvalidAIBaseURL  = "errors.invalidAiBaseUrl"
	CodeAIKeyRequired     = "errors.aiKeyRequired"
	CodeInvalidAIRole     = "errors.invalidAiRole"
	CodeAINotConfigured   = "errors.aiNotConfigured"
	CodeAIRefused         = "errors.aiRefused"
	// A member's own recording callback (a device the phone triggers): a
	// missing URL, one whose scheme does not match the transport, a transport
	// this build cannot fire, or an MQTT callback with no topic.
	CodeCallbackURLRequired   = "errors.callbackUrlRequired"
	CodeInvalidCallbackURL    = "errors.invalidCallbackUrl"
	CodeInvalidCallbackType   = "errors.invalidCallbackType"
	CodeCallbackTopicRequired = "errors.callbackTopicRequired"
	// CodeConversationTitle is retitling a thread to nothing, and
	// CodeNothingSelected is a bulk delete that named nothing to delete -
	// which is refused rather than read as "everything".
	CodeConversationTitle = "errors.conversationTitleRequired"
	CodeNothingSelected   = "errors.nothingSelected"
	// CodeSessionIncomplete is asking for a reading of a session that has not
	// been finished. Deliberately its own code: the member can fix it, by
	// logging the rest of their sets.
	CodeSessionIncomplete = "errors.sessionIncomplete"
	// CodeUpstream is a service strong-fish depends on being unreachable or
	// answering with something unusable - not the member's doing, and not
	// something a retry in the same second will fix.
	CodeUpstream             = "errors.upstream"
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
	CodeStorageDownloadFailed = "errors.storageDownloadFailed"
	CodeVideoTooLarge         = "errors.videoTooLarge"
	CodeUnsupportedVideo      = "errors.unsupportedVideo"
	CodeInvalidStorageType    = "errors.invalidStorageType"
	CodeInvalidStorageRole    = "errors.invalidStorageRole"
	CodeInvalidServiceAccount = "errors.invalidServiceAccount"
	// The events calendar.
	CodeEventTitleRequired  = "errors.eventTitleRequired"
	CodeInvalidEventDate    = "errors.invalidEventDate"
	CodeInvalidCalendarFile = "errors.invalidCalendarFile"
	// Profiles, search, invitations and coach requests.
	CodeInvalidBirthdate         = "errors.invalidBirthdate"
	CodeInvalidHeight            = "errors.invalidHeight"
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
	// A canceled context means the client went away mid-flight (closed the tab,
	// navigated, or an aborted fetch), not a server fault. The response won't be
	// read anyway, so record the cause at debug and answer 499 so it never lands in
	// the error log or triggers 5xx alerting.
	if errors.Is(err, context.Canceled) {
		slog.Debug("request canceled by client", "error", err)
		w.WriteHeader(statusClientClosedRequest)
		return
	}
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
		ProfileVisibility: user.ProfileVisibility, FitnessPublic: user.FitnessPublic,
		Birthdate:    user.Birthdate,
		CoachRequest: user.CoachRequest, Bodyweight: user.Bodyweight, Height: user.Height,
		Gender:       string(strength.NormalizeGender(user.Gender)),
		EffortMetric: models.NormalizeEffortMetric(user.EffortMetric),
		Socials:      models.NormalizeSocials(user.Socials),
		MFAEnabled:   user.MFAEnabled, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
		I18nCode: models.I18nCodeForRole(user.Role, activationMode),
	}
}

// recordConnection notes the address a session was minted from, against the
// member it belongs to, and - on a first sighting - enriches it with
// geolocation. The work is handed to the session store's visit tracker.
//
// Best-effort and fire-and-forget: this is administrative data, and neither a
// counter tick nor a geo lookup must ever slow a login or fail one. It runs on
// session creation rather than per request - the alternative is a write on
// every single call for data nobody reads that often.
func recordConnection(r *http.Request, visits middleware.VisitorStore, enrich middleware.Enricher, userID string) {
	ip := utils.ClientIP(r)
	if utils.IsBlank(ip) || visits == nil || utils.IsBlank(userID) {
		return
	}
	id := userID
	go middleware.TrackVisit(visits, enrich, &id, ip)
}

// recordAuthAudit writes an authentication event - a sign-in or a registration -
// to the superadmin audit trail, naming the account and the address it came
// from. Best-effort and nil-safe (see AuditStore.Record): a nil store, which is
// how an unwired handler leaves it, is a no-op.
func recordAuthAudit(audit *store.AuditStore, r *http.Request, action string, user models.User) {
	if audit == nil {
		return
	}
	audit.Record(r.Context(), user.ID, action, models.AuditSubjectUser, user.ID, map[string]any{
		"email": user.Email,
		"ip":    utils.ClientIP(r),
	})
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
