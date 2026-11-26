package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"strong-fish-api/internal/ai"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/templates"
)

// TestWriteAllowed pins the write allowlist: only the session-recording
// endpoints, and only under POST/PUT.
func TestWriteAllowed(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{"PUT", "/v1/training/a1/sets/s1/log", true},
		{"PUT", "/v1/training/a1/sets/s1/vbt", true},
		{"PUT", "/v1/training/a1/days/d1/log", true},
		{"PUT", "/v1/training/a1/status", true},
		{"POST", "/v1/training/a1/status", true},
		{"GET", "/v1/training/a1/status", false},  // a read is not a write
		{"PUT", "/v1/training/a1/sets/s1", false}, // not the log sub-path
		{"DELETE", "/v1/training/a1/sets/s1/log", false},
		{"PUT", "/v1/training/a1/days/d1/feedback", false},
		{"PUT", "/v1/users/me/profile", false}, // not a session recording
		{"PUT", "/v1/social/reports/r1", false},
	}
	for _, tc := range cases {
		if got := writeAllowed(tc.method, tc.path); got != tc.want {
			t.Errorf("writeAllowed(%q, %q) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

// recordingAPI is a stand-in for strong-fish's own API on the loopback base: it
// records what the tool sent and answers 200 with a little JSON.
type recordingAPI struct {
	hits   int
	method string
	path   string
	auth   string
	apiKey string
	body   string
}

func newToolHandler(t *testing.T) (*AIHandler, *recordingAPI, func()) {
	t.Helper()
	rec := &recordingAPI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.hits++
		rec.method = r.Method
		rec.path = r.URL.RequestURI()
		rec.auth = r.Header.Get("Authorization")
		rec.apiKey = r.Header.Get("X-Api-Key")
		raw, _ := io.ReadAll(r.Body)
		rec.body = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	h := NewAIHandler(nil, nil, nil, "").WithLoopback(server.URL)
	return h, rec, server.Close
}

// chatRequest is a chat request carrying the member's own credential, the way a
// signed-in call arrives (the userID lives in a middleware-package context key
// this test cannot set, so the confirm flow runs with a consistent empty id;
// the per-member property is covered by TestConfirmStore).
func chatRequest() *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat", nil)
	req.Header.Set("Authorization", "Bearer member-token")
	req.Header.Set("X-Api-Key", "member-key")
	return req
}

func parseToolResult(t *testing.T, result string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("tool result is not JSON: %q (%v)", result, err)
	}
	return out
}

// TestToolSfAPIRejectsUnsafePaths: the path guard and the write allowlist both
// refuse before any request leaves the process.
func TestToolSfAPIRejectsUnsafePaths(t *testing.T) {
	h, rec, done := newToolHandler(t)
	defer done()
	req := chatRequest()

	cases := []struct {
		name, method, path string
	}{
		{"absolute URL", "GET", "https://evil.example.com/v1/training"},
		{"protocol-relative host", "GET", "//evil.example.com/v1/training"},
		{"outside /v1", "GET", "/internal/secrets"},
		{"not an absolute path", "GET", "v1/training"},
		{"write not on the allowlist", "PUT", "/v1/users/me/profile"},
		{"delete is never a write", "DELETE", "/v1/training/a1/sets/s1/log"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := h.toolSfAPI(req, tc.method, tc.path, nil, "")
			if out := parseToolResult(t, result); out["error"] == nil {
				t.Errorf("expected an error result for %s %s, got %v", tc.method, tc.path, out)
			}
		})
	}
	if rec.hits != 0 {
		t.Errorf("a refused call still reached the API %d time(s)", rec.hits)
	}
}

// TestToolSfAPIForwardsCaller: a read runs as the member - their own
// Authorization and X-Api-Key travel on the self-call.
func TestToolSfAPIForwardsCaller(t *testing.T) {
	h, rec, done := newToolHandler(t)
	defer done()

	result := h.toolSfAPI(chatRequest(), "GET", "/v1/training?status=active", nil, "")
	out := parseToolResult(t, result)
	if out["status"] != float64(http.StatusOK) {
		t.Fatalf("expected status 200 in the result, got %v", out["status"])
	}
	if rec.hits != 1 {
		t.Fatalf("expected exactly one self-call, got %d", rec.hits)
	}
	if rec.method != http.MethodGet {
		t.Errorf("method = %q, want GET", rec.method)
	}
	if rec.path != "/v1/training?status=active" {
		t.Errorf("path = %q, want /v1/training?status=active", rec.path)
	}
	if rec.auth != "Bearer member-token" {
		t.Errorf("Authorization not forwarded: %q", rec.auth)
	}
	if rec.apiKey != "member-key" {
		t.Errorf("X-Api-Key not forwarded: %q", rec.apiKey)
	}
}

