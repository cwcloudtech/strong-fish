package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// maxMessageLength bounds one private message. Long enough for a paragraph of
// coaching, short enough that a thread stays a conversation.
const (
	maxMessageLength = 4000
	// The same cap a post has: pictures ride inline in the JSONB payload, so
	// this is also what keeps a row from growing without bound.
	maxMessagePictures = 4
)

// MessageHandler is private messaging, and the block list that governs it.
//
// You can only write to somebody whose profile you may see: the visibility a
// member chose for their profile is also the reach they agreed to be contacted
// at, and having one setting mean two things is better than asking them the
// same question twice.
type MessageHandler struct {
	messages     *store.MessageStore
	users        *store.UserStore
	maxImageSize int64
	profile      *ProfileHandler
}

func NewMessageHandler(messages *store.MessageStore, users *store.UserStore, profile *ProfileHandler,
	maxImageSize int64) *MessageHandler {
	return &MessageHandler{messages: messages, users: users, profile: profile, maxImageSize: maxImageSize}
}

func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	conversations, err := h.messages.ListConversations(r.Context(), callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversations)
}

// Unread backs the badge on the messages nav entry.
func (h *MessageHandler) Unread(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	count, err := h.messages.CountUnread(r.Context(), callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread": count})
}

// canReach decides whether callerID may message otherID: the other's profile
// has to be visible to them, and neither may have blocked the other.
func (h *MessageHandler) canReach(w http.ResponseWriter, r *http.Request, callerID, otherID string) (models.User, bool) {
	other, err := h.users.FindByID(r.Context(), otherID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Member not found", CodeUserNotFound)
		return models.User{}, false
	}
	if other.ID == callerID {
		writeError(w, http.StatusBadRequest, "You cannot message yourself", CodeCannotMessageSelf)
		return models.User{}, false
	}

	blocked, err := h.messages.IsBlockedEither(r.Context(), callerID, otherID)
	if err != nil {
		writeStoreError(w, err)
		return models.User{}, false
	}
	if blocked {
		// Deliberately not "you are blocked": telling somebody they were
		// blocked is information the person who blocked them did not agree to
		// share. It reads as "you cannot message this member".
		writeError(w, http.StatusForbidden, "You cannot message this member", CodeCannotMessage)
		return models.User{}, false
	}

	relation, err := h.profile.RelationTo(r.Context(), otherID, callerID)
	if err != nil {
		writeStoreError(w, err)
		return models.User{}, false
	}
	if !models.CanSeeProfile(other.ProfileVisibility, relation) {
		writeError(w, http.StatusForbidden, "You cannot message this member", CodeCannotMessage)
		return models.User{}, false
	}
	return other, true
}

// Thread opens the conversation with one member, creating it on first use.
//
// It is addressed by *who* rather than by conversation id, because there is
// exactly one thread per pair: making the client look an id up first would only
// add a round trip and a way to get it wrong.
func (h *MessageHandler) Thread(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	otherID := chi.URLParam(r, "userId")

	other, ok := h.canReach(w, r, callerID, otherID)
	if !ok {
		return
	}

	conversationID, err := h.messages.FindOrCreateConversation(r.Context(), callerID, otherID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	messages, err := h.messages.ListMessages(r.Context(), conversationID, 0)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// Opening the thread is what reading it means. Best-effort: a failed stamp
	// leaves the badge stale, which is not worth failing the read over.
	_ = h.messages.MarkRead(r.Context(), conversationID, callerID, time.Now().UTC())

	superadmin := h.isSuperadmin(r, callerID)
	for i := range messages {
		messages[i].Mine = messages[i].SenderID == callerID
		messages[i].Deletable = messages[i].Mine || superadmin
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conversationId": conversationID,
		"other":          summarize(other),
		"messages":       messages,
	})
}

type messagePayload struct {
	Content  string   `json:"content"`
	Pictures []string `json:"pictures"`
	// Audio is a voice message's URL, returned by the upload endpoint. The
	// caller supplies it because that upload went to their own storage, which
	// this API never holds a copy of.
	Audio string `json:"audio"`
}

func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	otherID := chi.URLParam(r, "userId")

	var p messagePayload
	if !decodeJSON(w, r, &p) {
		return
	}
	p.Content = strings.TrimSpace(p.Content)
	if len(p.Content) > maxMessageLength {
		writeError(w, http.StatusBadRequest, "This message is too long", CodeEmptyMessage)
		return
	}
	// A message needs *something* in it - but a picture or a voice message is
	// something, so text is no longer the only way to have said anything.
	if utils.IsBlank(p.Content) && len(p.Pictures) == 0 && utils.IsBlank(p.Audio) {
		writeError(w, http.StatusBadRequest, "A message needs some text", CodeEmptyMessage)
		return
	}
	if len(p.Pictures) > maxMessagePictures {
		writeError(w, http.StatusBadRequest, "Too many pictures on this message", CodeEmptyMessage)
		return
	}
	for _, picture := range p.Pictures {
		if utils.ImageSizeExceeds(picture, h.maxImageSize) {
			writeError(w, http.StatusBadRequest, "One of the pictures is too large", CodeImageTooLarge)
			return
		}
	}

	// Derived from the text, exactly as a post's is: whatever URL the sender
	// pasted is the message's embed, so a video shared in a thread renders the
	// way one shared in the feed does.
	var links []string
	if link := utils.FirstURL(p.Content); utils.IsNotBlank(link) {
		links = []string{link}
	}

	// Re-checked on every send, not just when the thread was opened: somebody
	// may have blocked the sender, or narrowed their profile, since.
	if _, ok := h.canReach(w, r, callerID, otherID); !ok {
		return
	}

	conversationID, err := h.messages.FindOrCreateConversation(r.Context(), callerID, otherID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	message, err := h.messages.Send(r.Context(), conversationID, callerID, store.MessageFields{
		Content: p.Content, Pictures: p.Pictures, Links: links, Audio: p.Audio,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	message.Mine = true
	message.Deletable = true
	writeJSON(w, http.StatusCreated, message)
}

// --- blocks ---

// isSuperadmin reports whether the caller moderates everything.
func (h *MessageHandler) isSuperadmin(r *http.Request, callerID string) bool {
	if utils.IsBlank(callerID) {
		return false
	}
	user, err := h.users.FindByID(r.Context(), callerID)
	return err == nil && user.Role == models.GlobalRoleSuperadmin
}

// Delete removes one private message.
//
// Its sender may take back what they wrote, and a superadmin may remove
// anything - the same moderation power they have over posts and comments. The
// recipient may not: a message someone else sent is part of the record of a
// conversation they were party to, and letting either side rewrite it would
// make a thread evidence of nothing.
func (h *MessageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	messageID := chi.URLParam(r, "messageId")

	message, err := h.messages.FindMessage(r.Context(), messageID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	if message.SenderID != callerID && !h.isSuperadmin(r, callerID) {
		// Not found rather than forbidden: a stranger guessing ids should not
		// learn which ones exist.
		writeError(w, http.StatusNotFound, "Message not found", CodeNotFound)
		return
	}

	if err := h.messages.DeleteMessage(r.Context(), messageID); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MessageHandler) ListBlocks(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	blocks, err := h.messages.ListBlocks(r.Context(), callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, blocks)
}

// Block hides a member: their posts leave the caller's feed, and neither can
// message the other.
//
// It does not require the target's profile to be visible. Somebody may need to
// block a member whose profile they can no longer see - that is precisely the
// situation blocking exists for.
func (h *MessageHandler) Block(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	blockedID := chi.URLParam(r, "userId")

	if blockedID == callerID {
		writeError(w, http.StatusBadRequest, "You cannot block yourself", CodeCannotBlockSelf)
		return
	}
	if _, err := h.users.FindByID(r.Context(), blockedID); err != nil {
		writeError(w, http.StatusNotFound, "Member not found", CodeUserNotFound)
		return
	}

	if err := h.messages.Block(r.Context(), callerID, blockedID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"blockedId": blockedID})
}

// Unblock lifts a block the caller placed. Only their own: being blocked is not
// something the blocked person gets to undo.
func (h *MessageHandler) Unblock(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	blockedID := chi.URLParam(r, "userId")

	if err := h.messages.Unblock(r.Context(), callerID, blockedID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"blockedId": blockedID})
}
