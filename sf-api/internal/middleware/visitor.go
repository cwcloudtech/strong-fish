package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
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

// BanIP refuses a request whose address is banned, and records the visit so the
// address - and the member behind it, when there is one - shows up on the
// administration roster.
//
// It sits in front of everything, so a ban holds on the login page and the
// public feed alike. The ban check fails open: if the lookup errors (a database
// blip), the request is allowed rather than the whole site going dark over an
// administrative feature. An address that will not parse is left alone rather
// than rejected - behind a trusted proxy the header is always a real address,
// and refusing an odd one would only be a way to lock legitimate traffic out.
//
// Every request is logged, not just signup/login: the member is resolved from
// the request's own JWT (so the attribution is known here, in front of auth),
// and the record is made at most once per calendar day per (ip, member) - the
// database enforces that, and a per-replica day cache keeps the hot path from
// even attempting a write on a repeat. A request with no (valid) token is
// logged anonymously, the same once-a-day.
func BanIP(store VisitorStore, enrich Enricher, secret string, alerter *BanAlerter) func(http.Handler) http.Handler {
	seen := newDayCache()
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
				alerter.Alert("ip:"+ip, fmt.Sprintf("banned ip %s tried %s %s", ip, r.Method, r.URL.Path))
				jsonErrorCode(w, http.StatusForbidden, "This address has been banned.", models.I18nIPBanned)
				return
			}

			// Resolve who is behind the request from its own token, and log the
			// visit once a day. A member browsing all afternoon from one address
			// is one row, bumped once.
			var userID *string
			if uid, ok := userIDFromRequest(secret, r); ok {
				userID = &uid
			}
			if seen.firstToday(dayKey(userID, ip)) {
				go TrackVisit(store, enrich, userID, ip)
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

	recorded, err := store.RecordVisit(ctx, userID, ip, time.Now().UTC())
	if err != nil {
		slog.WarnContext(ctx, "failed to record a visit", "ip", ip, "error", err)
		return
	}
	// Only when this call actually wrote (a first-ever sighting, or a new day) is
	// geo worth a look - and NeedsGeo then keeps it to once per address, so the
	// external lookup is retried across days only until it succeeds.
	if !recorded || enrich == nil {
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

// dayCache remembers which (member, ip) pairs have already been recorded today
// in this process, so the guard records each only once a day without a database
// round trip on every request. It resets when the day turns over, bounding it to
// one day's distinct visitors. The database is the real authority on once-a-day
// (across replicas and restarts); this is only the fast path in front of it.
type dayCache struct {
	mu   sync.Mutex
	day  string
	seen map[string]struct{}
}

func newDayCache() *dayCache {
	return &dayCache{day: today(), seen: make(map[string]struct{})}
}

// firstToday reports whether key has not been seen yet today, recording it when
// so. A new day clears the set.
func (c *dayCache) firstToday(key string) bool {
	now := today()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.day != now {
		c.day = now
		c.seen = make(map[string]struct{})
	}
	if _, ok := c.seen[key]; ok {
		return false
	}
	c.seen[key] = struct{}{}
	return true
}

func today() string { return time.Now().UTC().Format("2006-01-02") }

// dayKey identifies a (member-or-anonymous, ip) pair for the day cache.
func dayKey(userID *string, ip string) string {
	if userID != nil {
		return *userID + "|" + ip
	}
	return "|" + ip
}

// isInfraPath reports whether a path is an operational endpoint the guard
// should pass straight through - the health check and the metrics scrape.
func isInfraPath(path string) bool {
	return strings.HasSuffix(path, "/health") || strings.HasSuffix(path, "/metrics")
}
