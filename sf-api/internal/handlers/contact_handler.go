package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"strong-fish-api/internal/contact"
	"strong-fish-api/internal/utils"
)

// cwcloudContactErrors maps CWCloud's own contact-request i18n_code values onto
// this API's error response, so a rejection the submitter can actually act on
// (too many messages, too short, looks like spam) reaches them as such rather
// than as a generic failure. A code that isn't here - including a blank one,
// since CWCloud's i18n_code is optional - falls through to CodeContactFormFailed.
var cwcloudContactErrors = map[string]struct {
	status  int
	code    string
	message string
}{
	"cf_rate_limiting":  {http.StatusTooManyRequests, CodeContactRateLimited, "You're sending too many messages, please try again later"},
	"message_too_short": {http.StatusBadRequest, CodeContactMessageTooShort, "Your message is too short"},
	"gibberish":         {http.StatusBadRequest, CodeContactGibberish, "Your message looks like spam, please rewrite it"},
}

type ContactHandler struct {
	contact *contact.Client
}

func NewContactHandler(contact *contact.Client) *ContactHandler {
	return &ContactHandler{contact: contact}
}

// contactPayload is the body accepted by POST /v1/contact. Name and firstname
// are optional; the rest is required.
//
// The caller's IP is deliberately not part of it: it's read from the request's
// own X-Real-IP/X-Forwarded-By headers (set by the reverse proxy) and forwarded
// to CWCloud as X-Client-IP. That's what CWCloud rate-limits on, so letting the
// submitter supply it would hand them the means to sidestep the limit.
type contactPayload struct {
	Email     string `json:"email"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
	Name      string `json:"name"`
	Firstname string `json:"firstname"`
}

// Create submits the contact form to CWCloud's contact-request API.
//
// With CWCLOUD_CONTACT_FORM_ID unset it answers 405 rather than the 500 an
// unconfigured dependency would normally get: an operator who didn't set a form
// id chose not to have a contact form, which is a statement about the method
// being unavailable, not a server fault.
func (h *ContactHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.contact.Configured() {
		writeError(w, http.StatusMethodNotAllowed, "The contact form is not configured", CodeContactFormNotConfigured)
		return
	}

	var p contactPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Email) || utils.IsBlank(p.Subject) || utils.IsBlank(p.Message) {
		writeError(w, http.StatusBadRequest, "Please add email, subject and message fields", CodeAllFieldsRequired)
		return
	}
	if !utils.IsValidEmail(p.Email) {
		writeError(w, http.StatusBadRequest, "Please add a valid email", CodeInvalidEmail)
		return
	}

	err := h.contact.Send(r.Context(), contact.Submission{
		Email: p.Email, Subject: p.Subject, Message: p.Message,
		Name: p.Name, Firstname: p.Firstname,
		ClientIP: contact.ResolveClientIP(r),
	})
	if err != nil {
		var apiErr *contact.APIError
		if errors.As(err, &apiErr) {
			if mapped, ok := cwcloudContactErrors[apiErr.Code]; ok {
				writeError(w, mapped.status, mapped.message, mapped.code)
				return
			}
		}
		slog.Error("failed to submit contact form", "error", err)
		writeError(w, http.StatusBadGateway, "Failed to send your message, please try again later", CodeContactFormFailed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Your message has been sent."})
}
