package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/storage"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// videoContentTypes are the containers a browser can actually play back. The
// check is on the declared type rather than the extension, and the list is
// short on purpose: an upload that a <video> tag can't render is a post with a
// broken player in it.
var videoContentTypes = map[string]string{
	"video/mp4":       ".mp4",
	"video/webm":      ".webm",
	"video/ogg":       ".ogv",
	"video/quicktime": ".mov",
}

// audioContentTypes are the containers a browser can play back. A voice
// message recorded in a browser arrives as webm or mp4 depending on the engine;
// the rest are here because a phone may record them.
var audioContentTypes = map[string]string{
	"audio/webm": ".webm",
	"audio/ogg":  ".ogg",
	"audio/mp4":  ".m4a",
	"audio/mpeg": ".mp3",
	"audio/aac":  ".aac",
	"audio/wav":  ".wav",
}

// MediaHandler uploads a member's video to the object store they configured
// for themselves (see package storage). strong-fish stores no video of its
// own; what comes back is a URL, which the client pastes into the post it is
// composing - and from there the post's own link detection takes over.
type MediaHandler struct {
	users        *store.UserStore
	maxVideoSize int64
	maxAudioSize int64
}

func NewMediaHandler(users *store.UserStore, maxVideoSize, maxAudioSize int64) *MediaHandler {
	return &MediaHandler{users: users, maxVideoSize: maxVideoSize, maxAudioSize: maxAudioSize}
}

// UploadVideo stores one video and returns its public URL.
//
// With no storage connection configured it answers 405, not 400: the request
// is well-formed, the method simply isn't available on this account until the
// member points it at a bucket. That is the status the client toasts.
// UploadAudio stores a voice message. Same storage, same refusal when none is
// configured - the only differences are the accepted types and a smaller cap,
// since a spoken message is a fraction of a video's size and letting it use the
// video budget would just invite a 20MB recording nobody wants to wait for.
func (h *MediaHandler) UploadAudio(w http.ResponseWriter, r *http.Request) {
	h.upload(w, r, audioContentTypes, h.maxAudioSize, "voice")
}

func (h *MediaHandler) UploadVideo(w http.ResponseWriter, r *http.Request) {
	h.upload(w, r, videoContentTypes, h.maxVideoSize, "video")
}

// upload is the shared path: check the member has storage, bound the transfer,
// accept only a type that can be played back, and hand the bytes to their own
// bucket.
func (h *MediaHandler) upload(w http.ResponseWriter, r *http.Request,
	accepted map[string]string, maxSize int64, kind string) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !user.Storage.Configured() {
		writeError(w, http.StatusMethodNotAllowed,
			"Configure your own storage bucket before uploading a video", CodeStorageNotConfigured)
		return
	}

	// The limit is enforced twice: MaxBytesReader stops the transfer at the
	// cap rather than buffering a gigabyte to find out it was too big, and the
	// explicit length check is what produces a message the member can act on.
	r.Body = http.MaxBytesReader(w, r.Body, maxSize+1024)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "This video is too large", CodeVideoTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "No file was uploaded", CodeInvalidRequestBody)
		return
	}
	defer file.Close()

	if header.Size > maxSize {
		writeError(w, http.StatusRequestEntityTooLarge, "This video is too large", CodeVideoTooLarge)
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]))
	extension, ok := accepted[contentType]
	if !ok {
		writeError(w, http.StatusBadRequest, "This file is not something a browser can play", CodeUnsupportedVideo)
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Could not read the uploaded file", CodeInvalidRequestBody)
		return
	}
	if int64(len(data)) > maxSize {
		writeError(w, http.StatusRequestEntityTooLarge, "This video is too large", CodeVideoTooLarge)
		return
	}

	target, err := storage.New(user.Storage)
	if err != nil {
		writeError(w, http.StatusMethodNotAllowed, err.Error(), CodeStorageNotConfigured)
		return
	}

	url, err := target.Upload(r.Context(), mediaKey(userID, kind, header.Filename, extension), data, contentType)
	if err != nil {
		// The member's own bucket rejected this, so the message is theirs to
		// act on - a wrong key, a missing bucket, ACLs disabled - and saying
		// "internal error" would send them looking in the wrong place.
		writeError(w, http.StatusBadGateway, err.Error(), CodeStorageUploadFailed)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"url": url})
}

// mediaKey is where the object is written: one folder per member and kind, and
// a random name carrying the original's extension.
//
// The uploaded filename is deliberately not reused - it is attacker-controlled
// text going into a URL path - beyond taking a short, slugified hint from it so
// a bucket's listing is still readable by a human.
func mediaKey(userID, kind, filename, extension string) string {
	hint := utils.Slugify(strings.TrimSuffix(path.Base(filename), path.Ext(filename)))
	if len(hint) > 40 {
		hint = hint[:40]
	}
	if utils.IsBlank(hint) {
		hint = kind
	}
	random, err := utils.RandomHex(8)
	if err != nil {
		random = "0"
	}
	return fmt.Sprintf("strong-fish/%s/%s/%s-%s%s", userID, kind, hint, random, extension)
}

// --- the member's own storage connection ---

// StorageGet returns the connection as configured, with the credentials
// redacted: the UI needs to show what is set up, never the keys themselves.
func (h *MediaHandler) StorageGet(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connection": user.Storage.Redacted(),
		"configured": user.Storage.Configured(),
		"maxSize":    h.maxVideoSize,
	})
}

func (h *MediaHandler) StorageSet(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var conn models.StorageConnection
	if !decodeJSON(w, r, &conn) {
		return
	}

	current, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// A client echoing back the redaction marker means "keep the stored
	// secret", which is what lets somebody change their bucket name without
	// retyping a key they can no longer read.
	if conn.SecretKey == models.SecretSet {
		conn.SecretKey = current.Storage.SecretKey
	}
	if conn.ServiceAccountBase64 == models.SecretSet {
		conn.ServiceAccountBase64 = current.Storage.ServiceAccountBase64
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
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "Unknown storage type", CodeInvalidStorageType)
		return
	}

	if !conn.Configured() {
		writeError(w, http.StatusBadRequest, "Please fill in every field for this storage type", CodeAllFieldsRequired)
		return
	}

	user, err := h.users.SetStorage(r.Context(), userID, conn)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connection": user.Storage.Redacted(),
		"configured": user.Storage.Configured(),
		"maxSize":    h.maxVideoSize,
	})
}

func (h *MediaHandler) StorageDelete(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	if _, err := h.users.ClearStorage(r.Context(), userID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connection": models.StorageConnection{},
		"configured": false,
		"maxSize":    h.maxVideoSize,
	})
}
