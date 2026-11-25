package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"strong-fish-api/internal/ai"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/utils"
)

// The assistant's tool: strong-fish's own HTTP API, called as the signed-in
// member over loopback. The model reads the member's real data to ground an
// answer, and - for the one thing the member asked the assistant to do for
// them - records a training session.
//
// READS are open: every GET the member may make, they may make through here.
// WRITES are not. They are allowlisted to recording a session (see writeRoutes)
// and gated by a preview → the member's own yes → execute handshake, because
// the house rule is that nothing writes a member's data without them saying so
// in their own words (see [[confirm-before-writes]]). A guessed POST cannot
// delete their history or rewrite their programme.
const (
	toolSfAPIName = "strong_fish_api"

	// apiToolTimeout bounds one self-call. Generous: reading a whole programme
	// takes a moment.
	apiToolTimeout = 30 * time.Second
	// apiToolMaxBytes caps how much of a response is read into memory.
	apiToolMaxBytes = 2 << 20
	// apiToolTextCap caps the text handed back to the model, so a long list
	// grounds an answer without eating the prompt budget.
	apiToolTextCap = 64 * 1024
	// confirmTTL is how long a write preview's confirmation token stays good.
	// Long enough for the member to read it back and answer, short enough that
	// a token cannot be sat on.
	confirmTTL = 10 * time.Minute
)

// apiToolClient is the HTTP client the tool calls this very API with. Its proxy
// is pinned off: the call is to the loopback self-base, and routing a self-call
// through the corporate proxy would hang and then fail.
var apiToolClient = &http.Client{
	Timeout:   apiToolTimeout,
	Transport: &http.Transport{Proxy: nil},
}

// writeRoutes is the allowlist: the only writes the assistant may drive, and
// all of them are recording a training session. Everything else a member does
// - editing a programme, deleting a log, resolving a report - stays their own
// button to press. Matched against the path with its ids, so a write to any
// other endpoint is refused before a request is built.
var writeRoutes = []*regexp.Regexp{
	regexp.MustCompile(`^/v1/training/[^/]+/sets/[^/]+/log$`), // log a performed set
	regexp.MustCompile(`^/v1/training/[^/]+/sets/[^/]+/vbt$`), // log a set's bar speed
	regexp.MustCompile(`^/v1/training/[^/]+/days/[^/]+/log$`), // mark a day done
	regexp.MustCompile(`^/v1/training/[^/]+/status$`),         // in progress / finished / archived
}

// writeAllowed reports whether a write to path may be driven by the assistant.
func writeAllowed(method, path string) bool {
	if method != http.MethodPost && method != http.MethodPut {
		return false
	}
	for _, re := range writeRoutes {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// tools is the catalogue offered to the model. Empty (so the request is a plain
// chat) when no loopback base is wired.
func (h *AIHandler) tools() []ai.Tool {
	if h.selfURL == "" {
		return nil
	}
	return []ai.Tool{{
		Name: toolSfAPIName,
		Description: "Call strong-fish's own HTTP API as the signed-in member, to ground your answer in " +
			"their real data and to record a training session for them. It runs with the member's OWN " +
			"permissions: someone not allowed gets a forbidden result, which you relay as \"you don't " +
			"have permission\" without retrying. " +
			"READS (method GET) are open: list their assigned programmes and sessions, read one with its " +
			"days and sets and logged work, their one-rep maxes, bar-speed history, profile and clubs. " +
			"WRITES (method POST or PUT) are limited to recording a session - logging a set, marking a " +
			"day done, setting a programme's status - and are two-step. Call first WITHOUT confirm to " +
			"preview: nothing is written and you get a confirm_token back. State to the member, in plain " +
			"words, exactly what would be recorded (every field) and ask them. Only after the member's " +
			"OWN next message says yes, call again with the SAME method, path and body plus confirm set " +
			"to that token. Never treat your own reasoning as the yes; never reuse a token for a " +
			"different write; a confirmation is good only in this conversation and only for that exact " +
			"write. If a call returns a file, point the member to the screen it belongs to - never paste " +
			"its contents.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"method": map[string]any{
					"type": "string", "enum": []string{"GET", "POST", "PUT"},
					"description": "The HTTP method: GET to read, POST or PUT to record a session. Defaults to GET.",
				},
				"path": map[string]any{
					"type": "string",
					"description": "The API path, starting with '/v1/'. It may carry a query string, e.g. " +
						"'/v1/training', '/v1/training/{assignmentId}', " +
						"'/v1/training/{assignmentId}/sets/{setId}/log'.",
				},
				"body": map[string]any{
					"type":        "object",
					"description": "The JSON body for a write (POST/PUT). Omit for a GET.",
				},
				"confirm": map[string]any{
					"type": "string",
					"description": "The confirm_token from this write's own preview, set ONLY on the " +
						"execute call after the member has said yes. Omit it to preview.",
				},
			},
			"required": []string{"path"},
		},
	}}
}

