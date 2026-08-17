// Package email sends strong-fish's transactional emails (account
// confirmation, password reset, club invitation, program assignment) through
// CWCloud's email API - the same integration cwclock uses, driven by
// CWCLOUD_API_URL/CWCLOUD_API_KEY.
//
// It is deliberately best-effort throughout: a missing configuration or an
// unreachable CWCloud API is logged, never returned as an error, so the
// caller's own flow (registration, an invitation, ...) never fails because of
// it.
//
// Every message exists in English and French; the recipient's stored locale
// picks which one goes out.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"strong-fish-api/internal/templates"
	"strong-fish-api/internal/utils"
)

type request struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Content string `json:"content"`
}

// Sender posts emails to CWCloud's email API (POST {apiURL}/v1/email).
type Sender struct {
	apiURL string
	apiKey string
	from   string
	// selfBaseURL is this API's own public base URL, used to build the <img
	// src> logo URL emails reference - distinct from apiURL, which is
	// CWCloud's email-sending API.
	selfBaseURL string
	client      *http.Client
}

// NewSender builds a Sender for the given CWCloud API base URL/key and From
// address. apiURL/apiKey are allowed to be blank - send logs and skips rather
// than failing when they are.
func NewSender(apiURL, apiKey, from, selfBaseURL string) *Sender {
	return &Sender{
		apiURL: apiURL, apiKey: apiKey, from: from, selfBaseURL: selfBaseURL,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

var bodyTemplate = template.Must(template.New("email").Parse(templates.EmailHTML))

// buttonStyle mirrors the frontend's primary button so a CTA in an email looks
// like one in the app.
const buttonStyle = "display:inline-block;margin-top:8px;padding:11px 22px;" +
	"background-color:#0e5e9b;color:#ffffff;font-weight:600;" +
	"border-radius:8px;text-decoration:none;"

const (
	mutedStyle  = "color:#64748b;"
	centerStyle = "text-align:center;"
)

// localized picks the French variant when locale is "fr", English otherwise -
// English is the fallback for an account that never picked a language.
func localized(locale, en, fr string) string {
	return utils.If(locale == "fr", fr, en)
}

// logoURL resolves the <img src> for an email's header to a stable HTTPS URL
// rather than an embedded data: URI: many email-sending APIs and mail clients
// strip data: URIs from <img src> outright, a limitation no amount of correct
// HTML can work around.
func (s *Sender) logoURL() string {
	return s.selfBaseURL + "/v1/assets/logo.png"
}

// renderBody wraps body in the shared email layout.
func (s *Sender) renderBody(title string, body template.HTML) (string, error) {
	var buf bytes.Buffer
	err := bodyTemplate.Execute(&buf, struct {
		Title string
		Logo  template.URL
		Body  template.HTML
	}{Title: title, Logo: template.URL(s.logoURL()), Body: body})
	if err != nil {
		return utils.EMPTY, err
	}
	return buf.String(), nil
}

// send posts one email best-effort: a blank apiURL/apiKey or a failed request
// is logged and otherwise ignored.
func (s *Sender) send(ctx context.Context, to, subject, htmlContent string) {
	if utils.IsBlank(s.apiURL) || utils.IsBlank(s.apiKey) {
		slog.Warn("cwcloud email api is not configured (CWCLOUD_API_URL/CWCLOUD_API_KEY), skipping email", "to", to, "subject", subject)
		return
	}

	body, err := json.Marshal(request{From: s.from, To: to, Subject: subject, Content: htmlContent})
	if err != nil {
		slog.Error("failed to marshal email payload", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL+"/v1/email", bytes.NewReader(body))
	if err != nil {
		slog.Error("failed to build cwcloud email request", "error", err)
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Error("cwcloud email api is not available", "error", err, "to", to, "subject", subject)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		slog.Error("cwcloud email api returned an error", "status", resp.StatusCode, "to", to, "subject", subject)
	}
}

// deliver renders and sends one message, logging a template failure rather
// than propagating it (see the package comment on best-effort delivery).
func (s *Sender) deliver(ctx context.Context, to, subject string, body template.HTML, what string) {
	html, err := s.renderBody(subject, body)
	if err != nil {
		slog.Error("failed to render email", "kind", what, "error", err)
		return
	}
	s.send(ctx, to, subject, html)
}

// cta renders a centered call-to-action button linking to url.
func cta(url, label string) string {
	return fmt.Sprintf(`<p style="%s"><a href="%s" style="%s">%s</a></p>`,
		centerStyle, template.HTMLEscapeString(url), buttonStyle, template.HTMLEscapeString(label))
}

// muted renders secondary help text under a CTA.
func muted(text string) string {
	return fmt.Sprintf(`<p style="%s">%s</p>`, mutedStyle, template.HTMLEscapeString(text))
}

// SendConfirmation emails the account-confirmation link to a newly registered
// user (activation mode "email").
func (s *Sender) SendConfirmation(ctx context.Context, to, locale, confirmURL string) {
	subject := localized(locale, "Confirm your strong-fish account", "Confirmez votre compte strong-fish")
	body := template.HTML(localized(locale,
		`<p>Welcome to strong-fish!</p><p>Confirm your account to start training:</p>`+
			cta(confirmURL, "Confirm my account")+
			muted("If you didn't create this account, you can safely ignore this email."),
		`<p>Bienvenue sur strong-fish&nbsp;!</p><p>Confirmez votre compte pour commencer à vous entraîner&nbsp;:</p>`+
			cta(confirmURL, "Confirmer mon compte")+
			muted("Si vous n'êtes pas à l'origine de cette inscription, vous pouvez ignorer cet email."),
	))
	s.deliver(ctx, to, subject, body, "confirmation")
}

// SendPasswordReset emails the password-renewal link to a user who requested
// one.
func (s *Sender) SendPasswordReset(ctx context.Context, to, locale, resetURL string) {
	subject := localized(locale, "Reset your strong-fish password", "Réinitialisez votre mot de passe strong-fish")
	body := template.HTML(localized(locale,
		`<p>We received a request to reset your strong-fish password.</p>`+
			cta(resetURL, "Choose a new password")+
			muted("If you didn't request this, you can safely ignore this email."),
		`<p>Nous avons reçu une demande de réinitialisation de votre mot de passe strong-fish.</p>`+
			cta(resetURL, "Choisir un nouveau mot de passe")+
			muted("Si vous n'êtes pas à l'origine de cette demande, vous pouvez ignorer cet email."),
	))
	s.deliver(ctx, to, subject, body, "password reset")
}

// SendClubInvitation tells a member they were added to a club.
func (s *Sender) SendClubInvitation(ctx context.Context, to, locale, clubName, coachName, clubURL string) {
	subject := localized(locale,
		fmt.Sprintf("You joined %s on strong-fish", clubName),
		fmt.Sprintf("Vous avez rejoint %s sur strong-fish", clubName))
	body := template.HTML(localized(locale,
		fmt.Sprintf(`<p><strong>%s</strong> added you to the club <strong>%s</strong>.</p>`,
			template.HTMLEscapeString(coachName), template.HTMLEscapeString(clubName))+
			cta(clubURL, "Open the club"),
		fmt.Sprintf(`<p><strong>%s</strong> vous a ajouté au club <strong>%s</strong>.</p>`,
			template.HTMLEscapeString(coachName), template.HTMLEscapeString(clubName))+
			cta(clubURL, "Ouvrir le club"),
	))
	s.deliver(ctx, to, subject, body, "club invitation")
}

// SendProgramAssigned tells a member a coach assigned them a training program.
func (s *Sender) SendProgramAssigned(ctx context.Context, to, locale, programName, coachName, programURL string) {
	subject := localized(locale,
		fmt.Sprintf("New program: %s", programName),
		fmt.Sprintf("Nouveau programme : %s", programName))
	body := template.HTML(localized(locale,
		fmt.Sprintf(`<p><strong>%s</strong> assigned you the program <strong>%s</strong>.</p>`+
			`<p>Make sure your 1RMs are up to date - every set's load is computed from them.</p>`,
			template.HTMLEscapeString(coachName), template.HTMLEscapeString(programName))+
			cta(programURL, "Open my program"),
		fmt.Sprintf(`<p><strong>%s</strong> vous a assigné le programme <strong>%s</strong>.</p>`+
			`<p>Pensez à mettre à jour vos 1RM : les charges de chaque série en sont calculées.</p>`,
			template.HTMLEscapeString(coachName), template.HTMLEscapeString(programName))+
			cta(programURL, "Ouvrir mon programme"),
	))
	s.deliver(ctx, to, subject, body, "program assignment")
}
