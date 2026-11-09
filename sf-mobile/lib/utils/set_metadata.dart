/// Descriptive tags for a bar-speed recording, detected from the prescription
/// text of a training set - the exercise name and the coach's notes. These are
/// best-effort hints that pre-fill the record screen's dropdowns; the member
/// confirms or overrides them before recording. The same words are what the
/// AI reading is told about, so a tempo set's slow bar is not read as fatigue.
library;

const barSpeedSetTypes = ['warmup', 'topset', 'backoff'];

final RegExp _tempoNotation = RegExp(r'[0-9]+:[0-9]+:[0-9]*');
const _tempoWords = ['2ct', 'tempo', 'paused', 'spotto'];

/// A set is a tempo variation when its text carries a tempo or pause cue: a
/// "2ct", "tempo", "paused" or "spotto" word (case-insensitive), or a "3:3:0"
/// style tempo notation.
bool detectTempoVariation(String text) {
  final lower = text.toLowerCase();
  if (_tempoWords.any(lower.contains)) return true;
  return _tempoNotation.hasMatch(lower);
}

/// The set type read from the text - "topset", "backoff", or the default
/// "warmup" when the prescription names neither.
String detectSetType(String text) {
  final lower = text.toLowerCase();
  if (lower.contains('top set') ||
      lower.contains('topset') ||
      lower.contains('top-set')) {
    return 'topset';
  }
  if (lower.contains('back off') ||
      lower.contains('backoff') ||
      lower.contains('back-off')) {
    return 'backoff';
  }
  return 'warmup';
}

/// Validates a set type, falling back to warmup - mirrors the API.
String normalizeSetType(String value) =>
    barSpeedSetTypes.contains(value) ? value : 'warmup';
