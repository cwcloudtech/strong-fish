import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../native/bar_speed_capture.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/sensor_sheet.dart';

const _movements = ['squat', 'bench', 'deadlift'];

/// Records a set from a connected bar-speed sensor: pick the lift and the load,
/// hold through the set, and the raw stream is posted to StrongFish's own engine
/// (which computes and stores it).
///
/// The sensor is connected once in the sensor sheet and shared app-wide (see
/// [BarSpeedCapture]); this screen reads its live motion stream and keeps the
/// window between Start and Stop. A set opened from a session arrives with its
/// lift and prescribed load already filled, which the member can change.
class RecordBarSpeedScreen extends ConsumerStatefulWidget {
  final String initialMovement;
  final double initialLoad;

  const RecordBarSpeedScreen({
    super.key,
    this.initialMovement = 'deadlift',
    this.initialLoad = 0,
  });

  @override
  ConsumerState<RecordBarSpeedScreen> createState() => _RecordBarSpeedScreenState();
}

class _RecordBarSpeedScreenState extends ConsumerState<RecordBarSpeedScreen> {
  final BarSpeedCapture _capture = BarSpeedCapture.instance;
  final TextEditingController _load = TextEditingController();
  late String _movement =
      _movements.contains(widget.initialMovement) ? widget.initialMovement : 'deadlift';

  StreamSubscription<BarSpeedSample>? _sub;
  final List<Map<String, double>> _samples = [];
  double? _baseT;
  bool _recording = false;
  bool _busy = false;
  StanceSet? _result;
  String _error = '';

  @override
  void initState() {
    super.initState();
    if (widget.initialLoad > 0) {
      _load.text = widget.initialLoad % 1 == 0
          ? widget.initialLoad.toStringAsFixed(0)
          : widget.initialLoad.toStringAsFixed(1);
    }
    // One subscription for the screen's life: the sensor streams whenever it is
    // connected, and a recording is just the window we keep between Start and
    // Stop.
    _sub = _capture.samples().listen((sample) {
      if (!_recording) return;
      _baseT ??= sample.t;
      final json = sample.toJson();
      json['t'] = sample.t - _baseT!;
      _samples.add(json);
      if (mounted) setState(() {}); // refresh the live sample count
    });
  }

  @override
  void dispose() {
    _sub?.cancel();
    _load.dispose();
    super.dispose();
  }

  void _start() {
    setState(() {
      _samples.clear();
      _baseT = null;
      _result = null;
      _error = '';
      _recording = true;
    });
  }

  Future<void> _stop() async {
    setState(() {
      _recording = false;
      _busy = true;
    });
    try {
      final load = double.tryParse(_load.text.replaceAll(',', '.')) ?? 0;
      final set = await ref.read(apiProvider).analyzeBarSpeed(
            movement: _movement,
            load: load,
            samples: List.of(_samples),
          );
      if (mounted) setState(() => _result = set);
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final connection =
        ref.watch(barSpeedConnectionProvider).valueOrNull ?? const BarSpeedConnection();
    final connected = connection.isConnected;

    return Scaffold(
      appBar: AppBar(
        title: Text(t('devices.barSpeed.recordTitle')),
        actions: [
          IconButton(
            tooltip: t('devices.sensor.title'),
            icon: Icon(connected ? Icons.bluetooth_connected : Icons.bluetooth),
            onPressed: () => showSensorSheet(context),
          ),
        ],
      ),
      body: ListView(padding: const EdgeInsets.all(16), children: [
        // Nothing records without a sensor connected: say so, and offer the
        // one tap that fixes it, rather than a dead Start button.
        if (!connected)
          Card(
            color: colors.backgroundMuted,
            child: Padding(
              padding: const EdgeInsets.all(14),
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(t('devices.sensor.disconnected'),
                    style: const TextStyle(fontWeight: FontWeight.w600)),
                const SizedBox(height: 4),
                Text(t('devices.sensor.connectHint'), style: TextStyle(color: colors.textMuted)),
                const SizedBox(height: 10),
                FilledButton.icon(
                  onPressed: () => showSensorSheet(context),
                  icon: const Icon(Icons.bluetooth_searching),
                  label: Text(t('devices.sensor.connect')),
                ),
              ]),
            ),
          ),
        if (connected)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: Row(children: [
              Icon(Icons.bluetooth_connected, size: 18, color: colors.success),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  t('devices.sensor.connected', {
                    'name': connection.deviceName.isEmpty
                        ? connection.deviceId
                        : connection.deviceName
                  }),
                  style: TextStyle(color: colors.success),
                ),
              ),
            ]),
          ),
        Wrap(spacing: 8, children: [
          for (final movement in _movements)
            ChoiceChip(
              selected: _movement == movement,
              label: Text(t('devices.lift.$movement')),
              onSelected: _recording ? null : (_) => setState(() => _movement = movement),
            ),
        ]),
        const SizedBox(height: 14),
        TextField(
          controller: _load,
          enabled: !_recording && !_busy,
          keyboardType: const TextInputType.numberWithOptions(decimal: true),
          decoration: InputDecoration(labelText: t('devices.barSpeed.recordLoad')),
        ),
        const SizedBox(height: 18),
        FilledButton.icon(
          onPressed: (!connected || _busy) ? null : (_recording ? _stop : _start),
          icon: Icon(_recording ? Icons.stop : Icons.fiber_manual_record),
          label: Text(_recording
              ? t('devices.barSpeed.recordStop')
              : t('devices.barSpeed.recordStart')),
        ),
        if (_recording)
          Padding(
            padding: const EdgeInsets.only(top: 10),
            child: Text(
              t('devices.barSpeed.recordCapturing', {'count': '${_samples.length}'}),
              style: TextStyle(color: colors.textMuted),
            ),
          ),
        if (_busy)
          const Padding(
            padding: EdgeInsets.all(20),
            child: Center(child: CircularProgressIndicator()),
          ),
        if (_error.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Text(_error, style: TextStyle(color: colors.danger)),
          ),
        if (_result != null) ...[
          const SizedBox(height: 20),
          Text(t('devices.barSpeed.recordResult'),
              style: const TextStyle(fontWeight: FontWeight.w600)),
          const SizedBox(height: 8),
          _ResultView(set: _result!),
        ],
      ]),
    );
  }
}

/// The computed set: its headline numbers and the per-rep speeds.
class _ResultView extends ConsumerWidget {
  final StanceSet set;
  const _ResultView({required this.set});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final lines = <String>[
      '${t('devices.lift.${set.lift}')} · ${set.reps} × ${set.weightKg.toStringAsFixed(0)} kg',
      if (set.meanVelocity > 0)
        '${t('devices.barSpeed.mean')}: ${set.meanVelocity.toStringAsFixed(2)} m/s',
      if (set.peakVelocity > 0)
        '${t('devices.barSpeed.peak')}: ${set.peakVelocity.toStringAsFixed(2)} m/s',
      if (set.predictedRpe > 0)
        t('devices.barSpeed.rpeReal', {'real': set.predictedRpe.toStringAsFixed(1)}),
    ];
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      for (final line in lines)
        Padding(padding: const EdgeInsets.only(bottom: 2), child: Text(line)),
      if (set.repVelocities.isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(top: 6),
          child: Text(
            set.repVelocities.map((v) => v.toStringAsFixed(2)).join(' · '),
            style: TextStyle(color: colors.textMuted, fontFamily: 'monospace'),
          ),
        ),
    ]);
  }
}
