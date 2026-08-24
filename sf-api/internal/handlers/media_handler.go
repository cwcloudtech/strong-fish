package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"strong-fish-api/internal/middleware"
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
	storages     *store.StorageStore
	profiles     *ProfileHandler
	maxVideoSize int64
	maxAudioSize int64
	// apiBaseURL is where this API answers. A private bucket's objects are
	// addressed here rather than at the bucket, so the URL that goes into a
	// post has to be absolute - it is read by a browser that knows nothing
	// about this deployment.
	apiBaseURL string
	// mediaSecret signs the short-lived links the players use. A <video> tag
	// cannot carry an Authorization header, so the grant has to travel in the
	// URL - see SignedMediaURL.
	mediaSecret []byte
}

func NewMediaHandler(users *store.UserStore, storages *store.StorageStore, profiles *ProfileHandler,
	maxVideoSize, maxAudioSize int64, apiBaseURL, mediaSecret string) *MediaHandler {
	return &MediaHandler{
		users: users, storages: storages, profiles: profiles,
		maxVideoSize: maxVideoSize, maxAudioSize: maxAudioSize,
		apiBaseURL:  strings.TrimRight(apiBaseURL, "/"),
		mediaSecret: []byte(mediaSecret),
	}
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

	// Their own storage, or one somebody shared with them as a writer: a
	// member with no bucket of their own can still post a video if their coach
	// lent them theirs.
	destination, ok := h.writableStorage(w, r, userID)
	if !ok {
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

	target, err := storage.New(destination.Conn)
	if err != nil {
		writeError(w, http.StatusMethodNotAllowed, err.Error(), CodeStorageNotConfigured)
		return
	}

	key := mediaKey(userID, kind, header.Filename, extension)
	url, err := target.Upload(r.Context(), key, data, contentType)
	if err != nil {
		// The member's own bucket rejected this, so the message is theirs to
		// act on - a wrong key, a missing bucket, ACLs disabled - and saying
		// "internal error" would send them looking in the wrong place.
		writeError(w, http.StatusBadGateway, err.Error(), CodeStorageUploadFailed)
		return
	}

	// A private bucket's object has no address a reader could use: what goes
	// into the post is this API's, and the API fetches the object with the
	// stored credentials for a reader it has checked.
	if destination.Conn.Private {
		url = h.mediaURL(destination.ID, key, extension)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"url": url})
}

// mediaKeyPrefix is the folder every upload goes under, and the marker that
// says a key was written by this app - which is what lets the media route read
// the uploader's id back out of it.
const mediaKeyPrefix = "strong-fish"

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
	return fmt.Sprintf("%s/%s/%s/%s-%s%s", mediaKeyPrefix, userID, kind, hint, random, extension)
}
