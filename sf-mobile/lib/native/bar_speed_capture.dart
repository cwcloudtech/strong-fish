import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// One IMU tick from a bar-speed sensor: linear acceleration in m/s² on three
/// axes, and an optional orientation quaternion. The shape the velocity
/// endpoint takes (ServicesApi.analyzeBarSpeed).
class BarSpeedSample {
  final double t;
  final double ax;
  final double ay;
  final double az;
  final double qw;
  final double qx;
  final double qy;
  final double qz;

  const BarSpeedSample({
    required this.t,
    required this.ax,
    required this.ay,
    required this.az,
    this.qw = 0,
    this.qx = 0,
    this.qy = 0,
    this.qz = 0,
  });

  Map<String, double> toJson() => {
        't': t,
        'ax': ax,
        'ay': ay,
        'az': az,
        if (qw != 0 || qx != 0 || qy != 0 || qz != 0) ...{
          'qw': qw,
          'qx': qx,
          'qy': qy,
          'qz': qz,
        },
      };
}

/// Captures a set from a Bluetooth bar-speed sensor and hands back its samples,
/// to be posted to the velocity endpoint.
///
/// The transport is native: scanning, connecting and parsing the sensor's BLE
/// frames (a WitMotion-class serial IMU exposes its stream on the 0xFFE4/FFE5/
/// FFE9 characteristics) happen in Kotlin and Swift behind these channels,
/// because Dart cannot speak BLE. This file is the **contract** the UI codes
/// against; the per-platform native implementation is written and validated
/// against real hardware (it cannot be exercised in CI).
///
/// Until a native implementation is present, [isAvailable] returns false, so a
/// screen can fall back to another source rather than fail.
class BarSpeedCapture {
  static const MethodChannel _control = MethodChannel('sf/bar_speed_capture');
  static const EventChannel _stream = EventChannel('sf/bar_speed_capture/samples');

  /// Whether a native capture implementation is present on this platform. False
  /// on web, and false until the native side is wired, so the caller can degrade
  /// gracefully.
  static Future<bool> isAvailable() async {
    if (kIsWeb) return false;
    try {
      return await _control.invokeMethod<bool>('isAvailable') ?? false;
    } catch (_) {
      return false;
    }
  }

  /// Starts a capture and streams samples until [stop] is called. Each native
  /// event is a map of the sample's fields, mapped here to a [BarSpeedSample].
  Stream<BarSpeedSample> start() {
    _control.invokeMethod('start');
    return _stream.receiveBroadcastStream().map((event) {
      final map = (event as Map).cast<String, dynamic>();
      double at(String key) => (map[key] as num?)?.toDouble() ?? 0;
      return BarSpeedSample(
        t: at('t'),
        ax: at('ax'),
        ay: at('ay'),
        az: at('az'),
        qw: at('qw'),
        qx: at('qx'),
        qy: at('qy'),
        qz: at('qz'),
      );
    });
  }

  /// Stops the capture. Safe to call when nothing is running.
  Future<void> stop() async {
    try {
      await _control.invokeMethod('stop');
    } catch (_) {}
  }
}
