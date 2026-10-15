package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/oidc"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// DeviceHandler connects a member's outside accounts and reads what they
// counted.
//
// One handler for every provider rather than one per integration: what
// changes between them is the four methods behind devicelink.Provider, and
// everything here - who may connect, where the browser comes back to, how a
// range becomes dates, what the client is told when a grant has lapsed - is
// the same whoever is counting the steps.
type DeviceHandler struct {
	providers  []devicelink.Provider
	links      *store.DeviceLinkStore
	jwtSecret  string
	apiBaseURL string
	uiBaseURL  string
}

func NewDeviceHandler(providers []devicelink.Provider, links *store.DeviceLinkStore,
	jwtSecret, apiBaseURL, uiBaseURL string) *DeviceHandler {
	return &DeviceHandler{
		providers: providers, links: links, jwtSecret: jwtSecret,
		apiBaseURL: apiBaseURL, uiBaseURL: uiBaseURL,
	}
}

// redirectURI is both where the provider sends the browser back and what is
// echoed during the code exchange; the two must match exactly, so there is one
// function that builds it.
func (h *DeviceHandler) redirectURI(provider string) string {
	return h.apiBaseURL + "/v1/devices/" + provider + "/callback"
}

// List reports every integration this deployment knows about and whether the
// caller has connected it.
//
// Providers the operator has not configured are listed as unavailable rather
// than omitted: a screen that renders nothing at all leaves a member wondering
// whether the feature exists, and a support answer of "it is not switched on
// here" is a better one than silence.
func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	connected, err := h.links.ListProviders(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	linked := map[string]bool{}
	for _, provider := range connected {
		linked[provider] = true
	}

	results := make([]models.DeviceConnection, 0, len(h.providers))
	for _, provider := range h.providers {
		connection := models.DeviceConnection{
			Provider:  provider.Name(),
			Available: provider.Enabled(),
			Connected: linked[provider.Name()],
		}
		if connection.Connected {
			if link, err := h.links.Find(r.Context(), userID, provider.Name()); err == nil {
				at := link.CreatedAt
				connection.ConnectedAt = &at
			}
		}
		results = append(results, connection)
	}
	writeJSON(w, http.StatusOK, results)
}

// Authorize hands back the URL the member's browser has to visit to consent.
//
// A URL rather than a redirect, because the caller is script: the web app
// opens it in a tab and the phone in a system browser, and both need to decide
// that for themselves. It also means this route stays authenticated - the
// member is identified by their session here, and that identity is what the
// signed state carries through the consent screen to the callback.
func (h *DeviceHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	provider, ok := h.provider(w, r)
	if !ok {
		return
	}
	if !provider.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	state, err := oidc.SignState(h.jwtSecret, devicelink.StateLabel(provider.Name(), userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"url": provider.AuthorizationURL(h.redirectURI(provider.Name()), state),
	})
}

// Callback finishes the consent.
//
// Unauthenticated by necessity: what arrives is the provider redirecting a
// browser, with no Authorization header on it. The member it belongs to comes
// from the signed state instead, which is why the signature is checked before
// anything is stored.
//
// It answers with a redirect to the web app rather than JSON, because what is
// looking at it is a browser window the member is watching - on the phone too,
// where the system browser is where consent happened.
func (h *DeviceHandler) Callback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")

	if utils.IsNotBlank(r.URL.Query().Get("error")) {
		h.finish(w, r, providerName, "denied")
		return
	}

	state := r.URL.Query().Get("state")
	label, err := oidc.ProviderFromState(state)
	if err != nil {
		h.finish(w, r, providerName, "invalid_state")
		return
	}
	if err := oidc.VerifyState(h.jwtSecret, label, state); err != nil {
		h.finish(w, r, providerName, "invalid_state")
		return
	}
	stateProvider, userID, ok := devicelink.SplitStateLabel(label)
	// The provider in the path and the one the state was signed for have to
	// agree, or a state issued for one integration would authorize another.
	if !ok || stateProvider != providerName {
		h.finish(w, r, providerName, "invalid_state")
		return
	}

	provider, ok := devicelink.Find(h.providers, providerName)
	if !ok {
		h.finish(w, r, providerName, "unavailable")
		return
	}

	code := r.URL.Query().Get("code")
	if utils.IsBlank(code) {
		h.finish(w, r, providerName, "missing_code")
		return
	}

	token, err := provider.Exchange(r.Context(), code, h.redirectURI(providerName))
	if err != nil {
		// The redirect URI is logged because it is half of the commonest
		// failure: the provider compares it character for character against
		// what is registered, and the value it was given is the only way to
		// see which end is wrong.
		slog.Error("device link: the provider refused the code exchange",
			"provider", providerName, "redirect_uri", h.redirectURI(providerName), "error", err)
		h.finish(w, r, providerName, "exchange_failed")
		return
	}

	// Checked now rather than at the first read: consent can succeed against
	// an account that has never opened the provider's app, and "connect your
	// watch first" is only useful while the member is still in the flow.
	externalID, err := provider.CheckAccount(r.Context(), token.AccessToken)
	if err != nil {
		if errors.Is(err, devicelink.ErrNotLinked) {
			h.finish(w, r, providerName, "not_linked")
			return
		}
		// Its own outcome, not "exchange_failed": consent worked and the
		// token is good - what failed is reading the account with it, which
		// is usually the provider's API not being enabled on the project.
		// Telling the two apart is the difference between checking the OAuth
		// client and checking the API library.
		slog.Error("device link: consent succeeded but the account could not be read",
			"provider", providerName, "error", err)
		h.finish(w, r, providerName, "account_failed")
		return
	}

	if _, err := h.links.Save(r.Context(), models.DeviceLink{
		UserID: userID, Provider: providerName,
		AccessToken: token.AccessToken, RefreshToken: token.RefreshToken,
		ExpiresAt: token.ExpiresAt, Scope: token.Scope, ExternalUserID: externalID,
	}); err != nil {
		slog.Error("device link: the grant could not be stored",
			"provider", providerName, "error", err)
		h.finish(w, r, providerName, "store_failed")
		return
	}

	h.finish(w, r, providerName, "connected")
}

