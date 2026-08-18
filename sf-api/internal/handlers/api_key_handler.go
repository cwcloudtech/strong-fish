package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// ApiKeyHandler is the self-service view over a member's own keys. There is
// deliberately no administrative view across other people's: a key is a
// credential, and nobody needs to enumerate someone else's credentials.
type ApiKeyHandler struct {
	keys *store.ApiKeyStore
}

func NewApiKeyHandler(keys *store.ApiKeyStore) *ApiKeyHandler {
	return &ApiKeyHandler{keys: keys}
}

func (h *ApiKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	keys, err := h.keys.ListByUser(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

type apiKeyPayload struct {
	Description string  `json:"description"`
	ExpiresAt   *string `json:"expiresAt"`
}

func (h *ApiKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p apiKeyPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Description) {
		writeError(w, http.StatusBadRequest, "Please describe what this key is for", CodeApiKeyDescription)
		return
	}

	var expiresAt *time.Time
	if p.ExpiresAt != nil && utils.IsNotBlank(*p.ExpiresAt) {
		at, err := time.Parse(time.RFC3339, *p.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid expiration date", CodeInvalidExpiration)
			return
		}
		expiresAt = &at
	}

	key, token, err := h.keys.Create(r.Context(), userID, p.Description, expiresAt)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, models.ApiKeyCreated{
		ID:          key.ID,
		Description: key.Description,
		ExpiresAt:   key.ExpiresAt,
		CreatedAt:   key.CreatedAt,
		Token:       token,
	})
}

func (h *ApiKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "keyId")

	if err := h.keys.Delete(r.Context(), userID, id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}
