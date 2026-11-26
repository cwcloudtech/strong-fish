package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"strong-fish-api/internal/models"
)

// CWCloud's prompt API is the one format in this package that is not a vendor's
// own, and every way it differs is a way a mistake would pass silently: a
// payload the server accepts but ignores, a usage object read as zeros, an
// error envelope shown to a member in place of the sentence inside it. These
// tests put a stub CWAI behind a connection and read what was actually sent.

// cwcloudConnection is a connection pointed at a stub.
func cwcloudConnection(baseURL, model string) models.AIConnection {
	return models.AIConnection{
		Provider: "cwcloud", Format: models.AIFormatCWCloud, BaseURL: baseURL,
		Model: model, Token: "cwc-key", MaxTokens: 500,
	}
}

// TestTheCWCloudFormatSpeaksCWAI covers the three differences that would each
// break quietly: the model travels in `adapter`, a turn's text is in `message`
// rather than `content`, and there is no system field - the instructions go as
// a leading system turn, which CWAI relabels as an assistant one.
func TestTheCWCloudFormatSpeaksCWAI(t *testing.T) {
	var sent map[string]any
	var authToken, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		authToken = r.Header.Get("X-Auth-Token")
		_ = json.NewDecoder(r.Body).Decode(&sent)
		// 201, not 200 - and the answer is read all the same.
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"ok","response":"Three sets at 100kg.",
			"model":"mistral-medium-2505","prompt_id":7,"cid":"abc",
			"usage":{"prompt":8,"completion":10,"total":18,"cached":2}}`))
	}))
	defer server.Close()

	answer, err := New().Ask(context.Background(), cwcloudConnection(server.URL, "mistral-medium"),
		Request{
			System:   "You are the strong-fish coach",
			Messages: []Turn{TurnOf(models.ChatRoleUser, "what did I squat?")},
			// Offered on purpose: a caller that forgets this format has no
			// tools must not have them smuggled into the payload.
			Tools: []Tool{{Name: "strong_fish_api", Parameters: map[string]any{"type": "object"}}},
		})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}

	if path != "/v1/ai/prompt" {
		t.Errorf("path = %q, want the prompt endpoint", path)
	}
	if authToken != "cwc-key" {
		t.Errorf("X-Auth-Token = %q, want the connection's key", authToken)
	}
	if sent["adapter"] != "mistral-medium" {
		t.Errorf("adapter = %v, want the model named in CWAI's own field", sent["adapter"])
	}
	if _, ok := sent["model"]; ok {
		t.Errorf("CWAI takes no model field: %+v", sent)
	}
	if _, ok := sent["tools"]; ok {
		t.Errorf("this format has no tools, so none may be sent: %+v", sent)
	}
	if history, ok := sent["enable_history"].(bool); !ok || history {
		t.Errorf("enable_history = %v, want false - this app keeps the conversation, "+
			"and a member's CWCloud history is not ours to fill", sent["enable_history"])
	}
	settings, _ := sent["settings"].(map[string]any)
	if settings == nil || settings["max_tokens"] != float64(500) {
		t.Errorf("settings = %+v, want the answer capped by the connection", settings)
	}

	turns, _ := sent["messages"].([]any)
	if len(turns) != 2 {
		t.Fatalf("messages = %+v, want the instructions then the question", turns)
	}
	first, _ := turns[0].(map[string]any)
	if first["role"] != models.ChatRoleSystem || first["message"] != "You are the strong-fish coach" {
		t.Errorf("the system prompt travels as a turn whose text is in `message`: %+v", first)
	}
	if _, ok := first["content"]; ok {
		t.Errorf("a turn's text is in `message`, never `content`: %+v", first)
	}
	second, _ := turns[1].(map[string]any)
	if second["role"] != models.ChatRoleUser || second["message"] != "what did I squat?" {
		t.Errorf("turn = %+v, want the question in `message`", second)
	}

	if answer.Message != "Three sets at 100kg." {
		t.Errorf("message = %q", answer.Message)
	}
	// What CWCloud actually spent, not the adapter that was asked for.
	if answer.Model != "mistral-medium-2505" {
		t.Errorf("model = %q, want what CWCloud reported", answer.Model)
	}
	if answer.Tokens != (Usage{Input: 8, Output: 10, Cached: 2}) {
		t.Errorf("tokens = %+v, want CWAI's prompt/completion/cached read across - "+
			"a mapper written for OpenAI's names bills this member zero", answer.Tokens)
	}
}

// A conversation reaches CWAI as alternating turns, and the assistant's own
// are labelled as such; a tool result could only arrive from a caller that
// forgot this format has no tools, and is dropped rather than sent as an
// empty-roled turn CWAI would refuse.
func TestACWCloudTranscriptKeepsItsRolesAndDropsWhatCannotTravel(t *testing.T) {
	var sent map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"ok","response":"ok"}`))
	}))
	defer server.Close()

	_, err := New().Ask(context.Background(), cwcloudConnection(server.URL, "claude-sonnet"), Request{
		Messages: []Turn{
			TurnOf(models.ChatRoleUser, "hello"),
			TurnOf(models.ChatRoleAssistant, "hello yourself"),
			{Role: RoleTool, ToolCallID: "call_1", ToolName: "strong_fish_api", Text: `{"status":200}`},
			TurnOf(models.ChatRoleUser, ""),
			TurnOf(models.ChatRoleUser, "and now?"),
		},
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}

	turns, _ := sent["messages"].([]any)
	if len(turns) != 3 {
		t.Fatalf("messages = %+v, want the three turns that can travel", turns)
	}
	wantRoles := []string{models.ChatRoleUser, models.ChatRoleAssistant, models.ChatRoleUser}
	for i, want := range wantRoles {
		turn, _ := turns[i].(map[string]any)
		if turn["role"] != want {
			t.Errorf("turn %d role = %v, want %q", i, turn["role"], want)
		}
	}
}

