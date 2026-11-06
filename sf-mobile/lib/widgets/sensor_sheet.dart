import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
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
        child: SingleChildScrollView(
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
            const Divider(height: 24),
            const _CallbackSection(),
          ],
          ),
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

/// The member's callback devices: triggers the phone fires when a set starts
/// (an ESP32 of their own), over a webhook, a WebSocket or MQTT. Listed here
/// beside the Bluetooth sensors, since both answer "what records my bar speed".
/// Add, switch on or off, edit or remove one; the record screen fires every
/// enabled one.
class _CallbackSection extends ConsumerWidget {
  const _CallbackSection();

  Future<void> _reload(WidgetRef ref) async {
    ref.invalidate(recordingCallbacksProvider);
    await ref.read(recordingCallbacksProvider.future);
  }

  // Blank secrets on an update keep the stored ones, so flipping the switch
  // never drops a device's credentials. The messenger is captured before the
  // await so no BuildContext is used across the async gap.
  Future<void> _toggle(BuildContext context, WidgetRef ref, RecordingCallback callback) async {
    final messenger = ScaffoldMessenger.of(context);
    final tError = ref.read(tErrorProvider);
    try {
      await ref.read(apiProvider).setCallback(callback.id, {
        'name': callback.name,
        'type': callback.type,
        'url': callback.url,
        'headers': callback.headers.map((header) => header.toJson()).toList(),
        'username': callback.username,
        'topic': callback.topic,
        'clientId': callback.clientId,
        'qos': callback.qos,
        'token': '',
        'password': '',
        'enabled': !callback.enabled,
      });
      await _reload(ref);
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(tError(error))));
    }
  }

  Future<void> _edit(BuildContext context, WidgetRef ref, {RecordingCallback? callback}) async {
    final messenger = ScaffoldMessenger.of(context);
    final tError = ref.read(tErrorProvider);
    final payload = await showCallbackEditor(context, callback);
    if (payload == null) return;
    try {
      if (callback == null) {
        await ref.read(apiProvider).addCallback(payload);
      } else {
        await ref.read(apiProvider).setCallback(callback.id, payload);
      }
      await _reload(ref);
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(tError(error))));
    }
  }

  Future<void> _delete(BuildContext context, WidgetRef ref, RecordingCallback callback) async {
    final messenger = ScaffoldMessenger.of(context);
    final tError = ref.read(tErrorProvider);
    try {
      await ref.read(apiProvider).deleteCallback(callback.id);
      await _reload(ref);
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(tError(error))));
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final callbacks = ref.watch(recordingCallbacksProvider).valueOrNull ?? const <RecordingCallback>[];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(children: [
          Expanded(
            child: Text(t('callbacks.title'),
                style: TextStyle(color: colors.textMuted, fontSize: 12)),
          ),
          IconButton(
            icon: const Icon(Icons.add, size: 20),
            tooltip: t('callbacks.add'),
            visualDensity: VisualDensity.compact,
            onPressed: () => _edit(context, ref),
          ),
        ]),
        if (callbacks.isEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: 6),
            child: Text(t('callbacks.none'),
                style: TextStyle(color: colors.textMuted, fontSize: 12)),
          )
        else
          for (final callback in callbacks)
            ListTile(
              dense: true,
              contentPadding: EdgeInsets.zero,
              leading: Icon(Icons.wifi_tethering,
                  color: callback.enabled ? colors.success : colors.textMuted),
              title: Text(callback.label, maxLines: 1, overflow: TextOverflow.ellipsis),
              subtitle: Text(
                [
                  t('callbacks.type.${callback.type}'),
                  callback.url,
                  if (!callback.enabled) t('callbacks.disabled'),
                ].join(' · '),
                style: TextStyle(color: colors.textMuted, fontSize: 11),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              trailing: Row(mainAxisSize: MainAxisSize.min, children: [
                Switch(
                  value: callback.enabled,
                  onChanged: (_) => _toggle(context, ref, callback),
                ),
                PopupMenuButton<String>(
                  onSelected: (choice) => choice == 'edit'
                      ? _edit(context, ref, callback: callback)
                      : _delete(context, ref, callback),
                  itemBuilder: (context) => [
                    PopupMenuItem(value: 'edit', child: Text(t('common.edit'))),
                    PopupMenuItem(value: 'delete', child: Text(t('common.delete'))),
                  ],
                ),
              ]),
            ),
      ],
    );
  }
}

