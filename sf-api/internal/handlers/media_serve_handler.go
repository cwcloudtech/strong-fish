package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/storage"
	"strong-fish-api/internal/utils"
)

// Serving a private bucket's objects.
//
// A member whose bucket forbids public objects had no way to post a video at
// all: the upload made the object public as it wrote it, and what went into
// the post was the bucket's own URL. With a private connection neither is
// true, so the object is addressed here instead, and this API fetches it with
// the stored credentials for a reader it has checked.
//
// Who may read one follows the *uploader's* profile visibility - the same rule
// that decides whether their posts are readable at all (models.CanSeeProfile)
// - plus anybody the storage's owner explicitly gave access to. Neither is
// about the object: a video is only ever as private as the person who posted
// it.
//
// # Why the link is signed
//
// A <video> tag cannot carry an Authorization header, and neither can the
// phone's player. So the grant travels in the URL: a client that may read the
// object asks for a short-lived signature over (storage, key, viewer) and puts
// *that* URL in the player. The signature is what the streaming route checks,
// which is why it is minted only after the same visibility check the direct
// route makes.

// mediaLinkTTL is how long a signed playback link is good for. Long enough to
// start a video and seek around in it, short enough that a URL copied out of
// the network tab is worthless by the time it is pasted anywhere.
const mediaLinkTTL = 6 * time.Hour

// mediaURL is where a private object is addressed: this API, the storage it is
// in, and the key inside it.
//
// The key is base64url-encoded because it carries slashes, and the extension
// is put back on the end so the clients' own players still recognise what they
// are pointed at by shape - a URL ending in .mp4 is a video to every one of
// them.
func (h *MediaHandler) mediaURL(storageID, key, extension string) string {
	return fmt.Sprintf("%s/v1/media/%s/%s%s", h.apiBaseURL, storageID,
		base64.RawURLEncoding.EncodeToString([]byte(key)), extension)
}

// mediaRequest is one addressed object, taken apart.
type mediaRequest struct {
	storageID string
	key       string
}

func parseMediaRequest(r *http.Request) (mediaRequest, bool) {
	storageID := chi.URLParam(r, "storageId")
	encoded := chi.URLParam(r, "object")
	if utils.IsBlank(storageID) || utils.IsBlank(encoded) {
		return mediaRequest{}, false
	}

	// The extension is decoration for the clients' sake; the key is what comes
	// before it.
	if dot := strings.LastIndex(encoded, "."); dot > 0 {
		encoded = encoded[:dot]
	}
	key, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(key) == 0 {
		return mediaRequest{}, false
	}
	return mediaRequest{storageID: storageID, key: string(key)}, true
}

// SignMedia hands a client a URL its player can actually use.
//
// Called with a session, it checks the caller may read the object and answers
// with the same address carrying a signature and an expiry. Everything the
// signature covers is in the URL, so the streaming route needs no session of
// its own - which is the point, since no media element will send one.
func (h *MediaHandler) SignMedia(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	request, ok := parseMediaRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "This is not a media address", CodeInvalidRequestBody)
		return
	}

	target, ok := h.authorizeMedia(w, r, request, callerID)
	if !ok {
		return
	}
	_ = target

	expires := time.Now().Add(mediaLinkTTL).Unix()
	signed := fmt.Sprintf("%s/v1/media/%s/%s?expires=%d&viewer=%s&signature=%s",
		h.apiBaseURL, request.storageID, chi.URLParam(r, "object"), expires,
		url.QueryEscape(callerID), h.signMedia(request, callerID, expires))

	writeJSON(w, http.StatusOK, map[string]any{"url": signed, "expiresAt": expires})
}

// ServeMedia streams a private object.
//
// Two ways in, and both end at the same check: a signed link (what a player
// uses), or an ordinary session (what a script or a curl would send). A
// request carrying neither is refused rather than served, however public the
// uploader's profile is - a private bucket's contents are not a public URL
// somebody guessed the shape of.
func (h *MediaHandler) ServeMedia(w http.ResponseWriter, r *http.Request) {
	request, ok := parseMediaRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "This is not a media address", CodeInvalidRequestBody)
		return
	}

	viewerID, ok := h.mediaViewer(r, request)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Not authorised", CodeForbidden)
		return
	}

	target, ok := h.authorizeMedia(w, r, request, viewerID)
	if !ok {
		return
	}

	object, err := target.Download(r.Context(), request.key, r.Header.Get("Range"))
	if err != nil {
		// The owner's own store refused this - a deleted object, a rotated key
		// - so the message is theirs to act on.
		writeError(w, http.StatusBadGateway, err.Error(), CodeStorageDownloadFailed)
		return
	}
	defer object.Body.Close()

	if object.ContentType != "" {
		w.Header().Set("Content-Type", object.ContentType)
	}
	if object.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(object.ContentLength, 10))
	}
	if object.ContentRange != "" {
		w.Header().Set("Content-Range", object.ContentRange)
	}
	// Advertised even when the store did not: a player that knows it can seek
	// asks for ranges, and the request goes straight through to the store.
	w.Header().Set("Accept-Ranges", utils.If(object.AcceptRanges != "", object.AcceptRanges, "bytes"))
	// Private means private: a shared cache holding this would serve it to
	// somebody the check above would have refused.
	w.Header().Set("Cache-Control", "private, max-age=0, no-store")

	status := object.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = io.Copy(w, object.Body)
}

