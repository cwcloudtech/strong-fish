package middleware

import (
	"context"
	"net/http"

	"strong-fish-api/internal/utils"
)

// cidHeader is the correlation-id header, read from the caller when it sets one
// (a gateway, a client stitching a trace together) and echoed back on the
// response either way, so a caller can find its request in the logs by an id it
// chose. The same spelling ~/uprodit uses.
const cidHeader = "X-Cid"

// cidKey is where the resolved correlation id lives on the request context.
const cidKey contextKey = "cid"

// Correlation gives every request a correlation id - the caller's X-Cid when it
// sent one, a fresh random one otherwise - puts it on the context and the
// response header, so the access log line and anything else logged during the
// request can carry it. It is the outermost middleware, ahead of Instrument, so
// the id is already settled when the request is logged.
func Correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid := r.Header.Get(cidHeader)
		if utils.IsBlank(cid) {
			if generated, err := utils.RandomHex(16); err == nil {
				cid = generated
			}
		}
		w.Header().Set(cidHeader, cid)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cidKey, cid)))
	})
}

// CIDFromContext returns the request's correlation id, or "" when Correlation
// did not run (a code path with no HTTP request behind it).
func CIDFromContext(ctx context.Context) string {
	cid, _ := ctx.Value(cidKey).(string)
	return cid
}
