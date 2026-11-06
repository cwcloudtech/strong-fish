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
	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/email"
	"strong-fish-api/internal/garmin"
	"strong-fish-api/internal/googlehealth"
	"strong-fish-api/internal/myfitnesspal"
	"strong-fish-api/internal/handlers"
	"strong-fish-api/internal/metrics"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/oidc"
	"strong-fish-api/internal/router"
	"strong-fish-api/internal/stance"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/telemetry"
	"strong-fish-api/internal/utils"
	"strong-fish-api/internal/withings"
	"strong-fish-api/internal/yazio"
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
	storageStore := store.NewStorageStore(pool)
	messageStore := store.NewMessageStore(pool)
	deviceLinkStore := store.NewDeviceLinkStore(pool)
	aiStore := store.NewAIStore(pool)
	callbackStore := store.NewCallbackStore(pool)

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

	// The integrations that read what a member's watch counted. One today;
	// the list is what makes adding the next one a line here. Registered even
	// without credentials: the profile screen then shows it as switched off,
	// which is a better answer than an empty space.
	var devices []devicelink.Provider
	googleHealth := googlehealth.New(googlehealth.Config{
		// The same OAuth client that signs people in with Google. One client
		// can ask for different scopes per request, and a second set of
		// credentials for the same Cloud project would only be one more thing
		// to keep in step.
		ClientID:     cfg.OIDCGoogleClientID,
		ClientSecret: cfg.OIDCGoogleClientSecret,
		AuthURL:      cfg.GoogleHealthAuthURL,
		TokenURL:     cfg.GoogleHealthTokenURL,
		APIBaseURL:   cfg.GoogleHealthAPIURL,
	})
	devices = append(devices, googleHealth)
	// After Google Health on purpose: the order is the priority for steps and
	// heart rate (see devicelink.ActivitySource), and a member with both
	// connected is shown their watch's numbers rather than the scale app's.
	devices = append(devices, withings.New(withings.Config{
		ClientID:     cfg.WithingsClientID,
		ClientSecret: cfg.WithingsClientSecret,
		AuthURL:      cfg.WithingsAuthURL,
		APIBaseURL:   cfg.WithingsAPIURL,
	}))
	// Garmin Connect, third in the activity order: a member with Google Health
	// or Withings connected keeps those for steps and heart rate, and Garmin
	// fills in only when neither of the others is there.
	devices = append(devices, garmin.New(garmin.Config{
		TokenURL:   cfg.GarminTokenURL,
		APIBaseURL: cfg.GarminAPIURL,
	}))
	// Stance Fitness: barbell velocity and the RPE it computes from it. Last,
	// since it has no steps or heart rate - it never wins the activity source
	// - and like the others it is registered even unconfigured, so the screen
	// shows it switched off rather than hiding it.
	devices = append(devices, stance.NewProvider(stance.New(stance.Config{
		FirebaseAPIKey:     cfg.StanceFirebaseAPIKey,
		GoogleClientID:     cfg.StanceGoogleClientID,
		SupabaseProjectRef: cfg.StanceSupabaseProjectRef,
		SupabaseAnonKey:    cfg.StanceSupabaseAnonKey,
		SessionsRPC:        cfg.StanceSessionsRPC,
	})))
	// Nutrition sources: a member's food diary, read for the energy and protein
	// they logged. Each is connected per member with their own account - Yazio
	// with an email and password, MyFitnessPal with a personal token - so both
	// need no operator credentials and are always offered. Yazio first, which is
	// the priority when a member somehow connects both.
	devices = append(devices, yazio.New(yazio.Config{}))
	devices = append(devices, myfitnesspal.New(myfitnesspal.Config{}))

	strengthHandler := handlers.NewStrengthHandler(exerciseStore, userStore)
	profileHandler := handlers.NewProfileHandler(userStore, socialStore, clubStore, oneRMStore).
		WithStrength(strengthHandler)

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

	aiHandler := handlers.NewAIHandler(aiStore, userStore, cfg.UIBaseURL).
		WithMemberContext(oneRMStore, programStore, programHandler)
	// The two know each other: the device screens ask the model for a reading
	// of the last ninety days, and the chat is told what the last day looked
	// like. Wired here rather than in a constructor, because neither can be
	// built with the other already in hand.
	deviceHandler := handlers.NewDeviceHandler(devices, deviceLinkStore, userStore, cfg.JWTSecret,
		cfg.APIBaseURL, cfg.UIBaseURL, aiHandler).
		// So a coach or nutritionist of a shared club - and a public visitor when
		// the member opted in - can read an athlete's health data on their profile.
		WithRelations(profileHandler.RelationTo)
	aiHandler.WithHealth(deviceHandler.FitnessContext)

	api := router.New(router.Handlers{
		User: handlers.NewUserHandler(userStore, webauthnStore, cfg.JWTSecret, cfg.MaxImageSize,
			cfg.ActivationMode, mailer, cfg.APIBaseURL, cfg.UIBaseURL, cfg.ConfirmationEmailTTL),
		MFA: handlers.NewMFAHandler(userStore, webauthnStore, cfg.JWTSecret, cfg.ActivationMode, waInstance, mfaIssuer),
		OIDC: handlers.NewOIDCHandler(providers, userStore, webauthnStore, cfg.JWTSecret,
			cfg.APIBaseURL, cfg.UIBaseURL, cfg.OIDCKeycloakGroups, cfg.ActivationMode),
		Club:     handlers.NewClubHandler(clubStore, userStore, mailer, cfg.MaxImageSize, cfg.UIBaseURL),
		Exercise: handlers.NewExerciseHandler(exerciseStore, oneRMStore, userStore),
		Program:  programHandler,
		Training: handlers.NewTrainingHandler(programStore, clubStore, userStore, programHandler, aiStore, aiHandler),
		Social: handlers.NewSocialHandler(socialStore, clubStore, userStore, oneRMStore,
			messageStore, cfg.MaxImageSize),
		Profile: profileHandler,
		Admin: handlers.NewAdminHandler(userStore, webauthnStore, socialStore, mailer,
			cfg.ActivationMode, cfg.UIBaseURL),
		Config: handlers.NewConfigHandler(oidc.Names(providers), cfg.ActivationMode,
			cfg.PlateIncrement, cfg.MaxImageSize, cfg.MaxVideoSize, cfg.MaxAudioSize,
			cfg.Version, contactClient.Configured(),
			cfg.APIBaseURL, cfg.UIBaseURL, cfg.MobileURLPattern, cfg.AboutURL, cfg.DocURL),
		Contact: handlers.NewContactHandler(contactClient),
		ApiKey:  handlers.NewApiKeyHandler(apiKeyStore),
		// The JWT secret signs the playback links too: both say "this app
		// issued this, and it is still valid", and a deployment that has one
		// has the other.
		Media: handlers.NewMediaHandler(userStore, storageStore, profileHandler,
			cfg.MaxVideoSize, cfg.MaxAudioSize, cfg.APIBaseURL, cfg.JWTSecret),
		Storage:  handlers.NewStorageHandler(storageStore, userStore, cfg.MaxVideoSize),
		Event:    handlers.NewEventHandler(eventStore, clubStore, userStore, cfg.MaxUploadSize),
		Calendar: handlers.NewCalendarHandler(userStore, eventStore, clubStore, cfg.APIBaseURL),
		Search:   handlers.NewSearchHandler(userStore, clubStore, profileHandler),
		Strength: strengthHandler,
		AI:       aiHandler,
		Device:   deviceHandler,
		BarSpeed: handlers.NewBarSpeedHandler(store.NewBarSpeedStore(pool)).
			WithTracker(deviceHandler.BarSpeedSets),
		Callback:   handlers.NewCallbackHandler(callbackStore),
		Invitation: handlers.NewInvitationHandler(invitationStore, clubStore, userStore,
			mailer, cfg.UIBaseURL),
		Message: handlers.NewMessageHandler(messageStore, userStore, profileHandler, mailer, cfg.MaxImageSize, cfg.UIBaseURL),
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
