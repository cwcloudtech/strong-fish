package stance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// supabaseURLTemplate builds a project's API base URL from its ref: every
// Supabase project answers at https://<ref>.supabase.co.
const supabaseURLTemplate = "https://%s.supabase.co"

// idpRequestURI is the requestUri Firebase wants on an signInWithIdp call.
// With a Google ID token obtained on the device there is no real redirect, so
// any valid URI does; the value is not used for anything.
const idpRequestURI = "https://localhost"

// errGoogleDisabled is returned when a Google sign-in is attempted without the
// Google client id configured - the option should not have been offered.
var errGoogleDisabled = errors.New("stance: google sign-in is not configured")

// Config is what an operator sets to switch the integration on. Every value is
// provided by Stance (hello@stancefitness.co) and is one the Stance app itself
// uses: a Firebase Web API key, a public Google client id, and the Supabase
// project and anon key. There is no client secret - the Google ID token is
// obtained on the client with the public client id, exactly as the app does.
//
// Empty leaves the feature off: the provider lists as unavailable.
type Config struct {
	// FirebaseAPIKey is the project's Web API key - the client key Firebase
	// Auth calls carry. Required.
	FirebaseAPIKey string
	// GoogleClientID is the project's public Google client id (the app's
	// default_web_client_id). The client uses it to get a Google ID token;
	// StrongFish only hands that token to Firebase. Optional: without it, only
	// email/password pairing is offered. No secret is involved.
	GoogleClientID string
	// SupabaseProjectRef names the Stance project; its API base URL is built
	// from it with supabaseURLTemplate. SupabaseAnonKey is the project's
	// client key. Both empty means the feature is off.
	SupabaseProjectRef string
	SupabaseAnonKey    string
	// SupabaseURL overrides the base URL built from SupabaseProjectRef. Blank
	// in normal use; set directly to point at a stand-in while developing, or
	// in tests.
	SupabaseURL string
	// UsersTable resolves a Firebase uid to the internal user id; SessionsRPC
	// returns that user's sets. Overridable in case Stance's names differ.
	UsersTable  string
	SessionsRPC string
	// AppURL is where the "record a set" button sends the member: the Stance
	// app. Blank defaults to the app's store entry.
	AppURL string
	// The identity endpoints, overridable for tests/stand-ins.
	FirebaseAuthURL  string
	FirebaseTokenURL string
	// HTTPClient is injected by tests; nil uses a default with a sane timeout.
	HTTPClient *http.Client
}

// Enabled reports whether there is enough to attempt a call: an identity side
// (Firebase) and a read side (Supabase).
func (c Config) Enabled() bool {
	hasSupabase := strings.TrimSpace(c.SupabaseProjectRef) != "" || strings.TrimSpace(c.SupabaseURL) != ""
	return strings.TrimSpace(c.FirebaseAPIKey) != "" &&
		strings.TrimSpace(c.SupabaseAnonKey) != "" && hasSupabase
}

// GoogleEnabled reports whether the Google sign-in option is configured.
func (c Config) GoogleEnabled() bool {
	return c.Enabled() && strings.TrimSpace(c.GoogleClientID) != ""
}

// withDefaults fills in the standard endpoints and Stance's documented shapes
// for anything left blank, and derives the Supabase base URL from the ref.
func (c Config) withDefaults() Config {
	c.SupabaseURL = strings.TrimRight(strings.TrimSpace(c.SupabaseURL), "/")
	if c.SupabaseURL == "" {
		// Tolerant of the project ref being given as a bare ref, a host, or a
		// full URL, so a pasted "https://<ref>.supabase.co" still works.
		ref := strings.TrimRight(strings.TrimSpace(c.SupabaseProjectRef), "/")
		switch {
		case ref == "":
		case strings.Contains(ref, "://"):
			c.SupabaseURL = ref
		case strings.Contains(ref, "."):
			c.SupabaseURL = "https://" + ref
		default:
			c.SupabaseURL = fmt.Sprintf(supabaseURLTemplate, ref)
		}
	}
	if c.FirebaseAuthURL == "" {
		c.FirebaseAuthURL = "https://identitytoolkit.googleapis.com/v1"
	}
	if c.FirebaseTokenURL == "" {
		c.FirebaseTokenURL = "https://securetoken.googleapis.com/v1"
	}
	c.FirebaseAuthURL = strings.TrimRight(c.FirebaseAuthURL, "/")
	c.FirebaseTokenURL = strings.TrimRight(c.FirebaseTokenURL, "/")
	if c.UsersTable == "" {
		c.UsersTable = "users"
	}
	if c.SessionsRPC == "" {
		c.SessionsRPC = "get_user_sessions"
	}
	if c.AppURL == "" {
		c.AppURL = "https://play.google.com/store/apps/details?id=com.stancehome.Stance_Fitness"
	}
	return c
}

// client is the one implementation of Client. Identity goes through Firebase
// Auth (this file); the sets are read from Supabase (supabase.go).
type client struct {
	cfg  Config
	http *http.Client
}

// New builds a Client from a Config, filling defaults.
func New(cfg Config) Client {
	cfg = cfg.withDefaults()
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &client{cfg: cfg, http: httpClient}
}

