# AI instruct 38

## A device-agnostic bar-speed engine (any accelerometer → StrongFish API → StrongFish DB)

Record bar speed without the Stance app and without depending on Stance's
backend. **Any** accelerometer streams its raw signal to a StrongFish endpoint;
the API runs **our own** velocity / rep algorithm, keyed by the movement; the
result is persisted in **StrongFish's own database** and shown in the bar-speed
panel. Sources are interchangeable: the StrongFish phone app over BLE, or a
cheap sensor (e.g. an **ESP32 + accelerometer**) posting straight over HTTP.

This is a **plan**, not the implementation. It is written to be fully
independent: no Stance model, no Stance service, no write into Stance's store
(see "Explicitly out of scope").

---

## Why independent (and why that is fine)

Reverse-engineering `~/app-sf` showed that matching Stance's exact numbers would
need **their proprietary assets**, which they could gate behind an approval:

- Deadlift uses two bundled TFLite models (`assets/deadlift/unet_detrend…tflite`,
  `tcn_rep_seg…tflite`).
- Squat/bench is computed on **their** server
  (`preprocessing-python-squat-bench-ap.vercel.app`), not on the phone at all.

Per the decision to not depend on anything whitelist-gated: **we build our own
engine.** The physics and signal processing are open sports science, not their
IP:

- Rep detection and velocity from a vertical-acceleration signal use classical
  methods (filtering, integration with drift correction, peak finding with
  prominence) — the same family the app's own `DeadliftSignalProcessing` uses
  (gaussian/uniform filters, gradient, `findPeaks`, prominence), minus the ML
  refinement.
- The velocity→RPE mapping is the **personal profile** method, general to VBT:
  pair (felt RPE, last-rep mean velocity) from history, band each RPE by its
  interquartile velocities, and read a new set's RPE by matching its last-rep
  speed (±~20%) into those bands, clamped 5–10. (Documented in
  `sf-api/internal/templates/health.skill.md`.)

Our numbers will not be byte-identical to Stance's, and that is acceptable: this
is an independent engine, validated on its own terms (below), that works for any
device rather than one brand.

---

## Target architecture

```
 Source of raw samples                 StrongFish API (Go)              Velocity engine
 ─────────────────────                 ───────────────────              ───────────────
 (a) StrongFish phone, BLE   samples   POST /v1/bar-speed/analyze       /analyze?movement=…
     to any IMU          ───────────▶  {movement, load, unit,     ──▶   classical pipeline:
 (b) ESP32 + accelerometer,  (HTTP)    sampleRateHz, samples[],         filter → integrate →
     Wi-Fi HTTP POST                   deviceKey}                        detrend → rep segment →
 (c) WitMotion over BLE                                            ◀──   per-rep MCV/peak,
                                        reps[], mcv, peak, loss,          velocity loss, RPE
                                        predicted RPE
                                                 │
                                                 ▼
                                   persist in StrongFish's own DB
                                   (sf-db), shown in the bar-speed panel
```

The **movement** parameter selects the curve/thresholds (squat, bench, deadlift,
and later any lift). The **source is just a transport**: BLE from our app, or an
ESP32 posting JSON — both hit the same endpoint with the same sample shape, so a
new device is a new client, never a new backend.

Engine as a small **Python sidecar** (numpy/scipy are the natural fit for the
signal processing), called by the Go API over a private network. Go stays the
front door: auth, device keys, persistence, the existing device-link screens.

### Sample contract (what any device sends)

```jsonc
POST /v1/bar-speed/analyze
{
  "movement": "deadlift",          // squat | bench | deadlift | ...
  "load": 180, "unit": "kg",
  "sampleRateHz": 200,             // the device states its rate
  "samples": [                     // ordered, one per tick
    { "t": 0.000, "ax": 0.01, "ay": -0.02, "az": 9.83,
      "gx": 0.0, "gy": 0.0, "gz": 0.0,          // gyro optional
      "qw": 1, "qx": 0, "qy": 0, "qz": 0 }      // quaternion optional
  ]
}
```

Accelerometer alone is enough (we derive vertical accel from gravity when there
is no orientation); gyro/quaternion improve the vertical projection when present.
This keeps a 5 € ESP32 board a first-class source.

---

