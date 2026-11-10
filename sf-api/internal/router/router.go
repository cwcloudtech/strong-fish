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
	User       *handlers.UserHandler
	MFA        *handlers.MFAHandler
	OIDC       *handlers.OIDCHandler
	Club       *handlers.ClubHandler
	Exercise   *handlers.ExerciseHandler
	Program    *handlers.ProgramHandler
	Training   *handlers.TrainingHandler
	Social     *handlers.SocialHandler
	Profile    *handlers.ProfileHandler
	Admin      *handlers.AdminHandler
	Config     *handlers.ConfigHandler
	Contact    *handlers.ContactHandler
	ApiKey     *handlers.ApiKeyHandler
	Media      *handlers.MediaHandler
	Storage    *handlers.StorageHandler
	Event      *handlers.EventHandler
	Calendar   *handlers.CalendarHandler
	Search     *handlers.SearchHandler
	Invitation *handlers.InvitationHandler
	Message    *handlers.MessageHandler
	Strength   *handlers.StrengthHandler
	AI         *handlers.AIHandler
	Device     *handlers.DeviceHandler
	BarSpeed   *handlers.BarSpeedHandler
	Callback   *handlers.CallbackHandler
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
	// Observability. Instrument wraps every route so spans, the access log and
	// the metrics all agree on which endpoint a request hit; MetricsHandler is
	// what /v1/metrics serves.
	Instrument     func(http.Handler) http.Handler
	MetricsHandler http.Handler
}

