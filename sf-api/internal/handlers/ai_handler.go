package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/ai"
	"strong-fish-api/internal/metrics"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/templates"
	"strong-fish-api/internal/utils"
)

// AIHandler is a member's connections to a language model, and the questions
// they put through them.
//
// The connections are shaped on storages, down to the sharing: an account
// holds a credential and decides who else may spend it. A coach paying for a
// key and letting their athletes ask it questions is the same arrangement as a
// club sharing a bucket.
type AIHandler struct {
	connections *store.AIStore
	users       *store.UserStore
	audit       *store.AuditStore
	client      *ai.Client
	// uiBaseURL is where a shared conversation is read, and the attribution
	// line of an exported one.
	uiBaseURL string
	// What the chat tells the model about the member before their question:
	// their maxes, and the block they are in the middle of. Set with
	// WithMemberContext, because the program handler these come from is built
	// after this one - and left unset the chat still answers, without them.
	oneRMs   *store.OneRMStore
	programs *store.ProgramStore
	sets     *ProgramHandler
	// lastDay is the member's fitness data as markdown - recent steps and heart
	// rate, latest body composition, and the last bar-speed set of each lift. A
	// function rather than the device handler itself: that handler already
	// holds this one, and two pointers at each other is a knot nobody needs to
	// untie later.
	lastDay func(*http.Request, string, string) string
	// sleep is last night on its own, for a session review - the whole fitness
	// page would be too much there, but how the athlete slept bears on it.
	sleep func(*http.Request, string, string) string
	// selfURL is the loopback base the strong_fish_api tool calls this very API
	// with (http://127.0.0.1:<port>), so a self-call never leaves the host and
	// never meets the corporate proxy. Blank leaves the tool unoffered.
	selfURL string
	// confirms holds the short-lived tokens a write preview mints, so an execute
	// call can prove the user confirmed exactly this write and nothing else.
	confirms *confirmStore
}

func NewAIHandler(connections *store.AIStore, users *store.UserStore, audit *store.AuditStore, uiBaseURL string) *AIHandler {
	return &AIHandler{
		connections: connections, users: users, audit: audit, client: ai.New(),
		uiBaseURL: uiBaseURL, confirms: newConfirmStore(),
	}
}

// WithLoopback wires the base the assistant's strong_fish_api tool calls this
// very API with, over loopback. Without it the tool is not offered and the
// assistant answers from the member context and its own knowledge alone.
func (h *AIHandler) WithLoopback(selfURL string) *AIHandler {
	h.selfURL = strings.TrimSpace(selfURL)
	return h
}

// WithMemberContext gives the chat the member's own numbers to answer with.
func (h *AIHandler) WithMemberContext(oneRMs *store.OneRMStore, programs *store.ProgramStore,
	sets *ProgramHandler) *AIHandler {
	h.oneRMs, h.programs, h.sets = oneRMs, programs, sets
	return h
}

// WithHealth adds the member's fitness data - recent activity, body
// composition and bar speed - to that context.
func (h *AIHandler) WithHealth(lastDay func(*http.Request, string, string) string) *AIHandler {
	h.lastDay = lastDay
	return h
}

// WithSleep adds last night's sleep on its own, for the session review context.
func (h *AIHandler) WithSleep(sleep func(*http.Request, string, string) string) *AIHandler {
	h.sleep = sleep
	return h
}

// MemberSleepContext is last night's sleep as markdown, or "" when there is no
// reader wired or no recent night.
func (h *AIHandler) MemberSleepContext(r *http.Request, userID, locale string) string {
	if h.sleep == nil {
		return ""
	}
	return h.sleep(r, userID, locale)
}

type aiConnectionPayload struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	MaxTokens int    `json:"maxTokens"`
	Token     string `json:"token"`
}

