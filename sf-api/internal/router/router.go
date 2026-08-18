package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"strong-fish-api/internal/handlers"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/openapi"
	"strong-fish-api/internal/store"
)

// Handlers bundles every handler the router wires up, so adding one doesn't mean
// threading another positional argument through main.
type Handlers struct {
	User     *handlers.UserHandler
	MFA      *handlers.MFAHandler
	OIDC     *handlers.OIDCHandler
	Club     *handlers.ClubHandler
	Exercise *handlers.ExerciseHandler
	Program  *handlers.ProgramHandler
	Training *handlers.TrainingHandler
	Social   *handlers.SocialHandler
	Profile  *handlers.ProfileHandler
	Admin    *handlers.AdminHandler
	Config   *handlers.ConfigHandler
	Contact  *handlers.ContactHandler
	ApiKey   *handlers.ApiKeyHandler
	Media    *handlers.MediaHandler
	Event    *handlers.EventHandler
	Calendar *handlers.CalendarHandler
}

// Options carries the settings the middleware chain needs.
type Options struct {
	JWTSecret string
	// ApiKeys lets the auth middleware accept an X-Api-Key header as well as a
	// bearer token. Nil simply means no key can authenticate.
	ApiKeys            middleware.ApiKeyVerifier
	ActivationMode     string
	CorsEnabled        bool
	CorsAllowedOrigins []string
	ManifestPath       string
	// Version labels the generated OpenAPI document.
	Version string
}