## Adapters (drivers): one interface, many sources

Bar speed never comes from "Stance" in the code — it comes from a **source**
behind an interface, so a new device is a new adapter and nothing downstream
changes. Two capabilities, mirroring the existing device-link pattern
(`ActivityReader`, `BodyCompositionReader`, …):

```go
// Where a member's sets come from.
type BarSpeedSource interface {
    Name() string
    // The member's last set of each lift, from this source.
    LastSets(ctx context.Context, member Member) ([]BarSet, error)
}

// A source we capture into ourselves: our engine computes it and our store
// keeps it. Stance does NOT implement this - its capture lives in its own app.
type BarSpeedRecorder interface {
    BarSpeedSource
    // Turn a raw sample stream into a computed, persisted set.
    Record(ctx context.Context, member Member, capture Capture) (BarSet, error)
}

// Optional: a sink a computed set can also be mirrored into, for a member who
// wants a StrongFish-captured set to show up in another app too. Our own store
// is always written (by Record); a Publisher is an *additional*, external sink.
type BarSpeedPublisher interface {
    Publish(ctx context.Context, member Member, set BarSet, raw Capture) error
}
```

Adapters:

- **`stance` adapter** — implements `BarSpeedSource` only (read). It **wraps the
  existing calls to their Supabase/Firebase** (`get_user_sessions` keyed by the
  Firebase uid) and nothing more: we read the member's own Stance sets exactly
  as today, and we never write there. All Stance-specific knowledge stays behind
  this one adapter.
- **`native` adapter** — implements `BarSpeedRecorder`. `Record` calls our
  velocity engine then saves to StrongFish's DB; `LastSets` reads our DB. This is
  the adapter every device we drive (BLE phone, ESP32, …) feeds, regardless of
  the sensor — the device is just the transport that produced `Capture`.
- **Future vendor adapters** — a new integration implements `BarSpeedSource`
  (and `BarSpeedRecorder` if we capture it ourselves) and registers, nothing
  else moves.

A StrongFish-captured set is **always** written to our own store. To *also*
mirror it into another app's backend (so it shows there too), that backend's
adapter implements `BarSpeedPublisher`. For Stance this is **deferred**: it
would require them to open an authorized ingestion path (an official write API,
or the backend partnership), because mirroring into their production store via
their anon key is not something we do uninvited (see "Explicitly out of scope").
The seam exists so that, the day Stance authorizes it, a `stance` publisher
slots in without touching anything else.

**Publisher resolution, with a no-op default.** After `Record` saves to our own
store, the handler picks an external publisher for that capture — and the
default is a **no-op** (`nullPublisher`) that does nothing. A real publisher is
chosen only when a capture explicitly warrants one:

- Capture from an **ESP32 / any non-Stance device** → no-op. Our store only.
- Member has **not** entered Stance credentials → no-op. Our store only.
- Member paired Stance **and** the capture is from a Stance device → eligible
  for a `stance` publisher — which is itself deferred until Stance authorizes an
  ingestion path, so until then it too resolves to the no-op.

So nothing is ever pushed outward by default; a non-no-op publisher is an
explicit, per-capture, authorization-gated opt-in. This keeps the ESP32 and the
credential-less member on the clean path with no special-casing in the handler.

### Placeholder: the authorized Stance push (operator fills this in)

A stub, left empty on purpose. StrongFish ships **no** uninvited write; the
operator plugs their own implementation in here **once Stance has agreed** to an
ingestion path (an official write API, or the backend partnership):

```go
// stancePublisher would mirror a StrongFish-captured set into Stance's own
// backend, so it also shows in the Stance app.
//
// Intentionally a no-op. Registered behind the nullPublisher default, so the
// handler never changes.
type stancePublisher struct{ /* client, config */ }

func (p *stancePublisher) Publish(ctx context.Context, member Member, set BarSet, raw Capture) error {
    // --- Here we can push the data to them, WITH THEIR AGREEMENT. ---
    // Operator: implement the authorized push once Stance opens an ingestion
    // path. Left empty by design; until then the set lives only in our store.
    return nil
}
```

The day it is implemented and enabled, captures for a Stance-paired member start
mirroring; until then they do not, and nothing else in the pipeline moves.

