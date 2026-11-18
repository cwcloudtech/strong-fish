package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

// announceTimeout bounds the background work one announcement does - a geo
// lookup and a webhook POST. It runs detached from the request, so nothing a
// member does waits on a chat message going out.
const announceTimeout = 10 * time.Second

// Notifier announces a noteworthy event to an external channel such as Discord.
// It has two shapes: Notify posts a rich card with the member, their address and
// its geolocation (the sign-in-style events), and NotifyText posts a plain line
// (the [CONNECT]/[REGISTER]/[DISCONNECT]/[CHAT]/[REPORT] notices, whose wording
// mirrors ~/uprodit). It is an interface here so the concrete sender
// (internal/notify) stays out of the handler layer, and so a deployment with no
// webhook configured simply has no notifier.
type Notifier interface {
	Notify(ctx context.Context, event, member, email, ip string)
	NotifyText(ctx context.Context, text string)
}

// notifyEvent fires one rich-card announcement (a finished session, a new post),
// best-effort and off the request path.
func notifyEvent(n Notifier, r *http.Request, event string, user models.User) {
	if n == nil {
		return
	}
	member := memberLabel(user)
	ip := utils.ClientIP(r)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), announceTimeout)
		defer cancel()
		n.Notify(ctx, event, member, user.Email, ip)
	}()
}

// notifyText fires a plain-text line, best-effort and off the request path. A
// nil notifier (no webhook) is a no-op, and a failure to announce never touches
// the member's request.
func notifyText(n Notifier, text string) {
	if n == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), announceTimeout)
		defer cancel()
		n.NotifyText(ctx, text)
	}()
}

// --- auth notices (same wording as ~/uprodit's AuthenticationNotificationUtils) ---

// authInfo is the JSON blob embedded in the auth notices, mirroring ~/uprodit's.
// profileId is the user id here, since StrongFish has no separate profile.
type authInfo struct {
	Username  string `json:"username"`
	IP        string `json:"ip"`
	Status    string `json:"authenticationStatus"`
	ProfileID string `json:"profileId,omitempty"`
}

// notifyLogin posts "[CONNECT] :heart_eyes: url = <profile>, infos = {…}" on a
// successful sign-in (password or OIDC).
func notifyLogin(n Notifier, r *http.Request, user models.User, uiBaseURL string) {
	info := authInfo{Username: user.Email, IP: utils.ClientIP(r), Status: "success", ProfileID: user.ID}
	notifyText(n, fmt.Sprintf("[CONNECT] :heart_eyes: url = %s, infos = %s", profileURL(uiBaseURL, user), jsonStr(info)))
}

// notifyRegister posts "[REGISTER] :heart_eyes: username = <email>, url_adm =
// <profile>, lng = <locale>" on a new account.
func notifyRegister(n Notifier, r *http.Request, user models.User, uiBaseURL string) {
	lng := user.Locale
	if lng == "" {
		lng = "en"
	}
	notifyText(n, fmt.Sprintf("[REGISTER] :heart_eyes: username = %s, url_adm = %s, lng = %s",
		user.Email, profileURL(uiBaseURL, user), lng))
}

// notifyLogout posts "[DISCONNECT] :wave: url = <profile>, infos = {…}".
func notifyLogout(n Notifier, r *http.Request, user models.User, uiBaseURL string) {
	info := authInfo{Username: user.Email, IP: utils.ClientIP(r), Status: "logout", ProfileID: user.ID}
	notifyText(n, fmt.Sprintf("[DISCONNECT] :wave: url = %s, infos = %s", profileURL(uiBaseURL, user), jsonStr(info)))
}

// notifyChat posts "[CHAT] :nerd: from: <email> (<id>) to <email> (<id>)".
func notifyChat(n Notifier, sender, recipient models.User) {
	notifyText(n, fmt.Sprintf("[CHAT] :nerd: from: %s (%s) to %s (%s)",
		sender.Email, sender.ID, recipient.Email, recipient.ID))
}

// notifyReport posts "[REPORT] :nerd: from <email> (<id>) on <email> (<id>)".
func notifyReport(n Notifier, reporter, reported models.User) {
	notifyText(n, fmt.Sprintf("[REPORT] :nerd: from %s (%s) on %s (%s)",
		reporter.Email, reporter.ID, reported.Email, reported.ID))
}

// notifyPost posts "[POST] :nerd: from: <email> (<id>) to: <audience>" on a new
// feed post. The audience is the post's visibility, noting a club count when it
// is shared to clubs.
func notifyPost(n Notifier, author models.User, visibility string, clubCount int) {
	audience := visibility
	if clubCount > 0 {
		audience = fmt.Sprintf("%s (%d club(s))", visibility, clubCount)
	}
	notifyText(n, fmt.Sprintf("[POST] :nerd: from: %s (%s) to: %s", author.Email, author.ID, audience))
}

// notifyComment posts "[COMMENT] :nerd: from: <email> (<id>) to: <email> (<id>)"
// on a new comment - from the commenter, to the author of the post.
func notifyComment(n Notifier, commenter, postAuthor models.User) {
	notifyText(n, fmt.Sprintf("[COMMENT] :nerd: from: %s (%s) to: %s (%s)",
		commenter.Email, commenter.ID, postAuthor.Email, postAuthor.ID))
}

// profileURL is the link to a member's public profile, by handle when they have
// one and by id otherwise - "unknown" when there is no base URL to build on.
func profileURL(uiBaseURL string, user models.User) string {
	if utils.IsBlank(uiBaseURL) {
		return "unknown"
	}
	handle := user.Handle
	if utils.IsBlank(handle) {
		handle = user.ID
	}
	return strings.TrimRight(uiBaseURL, "/") + "/profile/" + handle
}

func jsonStr(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// memberLabel is the most recognisable name for a member in a rich-card
// announcement: their real name, else the handle or username they go by, else
// their email.
func memberLabel(user models.User) string {
	if label := strings.TrimSpace(user.Name + " " + user.Surname); label != "" {
		return label
	}
	if utils.IsNotBlank(user.Username) {
		return user.Username
	}
	if utils.IsNotBlank(user.Handle) {
		return "@" + user.Handle
	}
	return user.Email
}

// EventSession is the rich-card (geo) event for a finished training session.
const EventSession = "Training session completed"
