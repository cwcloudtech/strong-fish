package handlers

import (
	"mime/multipart"
	"net/textproto"
	"testing"
)

// TestResolveMediaType covers how an upload's type is decided.
//
// The case that shipped broken is the third one: the phone app's HTTP library
// is handed a path, not a browser File, and sends every part as
// application/octet-stream - so a perfectly ordinary .mp4 was refused as "not
// something a browser can play", on mobile only. What must NOT come out of
// fixing that is a filename becoming a way past the allow-list.
func TestResolveMediaType(t *testing.T) {
	accepted := map[string]string{
		"video/mp4":  ".mp4",
		"video/webm": ".webm",
	}

	cases := []struct {
		name        string
		declared    string
		filename    string
		wantType    string
		wantExt     string
		wantAllowed bool
	}{
		{"a browser declaring the type", "video/mp4", "clip.mp4", "video/mp4", ".mp4", true},
		{"with a charset parameter", "video/mp4; charset=binary", "clip.mp4", "video/mp4", ".mp4", true},

		// What the phone sends.
		{"unlabelled, known extension", "application/octet-stream", "clip.mp4", "video/mp4", ".mp4", true},
		{"no type header at all", "", "clip.webm", "video/webm", ".webm", true},
		{"unlabelled, uppercase extension", "application/octet-stream", "CLIP.MP4", "video/mp4", ".mp4", true},

		// And what the fallback must still refuse.
		{"unlabelled, extension nobody serves", "application/octet-stream", "archive.zip", "", "", false},
		{"unlabelled, no extension", "application/octet-stream", "clip", "", "", false},
		{"a type this app does not serve", "video/x-matroska", "clip.mkv", "", "", false},
		// Named as something unserved, but with an accepted extension: the
		// client said what it was, and that answer is taken.
		{"a wrong name is not rescued by the extension", "application/zip", "clip.mp4", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := &multipart.FileHeader{Filename: tc.filename, Header: textproto.MIMEHeader{}}
			if tc.declared != "" {
				header.Header.Set("Content-Type", tc.declared)
			}

			mediaType, extension, allowed := resolveMediaType(header, accepted)
			if allowed != tc.wantAllowed {
				t.Fatalf("allowed = %v, want %v", allowed, tc.wantAllowed)
			}
			if mediaType != tc.wantType || extension != tc.wantExt {
				t.Errorf("got (%q, %q), want (%q, %q)", mediaType, extension, tc.wantType, tc.wantExt)
			}
		})
	}
}
