import '../models/session_config.dart';

/// Reads the "key = value" lines the API writes (see the Go
/// ConfigHandler.buildClientConfig). A QR code is only a container for that
/// text, so the scanner and a pasted-in file go through this same function.
///
/// Unknown keys are ignored rather than rejected: the format is meant to grow,
/// and a config carrying a field this version doesn't know about should still
/// sign the device in. When a key repeats, the last line wins.
SessionConfig parseConfigText(String raw) {
  final lines = raw.split('\n');

  String valueOf(String key) {
    String? match;
    for (final line in lines) {
      if (line.trimLeft().startsWith('$key =')) match = line;
    }
    if (match == null) return '';
    final parts = match.split(' = ');
    return parts.length > 1 ? parts.sublist(1).join(' = ').trim() : '';
  }

  return SessionConfig(apiUrl: valueOf('api_url'), apiKey: valueOf('api_key'));
}
