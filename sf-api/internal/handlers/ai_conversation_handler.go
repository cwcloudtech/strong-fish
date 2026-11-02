package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programpdf"
	"strong-fish-api/internal/programpng"
	"strong-fish-api/internal/programsheet"
	"strong-fish-api/internal/utils"
)

// A member's threads with the model: listing them, reading one, sharing one by
// link, and exporting one as a document.
//
// Stored at all because an answer is worth keeping - the reading of why a squat
// stalled is still the reading three weeks later - and because somebody lending
// a key out is entitled to see what it was spent on.

// ListConversations is the caller's threads, newest activity first, without
// their contents.
func (h *AIHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	conversations, err := h.connections.ListConversations(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversations)
}

// GetConversation is one thread with its exchanges, for its owner.
func (h *AIHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	conversation, ok := h.ownConversation(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.withShareURL(conversation))
}

// ownConversation resolves the addressed thread and refuses anybody but its
// owner. Scoped in the query rather than compared afterwards, so a thread that
// is not the caller's is indistinguishable from one that does not exist.
func (h *AIHandler) ownConversation(w http.ResponseWriter, r *http.Request) (models.AIConversation, bool) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	conversation, err := h.connections.FindConversation(r.Context(),
		chi.URLParam(r, "conversationId"), userID)
	if err != nil {
		writeStoreError(w, err)
		return models.AIConversation{}, false
	}
	return conversation, true
}

// withShareURL fills in the link to hand somebody, so a client does not have
// to know how a shared conversation is addressed.
func (h *AIHandler) withShareURL(conversation models.AIConversation) models.AIConversation {
	if conversation.ShareToken != "" {
		conversation.ShareURL = h.uiBaseURL + "/shared/conversations/" + conversation.ShareToken
	}
	return conversation
}

type conversationTitlePayload struct {
	Title string `json:"title"`
}

// RenameConversation retitles a thread. The title is derived from the first
// question to begin with; this is for when that is not what the thread turned
// out to be about.
func (h *AIHandler) RenameConversation(w http.ResponseWriter, r *http.Request) {
	conversation, ok := h.ownConversation(w, r)
	if !ok {
		return
	}

	var p conversationTitlePayload
	if !decodeJSON(w, r, &p) {
		return
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "A title is required", CodeConversationTitle)
		return
	}
	if len(title) > 200 {
		title = title[:200]
	}

	if err := h.connections.RenameConversation(r.Context(), conversation.ID,
		conversation.OwnerID, title); err != nil {
		writeStoreError(w, err)
		return
	}
	conversation.Title = title
	writeJSON(w, http.StatusOK, h.withShareURL(conversation))
}