func (c *client) Enabled() bool          { return c.cfg.Enabled() }
func (c *client) GoogleEnabled() bool    { return c.cfg.GoogleEnabled() }
func (c *client) GoogleClientID() string { return c.cfg.GoogleClientID }
func (c *client) RecordURL() string      { return c.cfg.AppURL }

// --- Firebase Auth ---

// firebaseAuthResponse is the shape of an identitytoolkit sign-in, and of its
// error.
type firebaseAuthResponse struct {
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	LocalID      string `json:"localId"`
	ExpiresIn    string `json:"expiresIn"`
	Error        struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (r firebaseAuthResponse) session() Session {
	ttl := 3600
	if n, err := strconv.Atoi(r.ExpiresIn); err == nil && n > 0 {
		ttl = n
	}
	return Session{
		IDToken:      r.IDToken,
		RefreshToken: r.RefreshToken,
		FirebaseUID:  r.LocalID,
		ExpiresAt:    time.Now().Add(time.Duration(ttl) * time.Second),
	}
}

// PairPassword signs in with Firebase Auth email/password.
func (c *client) PairPassword(ctx context.Context, email, password string) (Session, error) {
	body := map[string]any{"email": email, "password": password, "returnSecureToken": true}
	var resp firebaseAuthResponse
	status, err := c.firebaseAuthPost(ctx, "accounts:signInWithPassword", body, &resp)
	if err != nil {
		return Session{}, err
	}
	if status/100 != 2 || resp.IDToken == "" {
		return Session{}, signInError(resp.Error.Message)
	}
	return resp.session(), nil
}

// PairGoogle signs the member in to Firebase with a Google ID token the client
// obtained with the public Google client id (providerId google.com) - the same
// exchange the app does after Google sign-in. No secret or code is involved.
func (c *client) PairGoogle(ctx context.Context, googleIDToken string) (Session, error) {
	if !c.cfg.GoogleEnabled() {
		return Session{}, errGoogleDisabled
	}
	if strings.TrimSpace(googleIDToken) == "" {
		return Session{}, ErrRefused
	}
	body := map[string]any{
		"postBody":            "id_token=" + url.QueryEscape(googleIDToken) + "&providerId=google.com",
		"requestUri":          idpRequestURI,
		"returnSecureToken":   true,
		"returnIdpCredential": true,
	}
	var resp firebaseAuthResponse
	status, err := c.firebaseAuthPost(ctx, "accounts:signInWithIdp", body, &resp)
	if err != nil {
		return Session{}, err
	}
	if status/100 != 2 || resp.IDToken == "" {
		return Session{}, signInError(resp.Error.Message)
	}
	return resp.session(), nil
}

// Refresh renews a session from its Firebase refresh token (securetoken).
func (c *client) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	endpoint := c.cfg.FirebaseTokenURL + "/token?key=" + url.QueryEscape(c.cfg.FirebaseAPIKey)
	var resp struct {
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		UserID       string `json:"user_id"`
		ExpiresIn    string `json:"expires_in"`
	}
	status, err := c.doForm(ctx, endpoint, form, &resp)
	if err != nil {
		return Session{}, err
	}
	if status/100 != 2 || resp.IDToken == "" {
		return Session{}, ErrReauthorize
	}
	ttl := 3600
	if n, convErr := strconv.Atoi(resp.ExpiresIn); convErr == nil && n > 0 {
		ttl = n
	}
	return Session{
		IDToken:      resp.IDToken,
		RefreshToken: firstNonEmpty(resp.RefreshToken, refreshToken),
		FirebaseUID:  resp.UserID,
		ExpiresAt:    time.Now().Add(time.Duration(ttl) * time.Second),
	}, nil
}

// --- transport ---

// firebaseAuthPost posts a JSON body to an identitytoolkit method, keyed by
// the Web API key. out is filled from the response even on error, so the
// caller can read error.message.
func (c *client) firebaseAuthPost(ctx context.Context, method string, body any, out any) (int, error) {
	endpoint := c.cfg.FirebaseAuthURL + "/" + method + "?key=" + url.QueryEscape(c.cfg.FirebaseAPIKey)
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.send(req, out)
}

// doForm posts a urlencoded form and decodes JSON.
func (c *client) doForm(ctx context.Context, endpoint string, form url.Values, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return c.send(req, out)
}

func (c *client) send(req *http.Request, out any) (int, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if out != nil && len(data) > 0 {
		_ = json.Unmarshal(data, out)
	}
	return resp.StatusCode, nil
}

// signInError turns a Firebase sign-in error message into the seam's error. A
// bad password or unknown account is a refusal the member can fix; anything
// else is reported as-is.
func signInError(message string) error {
	switch strings.ToUpper(strings.TrimSpace(message)) {
	case "", "INVALID_LOGIN_CREDENTIALS", "INVALID_PASSWORD", "EMAIL_NOT_FOUND",
		"INVALID_EMAIL", "MISSING_PASSWORD", "USER_DISABLED", "INVALID_IDP_RESPONSE":
		return ErrRefused
	}
	return fmt.Errorf("stance: sign-in failed (%s)", message)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