// mediaViewer resolves who is asking: the signature's viewer when the link is
// signed and still valid, or the session's own user.
func (h *MediaHandler) mediaViewer(r *http.Request, request mediaRequest) (string, bool) {
	query := r.URL.Query()
	if signature := query.Get("signature"); signature != "" {
		expires, err := strconv.ParseInt(query.Get("expires"), 10, 64)
		if err != nil || time.Now().Unix() > expires {
			return utils.EMPTY, false
		}
		viewer := query.Get("viewer")
		if !hmac.Equal([]byte(signature), []byte(h.signMedia(request, viewer, expires))) {
			return utils.EMPTY, false
		}
		return viewer, true
	}

	// No signature: an ordinary authenticated call. The route is outside the
	// authenticated group (a player cannot send a header), so the session is
	// resolved here rather than by the middleware.
	userID, _ := middleware.UserIDFromContext(r.Context())
	return userID, utils.IsNotBlank(userID)
}

// signMedia is the signature a playback link carries: what is being read, by
// whom, and until when. Every part is in the URL, so changing any of them -
// another object, another viewer, a later expiry - invalidates it.
func (h *MediaHandler) signMedia(request mediaRequest, viewerID string, expires int64) string {
	mac := hmac.New(sha256.New, h.mediaSecret)
	fmt.Fprintf(mac, "%s\n%s\n%s\n%d", request.storageID, request.key, viewerID, expires)
	return hex.EncodeToString(mac.Sum(nil))
}

// authorizeMedia decides whether viewerID may read this object, and builds the
// target to read it with.
//
// The order is deliberate: a grant on the storage itself is checked first,
// because it is explicit - somebody said "you may see what is in here" - and
// then the uploader's profile visibility, which is the rule everything else
// about their content follows.
func (h *MediaHandler) authorizeMedia(w http.ResponseWriter, r *http.Request,
	request mediaRequest, viewerID string) (storage.Target, bool) {
	stored, err := h.storages.FindByID(r.Context(), request.storageID)
	if err != nil {
		// A storage that is not there reads as a missing object rather than a
		// missing storage: which ids exist is not a reader's business.
		writeError(w, http.StatusNotFound, "This file is not available", CodeNotFound)
		return nil, false
	}
	if !stored.Conn.Private {
		// A public bucket's objects are addressed at the bucket. Serving them
		// here as well would be a second, unlogged way to reach the same file.
		writeError(w, http.StatusNotFound, "This file is not available", CodeNotFound)
		return nil, false
	}

	if !h.mayReadMedia(r, stored, request.key, viewerID) {
		// Not "forbidden": the object's existence is not something to confirm
		// to somebody who may not read it.
		writeError(w, http.StatusNotFound, "This file is not available", CodeNotFound)
		return nil, false
	}

	target, err := storage.New(stored.Conn)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error(), CodeStorageNotConfigured)
		return nil, false
	}
	return target, true
}

// mayReadMedia is the rule itself.
func (h *MediaHandler) mayReadMedia(r *http.Request, stored models.Storage, key, viewerID string) bool {
	if utils.IsBlank(viewerID) {
		return false
	}

	// Anybody the storage's owner gave access to - including the owner.
	if role, err := h.storages.RoleFor(r.Context(), stored.ID, viewerID); err == nil &&
		models.CanReadStorage(role) {
		return true
	}

	// Otherwise it is the uploader's profile that decides. The key carries
	// whose upload it was (see mediaKey), which is what makes this answerable
	// without a row per object.
	uploaderID := uploaderFromKey(key)
	if utils.IsBlank(uploaderID) {
		// A key from somewhere else entirely: the owner's grants above are the
		// only way in.
		return false
	}
	if uploaderID == viewerID {
		return true
	}

	uploader, err := h.users.FindByID(r.Context(), uploaderID)
	if err != nil {
		return false
	}
	relation, err := h.profiles.RelationTo(r.Context(), uploader.ID, viewerID)
	if err != nil {
		return false
	}
	return models.CanSeeProfile(uploader.ProfileVisibility, relation)
}

// uploaderFromKey reads whose upload an object was out of its key.
//
// mediaKey writes "strong-fish/{userID}/{kind}/{name}", so the id is the
// second segment. Read rather than stored: an object per row would be a table
// this app does not otherwise need, and the key is written by this app alone.
func uploaderFromKey(key string) string {
	segments := strings.Split(strings.Trim(key, "/"), "/")
	if len(segments) < 3 || segments[0] != mediaKeyPrefix {
		return utils.EMPTY
	}
	return segments[1]
}

// writableStorage picks where an upload goes: the caller's own storage, or one
// shared with them as a writer.
//
// ?storageId picks between several; without it the caller's own wins, because
// that is what somebody who has one means. A member with neither is told to
// configure one, which is the same refusal as before this could be shared.
func (h *MediaHandler) writableStorage(w http.ResponseWriter, r *http.Request, userID string) (models.Storage, bool) {
	available, err := h.storages.ListFor(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return models.Storage{}, false
	}

	wanted := strings.TrimSpace(r.URL.Query().Get("storageId"))
	for _, candidate := range available {
		if utils.IsNotBlank(wanted) && candidate.ID != wanted {
			continue
		}
		if models.CanWriteStorage(candidate.Role) && candidate.Conn.Configured() {
			return candidate, true
		}
	}

	writeError(w, http.StatusMethodNotAllowed,
		"Configure your own storage bucket before uploading a video", CodeStorageNotConfigured)
	return models.Storage{}, false
}