// The answer is a string or an array of them - both shapes come back from the
// same endpoint, depending on the adapter behind it.
func TestACWCloudAnswerMayBeAList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"ok","response":["Week one: squat.","Week two: deadlift."]}`))
	}))
	defer server.Close()

	// Also the /v1 root as a base: the endpoint is appended.
	answer, err := New().Ask(context.Background(),
		cwcloudConnection(server.URL+"/v1", "claude-sonnet"),
		Request{Messages: []Turn{TurnOf(models.ChatRoleUser, "my block?")}})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if answer.Message != "Week one: squat.\nWeek two: deadlift." {
		t.Errorf("message = %q, want the lines joined", answer.Message)
	}
	// Nothing reported: a zero count, not a failed answer.
	if answer.Tokens != (Usage{}) {
		t.Errorf("tokens = %+v, want zeros", answer.Tokens)
	}
	// With no model reported, the adapter the member chose is what answered.
	if answer.Model != "claude-sonnet" {
		t.Errorf("model = %q, want the adapter's own name as the fallback", answer.Model)
	}
}

// When the model BEHIND CWCloud fails, CWAI answers 424 wrapping the
// provider's own status and body. What the member can act on is that body -
// the key is wrong - not the envelope, so it is what the refusal carries.
func TestACWCloudUpstreamFailureCarriesTheRealCause(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFailedDependency)
		_, _ = w.Write([]byte(`{"status":"ko","i18n_code":"cwai_upstream_error","cid":"abc",
			"http_code":424,"upstream":{"http_code":401,"body":"Incorrect API key provided",
			"source":"subscription","adapter":"claude-sonnet"}}`))
	}))
	defer server.Close()

	_, err := New().Ask(context.Background(), cwcloudConnection(server.URL, "claude-sonnet"),
		Request{Messages: []Turn{TurnOf(models.ChatRoleUser, "hi")}})

	var refused ErrRefused
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a refusal", err)
	}
	for _, want := range []string{"401", "Incorrect API key", "claude-sonnet"} {
		if !strings.Contains(refused.Detail, want) {
			t.Errorf("detail = %q, want it to carry %q rather than the envelope",
				refused.Detail, want)
		}
	}
	if strings.Contains(refused.Detail, "cwai_upstream_error") {
		t.Errorf("detail = %q, want the cause and not the wrapper's i18n code", refused.Detail)
	}
}

// A 424 with nothing underneath still has to say something: the envelope's own
// words are then the best there is.
func TestACWCloudFailureWithNoUpstreamFallsBackToItsOwnWords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFailedDependency)
		_, _ = w.Write([]byte(`{"status":"ko","error":"adapter not granted to this account"}`))
	}))
	defer server.Close()

	_, err := New().Ask(context.Background(), cwcloudConnection(server.URL, "gemini-flash"),
		Request{Messages: []Turn{TurnOf(models.ChatRoleUser, "hi")}})

	var refused ErrRefused
	if !errors.As(err, &refused) || !strings.Contains(refused.Detail, "adapter not granted") {
		t.Fatalf("err = %v, want CWCloud's own words", err)
	}
}

// A refusal can arrive with a 2xx, because CWAI puts the verdict in the body.
// Reading only the HTTP status would turn it into "the provider returned no
// answer", which tells the member nothing about what to fix.
func TestACWCloudRefusalInTheBodyIsStillARefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"ko","error":"adapter not found: nope",
			"i18n_code":"cwai_adapter_notfound"}`))
	}))
	defer server.Close()

	_, err := New().Ask(context.Background(), cwcloudConnection(server.URL, "nope"),
		Request{Messages: []Turn{TurnOf(models.ChatRoleUser, "hi")}})

	var refused ErrRefused
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if !strings.Contains(refused.Detail, "adapter not found") {
		t.Errorf("detail = %q, want CWCloud's own words", refused.Detail)
	}
}