// normalize fills in what the member left blank and refuses what cannot work.
//
// The URL and the model come from the provider catalogue when the member left
// them alone, because the common case is somebody pasting a key and expecting
// it to work; they stay editable because pointing a connection at a local
// gateway is the reason the fields exist.
//
// The model is never checked against the catalogue's list. Those names are
// suggestions: a vendor ships one the day before the table hears about it,
// and refusing it here would make the app the reason somebody cannot use
// their own subscription.
func (h *AIHandler) normalize(w http.ResponseWriter, r *http.Request,
	p aiConnectionPayload) (models.AIConnection, bool) {
	provider, err := h.connections.FindProvider(r.Context(),
		strings.ToLower(strings.TrimSpace(p.Provider)))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "Pick a provider", CodeInvalidAIProvider)
			return models.AIConnection{}, false
		}
		writeStoreError(w, err)
		return models.AIConnection{}, false
	}

	baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if baseURL == "" {
		baseURL = provider.BaseURL
	}
	// Anything but http(s) would be a request this API makes on a member's
	// behalf to somewhere it should not - a file, an internal scheme.
	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://") {
		writeError(w, http.StatusBadRequest, "The address must be an http or https URL", CodeInvalidAIBaseURL)
		return models.AIConnection{}, false
	}

	model := strings.TrimSpace(p.Model)
	if model == "" {
		model = provider.DefaultModel
	}

	// A connection nobody named is named after what it is: the model, and
	// enough random characters to tell two of them apart. A list of three
	// connections all called "Anthropic" is a list nobody can pick from, and
	// asking for a name before the thing exists is a field people skip.
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = defaultConnectionName(model)
	}

	// The answer-length ceiling is the member's to raise, within a sane bound: a
	// blank or nonsense value takes the deployment default rather than being
	// rejected, and a slipped digit is capped rather than billed.
	maxTokens := p.MaxTokens
	if maxTokens <= 0 {
		maxTokens = models.DefaultMaxTokens
	}
	if maxTokens > models.MaxMaxTokens {
		maxTokens = models.MaxMaxTokens
	}

	return models.AIConnection{
		Name: name, Provider: provider.Name,
		BaseURL: baseURL, Model: model, MaxTokens: maxTokens,
		Token: strings.TrimSpace(p.Token),
	}, true
}

// defaultConnectionName is "<model>-<8 random characters>", the same shape
// the clients show as the field's placeholder - so saving a blank name gives
// the member exactly what the form said it would.
func defaultConnectionName(model string) string {
	suffix, err := utils.RandomHex(4)
	if err != nil {
		return model
	}
	return model + "-" + suffix
}

// List is every connection the caller may spend, their own first. Without
// tokens: this is what the settings screen renders.
func (h *AIHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	connections, err := h.connections.ListFor(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, connections)
}

// Providers is the catalogue: what this deployment knows how to talk to, the
// address and model each one defaults to, and the models worth suggesting -
// so the form fills itself in as the member picks, and its placeholders say
// what this provider's values look like rather than the last one's.
func (h *AIHandler) Providers(w http.ResponseWriter, r *http.Request) {
	providers, err := h.connections.ListProviders(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, providers)
}

func (h *AIHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p aiConnectionPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	connection, ok := h.normalize(w, r, p)
	if !ok {
		return
	}
	if utils.IsBlank(connection.Token) {
		writeError(w, http.StatusBadRequest, "A key is required", CodeAIKeyRequired)
		return
	}

	created, err := h.connections.Create(r.Context(), userID, connection)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	created.Token = ""
	writeJSON(w, http.StatusCreated, created)
}

func (h *AIHandler) Update(w http.ResponseWriter, r *http.Request) {
	owned, ok := h.ownConnection(w, r)
	if !ok {
		return
	}

	var p aiConnectionPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	connection, ok := h.normalize(w, r, p)
	if !ok {
		return
	}

	updated, err := h.connections.Update(r.Context(), owned.ID, connection)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	updated.Token = ""
	writeJSON(w, http.StatusOK, updated)
}

type aiOrderPayload struct {
	ConnectionIDs []string `json:"connectionIds"`
}

// Reorder sets which connection is first - the one a question goes through
// when nobody picks it, and the first row of every "ask with" menu. A real
// setting, the way a storage's order is.
func (h *AIHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p aiOrderPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if err := h.connections.Reorder(r.Context(), userID, p.ConnectionIDs); err != nil {
		writeStoreError(w, err)
		return
	}
	// The list back, in its new order: one answer rather than a 204 and a
	// second request to find out what happened.
	h.List(w, r)
}

