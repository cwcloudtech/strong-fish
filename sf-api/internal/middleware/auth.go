package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"strong-fish-api/internal/utils"
)

type contextKey string

const (
	userIDKey   contextKey = "userID"
	clubRoleKey contextKey = "clubRole"
)

// ApiKeyVerifier resolves the user a key token's hash belongs to. It's a
// narrow interface rather than *store.ApiKeyStore so this package keeps not
// depending on store.
type ApiKeyVerifier interface {
	VerifyHash(ctx context.Context, hash string) (userID string, err error)
}

// Auth authenticates a request and puts the user id in the request context,
// accepting either an X-Api-Key header or a JWT Bearer token.
//
// X-Api-Key wins when both are present, and a bad key is rejected outright
// rather than falling back to the JWT - a client that sent a key meant to use
// it, and silently authenticating as somebody else's session would be worse
// than a 401. Both paths set the same context value, so nothing downstream
// (RequireActiveUser, ClubMembership, ...) can tell them apart.
func Auth(secret string, apiKeys ApiKeyVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, present, ok := apiKeyUserID(apiKeys, r); present {
				if !ok {
					jsonError(w, http.StatusUnauthorized, "Not authorised")
					return
				}
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
				return
			}

			userID, ok := userIDFromRequest(secret, r)
			if !ok {
				jsonError(w, http.StatusUnauthorized, "Not authorised")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
		})
	}
}

// apiKeyUserID resolves an X-Api-Key header. present reports whether the
// header was sent at all, so the caller can tell "no key" (fall through to the
// JWT) from "a key that doesn't verify" (reject).
func apiKeyUserID(apiKeys ApiKeyVerifier, r *http.Request) (userID string, present, ok bool) {
	token := r.Header.Get("X-Api-Key")
	if apiKeys == nil || utils.IsBlank(token) {
		return utils.EMPTY, false, false
	}
	userID, err := apiKeys.VerifyHash(r.Context(), utils.HashToken(token))
	if err != nil || utils.IsBlank(userID) {
		return utils.EMPTY, true, false
	}
	return userID, true, true
}

// OptionalAuth is Auth for endpoints that serve logged-out visitors too (a
// shared public profile, the public feed): valid credentials identify the
// caller, and missing or bad ones simply leave them anonymous rather than
// rejecting the request. Unlike Auth, a bad API key here is not fatal - the
// endpoint has an anonymous answer to give.
func OptionalAuth(secret string, apiKeys ApiKeyVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, _, ok := apiKeyUserID(apiKeys, r); ok {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
				return
			}
			if userID, ok := userIDFromRequest(secret, r); ok {
				r = r.WithContext(context.WithValue(r.Context(), userIDKey, userID))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// userIDFromRequest verifies the Authorization header's bearer token.
func userIDFromRequest(secret string, r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return utils.EMPTY, false
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(strings.TrimPrefix(header, "Bearer "), claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return utils.EMPTY, false
	}

	sub, ok := claims["sub"].(string)
	if !ok || utils.IsBlank(sub) {
		return utils.EMPTY, false
	}
	return sub, true
}

// UserIDFromContext returns the authenticated caller's id, and false for an
// anonymous request (only possible behind OptionalAuth).
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok && utils.IsNotBlank(id)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonErrorCode(w, status, message, utils.EMPTY)
}

func jsonErrorCode(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]string{"message": message}
	if utils.IsNotBlank(code) {
		body["i18n_code"] = code
	}
	_ = json.NewEncoder(w).Encode(body)
}
