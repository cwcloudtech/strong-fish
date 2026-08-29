import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../api/services.dart';
import '../i18n/app_localizations.dart';
import '../models/models.dart';
import '../models/session_config.dart';

/// The default API URL. It's overridable at runtime (see [SessionNotifier.setApiUrl])
/// because a club typically runs its own server, so a single build has to be
/// able to point anywhere.
const String defaultApiUrl = String.fromEnvironment('SF_API_URL', defaultValue: 'https://api.strong-fish.app');

const _tokenKey = 'sf.token';
const _apiKeyKey = 'sf.apiKey';
const _apiUrlKey = 'sf.apiUrl';
const _localeKey = 'sf.locale';
const _themeKey = 'sf.theme';

final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());

final apiProvider = Provider<SfApi>((ref) => SfApi(ref.watch(apiClientProvider)));

/// restoring (checking storage at boot) | missing (no usable session, show the
/// login screen) | connected (ready for the app).
enum SessionStatus { restoring, missing, connected }

class SessionState {
  final SessionStatus status;
  final User? user;
  final String apiUrl;

  const SessionState({
    this.status = SessionStatus.restoring,
    this.user,
    this.apiUrl = defaultApiUrl,
  });

  SessionState copyWith({SessionStatus? status, User? user, String? apiUrl, bool clearUser = false}) =>
      SessionState(
        status: status ?? this.status,
        user: clearUser ? null : (user ?? this.user),
        apiUrl: apiUrl ?? this.apiUrl,
      );
}

/// Holds the session. The token lives in secure storage (it is a credential);
/// the API URL and UI preferences live in shared preferences.
class SessionNotifier extends Notifier<SessionState> {
  static const _storage = FlutterSecureStorage();

  @override
  SessionState build() => const SessionState();

  ApiClient get _client => ref.read(apiClientProvider);
  SfApi get _api => ref.read(apiProvider);

  /// Runs once at boot: restores a saved session and confirms the token is
  /// still accepted, since it may have expired or the account may have been
  /// banned since it was issued.
  Future<void> restore() async {
    final prefs = await SharedPreferences.getInstance();
    final apiUrl = prefs.getString(_apiUrlKey) ?? defaultApiUrl;
    _client.setApiUrl(apiUrl);

    // A device enrolled by QR code holds a key, not a session token, and its
    // key does not expire on its own - so it is checked first and restored the
    // same way.
    final apiKey = await _read(_apiKeyKey);
    final token = apiKey != null && apiKey.isNotEmpty ? null : await _readToken();
    if ((apiKey == null || apiKey.isEmpty) && (token == null || token.isEmpty)) {
      state = state.copyWith(status: SessionStatus.missing, apiUrl: apiUrl);
      return;
    }

    _client.setApiKey(apiKey);
    _client.setToken(token);
    try {
      final user = await _api.me();
      state = SessionState(status: SessionStatus.connected, user: user, apiUrl: apiUrl);
    } catch (_) {
      // Any failure here means the stored token is unusable; drop it rather
      // than leaving the app in a half-signed-in state.
      await logout();
      state = state.copyWith(apiUrl: apiUrl);
    }
  }

  Future<String?> _readToken() => _read(_tokenKey);

  /// Reads a stored credential, tolerating a secure-storage backend that isn't
  /// available (a device without a keystore) rather than crashing the launch.
  Future<String?> _read(String key) async {
    try {
      return await _storage.read(key: key);
    } catch (_) {
      return null;
    }
  }

  Future<void> setApiUrl(String url) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_apiUrlKey, url);
    _client.setApiUrl(url);
    state = state.copyWith(apiUrl: url);
  }

  Future<User> completeLogin(String token) async {
    try {
      await _storage.write(key: _tokenKey, value: token);
      // Signing in with a password replaces any key this device was enrolled
      // with; leaving it behind would keep authenticating as whoever the key
      // belonged to.
      await _storage.delete(key: _apiKeyKey);
    } catch (_) {
      // A device without secure storage still gets a working session for this
      // run; it just won't survive a restart.
    }
    _client.setApiKey(null);
    _client.setToken(token);
    final user = await _api.me();
    state = state.copyWith(status: SessionStatus.connected, user: user);
    return user;
  }

  Future<void> refresh() async {
    if (!_client.hasSession) return;
    try {
      state = state.copyWith(user: await _api.me());
    } on Object catch (error) {
      if (asApiException(error).isUnauthorized) await logout();
    }
  }

  /// Signs this device in with a scanned config: the API URL and the API key
  /// it carries, in that order - the key is meaningless against a different
  /// server.
  ///
  /// The key is only kept once the API has accepted it. A QR code that scans
  /// cleanly but names a server that rejects it should leave the device
  /// exactly as it was, not half-enrolled.
  Future<User> connectWithConfig(SessionConfig config) async {
    final previousUrl = state.apiUrl;
    _client.setApiUrl(config.apiUrl);
    _client.setToken(null);
    _client.setApiKey(config.apiKey);

    try {
      final user = await _api.me();
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_apiUrlKey, config.apiUrl);
      try {
        await _storage.delete(key: _tokenKey);
        await _storage.write(key: _apiKeyKey, value: config.apiKey);
      } catch (_) {
        // No keystore: the session works for this run but won't survive a
        // restart, which is the same trade a password login makes.
      }
      state = SessionState(
        status: SessionStatus.connected,
        user: user,
        apiUrl: config.apiUrl,
      );
      return user;
    } catch (error) {
      _client.setApiKey(null);
      _client.setApiUrl(previousUrl);
      rethrow;
    }
  }

  Future<void> logout() async {
    try {
      await _storage.delete(key: _tokenKey);
      await _storage.delete(key: _apiKeyKey);
    } catch (_) {
      // Nothing to clean up if secure storage was never available.
    }
    _client.clearSession();
    state = SessionState(status: SessionStatus.missing, apiUrl: state.apiUrl);
  }
}

