// Package ai asks a language model a question on a member's behalf.
//
// Three wire formats, not three vendors. OpenAI's chat-completions is also
// what Mistral, DeepSeek, OpenRouter, Groq, Ollama and LiteLLM speak, so a
// member pointing their "anthropic" connection at a local gateway is a change
// of URL and nothing else. What the format decides is the shape of the request
// body and where the key goes - a header named differently, a system prompt
// that is a field rather than a message - and that is all this package knows.
//
// Nothing here is streamed. A coach reading a paragraph about their session
// does not need it a token at a time, and an endpoint that answers once is one
// the phone and the browser can both call with an ordinary request.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"strong-fish-api/internal/models"
)

// ErrNotConfigured is a member asking a question with no connection behind
// them - no key of their own, and nobody has shared one.
var ErrNotConfigured = errors.New("ai: no connection available")

// ErrRefused is the provider rejecting the request: a bad key, a model the
// account cannot reach, a quota. It travels to the screen as itself because
// the member is the only one who can fix it.
type ErrRefused struct {
	Status int
	Detail string
}

func (e ErrRefused) Error() string {
	return fmt.Sprintf("ai: the provider refused the request (http %d): %s", e.Status, e.Detail)
}

// defaultMaxTokens bounds an answer when the caller names no limit. Long
// enough for a considered paragraph about a session, short enough that a
// runaway model does not spend somebody's budget in one question.
const defaultMaxTokens = 1200

// Client talks to one member's connection.
type Client struct {
	http *http.Client
}

func New() *Client {
	// Generous: a large model thinking about a month of training takes its
	// time, and a timeout that fires mid-answer wastes the tokens already
	// spent as well as the member's question.
	return &Client{http: &http.Client{Timeout: 120 * time.Second}}
}

// Request is one question, in the shape cwclock's web agent takes.
type Request struct {
	// System is the instruction the model answers under - the coaching skill.
	System string
	// Messages is the conversation so far, oldest first, the last being what
	// is being asked now.
	Messages  []models.ChatMessage
	MaxTokens int
}

// Response is one answer.
type Response struct {
	Message string `json:"message"`
	// Usage is what the provider reported, as it reported it - kept whole,
	// because every provider counts something the others do not and throwing
	// the rest away would lose it.
	Usage map[string]any `json:"usage,omitempty"`
	Model string         `json:"model,omitempty"`
	// Tokens is the same thing in the three numbers every provider agrees on,
	// so a meter does not have to know whose JSON it is reading.
	Tokens Usage `json:"tokens"`
}

// Usage is a provider's token count, normalized.
//
// Cached input is counted apart rather than folded into Input: it is what a
// provider charges a fraction for, and a bill that cannot be told apart from
// fresh input is one nobody can explain.
type Usage struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
	Cached int64 `json:"cached"`
}

// UsageOf reads a provider's own usage object into those three numbers.
//
// Three vocabularies for the same counts: OpenAI says prompt/completion with
// the cached part nested under prompt_tokens_details, Anthropic says
// input/output with two separate cache lines, Gemini says
// promptTokenCount/candidatesTokenCount with cachedContentTokenCount. Every
// name is looked for rather than switching on the provider, because a gateway
// speaking one format may report in another's names - and a missing key is
// simply a zero.
func UsageOf(usage map[string]any) Usage {
	read := func(keys ...string) int64 {
		var total int64
		for _, key := range keys {
			total += number(usage[key])
		}
		return total
	}

	tokens := Usage{
		Input: read("prompt_tokens", "input_tokens", "promptTokenCount"),
		Output: read("completion_tokens", "output_tokens", "candidatesTokenCount",
			"candidates_token_count"),
		Cached: read("cache_read_input_tokens", "cache_creation_input_tokens",
			"cachedContentTokenCount", "cached_content_token_count"),
	}
	// OpenAI nests its cached count a level down.
	if details, ok := usage["prompt_tokens_details"].(map[string]any); ok {
		tokens.Cached += number(details["cached_tokens"])
	}
	return tokens
}

// number reads a JSON value that should have been a count. Decoded JSON gives
// float64; a gateway that sends "1234" as a string is still counted.
func number(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0
		}
		return parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	}
	return 0
}

// Ask puts the question to the model behind connection.
func (c *Client) Ask(ctx context.Context, connection models.AIConnection, request Request) (Response, error) {
	if !connection.Configured() {
		return Response{}, ErrNotConfigured
	}
	if request.MaxTokens <= 0 {
		request.MaxTokens = defaultMaxTokens
	}
	model := strings.TrimSpace(connection.Model)

	// The format comes from the provider catalogue (see the ai_providers
	// table), so a new OpenAI-compatible service is a row rather than a case
	// here. An unknown one is assumed to speak that format, which is what a
	// gateway almost always does.
	switch connection.Format {
	case models.AIFormatAnthropic:
		return c.askAnthropic(ctx, connection, model, request)
	case models.AIFormatGemini:
		return c.askGemini(ctx, connection, model, request)
	default:
		return c.askOpenAI(ctx, connection, model, request)
	}
}

// --- OpenAI-compatible (also Mistral, LiteLLM, Ollama, OpenRouter...) ---

