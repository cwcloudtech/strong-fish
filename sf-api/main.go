package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"strong-fish-api/internal/config"
	"strong-fish-api/internal/contact"
	"strong-fish-api/internal/db"
	"strong-fish-api/internal/email"
	"strong-fish-api/internal/handlers"
	"strong-fish-api/internal/metrics"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/oidc"
	"strong-fish-api/internal/router"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/telemetry"
	"strong-fish-api/internal/utils"
)

// shutdownTimeout bounds how long the exporters get to flush what they have
// buffered. A process that will not exit is worse than a few lost spans.
const shutdownTimeout = 5 * time.Second

// shutdown flushes one exporter on the way out, logging rather than failing:
// by this point the process is leaving anyway.
func shutdown(fn func(context.Context) error, what string) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := fn(ctx); err != nil {
		slog.Error("failed to flush on shutdown", "component", what, "error", err)
	}
}

// mfaIssuer is the "issuer" shown by authenticator apps (Google Authenticator
// and friends) next to an enrolled TOTP entry.
const mfaIssuer = "strong-fish"

func main() {
	cfg := config.Load()
	runtime.GOMAXPROCS(cfg.MaxWorkers)

	ctx := context.Background()

	// Before anything else that logs: Setup installs the process-wide slog
	// default, so every slog call below - including the ones reporting a
	// failure to start - goes through the same sinks.
	tel, err := telemetry.Setup(ctx, telemetry.Config{
		Endpoint: cfg.OTELEndpoint, Proto: cfg.OTELProto, Version: cfg.Version,
	})
	if err != nil {
		slog.Error("failed to configure telemetry", "error", err)
		os.Exit(1)
	}
	defer shutdown(tel.Shutdown, "telemetry")

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
	apiKeyStore := store.NewApiKeyStore(pool)
	eventStore := store.NewEventStore(pool)
	invitationStore := store.NewInvitationStore(pool)
	messageStore := store.NewMessageStore(pool)

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

	profileHandler := handlers.NewProfileHandler(userStore, socialStore, clubStore, oneRMStore)

	programHandler := handlers.NewProgramHandler(programStore, clubStore, exerciseStore, oneRMStore,
		userStore, mailer, cfg.MaxUploadSize, cfg.PlateIncrement, cfg.UIBaseURL)

	observability, err := metrics.Setup(ctx, metrics.Config{
		Endpoint: cfg.OTELEndpoint, Proto: cfg.OTELProto, Version: cfg.Version,
	}, userStore, clubStore, programStore)
	if err != nil {
		slog.Error("failed to configure metrics", "error", err)
		os.Exit(1)
	}
	defer shutdown(observability.Shutdown, "metrics")

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
		Social: handlers.NewSocialHandler(socialStore, clubStore, userStore, oneRMStore,
			messageStore, cfg.MaxImageSize),
		Profile: profileHandler,
		Admin: handlers.NewAdminHandler(userStore, webauthnStore, socialStore, mailer,
			cfg.ActivationMode, cfg.UIBaseURL),
		Config: handlers.NewConfigHandler(oidc.Names(providers), cfg.ActivationMode,
			cfg.PlateIncrement, cfg.MaxImageSize, cfg.Version, contactClient.Configured(),
			cfg.APIBaseURL, cfg.UIBaseURL, cfg.MobileURLPattern, cfg.AboutURL, cfg.DocURL),
		Contact:  handlers.NewContactHandler(contactClient),
		ApiKey:   handlers.NewApiKeyHandler(apiKeyStore),
		Media:    handlers.NewMediaHandler(userStore, cfg.MaxVideoSize, cfg.MaxAudioSize),
		Event:    handlers.NewEventHandler(eventStore, clubStore, userStore),
		Calendar: handlers.NewCalendarHandler(userStore, eventStore, clubStore, cfg.APIBaseURL),
		Search:   handlers.NewSearchHandler(userStore, clubStore, profileHandler),
		Invitation: handlers.NewInvitationHandler(invitationStore, clubStore, userStore,
			mailer, cfg.UIBaseURL),
		Message: handlers.NewMessageHandler(messageStore, userStore, profileHandler, cfg.MaxImageSize),
	}, userStore, clubStore, router.Options{
		JWTSecret:          cfg.JWTSecret,
		ApiKeys:            apiKeyStore,
		ActivationMode:     cfg.ActivationMode,
		CorsEnabled:        cfg.CorsEnabled,
		CorsAllowedOrigins: cfg.CorsAllowedOrigins,
		ManifestPath:       cfg.ManifestPath,
		Version:            cfg.Version,
		Instrument:         middleware.Instrument(tel.Tracer, tel.Logger, observability.Observe),
		MetricsHandler:     observability.Handler,
	})

	slog.Info("strong-fish api listening", "port", cfg.Port, "version", cfg.Version,
		"activationMode", cfg.ActivationMode, "oidcProviders", oidc.Names(providers),
		"contactForm", contactClient.Configured(), "otelEndpoint", cfg.OTELEndpoint)

	if err := http.ListenAndServe(":"+cfg.Port, api); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
