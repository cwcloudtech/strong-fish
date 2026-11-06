package models

import "time"

// CallbackHeader is one custom header sent on an HTTP or WebSocket callback -
// the shape comwork-cloud's monitors carry, a name and a value.
type CallbackHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RecordingCallback is a member's own trigger that the phone fires when they
// start (and stop) a bar-speed set, so a device of their own - an ESP32 on the
// gym network - records the same set. The payload carries the lift and the
// load, the way a brand's app signals its sensor.
//
// Named after comwork-cloud's "callback": a webhook (an HTTP POST) is one
// transport of it, alongside a WebSocket message and an MQTT publish. Whichever
// the transport, the body is the same JSON; only how it is carried differs.
//
// Both secrets (Token, Password) travel back to their owner, unlike an AI key:
// the phone fires the callback and needs them, and a callback is the member's
// own device, shared with no one.
type RecordingCallback struct {
	ID      string `json:"id"`
	OwnerID string `json:"ownerId"`
	Name    string `json:"name"`
	// Type is the transport: "http" (a webhook), "websocket" or "mqtt".
	Type string `json:"type"`
	// URL is the destination: an http(s):// endpoint, a ws(s):// URL, or an
	// mqtt broker (mqtt://, mqtts://, ws://, wss://).
	URL string `json:"url"`

	// Token, when set, is the Authorization header on an http or websocket
	// callback - a write-through secret (blank on an edit keeps the stored one).
	Token    string `json:"token,omitempty"`
	HasToken bool   `json:"hasToken"`
	// Headers are extra request headers for an http or websocket callback.
	Headers []CallbackHeader `json:"headers,omitempty"`

	// Username/Password are basic auth on an http callback, or the broker
	// credentials on an mqtt one. Password is a write-through secret.
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	HasPassword bool   `json:"hasPassword"`

	// Topic, ClientID and QoS are the mqtt specifics: where to publish, the
	// client name to connect as, and the delivery guarantee (0, 1 or 2).
	Topic    string `json:"topic,omitempty"`
	ClientID string `json:"clientId,omitempty"`
	QoS      int    `json:"qos,omitempty"`

	// Enabled is whether this callback fires. A member can keep a device
	// configured but switched off.
	Enabled bool `json:"enabled"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// The transports a callback can use. http is a plain POST (a webhook);
// websocket opens a connection and sends the body once; mqtt publishes the body
// to a topic.
const (
	CallbackTypeHTTP      = "http"
	CallbackTypeWebSocket = "websocket"
	CallbackTypeMQTT      = "mqtt"
)

// DefaultCallbackType is what a callback with no type set becomes.
const DefaultCallbackType = CallbackTypeHTTP

// IsValidCallbackType reports whether a transport is one this build can fire.
func IsValidCallbackType(t string) bool {
	switch t {
	case CallbackTypeHTTP, CallbackTypeWebSocket, CallbackTypeMQTT:
		return true
	}
	return false
}
