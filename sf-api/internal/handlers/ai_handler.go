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
	client      *ai.Client
	// uiBaseURL is where a shared conversation is read, and the attribution
	// line of an exported one.
	uiBaseURL string
}

func NewAIHandler(connections *store.AIStore, users *store.UserStore, uiBaseURL string) *AIHandler {
	return &AIHandler{connections: connections, users: users, client: ai.New(), uiBaseURL: uiBaseURL}
}

type aiConnectionPayload struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
	Token    string `json:"token"`
}

// normalize fills in what the member left blank and refuses what cannot work.
//
// The URL and the model have defaults because the common case is somebody
// pasting a key and expecting it to work; they stay editable because pointing
// a connection at a local gateway is the reason the field exists.
func (h *AIHandler) normalize(w http.ResponseWriter, p aiConnectionPayload) (models.AIConnection, bool) {
	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	if !models.IsValidAIProvider(provider) {
		writeError(w, http.StatusBadRequest, "Pick a provider", CodeInvalidAIProvider)
		return models.AIConnection{}, false
	}

	baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if baseURL == "" {
		baseURL = models.DefaultAIBaseURL(provider)
	}
	// Anything but http(s) would be a request this API makes on a member's
	// behalf to somewhere it should not - a file, an internal scheme.
	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://") {
		writeError(w, http.StatusBadRequest, "The address must be an http or https URL", CodeInvalidAIBaseURL)
		return models.AIConnection{}, false
	}

	model := strings.TrimSpace(p.Model)
	if model == "" {
		model = models.DefaultAIModel(provider)
	}

	return models.AIConnection{
		Name: strings.TrimSpace(p.Name), Provider: provider,
		BaseURL: baseURL, Model: model, Token: strings.TrimSpace(p.Token),
	}, true
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

// Providers is what this build knows how to talk to, with the address and
// model each one defaults to - so the form can fill itself in as the member
// picks, rather than making them look the values up.
func (h *AIHandler) Providers(w http.ResponseWriter, r *http.Request) {
	type provider struct {
		Name         string `json:"name"`
		DefaultURL   string `json:"defaultUrl"`
		DefaultModel string `json:"defaultModel"`
	}
	list := make([]provider, 0, len(models.AIProviders))
	for _, name := range models.AIProviders {
		list = append(list, provider{
			Name:         name,
			DefaultURL:   models.DefaultAIBaseURL(name),
			DefaultModel: models.DefaultAIModel(name),
		})
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *AIHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p aiConnectionPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	connection, ok := h.normalize(w, p)
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
	connection, ok := h.normalize(w, p)
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

	answer, ok := h.ask(w, r, userID, "chat", ai.Request{
		System:    templates.CoachSkill + "\n\n" + h.languageRule(r, userID),
		Messages:  history,
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

	connection, _ := h.connections.Usable(r.Context(), userID)
	if err := h.connections.AppendExchange(r.Context(), conversation.ID, models.AIExchange{
		Question: question, Answer: answer.Message,
		Provider: connection.Provider, Model: answer.Model, Usage: answer.Usage,
	}); err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, chatResponse{
		Status: "ok", Message: answer.Message, Usage: answer.Usage, Model: answer.Model,
		ConversationID: conversation.ID, Title: conversation.Title,
	})
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

// languageRule tells the model which language to answer in.
//
// The member's configured language, not the language of the question: somebody
// reading the app in French is reading French, and a model left to guess
// answers in whatever the last exercise name looked like. English when nothing
// is configured.
func (h *AIHandler) languageRule(r *http.Request, userID string) string {
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		return LanguageRule("en")
	}
	return LanguageRule(localeOf(user, r))
}

// LanguageRule is that instruction for a locale, shared with the session
// reading - which is generated from a button rather than from a question, so
// there is nothing in it for the model to take a language from at all.
func LanguageRule(locale string) string {
	return "Answer in " + languageName(locale) + "."
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

// ask resolves the caller's connection and puts the question, turning the two
// failures a member can act on into codes the clients translate.
//
// feature says what the tokens are being spent on - "chat", "session",
// "health" - which is a label on the meter below rather than anything the
// model is told.
func (h *AIHandler) ask(w http.ResponseWriter, r *http.Request, userID, feature string,
	request ai.Request) (ai.Response, bool) {
	connection, err := h.connections.Usable(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusPreconditionRequired,
				"Connect an AI provider first", CodeAINotConfigured)
			return ai.Response{}, false
		}
		writeStoreError(w, err)
		return ai.Response{}, false
	}

	answer, err := h.client.Ask(r.Context(), connection, request)
	if err != nil {
		var refused ai.ErrRefused
		switch {
		case errors.Is(err, ai.ErrNotConfigured):
			writeError(w, http.StatusPreconditionRequired,
				"Connect an AI provider first", CodeAINotConfigured)
		case errors.As(err, &refused):
			// The provider's own refusal, logged with its words: a bad key, a
			// model the account cannot reach and an exhausted quota are
			// different problems the member alone can fix.
			slog.Error("ai: the provider refused",
				"provider", connection.Provider, "model", connection.Model, "error", err)
			writeError(w, http.StatusBadGateway, "The AI provider refused the request", CodeAIRefused)
		default:
			slog.Error("ai: the provider could not be reached",
				"provider", connection.Provider, "error", err)
			writeError(w, http.StatusBadGateway, "The AI provider could not be reached", CodeUpstream)
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
	return answer, true
}
