package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

// visitTimeout bounds the background work a visit kicks off - an upsert and, on
// a first sighting, a geo lookup over the network. It runs on its own context,
// detached from the request that triggered it, so the response is never held up
// by it and it is never cancelled when that response is written.
const visitTimeout = 10 * time.Second

// VisitorStore is what BanIP needs of the session store: the per-request ban
// check, and the pieces that record and enrich a visit. Narrow on purpose, so
// the middleware package keeps not importing store.
type VisitorStore interface {
	IsIpBanned(ctx context.Context, ip string) (bool, error)
	RecordVisit(ctx context.Context, userID *string, ip string, at time.Time) (bool, error)
	NeedsGeo(ctx context.Context, ip string) (bool, error)
	SetGeo(ctx context.Context, ip string, info map[string]string) error
}

// Enricher resolves what is known about an address. It matches geo.Geo.Enrich,
// and is wired in from main so this package depends on neither geo nor its HTTP
// providers. A nil Enricher simply records visits without geolocation.
type Enricher func(ctx context.Context, ip string) (map[string]string, error)

// BanIP refuses a request whose address is banned, and - for anonymous traffic
// - records the visit so the address shows up on the administration roster.
//
// It sits in front of everything, so a ban holds on the login page and the
// public feed alike. The ban check fails open: if the lookup errors (a database
// blip), the request is allowed rather than the whole site going dark over an
// administrative feature. An address that will not parse is left alone rather
// than rejected - behind a trusted proxy the header is always a real address,
// and refusing an odd one would only be a way to lock legitimate traffic out.
func BanIP(store VisitorStore, enrich Enricher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The infra endpoints are left untouched: a load balancer's health
			// poll and a metrics scrape are not visits to log, and gating them
			// on a database lookup would make /health depend on the database it
			// is meant to report on.
			if isInfraPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ip := utils.ClientIP(r)
			if utils.IsBlank(ip) || store == nil || net.ParseIP(ip) == nil {
				next.ServeHTTP(w, r)
				return
			}

			banned, err := store.IsIpBanned(r.Context(), ip)
			if err != nil {
				slog.WarnContext(r.Context(), "ip ban check failed, allowing", "ip", ip, "error", err)
			} else if banned {
				jsonErrorCode(w, http.StatusForbidden, "This address has been banned.", models.I18nIPBanned)
				return
			}

			// Only anonymous traffic is logged here. A signed-in member's
			// address is recorded against them when their session is minted
			// (recordConnection), which is where the identity is known - here,
			// in front of auth, every request looks anonymous. Skipping requests
			// that carry a credential keeps this from writing a nameless row for
			// every authenticated call too.
			if !hasCredential(r) {
				go TrackVisit(store, enrich, nil, ip)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TrackVisit upserts a visit and, when the address is newly seen, enriches it
// with a geo lookup. It is safe to call in its own goroutine: it runs on a
// fresh background context, recovers from a panic in the enricher, and never
// returns anything - a lost visit counter or a missing city must never disturb
// the request that spawned it.
func TrackVisit(store VisitorStore, enrich Enricher, userID *string, ip string) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("panic recording a visit", "ip", ip, "panic", rec)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), visitTimeout)
	defer cancel()

	firstSeen, err := store.RecordVisit(ctx, userID, ip, time.Now().UTC())
	if err != nil {
		slog.WarnContext(ctx, "failed to record a visit", "ip", ip, "error", err)
		return
	}
	if !firstSeen || enrich == nil {
		return
	}

	needs, err := store.NeedsGeo(ctx, ip)
	if err != nil || !needs {
		return
	}
	info, err := enrich(ctx, ip)
	if err != nil || len(info) == 0 {
		return
	}
	if err := store.SetGeo(ctx, ip, info); err != nil {
		slog.WarnContext(ctx, "failed to store geo enrichment", "ip", ip, "error", err)
	}
}

// hasCredential reports whether a request carries any proof of identity, so the
// ban guard can leave authenticated traffic for the session-mint path to log.
func hasCredential(r *http.Request) bool {
	return utils.IsNotBlank(r.Header.Get("Authorization")) || utils.IsNotBlank(r.Header.Get("X-Api-Key"))
}

// isInfraPath reports whether a path is an operational endpoint the guard
// should pass straight through - the health check and the metrics scrape.
func isInfraPath(path string) bool {
	return strings.HasSuffix(path, "/health") || strings.HasSuffix(path, "/metrics")
}