func New(h Handlers, users *store.UserStore, clubs *store.ClubStore, o Options) http.Handler {
	r := chi.NewRouter()

	// Outermost, so a request rejected by CORS or by auth is still counted and
	// still traced - the requests that never reach a handler are exactly the
	// ones worth seeing on a dashboard.
	if o.Instrument != nil {
		r.Use(o.Instrument)
	}

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

		// Prometheus scrapes this. It is unauthenticated because a scraper
		// cannot hold a session, and it is reachable only from wherever the
		// deployment exposes the API - the same assumption every /metrics
		// endpoint makes.
		// GET only, not r.Handle: a scrape is a GET, and registering every
		// method would put eight verbs on a read-only endpoint.
		if o.MetricsHandler != nil {
			r.Method(http.MethodGet, "/metrics", o.MetricsHandler)
		}

		// A program its coach chose to share. Unauthenticated by design - the
		// point is a link that works for anybody - and it is the store's
		// visibility predicate, not this route, that decides what may be read.
		// The strength calculator. Open on purpose: somebody working out what
		// their meet total is worth has no reason to have an account first,
		// and it is the page most likely to give them one. What it can see of
		// other members is one number - the spread of DOTS scores behind the
		// percentile bar - and never who holds them.
		r.Post("/strength/score", h.Strength.Score)
		r.With(middleware.OptionalAuth(o.JWTSecret, o.ApiKeys)).
			Get("/strength/defaults", h.Strength.Defaults)

		// A conversation its owner shared, read by whoever holds the link -
		// unauthenticated for the same reason a shared program is: a link
		// that only works for members is not a shared link. The token is the
		// whole credential, so it is 32 random bytes, and revoking it takes
		// effect at once.
		r.Get("/public/ai/conversations/{token}", h.AI.GetSharedConversation)
		r.Get("/public/ai/conversations/{token}/export.pdf", h.AI.ExportSharedConversation)
		r.Get("/public/ai/conversations/{token}/export.png", h.AI.ExportSharedConversation)

		r.Get("/public/programs/{programId}", h.Program.GetPublic)
		// The same sheet the member's own export produces, without the weights:
		// a reader outside the club has no maxes to resolve them against.
		// Both formats, chosen by the extension.
		r.Get("/public/programs/{programId}/export.pdf", h.Program.ExportPublicPDF)
		r.Get("/public/programs/{programId}/export.xlsx", h.Program.ExportPublicPDF)
		r.Get("/public/programs/{programId}/export.png", h.Program.ExportPublicPDF)

		// A private bucket's object, served by this API with the owner's own
		// credentials.
		//
		// Outside the authenticated group on purpose: a <video> tag cannot
		// send an Authorization header, so a player carries a signed link
		// instead and the handler resolves the reader from it. OptionalAuth is
		// what still identifies a caller who *can* send one - a script, the
		// phone fetching a file itself - and a request with neither a
		// signature nor a session is refused by the handler. "Unauthenticated
		// route" is about which middleware runs, not about who may read.
		r.With(middleware.OptionalAuth(o.JWTSecret, o.ApiKeys)).
			Get("/media/{storageId}/{object}", h.Media.ServeMedia)

		// A post its author published to everybody. Unauthenticated by design:
		// a link shared to Instagram is opened by people with no account.
		r.Get("/public/posts/{postId}", h.Social.GetPublicPost)

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

		// Where a provider sends the browser back after a member consents.
		// Unauthenticated by necessity - it is the provider's redirect, with
		// no Authorization header - so who it belongs to comes from the
		// signed state instead.
		r.Get("/devices/{provider}/callback", h.Device.Callback)

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

			// An athlete's fitness data read from their profile, keyed by
			// {userId} - the same reads as /me/devices. OptionalAuth, not a
			// session requirement: a coach or nutritionist of a shared club
			// reads it signed in, and anyone reads it when the member made their
			// fitness data public, even logged out, which is what a public
			// profile link needs. The handler resolves {userId} and authorizes,
			// answering 404 when the viewer may not see it. Reads only -
			// generating the AI reading needs a session and lives below.
			r.Group(func(r chi.Router) {
				r.Use(middleware.OptionalAuth(o.JWTSecret, o.ApiKeys))
				r.Route("/{userId}/devices", func(r chi.Router) {
					r.Get("/", h.Device.List)
					r.Get("/export.png", h.Device.Export)
					r.Get("/export.pdf", h.Device.Export)
					r.Get("/nutrition-targets", h.Device.NutritionTargets)
					r.Get("/{provider}/steps", h.Device.Steps)
					r.Get("/{provider}/heart-rate", h.Device.HeartRate)
					r.Get("/{provider}/body-composition", h.Device.BodyComposition)
					r.Get("/{provider}/nutrition", h.Device.Nutrition)
					r.Get("/{provider}/feedback", h.Device.HealthFeedback)
				})
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

				// The outside accounts that count a member's steps for them.
				r.Route("/me/devices", func(r chi.Router) {
					r.Get("/", h.Device.List)
					// One export of everything the member has connected -
					// steps, heart rate, body composition, bar speed and the
					// AI reading - not one per device.
					r.Get("/export.png", h.Device.Export)
					r.Get("/export.pdf", h.Device.Export)
					// The member's ideal protein band and maintenance energy,
					// computed from their body - not tied to any one provider,
					// so a member who tracks no food still sees a target.
					r.Get("/nutrition-targets", h.Device.NutritionTargets)
					r.Get("/{provider}/authorize", h.Device.Authorize)
					r.Delete("/{provider}", h.Device.Disconnect)
					r.Get("/{provider}/steps", h.Device.Steps)
					r.Get("/{provider}/heart-rate", h.Device.HeartRate)
					r.Get("/{provider}/body-composition", h.Device.BodyComposition)
					// Logged energy and protein from a food diary (Yazio,
					// MyFitnessPal), with the member's daily targets.
					r.Get("/{provider}/nutrition", h.Device.Nutrition)
					// What the AI made of the last ninety days. Generated on
					// demand, kept, and askable again once a day - the
					// numbers underneath move that often at most.
					r.Get("/{provider}/feedback", h.Device.HealthFeedback)
					r.Post("/{provider}/feedback", h.Device.GenerateHealthFeedback)
					r.Delete("/{provider}/feedback", h.Device.DeleteHealthFeedback)
					// Stance and anything else paired with an account rather
					// than an OAuth redirect: pair with a password, read the
					// last set of each lift.
					r.Post("/{provider}/pair", h.Device.PairDevice)
					r.Post("/{provider}/google", h.Device.PairDeviceGoogle)
					r.Post("/{provider}/pair-key", h.Device.PairDeviceKey)
					r.Get("/{provider}/bar-speed", h.Device.BarSpeed)
				})

				// Generating the AI reading of another member's fitness data: a
				// session is required so the reviewer's own subscription pays,
				// which is why this is here and not among the OptionalAuth reads.
				// The handler authorizes the {userId} as a coach or nutritionist
				// of a shared club (or the member themselves).
				r.Post("/{userId}/devices/{provider}/feedback", h.Device.GenerateHealthFeedback)

				// Bar speed captured by StrongFish itself: any accelerometer -
				// a phone over Bluetooth, a board over HTTP - posts the raw
				// stream and the movement, and gets the computed set back. The
				// list consolidates these with what a connected tracker reports,
				// keeping the most recent set of each lift.
				r.Get("/me/bar-speed", h.BarSpeed.List)
				r.Post("/me/bar-speed/analyze", h.BarSpeed.Analyze)
				// The reusable velocity-to-effort calculator: a movement and one
				// rep's velocity in, the estimated RPE/RIR out. Stores nothing, so
				// a client can read an effort estimate for any measured set.
				r.Post("/me/bar-speed/effort", h.BarSpeed.Effort)

				// The invitations waiting for this account's address. They are
				// matched by email, not by user id, so one that arrived before
				// the account existed is here too.
				r.Route("/me/invitations", func(r chi.Router) {
					r.Get("/", h.Invitation.Mine)
					r.Post("/{invitationId}/accept", h.Invitation.Accept)
					r.Post("/{invitationId}/decline", h.Invitation.Decline)
				})

				// The member's own object store for video uploads. It holds
				// live credentials, so it has its own endpoint rather than
				// riding along on the profile.
				// The member's own storage targets, in priority order: an
				// upload is written to every one of them, and the link in the
				// post comes from the first.
				r.Route("/me/storages", func(r chi.Router) {
					r.Get("/", h.Storage.List)
					r.Post("/", h.Storage.Create)
					// Which one is first is a real setting - it decides where
					// a posted link points - so it has its own endpoint rather
					// than riding on an edit.
					r.Put("/order", h.Storage.Reorder)

					r.Route("/{storageId}", func(r chi.Router) {
						r.Put("/", h.Storage.Update)
						r.Delete("/", h.Storage.Delete)
						// Who else may use it. Reading and writing the list is
						// the owner's alone - a writer handing out further
						// access would widen a bucket somebody else is paying
						// for.
						r.Get("/shares", h.Storage.ListGrants)
						r.Post("/shares", h.Storage.Share)
						r.Delete("/shares/{userId}", h.Storage.Unshare)
					})
				})

				// The member's own connections to a language model, shared
				// the way a storage is: an owner holds the credential and
				// decides who else may spend it.
				r.Route("/me/ai-connections", func(r chi.Router) {
					r.Get("/", h.AI.List)
					r.Post("/", h.AI.Create)
					// Which one is first is a real setting - it decides
					// which subscription answers when nobody picks - so it
					// has its own endpoint rather than riding on an edit.
					r.Put("/order", h.AI.Reorder)

					r.Route("/{connectionId}", func(r chi.Router) {
						r.Put("/", h.AI.Update)
						r.Delete("/", h.AI.Delete)
						// Reading and writing the list is the owner's alone -
						// anybody else handing out access would be spending a
						// key they do not pay for.
						r.Get("/shares", h.AI.ListGrants)
						r.Post("/shares", h.AI.Share)
						r.Delete("/shares/{userId}", h.AI.Unshare)
					})
				})

				// The member's own recording callbacks: triggers the phone
				// fires when they start a bar-speed set, so a device of their
				// own (an ESP32) records it too - a webhook (HTTP), a WebSocket
				// message or an MQTT publish. Not shared, so no ACL - a callback
				// points at the member's own hardware and is theirs alone.
				r.Route("/me/callbacks", func(r chi.Router) {
					r.Get("/", h.Callback.List)
					r.Post("/", h.Callback.Create)

					r.Route("/{callbackId}", func(r chi.Router) {
						r.Put("/", h.Callback.Update)
						r.Delete("/", h.Callback.Delete)
					})
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

			// Searching is readable logged out, but only ever returns public
			// profiles then - the visibility predicate is the same one the
			// profile endpoint applies, evaluated per row inside the query.
			r.Get("/search/members", h.Search.Members)
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

					// Inviting is not adding: a coach entering their own
					// athletes adds them, and reaching out to somebody who
					// has to agree - or who has no account yet - invites
					// them.
					r.Route("/invitations", func(r chi.Router) {
						manager(r).Get("/", h.Invitation.ListForClub)
						manager(r).Post("/", h.Invitation.Create)
						manager(r).Delete("/{invitationId}", h.Invitation.Withdraw)
					})

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
							manager(r).Put("/days/{dayId}/move", h.Program.MoveDay)
							manager(r).Post("/days/{dayId}/duplicate", h.Program.DuplicateDay)
							manager(r).Delete("/days/{dayId}", h.Program.DeleteDay)
							manager(r).Post("/days/{dayId}/sets", h.Program.AddSet)
							manager(r).Put("/sets/{setId}", h.Program.UpdateSet)
							manager(r).Put("/sets/{setId}/move", h.Program.MoveSet)
							manager(r).Post("/sets/{setId}/duplicate", h.Program.DuplicateSet)
							manager(r).Delete("/sets/{setId}", h.Program.DeleteSet)
							manager(r).Get("/assignments", h.Program.ListAssignments)
							manager(r).Post("/assignments", h.Program.Assign)
							manager(r).Delete("/assignments/{assignmentId}", h.Program.Unassign)
							// Readable by anybody who may read the program:
							// an athlete printing the block they were given
							// is not a coaching action.
							r.Get("/export.pdf", h.Program.ExportPDF)
							r.Get("/export.xlsx", h.Program.ExportPDF)
							r.Get("/export.png", h.Program.ExportPDF)
							// Not manager-gated: any member of this club may
							// copy a program they can read into their own
							// library. Copying it into *another club* is a
							// coaching action, and the handler checks the
							// destination for that.
							r.Post("/copy", h.Program.CopyToClub)
						})
					})
				})
			})

			// A member's own programs: the block an athlete writes for
			// themselves, which belongs to no club.
			//
			// The same handlers as the club routes above, deliberately: the
			// only difference is who is allowed in, which authorizeProgram
			// decides from whether the path names a club. Duplicating the
			// twelve editing endpoints for a second set of rules is how the
			// two would come to disagree.
			r.Route("/programs", func(r chi.Router) {
				r.Get("/", h.Program.ListMine)
				r.Post("/", h.Program.Create)
				r.Post("/import", h.Program.Import)

				r.Route("/{programId}", func(r chi.Router) {
					r.Get("/", h.Program.Get)
					r.Put("/", h.Program.Update)
					r.Delete("/", h.Program.Delete)
					r.Get("/export.pdf", h.Program.ExportPDF)
					r.Get("/export.xlsx", h.Program.ExportPDF)
					r.Get("/export.png", h.Program.ExportPDF)
					r.Post("/copy", h.Program.CopyToClub)
					r.Post("/days", h.Program.AddDay)
					r.Put("/days/{dayId}", h.Program.UpdateDay)
					r.Put("/days/{dayId}/move", h.Program.MoveDay)
					r.Post("/days/{dayId}/duplicate", h.Program.DuplicateDay)
					r.Delete("/days/{dayId}", h.Program.DeleteDay)
					r.Post("/days/{dayId}/sets", h.Program.AddSet)
					r.Put("/sets/{setId}", h.Program.UpdateSet)
					r.Put("/sets/{setId}/move", h.Program.MoveSet)
					r.Post("/sets/{setId}/duplicate", h.Program.DuplicateSet)
					r.Delete("/sets/{setId}", h.Program.DeleteSet)
					r.Get("/assignments", h.Program.ListAssignments)
					r.Post("/assignments", h.Program.Assign)
					r.Delete("/assignments/{assignmentId}", h.Program.Unassign)
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
					// A sensor recording written onto a set: only the vbt subtree,
					// never the member's own load or rep count.
					r.Put("/sets/{setId}/vbt", h.Training.LogSetVBT)
					r.Delete("/sets/{setId}/log", h.Training.DeleteLog)
					// A whole session ticked off in one go.
					r.Put("/days/{dayId}/log", h.Training.SetDayDone)
					// What the model said about a finished session. Kept, so
					// it is read rather than regenerated; deleting it is how
					// a member asks for another.
					r.Get("/days/{dayId}/feedback", h.Training.SessionFeedback)
					r.Post("/days/{dayId}/feedback", h.Training.GenerateSessionFeedback)
					// One reading, by its own id: a session holds as many as
					// were asked for, so "the" feedback of a day is not a
					// thing to address.
					r.Delete("/days/{dayId}/feedback/{feedbackId}", h.Training.DeleteSessionFeedback)
					// The block with the member's feedback on it, whole or one
					// week (?week=N), for them to send their coach - and for
					// their coach to read away from the app.
					r.Get("/export.pdf", h.Training.ExportAssignment)
					r.Get("/export.xlsx", h.Training.ExportAssignment)
					r.Get("/export.png", h.Training.ExportAssignment)
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

			// Private messages. A thread is addressed by who is in it, not by
			// an id: there is exactly one per pair, so making the client look
			// one up first would only add a round trip.
			r.Get("/messages", h.Message.List)
			r.Get("/messages/unread", h.Message.Unread)
			r.Get("/messages/with/{userId}", h.Message.Thread)
			r.Post("/messages/with/{userId}", h.Message.Send)
			// Addressed by message id, not by thread: a delete names the one
			// message, and the handler checks who may remove it.
			r.Delete("/messages/{messageId}", h.Message.Delete)

			// The block list. Blocking does not require the target to be
			// visible - needing to block somebody you can no longer see is
			// exactly the case it exists for.
			r.Get("/blocks", h.Message.ListBlocks)
			r.Post("/blocks/{userId}", h.Message.Block)
			r.Delete("/blocks/{userId}", h.Message.Unblock)

			// Uploading a video is a write, so it needs a session even though
			// reading the calendar doesn't.
			r.Post("/media/videos", h.Media.UploadVideo)
			r.Post("/media/audio", h.Media.UploadAudio)

			// A playback link for an object in a private bucket: checked here,
			// where there is a session to check, and good for a few hours.
			r.Get("/media/{storageId}/{object}/link", h.Media.SignMedia)

			// Every storage the caller may upload to: their own, and the ones
			// shared with them.
			r.Get("/storages", h.Storage.ListUsable)

			// Asking the model, and the threads of what was asked.
			r.Post("/ai/chat", h.AI.Chat)
			r.Get("/ai/providers", h.AI.Providers)
			r.Route("/ai/conversations", func(r chi.Router) {
				r.Get("/", h.AI.ListConversations)
				// Bulk delete as a POST: a body on a DELETE is permitted and
				// widely dropped, and a clear-history that silently clears
				// nothing is worse than an unfashionable verb. Registered
				// before the id route so "delete" is not read as one.
				r.Post("/delete", h.AI.DeleteConversations)
				r.Route("/{conversationId}", func(r chi.Router) {
					r.Get("/", h.AI.GetConversation)
					r.Put("/", h.AI.RenameConversation)
					r.Delete("/", h.AI.DeleteConversation)
					// The link to hand somebody, minted and revoked.
					r.Post("/share", h.AI.ShareConversation)
					r.Delete("/share", h.AI.UnshareConversation)
					r.Get("/export.pdf", h.AI.ExportConversation)
					r.Get("/export.png", h.AI.ExportConversation)
				})
			})

			// Registered as leaves, not as an r.Route subrouter: the calendar
			// is readable logged out, so "/events" already carries a GET in
			// the public group above. Mounting a subrouter on that same path
			// replaces the leaf with the mount - chi does not report this, and
			// the GET then answers 401/405 from inside the subrouter instead
			// of reaching its handler.
			r.Post("/events", h.Event.Create)
			r.Post("/events/import", h.Event.ImportCalendar)
			r.Put("/events/{eventId}", h.Event.Update)
			r.Delete("/events/{eventId}", h.Event.Delete)

			r.Post("/reports", h.Social.Report)

			// --- superadmin ---
			r.Route("/admin", func(r chi.Router) {
				r.Use(middleware.RequireSuperadmin(users))

				r.Get("/stats", h.Admin.Stats)
				r.Get("/clubs", h.Club.ListAll)
				r.Get("/reports", h.Social.ListReports)
				r.Get("/coach-requests", h.Admin.ListCoachRequests)
				r.Put("/coach-requests/{userId}", h.Admin.DecideCoachRequest)
				r.Put("/reports/{reportId}", h.Social.ResolveReport)

				r.Route("/users", func(r chi.Router) {
					r.Get("/", h.Admin.ListUsers)
					r.Put("/{userId}", h.Admin.UpdateUser)
					r.Delete("/{userId}", h.Admin.DeleteUser)
					r.Delete("/{userId}/mfa", h.Admin.ClearMFA)
					r.Get("/{userId}/ips", h.Admin.UserIPs)
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