final sessionProvider = NotifierProvider<SessionNotifier, SessionState>(SessionNotifier.new);

/// The app's language, persisted and defaulting to the device's.
class LocaleNotifier extends Notifier<String> {
  @override
  String build() => 'en';

  Future<void> load() async {
    final prefs = await SharedPreferences.getInstance();
    final stored = prefs.getString(_localeKey);
    if (stored != null && dictionaries.containsKey(stored)) {
      state = stored;
      return;
    }
    final device = WidgetsBinding.instance.platformDispatcher.locale.languageCode;
    state = dictionaries.containsKey(device) ? device : 'en';
  }

  Future<void> set(String locale) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_localeKey, locale);
    state = locale;
  }
}

final localeProvider = NotifierProvider<LocaleNotifier, String>(LocaleNotifier.new);

/// A translate function bound to the current locale, so widgets call `t('...')`.
final tProvider = Provider<String Function(String, [Map<String, String>?])>((ref) {
  final locale = ref.watch(localeProvider);
  return (key, [vars]) => translate(locale, key, vars);
});

/// Translates an API failure into the current locale.
final tErrorProvider = Provider<String Function(Object)>((ref) {
  final locale = ref.watch(localeProvider);
  return (error) => translateError(locale, asApiException(error));
});

class ThemeModeNotifier extends Notifier<ThemeMode> {
  @override
  ThemeMode build() => ThemeMode.system;

  Future<void> load() async {
    final prefs = await SharedPreferences.getInstance();
    switch (prefs.getString(_themeKey)) {
      case 'light':
        state = ThemeMode.light;
      case 'dark':
        state = ThemeMode.dark;
      default:
        state = ThemeMode.system;
    }
  }

  Future<void> set(ThemeMode mode) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_themeKey, mode.name);
    state = mode;
  }
}

final themeModeProvider = NotifierProvider<ThemeModeNotifier, ThemeMode>(ThemeModeNotifier.new);

/// The exercise catalog, loaded once and reused by the 1RM screen.
/// The deployment's own settings: the upload caps, mostly.
///
/// Fetched rather than assumed, because a club that pays for its own bandwidth
/// can raise or lower them (SF_MAX_VIDEO_SIZE). Read before an upload so a file
/// that cannot land is refused in a sentence rather than after two minutes of
/// transfer - the API stops an oversized body part-way, which reaches the phone
/// as a lost connection and explains nothing.
final serverConfigProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  return ref.watch(apiProvider).config();
});

/// The largest video this deployment accepts, in bytes, or 0 when the config
/// cannot be read - in which case the upload is attempted and the API has the
/// final word, as it always did.
///
/// Awaited rather than read: nothing on screen watches the config, so a lazy
/// provider would never have fetched it and the limit would read as 0 forever.
/// The result is cached by riverpod, so this costs one request per session.
Future<int> maxVideoSize(WidgetRef ref) async {
  try {
    final config = await ref.read(serverConfigProvider.future);
    final value = config['maxVideoSize'];
    return value is num ? value.toInt() : 0;
  } catch (_) {
    return 0;
  }
}

/// A byte count as something a person can compare a file against - "500 MB".
///
/// Rounded rather than exact: the number exists to answer "is my video near
/// the limit", and 524288000 answers that worse than 500 MB does.
String formatBytes(int bytes) {
  final mb = bytes / (1024 * 1024);
  if (mb <= 0) return '';
  return mb >= 1024 ? '${(mb / 1024).toStringAsFixed(1)} GB' : '${mb.round()} MB';
}

final exercisesProvider = FutureProvider<List<Exercise>>((ref) async {
  // Rebuilds when the session changes, so signing in as someone else doesn't
  // serve the previous account's cached list.
  ref.watch(sessionProvider.select((session) => session.user?.id));
  return ref.watch(apiProvider).exercises();
});

final oneRmsProvider = FutureProvider<List<OneRm>>((ref) async {
  ref.watch(sessionProvider.select((session) => session.user?.id));
  return ref.watch(apiProvider).oneRms();
});

final assignmentsProvider = FutureProvider<List<Assignment>>((ref) async {
  ref.watch(sessionProvider.select((session) => session.user?.id));
  return ref.watch(apiProvider).assignments();
});

final assignmentProvider =
    FutureProvider.family<AssignmentDetail, String>((ref, assignmentId) async {
  return ref.watch(apiProvider).assignment(assignmentId);
});

final conversationsProvider = FutureProvider<List<Conversation>>((ref) async {
  return ref.watch(apiProvider).conversations();
});

final threadProvider = FutureProvider.family<Thread, String>((ref, userId) async {
  return ref.watch(apiProvider).thread(userId);
});

final invitationsProvider = FutureProvider<List<Invitation>>((ref) async {
  return ref.watch(apiProvider).invitations();
});

final eventsProvider = FutureProvider<List<Event>>((ref) async {
  return ref.watch(apiProvider).events();
});

final clubsProvider = FutureProvider<List<Club>>((ref) async {
  ref.watch(sessionProvider.select((session) => session.user?.id));
  return ref.watch(apiProvider).clubs();
});
