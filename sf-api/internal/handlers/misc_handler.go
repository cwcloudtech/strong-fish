package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"strong-fish-api/internal/assets"
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
}

func NewConfigHandler(oidcProviders []string, activationMode string, plateIncrement float64, maxImageSize int64, version string) *ConfigHandler {
	return &ConfigHandler{
		oidcProviders: oidcProviders, activationMode: activationMode,
		plateIncrement: plateIncrement, maxImageSize: maxImageSize, version: version,
	}
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
	})
}
