package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/healthsheet"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/oidc"
	"strong-fish-api/internal/programpdf"
	"strong-fish-api/internal/programpng"
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
	providers []devicelink.Provider
	links     *store.DeviceLinkStore
	// users is only for naming an export after the member it belongs to: a
	// picture of somebody's week that travels without their name on it is one
	// nobody can place a fortnight later.
	users      *store.UserStore
	jwtSecret  string
	apiBaseURL string
	uiBaseURL  string
	// ai is borrowed for the one thing this handler asks a model: a reading
	// of the member's health data. Reused rather than rebuilt so a connection
	// is resolved, spent and reported on in exactly one place.
	ai *AIHandler
	// relationTo resolves what a viewer is to a profile's owner - coach,
	// nutritionist, superadmin - so a coach or nutritionist of a shared club can
	// read an athlete's health data. nil until WithRelations is called, in which
	// case only self-access works. Wired to ProfileHandler.RelationTo.
	relationTo func(ctx context.Context, targetID, callerID string) (models.ViewerRelation, error)
}

func NewDeviceHandler(providers []devicelink.Provider, links *store.DeviceLinkStore,
	users *store.UserStore, jwtSecret, apiBaseURL, uiBaseURL string,
	aiHandler *AIHandler) *DeviceHandler {
	return &DeviceHandler{
		providers: providers, links: links, users: users, jwtSecret: jwtSecret,
		apiBaseURL: apiBaseURL, uiBaseURL: uiBaseURL, ai: aiHandler,
	}
}

// WithRelations wires the viewer→owner relation resolver, which is what lets a
// coach or nutritionist of a shared club read an athlete's health data.
func (h *DeviceHandler) WithRelations(
	relationTo func(ctx context.Context, targetID, callerID string) (models.ViewerRelation, error)) *DeviceHandler {
	h.relationTo = relationTo
	return h
}

// readUserID is whose health data a read is about: the session user for a
// /me route, or the {userId} in the path for a coach, a nutritionist or a
// public visitor reading someone's profile. It authorizes the latter and
// answers 404 (not 403) when refused, so a profile's existence is not leaked by
// the health endpoints any more than by the profile itself.
func (h *DeviceHandler) readUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	sessionID, _ := middleware.UserIDFromContext(r.Context())
	memberID := chi.URLParam(r, "userId")
	if memberID == "" || memberID == sessionID {
		if sessionID == "" {
			writeError(w, http.StatusUnauthorized, "Sign in to see your fitness data", CodeNotFound)
			return "", false
		}
		return sessionID, true
	}
	if h.canViewHealth(r.Context(), sessionID, memberID) {
		return memberID, true
	}
	writeError(w, http.StatusNotFound, "Not found", CodeNotFound)
	return "", false
}

// adviseUserID is whose data an action that spends an AI subscription is about:
// the session user, or a {userId} they coach or advise. Unlike readUserID it
// requires a session and refuses public access - a public profile lets anyone
// read a reading, not generate one on the athlete's behalf.
func (h *DeviceHandler) adviseUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	sessionID, _ := middleware.UserIDFromContext(r.Context())
	if sessionID == "" {
		writeError(w, http.StatusUnauthorized, "Sign in first", CodeNotFound)
		return "", false
	}
	memberID := chi.URLParam(r, "userId")
	if memberID == "" || memberID == sessionID {
		return sessionID, true
	}
	if h.relationTo != nil {
		if relation, err := h.relationTo(r.Context(), memberID, sessionID); err == nil &&
			(relation.Superadmin || relation.AdvisesOrManages()) {
			return memberID, true
		}
	}
	writeError(w, http.StatusNotFound, "Not found", CodeNotFound)
	return "", false
}

