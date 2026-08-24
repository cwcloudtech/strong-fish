// Package storage uploads a member's video to the object store they configured
// for themselves.
//
// strong-fish deliberately hosts no video of its own: a 20MB file per post is
// not something a JSONB column should hold, and paying to serve other people's
// training footage is not this app's business. So a member who wants to post a
// video brings their own bucket - an S3-compatible one, or a Google Drive
// folder - and what ends up in the post is a link to the object in it.
//
// Both providers are addressed through the same Target, and both are spoken to
// over their plain REST APIs: an S3 request is hand-signed with SigV4 and a
// Drive one with a service-account JWT, so neither cloud SDK is a dependency.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"strong-fish-api/internal/models"
)

// ErrNotConfigured is what an upload attempt returns when the member has no
// storage connection. The handler turns it into a 405, which is what the
// client toasts.
var ErrNotConfigured = errors.New("no storage connection is configured")

// Target is one member's storage destination.
type Target interface {
	// Upload stores data under key and returns the URL the object can be read
	// at.
	//
	// On a public connection that URL has to be readable by a browser with no
	// credentials - it goes into a post, to be played by a plain media element
	// - so the implementation grants public access as it writes. On a private
	// one it grants nothing, and the URL it returns is only reachable by
	// Download: the API serves those objects itself, to readers it has
	// checked.
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)

	// Download reads an object back, for a private connection whose objects
	// nobody else can fetch directly. rangeHeader is the reader's own Range
	// request, passed through untouched: a video player seeks by asking for
	// byte ranges, and a store that supports it should answer it rather than
	// having this app buffer a whole file to serve the middle of it.
	Download(ctx context.Context, key, rangeHeader string) (*Object, error)
}

// Object is a stored file being read back.
type Object struct {
	// Body is the caller's to close.
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	// ContentRange and StatusCode carry a partial answer through untouched, so
	// a player seeking in a video gets the 206 it asked for.
	ContentRange string
	StatusCode   int
	AcceptRanges string
}

// New builds the Target for a connection, or ErrNotConfigured when there is
// nothing usable to build one from.
func New(conn models.StorageConnection) (Target, error) {
	if !conn.Configured() {
		return nil, ErrNotConfigured
	}
	switch conn.Type {
	case models.StorageTypeS3:
		return newS3Target(conn), nil
	case models.StorageTypeGoogleDrive:
		return newDriveTarget(conn)
	default:
		return nil, fmt.Errorf("unknown storage type %q", conn.Type)
	}
}
