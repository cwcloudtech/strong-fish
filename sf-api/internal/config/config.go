package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

type Config struct {
	Port                     string
	DatabaseURL              string
	JWTSecret                string
	MaxWorkers               int
	PostgresPoolSize         int
	CorsEnabled              bool
	CorsAllowedOrigins       []string
	Version                  string
	ManifestPath             string
	MaxImageSize             int64
	MaxUploadSize            int64
	PlateIncrement           float64
	APIBaseURL               string
	UIBaseURL                string
	OIDCGoogleClientID       string
	OIDCGoogleClientSecret   string
	OIDCGithubClientID       string
	OIDCGithubClientSecret   string
	OIDCKeycloakBaseURL      string
	OIDCKeycloakClientID     string
	OIDCKeycloakClientSecret string
	OIDCKeycloakGroups       []string
	CWCloudAPIURL            string
	CWCloudAPIKey            string
	CWCloudContactFormID     string
	EmailFrom                string
	ConfirmationEmailTTL     time.Duration
	ActivationMode           string
	MobileURLPattern         string
	MaxVideoSize             int64
	// OTELEndpoint is the single collector traces, logs and metrics are all
	// pushed to. Blank disables export - logs still go to stdout/stderr, and
	// /v1/metrics still serves, so a deployment with no collector is fully
	// observable locally.
	OTELEndpoint string
	OTELProto    string
	// AboutURL is where the About link points. The page itself lives in the
	// wiki now, so the app links out rather than shipping a second copy of the
	// text that would drift from it.
	AboutURL string
	// DocURL is the wiki's own root, linked from the sidebar.
	DocURL string
}

const (
	// defaultMaxImageSize caps a single base64 image (avatar, club logo, post
	// picture) - they're stored inline in the JSONB payload, so this is also
	// what keeps a row from growing unbounded.
	defaultMaxImageSize int64 = 2 * 1024 * 1024
	// defaultMaxUploadSize caps an uploaded program spreadsheet.
	defaultMaxUploadSize int64 = 10 * 1024 * 1024
	// defaultConfirmationEmailExpirationHours bounds how long an
	// account-confirmation or password-reset link stays usable.
	defaultConfirmationEmailExpirationHours = 24
	// defaultMaxVideoSize caps one uploaded video. Unlike images, videos never
	// touch this app's own storage - they go to the member's own bucket - so
	// this is about what is reasonable to push through the API, not about what
	// a row can hold.
	defaultMaxVideoSize int64 = 20 * 1024 * 1024
	// defaultPlateIncrement is the weight step computed loads are rounded to
	// for the "on the bar" column: 2.5kg is one small plate per side.
	defaultPlateIncrement = 2.5
)