// DeleteConversation removes one thread.
func (h *AIHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	removed, err := h.connections.DeleteConversations(r.Context(), userID,
		[]string{chi.URLParam(r, "conversationId")}, false)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if removed == 0 {
		writeError(w, http.StatusNotFound, "Conversation not found", CodeNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type deleteConversationsPayload struct {
	IDs []string `json:"ids"`
	// All empties the history. Asked for explicitly rather than inferred from
	// an empty id list, because "delete these" with the list accidentally
	// empty would otherwise mean "delete everything".
	All bool `json:"all"`
}

// DeleteConversations clears a selection, or the whole history.
//
// A POST rather than a DELETE with a body: a request body on a DELETE is
// permitted but widely dropped - by proxies, by some HTTP clients, by Dio's
// own defaults - and a bulk delete that silently deletes nothing is worse than
// an unfashionable verb.
func (h *AIHandler) DeleteConversations(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p deleteConversationsPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !p.All && len(p.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "Pick what to delete", CodeNothingSelected)
		return
	}

	removed, err := h.connections.DeleteConversations(r.Context(), userID, p.IDs, p.All)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": removed})
}

// ShareConversation mints the link that lets anybody holding it read the
// thread, the way ChatGPT and Gemini share one.
//
// Sharing twice hands back the same link rather than minting a second: a
// member who presses the button again wants the link, not to break the one
// they have already sent somebody.
func (h *AIHandler) ShareConversation(w http.ResponseWriter, r *http.Request) {
	conversation, ok := h.ownConversation(w, r)
	if !ok {
		return
	}

	token, err := h.connections.ShareConversation(r.Context(), conversation.ID, conversation.OwnerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	conversation.Shared, conversation.ShareToken = true, token
	writeJSON(w, http.StatusOK, h.withShareURL(conversation))
}

// UnshareConversation revokes the link. Immediate, which is the point of
// revoking: whoever holds it stops being able to read the thread.
func (h *AIHandler) UnshareConversation(w http.ResponseWriter, r *http.Request) {
	conversation, ok := h.ownConversation(w, r)
	if !ok {
		return
	}
	if err := h.connections.UnshareConversation(r.Context(), conversation.ID,
		conversation.OwnerID); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSharedConversation reads a thread by its link token, for anybody at all.
//
// Unauthenticated by design - a link that only works for members is not a
// shared link - and the token is the whole credential, which is why it is 32
// random bytes and why revoking it takes effect at once. The token itself is
// not echoed back: a reader already has it, and a page that prints it is a
// page that leaks it into screenshots.
func (h *AIHandler) GetSharedConversation(w http.ResponseWriter, r *http.Request) {
	conversation, ok := h.sharedConversation(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

func (h *AIHandler) sharedConversation(w http.ResponseWriter, r *http.Request) (models.AIConversation, bool) {
	conversation, err := h.connections.FindSharedConversation(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		writeStoreError(w, err)
		return models.AIConversation{}, false
	}
	return conversation, true
}

// --- exports ---

// ExportConversation hands the thread back as a document, for a member who
// wants to keep it or send it on.
func (h *AIHandler) ExportConversation(w http.ResponseWriter, r *http.Request) {
	conversation, ok := h.ownConversation(w, r)
	if !ok {
		return
	}
	h.writeConversationSheet(w, r, conversation)
}

// ExportSharedConversation is the same document from a shared link, so what
// somebody shares can be saved by whoever they shared it with.
func (h *AIHandler) ExportSharedConversation(w http.ResponseWriter, r *http.Request) {
	conversation, ok := h.sharedConversation(w, r)
	if !ok {
		return
	}
	h.writeConversationSheet(w, r, conversation)
}

func (h *AIHandler) writeConversationSheet(w http.ResponseWriter, r *http.Request,
	conversation models.AIConversation) {
	locale := sheetLocale(r)
	format := exportFormat(r)

	title := conversation.Title
	if utils.IsBlank(title) {
		title = programsheet.Heading(locale, "conversation")
	}
	subtitle := conversation.UpdatedAt.Format("2 January 2006")
	sections := conversationSections(conversation, locale)
	footer := "Strong Fish - " + h.uiBaseURL

	var (
		body  []byte
		media string
		err   error
	)
	switch format {
	case formatPNG:
		body, err = programpng.RenderTables(title, subtitle, sections, footer)
		media = mediaPNG
	default:
		body, err = programpdf.RenderTables(title, subtitle, sections, footer)
		media = mediaPDF
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}

	w.Header().Set("Content-Type", media)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", sheetFileName(title, format)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// conversationSections lays a thread out as two tables per exchange: what was
// asked, then what was answered.
//
// A table rather than running prose because that is what both renderers draw,
// and it suits the material: a question and its answer are a pair, and a
// bordered block each is how a reader tells one exchange from the next on a
// page with no colour.
// exchangeCredit is the line under an answer: which model produced it and
// when, the same credit a session reading carries.
//
// The model leads here, where a session reading names the subscription: a
// thread can be asked of two models through one key, so the model is what
// tells two answers apart. The subscription is the fallback for a provider
// that reported no model at all.
func exchangeCredit(exchange models.AIExchange, locale string) string {
	name := creditName(exchange.Model, exchange.Connection)
	if name == "" || exchange.CreatedAt.IsZero() {
		return ""
	}
	return programsheet.GeneratedBy(locale, name, exchange.CreatedAt)
}

func conversationSections(conversation models.AIConversation, locale string) []programsheet.Section {
	sections := make([]programsheet.Section, 0, len(conversation.Exchanges)*2)
	for i, exchange := range conversation.Exchanges {
		sections = append(sections, programsheet.Section{
			Heading: fmt.Sprintf(programsheet.Heading(locale, "exchange"), i+1),
			Columns: []programsheet.Column{{Header: programsheet.Heading(locale, "question"), Weight: 1}},
			Rows:    textRows(exchange.Question),
		})
		// The answer, then what produced it: with a picker above the thread,
		// two answers in one conversation can come from two models, and a
		// printed page that does not say which is a page nobody can check.
		answer := textRows(exchange.Answer)
		if credit := exchangeCredit(exchange, locale); credit != "" {
			answer = append(answer, programsheet.Line{Cells: []string{credit}})
		}
		sections = append(sections, programsheet.Section{
			Columns: []programsheet.Column{{Header: programsheet.Heading(locale, "answer"), Weight: 1}},
			Rows:    answer,
		})
	}
	return sections
}

// textRows turns an answer into one row per line.
//
// The model answers in markdown and neither renderer draws markdown; the same
// flattening the program sheets use (see programsheet.PlainLines) keeps the
// shape of the answer without its markers.
func textRows(text string) []programsheet.Line {
	rows := []programsheet.Line{}
	for _, line := range programsheet.PlainLines(text) {
		rows = append(rows, programsheet.Line{Cells: []string{line}})
	}
	if len(rows) == 0 {
		rows = append(rows, programsheet.Line{Cells: []string{""}})
	}
	return rows
}
