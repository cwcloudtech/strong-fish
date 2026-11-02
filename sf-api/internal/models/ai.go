package models

import (
	"strings"
	"time"
)

// The language model providers a connection can point at.
//
// A provider is a wire format and a default address, not a vendor lock: the
// base URL is the member's to change, which is how one of these points at a
// local LiteLLM, an OpenRouter account or anything else speaking the same
// protocol. That is why there are three names here and not a dozen - what
// differs between a dozen services is the URL, not the format.
const (
	// AIProviderOpenAI is the OpenAI chat-completions format, which is also
	// what Mistral, DeepSeek, OpenRouter, Groq, Ollama and LiteLLM speak.
	AIProviderOpenAI = "openai"
	// AIProviderMistral is the same format at Mistral's own address, kept
	// apart so the picker names a service a member recognises.
	AIProviderMistral = "mistral"
	// AIProviderAnthropic is Claude's own /messages format.
	AIProviderAnthropic = "anthropic"
	// AIProviderGemini is Google's generateContent format.
	AIProviderGemini = "gemini"
)

// AIProviders is every provider, in the order a picker lists them.
var AIProviders = []string{
	AIProviderOpenAI, AIProviderMistral, AIProviderAnthropic, AIProviderGemini,
}

// IsValidAIProvider reports whether name is one this build knows how to talk
// to.
func IsValidAIProvider(name string) bool {
	for _, provider := range AIProviders {
		if provider == name {
			return true
		}
	}
	return false
}

// DefaultAIBaseURL is where a provider lives unless the member says otherwise.
// Offered as a starting point in the form rather than imposed: pointing a
// connection at a local gateway is the reason the field is editable.
func DefaultAIBaseURL(provider string) string {
	switch provider {
	case AIProviderOpenAI:
		return "https://api.openai.com/v1"
	case AIProviderMistral:
		return "https://api.mistral.ai/v1"
	case AIProviderAnthropic:
		return "https://api.anthropic.com/v1"
	case AIProviderGemini:
		return "https://generativelanguage.googleapis.com/v1beta"
	}
	return ""
}

// DefaultAIModel is a reasonable model for a provider, so a member who has
// only pasted a key gets an answer rather than an error about a missing model.
func DefaultAIModel(provider string) string {
	switch provider {
	case AIProviderOpenAI:
		return "gpt-4o-mini"
	case AIProviderMistral:
		return "mistral-small-latest"
	case AIProviderAnthropic:
		return "claude-sonnet-4-5"
	case AIProviderGemini:
		return "gemini-2.0-flash"
	}
	return ""
}

// Roles on a connection.
//
// Two, not storage's three: there is no reading a connection without spending
// it, so "may look at it" is not a thing worth granting.
const (
	AIRoleOwner = "owner"
	AIRoleUser  = "user"
)

// IsValidAIRole reports whether role is one an owner may grant.
func IsValidAIRole(role string) bool { return role == AIRoleUser }

// CanUseAI reports whether role may ask the model.
func CanUseAI(role string) bool { return role == AIRoleOwner || role == AIRoleUser }

