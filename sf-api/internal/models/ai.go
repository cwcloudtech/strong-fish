package models

import (
	"strings"
	"time"
)

// The wire formats a provider can speak.
//
// Three cover every provider worth naming, which is why the catalogue stores
// one of these rather than a vendor: what differs between OpenAI, Mistral,
// DeepSeek, OpenRouter, a LiteLLM and an Ollama is the address, not the
// protocol. Only Claude and Gemini need a format of their own.
const (
	AIFormatOpenAI    = "openai"
	AIFormatAnthropic = "anthropic"
	AIFormatGemini    = "gemini"
)

// AIProviderInfo is one row of the provider catalogue (see the ai_providers
// table): where a provider lives, how to talk to it, and what it serves.
//
// Data rather than code. A vendor renames a model, an operator points the
// whole deployment at a gateway, a new OpenAI-compatible service appears -
// none of that should be a release, and none of it is anything this API has
// to understand beyond the format.
type AIProviderInfo struct {
	Name string `json:"name"`
	// Label is what a picker shows. Vendor branding, so it is the same in
	// every language and is not in the dictionaries.
	Label  string `json:"label"`
	Format string `json:"format"`
	// BaseURL and DefaultModel are a starting point offered to the member,
	// never imposed: pointing a connection at a local gateway is the reason
	// both fields are editable.
	BaseURL      string `json:"defaultUrl"`
	DefaultModel string `json:"defaultModel"`
	// Models are the names to suggest. A suggestion, never a constraint - a
	// vendor ships a model the day before this table hears about it, and a
	// form that refuses it would be a form nobody can use.
	Models []string `json:"models"`
	// KeyHint is what this provider's key looks like, for the form's
	// placeholder - an empty box tells a member nothing about what to paste.
	KeyHint string `json:"keyHint,omitempty"`
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
	// Format is the provider's wire protocol, read from the catalogue when a
	// connection is about to be spent. Blank everywhere else: it is not the
	// member's to set, and a client has no use for it.
	Format  string `json:"-"`
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model,omitempty"`
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
	return strings.TrimSpace(c.Provider) != "" &&
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
	// local model is not later mistaken for one from a large hosted one, and
	// Connection names the subscription that paid - what the credit line
	// under the reading says.
	Provider   string    `json:"provider,omitempty"`
	Model      string    `json:"model,omitempty"`
	Connection string    `json:"connection,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
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
	Source     string `json:"source"`
	Content    string `json:"content"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	Connection string `json:"connection,omitempty"`
	// Days is how long a window it read, so a fortnight's reading is not
	// mistaken for a quarter's.
	Days      int       `json:"days,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// NextAt is when another reading may be asked for. Sent with the stored
	// one so a button can say when it comes back rather than only refusing.
	NextAt time.Time `json:"nextAt,omitempty"`
}
