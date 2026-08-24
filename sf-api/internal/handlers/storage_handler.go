package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/storage"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// StorageHandler is a member's own object store, and who else may use it.
//
// The connection used to live in the member's user payload; it is a row of its
// own now because it is shareable (see V12). What that changes here is the
// access list: a coach can lend their bucket to an athlete as a writer, or
// open it to a club-mate as a reader, and take it back.
type StorageHandler struct {
	storages     *store.StorageStore
	users        *store.UserStore
	maxVideoSize int64
}

func NewStorageHandler(storages *store.StorageStore, users *store.UserStore, maxVideoSize int64) *StorageHandler {
	return &StorageHandler{storages: storages, users: users, maxVideoSize: maxVideoSize}
}

// normalize validates and cleans a connection a form sent, or writes the
// refusal and answers false.
func (h *StorageHandler) normalize(w http.ResponseWriter, conn models.StorageConnection,
	current models.StorageConnection) (models.StorageConnection, bool) {
	// A client echoing back the redaction marker means "keep the stored
	// secret", which is what lets somebody change their bucket name without
	// retyping a key they can no longer read.
	if conn.SecretKey == models.SecretSet {
		conn.SecretKey = current.SecretKey
	}
	if conn.ServiceAccountBase64 == models.SecretSet {
		conn.ServiceAccountBase64 = current.ServiceAccountBase64
	}

	switch conn.Type {
	case models.StorageTypeS3:
		conn.ServiceAccountBase64, conn.FolderID = utils.EMPTY, utils.EMPTY
	case models.StorageTypeGoogleDrive:
		conn.Endpoint, conn.BucketName, conn.Region = utils.EMPTY, utils.EMPTY, utils.EMPTY
		conn.AccessKey, conn.SecretKey = utils.EMPTY, utils.EMPTY
		// A malformed service-account key is worth catching now rather than
		// weeks later on the first upload.
		if _, err := storage.DecodeServiceAccount(conn.ServiceAccountBase64); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), CodeInvalidServiceAccount)
			return conn, false
		}
	default:
		writeError(w, http.StatusBadRequest, "Unknown storage type", CodeInvalidStorageType)
		return conn, false
	}

	if !conn.Configured() {
		writeError(w, http.StatusBadRequest, "Please fill in every field for this storage type", CodeAllFieldsRequired)
		return conn, false
	}
	return conn, true
}

// redact is a list as the settings screen reads it: everything needed to see
// what is set up, with the credentials replaced by a marker.
func redact(storages []models.Storage) []models.Storage {
	for i := range storages {
		storages[i].Conn = storages[i].Conn.Redacted()
	}
	return storages
}

// List is the caller's own targets, in their priority order.
func (h *StorageHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	storages, err := h.storages.ListOwn(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"storages": redact(storages),
		"maxSize":  h.maxVideoSize,
	})
}

// ListUsable is every target the caller may upload to: their own, and the ones
// shared with them. The connections are redacted - somebody granted write
// access may use a bucket, not read its keys out.
func (h *StorageHandler) ListUsable(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	available, err := h.storages.ListFor(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, redact(available))
}

// Create adds a target at the end of the caller's list.
func (h *StorageHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var conn models.StorageConnection
	if !decodeJSON(w, r, &conn) {
		return
	}
	conn, ok := h.normalize(w, conn, models.StorageConnection{})
	if !ok {
		return
	}

	if _, err := h.storages.Create(r.Context(), userID, conn); err != nil {
		writeStoreError(w, err)
		return
	}
	h.List(w, r)
}

// Update rewrites one of the caller's own targets.
func (h *StorageHandler) Update(w http.ResponseWriter, r *http.Request) {
	stored, ok := h.ownStorage(w, r)
	if !ok {
		return
	}

	var conn models.StorageConnection
	if !decodeJSON(w, r, &conn) {
		return
	}
	conn, ok = h.normalize(w, conn, stored.Conn)
	if !ok {
		return
	}

	if _, err := h.storages.Update(r.Context(), stored.ID, conn); err != nil {
		writeStoreError(w, err)
		return
	}
	h.List(w, r)
}

// Delete removes one target, and with it every grant on it: sharing a bucket
// that is no longer configured would leave rows pointing at nothing.
func (h *StorageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	stored, ok := h.ownStorage(w, r)
	if !ok {
		return
	}
	if err := h.storages.Delete(r.Context(), stored.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	h.List(w, r)
}

type storageOrderPayload struct {
	StorageIDs []string `json:"storageIds"`
}

// Reorder sets which target is first - which is the one whose link goes into a
// post, so it is a real setting rather than a display preference.
func (h *StorageHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p storageOrderPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if err := h.storages.Reorder(r.Context(), userID, p.StorageIDs); err != nil {
		writeStoreError(w, err)
		return
	}
	h.List(w, r)
}

// --- sharing ---

// ownStorage loads the target named by the route, refusing anything that is
// not the caller's own.
//
// Editing and sharing are the owner's alone: a writer who could hand out
// further access would be able to widen a bucket its owner is paying for. A
// target belonging to somebody else reads as missing rather than forbidden -
// which storage ids exist is not a caller's business.
func (h *StorageHandler) ownStorage(w http.ResponseWriter, r *http.Request) (models.Storage, bool) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	stored, err := h.storages.FindByID(r.Context(), chi.URLParam(r, "storageId"))
	if errors.Is(err, store.ErrNotFound) || (err == nil && stored.OwnerID != userID) {
		writeError(w, http.StatusNotFound, "This storage is not available", CodeNotFound)
		return models.Storage{}, false
	}
	if err != nil {
		writeStoreError(w, err)
		return models.Storage{}, false
	}
	return stored, true
}

// ListGrants is the access list of the caller's own storage.
func (h *StorageHandler) ListGrants(w http.ResponseWriter, r *http.Request) {
	stored, ok := h.ownStorage(w, r)
	if !ok {
		return
	}

	grants, err := h.storages.ListGrants(r.Context(), stored.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grants)
}

type storageGrantPayload struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

// Share gives somebody read or write access, or changes the role they have.
func (h *StorageHandler) Share(w http.ResponseWriter, r *http.Request) {
	stored, ok := h.ownStorage(w, r)
	if !ok {
		return
	}

	var p storageGrantPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !models.IsValidStorageRole(p.Role) {
		writeError(w, http.StatusBadRequest, "Pick reader or writer", CodeInvalidStorageRole)
		return
	}
	if p.UserID == stored.OwnerID {
		// The owner already has every right there is, and writing a second row
		// for them would let a later revoke take their own storage away.
		writeError(w, http.StatusBadRequest, "This storage is already yours", CodeInvalidStorageRole)
		return
	}
	// A grant to somebody who does not exist would be a row nothing can show.
	if _, err := h.users.FindByID(r.Context(), p.UserID); err != nil {
		writeStoreError(w, err)
		return
	}

	if err := h.storages.Grant(r.Context(), stored.ID, p.UserID, p.Role); err != nil {
		writeStoreError(w, err)
		return
	}
	grants, err := h.storages.ListGrants(r.Context(), stored.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grants)
}

// Unshare takes an access away.
func (h *StorageHandler) Unshare(w http.ResponseWriter, r *http.Request) {
	stored, ok := h.ownStorage(w, r)
	if !ok {
		return
	}

	if err := h.storages.Revoke(r.Context(), stored.ID, chi.URLParam(r, "userId")); err != nil {
		writeStoreError(w, err)
		return
	}
	grants, err := h.storages.ListGrants(r.Context(), stored.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grants)
}
