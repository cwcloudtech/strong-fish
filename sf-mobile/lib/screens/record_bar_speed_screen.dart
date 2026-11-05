import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../native/bar_speed_capture.dart';
import '../providers/providers.dart';
import '../theme.dart';

const _movements = ['squat', 'bench', 'deadlift'];

/// Records a set from a Bluetooth bar-speed sensor: pick the lift and the load,
/// hold through the set, and the raw stream is posted to StrongFish's own engine
/// (which computes and stores it). The capture transport is native (see
/// [BarSpeedCapture]); this screen drives it and shows the result.
class RecordBarSpeedScreen extends ConsumerStatefulWidget {
  final String initialMovement;
  const RecordBarSpeedScreen({super.key, this.initialMovement = 'deadlift'});

  @override
  ConsumerState<RecordBarSpeedScreen> createState() => _RecordBarSpeedScreenState();
}

class _RecordBarSpeedScreenState extends ConsumerState<RecordBarSpeedScreen> {
  final BarSpeedCapture _capture = BarSpeedCapture();
  final TextEditingController _load = TextEditingController();
  late String _movement = _movements.contains(widget.initialMovement) ? widget.initialMovement : 'deadlift';
  StreamSubscription<BarSpeedSample>? _sub;
  final List<Map<String, double>> _samples = [];
  bool _recording = false;
  bool _busy = false;
  StanceSet? _result;
  String _error = '';

  @override
  void dispose() {
    _sub?.cancel();
    _capture.stop();
    _load.dispose();
    super.dispose();
  }

  void _start() {
    setState(() {
      _samples.clear();
      _result = null;
      _error = '';
      _recording = true;
    });
    _sub = _capture.start().listen(
      (sample) {
        _samples.add(sample.toJson());
        if (mounted) setState(() {}); // refresh the live sample count
      },
      onError: (Object error) {
        if (mounted) {
          setState(() {
            _error = ref.read(tErrorProvider)(error);
            _recording = false;
          });
        }
      },
    );
  }

  Future<void> _stop() async {
    await _sub?.cancel();
    await _capture.stop();
    if (!mounted) return;
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

    return Scaffold(
      appBar: AppBar(title: Text(t('devices.barSpeed.recordTitle'))),
      body: ListView(padding: const EdgeInsets.all(16), children: [
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
          onPressed: _busy ? null : (_recording ? _stop : _start),
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