// runTool executes one tool call and returns its result as a JSON string. It
// never returns an error: a failed tool reports the failure to the model as
// data, so the agent loop and the chat carry on.
func (h *AIHandler) runTool(r *http.Request, call ai.ToolCall) string {
	switch call.Name {
	case toolSfAPIName:
		var args struct {
			Method  string          `json:"method"`
			Path    string          `json:"path"`
			Body    json.RawMessage `json:"body"`
			Confirm string          `json:"confirm"`
		}
		_ = json.Unmarshal(call.Args, &args)
		return h.toolSfAPI(r, args.Method, args.Path, args.Body, args.Confirm)
	default:
		return aiToolErrorJSON("unknown tool")
	}
}

// toolSfAPI performs one call against strong-fish's own API as the signed-in
// member. The method is GET for a read, or POST/PUT for an allowlisted,
// confirmed write.
//
// Three things are pinned rather than trusted:
//   - the path must be relative and under /v1/, so the member's token can never
//     be pointed at another host;
//   - a write must be on the allowlist, so a guessed POST cannot reach an
//     endpoint recording a session was never meant to touch;
//   - a write must carry a confirmation token tying it to this member and this
//     exact body, so a write the member did not confirm is refused, not run.
func (h *AIHandler) toolSfAPI(r *http.Request, method, path string, body json.RawMessage, confirm string) string {
	if h.selfURL == "" {
		return aiToolJSON(map[string]any{"error": "the strong-fish API is not reachable from here"})
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}

	// A server-relative path under /v1/ only. Anything that could reach off this
	// API - an absolute URL, a scheme, a protocol-relative //host, an opaque
	// reference - is refused.
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return aiToolJSON(map[string]any{
			"error": "path must be an absolute path on this API, e.g. /v1/training",
		})
	}
	parsed, err := url.Parse(path)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Opaque != "" {
		return aiToolJSON(map[string]any{"error": "path must be a relative API path, not a full URL"})
	}
	if !strings.HasPrefix(parsed.EscapedPath(), "/v1/") {
		return aiToolJSON(map[string]any{"error": "path must start with /v1/"})
	}
	cleanPath := parsed.EscapedPath()
	requestURI := cleanPath
	if parsed.RawQuery != "" {
		requestURI += "?" + parsed.RawQuery
	}

	switch method {
	case http.MethodGet:
		return h.selfCall(r, http.MethodGet, requestURI, nil)
	case http.MethodPost, http.MethodPut:
		return h.toolWrite(r, method, cleanPath, requestURI, body, confirm)
	default:
		return aiToolJSON(map[string]any{"error": "method must be GET, POST or PUT"})
	}
}

// toolWrite handles the preview → confirm → execute handshake for a write.
func (h *AIHandler) toolWrite(r *http.Request, method, cleanPath, requestURI string,
	body json.RawMessage, confirm string) string {
	if !writeAllowed(method, cleanPath) {
		return aiToolJSON(map[string]any{
			"error": "that write is not available through the assistant",
			"note": "only recording a training session is: logging a set " +
				"(/v1/training/{id}/sets/{setId}/log or .../vbt), marking a day done " +
				"(/v1/training/{id}/days/{dayId}/log), or setting a programme's status " +
				"(/v1/training/{id}/status). Ask the member to do anything else from the app.",
		})
	}

	userID, _ := middleware.UserIDFromContext(r.Context())
	canonicalBody := canonicalJSON(body)

	if strings.TrimSpace(confirm) == "" {
		// Preview: mint a token tying this member to this exact write, and write
		// nothing. The model must read it back to the member and get a yes.
		token, ok := h.confirms.issue(userID, method, cleanPath, canonicalBody)
		if !ok {
			return aiToolJSON(map[string]any{"error": "could not prepare the confirmation; try again"})
		}
		return aiToolJSON(map[string]any{
			"preview":       true,
			"would_write":   map[string]any{"method": method, "path": cleanPath, "body": json.RawMessage(canonicalBody)},
			"confirm_token": token,
			"note": "NOTHING has been written. State to the member, in plain words, exactly what this " +
				"would record - the action and every field - and ask them to confirm. Only after the " +
				"member's OWN next message says yes, call this tool again with the SAME method, path and " +
				"body plus confirm set to this confirm_token. The confirmation is valid only in this " +
				"conversation and only for this exact write.",
		})
	}

	if !h.confirms.consume(confirm, userID, method, cleanPath, canonicalBody) {
		return aiToolJSON(map[string]any{
			"error": "the confirmation is missing, expired, already used, or does not match this exact write",
			"note": "do not retry blindly: preview the write again (call without confirm), show the " +
				"member what it would record, and use the new confirm_token only after they say yes.",
		})
	}

	return h.selfCall(r, method, requestURI, []byte(canonicalBody))
}

