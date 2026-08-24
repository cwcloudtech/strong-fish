package handlers

import (
	"strings"
	"testing"

	"strong-fish-api/internal/models"
)

// TestPostedURL covers which address ends up in the post, per kind of target.
//
// This is the bug that shipped: a Drive upload returned an address the app
// then rewrote into one of its own, built from a bucket key Drive does not
// have - so the post carried a link that could never resolve, and on the phone
// (where Drive was the storage in use) an upload appeared to do nothing at all.
func TestPostedURL(t *testing.T) {
	handler := &MediaHandler{apiBaseURL: "https://api.example.com"}
	const key = "strong-fish/user-1/video/clip-abc.mp4"

	t.Run("a bucket is served by this API", func(t *testing.T) {
		target := models.Storage{ID: "st-1", Conn: models.StorageConnection{Type: models.StorageTypeS3}}
		got := handler.postedURL(target, key, "http://bucket.example.com/whatever.mp4", ".mp4")

		if !strings.HasPrefix(got, "https://api.example.com/v1/media/st-1/") {
			t.Errorf("posted %q, want an address on this API", got)
		}
		// The extension rides along so the players recognise it by shape.
		if !strings.HasSuffix(got, ".mp4") {
			t.Errorf("posted %q, want it to end in the file's extension", got)
		}
	})

	t.Run("a private bucket is no different", func(t *testing.T) {
		target := models.Storage{ID: "st-2", Conn: models.StorageConnection{Type: models.StorageTypeS3, Private: true}}
		got := handler.postedURL(target, key, "", ".mp4")

		if !strings.HasPrefix(got, "https://api.example.com/v1/media/st-2/") {
			t.Errorf("posted %q, want an address on this API", got)
		}
	})

	t.Run("a drive file keeps the address drive gave it", func(t *testing.T) {
		target := models.Storage{ID: "st-3", Conn: models.StorageConnection{Type: models.StorageTypeGoogleDrive}}
		const preview = "https://drive.google.com/file/d/1AbC/preview"

		got := handler.postedURL(target, key, preview, ".mp4")
		if got != preview {
			t.Errorf("posted %q, want Drive's own %q", got, preview)
		}
	})
}