func (h *AIHandler) Delete(w http.ResponseWriter, r *http.Request) {
	owned, ok := h.ownConnection(w, r)
	if !ok {
		return
	}
	if err := h.connections.Delete(r.Context(), owned.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ownConnection resolves the addressed connection and refuses anybody but its
// owner. Shared access is the right to spend a connection, never to edit or
// re-share it.
func (h *AIHandler) ownConnection(w http.ResponseWriter, r *http.Request) (models.AIConnection, bool) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	connection, err := h.connections.FindByID(r.Context(), chi.URLParam(r, "connectionId"))
	if err != nil {
		writeStoreError(w, err)
		return models.AIConnection{}, false
	}
	if connection.OwnerID != userID {
		// Not "forbidden": whether somebody else's connection exists is not a
		// question this answers.
		writeError(w, http.StatusNotFound, "Connection not found", CodeNotFound)
		return models.AIConnection{}, false
	}
	return connection, true
}

func (h *AIHandler) ListGrants(w http.ResponseWriter, r *http.Request) {
	owned, ok := h.ownConnection(w, r)
	if !ok {
		return
	}
	grants, err := h.connections.ListGrants(r.Context(), owned.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grants)
}

type aiGrantPayload struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

// Share lets somebody else put questions through this connection.
func (h *AIHandler) Share(w http.ResponseWriter, r *http.Request) {
	owned, ok := h.ownConnection(w, r)
	if !ok {
		return
	}

	var p aiGrantPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	// One role to grant, so a blank one is the obvious intent rather than an
	// error to bounce back.
	role := strings.TrimSpace(p.Role)
	if role == "" {
		role = models.AIRoleUser
	}
	if !models.IsValidAIRole(role) {
		writeError(w, http.StatusBadRequest, "Invalid role", CodeInvalidAIRole)
		return
	}
	if p.UserID == owned.OwnerID {
		// The owner already has every right there is, and a second row for
		// them would let a later revoke take their own connection away.
		writeError(w, http.StatusBadRequest, "This connection is already yours", CodeInvalidAIRole)
		return
	}
	if _, err := h.users.FindByID(r.Context(), p.UserID); err != nil {
		writeStoreError(w, err)
		return
	}

	if err := h.connections.Grant(r.Context(), owned.ID, p.UserID, role); err != nil {
		writeStoreError(w, err)
		return
	}
	grants, err := h.connections.ListGrants(r.Context(), owned.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grants)
}

func (h *AIHandler) Unshare(w http.ResponseWriter, r *http.Request) {
	owned, ok := h.ownConnection(w, r)
	if !ok {
		return
	}
	if err := h.connections.Revoke(r.Context(), owned.ID, chi.URLParam(r, "userId")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- asking ---

type chatSettings struct {
	MaxTokens int `json:"max_tokens"`
}

// chatPayload is the shape cwclock's web agent takes, so a client written
// against one speaks to the other: a conversation plus the turn being added.
type chatPayload struct {
	Settings chatSettings `json:"settings"`
	Message  string       `json:"message"`
	// Messages is the thread, for a client holding one that is not stored -
	// ignored when ConversationID names a stored thread, whose own exchanges
	// are the history. The server's record of what was said is the one that
	// cannot be edited on the way back in.
	Messages []models.ChatMessage `json:"messages"`
	// ConversationID continues a stored thread. Blank starts one: every
	// question asked here is kept, so a member can come back to what they
	// were told.
	ConversationID string `json:"conversationId"`
	// ConnectionID is which subscription to spend. Blank is the member's
	// default - the picker only matters to somebody who has more than one.
	ConnectionID string `json:"connectionId"`
}

type chatResponse struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Usage   map[string]any `json:"usage,omitempty"`
	Model   string         `json:"model,omitempty"`
	// ConversationID is the thread this went into - the one just created, when
	// the question started one - and Title what it is called.
	ConversationID string `json:"conversationId,omitempty"`
	Title          string `json:"title,omitempty"`
}

// maxChatHistory bounds how much of a conversation is sent back to the model.
// A long thread costs more every turn and adds little: what matters is the
// last few exchanges and the skill.
const maxChatHistory = 20

// Chat answers a question as a coach, and keeps the exchange.
//
// The connection spent is the caller's own, or the first shared with them -
// never chosen per question. Somebody with one key should not have to pick it,
// and somebody using their coach's should not have to know whose it is.
//
// Every question and answer is stored as a conversation, in cwcloud's shape: a
// titled thread, a row per exchange. A chat that forgets itself on reload is
// one nobody uses twice, and an answer that cost money to produce is worth
// more than a browser tab.
func (h *AIHandler) Chat(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p chatPayload
	if !decodeJSON(w, r, &p) {
		return
	}

	// A stored thread is its own history. Taking it from the request instead
	// would let a client rewrite what it was told two turns ago, and would
	// make the thread the model sees differ from the one on screen.
	var conversation models.AIConversation
	history := p.Messages
	if utils.IsNotBlank(p.ConversationID) {
		stored, err := h.connections.FindConversation(r.Context(), p.ConversationID, userID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		conversation = stored
		history = historyOf(stored)
	}

	question := strings.TrimSpace(p.Message)
	if question != "" {
		history = append(history, models.ChatMessage{Role: models.ChatRoleUser, Message: question})
	}
	if len(history) == 0 {
		writeError(w, http.StatusBadRequest, "Ask something", CodeEmptyMessage)
		return
	}
	if question == "" {
		// A client replaying a thread without a new turn: the question being
		// answered is the last one in it.
		question = lastQuestion(history)
	}
	if len(history) > maxChatHistory {
		history = history[len(history)-maxChatHistory:]
	}

	// The skill, then who is asking, then the language. The member's own
	// numbers travel with every question rather than being asked for: a coach
	// does not ask what you squat every time you speak to them.
	locale := h.localeOf(r, userID)
	tools := h.tools()
	system := templates.CoachSkill + "\n\n" + h.memberContext(r, userID, locale) +
		"\n\n" + ChatLanguageRule(locale)
	if len(tools) > 0 {
		system += "\n\n" + templates.AssistantToolSkill
	}
	answer, ok := h.ask(w, r, userID, p.ConnectionID, "chat", ai.Request{
		System:    system,
		Messages:  turnsOf(history),
		Tools:     tools,
		MaxTokens: p.Settings.MaxTokens,
	})
	if !ok {
		return
	}

	// Stored after the answer, not before: a thread holding a question that
	// was never answered is a thread with a hole in it, and the failure the
	// member saw was a provider refusing, not a conversation they had.
	if conversation.ID == "" {
		id, err := h.connections.CreateConversation(r.Context(), userID,
			models.AIConversationTitle(question))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		conversation.ID, conversation.Title = id, models.AIConversationTitle(question)
	}

	connection, _ := h.usable(r, userID, p.ConnectionID)
	if err := h.connections.AppendExchange(r.Context(), conversation.ID, models.AIExchange{
		Question: question, Answer: answer.Message,
		Provider: connection.Provider, Model: answeredWith(answer.Model, connection.Model),
		Connection: connection.Name, Usage: answer.Usage,
	}); err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, chatResponse{
		Status: "ok", Message: answer.Message, Usage: answer.Usage, Model: answer.Model,
		ConversationID: conversation.ID, Title: conversation.Title,
	})
}

// --- cwcloud external adapter (ai-instruct-43) ---

// cwAdapterPayload is the external-adapter contract CWCloud calls with
// (cwcloud.tech/docs/tutorials/cwai#external-adapter-interface). It arrives in
// two shapes: a single `message`, or a `messages` thread of role-tagged turns.
// Both name the subscription to spend in `adapter` and cap the answer with
// `settings.max_tokens`.
type cwAdapterPayload struct {
	Adapter  string       `json:"adapter"`
	Settings chatSettings `json:"settings"`
	// Message is the single-prompt shape.
	Message string `json:"message"`
	// Messages is the role-tagged thread shape: a "system" turn becomes the
	// model's instruction, the rest the conversation. Whether it is present is
	// also what decides the answer's shape (choices vs a bare message).
	Messages []models.ChatMessage `json:"messages"`
	// Regenerate and EnableHistory are part of the contract. This adapter is
	// stateless - cwcloud sends the whole thread each call, and the contract
	// carries no conversation id to continue a stored one - so they are parsed
	// but not acted on. The spend is still metered and audited (see ask).
	Regenerate    bool `json:"regenerate"`
	EnableHistory bool `json:"enable_history"`
}

type cwAdapterUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type cwAdapterChoiceMessage struct {
	Content string `json:"content"`
}

type cwAdapterChoice struct {
	Message cwAdapterChoiceMessage `json:"message"`
}

// cwAdapterResponse is the contract's answer: the text in `message` for a
// single-prompt call, or in `choices` for a thread call - the shape cwcloud
// sent decides which - with the token count named the way the contract names it.
type cwAdapterResponse struct {
	Status  string            `json:"status"`
	Message string            `json:"message,omitempty"`
	Choices []cwAdapterChoice `json:"choices,omitempty"`
	Usage   cwAdapterUsage    `json:"usage"`
}

// CWCloudAdapter answers a prompt for CWCloud through its external-adapter
// interface, spending the caller's AI subscription. It is the chat path put
// behind cwcloud's contract: the same model call, metering and audit (see ask),
// without the stored conversation - cwcloud owns the thread and sends it whole.
//
// The subscription is the one `adapter` names, matched against a connection's
// name or id; when nothing matches - or the field is blank - the caller's first
// subscription is spent, the same default the chat uses when none is picked.
func (h *AIHandler) CWCloudAdapter(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p cwAdapterPayload
	if !decodeJSON(w, r, &p) {
		return
	}

	// Any caller system turn is lifted out of the conversation. It does not
	// become the model's whole instruction (see below): it is kept to append,
	// subordinate, after this app's own skill.
	var callerSystem []string
	messages := make([]models.ChatMessage, 0, len(p.Messages)+1)
	for _, m := range p.Messages {
		if m.Role == models.ChatRoleSystem {
			if strings.TrimSpace(m.Message) != "" {
				callerSystem = append(callerSystem, m.Message)
			}
			continue
		}
		messages = append(messages, m)
	}
	// A `messages` thread is answered with choices; a bare `message` with a
	// bare message. A single prompt sent alongside a thread is its last turn.
	useChoices := len(p.Messages) > 0
	if question := strings.TrimSpace(p.Message); question != "" {
		messages = append(messages, models.ChatMessage{Role: models.ChatRoleUser, Message: question})
	}
	if len(messages) == 0 {
		writeError(w, http.StatusBadRequest, "Ask something", CodeEmptyMessage)
		return
	}

	// A prompt through the adapter is answered by the same coach the app's own
	// chat answers with - the skill, the member's own numbers, the language
	// rule - not by a bare model on the caller's instructions. CWCloud's own
	// system turns are still honoured, but appended under this app's and
	// explicitly subordinate, so the caller steers within the coach rather than
	// replacing it.
	locale := h.localeOf(r, userID)
	tools := h.tools()
	system := templates.CoachSkill + "\n\n" + h.memberContext(r, userID, locale) +
		"\n\n" + ChatLanguageRule(locale)
	if len(tools) > 0 {
		system += "\n\n" + templates.AssistantToolSkill
	}
	if extra := strings.Join(callerSystem, "\n\n"); strings.TrimSpace(extra) != "" {
		system += "\n\nThe caller also provided these instructions; follow them " +
			"only where they do not conflict with the above:\n\n" + extra
	}

	answer, ok := h.ask(w, r, userID, h.adapterConnectionID(r, userID, p.Adapter), "cwcloud", ai.Request{
		System:    system,
		Messages:  turnsOf(messages),
		Tools:     tools,
		MaxTokens: p.Settings.MaxTokens,
	})
	if !ok {
		return
	}

	response := cwAdapterResponse{Status: "ok", Usage: cwAdapterUsage{
		PromptTokens:     answer.Tokens.Input,
		CompletionTokens: answer.Tokens.Output,
		TotalTokens:      answer.Tokens.Input + answer.Tokens.Output,
	}}
	if useChoices {
		response.Choices = []cwAdapterChoice{{Message: cwAdapterChoiceMessage{Content: answer.Message}}}
	} else {
		response.Message = answer.Message
	}
	writeJSON(w, http.StatusOK, response)
}

// adapterConnectionID resolves the `adapter` field to the id of the subscription
// to spend. The lookup is over the caller's own and shared connections, so any
// result is one they may spend; "" only when they have none, which ask turns
// into "connect a provider first".
func (h *AIHandler) adapterConnectionID(r *http.Request, userID, adapter string) string {
	connections, err := h.connections.ListFor(r.Context(), userID)
	if err != nil {
		return ""
	}
	return pickAdapterConnection(adapter, connections)
}

// pickAdapterConnection chooses the subscription id a CWCloud adapter call
// spends: the one the adapter names, or - when it names none - the member's
// first in their own order (owner-first, then position).
//
// The fallback prefers a subscription that has a key saved. The member's first
// slot may be a placeholder with no credential, and spending that one would
// answer "connect a provider first" while a connected subscription sits right
// behind it; so the first key-bearing one is taken, falling back to the very
// first only when none has a key (then the error is a truthful one). "" only
// when there are no subscriptions at all.
func pickAdapterConnection(adapter string, connections []models.AIConnection) string {
	if len(connections) == 0 {
		return ""
	}
	if id := matchAdapterConnection(adapter, connections); id != "" {
		return id
	}
	for _, c := range connections {
		if c.HasToken {
			return c.ID
		}
	}
	return connections[0].ID
}

// matchAdapterConnection picks the subscription a CWCloud `adapter` label names,
// or "" when none fits.
//
// The adapter is a label written in cwcloud's own catalogue ("strong-fish big
// model", "coach (claude)"), so the containment runs the caller's way round:
// the ADAPTER carries the subscription's name or id, not the other way. "big"
// does not find "Big model" - somebody asking for "big" did not ask for it -
// but "ask the big model please" does.
//
// Most certain first: the adapter IS an id, then IS a name (case-insensitive),
// then CONTAINS an id, then CONTAINS a name - the longest such name, so an
// adapter carrying "my daily driver setup" takes "Daily driver" over "driver".
// Ties below the longest-name rule fall to the list order (own before shared,
// then position). A blank name matches nothing, so it is never read as carried
// by every adapter. Nothing matching leaves "" - the default subscription stands.
func matchAdapterConnection(adapter string, connections []models.AIConnection) string {
	adapter = strings.TrimSpace(adapter)
	if adapter == "" {
		return ""
	}
	lower := strings.ToLower(adapter)

	var exactName, containsID, longestName string
	longestLen := 0
	for _, c := range connections {
		// The adapter IS the id: a uuid is unique, so this is the answer.
		if strings.EqualFold(c.ID, adapter) {
			return c.ID
		}
		name := strings.TrimSpace(c.Name)
		switch {
		case name != "" && strings.EqualFold(name, adapter):
			if exactName == "" {
				exactName = c.ID
			}
		case c.ID != "" && strings.Contains(lower, strings.ToLower(c.ID)):
			if containsID == "" {
				containsID = c.ID
			}
		case name != "" && strings.Contains(lower, strings.ToLower(name)):
			if len(name) > longestLen {
				longestLen, longestName = len(name), c.ID
			}
		}
	}
	switch {
	case exactName != "":
		return exactName
	case containsID != "":
		return containsID
	default:
		return longestName
	}
}

// answeredWith is which model to record against an answer: the one the
// provider reported, or the one the subscription is set to.
//
// Two because they disagree: Anthropic echoes the model back, an
// OpenAI-compatible gateway may answer with its own routing name, and some
// report nothing at all - and an answer with no model against it is one
// nobody can tell apart from the next.
func answeredWith(reported, configured string) string {
	for _, candidate := range []string{reported, configured} {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}
	return ""
}

// historyOf flattens a stored thread into turns, bounded the same way a
// client-held one is.
func historyOf(conversation models.AIConversation) []models.ChatMessage {
	history := make([]models.ChatMessage, 0, len(conversation.Exchanges)*2)
	for _, exchange := range conversation.Exchanges {
		history = append(history,
			models.ChatMessage{Role: models.ChatRoleUser, Message: exchange.Question},
			models.ChatMessage{Role: models.ChatRoleAssistant, Message: exchange.Answer})
	}
	return history
}

// lastQuestion is the most recent thing the member said, which is what an
// exchange is stored under.
func lastQuestion(history []models.ChatMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == models.ChatRoleUser {
			return history[i].Message
		}
	}
	return history[len(history)-1].Message
}

// localeOf is the language this member reads the app in.
//
// Their configured one, not the language of the question: somebody reading
// the app in French is reading French, and a model left to guess answers in
// whatever the last exercise name looked like. English when nothing is set.
func (h *AIHandler) localeOf(r *http.Request, userID string) string {
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		return "en"
	}
	return localeOf(user, r)
}

