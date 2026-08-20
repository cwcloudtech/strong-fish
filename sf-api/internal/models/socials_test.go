package models

import "testing"

// TestNormalizeSocialAccount pins what a link ends up pointing at. The clients
// build "<base>/<stored>", so anything that survives this function is a live
// link on somebody's profile - which is exactly why a value that is not a name
// on that service has to come back empty rather than half-cleaned.
func TestNormalizeSocialAccount(t *testing.T) {
	cases := []struct {
		name, network, in, want string
	}{
		{"a plain name", SocialInstagram, "marie.lifts", "marie.lifts"},
		{"the at somebody types", SocialInstagram, "@marie.lifts", "marie.lifts"},
		{"spaces around it", SocialInstagram, "  marie.lifts  ", "marie.lifts"},
		{"nothing", SocialInstagram, "", ""},

		// People paste the address bar. That is simply what happens.
		{"a pasted profile URL", SocialInstagram, "https://www.instagram.com/marie.lifts", "marie.lifts"},
		{"with a trailing slash", SocialInstagram, "https://instagram.com/marie.lifts/", "marie.lifts"},
		{"without the scheme", SocialInstagram, "instagram.com/marie.lifts", "marie.lifts"},
		{"tiktok keeps its at out of the name", SocialTikTok, "https://www.tiktok.com/@marie.lifts", "marie.lifts"},
		{"x, from the old host", SocialX, "https://twitter.com/marielifts", "marielifts"},
		{"bluesky handles are domains", SocialBluesky, "https://bsky.app/profile/marie.bsky.social", "marie.bsky.social"},
		{"openpowerlifting's lifter page", SocialOpenPowerlifting, "https://www.openpowerlifting.org/u/mariedubois", "mariedubois"},

		// A URL of some *other* site is not this network's account. Keeping the
		// last segment would build a link to a stranger's profile.
		{"somebody else's site", SocialInstagram, "https://example.com/marie", ""},
		{"a bare path", SocialInstagram, "some/path", ""},
		{"a whole sentence", SocialInstagram, "find me on instagram", ""},
		{"a scheme on its own", SocialInstagram, "javascript:alert(1)", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeSocialAccount(tc.network, tc.in); got != tc.want {
				t.Errorf("NormalizeSocialAccount(%q, %q) = %q, want %q", tc.network, tc.in, got, tc.want)
			}
		})
	}
}

// TestNormalizeSocialsTruncates guards the one field that is free text: a rank
// is read off a page and typed in, and a paste of the whole page should not
// become a profile field.
func TestNormalizeSocialsTruncates(t *testing.T) {
	long := make([]rune, maxSocialLength*2)
	for i := range long {
		long[i] = 'a'
	}

	got := NormalizeSocials(Socials{OpenPowerliftingRank: string(long), Instagram: string(long)})
	if len([]rune(got.OpenPowerliftingRank)) != maxSocialLength {
		t.Errorf("rank kept %d characters, want %d", len([]rune(got.OpenPowerliftingRank)), maxSocialLength)
	}
	if len([]rune(got.Instagram)) != maxSocialLength {
		t.Errorf("account kept %d characters, want %d", len([]rune(got.Instagram)), maxSocialLength)
	}
}

// TestSocialsGetCoversEveryNetwork keeps the accessor and the list in step: a
// network added to one and forgotten in the other would render as an empty
// field the member cannot fill.
func TestSocialsGetCoversEveryNetwork(t *testing.T) {
	filled := Socials{
		Instagram: "a", TikTok: "b", X: "c", Bluesky: "d", OpenPowerlifting: "e",
	}
	for _, network := range SocialNetworks {
		if filled.Get(network) == "" {
			t.Errorf("Get(%q) returned nothing for a filled profile", network)
		}
	}
	if filled.Empty() {
		t.Error("a filled profile reports itself empty")
	}
	if !(Socials{}).Empty() {
		t.Error("an untouched profile does not report itself empty")
	}
}
