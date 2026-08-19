package models

import "testing"

// A colour reaches a stylesheet, so what gets stored has to be a colour and
// nothing else: a client that sends a name, a shorthand or a function call
// should have it dropped, not passed through.
func TestNormalizeHexColor(t *testing.T) {
	cases := []struct {
		name  string
		given string
		want  string
	}{
		{"a six-digit hex is kept", "#1cb9f7", "#1cb9f7"},
		{"case is normalized so equal colours compare equal", "#1CB9F7", "#1cb9f7"},
		{"surrounding space is trimmed", "  #1cb9f7 ", "#1cb9f7"},
		{"no colour is not an error", "", ""},
		{"three-digit shorthand is dropped: the UI never writes it", "#abc", ""},
		{"a colour name is dropped", "red", ""},
		{"a missing hash is dropped", "1cb9f7", ""},
		{"eight digits are dropped: alpha is not the client's to set", "#1cb9f7ff", ""},
		{"a CSS function is dropped", "rgb(0,0,0)", ""},
		{"an injection attempt is dropped rather than escaped", "#000;background:url(x)", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeHexColor(tc.given); got != tc.want {
				t.Fatalf("NormalizeHexColor(%q) = %q, want %q", tc.given, got, tc.want)
			}
		})
	}
}
