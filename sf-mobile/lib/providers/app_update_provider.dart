import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:open_filex/open_filex.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:path_provider/path_provider.dart';

/// Where the app checks for a newer version of itself, and where it downloads
/// it from.
///
/// Deliberately *not* the session's API URL. That URL is configurable - it can
/// arrive from a scanned QR code - and letting it decide which APK this device
/// installs would turn a bad QR code into arbitrary code execution. Updates
/// track the release the app was installed from, and nothing a session says
/// can redirect them.
///
/// A fork running its own builds overrides both at compile time:
/// `flutter build apk --dart-define=SF_UPDATE_URL=https://strong-fish.example`.
const _updateBaseUrl = String.fromEnvironment(
  'SF_UPDATE_URL',
  defaultValue: 'https://api.strong-fish.com',
);
const _downloadBaseUrl = String.fromEnvironment(
  'SF_DOWNLOAD_URL',
  defaultValue: 'https://strong-fish.com',
);

String get _manifestUrl => '$_updateBaseUrl/v1/manifest';

String _apkUrl(String version) => '$_downloadBaseUrl/strong-fish-v$version.apk';

/// Compares dotted version strings segment by segment and numerically, with
/// missing segments read as 0.
///
/// String comparison would be wrong in the way that matters: "1.10" sorts
/// before "1.9" alphabetically, so every tenth release would go unnoticed.
/// Returns > 0 when [a] is newer than [b].
int compareVersions(String a, String b) {
  final partsA = a.split('.');
  final partsB = b.split('.');
  final length = partsA.length > partsB.length ? partsA.length : partsB.length;
  for (var i = 0; i < length; i++) {
    final na = i < partsA.length ? int.tryParse(partsA[i]) ?? 0 : 0;
    final nb = i < partsB.length ? int.tryParse(partsB[i]) ?? 0 : 0;
    if (na != nb) return na.compareTo(nb);
  }
  return 0;
}

class AppUpdateState {
  /// The newer version on offer, or null when this build is current.
  final String? availableVersion;
  final bool downloading;
  final double progress;

  const AppUpdateState({this.availableVersion, this.downloading = false, this.progress = 0});

  AppUpdateState copyWith({String? availableVersion, bool? downloading, double? progress}) =>
      AppUpdateState(
        availableVersion: availableVersion ?? this.availableVersion,
        downloading: downloading ?? this.downloading,
        progress: progress ?? this.progress,
      );
}

/// Checks for a newer build and, on request, downloads its APK and hands it to
/// Android's package installer, which updates the app in place.
///
/// In place matters: the session token and the API key live in secure storage,
/// which an update leaves alone, so nobody has to sign in again afterwards.
class AppUpdateNotifier extends Notifier<AppUpdateState> {
  @override
  AppUpdateState build() => const AppUpdateState();

  Future<void> checkForUpdate() async {
    try {
      final response = await Dio().get<Map<String, dynamic>>(_manifestUrl);
      final remote = response.data?['version'] as String?;
      if (remote == null) return;

      final info = await PackageInfo.fromPlatform();
      if (compareVersions(remote, info.version) > 0) {
        state = state.copyWith(availableVersion: remote);
      }
    } catch (_) {
      // Silent on purpose. A failed or offline check simply shows no upgrade
      // button; an error banner every time the profile screen opens without a
      // network would be worse than not knowing.
    }
  }

  Future<void> downloadAndInstall() async {
    final version = state.availableVersion;
    if (version == null || state.downloading) return;

    state = state.copyWith(downloading: true, progress: 0);
    try {
      final directory = await getTemporaryDirectory();
      final path = '${directory.path}/strong-fish-v$version.apk';
      await Dio().download(
        _apkUrl(version),
        path,
        onReceiveProgress: (received, total) {
          if (total > 0) state = state.copyWith(progress: received / total);
        },
      );

      final result = await OpenFilex.open(path);
      if (result.type != ResultType.done) {
        throw Exception(result.message);
      }
    } finally {
      state = state.copyWith(downloading: false);
    }
  }
}

final appUpdateProvider = NotifierProvider<AppUpdateNotifier, AppUpdateState>(AppUpdateNotifier.new);