func Load() Config {
	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASSWORD")
	host := os.Getenv("POSTGRES_HOST")
	port := utils.GetEnv("POSTGRES_PORT", "5432")
	database := os.Getenv("POSTGRES_DB")
	sslmode := utils.GetEnv("POSTGRES_SSLMODE", "disable")

	maxWorkers, err := strconv.Atoi(utils.GetEnv("MAX_WORKERS", "5"))
	if err != nil || maxWorkers <= 0 {
		maxWorkers = 5
	}

	postgresPoolSize, err := strconv.Atoi(utils.GetEnv("POSTGRES_POOL_SIZE", "5"))
	if err != nil || postgresPoolSize <= 0 {
		postgresPoolSize = 5
	}

	maxImageSize, err := strconv.ParseInt(os.Getenv("SF_MAX_IMAGE_SIZE"), 10, 64)
	if err != nil || maxImageSize <= 0 {
		maxImageSize = defaultMaxImageSize
	}

	maxUploadSize, err := strconv.ParseInt(os.Getenv("SF_MAX_UPLOAD_SIZE"), 10, 64)
	if err != nil || maxUploadSize <= 0 {
		maxUploadSize = defaultMaxUploadSize
	}

	// A negative increment would round loads backwards; zero is meaningful
	// though - it disables rounding for gyms with fractional plates.
	plateIncrement, err := strconv.ParseFloat(os.Getenv("SF_PLATE_INCREMENT"), 64)
	if err != nil || plateIncrement < 0 {
		plateIncrement = defaultPlateIncrement
	}

	maxVideoSize, err := strconv.ParseInt(os.Getenv("SF_MAX_VIDEO_SIZE"), 10, 64)
	if err != nil || maxVideoSize <= 0 {
		maxVideoSize = defaultMaxVideoSize
	}

	confirmationHours, err := strconv.Atoi(os.Getenv("SF_CONFIRMATION_EMAIL_EXPIRATION"))
	if err != nil || confirmationHours <= 0 {
		confirmationHours = defaultConfirmationEmailExpirationHours
	}

	// Anyone can subscribe, so email confirmation is the default: a new
	// account activates itself through its link rather than waiting on a
	// superadmin.
	activationMode := utils.GetEnv("SF_ACTIVATION_MODE", models.ActivationModeEmail)
	if !models.IsValidActivationMode(activationMode) {
		activationMode = models.ActivationModeEmail
	}

	return Config{
		Port:                     utils.GetEnv("PORT", "8080"),
		DatabaseURL:              fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, database, sslmode),
		JWTSecret:                os.Getenv("JWT_SECRET"),
		MaxWorkers:               maxWorkers,
		PostgresPoolSize:         postgresPoolSize,
		CorsEnabled:              utils.IsTrue(os.Getenv("SF_CORS_ENABLED")),
		CorsAllowedOrigins:       utils.SplitList(utils.GetEnv("SF_CORS_ALLOWED_ORIGINS", "*")),
		Version:                  versionFromManifest(utils.GetEnv("SF_MANIFEST_PATH", "manifest.json"), utils.GetEnv("APP_VERSION", "1.0.0")),
		ManifestPath:             utils.GetEnv("SF_MANIFEST_PATH", "manifest.json"),
		MaxImageSize:             maxImageSize,
		MaxUploadSize:            maxUploadSize,
		PlateIncrement:           plateIncrement,
		APIBaseURL:               utils.GetBaseUrlFromEnvWithFallback("SF_API_URL", "http://localhost:8080"),
		UIBaseURL:                utils.GetBaseUrlFromEnvWithFallback("SF_UI_URL", "http://localhost:3000"),
		OIDCGoogleClientID:       os.Getenv("SF_OIDC_GOOGLE_CLIENT_ID"),
		OIDCGoogleClientSecret:   os.Getenv("SF_OIDC_GOOGLE_CLIENT_SECRET"),
		OIDCGithubClientID:       os.Getenv("SF_OIDC_GITHUB_CLIENT_ID"),
		OIDCGithubClientSecret:   os.Getenv("SF_OIDC_GITHUB_CLIENT_SECRET"),
		OIDCKeycloakBaseURL:      utils.GetBaseUrlFromEnv("SF_OIDC_KEYCLOAK_BASE_URL"),
		OIDCKeycloakClientID:     os.Getenv("SF_OIDC_KEYCLOAK_CLIENT_ID"),
		OIDCKeycloakClientSecret: os.Getenv("SF_OIDC_KEYCLOAK_CLIENT_SECRET"),
		OIDCKeycloakGroups:       utils.SplitList(os.Getenv("SF_OIDC_KEYCLOAK_GROUPS")),
		CWCloudAPIURL:            utils.GetBaseUrlFromEnvWithFallback("CWCLOUD_API_URL", "https://api.cwcloud.tech"),
		CWCloudAPIKey:            os.Getenv("CWCLOUD_API_KEY"),
		CWCloudContactFormID:     os.Getenv("CWCLOUD_CONTACT_FORM_ID"),
		EmailFrom:                utils.GetEnv("SF_MAIL_FROM", "noreply@cwcloud.tech"),
		ConfirmationEmailTTL:     time.Duration(confirmationHours) * time.Hour,
		ActivationMode:           activationMode,
		MobileURLPattern:         utils.GetEnv("SF_MOBILE_URL_PATTERN", "/strong-fish-v{version}.apk"),
		MaxVideoSize:             maxVideoSize,
		OTELEndpoint:             os.Getenv("SF_OTEL_ENDPOINT"),
		OTELProto:                utils.GetEnv("SF_OTEL_PROTO", "otlp/grpc"),
		AboutURL:                 utils.GetEnv("SF_ABOUT_URL", "https://doc.strong-fish.com/docs/about"),
		DocURL:                   utils.GetEnv("SF_DOC_URL", "https://doc.strong-fish.com"),
	}
}

// versionFromManifest reads the version field from the manifest JSON file at
// path, falling back to fallback if the file can't be read or parsed.
func versionFromManifest(path, fallback string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	var m struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &m); err != nil || utils.IsBlank(m.Version) {
		return fallback
	}
	return m.Version
}
