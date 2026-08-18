package utils

import "testing"

func TestFirstURL(t *testing.T) {
	cases := []struct{ text, want string }{
		{"", ""},
		{"no link here", ""},
		{"https://example.org/clip.mp4", "https://example.org/clip.mp4"},
		{"  https://example.org/clip.mp4  ", "https://example.org/clip.mp4"},
		{"watch this https://youtu.be/abc123 it was heavy", "https://youtu.be/abc123"},
		// Sentence punctuation is not part of the URL.
		{"watch this: https://example.org/clip.", "https://example.org/clip"},
		{"see (https://example.org/a) for more", "https://example.org/a"},
		// Only the first one - a post has one embed, not a wall of them.
		{"https://a.example/1 and https://b.example/2", "https://a.example/1"},
		{"http://plain.example/x", "http://plain.example/x"},
		// A query string survives intact.
		{"https://www.youtube.com/watch?v=abc_123 nice", "https://www.youtube.com/watch?v=abc_123"},
		// Not a URL scheme this renders.
		{"ftp://example.org/file", ""},
	}

	for _, c := range cases {
		if got := FirstURL(c.text); got != c.want {
			t.Errorf("FirstURL(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}
