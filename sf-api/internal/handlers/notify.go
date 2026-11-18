package handlers

import (
	"context"
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

// Notifier announces a noteworthy event - a sign-in, a sign-out, a finished
// training session, a new post - to an external channel such as Discord, tagged
// with who did it and the address they did it from. It is an interface here so
// the concrete sender (internal/notify) stays out of the handler layer, and so
// a deployment with no webhook configured simply has no notifier.
//
// It resolves the geolocation of ip itself: the handlers know the member and
// the address, and where that address is is the notifier's to find out.
type Notifier interface {
	Notify(ctx context.Context, event, member, email, ip string)
}

// notifyEvent fires one announcement, best-effort and off the request path. A
// nil notifier (no webhook configured) is a no-op, and a failure to announce is
// never allowed to touch the member's request.
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

// memberLabel is the most recognisable name for a member in an announcement:
// their real name, else the handle or username they go by, else their email.
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

// The events announced. Kept as constants so the wording is in one place and a
// log search or a Discord filter can rely on it.
const (
	EventSignIn  = "Sign-in"
	EventSignOut = "Sign-out"
	EventSignUp  = "Sign-up"
	EventSession = "Training session completed"
	EventNewPost = "New post"
)