/// The add/edit sheet for a callback device: the transport, the URL, and the
/// credentials each kind needs. Returns the payload to save, or null when the
/// member cancels - the caller does the write and the reload.
Future<Map<String, dynamic>?> showCallbackEditor(
    BuildContext context, RecordingCallback? callback) {
  return showDialog<Map<String, dynamic>>(
    context: context,
    builder: (_) => _CallbackEditor(callback: callback),
  );
}

class _CallbackEditor extends ConsumerStatefulWidget {
  final RecordingCallback? callback;
  const _CallbackEditor({this.callback});

  @override
  ConsumerState<_CallbackEditor> createState() => _CallbackEditorState();
}

class _CallbackEditorState extends ConsumerState<_CallbackEditor> {
  late final TextEditingController _name =
      TextEditingController(text: widget.callback?.name ?? '');
  late final TextEditingController _url =
      TextEditingController(text: widget.callback?.url ?? '');
  late final TextEditingController _headers = TextEditingController(
      text: (widget.callback?.headers ?? const [])
          .map((header) => '${header.name}: ${header.value}')
          .join('\n'));
  late final TextEditingController _username =
      TextEditingController(text: widget.callback?.username ?? '');
  late final TextEditingController _topic =
      TextEditingController(text: widget.callback?.topic ?? '');
  late final TextEditingController _clientId =
      TextEditingController(text: widget.callback?.clientId ?? '');
  // The two secrets are write-only: left blank on an edit, the server keeps the
  // stored one, so a field never shows a secret it is not allowed to read.
  final TextEditingController _token = TextEditingController();
  final TextEditingController _password = TextEditingController();

  late String _type = widget.callback?.type.isNotEmpty == true ? widget.callback!.type : 'http';
  late int _qos = widget.callback?.qos ?? 0;
  late bool _enabled = widget.callback?.enabled ?? true;

  @override
  void dispose() {
    _name.dispose();
    _url.dispose();
    _headers.dispose();
    _username.dispose();
    _topic.dispose();
    _clientId.dispose();
    _token.dispose();
    _password.dispose();
    super.dispose();
  }

