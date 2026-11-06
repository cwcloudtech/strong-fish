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

/// A sensor seen during a scan: its address, the name it advertises (blank for a
/// device that advertises none), and how strong its signal is - the nearer one
/// is the one on the bar in front of you.
class BleDevice {
  final String id;
  final String name;
  final int rssi;

  const BleDevice({required this.id, required this.name, this.rssi = 0});

  /// What to show when the device advertises no name: its address is better
  /// than nothing to tell two unnamed ones apart.
  String get label => name.trim().isEmpty ? id : name.trim();
}

/// Where a sensor is in the connect dance, so a screen can show "connecting…",
/// "connected" or "disconnected" rather than guessing from whether samples are
/// arriving.
enum BleConnectionState { idle, scanning, connecting, connected, disconnected, failed }

/// A snapshot of the one sensor the app talks to: its state, which device it is,
/// and the reason it failed when it did.
class BarSpeedConnection {
  final BleConnectionState state;
  final String deviceId;
  final String deviceName;
  final String error;

  /// The sensor's battery level, 0..100, or -1 until it has reported one (and
  /// once disconnected). WitMotion reports it a second or two after connecting
  /// and every half-minute after, so it is briefly unknown on a fresh connect.
  final int battery;

  const BarSpeedConnection({
    this.state = BleConnectionState.idle,
    this.deviceId = '',
    this.deviceName = '',
    this.error = '',
    this.battery = -1,
  });

  bool get isConnected => state == BleConnectionState.connected;
  bool get isBusy => state == BleConnectionState.connecting || state == BleConnectionState.scanning;

  /// Whether a battery reading has arrived (so the UI shows it only once real).
  bool get hasBattery => battery >= 0;

  static BleConnectionState _stateOf(String? name) {
    switch (name) {
      case 'scanning':
        return BleConnectionState.scanning;
      case 'connecting':
        return BleConnectionState.connecting;
      case 'connected':
        return BleConnectionState.connected;
      case 'disconnected':
        return BleConnectionState.disconnected;
      case 'failed':
        return BleConnectionState.failed;
      default:
        return BleConnectionState.idle;
    }
  }

  factory BarSpeedConnection.fromMap(Map<dynamic, dynamic> map) => BarSpeedConnection(
        state: _stateOf(map['state'] as String?),
        deviceId: (map['deviceId'] as String?) ?? '',
        deviceName: (map['name'] as String?) ?? '',
        error: (map['error'] as String?) ?? '',
        battery: (map['battery'] as num?)?.toInt() ?? -1,
      );
}

/// Talks to a bar-speed sensor over Bluetooth: scan for what is nearby, connect
/// to one, and read its motion stream - the same arrangement a brand's own app
/// uses (a device you pair once and then record set after set against).
///
/// The transport is native: scanning, connecting and parsing the sensor's BLE
/// frames (a WitMotion-class serial IMU streams on 0xFFE4 under service 0xFFE5)
/// happen in Kotlin and Swift behind these channels, because Dart cannot speak
/// BLE. This file is the **contract** the UI codes against; the per-platform
/// native side is written and validated against real hardware and cannot be
/// exercised in CI.
///
/// One sensor at a time, so this is a singleton ([instance]): the connection is
/// app-wide, and the device list and the record screen share it rather than each
/// opening their own.
class BarSpeedCapture {
  BarSpeedCapture._();
  static final BarSpeedCapture instance = BarSpeedCapture._();

  static const MethodChannel _control = MethodChannel('sf/bar_speed_capture');
  static const EventChannel _devicesChannel = EventChannel('sf/bar_speed_capture/devices');
  static const EventChannel _stateChannel = EventChannel('sf/bar_speed_capture/state');
  static const EventChannel _samplesChannel = EventChannel('sf/bar_speed_capture/samples');

  Stream<List<BleDevice>>? _devices;
  Stream<BarSpeedConnection>? _state;
  Stream<BarSpeedSample>? _samples;

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

  /// The sensors seen so far in the running scan, re-emitted as a whole list
  /// every time a new one appears. Call [startScan] to begin; the stream is
  /// empty until then.
  Stream<List<BleDevice>> devices() {
    return _devices ??= _devicesChannel.receiveBroadcastStream().map((event) {
      final list = (event as List).cast<dynamic>();
      return [
        for (final entry in list)
          if (entry is Map)
            BleDevice(
              id: (entry['id'] as String?) ?? '',
              name: (entry['name'] as String?) ?? '',
              rssi: (entry['rssi'] as num?)?.toInt() ?? 0,
            ),
      ];
    });
  }

  /// The connection's state, pushed whenever it changes. Shared: every screen
  /// that watches it sees the one sensor the app is talking to.
  Stream<BarSpeedConnection> connectionState() {
    return _state ??= _stateChannel
        .receiveBroadcastStream()
        .map((event) => BarSpeedConnection.fromMap((event as Map).cast<dynamic, dynamic>()));
  }

  /// The motion stream from the connected sensor. It flows whenever a sensor is
  /// connected; a recording is simply a window of it the caller keeps.
  Stream<BarSpeedSample> samples() {
    return _samples ??= _samplesChannel.receiveBroadcastStream().map((event) {
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

  /// Begins scanning for nearby sensors. Broad on purpose - a sensor that does
  /// not advertise its service UUID (many do not) would never appear behind a
  /// service filter, which is the usual reason one "will not pair".
  Future<void> startScan() async {
    try {
      await _control.invokeMethod('startScan');
    } catch (_) {}
  }

  Future<void> stopScan() async {
    try {
      await _control.invokeMethod('stopScan');
    } catch (_) {}
  }

  /// Connects to the sensor with this address, enabling its motion stream.
  Future<void> connect(String deviceId) async {
    await _control.invokeMethod('connect', {'deviceId': deviceId});
  }

  /// Drops the current connection. Safe to call when nothing is connected.
  Future<void> disconnect() async {
    try {
      await _control.invokeMethod('disconnect');
    } catch (_) {}
  }
}
