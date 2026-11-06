package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
)

// CallbackHandler is a member's recording callbacks: the triggers their phone
// fires when a bar-speed set starts, so a device of their own - an ESP32 - can
// record it. A webhook (HTTP POST) is one transport; a WebSocket message and an
// MQTT publish are the others.
//
// The firing happens on the phone, not here: the destination is usually a
// device on the member's local network, which the API cannot reach. This
// handler only keeps the list the phone reads and fires against.
type CallbackHandler struct {
	callbacks *store.CallbackStore
}

func NewCallbackHandler(callbacks *store.CallbackStore) *CallbackHandler {
	return &CallbackHandler{callbacks: callbacks}
}

type callbackPayload struct {
	Name     string                  `json:"name"`
	Type     string                  `json:"type"`
	URL      string                  `json:"url"`
	Token    string                  `json:"token"`
	Headers  []models.CallbackHeader `json:"headers"`
	Username string                  `json:"username"`
	Password string                  `json:"password"`
	Topic    string                  `json:"topic"`
	ClientID string                  `json:"clientId"`
	QoS      int                     `json:"qos"`
	// A pointer so a create that leaves it out defaults to on, while an update
	// can switch a device off with an explicit false.
	Enabled *bool `json:"enabled"`
}

// urlSchemes is the set of URL prefixes each transport will accept. http makes
// the request on the member's behalf, so a file or internal scheme is refused;
// the mqtt broker may be reached over tcp or websockets.
var urlSchemes = map[string][]string{
	models.CallbackTypeHTTP:      {"https://", "http://"},
	models.CallbackTypeWebSocket: {"wss://", "ws://"},
	models.CallbackTypeMQTT:      {"mqtts://", "mqtt://", "wss://", "ws://", "ssl://", "tcp://"},
}

// normalize fills in what the member left blank and refuses what cannot work.
func (h *CallbackHandler) normalize(w http.ResponseWriter, p callbackPayload) (models.RecordingCallback, bool) {
	callbackType := strings.ToLower(strings.TrimSpace(p.Type))
	if callbackType == "" {
		callbackType = models.DefaultCallbackType
	}
	if !models.IsValidCallbackType(callbackType) {
		writeError(w, http.StatusBadRequest,
			"The transport must be http, websocket or mqtt", CodeInvalidCallbackType)
		return models.RecordingCallback{}, false
	}

	endpoint := strings.TrimSpace(p.URL)
	if endpoint == "" {
		writeError(w, http.StatusBadRequest, "A URL is required", CodeCallbackURLRequired)
		return models.RecordingCallback{}, false
	}
	if !hasAnyPrefix(endpoint, urlSchemes[callbackType]) {
		writeError(w, http.StatusBadRequest,
			"The URL scheme does not match the transport", CodeInvalidCallbackURL)
		return models.RecordingCallback{}, false
	}

	// An MQTT publish needs somewhere to go: a broker with no topic is a
	// connection that delivers nothing.
	topic := strings.TrimSpace(p.Topic)
	if callbackType == models.CallbackTypeMQTT && topic == "" {
		writeError(w, http.StatusBadRequest, "An MQTT topic is required", CodeCallbackTopicRequired)
		return models.RecordingCallback{}, false
	}

	// A callback nobody named is named after where it points, so a list of two
	// reads as two devices rather than two blanks.
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = defaultCallbackName(endpoint)
	}

	qos := p.QoS
	if qos < 0 {
		qos = 0
	}
	if qos > 2 {
		qos = 2
	}

	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}

	return models.RecordingCallback{
		Name: name, Type: callbackType, URL: endpoint,
		Token: strings.TrimSpace(p.Token), Headers: cleanHeaders(p.Headers),
		Username: strings.TrimSpace(p.Username), Password: strings.TrimSpace(p.Password),
		Topic: topic, ClientID: strings.TrimSpace(p.ClientID), QoS: qos,
		Enabled: enabled,
	}, true
}

// cleanHeaders drops the blank rows a form leaves behind and trims the rest, so
// a header with no name never reaches a device as ": value".
func cleanHeaders(headers []models.CallbackHeader) []models.CallbackHeader {
	cleaned := make([]models.CallbackHeader, 0, len(headers))
	for _, header := range headers {
		name := strings.TrimSpace(header.Name)
		if name == "" {
			continue
		}
		cleaned = append(cleaned, models.CallbackHeader{Name: name, Value: strings.TrimSpace(header.Value)})
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// defaultCallbackName is the endpoint's host, or "Callback" when it has none -
// enough to tell two devices apart in a list.
func defaultCallbackName(endpoint string) string {
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return "Callback"
}

// List is every callback the caller has, secrets included - the phone needs
// them to fire, and a member's callbacks are nobody's but their own.
func (h *CallbackHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	callbacks, err := h.callbacks.ListFor(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, callbacks)
}

func (h *CallbackHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p callbackPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	callback, ok := h.normalize(w, p)
	if !ok {
		return
	}

	created, err := h.callbacks.Create(r.Context(), userID, callback)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *CallbackHandler) Update(w http.ResponseWriter, r *http.Request) {
	owned, ok := h.ownCallback(w, r)
	if !ok {
		return
	}

	var p callbackPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	callback, ok := h.normalize(w, p)
	if !ok {
		return
	}

	updated, err := h.callbacks.Update(r.Context(), owned.ID, callback)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *CallbackHandler) Delete(w http.ResponseWriter, r *http.Request) {
	owned, ok := h.ownCallback(w, r)
	if !ok {
		return
	}
	if err := h.callbacks.Delete(r.Context(), owned.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ownCallback resolves the addressed callback and refuses anybody but its owner.
func (h *CallbackHandler) ownCallback(w http.ResponseWriter, r *http.Request) (models.RecordingCallback, bool) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	callback, err := h.callbacks.FindByID(r.Context(), chi.URLParam(r, "callbackId"))
	if err != nil {
		writeStoreError(w, err)
		return models.RecordingCallback{}, false
	}
	if callback.OwnerID != userID {
		// Not "forbidden": whether somebody else's callback exists is not a
		// question this answers.
		writeError(w, http.StatusNotFound, "Callback not found", CodeNotFound)
		return models.RecordingCallback{}, false
	}
	return callback, true
}