// LanguageRule pins a single language, named for the model. It is the fitness
// reading's rule: that one is generated from a button, with nothing in it for
// the model to take a language from, so it answers in the member's configured
// language outright.
func LanguageRule(locale string) string {
	return "Answer in " + languageName(locale) + "."
}

// ChatLanguageRule lets the member's own words lead: answer in the language of
// their latest message, and fall back to their configured language only when
// that is unclear (a one-word question, an emoji). The prompt comes first, the
// account setting second.
func ChatLanguageRule(locale string) string {
	return "Answer in the language of the member's latest message. " +
		"If that is unclear, answer in " + languageName(locale) + "."
}

// SessionLanguageRule lets the athlete's own comments lead: answer in the
// language they wrote their set comments in, and fall back to their configured
// language only when they left none. The feedback detected in the session comes
// first, the account setting second.
func SessionLanguageRule(locale string) string {
	return "Answer in the language the athlete wrote their set comments in. " +
		"If they left no comments, answer in " + languageName(locale) + "."
}

// languageName is the locale's language, named in English because that is the
// instruction's own language.
func languageName(locale string) string {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "fr":
		return "French"
	case "ar":
		return "Arabic"
	case "es":
		return "Spanish"
	case "de":
		return "German"
	case "it":
		return "Italian"
	case "pt":
		return "Portuguese"
	case "nl":
		return "Dutch"
	}
	return "English"
}

