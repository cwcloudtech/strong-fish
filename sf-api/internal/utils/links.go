package utils

import "regexp"

// urlPattern matches an http(s) URL inside free text.
//
// The trailing class deliberately excludes the punctuation a sentence ends
// with, so "watch this: https://example.org/clip." doesn't capture the full
// stop, and a URL wrapped in brackets or quotes stops at the closing one. It
// is not a general-purpose URL grammar - it only has to recognise what
// somebody pastes into a post.
var urlPattern = regexp.MustCompile(`https?://[^\s<>"'\x60]+[^\s<>"'\x60.,;:!?)\]}]`)

// FirstURL returns the first http(s) URL in text, or "" if there is none.
//
// This is what replaced the separate "add a link" field: a post carries one
// piece of text, and whatever the author pasted into it is what gets rendered
// as a player or a link card.
func FirstURL(text string) string {
	found := urlPattern.FindString(text)
	if found == "" {
		// A URL that is the entire text has no trailing character for the
		// pattern's final class to match, so a bare "https://x.org" would
		// otherwise be missed.
		if bare := regexp.MustCompile(`^\s*(https?://\S+)\s*$`).FindStringSubmatch(text); bare != nil {
			return bare[1]
		}
	}
	return found
}
