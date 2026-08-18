/// The parsed `api_url = ...` / `api_key = ...` config text.
///
/// It is the same text three ways: the file the web app's API-keys screen
/// downloads for a future CLI, the payload of the QR code shown beside it, and
/// what this app reads when that code is scanned. One format, so enrolling a
/// device and configuring a script are the same act.
class SessionConfig {
  final String apiUrl;
  final String apiKey;

  const SessionConfig({required this.apiUrl, required this.apiKey});

  bool get isComplete => apiUrl.isNotEmpty && apiKey.isNotEmpty;
}