// usable is the connection a question goes through: the one asked for, or the
// member's default.
func (h *AIHandler) usable(r *http.Request, userID, connectionID string) (models.AIConnection, error) {
	if strings.TrimSpace(connectionID) == "" {
		return h.connections.Usable(r.Context(), userID)
	}
	return h.connections.UsableByID(r.Context(), userID, strings.TrimSpace(connectionID))
}

// ask resolves the caller's connection and puts the question, turning the two
// failures a member can act on into codes the clients translate.
//
// feature says what the tokens are being spent on - "chat", "session",
// "health" - which is a label on the meter below rather than anything the
// model is told.
func (h *AIHandler) ask(w http.ResponseWriter, r *http.Request, userID, connectionID, feature string,
	request ai.Request) (ai.Response, bool) {
	// The one the member picked, or their default when they did not say.
	// Somebody with a single key should never have to choose it; somebody
	// with their own and their coach's should always be able to.
	connection, err := h.usable(r, userID, connectionID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound) && connectionID != "":
			// Theirs to spend or not there at all: the two are the same
			// answer, and which it is is not this endpoint's to disclose.
			writeError(w, http.StatusNotFound, "Connection not found", CodeNotFound)
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusPreconditionRequired,
				"Connect an AI provider first", CodeAINotConfigured)
		default:
			writeStoreError(w, err)
		}
		return ai.Response{}, false
	}

	answer, err := h.converse(r, connection, request)
	if err != nil {
		var refused ai.ErrRefused
		switch {
		case errors.Is(err, ai.ErrNotConfigured):
			writeError(w, http.StatusPreconditionRequired,
				"Connect an AI provider first", CodeAINotConfigured)
		case errors.As(err, &refused):
			// The provider's own refusal, logged with its words: a bad key, a
			// model the account cannot reach and an exhausted quota are
			// different problems the member alone can fix. The provider and its
			// status and body travel to the client too, so the troubleshoot
			// console can show them - the same shape a device read uses.
			slog.Error("ai: the provider refused",
				"provider", connection.Provider, "model", connection.Model, "error", err)
			writeJSON(w, http.StatusBadGateway, errorBody{
				Message:  "The AI provider refused the request",
				I18nCode: CodeAIRefused,
				Detail:   err.Error(),
				Provider: connection.Provider,
			})
		default:
			slog.Error("ai: the provider could not be reached",
				"provider", connection.Provider, "error", err)
			writeJSON(w, http.StatusBadGateway, errorBody{
				Message:  "The AI provider could not be reached",
				I18nCode: CodeUpstream,
				Detail:   err.Error(),
				Provider: connection.Provider,
			})
		}
		return ai.Response{}, false
	}

	// What it cost, against the key that paid for it. Recorded here because
	// here is the one place every feature's question goes through, so no
	// future caller can spend somebody's tokens without it being counted.
	model := answer.Model
	if model == "" {
		model = connection.Model
	}
	metrics.RecordAISpend(r.Context(), metrics.AISpend{
		UserID: userID, ConnectionID: connection.ID, OwnerID: connection.OwnerID,
		Provider: connection.Provider, Model: model, Feature: feature,
		Input: answer.Tokens.Input, Output: answer.Tokens.Output, Cached: answer.Tokens.Cached,
	})
	// And, for the superadmin audit trail, who spent what on which key. Same
	// chokepoint, same reason: every feature's question is counted here, so none
	// can reach the model without leaving a record. Fire-and-forget (see
	// AuditStore.Record) - a nil store is a no-op.
	if h.audit != nil {
		h.audit.Record(r.Context(), userID, models.AuditActionAICall,
			models.AuditSubjectAIConnection, connection.ID, map[string]any{
				"provider": connection.Provider,
				"model":    model,
				"feature":  feature,
				"input":    answer.Tokens.Input,
				"output":   answer.Tokens.Output,
				"cached":   answer.Tokens.Cached,
			})
	}
	return answer, true
}

