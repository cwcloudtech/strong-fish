// Package ai asks a language model a question on a member's behalf, and lets
// the model call back for help - strong-fish's own API as a tool - before it
// answers.
//
// Four wire formats, not four vendors. OpenAI's chat-completions is also what
// Mistral, DeepSeek, OpenRouter, Groq, Ollama and LiteLLM speak, so a member
// pointing their "anthropic" connection at a local gateway is a change of URL
// and nothing else. What the format decides is the shape of the request body
// and where the key goes - a header named differently, a system prompt that is
// a field rather than a message - and that is all this package knows. The
// fourth, CWCloud's prompt API, is not a vendor but a layer over them.
//
// Tool calling is the second thing the format decides. Three of the protocols
// let a request carry a list of functions the model may call, answer with a
// call rather than prose, and take the call's result back in the next turn -
// but each spells it differently: OpenAI's tool_calls / role=tool messages,
// Anthropic's tool_use / tool_result content blocks, Gemini's functionCall /
// functionResponse parts. This package normalizes the three into Tool /
// ToolCall / Turn so the caller runs one agent loop whatever the provider. A
// request with no tools behaves exactly as a plain chat - which is the only
// way CWCloud's format can be asked anything at all, because it has no tools
// to offer (see models.AIFormatSupportsTools).
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

// RoleTool is the transcript role of a tool result, alongside the
// user/assistant roles. Each format carries it differently; Turn hides that.
const RoleTool = "tool"

// requestTimeout bounds one call to a provider. A large model reasoning over a
// month of training takes its time, and a longer answer (a subscription's
// MaxTokens, up to models.DefaultMaxTokens) takes longer still to come back; a
// timeout that fires mid-answer wastes the tokens already spent as well as the
// member's question, so it is deliberately generous.
const requestTimeout = 300 * time.Second

// Client talks to one member's connection.
type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: requestTimeout}}
}

// Tool is a function the model may call to help itself answer: a name, a
// sentence telling the model when to reach for it, and a JSON-Schema object
// describing its arguments. The three formats all take this shape, under
// different field names.
type Tool struct {
	Name        string
	Description string
	// Parameters is a JSON-Schema object, passed through to whichever format
	// is in use.
	Parameters map[string]any
}

// ToolCall is the model asking to run a tool: which one, with what arguments.
// ID matches the call to its result in the formats that key on an id (OpenAI,
// Anthropic); Gemini matches on Name instead.
type ToolCall struct {
	ID   string
	Name string
	// Args is the arguments object the model produced, as raw JSON.
	Args json.RawMessage
}

// Turn is one entry of the transcript, richer than models.ChatMessage because a
// turn may be an assistant's tool call or a tool's result, not only prose.
type Turn struct {
	Role       string
	Text       string
	ToolCalls  []ToolCall
	ToolCallID string
	ToolName   string
}

// TurnOf is a plain text turn - the shape a stored conversation flattens to.
func TurnOf(role, text string) Turn { return Turn{Role: role, Text: text} }

// Request is one call to the model.
type Request struct {
	// System is the instruction the model answers under - the coaching skill.
	System string
	// Messages is the transcript so far, oldest first, the last being what is
	// being asked now.
	Messages []Turn
	// Tools are the functions the model may call this turn. Empty means a
	// plain, single-shot answer.
	Tools     []Tool
	MaxTokens int
}

