package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime"

	"github.com/go-webauthn/webauthn/webauthn"

	"strong-fish-api/internal/config"
	"strong-fish-api/internal/contact"
	"strong-fish-api/internal/db"
	"strong-fish-api/internal/email"
	"strong-fish-api/internal/handlers"
	"strong-fish-api/internal/oidc"
	"strong-fish-api/internal/router"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// mfaIssuer is the "issuer" shown by authenticator apps (Google Authenticator
// and friends) next to an enrolled TOTP entry.
const mfaIssuer = "strong-fish"

func main() {
	cfg := config.Load()
	runtime.GOMAXPROCS(cfg.MaxWorkers)

	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL, cfg.PostgresPoolSize)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	userStore := store.NewUserStore(pool)
	clubStore := store.NewClubStore(pool)
	exerciseStore := store.NewExerciseStore(pool)
	oneRMStore := store.NewOneRMStore(pool)
	programStore := store.NewProgramStore(pool)
	socialStore := store.NewSocialStore(pool)
	webauthnStore := store.NewWebAuthnCredentialStore(pool)

	mailer := email.NewSender(cfg.CWCloudAPIURL, cfg.CWCloudAPIKey, cfg.EmailFrom, cfg.APIBaseURL)
	contactClient := contact.New(cfg.CWCloudAPIURL, cfg.CWCloudContactFormID)

	// WebAuthn binds credentials to a relying-party id, which must be the
	// frontend's bare hostname - a security key registered against one origin
	// won't assert against another.
	rpID := cfg.UIBaseURL
	if u, err := url.Parse(cfg.UIBaseURL); err == nil && utils.IsNotBlank(u.Hostname()) {
		rpID = u.Hostname()
	}
	waInstance, err := webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: mfaIssuer,
		RPOrigins:     []string{cfg.UIBaseURL},
	})
	if err != nil {
		slog.Error("failed to configure WebAuthn", "error", err)
		os.Exit(1)
	}

	providers := oidc.BuildProviders(cfg)

	programHandler := handlers.NewProgramHandler(programStore, clubStore, exerciseStore, oneRMStore,
		userStore, mailer, cfg.MaxUploadSize, cfg.PlateIncrement, cfg.UIBaseURL)

	api := router.New(router.Handlers{
		User: handlers.NewUserHandler(userStore, webauthnStore, cfg.JWTSecret, cfg.MaxImageSize,
			cfg.ActivationMode, mailer, cfg.APIBaseURL, cfg.UIBaseURL, cfg.ConfirmationEmailTTL),
		MFA: handlers.NewMFAHandler(userStore, webauthnStore, cfg.JWTSecret, cfg.ActivationMode, waInstance, mfaIssuer),
		OIDC: handlers.NewOIDCHandler(providers, userStore, webauthnStore, cfg.JWTSecret,
			cfg.APIBaseURL, cfg.UIBaseURL, cfg.OIDCKeycloakGroups, cfg.ActivationMode),
		Club:     handlers.NewClubHandler(clubStore, userStore, mailer, cfg.MaxImageSize, cfg.UIBaseURL),
		Exercise: handlers.NewExerciseHandler(exerciseStore, oneRMStore, userStore),
		Program:  programHandler,
		Training: handlers.NewTrainingHandler(programStore, clubStore, userStore, programHandler),
		Social:   handlers.NewSocialHandler(socialStore, clubStore, userStore, oneRMStore, cfg.MaxImageSize),
		Profile:  handlers.NewProfileHandler(userStore, socialStore, clubStore, oneRMStore),
		Admin:    handlers.NewAdminHandler(userStore, webauthnStore, socialStore, cfg.ActivationMode),
		Config: handlers.NewConfigHandler(oidc.Names(providers), cfg.ActivationMode,
			cfg.PlateIncrement, cfg.MaxImageSize, cfg.Version, contactClient.Configured()),
		Contact: handlers.NewContactHandler(contactClient),
	}, userStore, clubStore, router.Options{
		JWTSecret:          cfg.JWTSecret,
		ActivationMode:     cfg.ActivationMode,
		CorsEnabled:        cfg.CorsEnabled,
		CorsAllowedOrigins: cfg.CorsAllowedOrigins,
		ManifestPath:       cfg.ManifestPath,
	})

	slog.Info("strong-fish api listening", "port", cfg.Port, "version", cfg.Version,
		"activationMode", cfg.ActivationMode, "oidcProviders", oidc.Names(providers),
		"contactForm", contactClient.Configured())

	if err := http.ListenAndServe(":"+cfg.Port, api); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