// noToolsRule replaces the tools for a provider that has none (CWCloud's
// prompt API today). It is deliberately blunt: the alternative to saying this
// is an assistant that reports sets it never read.
const noToolsRule = `# No tools in this configuration

The AI connection answering this question reaches a provider that cannot call ` +
	"tools, so `strong_fish_api` is NOT available to you on this turn, whatever " +
	`the rest of these instructions say about it.

Answer from the conversation and the member context above, and from nothing ` +
	`else. Where an answer would need their own rows - a programme's days, a ` +
	`logged set, a bar-speed history, a 1RM you have not been given - say plainly ` +
	`that you cannot read them through this connection and name the screen that ` +
	`holds them: "My training" for programmes and logged sessions, "My 1RMs" for ` +
	`maxes, "Fitness data" for steps, sleep and body composition. Never say you ` +
	`looked something up, never produce a figure as if it came from their data, ` +
	`and never claim to have recorded a session - recording one takes a tool you ` +
	`do not have, so tell them to log it on the session screen themselves. Never ` +
	`offer a file or a download link: a file can only be produced by a tool.`

// maxToolRounds bounds the agent loop: how many times the model may call tools
// before it must answer. Each round is one more provider call, so it is kept
// small; after the cap a final call is made with the tools withheld, so the
// loop always ends in prose rather than in another request to run a tool.
const maxToolRounds = 4