// Response is one answer. When ToolCalls is non-empty the model has not
// answered yet - it wants those calls run and their results handed back before
// it will produce prose.
type Response struct {
	Message   string         `json:"message"`
	ToolCalls []ToolCall     `json:"-"`
	Usage     map[string]any `json:"usage,omitempty"`
	Model     string         `json:"model,omitempty"`
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

// Plus is the sum of two counts, field by field. An agent answer is several
// provider calls and each one is billed, so what a question cost is the sum
// over its rounds, not the last round's figure.
func (u Usage) Plus(other Usage) Usage {
	return Usage{
		Input:  u.Input + other.Input,
		Output: u.Output + other.Output,
		Cached: u.Cached + other.Cached,
	}
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

// Ask puts the question to the model behind connection. When the request
// carries tools the answer may be a set of ToolCalls rather than prose; the
// caller runs them and asks again with the results appended (see the handler's
// agent loop).
func (c *Client) Ask(ctx context.Context, connection models.AIConnection, request Request) (Response, error) {
	if !connection.Configured() {
		return Response{}, ErrNotConfigured
	}
	// The ceiling is the caller's if they named one (the chat passes a member's
	// setting), otherwise the subscription's own, otherwise the deployment
	// default. A connection backfilled by migration always carries a value, so
	// the last fallback only covers one somehow saved without one.
	if request.MaxTokens <= 0 {
		request.MaxTokens = connection.MaxTokens
	}
	if request.MaxTokens <= 0 {
		request.MaxTokens = models.DefaultMaxTokens
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
	case models.AIFormatCWCloud:
		return c.askCWCloud(ctx, connection, model, request)
	default:
		return c.askOpenAI(ctx, connection, model, request)
	}
}

// --- CWCloud's prompt API (CWAI) -------------------------------------------
//
// Not a vendor but a layer over them: one CWCloud key reaches whichever
// adapters that account is granted - a Mistral, a Claude, a Gemini - and the
// one this question goes to is named in `adapter`, which is why a connection's
// "model" here is an adapter name rather than a vendor's model id.
//
// Three things about the protocol are worth knowing while reading this:
//
//   - **A turn's text is in `message`**, not `content`, and the request
//     carries no system field. CWAI maps a `system` role onto an assistant
//     turn, so the instructions arrive as the first thing the assistant is
//     taken to have said. That is the whole of what this format can do with a
//     system prompt, and it is said here rather than discovered later.
//   - **The answer is `response`, a string OR an array of strings**, which the
//     clients join with newlines. Both shapes come back from the same endpoint
//     depending on the adapter behind it.
//   - **It answers 201**, and a refusal is `{"status":"ko", ...}` with the HTTP
//     code to match - 424 when the model *behind* CWAI failed, and then the
//     real cause is in `upstream.body`, which is the sentence the member needs
//     rather than the wrapper around it.
//
// There are no tools: see models.AIFormatSupportsTools.

// cwcloudMessage is one turn in CWAI's shape.
type cwcloudMessage struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

// cwcloudPromptPath is appended to a base URL that does not already name the
// endpoint. The catalogue's default is the full address
// (https://api.cwcloud.tech/v1/ai/prompt), because that is what CWCloud's own
// documentation gives people to paste; somebody who pastes the host alone, or
// the /v1 root, gets the same call rather than a 404 they have to decode.
const cwcloudPromptPath = "/v1/ai/prompt"

// cwcloudEndpoint works out what to append to the configured base URL.
func cwcloudEndpoint(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch {
	case strings.HasSuffix(base, "/ai/prompt"):
		return ""
	case strings.HasSuffix(base, "/v1"):
		return "/ai/prompt"
	default:
		return cwcloudPromptPath
	}
}

func (c *Client) askCWCloud(ctx context.Context, connection models.AIConnection,
	model string, request Request) (Response, error) {
	messages := make([]cwcloudMessage, 0, len(request.Messages)+1)
	// The system prompt first, as a system turn: CWAI will relabel it as an
	// assistant turn, which is the closest thing it has to an instruction.
	if strings.TrimSpace(request.System) != "" {
		messages = append(messages, cwcloudMessage{
			Role: models.ChatRoleSystem, Message: request.System,
		})
	}
	for _, turn := range request.Messages {
		// A tool result cannot arise here - no tools are ever offered to this
		// format - and a turn with no text would be refused as empty.
		if turn.Role == RoleTool || strings.TrimSpace(turn.Text) == "" {
			continue
		}
		role := models.ChatRoleUser
		if turn.Role == models.ChatRoleAssistant {
			role = models.ChatRoleAssistant
		}
		messages = append(messages, cwcloudMessage{Role: role, Message: turn.Text})
	}

	var parsed struct {
		Status string `json:"status"`
		// Response is a string or an array of them, so it is read raw.
		Response json.RawMessage `json:"response"`
		Model    string          `json:"model"`
		Usage    struct {
			Prompt     int64 `json:"prompt"`
			Completion int64 `json:"completion"`
			Total      int64 `json:"total"`
			Cached     int64 `json:"cached"`
		} `json:"usage"`
		Error    string `json:"error"`
		I18nCode string `json:"i18n_code"`
		CID      string `json:"cid"`
	}

	err := c.call(ctx, connection, cwcloudEndpoint(connection.BaseURL), map[string]string{
		"X-Auth-Token": connection.Token,
	}, struct {
		Adapter  string           `json:"adapter"`
		Messages []cwcloudMessage `json:"messages"`
		Settings map[string]any   `json:"settings"`
		// CWCloud keeps its own prompt history by default. This app keeps the
		// conversation itself, and a model call made on a member's behalf has
		// no business filling somebody's CWCloud history with it.
		EnableHistory bool `json:"enable_history"`
	}{
		Adapter: model, Messages: messages,
		Settings:      map[string]any{"max_tokens": request.MaxTokens},
		EnableHistory: false,
	}, &parsed)
	if err != nil {
		return Response{}, cwcloudFailure(err)
	}

	// A refusal can also arrive with a 2xx, since the verdict is in the body.
	if parsed.Status != "" && !strings.EqualFold(parsed.Status, "ok") {
		return Response{}, ErrRefused{
			Status: http.StatusFailedDependency,
			Detail: snippet([]byte(firstText(parsed.Error, parsed.I18nCode,
				"CWCloud refused the request"))),
		}
	}

	answer := cwcloudAnswer(parsed.Response)
	if strings.TrimSpace(answer) == "" {
		return Response{}, errors.New("ai: the provider returned no answer")
	}
	return Response{
		Message: strings.TrimSpace(answer),
		// The model CWCloud actually spent, when it says - otherwise the
		// adapter's own name, which is what the member chose.
		Model: firstText(parsed.Model, model),
		// CWAI's own vocabulary is prompt/completion/cached, so the counts are
		// read off the struct rather than through UsageOf, which looks for the
		// three vendors' names and would bill this member zero. They are
		// republished under OpenAI's names so a stored exchange and the
		// token notice read the same whatever answered.
		Usage: map[string]any{
			"prompt_tokens": parsed.Usage.Prompt, "completion_tokens": parsed.Usage.Completion,
			"total_tokens": parsed.Usage.Total, "cached_tokens": parsed.Usage.Cached,
		},
		Tokens: Usage{
			Input: parsed.Usage.Prompt, Output: parsed.Usage.Completion, Cached: parsed.Usage.Cached,
		},
	}, nil
}

// cwcloudAnswer reads the answer, which is a string or an array of them.
func cwcloudAnswer(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var parts []string
	if json.Unmarshal(raw, &parts) == nil {
		return strings.Join(parts, "\n")
	}
	return ""
}

// cwcloudFailure unwraps the cause from a CWAI refusal. When the model behind
// CWCloud is what failed, CWAI answers 424 carrying the provider's own status
// and body; "401 Incorrect API key" is what the member can act on, and the
// wrapper around it is not.
func cwcloudFailure(err error) error {
	var refused ErrRefused
	if !errors.As(err, &refused) {
		return err
	}
	var body struct {
		Error    string `json:"error"`
		I18nCode string `json:"i18n_code"`
		Upstream struct {
			HTTPCode *int   `json:"http_code"`
			Body     string `json:"body"`
			Source   string `json:"source"`
			Adapter  string `json:"adapter"`
		} `json:"upstream"`
	}
	if json.Unmarshal([]byte(refused.Detail), &body) != nil {
		return err
	}
	switch {
	case strings.TrimSpace(body.Upstream.Body) != "":
		detail := body.Upstream.Body
		if body.Upstream.HTTPCode != nil {
			detail = fmt.Sprintf("the model behind %s answered %d: %s",
				firstText(body.Upstream.Adapter, "CWCloud"), *body.Upstream.HTTPCode, detail)
		}
		refused.Detail = snippet([]byte(detail))
	case strings.TrimSpace(body.Error) != "":
		refused.Detail = snippet([]byte(body.Error))
	case strings.TrimSpace(body.I18nCode) != "":
		refused.Detail = body.I18nCode
	}
	return refused
}

// firstText is the first of these that says something, trimmed.
func firstText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// --- OpenAI-compatible (also Mistral, LiteLLM, Ollama, OpenRouter...) ---

type openAIFunction struct {
	Name string `json:"name"`
	// Arguments is a JSON *string*, not an object - the format's own quirk.
	Arguments string `json:"arguments"`
}

type openAIToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIMessage struct {
	Role string `json:"role"`
	// Content is a string, or null on a pure tool-call turn.
	Content    any              `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

func (c *Client) askOpenAI(ctx context.Context, connection models.AIConnection,
	model string, request Request) (Response, error) {
	body := struct {
		Model     string          `json:"model"`
		Messages  []openAIMessage `json:"messages"`
		MaxTokens int             `json:"max_tokens"`
		Tools     []openAITool    `json:"tools,omitempty"`
	}{Model: model, MaxTokens: request.MaxTokens}

	// The system prompt is the first message in this format, rather than a
	// field of its own as it is for Claude.
	if request.System != "" {
		body.Messages = append(body.Messages,
			openAIMessage{Role: models.ChatRoleSystem, Content: request.System})
	}
	for _, turn := range request.Messages {
		body.Messages = append(body.Messages, openAITurn(turn))
	}
	for _, tool := range request.Tools {
		var t openAITool
		t.Type = "function"
		t.Function.Name = tool.Name
		t.Function.Description = tool.Description
		t.Function.Parameters = tool.Parameters
		body.Tools = append(body.Tools, t)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content   string           `json:"content"`
				ToolCalls []openAIToolCall `json:"tool_calls"`
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
	message := parsed.Choices[0].Message
	response := Response{
		Message: strings.TrimSpace(message.Content),
		Usage:   parsed.Usage, Model: parsed.Model, Tokens: UsageOf(parsed.Usage),
	}
	for _, call := range message.ToolCalls {
		response.ToolCalls = append(response.ToolCalls, ToolCall{
			ID: call.ID, Name: call.Function.Name, Args: rawArgs(call.Function.Arguments),
		})
	}
	if len(response.ToolCalls) == 0 && response.Message == "" {
		return Response{}, errors.New("ai: the provider returned no answer")
	}
	return response, nil
}

// openAITurn renders one transcript turn in OpenAI's message shape.
func openAITurn(turn Turn) openAIMessage {
	switch turn.Role {
	case RoleTool:
		return openAIMessage{Role: RoleTool, ToolCallID: turn.ToolCallID, Content: turn.Text}
	case models.ChatRoleAssistant:
		if len(turn.ToolCalls) > 0 {
			message := openAIMessage{Role: models.ChatRoleAssistant}
			if turn.Text != "" {
				message.Content = turn.Text
			}
			for _, call := range turn.ToolCalls {
				message.ToolCalls = append(message.ToolCalls, openAIToolCall{
					ID: call.ID, Type: "function",
					Function: openAIFunction{Name: call.Name, Arguments: argsString(call.Args)},
				})
			}
			return message
		}
		return openAIMessage{Role: models.ChatRoleAssistant, Content: turn.Text}
	default:
		return openAIMessage{Role: models.ChatRoleUser, Content: turn.Text}
	}
}

// --- Anthropic ---

type anthropicBlock struct {
	Type string `json:"type"`
	// text block
	Text string `json:"text,omitempty"`
	// tool_use block
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result block
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type anthropicMessage struct {
	Role string `json:"role"`
	// Content is a plain string, or a []anthropicBlock for tool turns.
	Content any `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

func (c *Client) askAnthropic(ctx context.Context, connection models.AIConnection,
	model string, request Request) (Response, error) {
	body := struct {
		Model     string             `json:"model"`
		System    string             `json:"system,omitempty"`
		Messages  []anthropicMessage `json:"messages"`
		MaxTokens int                `json:"max_tokens"`
		Tools     []anthropicTool    `json:"tools,omitempty"`
	}{Model: model, System: request.System, MaxTokens: request.MaxTokens}

	for i := 0; i < len(request.Messages); {
		turn := request.Messages[i]
		// A system turn that somehow arrived in the history belongs in the
		// system field, which already carries the skill.
		if turn.Role == models.ChatRoleSystem {
			i++
			continue
		}
		// A round's tool results must arrive as ONE user message of tool_result
		// blocks - two user messages in a row break Claude's role alternation.
		if turn.Role == RoleTool {
			blocks := []anthropicBlock{}
			for i < len(request.Messages) && request.Messages[i].Role == RoleTool {
				result := request.Messages[i]
				blocks = append(blocks, anthropicBlock{
					Type: "tool_result", ToolUseID: result.ToolCallID, Content: result.Text,
				})
				i++
			}
			body.Messages = append(body.Messages,
				anthropicMessage{Role: models.ChatRoleUser, Content: blocks})
			continue
		}
		body.Messages = append(body.Messages, anthropicTurn(turn))
		i++
	}
	for _, tool := range request.Tools {
		body.Tools = append(body.Tools, anthropicTool{
			Name: tool.Name, Description: tool.Description, InputSchema: tool.Parameters,
		})
	}

	var parsed struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
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

	response := Response{Usage: parsed.Usage, Model: parsed.Model, Tokens: UsageOf(parsed.Usage)}
	var answer strings.Builder
	for _, block := range parsed.Content {
		switch block.Type {
		case "text":
			answer.WriteString(block.Text)
		case "tool_use":
			response.ToolCalls = append(response.ToolCalls, ToolCall{
				ID: block.ID, Name: block.Name, Args: rawObject(block.Input),
			})
		}
	}
	response.Message = strings.TrimSpace(answer.String())
	if len(response.ToolCalls) == 0 && response.Message == "" {
		return Response{}, errors.New("ai: the provider returned no answer")
	}
	return response, nil
}

// anthropicTurn renders one non-tool-result turn in Claude's shape.
func anthropicTurn(turn Turn) anthropicMessage {
	switch turn.Role {
	case models.ChatRoleAssistant:
		if len(turn.ToolCalls) > 0 {
			blocks := []anthropicBlock{}
			if turn.Text != "" {
				blocks = append(blocks, anthropicBlock{Type: "text", Text: turn.Text})
			}
			for _, call := range turn.ToolCalls {
				blocks = append(blocks, anthropicBlock{
					Type: "tool_use", ID: call.ID, Name: call.Name, Input: rawObject(call.Args),
				})
			}
			return anthropicMessage{Role: models.ChatRoleAssistant, Content: blocks}
		}
		return anthropicMessage{Role: models.ChatRoleAssistant, Content: turn.Text}
	default:
		return anthropicMessage{Role: models.ChatRoleUser, Content: turn.Text}
	}
}

// --- Gemini ---

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

type geminiFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

func (c *Client) askGemini(ctx context.Context, connection models.AIConnection,
	model string, request Request) (Response, error) {
	var systemInstruction *geminiContent
	if request.System != "" {
		systemInstruction = &geminiContent{Parts: []geminiPart{{Text: request.System}}}
	}
	var contents []geminiContent
	for i := 0; i < len(request.Messages); {
		turn := request.Messages[i]
		if turn.Role == models.ChatRoleSystem {
			i++
			continue
		}
		if turn.Role == RoleTool {
			parts := []geminiPart{}
			for i < len(request.Messages) && request.Messages[i].Role == RoleTool {
				result := request.Messages[i]
				parts = append(parts, geminiPart{FunctionResponse: &geminiFunctionResponse{
					Name: result.ToolName, Response: toolResponseObject(result.Text),
				}})
				i++
			}
			contents = append(contents, geminiContent{Role: models.ChatRoleUser, Parts: parts})
			continue
		}
		contents = append(contents, geminiTurn(turn))
		i++
	}

	var tools []geminiTool
	if len(request.Tools) > 0 {
		declarations := make([]geminiFunctionDeclaration, 0, len(request.Tools))
		for _, tool := range request.Tools {
			declarations = append(declarations, geminiFunctionDeclaration{
				Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters,
			})
		}
		tools = []geminiTool{{FunctionDeclarations: declarations}}
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []geminiPart `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata map[string]any `json:"usageMetadata"`
	}
	path := fmt.Sprintf("/models/%s:generateContent", model)
	if err := c.call(ctx, connection, path, map[string]string{
		"x-goog-api-key": connection.Token,
	}, struct {
		SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
		Contents          []geminiContent `json:"contents"`
		Tools             []geminiTool    `json:"tools,omitempty"`
		GenerationConfig  any             `json:"generationConfig"`
	}{systemInstruction, contents, tools, struct {
		MaxOutputTokens int `json:"maxOutputTokens"`
	}{request.MaxTokens}}, &parsed); err != nil {
		return Response{}, err
	}

	response := Response{Usage: parsed.UsageMetadata, Model: model, Tokens: UsageOf(parsed.UsageMetadata)}
	var answer strings.Builder
	for _, candidate := range parsed.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				response.ToolCalls = append(response.ToolCalls, ToolCall{
					Name: part.FunctionCall.Name, Args: rawObject(part.FunctionCall.Args),
				})
				continue
			}
			answer.WriteString(part.Text)
		}
		break
	}
	response.Message = strings.TrimSpace(answer.String())
	if len(response.ToolCalls) == 0 && response.Message == "" {
		return Response{}, errors.New("ai: the provider returned no answer")
	}
	return response, nil
}

// geminiTurn renders one non-tool-result turn in Gemini's shape (Gemini calls
// the assistant "model").
func geminiTurn(turn Turn) geminiContent {
	switch turn.Role {
	case models.ChatRoleAssistant:
		if len(turn.ToolCalls) > 0 {
			parts := []geminiPart{}
			if turn.Text != "" {
				parts = append(parts, geminiPart{Text: turn.Text})
			}
			for _, call := range turn.ToolCalls {
				parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{
					Name: call.Name, Args: rawObject(call.Args),
				}})
			}
			return geminiContent{Role: "model", Parts: parts}
		}
		return geminiContent{Role: "model", Parts: []geminiPart{{Text: turn.Text}}}
	default:
		return geminiContent{Role: models.ChatRoleUser, Parts: []geminiPart{{Text: turn.Text}}}
	}
}

// --- tool-argument helpers ---

// rawArgs turns OpenAI's arguments string into the raw JSON an argument object
// is, defaulting an empty or malformed value to {}.
func rawArgs(arguments string) json.RawMessage {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" || !json.Valid([]byte(arguments)) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(arguments)
}

// rawObject keeps a raw-JSON argument object usable on the wire, defaulting a
// missing or invalid one to {}.
func rawObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage("{}")
	}
	return raw
}

// argsString renders an argument object back as the JSON string OpenAI echoes
// tool calls in.
func argsString(raw json.RawMessage) string {
	if len(raw) == 0 || !json.Valid(raw) {
		return "{}"
	}
	return string(raw)
}

// toolResponseObject wraps a tool's result for Gemini's
// functionResponse.response, which must be an object.
func toolResponseObject(result string) map[string]any {
	var object map[string]any
	if json.Unmarshal([]byte(result), &object) == nil {
		return object
	}
	return map[string]any{"result": result}
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