// Wherever the base URL stops, the call lands on the prompt endpoint: the
// catalogue's default is the whole address, and somebody who pastes the host or
// the /v1 root should not meet a 404 they have to decode.
func TestTheCWCloudEndpointToleratesAnyBase(t *testing.T) {
	for base, want := range map[string]string{
		"https://api.cwcloud.tech/v1/ai/prompt":  "",
		"https://api.cwcloud.tech/v1/ai/prompt/": "",
		" https://api.cwcloud.tech/v1/ai/prompt": "",
		"https://api.cwcloud.tech/v1":            "/ai/prompt",
		"https://api.cwcloud.tech/v1/":           "/ai/prompt",
		"https://api.cwcloud.tech":               "/v1/ai/prompt",
		"https://cwai.example.com/":              "/v1/ai/prompt",
	} {
		if got := cwcloudEndpoint(base); got != want {
			t.Errorf("cwcloudEndpoint(%q) = %q, want %q", base, got, want)
		}
	}
}

// The format's own answer to "may I be handed tools". One predicate, so a
// future format that cannot call them is added here rather than worked around
// at a call site.
func TestOnlyCWCloudLacksTools(t *testing.T) {
	for _, format := range []string{models.AIFormatOpenAI, models.AIFormatAnthropic,
		models.AIFormatGemini, "some-gateway", ""} {
		if !models.AIFormatSupportsTools(format) {
			t.Errorf("format %q should take tools", format)
		}
	}
	for _, format := range []string{models.AIFormatCWCloud, " cwcloud "} {
		if models.AIFormatSupportsTools(format) {
			t.Errorf("CWAI (%q) has no way to declare a tool or receive a call", format)
		}
	}
}

// A connection with no key cannot be spent, and that is not worth a round trip
// to CWCloud.
func TestACWCloudConnectionWithoutAKeyIsNotCalled(t *testing.T) {
	_, err := New().Ask(context.Background(), models.AIConnection{
		Provider: "cwcloud", Format: models.AIFormatCWCloud,
		BaseURL: "https://api.cwcloud.tech/v1/ai/prompt", Model: "claude-sonnet",
	}, Request{Messages: []Turn{TurnOf(models.ChatRoleUser, "hi")}})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}
