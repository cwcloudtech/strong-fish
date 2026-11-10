// Velocity-based effort, mirrored from the API's canonical model
// (sf-api/internal/velocity/effort.go) so an inline panel can show an estimate
// without a round-trip. The reusable endpoint POST /users/me/bar-speed/effort
// is the server-authoritative version; keep the constants below in step with it.
//
// RPE and RIR are the same number two ways: RIR = 10 - RPE. Effort is always
// computed and stored as RPE; the metric is a display choice.

// Per-movement minimum velocity threshold (MVT, m/s, where RPE is 10) and k,
// the mean concentric velocity lost per rep of fatigue.
const EFFORT_MODELS = {
  squat: { mvt: 0.3, k: 0.05 },
  bench: { mvt: 0.17, k: 0.045 },
  deadlift: { mvt: 0.16, k: 0.05 },
};

/**
 * The velocity-based RPE/RIR for one rep's mean concentric velocity on a lift,
 * or null when there is no model for the movement or the velocity is unusable.
 * RPE is clamped to 5..10 (velocity cannot read a confident lower number) and
 * rounded to the nearest half point.
 */
export function estimateEffort(movement, lastRepVelocity) {
  const model = EFFORT_MODELS[movement];
  if (!model || !(lastRepVelocity > 0)) return null;
  let rpe = 10 - (lastRepVelocity - model.mvt) / model.k;
  rpe = Math.min(10, Math.max(5, rpe));
  rpe = Math.round(rpe * 2) / 2;
  return { rpe, rir: 10 - rpe };
}

/** The last (slowest) rep's velocity - the set's effort signal. */
export function lastRepVelocity(velocities) {
  const vs = velocities || [];
  return vs.length ? vs[vs.length - 1] : 0;
}

/** True when the member reads effort in reps-in-reserve rather than RPE. */
export function isRir(metric) {
  return String(metric || "").toLowerCase() === "rir";
}

/** An RPE value shown in the member's chosen scale: the RPE, or 10 - RPE. */
export function formatEffort(rpe, metric) {
  if (rpe == null) return "";
  const value = isRir(metric) ? Math.max(0, 10 - rpe) : rpe;
  // Trim a pointless .0 so "7" reads as 7, "7.5" stays 7.5.
  return String(Math.round(value * 2) / 2);
}
