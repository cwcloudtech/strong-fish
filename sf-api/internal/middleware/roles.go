package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
)

// RequireActiveUser blocks disabled and banned accounts from every
// authenticated action beyond logging in and reading their own status. The role
// is re-read from the database on every request since it can change after the
// token was issued (a superadmin confirms, disables or bans the account later).
func RequireActiveUser(users *store.UserStore, activationMode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := UserIDFromContext(r.Context())
			user, err := users.FindByID(r.Context(), userID)
			if err != nil {
				jsonError(w, http.StatusUnauthorized, "Not authorised")
				return
			}
			switch user.Role {
			case models.GlobalRoleBan:
				jsonErrorCode(w, http.StatusForbidden, "Your account has been banned by an administrator.", models.I18nAccountBanned)
				return
			case models.GlobalRoleDisabled:
				jsonErrorCode(w, http.StatusForbidden, "Your account is not active yet.", models.I18nCodeForRole(user.Role, activationMode))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSuperadmin restricts a route to the account-wide superadmin: user
// management, moderating anyone's post or comment, and reaching into a club
// they don't belong to.
func RequireSuperadmin(users *store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := UserIDFromContext(r.Context())
			user, err := users.FindByID(r.Context(), userID)
			if err != nil || user.Role != models.GlobalRoleSuperadmin {
				jsonError(w, http.StatusForbidden, "Superadmin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireCoach restricts a route to accounts that may create clubs, upload
// programs and extend the exercise catalog.
func RequireCoach(users *store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := UserIDFromContext(r.Context())
			user, err := users.FindByID(r.Context(), userID)
			if err != nil || !models.CanCoach(user.Role) {
				jsonError(w, http.StatusForbidden, "Coach access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClubMembership resolves the caller's role in the {clubId} being addressed and
// puts it in the request context, rejecting anyone who isn't a member. A
// superadmin is admitted to every club as an admin, so moderation and support
// don't require joining one.
func ClubMembership(clubs *store.ClubStore, users *store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := UserIDFromContext(r.Context())
			clubID := chi.URLParam(r, "clubId")

			membership, err := clubs.FindMembership(r.Context(), clubID, userID)
			if err == nil {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clubRoleKey, membership.Role)))
				return
			}
			if !errors.Is(err, store.ErrNotFound) {
				jsonError(w, http.StatusInternalServerError, err.Error())
				return
			}

			user, userErr := users.FindByID(r.Context(), userID)
			if userErr == nil && user.Role == models.GlobalRoleSuperadmin {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clubRoleKey, models.RoleAdmin)))
				return
			}

			// A non-member is told the club doesn't exist rather than that they
			// aren't in it: a club's existence is itself private.
			jsonError(w, http.StatusNotFound, "Club not found")
		})
	}
}

// RequireClubManager restricts a route to a club's owner or admins. It must sit
// behind ClubMembership, which is what resolves the role.
func RequireClubManager(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := ClubRoleFromContext(r.Context())
		if !models.CanManageClub(role) {
			jsonError(w, http.StatusForbidden, "You must be an owner or an admin of this club")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClubRoleFromContext returns the caller's role in the club being addressed, as
// resolved by ClubMembership.
func ClubRoleFromContext(ctx context.Context) (models.Role, bool) {
	role, ok := ctx.Value(clubRoleKey).(models.Role)
	return role, ok
}
