package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"strong-fish-api/internal/assets"
	"strong-fish-api/internal/utils"
)

// Health is the liveness probe.
func Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// NewManifestHandler serves the build manifest (the app version), which the
// mobile app polls to decide whether an update is available.
func NewManifestHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{"version": "1.0.0"})
			return
		}
		var manifest map[string]any
		if err := json.Unmarshal(data, &manifest); err != nil {
			writeJSON(w, http.StatusOK, map[string]string{"version": "1.0.0"})
			return
		}
		writeJSON(w, http.StatusOK, manifest)
	}
}

// AssetsLogo serves the bundled logo as a real image over HTTP, so transactional
// emails can reference it by URL instead of embedding it as a data: URI - mail
// clients and email-sending APIs commonly strip those from <img src> outright,
// a limitation no amount of correctly escaping the source HTML can work around.
// No auth: it needs to be fetchable by a mail client with no credentials, and
// it's just a logo.
func AssetsLogo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(assets.LogoPNG)
}

// ConfigHandler tells the frontend what this deployment supports, so a single
// build works against any configuration: which OIDC providers are enabled, how
// accounts activate, and the plate increment loads are rounded to.
type ConfigHandler struct {
	oidcProviders  []string
	activationMode string
	plateIncrement float64
	maxImageSize   int64
	version        string
	contactEnabled bool
	apiBaseURL     string
	uiBaseURL      string
	mobileURLPat   string
}

func NewConfigHandler(oidcProviders []string, activationMode string, plateIncrement float64,
	maxImageSize int64, version string, contactEnabled bool,
	apiBaseURL, uiBaseURL, mobileURLPattern string) *ConfigHandler {
	return &ConfigHandler{
		oidcProviders: oidcProviders, activationMode: activationMode,
		plateIncrement: plateIncrement, maxImageSize: maxImageSize, version: version,
		contactEnabled: contactEnabled, apiBaseURL: apiBaseURL, uiBaseURL: uiBaseURL,
		mobileURLPat: mobileURLPattern,
	}
}

// mobileAppURL is where this deployment's Android build can be downloaded:
// the configured pattern with the running version substituted in. A pattern
// given as a path is resolved against the frontend's own origin, because the
// URL also has to survive being encoded into a QR code and scanned on a phone
// that has no idea what "/strong-fish-v1.0.0.apk" is relative to.
func (h *ConfigHandler) mobileAppURL() string {
	if utils.IsBlank(h.mobileURLPat) {
		return utils.EMPTY
	}
	url := strings.ReplaceAll(h.mobileURLPat, "{version}", h.version)
	if strings.HasPrefix(url, "/") {
		url = strings.TrimSuffix(h.uiBaseURL, "/") + url
	}
	return url
}

// MobileApp is public: the download link and a QR code of it are what an
// anonymous visitor on a desktop uses to get the app onto their phone, and
// requiring a session to see them would defeat the point.
func (h *ConfigHandler) MobileApp(w http.ResponseWriter, r *http.Request) {
	url := h.mobileAppURL()
	if utils.IsBlank(url) {
		writeError(w, http.StatusNotFound, "No mobile build is published for this deployment", CodeNotFound)
		return
	}

	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"url":       url,
		"version":   h.version,
		"qrCodePng": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

// clientConfigPayload carries the plaintext API key token to format, in the
// POST body rather than a header: a custom request header would force a CORS
// exception on reverse proxies that a plain JSON POST doesn't need.
//
// The token has to come from the caller because it cannot come from anywhere
// else - the store only ever keeps its sha256 (see models.ApiKey), so the
// frontend, holding the response to the create call it just made, is the sole
// possible source. These endpoints format a token, they never look one up.
type clientConfigPayload struct {
	Key string `json:"key"`
}

// buildClientConfig renders the config a CLI reads and the mobile app scans.
// The two are deliberately the same text, so one QR code enrolls either.
func (h *ConfigHandler) buildClientConfig(token string) string {
	return fmt.Sprintf("api_url = %s\napi_key = %s\n", h.apiBaseURL, token)
}

func (h *ConfigHandler) clientConfigText(w http.ResponseWriter, r *http.Request) (string, bool) {
	var p clientConfigPayload
	if !decodeJSON(w, r, &p) {
		return utils.EMPTY, false
	}
	if utils.IsBlank(p.Key) {
		writeError(w, http.StatusBadRequest, "Missing API key token", CodeConfigTokenRequired)
		return utils.EMPTY, false
	}
	return h.buildClientConfig(p.Key), true
}

// ClientConfigFile returns the config as a downloadable file, for a CLI to
// read as-is.
func (h *ConfigHandler) ClientConfigFile(w http.ResponseWriter, r *http.Request) {
	text, ok := h.clientConfigText(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="strong-fish.conf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

// ClientConfigQR renders the same text into a QR code, base64-embedded the way
// MFAHandler.TOTPSetup ships its enrollment code. Scanning it in the mobile
// app is what signs that device in.
func (h *ConfigHandler) ClientConfigQR(w http.ResponseWriter, r *http.Request) {
	text, ok := h.clientConfigText(w, r)
	if !ok {
		return
	}

	png, err := qrcode.Encode(text, qrcode.Medium, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"qrCodePng": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	providers := h.oidcProviders
	if providers == nil {
		providers = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"oidcProviders":  providers,
		"activationMode": h.activationMode,
		"plateIncrement": h.plateIncrement,
		"maxImageSize":   h.maxImageSize,
		"version":        h.version,
		"locales":        []string{"en", "fr"},
		// Lets the frontend hide the contact link entirely when no form id is
		// configured, rather than offering a page that can only answer 405.
		"contactEnabled": h.contactEnabled,
	})
}
