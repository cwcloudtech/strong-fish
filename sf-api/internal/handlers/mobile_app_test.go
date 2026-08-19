package handlers

import "testing"

// The mobile app used to build the download URL itself, from a host hardcoded
// in the Flutter source. It said strong-fish.com while CI publishes the build
// at www.strong-fish.com, so the download returned an error page, which was
// saved under an .apk name and only failed at Android's installer - reported as
// "there's a problem with the app file", nowhere near the actual cause.
//
// The app now asks this endpoint instead, so there is one answer to "where is
// the APK" and it comes from the same settings the web app's download link
// uses. That makes these the tests that keep the two in step.
func TestMobileAppURL(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		uiURL   string
		version string
		want    string
	}{
		{
			name:    "the deployed configuration",
			pattern: "/strong-fish-v{version}.apk",
			uiURL:   "https://www.strong-fish.com",
			version: "1.0.2",
			want:    "https://www.strong-fish.com/strong-fish-v1.0.2.apk",
		},
		{
			// A path has to be resolved against the frontend's origin: a QR
			// code or a phone has nothing to resolve "/strong-fish-v1.0.2.apk"
			// against.
			name:    "a trailing slash on the base does not double up",
			pattern: "/strong-fish-v{version}.apk",
			uiURL:   "https://www.strong-fish.com/",
			version: "1.0.2",
			want:    "https://www.strong-fish.com/strong-fish-v1.0.2.apk",
		},
		{
			name:    "an absolute pattern is taken as given",
			pattern: "https://downloads.example.org/sf-{version}.apk",
			uiURL:   "https://www.strong-fish.com",
			version: "2.1.0",
			want:    "https://downloads.example.org/sf-2.1.0.apk",
		},
		{
			// No published build: the endpoint answers 404 and the app offers
			// no button, rather than one that downloads nothing.
			name:    "no pattern means no download",
			pattern: "",
			uiURL:   "https://www.strong-fish.com",
			version: "1.0.2",
			want:    "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewConfigHandler(nil, "email", 2.5, 0, c.version, false,
				"https://api.strong-fish.com", c.uiURL, c.pattern, "", "")
			if got := h.mobileAppURL(); got != c.want {
				t.Errorf("mobileAppURL() = %q, want %q", got, c.want)
			}
		})
	}
}