// TestToolSfAPIWriteConfirmation: a write previews (and writes nothing) without
// a confirmation, executes with the right one, and is refused when the token
// does not match this exact write or is reused.
func TestToolSfAPIWriteConfirmation(t *testing.T) {
	h, rec, done := newToolHandler(t)
	defer done()
	req := chatRequest()

	const path = "/v1/training/a1/sets/s1/log"
	body := json.RawMessage(`{"reps":5,"load":100}`)

	// 1. Preview: no confirm → nothing is written, a token comes back.
	preview := parseToolResult(t, h.toolSfAPI(req, "PUT", path, body, ""))
	if preview["preview"] != true {
		t.Fatalf("expected a preview, got %v", preview)
	}
	token, _ := preview["confirm_token"].(string)
	if token == "" {
		t.Fatal("preview returned no confirm_token")
	}
	if rec.hits != 0 {
		t.Fatalf("a preview wrote to the API (%d hit(s))", rec.hits)
	}

	// 2. A tampered body with that token is refused (hash mismatch).
	tampered := parseToolResult(t, h.toolSfAPI(req, "PUT", path, json.RawMessage(`{"reps":5,"load":200}`), token))
	if tampered["error"] == nil {
		t.Errorf("a tampered body was accepted: %v", tampered)
	}
	if rec.hits != 0 {
		t.Fatalf("a refused write still reached the API (%d hit(s))", rec.hits)
	}

	// 3. The right token on the exact write executes it.
	executed := parseToolResult(t, h.toolSfAPI(req, "PUT", path, body, token))
	if executed["error"] != nil || executed["status"] != float64(http.StatusOK) {
		t.Fatalf("expected the write to execute, got %v", executed)
	}
	if rec.hits != 1 || rec.method != http.MethodPut || !strings.HasPrefix(rec.path, "/v1/training/a1/sets/s1/log") {
		t.Fatalf("the write did not reach the API as expected: hits=%d method=%q path=%q", rec.hits, rec.method, rec.path)
	}
	// The body is canonicalised (keys sorted) but carries the same values.
	var got map[string]any
	if err := json.Unmarshal([]byte(rec.body), &got); err != nil || got["reps"] != float64(5) || got["load"] != float64(100) {
		t.Errorf("the executed body was not the previewed one: %q", rec.body)
	}

	// 4. The token is single-use: replaying it is refused.
	replay := parseToolResult(t, h.toolSfAPI(req, "PUT", path, body, token))
	if replay["error"] == nil {
		t.Errorf("a used token was accepted again: %v", replay)
	}
}

// TestConfirmStore pins the token's security properties directly: it is good
// once, only for the member, method, path and exact body it was minted for, and
// only before it expires.
func TestConfirmStore(t *testing.T) {
	const (
		user   = "u-1"
		method = "PUT"
		path   = "/v1/training/a1/sets/s1/log"
		body   = `{"load":100,"reps":5}`
	)

	// Wrong member / method / path / body are each refused - and leave the
	// token live, because a mismatch is not a spend.
	s := newConfirmStore()
	token, ok := s.issue(user, method, path, body)
	if !ok || token == "" {
		t.Fatal("issue failed")
	}
	if s.consume(token, "someone-else", method, path, body) {
		t.Error("spent by the wrong member")
	}
	if s.consume(token, user, "POST", path, body) {
		t.Error("spent under the wrong method")
	}
	if s.consume(token, user, method, "/v1/training/a1/status", body) {
		t.Error("spent against the wrong path")
	}
	if s.consume(token, user, method, path, `{"load":200,"reps":5}`) {
		t.Error("spent with a different body")
	}
	// The matching call now succeeds (the mismatches did not burn it)...
	if !s.consume(token, user, method, path, body) {
		t.Error("the matching call was refused")
	}
	// ...and a match is single-use, so a second one fails.
	if s.consume(token, user, method, path, body) {
		t.Error("a token was accepted twice")
	}

	// An expired token is refused.
	token3, _ := s.issue(user, method, path, body)
	s.mu.Lock()
	entry := s.byID[token3]
	entry.expires = time.Now().Add(-time.Minute)
	s.byID[token3] = entry
	s.mu.Unlock()
	if s.consume(token3, user, method, path, body) {
		t.Error("an expired token was accepted")
	}
}