// AIConnection is one account's access to a model.
type AIConnection struct {
	ID      string `json:"id"`
	OwnerID string `json:"ownerId"`
	// Name is what the member calls it - "my Mistral key", "the club's
	// LiteLLM". Blank falls back to the provider's name on screen.
	Name     string `json:"name,omitempty"`
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model,omitempty"`
	// Token is write-only. It is set through the form and never sent back:
	// what a client needs to render is whether one is stored, not what it is.
	Token string `json:"-"`
	// HasToken is that answer, so a form can say "a key is saved" and offer
	// to replace it rather than showing a blank field that would wipe it.
	HasToken bool `json:"hasToken"`
	// Role is the caller's own role, filled in by the queries that read a
	// connection for somebody.
	Role      string    `json:"role,omitempty"`
	OwnerName string    `json:"ownerName,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Configured reports whether this connection has enough to attempt a call. A
// shape check, not a reachability one: the first question is what finds out
// whether the key actually works.
func (c AIConnection) Configured() bool {
	return IsValidAIProvider(c.Provider) &&
		strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.Token) != ""
}

// AIGrant is one line of a connection's access list, as its owner reads it.
type AIGrant struct {
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Handle    string    `json:"handle,omitempty"`
	Picture   string    `json:"picture,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// ChatMessage is one turn of a conversation, in the shape cwclock's web agent
// uses - role plus message, rather than the provider-specific "content" shapes
// underneath.
type ChatMessage struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

// Chat roles.
const (
	ChatRoleUser      = "user"
	ChatRoleAssistant = "assistant"
	ChatRoleSystem    = "system"
)

// SessionFeedback is what the model said about one session, as stored.
//
// Kept rather than regenerated on every visit: the answer costs money, two
// askings of the same question give two different readings, and a member
// should be able to come back in March to what it told them. Deleting it is
// how you ask for a new one.
type SessionFeedback struct {
	ID           string `json:"id"`
	AssignmentID string `json:"assignmentId"`
	DayID        string `json:"dayId"`
	AuthorID     string `json:"authorId"`
	AuthorName   string `json:"authorName,omitempty"`
	Content      string `json:"content"`
	// Provider and Model say what produced it, so a reading from a small
	// local model is not later mistaken for one from a large hosted one.
	Provider  string    `json:"provider,omitempty"`
	Model     string    `json:"model,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AIConversation is a thread of questions and answers.
//
// Shaped on cwcloud's prompt_list: a title derived from the first question,
// and the exchanges under it.
type AIConversation struct {
	ID string `json:"id"`
	// OwnerID is whose thread it is, which a shared read does not disclose.
	OwnerID string `json:"ownerId,omitempty"`
	Title   string `json:"title"`
	// Shared is whether a link exists. The token itself only goes out once,
	// in the response to sharing it, and to the owner asking for it back -
	// a list of everybody's conversations carrying their share tokens would
	// be a list of readable links.
	Shared     bool   `json:"shared"`
	ShareToken string `json:"shareToken,omitempty"`
	// ShareURL is that token as the link to hand somebody, filled in by the
	// API so a client does not have to know how a shared thread is addressed.
	ShareURL  string       `json:"shareUrl,omitempty"`
	Exchanges []AIExchange `json:"exchanges,omitempty"`
	// Count is how many exchanges it holds, for a list that shows the thread
	// without loading it.
	Count     int       `json:"count"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AIExchange is one question and the answer it got.
type AIExchange struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	// Usage is what the provider reported spending, kept so somebody lending
	// a key can see where it went.
	Usage     map[string]any `json:"usage,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

// AIConversationTitle is what a thread is called before anybody renames it: the
// first question, cut at a word boundary.
//
// Derived rather than asked for. Nobody titles a conversation they are about
// to have, and a list of "New chat" tells a member nothing about which one to
// open.
func AIConversationTitle(question string) string {
	const limit = 60
	title := strings.Join(strings.Fields(question), " ")
	if title == "" {
		return "New conversation"
	}
	if len(title) <= limit {
		return title
	}
	cut := strings.LastIndex(title[:limit], " ")
	if cut < 20 {
		cut = limit
	}
	return strings.TrimSpace(title[:cut]) + "..."
}

// HealthFeedback is what the model said about a member's health data.
//
// Replaced rather than accumulated: the subject is a rolling ninety-day
// window, so an old reading of it is not a second record but a stale one.
type HealthFeedback struct {
	ID string `json:"id"`
	// Source is the device account it was read from - google_health,
	// withings - as against Provider, which is the AI vendor.
	Source   string `json:"source"`
	Content  string `json:"content"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	// Days is how long a window it read, so a fortnight's reading is not
	// mistaken for a quarter's.
	Days      int       `json:"days,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// NextAt is when another reading may be asked for. Sent with the stored
	// one so a button can say when it comes back rather than only refusing.
	NextAt time.Time `json:"nextAt,omitempty"`
}
