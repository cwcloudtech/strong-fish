package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const EMPTY = ""

// IsNotBlank reports whether str contains at least one non-whitespace
// character.
func IsNotBlank(str string) bool {
	return len(strings.TrimSpace(str)) > 0
}

// IsBlank reports whether str is empty or contains only whitespace.
func IsBlank(str string) bool {
	return !IsNotBlank(str)
}

// If returns vtrue when cond is true, vfalse otherwise.
func If[T any](cond bool, vtrue, vfalse T) T {
	if cond {
		return vtrue
	}
	return vfalse
}

// GetEnv returns the environment variable key, or fallback when it is unset
// or blank.
func GetEnv(key, fallback string) string {
	return If(IsNotBlank(os.Getenv(key)), os.Getenv(key), fallback)
}

// IsTrue reports whether str is not a false value. False values are: false,
// no, off, ko, 0, and the empty string.
func IsTrue(str string) bool {
	if IsBlank(str) {
		return false
	}
	normalized := strings.TrimSpace(strings.ToLower(str))
	if slices.Contains([]string{"false", "ko", "no", "off", "0"}, normalized) {
		return false
	}
	if num, err := strconv.ParseFloat(normalized, 64); err == nil {
		return num > 0
	}
	return true
}

// SplitList splits a comma-or-semicolon-separated list, trims each entry and
// drops blanks - so an unset value yields an empty slice rather than [""].
func SplitList(str string) []string {
	out := []string{}
	for _, part := range strings.FieldsFunc(str, func(r rune) bool { return r == ',' || r == ';' }) {
		if part = strings.TrimSpace(part); IsNotBlank(part) {
			out = append(out, part)
		}
	}
	return out
}

func GetBaseUrl(url string) string {
	return strings.TrimSuffix(url, "/")
}

func GetBaseUrlFromEnvWithFallback(envKey, fallback string) string {
	return GetBaseUrl(GetEnv(envKey, fallback))
}

func GetBaseUrlFromEnv(envKey string) string {
	return GetBaseUrlFromEnvWithFallback(envKey, EMPTY)
}

// HashToken returns the sha256 hex digest of a plaintext token; only the
// digest is ever stored.
// RandomHex returns n cryptographically random bytes, hex-encoded (so 2n
// characters). Used where a name has to be unguessable but carries no meaning
// of its own - an uploaded object's key, for instance.
func RandomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return EMPTY, err
	}
	return hex.EncodeToString(buf), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// emailPattern is a permissive local@domain.tld shape check, not full RFC
// 5322 validation - just enough to catch obvious typos.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// IsValidEmail reports whether str looks like a plausible email address.
func IsValidEmail(str string) bool {
	return emailPattern.MatchString(strings.TrimSpace(str))
}

// minPasswordLength is the shortest password IsPasswordValid accepts,
// matching CWCloud's password policy.
const minPasswordLength = 8

var (
	passwordUpperPattern  = regexp.MustCompile(`[A-Z]`)
	passwordLowerPattern  = regexp.MustCompile(`[a-z]`)
	passwordSymbolPattern = regexp.MustCompile(`[^A-Za-z0-9]`)
)

// IsPasswordValid reports whether password meets the password policy - at
// least minPasswordLength characters with an uppercase letter, a lowercase
// letter and a symbol. When it doesn't, it also returns the i18n code of the
// first unmet rule, sent back to the client as the response's i18n_code.
func IsPasswordValid(password string) (bool, string) {
	switch {
	case len(password) < minPasswordLength:
		return false, "errors.passwordTooShort"
	case !passwordUpperPattern.MatchString(password):
		return false, "errors.passwordNoUpper"
	case !passwordLowerPattern.MatchString(password):
		return false, "errors.passwordNoLower"
	case !passwordSymbolPattern.MatchString(password):
		return false, "errors.passwordNoSymbol"
	default:
		return true, EMPTY
	}
}

// ImageSizeExceeds reports whether a base64 (optionally data-URI prefixed)
// image string decodes to more than maxBytes. A blank image never exceeds, so
// clearing a picture is always allowed regardless of the limit.
func ImageSizeExceeds(image string, maxBytes int64) bool {
	if IsBlank(image) {
		return false
	}
	payload := image
	if strings.HasPrefix(image, "data:") {
		if comma := strings.IndexByte(image, ','); comma >= 0 {
			payload = image[comma+1:]
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return false
	}
	return int64(len(decoded)) > maxBytes
}

// diacritics maps the accented letters French exercise names actually use to
// their ASCII equivalent, so "Élévations latérales" and "Elevations laterales"
// slugify identically.
var diacritics = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ä': 'a', 'ã': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'ö': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ý': 'y', 'ÿ': 'y', 'ñ': 'n', 'ç': 'c',
}

// Slugify normalizes a free-text name into the lowercase, dash-separated key
// exercises are looked up by - both for a coach's autocomplete and for
// resolving a spreadsheet's exercise column on import. Runs of anything that
// isn't a letter or a digit collapse into a single dash, so "Tempo squat
// 3:1:3", "tempo  squat 3-1-3" and "TEMPO_SQUAT_3.1.3" all reach the same
// entry.
func Slugify(name string) string {
	var b strings.Builder
	lastDash := true // leading separators are dropped
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if mapped, ok := diacritics[r]; ok {
			r = mapped
		}
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII, unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
