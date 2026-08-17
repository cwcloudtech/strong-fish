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

// Auth authenticates a request from its JWT Bearer token and puts the user id
// in the request context.
func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := userIDFromRequest(secret, r)
			if !ok {
				jsonError(w, http.StatusUnauthorized, "Not authorised")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
		})
	}
}

// OptionalAuth is Auth for endpoints that serve logged-out visitors too (a
// shared public profile, the public feed): a valid token identifies the caller,
// and a missing or bad one simply leaves them anonymous rather than rejecting
// the request.
func OptionalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