The bar-speed handler and the panel **iterate the registered sources and merge**
their sets, each tagged by source — a Stance-recorded squat and a StrongFish-
recorded squat sit side by side, labelled. A member with only our device sees
only the native source; a member with both sees both.

`Capture` is the normalized sample stream (the contract above) plus `movement`,
`load`, `unit`, `sampleRateHz` and the source/device id - the one shape every
device adapter produces and the engine consumes, so the engine never learns
about a specific sensor.

---

## Status

Implemented on `feat/bar-speed-engine`:

- **Engine** (`sf-api/internal/velocity`): classical VBT pipeline + predicted
  RPE from the member's own history, with tests.
- **Endpoint** `POST /me/bar-speed/analyze`: any source posts the raw stream +
  movement; computes, persists (reps found) and returns the set.
- **Persistence** (`sf-db` V21 + `store.BarSpeedStore`): recordings + reps in
  our own DB.
- **Consolidation** `GET /me/bar-speed`: most recent set of each lift across our
  store and a connected tracker, tagged by source; one panel on web + mobile.
- **ESP32 auth**: the existing StrongFish API key (X-Api-Key) authenticates a
  headless device - no new endpoint or key type needed.
- **ESP32 reference firmware**: `hardware/esp32-barspeed/`.
- **Mobile BLE contract**: `sf-mobile/lib/native/bar_speed_capture.dart` +
  `ServicesApi.analyzeBarSpeed`.

Remaining (needs hardware to validate, so not landed blind):

- The **native BLE** implementation (Kotlin/Swift) behind the capture contract,
  and the **record-a-set screen** that drives it. The engine, endpoint and
  consolidation are all exercisable without it (the ESP32 path, and tests).
- Engine hardening for noisy/low-rate sensors (per-movement thresholds, a
  confidence score).
- The authorized Stance publisher (the no-op stub stands until they agree).

## Work breakdown (phases)

### Phase 1 — Velocity engine (`sf-velocity/`, Python)
- `POST /analyze` → `{reps:[{meanMps, peakMps, startSec, endSec}],
  meanFrequencyHz, predictedRpe}`.
- Pipeline: project to vertical accel (`(worldZ_g − 1)·9.81` with orientation,
  else high-pass the raw magnitude), integrate at `sampleRateHz` with bias/drift
  correction, segment reps by prominence peak-finding (min/max rep duration,
  merge gaps), per-rep MCV = mean concentric velocity, peak = max.
- RPE via the personal profile (needs the member's history — Phase 3 passes it,
  or the engine returns velocity-only and the API derives RPE).
- Deterministic, stateless, private network only.

### Phase 2 — Ingestion endpoint (`sf-api`)
- `POST /v1/bar-speed/analyze` (authenticated member) and a device variant for
  headless sources: `POST /v1/bar-speed/ingest` with a **per-member device key**
  (so an ESP32 can post without a login). Validate, rate-limit, cap payload size.
- Resolve the member, call the engine, map to `BarSet` + a stored recording.

### Phase 3 — Persistence in StrongFish's own DB (`sf-db`, `sf-api/internal`)

Mirror Stance's persistence **architecture** (it is good), in **our** store:

