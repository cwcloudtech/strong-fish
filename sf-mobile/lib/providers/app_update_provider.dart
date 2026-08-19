import 'dart:io';

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
  defaultValue: 'https://www.strong-fish.com',
);
/// Where the build is published, for the case where the API cannot say.
///
/// The www matters: CI publishes the APK alongside the frontend, whose address
/// is www.strong-fish.com. A guess at the apex domain here is what used to
/// download an error page and hand it to the installer.
const _downloadBaseUrl = String.fromEnvironment(
  'SF_DOWNLOAD_URL',
  defaultValue: 'https://www.strong-fish.com',
);

String get _mobileAppUrl => '$_updateBaseUrl/v1/mobile-app';
String get _manifestUrl => '$_updateBaseUrl/v1/manifest';

String _fallbackApkUrl(String version) => '$_downloadBaseUrl/strong-fish-v$version.apk';

/// An APK is a ZIP, so it starts with ZIP's local file header. Checking those
/// four bytes is what tells a real download apart from an error page saved
/// under an .apk name - which is how a wrong URL used to surface, as Android
/// refusing the file with "there's a problem with the app file" long after the
/// actual mistake.
const _zipMagic = [0x50, 0x4B, 0x03, 0x04];

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

  /// Where to download it, as the server named it. Not built here from a
  /// hardcoded host: the APK is served by the frontend, whose address the
  /// deployment configures, and a second guess at it in this file is exactly
  /// what silently broke - it said strong-fish.com while the build was
  /// published at www.strong-fish.com.
  final String? downloadUrl;
  final bool downloading;
  final double progress;

  const AppUpdateState({
    this.availableVersion,
    this.downloadUrl,
    this.downloading = false,
    this.progress = 0,
  });

  AppUpdateState copyWith({
    String? availableVersion,
    String? downloadUrl,
    bool? downloading,
    double? progress,
  }) =>
      AppUpdateState(
        availableVersion: availableVersion ?? this.availableVersion,
        downloadUrl: downloadUrl ?? this.downloadUrl,
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
      // The API reports both the published version and the URL it is published
      // at, built from the same settings the web app's download link uses. That
      // is the point: one source of truth, so the two cannot drift.
      String? remote;
      String? url;

      try {
        final response = await Dio().get<Map<String, dynamic>>(_mobileAppUrl);
        remote = response.data?['version'] as String?;
        url = response.data?['url'] as String?;
      } on DioException {
        // An older deployment has no /v1/mobile-app, and every deployment has
        // the manifest. Falling back to it - and to the published URL pattern -
        // is what keeps the check working against a backend that predates the
        // endpoint; requiring the endpoint outright stopped updates being
        // detected at all.
        final manifest = await Dio().get<Map<String, dynamic>>(_manifestUrl);
        remote = manifest.data?['version'] as String?;
      }

      if (remote == null || remote.isEmpty) return;
      // The API's answer is preferred - it is built from the deployment's own
      // settings - and the pattern is only the last resort.
      url = (url != null && url.isNotEmpty) ? url : _fallbackApkUrl(remote);

      final info = await PackageInfo.fromPlatform();
      if (compareVersions(remote, info.version) > 0) {
        state = state.copyWith(availableVersion: remote, downloadUrl: url);
      }
    } catch (_) {
      // Silent on purpose. A failed or offline check simply shows no upgrade
      // button; an error banner every time the profile screen opens without a
      // network would be worse than not knowing.
    }
  }

  Future<void> downloadAndInstall() async {
    final version = state.availableVersion;
    final url = state.downloadUrl;
    if (version == null || url == null || state.downloading) return;

    state = state.copyWith(downloading: true, progress: 0);
    try {
      final directory = await getTemporaryDirectory();
      final file = File('${directory.path}/strong-fish-v$version.apk');
      await Dio().download(
        url,
        file.path,
        onReceiveProgress: (received, total) {
          if (total > 0) state = state.copyWith(progress: received / total);
        },
      );

      // A 200 is not proof of an APK. A misconfigured host answers a missing
      // file with an HTML page, which downloads perfectly happily and then
      // fails at the installer with a message pointing nowhere near the cause.
      // Checking here fails at the step that is actually wrong.
      final head = await file.openRead(0, _zipMagic.length).expand((bytes) => bytes).toList();
      if (!_listEquals(head, _zipMagic)) {
        await file.delete();
        throw Exception('The download from $url is not an installable package');
      }

      final result = await OpenFilex.open(file.path);
      if (result.type != ResultType.done) {
        throw Exception(result.message);
      }
    } finally {
      state = state.copyWith(downloading: false);
    }
  }
}

final appUpdateProvider = NotifierProvider<AppUpdateNotifier, AppUpdateState>(AppUpdateNotifier.new);

bool _listEquals(List<int> a, List<int> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}
