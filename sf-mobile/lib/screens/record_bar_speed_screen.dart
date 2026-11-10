import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../native/bar_speed_capture.dart';
import '../providers/providers.dart';
import '../services/callback_trigger.dart';
import '../theme.dart';
import '../utils/set_metadata.dart';
import '../widgets/battery_badge.dart';
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
  // When opened from a training-session set, these say which set to write the
  // result back to, so a recorded set fills in its reps, load and measured RPE
  // rather than being entered twice. Blank when recording stands alone.
  final String assignmentId;
  final String setId;
  // Pre-filled from a training session's prescription (auto-detected), the
  // default warm-up / no-tempo otherwise. The member confirms both before
  // recording.
  final String initialSetType;
  final bool initialTempoVariation;

  const RecordBarSpeedScreen({
    super.key,
    this.initialMovement = 'squat',
    this.initialLoad = 0,
    this.assignmentId = '',
    this.setId = '',
    this.initialSetType = 'warmup',
    this.initialTempoVariation = false,
  });

  @override
  ConsumerState<RecordBarSpeedScreen> createState() => _RecordBarSpeedScreenState();
}

class _RecordBarSpeedScreenState extends ConsumerState<RecordBarSpeedScreen> {
  final BarSpeedCapture _capture = BarSpeedCapture.instance;
  final TextEditingController _load = TextEditingController();
  late String _movement =
      _movements.contains(widget.initialMovement) ? widget.initialMovement : 'squat';
  late String _setType = normalizeSetType(widget.initialSetType);
  late bool _tempo = widget.initialTempoVariation;

  StreamSubscription<BarSpeedSample>? _sub;
  final List<Map<String, double>> _samples = [];
  double? _baseT;
  bool _recording = false;
  bool _busy = false;
  StanceSet? _result;
  // The reviewed rep count (correcting what the engine detected) and whether the
  // reviewed set has been written back to its training-session set yet.
  final TextEditingController _reps = TextEditingController();
  bool _savedToSet = false;
  String _error = '';