func New(h Handlers, users *store.UserStore, clubs *store.ClubStore, o Options) http.Handler {
	r := chi.NewRouter()

	if o.CorsEnabled {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: o.CorsAllowedOrigins,
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Authorization", "Content-Type"},
		}))
	}

	// authenticated wraps a route group in "logged in and not disabled/banned".
	authenticated := func(fn func(chi.Router)) func(chi.Router) {
		return func(r chi.Router) {
			r.Use(middleware.Auth(o.JWTSecret, o.ApiKeys))
			r.Use(middleware.RequireActiveUser(users, o.ActivationMode))
			fn(r)
		}
	}

	r.Route("/v1", func(r chi.Router) {
		// --- public ---
		r.Get("/health", handlers.Health)
		r.Get("/manifest", handlers.NewManifestHandler(o.ManifestPath))
		r.Get("/config", h.Config.Get)
		r.Get("/assets/logo.png", handlers.AssetsLogo)

		// The contact form is deliberately public: someone who can't sign in is
		// exactly who most needs to reach us.
		r.Post("/contact", h.Contact.Create)

		// Where to get the Android build, and a QR code of that link for
		// someone reading this on a desktop. Public for the same reason the
		// contact form is: you need the app before you have an account.
		r.Get("/mobile-app", h.Config.MobileApp)

		// A program its coach chose to share. Unauthenticated by design - the
		// point is a link that works for anybody - and it is the store's
		// visibility predicate, not this route, that decides what may be read.
		r.Get("/public/programs/{programId}", h.Program.GetPublic)

		// The ICS feed Outlook and Google Calendar poll. Unauthenticated by
		// necessity - neither can send an Authorization header - so the token
		// in the path is the whole credential.
		r.Get("/calendar/{token}", h.Calendar.Feed)

		r.Route("/oidc", func(r chi.Router) {
			r.Get("/", h.OIDC.ListProviders)
			r.Get("/callback", h.OIDC.FrontendCallback)
			r.Get("/{provider}/login", h.OIDC.Login)
			r.Get("/{provider}/callback", h.OIDC.Callback)
		})

		r.Get("/user/confirmation", h.User.Confirm)

		r.Route("/users", func(r chi.Router) {
			r.Post("/", h.User.Register)
			r.Post("/login", h.User.Login)
			r.Post("/forgot-password", h.User.ForgotPassword)
			r.Post("/reset-password", h.User.ResetPassword)

			// Finishing a login gated by MFA: these carry their own short-lived
			// challenge/ceremony token instead of a session, so they sit outside
			// the authenticated group.
			r.Route("/login/mfa", func(r chi.Router) {
				r.Post("/totp", h.MFA.LoginTOTP)
				r.Post("/webauthn/begin", h.MFA.LoginWebAuthnBegin)
				r.Post("/webauthn/finish", h.MFA.LoginWebAuthnFinish)
			})

			// /me needs a session but must stay reachable by a disabled account,
			// so it can be told why it's blocked.
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(o.JWTSecret, o.ApiKeys))
				r.Get("/me", h.User.Me)
			})

			r.Group(authenticated(func(r chi.Router) {
				r.Put("/me", h.User.UpdateProfile)
				r.Put("/me/picture", h.User.UpdatePicture)
				r.Get("/search", h.User.Search)

				// A member's own keys, and the CLI/mobile config built from
				// one. The config endpoints POST the token they format
				// because a custom request header would need a CORS
				// exception on reverse proxies that a JSON body doesn't.
				r.Route("/me/api-keys", func(r chi.Router) {
					r.Get("/", h.ApiKey.List)
					r.Post("/", h.ApiKey.Create)
					r.Delete("/{keyId}", h.ApiKey.Delete)
				})

				// The member's own object store for video uploads. It holds
				// live credentials, so it has its own endpoint rather than
				// riding along on the profile.
				r.Route("/me/storage", func(r chi.Router) {
					r.Get("/", h.Media.StorageGet)
					r.Put("/", h.Media.StorageSet)
					r.Delete("/", h.Media.StorageDelete)
				})

				r.Route("/me/calendar-feed", func(r chi.Router) {
					r.Get("/", h.Calendar.Status)
					r.Post("/enable", h.Calendar.Enable)
					r.Post("/disable", h.Calendar.Disable)
					r.Post("/regenerate", h.Calendar.Regenerate)
				})

				r.Route("/me/config", func(r chi.Router) {
					r.Post("/file", h.Config.ClientConfigFile)
					r.Post("/qr", h.Config.ClientConfigQR)
				})

				r.Route("/me/mfa", func(r chi.Router) {
					r.Get("/", h.MFA.Status)
					r.Post("/totp/setup", h.MFA.TOTPSetup)
					r.Post("/totp/confirm", h.MFA.TOTPConfirm)
					r.Delete("/totp", h.MFA.TOTPDisable)
					r.Post("/webauthn/begin", h.MFA.WebAuthnRegisterBegin)
					r.Post("/webauthn/finish", h.MFA.WebAuthnRegisterFinish)
					r.Delete("/webauthn/{credentialId}", h.MFA.WebAuthnDelete)
				})
			}))
		})

		// Public profiles are readable logged out when their owner opted in, so
		// a shared link works; OptionalAuth still identifies a logged-in caller
		// so their own follow state and club-only posts come through.
		r.Group(func(r chi.Router) {
			r.Use(middleware.OptionalAuth(o.JWTSecret, o.ApiKeys))
			r.Get("/profiles/{handle}", h.Profile.Get)
			r.Get("/profiles/{handle}/posts", h.Profile.Posts)
			r.Get("/profiles/{handle}/follows", h.Social.ListFollows)

			// The calendar is readable logged out: a meet anybody can enter is
			// exactly what is worth finding before you have an account. What
			// comes back still depends on the caller - OptionalAuth is what
			// adds their clubs' own dates to the public ones.
			r.Get("/events", h.Event.List)
			r.Get("/events/{eventId}", h.Event.Get)
		})

		// --- authenticated ---
		r.Group(authenticated(func(r chi.Router) {
			r.Post("/profiles/{handle}/follow", h.Social.Follow)
			r.Delete("/profiles/{handle}/follow", h.Social.Unfollow)

			// The exercise catalog is shared across every club: any member can
			// read it (it names the movements in their program), and any coach
			// can extend it - a program can't be written without naming the
			// movements it prescribes.
			//
			// Editing and deleting are the superadmin's, though: an entry is
			// shared by every club, so a rename ripples through everyone's
			// programs and a delete cascades into their sets.
			r.Route("/exercises", func(r chi.Router) {
				r.Get("/", h.Exercise.List)
				r.With(middleware.RequireCoach(users)).Post("/", h.Exercise.Create)

				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireSuperadmin(users))
					r.Put("/{exerciseId}", h.Exercise.Update)
					r.Get("/{exerciseId}/usage", h.Exercise.Usage)
					r.Delete("/{exerciseId}", h.Exercise.Delete)
				})
			})

			// A member's own 1RMs. Updating one is what recomputes every set of
			// every program they're running.
			r.Route("/one-rms", func(r chi.Router) {
				r.Get("/", h.Exercise.ListOneRMs)
				r.Put("/{exerciseId}", h.Exercise.SetOneRM)
				r.Delete("/{exerciseId}", h.Exercise.DeleteOneRM)
			})

			r.Route("/clubs", func(r chi.Router) {
				r.Get("/", h.Club.List)

				r.With(middleware.RequireCoach(users)).Post("/", h.Club.Create)

				r.Route("/{clubId}", func(r chi.Router) {
					r.Use(middleware.ClubMembership(clubs, users))

					// manager restricts one endpoint to the club's owner and
					// admins. It's applied per-route rather than by wrapping a
					// whole group, because a group and a plain route can't both
					// claim the same path prefix - mounting a "/programs"
					// subrouter alongside a "/programs" handler makes the
					// handler unreachable.
					manager := func(r chi.Router) chi.Router { return r.With(middleware.RequireClubManager) }

					r.Get("/", h.Club.Get)
					manager(r).Put("/", h.Club.Update)
					manager(r).Delete("/", h.Club.Delete)
					manager(r).Post("/transfer", h.Club.TransferOwnership)
					manager(r).Get("/feedback", h.Program.ListFeedback)
					r.Get("/feed", h.Social.ClubFeed)

					r.Route("/members", func(r chi.Router) {
						r.Get("/", h.Club.ListMembers)
						r.Delete("/me", h.Club.Leave)
						manager(r).Post("/", h.Club.AddMember)
						manager(r).Put("/{userId}", h.Club.SetMemberRole)
						manager(r).Delete("/{userId}", h.Club.RemoveMember)
					})

					r.Route("/programs", func(r chi.Router) {
						r.Get("/", h.Program.List)
						// A program is either imported from a spreadsheet or
						// built session by session; both end up in the same
						// shape.
						manager(r).Post("/", h.Program.Create)
						manager(r).Post("/import", h.Program.Import)

						r.Route("/{programId}", func(r chi.Router) {
							r.Get("/", h.Program.Get)
							manager(r).Put("/", h.Program.Update)
							manager(r).Delete("/", h.Program.Delete)
							manager(r).Post("/days", h.Program.AddDay)
							manager(r).Put("/days/{dayId}", h.Program.UpdateDay)
							manager(r).Delete("/days/{dayId}", h.Program.DeleteDay)
							manager(r).Post("/days/{dayId}/sets", h.Program.AddSet)
							manager(r).Put("/sets/{setId}", h.Program.UpdateSet)
							manager(r).Delete("/sets/{setId}", h.Program.DeleteSet)
							manager(r).Get("/assignments", h.Program.ListAssignments)
							manager(r).Post("/assignments", h.Program.Assign)
							manager(r).Delete("/assignments/{assignmentId}", h.Program.Unassign)
						})
					})
				})
			})

			// The member's own training: the programs assigned to them, and the
			// feedback they leave on each set.
			r.Route("/training", func(r chi.Router) {
				r.Get("/", h.Training.ListAssignments)
				r.Route("/{assignmentId}", func(r chi.Router) {
					r.Get("/", h.Training.Get)
					r.Put("/status", h.Training.SetStatus)
					r.Put("/sets/{setId}/log", h.Training.LogSet)
					r.Delete("/sets/{setId}/log", h.Training.DeleteLog)
				})
			})

			// The social feed.
			r.Route("/posts", func(r chi.Router) {
				r.Get("/", h.Social.Feed)
				r.Get("/discover", h.Social.Discover)
				r.Post("/", h.Social.CreatePost)

				r.Route("/{postId}", func(r chi.Router) {
					r.Get("/", h.Social.GetPost)
					r.Put("/", h.Social.UpdatePost)
					r.Delete("/", h.Social.DeletePost)
					r.Post("/like", h.Social.Like)
					r.Delete("/like", h.Social.Unlike)

					r.Route("/comments", func(r chi.Router) {
						r.Get("/", h.Social.ListComments)
						r.Post("/", h.Social.CreateComment)
						r.Put("/{commentId}", h.Social.UpdateComment)
						r.Delete("/{commentId}", h.Social.DeleteComment)
					})
				})
			})

			// Uploading a video is a write, so it needs a session even though
			// reading the calendar doesn't.
			r.Post("/media/videos", h.Media.UploadVideo)

			r.Route("/events", func(r chi.Router) {
				r.Post("/", h.Event.Create)
				r.Put("/{eventId}", h.Event.Update)
				r.Delete("/{eventId}", h.Event.Delete)
			})

			r.Post("/reports", h.Social.Report)

			// --- superadmin ---
			r.Route("/admin", func(r chi.Router) {
				r.Use(middleware.RequireSuperadmin(users))

				r.Get("/stats", h.Admin.Stats)
				r.Get("/clubs", h.Club.ListAll)
				r.Get("/reports", h.Social.ListReports)
				r.Put("/reports/{reportId}", h.Social.ResolveReport)

				r.Route("/users", func(r chi.Router) {
					r.Get("/", h.Admin.ListUsers)
					r.Put("/{userId}", h.Admin.UpdateUser)
					r.Delete("/{userId}", h.Admin.DeleteUser)
					r.Delete("/{userId}/mfa", h.Admin.ClearMFA)
				})
			})
		}))
	})

	// Generated once every /v1 route above is registered, so the document can
	// never describe a router that isn't the one running.
	r.Get("/openapi.json", handlers.NewOpenAPIHandler(openapi.Generate(r, "strong-fish API", o.Version)))
	r.Get("/", handlers.ServeSwaggerUI)

	return r
}
