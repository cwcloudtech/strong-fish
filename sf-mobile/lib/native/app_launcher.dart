import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/services.dart';

const _channel = MethodChannel('sf/app_launcher');

/// Launches an installed app by its package name in any user profile on the
/// device - the personal one or a managed work profile - via the native
/// LauncherApps path (see MainActivity). Returns true when it was launched.
///
/// Android only: a no-op returning false on iOS, web, or an empty package.
Future<bool> launchAppAnyProfile(String package) async {
  if (kIsWeb || package.isEmpty || !Platform.isAndroid) return false;
  try {
    return await _channel
            .invokeMethod<bool>('launchPackageAnyProfile', {'package': package}) ??
        false;
  } catch (_) {
    return false;
  }
}