  // The troubleshoot console: the most recent raw samples, shown live when the
  // bug icon is on, so a member can see whether the sensor is actually
  // streaming - and what it is streaming - rather than guessing from a result.
  bool _showDebug = false;
  final List<BarSpeedSample> _debugSamples = [];
  static const int _debugMax = 14;
  // The stream runs at ~200 Hz; the console is repainted at most this often so
  // it stays readable and cheap.
  DateTime _lastDebugPaint = DateTime.fromMillisecondsSinceEpoch(0);

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
      if (_recording) {
        _baseT ??= sample.t;
        final json = sample.toJson();
        json['t'] = sample.t - _baseT!;
        _samples.add(json);
      }
      // The console reads live even when not recording, so a connection can be
      // checked before a set. Repainted on a throttle, the capture count on
      // each sample as before.
      if (_showDebug) {
        _debugSamples.add(sample);
        while (_debugSamples.length > _debugMax) {
          _debugSamples.removeAt(0);
        }
      }
      if (!mounted) return;
      if (_recording) {
        setState(() {}); // the live rep count
      } else if (_showDebug) {
        final now = DateTime.now();
        if (now.difference(_lastDebugPaint).inMilliseconds >= 100) {
          _lastDebugPaint = now;
          setState(() {});
        }
      }
    });
  }

  @override
  void dispose() {
    _sub?.cancel();
    _load.dispose();
    _reps.dispose();
    super.dispose();
  }

  void _start() {
    // Signal the member's own device(s) too, if they set any up: the callback
    // fires alongside the local capture, so an ESP32 records the same set.
    _fireCallbacks('start');
    setState(() {
      _samples.clear();
      _baseT = null;
      _result = null;
      _error = '';
      _recording = true;
    });
  }

  Future<void> _stop() async {
    setState(() => _recording = false);
    _fireCallbacks('stop');

    // Analyse whenever the sensor actually streamed samples - even if the BLE
    // link reads as dropped at the instant Stop is pressed, which it often does.
    // The only case with nothing to compute is a callback-only recording, where
    // the member's own device captured the set and no samples reached the phone.
    // (The earlier `!connected` guard here swallowed the whole set - no result
    // shown, nothing persisted - whenever the link blipped on stop.)
    if (_samples.isEmpty) return;

    setState(() => _busy = true);
    try {
      final load = double.tryParse(_load.text.replaceAll(',', '.')) ?? 0;
      final set = await ref.read(apiProvider).analyzeBarSpeed(
            movement: _movement,
            load: load,
            samples: List.of(_samples),
            setType: _setType,
            tempoVariation: _tempo,
          );
      if (!mounted) return;
      final reps = set.reps > 0 ? set.reps : set.repVelocities.length;
      setState(() {
        _result = set;
        _reps.text = '$reps';
        _savedToSet = false;
      });
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Writes the reviewed set back to the training-session set it was opened
  /// from: the rep count the member confirmed (correcting what the engine
  /// detected), the load that was on the bar and the measured RPE, marked done.
  /// Explicit, on the Save button - the member sees the numbers first and fixes
  /// the rep count before anything is logged. Standalone recordings have no set
  /// to write to; the recording itself is already kept by the analyse call.
  Future<void> _saveToSet() async {
    final set = _result;
    if (set == null || widget.assignmentId.isEmpty || widget.setId.isEmpty) return;
    final messenger = ScaffoldMessenger.of(context);
    final t = ref.read(tProvider);
    // The member's corrected count, falling back to what was detected.
    final reps = int.tryParse(_reps.text.trim()) ??
        (set.reps > 0 ? set.reps : set.repVelocities.length);
    setState(() => _busy = true);
    try {
      // Only the vbt subtree is written - the sensor's own load, reps and
      // velocities. The member's Load column and rep count are theirs and are
      // never touched by a recording; the two are shown side by side.
      await ref.read(apiProvider).logSetVBT(
            widget.assignmentId,
            widget.setId,
            {
              'reps': reps,
              if (set.weightKg > 0) 'weightKg': set.weightKg,
              if (set.meanVelocity > 0) 'meanVelocity': set.meanVelocity,
              if (set.peakVelocity > 0) 'peakVelocity': set.peakVelocity,
              if (_velocityLoss(set.repVelocities) > 0)
                'velocityLoss': _velocityLoss(set.repVelocities),
              if (set.repVelocities.isNotEmpty) 'repVelocities': set.repVelocities,
              'recordedAt': DateTime.now().toUtc().toIso8601String(),
            },
          );
      if (!mounted) return;
      setState(() => _savedToSet = true);
      // A real notification of what was saved, in terms of bar speed: the reps
      // kept, the mean concentric speed, and the measured RPE - whichever of
      // them the capture produced - rather than a bare "saved".
      final parts = <String>[
        t('devices.barSpeed.detectedReps', {'count': '$reps'}),
        if (set.meanVelocity > 0) '${set.meanVelocity.toStringAsFixed(2)} m/s',
        if (set.predictedRpe > 0) t('devices.barSpeed.rpeReal', {'real': set.predictedRpe.toStringAsFixed(1)}),
      ];
      messenger.showSnackBar(
          SnackBar(content: Text(t('devices.barSpeed.savedSummary', {'detail': parts.join(' · ')}))));
    } catch (error) {
      if (mounted) messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Fires the lift and load to the member's enabled callback device(s) - over
  /// a webhook, a WebSocket or MQTT. Fire and forget: a device being off must
  /// not hold up the set on the phone, so this is not awaited by Start/Stop - it
  /// reports its own outcome in a snackbar.
  Future<void> _fireCallbacks(String action) async {
    final callbacks = ref.read(recordingCallbacksProvider).valueOrNull ?? const <RecordingCallback>[];
    final enabled = callbacks.where((c) => c.enabled && c.url.trim().isNotEmpty).toList();
    if (enabled.isEmpty) return;

    final load = double.tryParse(_load.text.replaceAll(',', '.')) ?? 0;
    final results = await CallbackTrigger.instance.fire(
      enabled,
      BarSpeedTrigger(lift: _movement, load: load, action: action),
    );
    if (!mounted) return;

    final t = ref.read(tProvider);
    final messenger = ScaffoldMessenger.of(context);
    final failed = results.where((result) => !result.ok).toList();
    if (failed.isNotEmpty) {
      messenger.showSnackBar(SnackBar(
          content: Text(t('devices.barSpeed.triggerFailed', {'name': failed.first.callback.label}))));
    } else if (action == 'start') {
      // One confirmation, on start: the stop signal is silent unless it fails.
      messenger.showSnackBar(SnackBar(
          content: Text(t('devices.barSpeed.triggered', {'count': '${results.length}'}))));
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final connection =
        ref.watch(barSpeedConnectionProvider).valueOrNull ?? const BarSpeedConnection();
    final connected = connection.isConnected;
    // A callback-only recording is allowed too: with a device of their own set
    // up, Start triggers it even when no sensor is connected to the phone.
    final callbacks = ref.watch(recordingCallbacksProvider).valueOrNull ?? const <RecordingCallback>[];
    final hasCallbacks = callbacks.any((c) => c.enabled && c.url.trim().isNotEmpty);
    final canStart = connected || hasCallbacks;

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
        // one tap that fixes it, rather than a dead Start button - unless a
        // callback device is set up, which can record on its own.
        if (!connected && !hasCallbacks)
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
        // No sensor, but a callback device is set up: say what Start will do.
        if (!connected && hasCallbacks)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: Row(children: [
              Icon(Icons.sensors, size: 18, color: colors.textMuted),
              const SizedBox(width: 6),
              Expanded(
                child: Text(t('devices.barSpeed.triggerOnly'),
                    style: TextStyle(color: colors.textMuted)),
              ),
            ]),
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
              if (connection.hasBattery) BatteryBadge(percent: connection.battery),
              // Troubleshoot: show the raw sensor values live in a console.
              IconButton(
                onPressed: () => setState(() {
                  _showDebug = !_showDebug;
                  if (!_showDebug) _debugSamples.clear();
                }),
                tooltip: t('devices.barSpeed.troubleshoot'),
                iconSize: 20,
                visualDensity: VisualDensity.compact,
                color: _showDebug ? colors.primary : colors.textMuted,
                icon: const Icon(Icons.bug_report_outlined),
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
        const SizedBox(height: 12),
        Row(children: [
          Expanded(
            child: DropdownButtonFormField<String>(
              initialValue: _setType,
              isExpanded: true,
              decoration: InputDecoration(labelText: t('devices.barSpeed.setType')),
              items: [
                for (final type in barSpeedSetTypes)
                  DropdownMenuItem(
                    value: type,
                    child: Text(t('devices.barSpeed.setType_$type')),
                  ),
              ],
              onChanged: (_recording || _busy)
                  ? null
                  : (value) => setState(() => _setType = value ?? 'warmup'),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: DropdownButtonFormField<bool>(
              initialValue: _tempo,
              isExpanded: true,
              decoration:
                  InputDecoration(labelText: t('devices.barSpeed.tempoVariation')),
              items: [
                DropdownMenuItem(value: false, child: Text(t('common.no'))),
                DropdownMenuItem(value: true, child: Text(t('common.yes'))),
              ],
              onChanged: (_recording || _busy)
                  ? null
                  : (value) => setState(() => _tempo = value ?? false),
            ),
          ),
        ]),
        const SizedBox(height: 18),
        FilledButton.icon(
          onPressed: (!canStart || _busy) ? null : (_recording ? _stop : _start),
          icon: Icon(_recording ? Icons.stop : Icons.fiber_manual_record),
          label: Text(_recording
              ? t('devices.barSpeed.recordStop')
              : t('devices.barSpeed.recordStart')),
        ),
        if (_recording && connected)
          Padding(
            padding: const EdgeInsets.only(top: 10),
            child: Text(
              t('devices.barSpeed.recordCapturing', {'count': '${_samples.length}'}),
              style: TextStyle(color: colors.textMuted),
            ),
          ),
        if (_showDebug)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: _DebugConsole(samples: _debugSamples),
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
          // Review before it is logged: the engine's rep count is a starting
          // point the member can correct, then Save writes it to the session set.
          if (widget.assignmentId.isNotEmpty && widget.setId.isNotEmpty) ...[
            const SizedBox(height: 12),
            Row(children: [
              SizedBox(
                width: 120,
                child: TextField(
                  controller: _reps,
                  enabled: !_busy,
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(labelText: t('devices.barSpeed.editReps')),
                ),
              ),
              const Spacer(),
              FilledButton.icon(
                onPressed: _busy ? null : _saveToSet,
                icon: Icon(_savedToSet ? Icons.check : Icons.save),
                label: Text(_savedToSet
                    ? t('devices.barSpeed.savedToSet')
                    : t('devices.barSpeed.saveToSet')),
              ),
            ]),
          ],
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
    // The number of reps the engine detected - the per-rep speeds are the proof
    // of it, so when the set-level count is missing, their length is the count.
    final reps = set.reps > 0 ? set.reps : set.repVelocities.length;
    final lines = <String>[
      '${t('devices.lift.${set.lift}')} · ${t('devices.barSpeed.detectedReps', {'count': '$reps'})}'
          '${set.weightKg > 0 ? ' × ${set.weightKg.toStringAsFixed(0)} kg' : ''}',
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

/// A live window on the raw sensor stream, for troubleshooting: the most recent
/// samples as they arrive, newest at the bottom. Shown only when the bug icon
/// is on. The numbers are the acceleration in m/s² and the quaternion the
/// sensor sent; if nothing scrolls, the sensor is not streaming.
/// How much the bar slowed across the set, as a percentage of the fastest rep -
/// the same velocity-loss the panel and the export show.
double _velocityLoss(List<double> velocities) {
  if (velocities.length < 2) return 0;
  final best = velocities.reduce((a, b) => a > b ? a : b);
  final last = velocities.last;
  if (best <= 0 || last >= best) return 0;
  return (best - last) / best * 100;
}

class _DebugConsole extends ConsumerWidget {
  final List<BarSpeedSample> samples;
  const _DebugConsole({required this.samples});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    const ink = Color(0xFF9CDCFE);
    const mono = TextStyle(fontFamily: 'monospace', fontSize: 11, color: ink, height: 1.4);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: const Color(0xFF1E1E1E),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          const Icon(Icons.terminal, size: 14, color: ink),
          const SizedBox(width: 6),
          Text(t('devices.barSpeed.debugTitle'),
              style: const TextStyle(color: ink, fontSize: 12, fontWeight: FontWeight.w600)),
          const Spacer(),
          Text('${samples.length}', style: const TextStyle(fontFamily: 'monospace', fontSize: 11, color: Color(0xFF808080))),
        ]),
        const SizedBox(height: 6),
        if (samples.isEmpty)
          Text(t('devices.barSpeed.debugWaiting'), style: mono)
        else
          for (final s in samples)
            Text(
              't=${s.t.toStringAsFixed(2)}  '
              'a=[${s.ax.toStringAsFixed(2)}, ${s.ay.toStringAsFixed(2)}, ${s.az.toStringAsFixed(2)}]  '
              'q=[${s.qw.toStringAsFixed(2)}, ${s.qx.toStringAsFixed(2)}, ${s.qy.toStringAsFixed(2)}, ${s.qz.toStringAsFixed(2)}]',
              style: mono,
            ),
      ]),
    );
  }
}
