package models

import "time"

// Conversation is the thread between two members.
//
// There is exactly one per pair, and it is derived from the pair rather than
// created: nobody "starts" a conversation, they send the first message into the
// one that was always implied.
type Conversation struct {
	ID string `json:"id"`
	// Other is who the caller is talking to. A conversation has no meaning
	// except relative to somebody, so it is always projected for one reader.
	Other UserSummary `json:"other"`
	// LastMessage and LastSenderID render the list without a message query per
	// row.
	LastMessage  string    `json:"lastMessage,omitempty"`
	LastSenderID string    `json:"lastSenderId,omitempty"`
	Unread       int       `json:"unread"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Message is one line in a conversation.
type Message struct {
	ID             string      `json:"id"`
	ConversationID string      `json:"conversationId"`
	SenderID       string      `json:"senderId"`
	Sender         UserSummary `json:"sender"`
	Content        string      `json:"content"`
	// Pictures are base64 data URIs carried inline, the way a post's are.
	Pictures []string `json:"pictures,omitempty"`
	// Links is derived from the content, never submitted: whatever URL the
	// sender pasted is what gets rendered as a player or a card. Same rule as
	// a post, so a video shared in a thread behaves like one shared in the feed.
	Links []string `json:"links,omitempty"`
	// Audio is a voice message: a URL in the sender's own storage, since this
	// app hosts no media of its own.
	Audio string `json:"audio,omitempty"`
	// Mine saves the client comparing ids to decide which side to draw the
	// bubble on.
	Mine      bool       `json:"mine"`
	ReadAt    *time.Time `json:"readAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// Block is one member's decision to stop hearing from another.
//
// It is stored directionally - who blocked whom - but enforced in both: a
// blocked member's posts leave the blocker's feed and the blocker's leave
// theirs, and neither can message the other. What stays one-directional is who
// may lift it.
type Block struct {
	ID        string      `json:"id"`
	BlockedID string      `json:"blockedId"`
	Blocked   UserSummary `json:"blocked"`
	CreatedAt time.Time   `json:"createdAt"`
}

// ConnectionIP is one address an account has connected from, with how often.
//
// Kept in the user's payload rather than in a table: it is a short per-account
// list, read only on that account's administration screen, and a row per login
// would be a write on every single authentication for data nobody queries
// across accounts.
type ConnectionIP struct {
	IP        string    `json:"ip"`
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

// MaxConnectionIPs bounds the list. An account connecting from a new address
// every time - a phone on mobile data - would otherwise grow its own row
// without limit; past this, the least recently seen address is dropped.
const MaxConnectionIPs = 50