  // Headers typed as "Name: value" lines, back to the array the API takes.
  List<Map<String, String>> _parseHeaders() {
    final list = <Map<String, String>>[];
    for (final line in _headers.text.split('\n')) {
      final at = line.indexOf(':');
      final name = (at < 0 ? line : line.substring(0, at)).trim();
      if (name.isEmpty) continue;
      list.add({'name': name, 'value': at < 0 ? '' : line.substring(at + 1).trim()});
    }
    return list;
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final ready = _url.text.trim().isNotEmpty;
    final carriesHeaders = _type == 'http' || _type == 'websocket';
    final isMqtt = _type == 'mqtt';
    final urlHint = _type == 'mqtt'
        ? 'mqtt://broker.local:1883'
        : _type == 'websocket'
            ? 'ws://esp32.local/ws'
            : 'http://esp32.local/record';

    return AlertDialog(
      title: Text(t('callbacks.title')),
      content: SingleChildScrollView(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(
            controller: _name,
            decoration: InputDecoration(
              labelText: t('callbacks.name'),
              hintText: t('callbacks.namePlaceholder'),
              isDense: true,
            ),
          ),
          const SizedBox(height: 10),
          DropdownButtonFormField<String>(
            initialValue: _type,
            decoration: InputDecoration(labelText: t('callbacks.transport'), isDense: true),
            items: [
              for (final type in const ['http', 'websocket', 'mqtt'])
                DropdownMenuItem(value: type, child: Text(t('callbacks.type.$type'))),
            ],
            onChanged: (value) => setState(() => _type = value ?? 'http'),
          ),
          const SizedBox(height: 10),
          TextField(
            controller: _url,
            keyboardType: TextInputType.url,
            autocorrect: false,
            onChanged: (_) => setState(() {}),
            decoration: InputDecoration(
              labelText: t('callbacks.url'),
              hintText: urlHint,
              helperText: t('callbacks.urlHelp'),
              helperMaxLines: 3,
              isDense: true,
            ),
          ),
          if (carriesHeaders) ...[
            const SizedBox(height: 10),
            TextField(
              controller: _headers,
              minLines: 1,
              maxLines: 3,
              autocorrect: false,
              decoration: InputDecoration(
                labelText: t('callbacks.headers'),
                hintText: 'X-Device-Key: abc123',
                helperText: t('callbacks.headersHelp'),
                helperMaxLines: 2,
                isDense: true,
              ),
            ),
            const SizedBox(height: 10),
            TextField(
              controller: _token,
              obscureText: true,
              autocorrect: false,
              decoration: InputDecoration(
                labelText: t('callbacks.token'),
                hintText: (widget.callback?.hasToken ?? false) ? t('callbacks.secretSet') : null,
                helperText: (widget.callback?.hasToken ?? false)
                    ? t('callbacks.secretKeep')
                    : t('callbacks.tokenHelp'),
                helperMaxLines: 2,
                isDense: true,
              ),
            ),
          ],
          if (isMqtt) ...[
            const SizedBox(height: 10),
            TextField(
              controller: _topic,
              autocorrect: false,
              decoration: InputDecoration(
                labelText: t('callbacks.topic'),
                hintText: 'strongfish/bar-speed',
                isDense: true,
              ),
            ),
            const SizedBox(height: 10),
            Row(children: [
              Expanded(
                child: TextField(
                  controller: _clientId,
                  autocorrect: false,
                  decoration: InputDecoration(labelText: t('callbacks.clientId'), isDense: true),
                ),
              ),
              const SizedBox(width: 10),
              SizedBox(
                width: 90,
                child: DropdownButtonFormField<int>(
                  initialValue: _qos,
                  decoration: InputDecoration(labelText: t('callbacks.qos'), isDense: true),
                  items: const [
                    DropdownMenuItem(value: 0, child: Text('0')),
                    DropdownMenuItem(value: 1, child: Text('1')),
                    DropdownMenuItem(value: 2, child: Text('2')),
                  ],
                  onChanged: (value) => setState(() => _qos = value ?? 0),
                ),
              ),
            ]),
          ],
          const SizedBox(height: 10),
          TextField(
            controller: _username,
            autocorrect: false,
            decoration: InputDecoration(labelText: t('callbacks.username'), isDense: true),
          ),
          const SizedBox(height: 10),
          TextField(
            controller: _password,
            obscureText: true,
            autocorrect: false,
            decoration: InputDecoration(
              labelText: t('callbacks.password'),
              hintText: (widget.callback?.hasPassword ?? false) ? t('callbacks.secretSet') : null,
              helperText: isMqtt ? t('callbacks.mqttAuthHelp') : t('callbacks.basicAuthHelp'),
              helperMaxLines: 2,
              isDense: true,
            ),
          ),
          const SizedBox(height: 6),
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            dense: true,
            title: Text(t('callbacks.enabled')),
            value: _enabled,
            onChanged: (value) => setState(() => _enabled = value),
          ),
        ]),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(t('common.cancel')),
        ),
        FilledButton(
          onPressed: ready
              ? () => Navigator.of(context).pop({
                    'name': _name.text.trim(),
                    'type': _type,
                    'url': _url.text.trim(),
                    'token': _token.text.trim(),
                    'headers': _parseHeaders(),
                    'username': _username.text.trim(),
                    'password': _password.text,
                    'topic': _topic.text.trim(),
                    'clientId': _clientId.text.trim(),
                    'qos': _qos,
                    'enabled': _enabled,
                  })
              : null,
          child: Text(t('common.save')),
        ),
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