// canViewHealth decides whether viewerID (possibly "" for a logged-out visitor)
// may read targetID's fitness data: the member themselves, a provider the
// member made public, a superadmin, or a coach or nutritionist of a club the
// member is in.
func (h *DeviceHandler) canViewHealth(ctx context.Context, viewerID, targetID string) bool {
	if viewerID != "" && viewerID == targetID {
		return true
	}
	target, err := h.users.FindByID(ctx, targetID)
	if err != nil {
		return false
	}
	if target.FitnessPublic {
		return true
	}
	if viewerID == "" || h.relationTo == nil {
		return false
	}
	relation, err := h.relationTo(ctx, targetID, viewerID)
	if err != nil {
		return false
	}
	return relation.Self || relation.Superadmin || relation.AdvisesOrManages()
}

// memberName is who the export belongs to, or blank when that cannot be
// resolved - a missing name is a title the sheet does without, not a failure.
func (h *DeviceHandler) memberName(ctx context.Context, userID string) string {
	user, err := h.users.FindByID(ctx, userID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(user.Name + " " + user.Surname)
}

// effortMetric is how the member wants effort shown on their exported sheet -
// their own setting, or RPE when it cannot be read.
func (h *DeviceHandler) effortMetric(ctx context.Context, userID string) string {
	user, err := h.users.FindByID(ctx, userID)
	if err != nil {
		return models.EffortMetricRPE
	}
	return models.NormalizeEffortMetric(user.EffortMetric)
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
	userID, ok := h.readUserID(w, r)
	if !ok {
		return
	}

	connected, err := h.links.ListProviders(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	linked := map[string]bool{}
	for _, provider := range connected {
		linked[provider] = true
	}

	source := devicelink.ActivitySource(h.providers, linked)
	bodySource := devicelink.BodyCompositionSource(h.providers, linked)
	nutritionSource := devicelink.NutritionSource(h.providers, linked)
	sleepSource := devicelink.SleepSource(h.providers, linked)

	results := make([]models.DeviceConnection, 0, len(h.providers))
	for _, provider := range h.providers {
		connection := models.DeviceConnection{
			Provider:              provider.Name(),
			Available:             provider.Enabled(),
			Connected:             linked[provider.Name()],
			Capabilities:          devicelink.Capabilities(provider),
			ActivitySource:        provider.Name() == source,
			BodyCompositionSource: provider.Name() == bodySource,
			NutritionSource:       provider.Name() == nutritionSource,
			SleepSource:           provider.Name() == sleepSource,
		}
		if _, ok := provider.(devicelink.AccountPairer); ok {
			connection.AccountPaired = true
		}
		if _, ok := provider.(devicelink.KeyPairer); ok {
			connection.KeyPaired = true
		}
		if provider.Enabled() && provider.AuthorizationURL(h.redirectURI(provider.Name()), "probe") != "" {
			connection.OAuth = true
		}
		if pairer, ok := provider.(devicelink.GoogleAccountPairer); ok && provider.Enabled() {
			connection.GoogleClientID = pairer.GoogleClientID()
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

	token, err := provider.Exchange(r.Context(), code, h.redirectURI(providerName), state)
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
	if utils.IsBlank(externalID) {
		externalID = token.UserID
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
	"24h":  1,
	"48h":  2,
	"week": 7,
	// A fortnight is the widest view where both measures are whole: it is
	// exactly as far back as the provider will go for heart rate.
	"14d":   14,
	"month": 30,
	"90d":   90,
}

// Steps returns a member's daily step counts for one of the named ranges.
func (h *DeviceHandler) Steps(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.readUserID(w, r)
	if !ok {
		return
	}

	provider, link, days, rangeName, ok := h.readable(w, r, userID)
	if !ok {
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

// HeartRate returns a member's daily heart rate for one of the named ranges,
// with the zone each day reached.
func (h *DeviceHandler) HeartRate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.readUserID(w, r)
	if !ok {
		return
	}

	provider, link, days, rangeName, ok := h.readable(w, r, userID)
	if !ok {
		return
	}

	// Shortened rather than refused. The provider reaches back a fortnight
	// for beats and three months for steps, and refusing the wider views
	// would cost the member the one number that is still true on them - what
	// their heart was doing most recently. The response says it was
	// shortened, so a client can show that number without drawing a chart
	// captioned with a range it does not cover.
	clamped := false
	if limit := provider.MaxHeartRateDays(); days > limit {
		days, clamped = limit, true
	}

	accessToken, err := h.freshToken(r, provider, link)
	if err != nil {
		h.writeProviderError(w, err, provider.Name())
		return
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(days - 1))

	counts, err := provider.DailyHeartRate(r.Context(), accessToken, start, end)
	if err != nil {
		h.writeProviderError(w, err, provider.Name())
		return
	}

	series := models.HeartSeries{
		Provider: provider.Name(),
		Range:    rangeName,
		Clamped:  clamped,
		From:     start.Format("2006-01-02"),
		To:       end.Format("2006-01-02"),
		Days:     make([]models.HeartDay, 0, len(counts)),
	}
	total, known := 0, 0
	for _, count := range counts {
		day := models.HeartDay{
			Date:  count.Date,
			Avg:   int(math.Round(count.Avg)),
			Max:   int(math.Round(count.Max)),
			Min:   int(math.Round(count.Min)),
			Zone:  count.Zone,
			Known: count.Known,
		}
		series.Days = append(series.Days, day)
		if day.Known {
			total += day.Avg
			known++
		}
		if day.Max > series.PeakBPM {
			series.PeakBPM = day.Max
		}
	}
	if known > 0 {
		series.AverageBPM = total / known
	}
	writeJSON(w, http.StatusOK, series)
}

// Export renders all of a member's fitness data as one picture or PDF.
//
// One document, not one per device: steps and heart rate from the activity
// source, the latest body composition from the scale, the bar-speed charts
// from the tracker, and the AI's reading of the lot at the end. Each part is
// there only when something is behind it, so a member with just a scale and a
// bar-speed tracker still gets a sheet - their two things, and no empty step
// chart. The same two formats the programs offer, for the same two reasons: an
// image is what gets pasted into a conversation, a PDF is what gets kept.
func (h *DeviceHandler) Export(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.readUserID(w, r)
	if !ok {
		return
	}
	locale := sheetLocale(r)

	connectedNames, err := h.links.ListProviders(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	linked := map[string]bool{}
	for _, name := range connectedNames {
		linked[name] = true
	}

	// Steps and heart rate come from the one activity source, over the range
	// asked for (the window the screen is showing). Best-effort: a provider
	// that will not answer must not cost the member the rest of the sheet.
	var (
		steps models.StepSeries
		heart *models.HeartSeries
	)
	if name := devicelink.ActivitySource(h.providers, linked); name != "" {
		if found, ok := devicelink.Find(h.providers, name); ok {
			if activity, ok := found.(devicelink.ActivityProvider); ok && activity.Enabled() {
				if link, err := h.links.Find(r.Context(), userID, name); err == nil {
					days := exportRangeDays(r)
					if max := activity.MaxRangeDays(); days > max {
						days = max
					}
					if s, hh, err := h.healthSeries(r, activity, link, days); err == nil {
						steps, heart = s, hh
					} else {
						slog.Warn("fitness export: activity unavailable", "provider", name, "error", err)
					}
				}
			}
		}
	}

	sections := healthsheet.AllSections(steps, heart,
		h.exportNutrition(r, userID, exportRangeDays(r)), h.computeTargets(r, userID),
		h.exportBodyComposition(r, userID), h.exportSleep(r, userID),
		h.BarSpeedSets(r, userID), locale,
		h.effortMetric(r.Context(), userID))
	// The AI's reading of the same data, last, when it was asked recently
	// enough to be about this sheet - from whichever provider it was generated
	// under (see recentReadingAny).
	if reading := h.recentReadingAny(r, userID, linked, locale); reading != nil {
		sections = append(sections, *reading)
	}
	if len(sections) == 0 {
		writeError(w, http.StatusNotFound, "No fitness data to export", CodeNotFound)
		return
	}

	title := healthsheet.Title(h.memberName(r.Context(), userID), locale)
	subtitle := ""
	fileDate := time.Now().UTC().Format("2006-01-02")
	if len(steps.Days) > 0 {
		subtitle = healthsheet.Subtitle(steps, heart, locale)
		fileDate = steps.To
	}
	footer := h.uiBaseURL

	var (
		doc   []byte
		media string
	)
	if exportFormat(r) == formatPNG {
		doc, err = programpng.RenderTables(title, subtitle, sections, footer)
		media = mediaPNG
	} else {
		doc, err = programpdf.RenderTables(title, subtitle, sections, footer)
		media = mediaPDF
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}

	w.Header().Set("Content-Type", media)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", sheetFileName("fitness-"+fileDate, exportFormat(r))))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc)
}

// exportRangeDays reads the step/heart window off the export's range query,
// defaulting to a week and never exceeding what the ranges table offers.
func exportRangeDays(r *http.Request) int {
	rangeName := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("range")))
	if days, ok := stepRanges[rangeName]; ok {
		return days
	}
	return stepRanges["week"]
}

// BodyComposition returns the latest reading of every measure the member's
// scale has taken.
//
// No range: the panel is "where am I now", and the date on each measure says
// how current that is - a scale that measures fat but was last stepped on
// with bare feet a month ago says so, rather than being averaged away.
func (h *DeviceHandler) BodyComposition(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.readUserID(w, r)
	if !ok {
		return
	}

	found, ok := h.provider(w, r)
	if !ok {
		return
	}
	reader, ok := found.(devicelink.BodyCompositionReader)
	if !ok {
		writeError(w, http.StatusNotFound, "This provider has no body composition data", CodeNotFound)
		return
	}
	if !found.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	link, err := h.links.Find(r.Context(), userID, found.Name())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No account connected", CodeDeviceNotConnected)
			return
		}
		writeStoreError(w, err)
		return
	}

	accessToken, err := h.freshToken(r, found, link)
	if err != nil {
		h.writeProviderError(w, err, found.Name())
		return
	}

	measures, err := reader.LatestBodyComposition(r.Context(), accessToken)
	if err != nil {
		h.writeProviderError(w, err, found.Name())
		return
	}

	writeJSON(w, http.StatusOK, bodyComposition(found.Name(), measures))
}

// bodyComposition is a provider's measures in the shape the clients and the
// export read.
func bodyComposition(provider string, measures []devicelink.BodyMeasure) models.BodyComposition {
	result := models.BodyComposition{
		Provider: provider,
		Measures: make([]models.BodyMeasure, 0, len(measures)),
	}
	for _, measure := range measures {
		result.Measures = append(result.Measures, models.BodyMeasure{
			Type: measure.Type, Value: measure.Value, Unit: measure.Unit, TakenAt: measure.TakenAt,
		})
	}
	return result
}

// exportBodyComposition is the latest body composition for an export, from
// whichever connected provider has a scale behind it - not necessarily the
// one the export was asked of: steps from Google Health and weight from
// Withings belong on the same sheet, because they are the same member.
//
// Optional, like the heart rate: a scale that cannot be read right now must
// not cost the member the export of their steps.
// healthSeries is the member's steps and heart rate over the last days, as
// the export and the AI reading both need them.
//
// Heart rate is optional on purpose. It reaches back a fortnight against the
// steps' three months, and a provider that will not answer for this range
// must not cost the member the document they asked for. A series with no
// usable average comes back as nothing at all rather than as a chart of
// blanks.
func (h *DeviceHandler) healthSeries(r *http.Request, provider devicelink.ActivityProvider,
	link models.DeviceLink, days int) (models.StepSeries, *models.HeartSeries, error) {
	accessToken, err := h.freshToken(r, provider, link)
	if err != nil {
		return models.StepSeries{}, nil, err
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(days - 1))

	counts, err := provider.DailySteps(r.Context(), accessToken, start, end)
	if err != nil {
		return models.StepSeries{}, nil, err
	}
	steps := models.StepSeries{From: start.Format("2006-01-02"), To: end.Format("2006-01-02")}
	for _, count := range counts {
		steps.Days = append(steps.Days, models.StepDay{Date: count.Date, Steps: count.Steps})
		steps.Total += count.Steps
	}
	if len(steps.Days) > 0 {
		steps.DailyAverage = steps.Total / int64(len(steps.Days))
	}

	heartDays := days
	if limit := provider.MaxHeartRateDays(); heartDays > limit {
		heartDays = limit
	}
	readings, err := provider.DailyHeartRate(r.Context(), accessToken,
		end.AddDate(0, 0, -(heartDays-1)), end)
	if err != nil {
		slog.Warn("health data: heart rate unavailable, steps alone",
			"provider", provider.Name(), "error", err)
		return steps, nil, nil
	}

	series := models.HeartSeries{}
	total, known := 0, 0
	for _, reading := range readings {
		day := models.HeartDay{
			Date: reading.Date, Avg: int(math.Round(reading.Avg)),
			Max: int(math.Round(reading.Max)), Min: int(math.Round(reading.Min)),
			Zone: reading.Zone, Known: reading.Known,
		}
		series.Days = append(series.Days, day)
		if day.Known {
			total += day.Avg
			known++
		}
		if day.Max > series.PeakBPM {
			series.PeakBPM = day.Max
		}
	}
	if known == 0 {
		return steps, nil, nil
	}
	series.AverageBPM = total / known
	return steps, &series, nil
}

// exportBodyComposition is the latest body composition for an export, from the
// one scale the screen shows too: BodyCompositionSource picks it by the same
// priority (Withings before Garmin), so a member with both does not get two
// tables on the sheet, and the sheet agrees with the screen on whose it is.
func (h *DeviceHandler) exportBodyComposition(r *http.Request, userID string) *models.BodyComposition {
	connected, err := h.links.ListProviders(r.Context(), userID)
	if err != nil {
		return nil
	}
	linked := map[string]bool{}
	for _, name := range connected {
		linked[name] = true
	}
	name := devicelink.BodyCompositionSource(h.providers, linked)
	if name == "" {
		return nil
	}
	found, ok := devicelink.Find(h.providers, name)
	if !ok {
		return nil
	}
	reader, ok := found.(devicelink.BodyCompositionReader)
	if !ok {
		return nil
	}
	link, err := h.links.Find(r.Context(), userID, name)
	if err != nil {
		return nil
	}
	accessToken, err := h.freshToken(r, found, link)
	if err != nil {
		slog.Warn("health export: body composition unavailable", "provider", name, "error", err)
		return nil
	}
	measures, err := reader.LatestBodyComposition(r.Context(), accessToken)
	if err != nil {
		slog.Warn("health export: body composition unavailable", "provider", name, "error", err)
		return nil
	}
	result := bodyComposition(name, measures)
	return &result
}

// Sleep returns the member's last night from one provider: the good/fair/poor
// read and the stretches the chart draws.
func (h *DeviceHandler) Sleep(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.readUserID(w, r)
	if !ok {
		return
	}
	found, ok := h.provider(w, r)
	if !ok {
		return
	}
	reader, ok := found.(devicelink.SleepReader)
	if !ok {
		writeError(w, http.StatusNotFound, "This provider has no sleep data", CodeNotFound)
		return
	}
	if !found.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}
	link, err := h.links.Find(r.Context(), userID, found.Name())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No account connected", CodeDeviceNotConnected)
			return
		}
		writeStoreError(w, err)
		return
	}
	accessToken, err := h.freshToken(r, found, link)
	if err != nil {
		h.writeProviderError(w, err, found.Name())
		return
	}
	night, err := reader.LastNightSleep(r.Context(), accessToken)
	if err != nil {
		h.writeProviderError(w, err, found.Name())
		return
	}
	writeJSON(w, http.StatusOK, sleepNight(found.Name(), night))
}