- **Same table decomposition** (our schema, our DB): `bar_speed_recordings`
  (member, movement, load, unit, performed_at, mean/peak velocity, velocity
  loss, given/predicted RPE, source/device, `revision`, `deleted`) +
  `bar_speed_reps` (recording, order, mean, peak) + an optional
  `bar_speed_streams` for the raw samples and `*_algorithm_metadata` /
  `*_imu_quality` when we keep them. One `sf-db` migration per feature (merged
  into the feature's V-file).
- **Idempotent upsert on a stable id + `revision`**, like their `on_conflict`
  on the recording id, so a retried sync never duplicates a set.
- **Local-first + a pending sync queue on mobile** (same idea as their Realm +
  `Pending*` entities): a capture is written locally and flushed to our API when
  there is connectivity, so a set recorded in a gym basement is not lost. The
  queue is ours and trivial compared to theirs (one entity kind: a pending
  capture) because our engine runs server-side.
- A `RecordingStore` interface; the bar-speed panel reads our store **in
  addition to** the existing Stance read, so a Stance-recorded set and a
  StrongFish-recorded set show in the same place, tagged by source.

### Phase 4 — Device keys (`sf-api`)
- Per-member device keys (create/revoke) so an ESP32 (or any headless sensor)
  can authenticate the `ingest` call. Reuse the existing API-key machinery if it
  fits; scope a key to bar-speed ingest only.

### Phase 5 — Phone capture (`sf-mobile`)
- BLE capture controller (scan/connect/stream/buffer a set) behind one Dart
  interface; Android first, iOS later. Works with any BLE IMU exposing a serial
  stream (WitMotion's `ffe*` family included).
- On a comp-lift set, record → upload to `/analyze` → show the charts from the
  response. Keep "launch the Stance app" as a fallback when nothing is paired.

### Phase 6 — ESP32 reference firmware (`hardware/esp32-barspeed/`, optional)
- A minimal sketch: read an accelerometer (e.g. MPU-6050/LSM6DS3), buffer a set,
  POST the sample contract to `/v1/bar-speed/ingest` with the device key. Proves
  the endpoint is genuinely device-agnostic and gives cheap hardware a path.

---

## Validation / parity

Because the engine is ours, validate it against physics and repeatability, not
against Stance:
- Known-signal tests: synthesize accel traces for N reps at a set velocity and
  assert the engine recovers the rep count and velocity.
- Field check: a few sets recorded on both a known VBT reference and our device;
  compare MCV trends (not absolute identity).
- Where we happen to hold a raw stream, replay it twice and assert determinism.

## Risks

- **BLE fragility / device variety**: each sensor frames data differently; keep a
  thin per-device parser, one normalized sample shape inward.
- **Mobile-only BLE**: the web cannot read BLE; ESP32-over-HTTP covers the
  no-phone case. Reading/exporting already works everywhere.
- **Signal quality on cheap sensors**: lower sample rates / noisier accel widen
  the error; the engine should report a confidence and refuse implausible sets.
- **No smoke tests** (project convention): validate with the known-signal tests
  and compile/analyze; reproduce issues with throwaway repros.

## Explicitly out of scope (and why)

- **No writing into Stance's Supabase/Firebase**, and nothing that relies on a
  weakness in their access rules. StrongFish persists bar speed in **its own**
  database. We keep *reading* a member's own Stance data through the existing,
  agreed integration, but we do not push records into a third party's production
  store. This holds even for the "so it shows in the Stance app too" case and
  even for a Stance device: mirroring a StrongFish-captured set into their
  backend via their anon key is an uninvited write into a third party's prod
  system, and it would inject our own differently-computed numbers into their
  store as if they were their app's recordings. The `BarSpeedPublisher` seam is
  there precisely so this becomes a clean, one-adapter change **once Stance
  opens an authorized ingestion path** (an official write API, or the backend
  partnership) — the authorization is the prerequisite, not the plumbing. If a
  genuine security weakness in their setup is observed, the right move is a
  private, responsible disclosure to their team (hello@stancefitness.co), not
  using it.
- The partnership pitch (StrongFish as a shared, device-agnostic backend) stands
  on its own merits — any accelerometer, one engine, open to cheap hardware — and
  does not need leverage from their store to be worth proposing.

---

## Related, independent: "set done" timestamp + PDF column

Separate from the device engine, shippable on its own. When a member marks a set
**done** in a training session, record **when** and show it on the export.

- **Persist the moment.** Add `done_at` (ISO-8601, UTC, e.g.
  `2026-11-05T14:32:10Z`) to the set log alongside the existing `done` flag
  (`sf-api` log model + store; one `sf-db` migration merged into the feature's
  V-file). The clients send it when the set is ticked (`sf-ui` and `sf-mobile`),
  so it is the real time the lifter finished the set, not when the sheet is
  rendered. Keep it immutable once set unless the set is un-ticked.
- **Show it on the PDF (and PNG).** Add a "Done at" column to the feedback
  sheet (`programsheet.Columns`/`Row` in feedback mode, picked up by
  `programpdf`/`programpng`/`programxlsx`), rendered as a short local date-time,
  blank for sets not yet done. Only in the feedback/with-logs variant — the
  blank prescription sheet has nothing to put there.
- Backfill is not needed: existing logs simply have no `done_at` and print blank
  until the member logs new sets.