func (c *Client) askOpenAI(ctx context.Context, connection models.AIConnection,
	model string, request Request) (Response, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := struct {
		Model     string    `json:"model"`
		Messages  []message `json:"messages"`
		MaxTokens int       `json:"max_tokens"`
	}{Model: model, MaxTokens: request.MaxTokens}

	// The system prompt is the first message in this format, rather than a
	// field of its own as it is for Claude.
	if request.System != "" {
		body.Messages = append(body.Messages, message{Role: models.ChatRoleSystem, Content: request.System})
	}
	for _, turn := range request.Messages {
		body.Messages = append(body.Messages, message{Role: turn.Role, Content: turn.Message})
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage map[string]any `json:"usage"`
		Model string         `json:"model"`
	}
	if err := c.call(ctx, connection, "/chat/completions", map[string]string{
		"Authorization": "Bearer " + connection.Token,
	}, body, &parsed); err != nil {
		return Response{}, err
	}
	if len(parsed.Choices) == 0 {
		return Response{}, errors.New("ai: the provider returned no answer")
	}
	return Response{
		Message: strings.TrimSpace(parsed.Choices[0].Message.Content),
		Usage:   parsed.Usage, Model: parsed.Model, Tokens: UsageOf(parsed.Usage),
	}, nil
}

// --- Anthropic ---

func (c *Client) askAnthropic(ctx context.Context, connection models.AIConnection,
	model string, request Request) (Response, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := struct {
		Model     string    `json:"model"`
		System    string    `json:"system,omitempty"`
		Messages  []message `json:"messages"`
		MaxTokens int       `json:"max_tokens"`
	}{Model: model, System: request.System, MaxTokens: request.MaxTokens}

	for _, turn := range request.Messages {
		// Claude takes user and assistant turns only; a system turn that
		// somehow arrived in the history belongs in the system field, which
		// already carries the skill.
		if turn.Role == models.ChatRoleSystem {
			continue
		}
		body.Messages = append(body.Messages, message{Role: turn.Role, Content: turn.Message})
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage map[string]any `json:"usage"`
		Model string         `json:"model"`
	}
	if err := c.call(ctx, connection, "/messages", map[string]string{
		"x-api-key":         connection.Token,
		"anthropic-version": "2023-06-01",
	}, body, &parsed); err != nil {
		return Response{}, err
	}

	var answer strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			answer.WriteString(block.Text)
		}
	}
	if answer.Len() == 0 {
		return Response{}, errors.New("ai: the provider returned no answer")
	}
	return Response{Message: strings.TrimSpace(answer.String()), Usage: parsed.Usage,
		Model: parsed.Model, Tokens: UsageOf(parsed.Usage)}, nil
}

// --- Gemini ---

func (c *Client) askGemini(ctx context.Context, connection models.AIConnection,
	model string, request Request) (Response, error) {
	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role,omitempty"`
		Parts []part `json:"parts"`
	}
	body := struct {
		SystemInstruction *content `json:"systemInstruction,omitempty"`
		Contents          []content
		GenerationConfig  struct {
			MaxOutputTokens int `json:"maxOutputTokens"`
		} `json:"generationConfig"`
	}{}
	body.GenerationConfig.MaxOutputTokens = request.MaxTokens
	if request.System != "" {
		body.SystemInstruction = &content{Parts: []part{{Text: request.System}}}
	}
	for _, turn := range request.Messages {
		if turn.Role == models.ChatRoleSystem {
			continue
		}
		// Gemini calls the assistant "model".
		role := turn.Role
		if role == models.ChatRoleAssistant {
			role = "model"
		}
		body.Contents = append(body.Contents, content{Role: role, Parts: []part{{Text: turn.Message}}})
	}

	// Gemini names the model in the path and takes its key as a header.
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata map[string]any `json:"usageMetadata"`
	}
	path := fmt.Sprintf("/models/%s:generateContent", model)
	if err := c.call(ctx, connection, path, map[string]string{
		"x-goog-api-key": connection.Token,
	}, struct {
		SystemInstruction *content  `json:"systemInstruction,omitempty"`
		Contents          []content `json:"contents"`
		GenerationConfig  any       `json:"generationConfig"`
	}{body.SystemInstruction, body.Contents, body.GenerationConfig}, &parsed); err != nil {
		return Response{}, err
	}

	var answer strings.Builder
	for _, candidate := range parsed.Candidates {
		for _, p := range candidate.Content.Parts {
			answer.WriteString(p.Text)
		}
		break
	}
	if answer.Len() == 0 {
		return Response{}, errors.New("ai: the provider returned no answer")
	}
	return Response{Message: strings.TrimSpace(answer.String()), Usage: parsed.UsageMetadata,
		Model: model, Tokens: UsageOf(parsed.UsageMetadata)}, nil
}

// --- the wire ---

func (c *Client) call(ctx context.Context, connection models.AIConnection, path string,
	headers map[string]string, body, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	endpoint := strings.TrimRight(connection.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		// The provider's own words, trimmed. A key with no credit and a model
		// that does not exist are different problems with the same status,
		// and only the body says which.
		return ErrRefused{Status: resp.StatusCode, Detail: snippet(raw)}
	}
	return json.Unmarshal(raw, out)
}

// snippet is as much of a refusal as belongs in a log line or an error.
func snippet(body []byte) string {
	const limit = 300
	text := strings.TrimSpace(string(body))
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}
