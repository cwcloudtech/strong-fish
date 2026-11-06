import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../native/bar_speed_capture.dart';
import '../providers/providers.dart';
import '../theme.dart';

/// Opens the sensor sheet: scan for nearby bar-speed sensors, connect to one,
/// and see whether it is connected - the same thing a brand's own app does, so
/// a sensor you pair once stays paired across the sets you record.
Future<void> showSensorSheet(BuildContext context) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => const SensorSheet(),
  );
}

/// The device list: what is nearby, which one is connected, and the buttons to
/// connect or drop it. Scanning runs only while this is open.
class SensorSheet extends ConsumerStatefulWidget {
  const SensorSheet({super.key});

  @override
  ConsumerState<SensorSheet> createState() => _SensorSheetState();
}

class _SensorSheetState extends ConsumerState<SensorSheet> {
  final BarSpeedCapture _capture = BarSpeedCapture.instance;
  String _connecting = '';

  @override
  void initState() {
    super.initState();
    _capture.startScan();
  }

  @override
  void dispose() {
    // The scan stops with the sheet; the connection does not - that is the
    // point of pairing once and recording many sets.
    _capture.stopScan();
    super.dispose();
  }

  Future<void> _connect(String id) async {
    setState(() => _connecting = id);
    try {
      await _capture.connect(id);
    } catch (_) {
      // The connection state stream carries the failure to the UI.
    } finally {
      if (mounted) setState(() => _connecting = '');
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final connection =
        ref.watch(barSpeedConnectionProvider).valueOrNull ?? const BarSpeedConnection();
    final devices = ref.watch(barSpeedDevicesProvider).valueOrNull ?? const <BleDevice>[];

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 4, 16, 16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(t('devices.sensor.title'),
                      style: Theme.of(context).textTheme.titleMedium),
                ),
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ],
            ),
            const SizedBox(height: 12),
            _StatusBanner(connection: connection),
            if (connection.isConnected) ...[
              const SizedBox(height: 10),
              OutlinedButton.icon(
                onPressed: () => _capture.disconnect(),
                icon: const Icon(Icons.bluetooth_disabled),
                label: Text(t('devices.sensor.disconnect')),
              ),
            ],
            const Divider(height: 24),
            Text(t('devices.sensor.nearby'),
                style: TextStyle(color: colors.textMuted, fontSize: 12)),
            const SizedBox(height: 6),
            if (devices.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 18),
                child: Text(t('devices.sensor.noDevices'),
                    style: TextStyle(color: colors.textMuted)),
              )
            else
              ConstrainedBox(
                constraints: const BoxConstraints(maxHeight: 320),
                child: ListView(
                  shrinkWrap: true,
                  children: [
                    for (final device in devices)
                      _DeviceRow(
                        device: device,
                        connected: connection.isConnected && connection.deviceId == device.id,
                        connecting: _connecting == device.id ||
                            (connection.state == BleConnectionState.connecting &&
                                connection.deviceId == device.id),
                        onConnect: () => _connect(device.id),
                      ),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// A line that says, in words, where the one sensor is: connected to which
/// device, connecting, or not connected - with the failure reason when it
/// failed.
class _StatusBanner extends ConsumerWidget {
  final BarSpeedConnection connection;
  const _StatusBanner({required this.connection});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);

    late final IconData icon;
    late final Color tint;
    late final String text;
    switch (connection.state) {
      case BleConnectionState.connected:
        icon = Icons.bluetooth_connected;
        tint = colors.success;
        text = t('devices.sensor.connected',
            {'name': connection.deviceName.isEmpty ? connection.deviceId : connection.deviceName});
        break;
      case BleConnectionState.connecting:
        icon = Icons.bluetooth_searching;
        tint = colors.warning;
        text = t('devices.sensor.connecting');
        break;
      case BleConnectionState.failed:
        icon = Icons.error_outline;
        tint = colors.danger;
        text = connection.error.isNotEmpty
            ? connection.error
            : t('devices.sensor.failed');
        break;
      default:
        icon = Icons.bluetooth_disabled;
        tint = colors.textMuted;
        text = t('devices.sensor.disconnected');
    }

    return Row(
      children: [
        Icon(icon, color: tint, size: 20),
        const SizedBox(width: 8),
        Expanded(child: Text(text, style: TextStyle(color: tint))),
      ],
    );
  }
}

/// One scanned sensor: its name (or address), its signal, and a connect button -
/// greyed to a spinner while it is the one connecting.
class _DeviceRow extends ConsumerWidget {
  final BleDevice device;
  final bool connected;
  final bool connecting;
  final VoidCallback onConnect;

  const _DeviceRow({
    required this.device,
    required this.connected,
    required this.connecting,
    required this.onConnect,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);

    return ListTile(
      dense: true,
      contentPadding: EdgeInsets.zero,
      leading: Icon(Icons.sensors, color: connected ? colors.success : colors.textMuted),
      title: Text(device.label),
      subtitle: device.rssi != 0
          ? Text(t('devices.sensor.rssi', {'rssi': '${device.rssi}'}),
              style: TextStyle(color: colors.textMuted, fontSize: 11))
          : null,
      trailing: connected
          ? Icon(Icons.check_circle, color: colors.success)
          : connecting
              ? const SizedBox(
                  width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
              : TextButton(onPressed: onConnect, child: Text(t('devices.sensor.connect'))),
    );
  }
}