// sleepNight maps a provider's night into the shape the clients and the export
// read.
func sleepNight(provider string, night devicelink.SleepNight) models.SleepNight {
	out := models.SleepNight{
		Provider:  provider,
		Date:      night.Date,
		Start:     night.Start,
		End:       night.End,
		AsleepMin: night.AsleepMin,
		AwakeMin:  night.AwakeMin,
	}
	for _, segment := range night.Segments {
		out.Segments = append(out.Segments, models.SleepSegment{
			Start: segment.Start, End: segment.End, Awake: segment.Awake,
		})
	}
	return out
}

// exportSleep is last night for an export or an AI reading, from the one source
// the screen shows too - SleepSources' priority (Google Health/Fitbit, Garmin,
// then Withings). Optional, like the rest: a tracker that will not answer must
// not cost the member the document. The sources are tried in order, so a first
// one with nothing recent falls through to the next.
func (h *DeviceHandler) exportSleep(r *http.Request, userID string) *models.SleepNight {
	connected, err := h.links.ListProviders(r.Context(), userID)
	if err != nil {
		return nil
	}
	linked := map[string]bool{}
	for _, name := range connected {
		linked[name] = true
	}
	for _, name := range devicelink.SleepSources(h.providers, linked) {
		found, ok := devicelink.Find(h.providers, name)
		if !ok {
			continue
		}
		reader, ok := found.(devicelink.SleepReader)
		if !ok {
			continue
		}
		link, err := h.links.Find(r.Context(), userID, name)
		if err != nil {
			continue
		}
		accessToken, err := h.freshToken(r, found, link)
		if err != nil {
			slog.Warn("health export: sleep unavailable", "provider", name, "error", err)
			continue
		}
		night, err := reader.LastNightSleep(r.Context(), accessToken)
		if err != nil {
			slog.Warn("health export: sleep unavailable", "provider", name, "error", err)
			continue
		}
		if night.Date == "" {
			continue
		}
		result := sleepNight(name, night)
		return &result
	}
	return nil
}

