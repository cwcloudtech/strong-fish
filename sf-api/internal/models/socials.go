package models

import (
	"net/url"
	"strings"
)

// The accounts a member may show on their profile.
//
// Each is stored as the account's own name on that service - "marie.lifts",
// not "https://instagram.com/marie.lifts" - because the address is a property
// of the network rather than of the member: the clients hold the network's base
// URL beside its label and icon, and build the link from the two. Storing whole
// URLs instead would let a profile link anywhere, which is a link somebody else
// clicks.
const (
	SocialInstagram        = "instagram"
	SocialTikTok           = "tiktok"
	SocialX                = "x"
	SocialBluesky          = "bluesky"
	SocialOpenPowerlifting = "openpowerlifting"
)

// SocialNetworks are the keys a profile may carry, in the order the forms
// offer them. The clients keep a table of the same keys with a label, an icon
// and a base URL each; this list is what the API accepts, and the test beside
// it is what keeps the two from drifting.
var SocialNetworks = []string{
	SocialInstagram, SocialTikTok, SocialX, SocialBluesky, SocialOpenPowerlifting,
}

// socialHosts are the addresses a member is likely to paste instead of typing
// their name, per network. Anything whose host ends in one of these is read as
// that network's own URL and reduced to the account in it.
var socialHosts = map[string][]string{
	SocialInstagram:        {"instagram.com"},
	SocialTikTok:           {"tiktok.com"},
	SocialX:                {"x.com", "twitter.com"},
	SocialBluesky:          {"bsky.app"},
	SocialOpenPowerlifting: {"openpowerlifting.org"},
}

// maxSocialLength bounds what is stored. Every one of these services caps its
// own names well below this; the limit is here so a paste of something else
// entirely does not become a profile field.
const maxSocialLength = 120

// Socials is a member's presence elsewhere, as shown on their profile.
type Socials struct {
	Instagram        string `json:"instagram,omitempty"`
	TikTok           string `json:"tiktok,omitempty"`
	X                string `json:"x,omitempty"`
	Bluesky          string `json:"bluesky,omitempty"`
	OpenPowerlifting string `json:"openpowerlifting,omitempty"`
	// OpenPowerliftingRank is what the federation's database ranks them at -
	// free text, because it is a placing somebody reads off a page ("12th",
	// "Elite", "#340 FR -83kg") rather than a number this app computes.
	OpenPowerliftingRank string `json:"openpowerliftingRank,omitempty"`
}

// Get returns one network's account by key.
func (s Socials) Get(network string) string {
	switch network {
	case SocialInstagram:
		return s.Instagram
	case SocialTikTok:
		return s.TikTok
	case SocialX:
		return s.X
	case SocialBluesky:
		return s.Bluesky
	case SocialOpenPowerlifting:
		return s.OpenPowerlifting
	}
	return ""
}

// Empty reports whether nothing at all was filled in, so a profile with no
// accounts carries no social section rather than an empty one.
func (s Socials) Empty() bool {
	return s == Socials{}
}

// NormalizeSocials cleans up what a form sent, field by field.
func NormalizeSocials(s Socials) Socials {
	return Socials{
		Instagram:            NormalizeSocialAccount(SocialInstagram, s.Instagram),
		TikTok:               NormalizeSocialAccount(SocialTikTok, s.TikTok),
		X:                    NormalizeSocialAccount(SocialX, s.X),
		Bluesky:              NormalizeSocialAccount(SocialBluesky, s.Bluesky),
		OpenPowerlifting:     NormalizeSocialAccount(SocialOpenPowerlifting, s.OpenPowerlifting),
		OpenPowerliftingRank: truncate(strings.TrimSpace(s.OpenPowerliftingRank), maxSocialLength),
	}
}

// NormalizeSocialAccount reduces whatever somebody typed to the account name
// the clients build a link from.
//
// People paste the address bar - that is simply what happens - so a URL of the
// right network is accepted and the account taken out of it. A leading "@" goes
// too. Anything left carrying a slash, a space or a colon is refused rather
// than half-repaired: it is not a name on that service, and a link built from
// it would go somewhere unintended.
func NormalizeSocialAccount(network, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if account, ok := accountFromURL(network, value); ok {
		value = account
	}
	value = strings.TrimPrefix(strings.TrimSpace(value), "@")

	// Bluesky handles are domains, so a dot is legitimate there; nothing on
	// this list may contain a path separator or whitespace.
	if strings.ContainsAny(value, "/\\ \t:?#") {
		return ""
	}
	return truncate(value, maxSocialLength)
}

// accountFromURL takes the account out of one of the network's own addresses.
func accountFromURL(network, value string) (string, bool) {
	if !strings.Contains(value, "/") {
		return "", false
	}

	// A pasted address may arrive without its scheme ("instagram.com/marie"),
	// which url.Parse reads as a path rather than a host.
	candidate := value
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", false
	}

	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	known := false
	for _, suffix := range socialHosts[network] {
		if host == suffix {
			known = true
			break
		}
	}
	if !known {
		return "", false
	}

	// The last non-empty segment is the account on every one of these:
	// instagram.com/marie, tiktok.com/@marie, bsky.app/profile/marie.bsky.social,
	// openpowerlifting.org/u/mariedubois.
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		if segment := strings.TrimSpace(segments[i]); segment != "" {
			return segment, true
		}
	}
	return "", false
}

func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