// selfCall makes the loopback request as the signed-in member and returns the
// response as a JSON string the model reads. It forwards whatever credential
// authenticated the chat, so the API's own guards decide what comes back; a
// 401/403 is handed to the model as data rather than retried.
func (h *AIHandler) selfCall(r *http.Request, method, requestURI string, body []byte) string {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method,
		strings.TrimRight(h.selfURL, "/")+requestURI, reader)
	if err != nil {
		return aiToolJSON(map[string]any{"error": "could not build the request"})
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Forward the caller's own credential; the self-call then acts as exactly
	// this member and nobody else.
	if key := r.Header.Get("X-Api-Key"); key != "" {
		req.Header.Set("X-Api-Key", key)
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	// Keep the correlation id, so the chat turn and the call it made share one
	// line of enquiry in the log.
	if cid := middleware.CIDFromContext(r.Context()); cid != "" {
		req.Header.Set("X-Cid", cid)
	}
	req.Header.Set("Accept", "application/json, text/plain")

	resp, err := apiToolClient.Do(req)
	if err != nil {
		return aiToolJSON(map[string]any{"error": "the strong-fish API could not be reached"})
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return aiToolJSON(map[string]any{
			"status": resp.StatusCode, "forbidden": true,
			"note": "the signed-in member is not allowed to do this; tell them they do not have " +
				"permission for it and do not retry",
		})
	}

	contentType := resp.Header.Get("Content-Type")
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, int64(apiToolMaxBytes)))
	if !aiLooksTextual(contentType) || !utf8.Valid(raw) {
		// A binary body (a PDF/PNG export) is not put in the prompt.
		return aiToolJSON(map[string]any{
			"status": resp.StatusCode, "contentType": contentType,
			"note": "a file was produced; its contents are not shown here. Point the member to the " +
				"screen it belongs to rather than pasting it.",
		})
	}
	text, truncated := aiCapText(raw)
	return aiToolJSON(map[string]any{
		"status": resp.StatusCode, "contentType": contentType, "body": text, "truncated": truncated,
	})
}

// --- the write-confirmation store ---

// confirmation is one outstanding write preview: who may run it, which write
// exactly, and until when.
type confirmation struct {
	userID   string
	method   string
	path     string
	bodyHash string
	expires  time.Time
}

// confirmStore holds the short-lived tokens write previews mint. Tokens are
// single-use and tied to a member, a method, a path and the body's hash, so a
// token cannot run a different write, run twice, or run as someone else.
type confirmStore struct {
	mu   sync.Mutex
	byID map[string]confirmation
}

func newConfirmStore() *confirmStore {
	return &confirmStore{byID: map[string]confirmation{}}
}

// issue records a pending write and returns its token, or ok=false when a token
// could not be minted.
func (s *confirmStore) issue(userID, method, path, canonicalBody string) (string, bool) {
	token, err := utils.RandomHex(16)
	if err != nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	s.byID[token] = confirmation{
		userID: userID, method: method, path: path,
		bodyHash: hashBody(canonicalBody), expires: time.Now().Add(confirmTTL),
	}
	return token, true
}

// consume checks a token against the exact write it was minted for and, when it
// matches, spends it. A match is single-use (the token is removed); a mismatch
// leaves it live, so a fat-fingered attempt does not burn a confirmation the
// member did give - the body hash still blocks a wrong body from ever
// succeeding, so leaving it costs no safety.
func (s *confirmStore) consume(token, userID, method, path, canonicalBody string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	c, ok := s.byID[token]
	if !ok {
		return false
	}
	matches := c.userID == userID && c.method == method && c.path == path &&
		c.bodyHash == hashBody(canonicalBody) && time.Now().Before(c.expires)
	if matches {
		delete(s.byID, token)
	}
	return matches
}

// prune drops expired tokens. Called under the lock on every issue/consume, so
// the map never grows past what is live - there is no background sweeper to
// outlive the process.
func (s *confirmStore) prune() {
	now := time.Now()
	for token, c := range s.byID {
		if now.After(c.expires) {
			delete(s.byID, token)
		}
	}
}

func hashBody(canonicalBody string) string {
	sum := sha256.Sum256([]byte(canonicalBody))
	return hex.EncodeToString(sum[:])
}

// canonicalJSON re-marshals a body object so the preview and the execute hash
// the same bytes even if the model reorders the fields; a missing or invalid
// body is the empty object.
func canonicalJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "{}"
	}
	out, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(out)
}

// --- small result helpers ---

// aiLooksTextual is true for the content types whose body is worth returning
// verbatim (JSON, plain text, CSV, XML).
func aiLooksTextual(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "json") || strings.Contains(ct, "text") ||
		strings.Contains(ct, "csv") || strings.Contains(ct, "xml")
}

// aiCapText bounds one text field handed to the model, flagging when it was cut
// - a silently truncated list is how a model concludes "there are only twelve".
func aiCapText(b []byte) (string, bool) {
	if len(b) > apiToolTextCap {
		return string(b[:apiToolTextCap]), true
	}
	return string(b), false
}

// aiToolJSON encodes a tool result; a marshalling failure still comes back as
// JSON the model can read.
func aiToolJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return aiToolErrorJSON("could not encode the tool result")
	}
	return string(encoded)
}

// aiToolErrorJSON is a tool's failure, reported to the model as data rather
// than raised as an error - a bad tool call must not break the chat.
func aiToolErrorJSON(message string) string {
	encoded, _ := json.Marshal(map[string]string{"error": message})
	return string(encoded)
}