// converse runs the question as an agent. It offers the request's tools, and
// whenever the model answers with tool calls rather than prose it runs them
// (runTool), appends the call and its results to the transcript and asks again,
// up to maxToolRounds. The token cost is summed over every round, because each
// round is billed; a request carrying no tools makes exactly one call and this
// is a plain chat.
func (h *AIHandler) converse(r *http.Request, connection models.AIConnection,
	request ai.Request) (ai.Response, error) {
	// A format that cannot be handed tools is asked without them, and told
	// so. Here rather than at each call site because here is the one place
	// every feature's question passes through, and because the connection -
	// and so its format - is only known once it has been resolved. The skill
	// spends paragraphs on reading the API before answering, and a model given
	// those instructions with no way to obey them does not stay silent about
	// it: it says it looked something up, or that it logged the set.
	if !models.AIFormatSupportsTools(connection.Format) {
		request.Tools = nil
		request.System = strings.TrimRight(request.System, "\n") + "\n\n" + noToolsRule
	}

	var spent ai.Usage
	for round := 0; round < maxToolRounds; round++ {
		answer, err := h.client.Ask(r.Context(), connection, request)
		if err != nil {
			return ai.Response{}, err
		}
		spent = spent.Plus(answer.Tokens)
		if len(answer.ToolCalls) == 0 {
			answer.Tokens = spent
			return answer, nil
		}
		// Keep the assistant's own tool-call turn, then one result per call, so
		// the next round sees what it asked and what it got back.
		request.Messages = append(request.Messages, ai.Turn{
			Role: models.ChatRoleAssistant, Text: answer.Message, ToolCalls: answer.ToolCalls,
		})
		for _, call := range answer.ToolCalls {
			request.Messages = append(request.Messages, ai.Turn{
				Role: ai.RoleTool, ToolCallID: call.ID, ToolName: call.Name,
				Text: h.runTool(r, call),
			})
		}
	}
	// Out of rounds: one last answer with the tools withheld, so the model turns
	// what it gathered into prose instead of asking for another call.
	request.Tools = nil
	answer, err := h.client.Ask(r.Context(), connection, request)
	if err != nil {
		return ai.Response{}, err
	}
	answer.Tokens = spent.Plus(answer.Tokens)
	return answer, nil
}

// turnsOf flattens plain chat messages into the ai.Turn transcript the client
// takes - the shape that can also carry an agent's tool calls, which a bare
// message cannot.
func turnsOf(messages []models.ChatMessage) []ai.Turn {
	turns := make([]ai.Turn, 0, len(messages))
	for _, m := range messages {
		turns = append(turns, ai.TurnOf(m.Role, m.Message))
	}
	return turns
}