// readable resolves everything a read of a member's data needs: the provider,
// their link to it, and how many days were asked for. Shared by the two reads
// so they cannot drift on what a range means or on who may ask.
func (h *DeviceHandler) readable(w http.ResponseWriter, r *http.Request, userID string) (
	devicelink.ActivityProvider, models.DeviceLink, int, string, bool) {
	found, ok := h.provider(w, r)
	if !ok {
		return nil, models.DeviceLink{}, 0, "", false
	}
	provider, ok := found.(devicelink.ActivityProvider)
	if !ok {
		writeError(w, http.StatusNotFound, "This provider has no activity data", CodeNotFound)
		return nil, models.DeviceLink{}, 0, "", false
	}
	if !provider.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return nil, models.DeviceLink{}, 0, "", false
	}

	rangeName := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("range")))
	if rangeName == "" {
		rangeName = "week"
	}
	days, known := stepRanges[rangeName]
	if !known {
		writeError(w, http.StatusBadRequest, "Unknown range", CodeInvalidRange)
		return nil, models.DeviceLink{}, 0, "", false
	}
	if max := provider.MaxRangeDays(); days > max {
		days = max
	}

	link, err := h.links.Find(r.Context(), userID, provider.Name())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No account connected", CodeDeviceNotConnected)
			return nil, models.DeviceLink{}, 0, "", false
		}
		writeStoreError(w, err)
		return nil, models.DeviceLink{}, 0, "", false
	}
	return provider, link, days, rangeName, true
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
		// Not 401: a provider's expired grant must never read as the member's
		// own StrongFish session being gone, or the clients log the whole
		// account out (their 401 handler) when all that's needed is to
		// reconnect one connected device. 424 says "a dependency - the device
		// authorization - has to be re-established", which is exactly the case.
		writeError(w, http.StatusFailedDependency, "Connect the account again", CodeDeviceReauthorize)
	default:
		slog.Error("device provider call failed", "provider", provider, "error", err)
		writeJSON(w, http.StatusBadGateway, errorBody{
			Message:  "The provider could not be reached",
			I18nCode: CodeUpstream,
			Detail:   err.Error(),
			Provider: provider,
		})
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
