// Velocity-based effort, mirrored from the API's canonical model
// (sf-api/internal/velocity/effort.go) so a panel can show an estimate without a
// round-trip. The reusable endpoint POST /users/me/bar-speed/effort is the
// server-authoritative version; keep the constants below in step with it.
//
// RPE and RIR are the same number two ways: RIR = 10 - RPE. Effort is always
// computed and stored as RPE; the metric is a display choice.

/// Per-movement minimum velocity threshold (MVT, m/s, where RPE is 10) and k,
/// the mean concentric velocity lost per rep of fatigue.
const Map<String, ({double mvt, double k})> _effortModels = {
  'squat': (mvt: 0.30, k: 0.05),
  'bench': (mvt: 0.17, k: 0.045),
  'deadlift': (mvt: 0.16, k: 0.05),
};

/// A velocity-based effort reading: the RPE, and the RIR it implies.
class EffortEstimate {
  final double rpe;
  final double rir;
  const EffortEstimate(this.rpe, this.rir);
}

/// The velocity-based RPE/RIR for one rep's mean concentric velocity on a lift,
/// or null when there is no model for the movement or the velocity is unusable.
/// RPE is clamped to 5..10 and rounded to the nearest half point.
EffortEstimate? estimateEffort(String movement, double lastRepVelocity) {
  final model = _effortModels[movement];
  if (model == null || lastRepVelocity <= 0) return null;
  var rpe = 10 - (lastRepVelocity - model.mvt) / model.k;
  if (rpe < 5) rpe = 5;
  if (rpe > 10) rpe = 10;
  rpe = (rpe * 2).round() / 2;
  return EffortEstimate(rpe, 10 - rpe);
}

/// The last (slowest) rep's velocity - the set's effort signal.
double lastRepVelocity(List<double> velocities) =>
    velocities.isEmpty ? 0 : velocities.last;

/// True when the member reads effort in reps-in-reserve rather than RPE.
bool isRir(String? metric) => (metric ?? '').toLowerCase() == 'rir';

/// An RPE value shown in the member's chosen scale: the RPE, or 10 - RPE.
String formatEffort(double rpe, String? metric) {
  final value = isRir(metric) ? (10 - rpe).clamp(0, 10) : rpe;
  final rounded = (value * 2).round() / 2;
  // Trim a pointless .0 so "7" reads as 7, "7.5" stays 7.5.
  return rounded == rounded.roundToDouble()
      ? rounded.toStringAsFixed(0)
      : rounded.toString();
}
