package handlers

import (
	"errors"
	"net/http"

	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/stance"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// Pairing and reading an account-paired bar-speed provider (Stance).
//
// Unlike Google Health and Withings, Stance has no consent page to send a
// browser to: it is paired with the member's own email and password, which
// are exchanged once for a Supabase grant and never stored. So it gets its own
// pairing path here, alongside the OAuth Authorize/Callback the others use.

type pairPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// PairDevice connects an account-paired provider from an email and password.
//
// The password is used exactly once - to get the grant - and is never written
// down; only the tokens are, the same rule every provider follows. A provider
// that is not an AccountPairer (the OAuth ones) is refused here rather than
// silently doing nothing.
func (h *DeviceHandler) PairDevice(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	found, ok := h.provider(w, r)
	if !ok {
		return
	}
	pairer, ok := found.(devicelink.AccountPairer)
	if !ok {
		writeError(w, http.StatusBadRequest,
			"This provider is connected from its own site, not with an account",
			CodeDeviceNotPairable)
		return
	}
	if !found.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	var p pairPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Email) || utils.IsBlank(p.Password) {
		writeError(w, http.StatusBadRequest, "Enter your email and password", CodeStancePairFailed)
		return
	}

	token, err := pairer.PairAccount(r.Context(), p.Email, p.Password)
	if err != nil {
		if errors.Is(err, stance.ErrRefused) {
			// A bad email or password - theirs to fix, not a server fault.
			writeError(w, http.StatusUnauthorized, "Those credentials were refused", CodeStancePairFailed)
			return
		}
		h.writeProviderError(w, err, found.Name())
		return
	}

	if _, err := h.links.Save(r.Context(), models.DeviceLink{
		UserID: userID, Provider: found.Name(),
		AccessToken: token.AccessToken, RefreshToken: token.RefreshToken,
		ExpiresAt: token.ExpiresAt, ExternalUserID: token.UserID,
	}); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type googlePairPayload struct {
	IDToken string `json:"idToken"`
}

// PairDeviceGoogle connects an account-paired provider from a Google ID token
// the client obtained with the public client id. Like PairDevice, only the
// resulting session is stored, never the token the client sent.
func (h *DeviceHandler) PairDeviceGoogle(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	found, ok := h.provider(w, r)
	if !ok {
		return
	}
	pairer, ok := found.(devicelink.GoogleAccountPairer)
	if !ok || pairer.GoogleClientID() == "" {
		writeError(w, http.StatusBadRequest,
			"This provider cannot be connected with Google", CodeDeviceNotPairable)
		return
	}
	if !found.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	var p googlePairPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.IDToken) {
		writeError(w, http.StatusBadRequest, "Missing Google sign-in token", CodeStancePairFailed)
		return
	}

	token, err := pairer.PairGoogleIDToken(r.Context(), p.IDToken)
	if err != nil {
		if errors.Is(err, stance.ErrRefused) {
			writeError(w, http.StatusUnauthorized, "That Google sign-in was refused", CodeStancePairFailed)
			return
		}
		h.writeProviderError(w, err, found.Name())
		return
	}

	if _, err := h.links.Save(r.Context(), models.DeviceLink{
		UserID: userID, Provider: found.Name(),
		AccessToken: token.AccessToken, RefreshToken: token.RefreshToken,
		ExpiresAt: token.ExpiresAt, ExternalUserID: token.UserID,
	}); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type keyPairPayload struct {
	ApiKey string `json:"apiKey"`
}

// PairDeviceKey connects a key-paired provider (Garmin) from the member's own
// personal API key. The key is exchanged for a grant and kept only as the
// stored refresh credential, never echoed back.
func (h *DeviceHandler) PairDeviceKey(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	found, ok := h.provider(w, r)
	if !ok {
		return
	}
	pairer, ok := found.(devicelink.KeyPairer)
	if !ok {
		writeError(w, http.StatusBadRequest,
			"This provider is not connected with a key", CodeDeviceNotPairable)
		return
	}
	if !found.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	var p keyPairPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.ApiKey) {
		writeError(w, http.StatusBadRequest, "Enter your personal API key", CodeStancePairFailed)
		return
	}

	token, err := pairer.PairKey(r.Context(), p.ApiKey)
	if err != nil {
		if errors.Is(err, devicelink.ErrReauthorize) {
			writeError(w, http.StatusUnauthorized, "That key was refused", CodeStancePairFailed)
			return
		}
		h.writeProviderError(w, err, found.Name())
		return
	}

	if _, err := h.links.Save(r.Context(), models.DeviceLink{
		UserID: userID, Provider: found.Name(),
		AccessToken: token.AccessToken, RefreshToken: token.RefreshToken,
		ExpiresAt: token.ExpiresAt, ExternalUserID: token.UserID,
	}); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BarSpeed returns the member's last set of each tracked lift.
func (h *DeviceHandler) BarSpeed(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	found, ok := h.provider(w, r)
	if !ok {
		return
	}
	reader, ok := found.(devicelink.BarSpeedReader)
	if !ok {
		writeError(w, http.StatusNotFound, "This provider has no bar-speed data", CodeNotFound)
		return
	}
	if !found.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	sets, err := h.barSets(r, found, reader, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No account connected", CodeDeviceNotConnected)
			return
		}
		h.writeProviderError(w, err, found.Name())
		return
	}
	writeJSON(w, http.StatusOK, sets)
}

// barSets resolves the grant and reads the last set of each lift, as the API
// model. Shared by the endpoint above and the health export, so a sheet and a
// screen never disagree about what the last set was.
func (h *DeviceHandler) barSets(r *http.Request, provider devicelink.Provider,
	reader devicelink.BarSpeedReader, userID string) ([]models.BarSet, error) {
	link, err := h.links.Find(r.Context(), userID, provider.Name())
	if err != nil {
		return nil, err
	}
	accessToken, err := h.freshToken(r, provider, link)
	if err != nil {
		return nil, err
	}

	sets, err := reader.LastSets(r.Context(), accessToken, link.ExternalUserID)
	if err != nil {
		return nil, err
	}

	out := make([]models.BarSet, 0, len(sets))
	for _, set := range sets {
		out = append(out, models.BarSet{
			Lift: set.Lift, WeightKg: set.WeightKg, Reps: set.Reps,
			GivenRpe: set.GivenRPE, PredictedRpe: set.PredictedRPE,
			MeanVelocity: set.MeanVelocity, PeakVelocity: set.PeakVelocity,
			RepVelocities: set.RepVelocities, RepPeakVelocities: set.RepPeakVelocities,
			PerformedAt: set.PerformedAt,
		})
	}
	return out, nil
}

// barSpeedForExport is the member's last sets for the health sheet, or nil
// when they track no bar speed. Best-effort: a sheet is not worth failing
// because a third-party read was slow, so any error is swallowed and the
// section is simply left out.
func (h *DeviceHandler) barSpeedForExport(r *http.Request, userID string) []models.BarSet {
	for _, provider := range h.providers {
		reader, ok := provider.(devicelink.BarSpeedReader)
		if !ok || !provider.Enabled() {
			continue
		}
		if _, err := h.links.Find(r.Context(), userID, provider.Name()); err != nil {
			continue
		}
		if sets, err := h.barSets(r, provider, reader, userID); err == nil {
			return sets
		}
	}
	return nil
}