// TestToolsOfferedOnlyWithLoopback: the tool is offered only when a loopback
// base is wired, so without one the request is a plain chat.
func TestToolsOfferedOnlyWithLoopback(t *testing.T) {
	if got := (&AIHandler{}).tools(); got != nil {
		t.Errorf("tools offered with no loopback base: %v", got)
	}
	h := NewAIHandler(nil, nil, nil, "").WithLoopback("http://127.0.0.1:8080")
	if len(h.tools()) != 1 || h.tools()[0].Name != toolSfAPIName {
		t.Errorf("expected the single %s tool, got %v", toolSfAPIName, h.tools())
	}
}

// TestAFormatWithoutToolsIsToldSo is the whole reason models.AIFormatSupportsTools
// exists. CWCloud's prompt API cannot be handed a function, so the catalogue is
// withheld - and the model is told, because the skill spends paragraphs on
// reading the API before answering and one given those instructions with no way
// to obey them does not stay silent: it claims it looked something up, or worse,
// that it recorded the session.
func TestAFormatWithoutToolsIsToldSo(t *testing.T) {
	var calls int
	var sent map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"ok","response":"Open My training to see the block."}`))
	}))
	defer server.Close()

	h := NewAIHandler(nil, nil, nil, "").WithLoopback("http://127.0.0.1:8080")
	if len(h.tools()) == 0 {
		t.Fatal("the tool should be offered with a loopback base, or this test proves nothing")
	}
	answer, err := h.converse(httptest.NewRequest(http.MethodPost, "/v1/ai/chat", nil),
		models.AIConnection{
			Provider: "cwcloud", Format: models.AIFormatCWCloud, BaseURL: server.URL,
			Model: "claude-sonnet", Token: "cwc-key", MaxTokens: 512,
		}, ai.Request{
			System:   templates.CoachSkill + "\n\n" + templates.AssistantToolSkill,
			Messages: []ai.Turn{ai.TurnOf(models.ChatRoleUser, "what is my block?")},
			Tools:    h.tools(),
		})
	if err != nil {
		t.Fatalf("converse: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want one - with no tools there is no round to loop over", calls)
	}
	if answer.Message == "" {
		t.Error("the answer should still be prose")
	}
	if _, ok := sent["tools"]; ok {
		t.Errorf("the catalogue was sent to a format that cannot call it: %+v", sent)
	}

	turns, _ := sent["messages"].([]any)
	if len(turns) == 0 {
		t.Fatal("nothing was sent")
	}
	system, _ := turns[0].(map[string]any)
	instructions, _ := system["message"].(string)
	if !strings.Contains(instructions, "No tools in this configuration") {
		t.Errorf("the instructions do not say the tools are gone: %q", instructions)
	}
	// The three claims a tool-less assistant must not make.
	for _, want := range []string{"NOT available to you", "Never say you looked something up",
		"never claim to have recorded a session"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("the no-tools rule is missing %q", want)
		}
	}
}

// And the contrast: a format that does take tools is handed them, with no rule
// telling it otherwise.
func TestAFormatWithToolsKeepsThem(t *testing.T) {
	var sent map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Three sets."}}]}`))
	}))
	defer server.Close()

	h := NewAIHandler(nil, nil, nil, "").WithLoopback("http://127.0.0.1:8080")
	if _, err := h.converse(httptest.NewRequest(http.MethodPost, "/v1/ai/chat", nil),
		models.AIConnection{
			Provider: "openai", Format: models.AIFormatOpenAI, BaseURL: server.URL,
			Model: "gpt-4o-mini", Token: "sk-test", MaxTokens: 512,
		}, ai.Request{
			System:   "you are the coach",
			Messages: []ai.Turn{ai.TurnOf(models.ChatRoleUser, "what is my block?")},
			Tools:    h.tools(),
		}); err != nil {
		t.Fatalf("converse: %v", err)
	}

	if _, ok := sent["tools"]; !ok {
		t.Errorf("the catalogue should have been offered: %+v", sent)
	}
	messages, _ := sent["messages"].([]any)
	system, _ := messages[0].(map[string]any)
	if instructions, _ := system["content"].(string); strings.Contains(instructions, "No tools") {
		t.Errorf("a format that has tools must not be told it has none: %q", instructions)
	}
}