// finish sends the browser back to the web app with the outcome in the query
// string, which is the only channel there is: the window came from the
// provider, not from the app.
func (h *DeviceHandler) finish(w http.ResponseWriter, r *http.Request, provider, status string) {
	http.Redirect(w, r, h.uiBaseURL+"/dashboard/steps?device="+url.QueryEscape(provider)+
		"&status="+url.QueryEscape(status), http.StatusFound)
}

// Disconnect drops a member's grant.
//
// Only the local row: revoking at the provider is the member's to do, from
// their own account page, and an app that silently revoked a token it was
// given would be making a decision about somebody else's Google account.
func (h *DeviceHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	provider, ok := h.provider(w, r)
	if !ok {
		return
	}
	if err := h.links.Delete(r.Context(), userID, provider.Name()); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// stepRanges is every view the app offers, as a number of days ending today.
//
// Ninety is not an arbitrary ceiling: it is the longest span the provider
// returns in one call, so it is the longest view that can exist without the
// app quietly making several requests and stitching them together.
var stepRanges = map[string]int{
	"24h":   1,
	"48h":   2,
	"week":  7,
	"month": 30,
	"90d":   90,
}

// Steps returns a member's daily step counts for one of the named ranges.
func (h *DeviceHandler) Steps(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	provider, ok := h.provider(w, r)
	if !ok {
		return
	}

	if !provider.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	rangeName := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("range")))
	if rangeName == "" {
		rangeName = "week"
	}
	days, known := stepRanges[rangeName]
	if !known {
		writeError(w, http.StatusBadRequest, "Unknown range", CodeInvalidRange)
		return
	}
	if max := provider.MaxRangeDays(); days > max {
		days = max
	}

	link, err := h.links.Find(r.Context(), userID, provider.Name())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No account connected", CodeDeviceNotConnected)
			return
		}
		writeStoreError(w, err)
		return
	}

	accessToken, err := h.freshToken(r, provider, link)
	if err != nil {
		h.writeProviderError(w, err, provider.Name())
		return
	}

	// Whole days, ending today. The provider aggregates by calendar day, so a
	// rolling window is not on offer: "last 24h" is today's count, which is
	// also the number the watch itself shows.
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(days - 1))

	counts, err := provider.DailySteps(r.Context(), accessToken, start, end)
	if err != nil {
		h.writeProviderError(w, err, provider.Name())
		return
	}

	series := models.StepSeries{
		Provider: provider.Name(),
		Range:    rangeName,
		From:     start.Format("2006-01-02"),
		To:       end.Format("2006-01-02"),
		Days:     make([]models.StepDay, 0, len(counts)),
	}
	for _, count := range counts {
		series.Days = append(series.Days, models.StepDay{Date: count.Date, Steps: count.Steps})
		series.Total += count.Steps
	}
	if len(series.Days) > 0 {
		series.DailyAverage = series.Total / int64(len(series.Days))
	}
	writeJSON(w, http.StatusOK, series)
}

// freshToken renews the stored grant when it has lapsed, and writes the new
// one back so the next read does not have to.
func (h *DeviceHandler) freshToken(r *http.Request, provider devicelink.Provider,
	link models.DeviceLink) (string, error) {
	if !link.Expired() {
		return link.AccessToken, nil
	}
	if utils.IsBlank(link.RefreshToken) {
		return utils.EMPTY, devicelink.ErrReauthorize
	}

	token, err := provider.Refresh(r.Context(), link.RefreshToken)
	if err != nil {
		return utils.EMPTY, err
	}

	link.AccessToken = token.AccessToken
	link.RefreshToken = token.RefreshToken
	link.ExpiresAt = token.ExpiresAt
	if token.Scope != "" {
		link.Scope = token.Scope
	}
	if _, err := h.links.Save(r.Context(), link); err != nil {
		// The token is good even if storing it failed; the read can go ahead
		// and the next one will refresh again.
		slog.Error("device link refresh not stored", "provider", provider.Name(), "error", err)
	}
	return token.AccessToken, nil
}

// writeProviderError turns the two failures a member can act on into codes the
// clients can translate, and everything else into a plain upstream error.
func (h *DeviceHandler) writeProviderError(w http.ResponseWriter, err error, provider string) {
	switch {
	case errors.Is(err, devicelink.ErrNotLinked):
		writeError(w, http.StatusConflict, "Account not linked at the provider", CodeDeviceNotLinked)
	case errors.Is(err, devicelink.ErrReauthorize):
		writeError(w, http.StatusUnauthorized, "Connect the account again", CodeDeviceReauthorize)
	default:
		slog.Error("device provider call failed", "provider", provider, "error", err)
		writeError(w, http.StatusBadGateway, "The provider could not be reached", CodeUpstream)
	}
}

// provider resolves the {provider} path segment, answering 404 for one this
// deployment has not configured.
func (h *DeviceHandler) provider(w http.ResponseWriter, r *http.Request) (devicelink.Provider, bool) {
	provider, ok := devicelink.Find(h.providers, chi.URLParam(r, "provider"))
	if !ok {
		writeError(w, http.StatusNotFound, "Unknown or disabled provider", CodeNotFound)
		return nil, false
	}
	return provider, true
}
