import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/strength_badges.dart';

/// The powerlifting calculator: three coefficients on one total.
///
/// The form arrives filled from the member's profile and recorded maxes - the
/// numbers are already in the app, and retyping them is how a calculator goes
/// unused. The scoring is the API's (internal/strength), never repeated here.
class StrengthScreen extends ConsumerStatefulWidget {
  const StrengthScreen({super.key});

  @override
  ConsumerState<StrengthScreen> createState() => _StrengthScreenState();
}

/// Pounds to kilograms, for the members who think in the other one.
const _lbPerKg = 2.2046226218;

class _StrengthScreenState extends ConsumerState<StrengthScreen> {
  final _bodyweight = TextEditingController();
  final _squat = TextEditingController();
  final _bench = TextEditingController();
  final _deadlift = TextEditingController();

  String _gender = 'male';
  String _division = 'raw';
  String _unit = 'kg';
  StrengthResult? _result;
  String? _error;
  Timer? _debounce;

  @override
  void initState() {
    super.initState();
    for (final controller in [_bodyweight, _squat, _bench, _deadlift]) {
      controller.addListener(_schedule);
    }
    _prefill();
  }

  @override
  void dispose() {
    _debounce?.cancel();
    for (final controller in [_bodyweight, _squat, _bench, _deadlift]) {
      controller.dispose();
    }
    super.dispose();
  }

  /// Whatever the API knows about whoever is asking. A signed-out caller gets
  /// an empty form back, so there is nothing to branch on here.
  Future<void> _prefill() async {
    try {
      final defaults = await ref.read(apiProvider).strengthDefaults();
      if (!mounted) return;
      setState(() {
        _gender = (defaults['gender'] as String?) ?? 'male';
        _division = (defaults['division'] as String?) ?? 'raw';
        _bodyweight.text = _fill(defaults['bodyweight']);
        _squat.text = _fill(defaults['squat']);
        _bench.text = _fill(defaults['bench']);
        _deadlift.text = _fill(defaults['deadlift']);
      });
    } catch (_) {
      // An empty form is a working calculator; nothing to report.
    }
  }

  String _fill(dynamic value) {
    final number = value is num ? value.toDouble() : 0.0;
    if (number <= 0) return '';
    return number % 1 == 0 ? number.toStringAsFixed(0) : number.toStringAsFixed(1);
  }

  double _kg(TextEditingController controller) {
    final parsed = double.tryParse(controller.text.trim().replaceAll(',', '.')) ?? 0;
    if (parsed <= 0) return 0;
    return _unit == 'lb' ? parsed / _lbPerKg : parsed;
  }

  /// Scored as the numbers are typed, debounced: the formulas live in one place
  /// and a calculator that needs a button pressed to answer is a form.
  void _schedule() {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), _score);
  }

  Future<void> _score() async {
    final bodyweight = _kg(_bodyweight);
    final lifts = _kg(_squat) + _kg(_bench) + _kg(_deadlift);
    if (bodyweight <= 0 || lifts <= 0) {
      if (mounted) setState(() => _result = null);
      return;
    }

    try {
      final result = await ref.read(apiProvider).score(
            gender: _gender,
            division: _division,
            bodyweight: bodyweight,
            squat: _kg(_squat),
            bench: _kg(_bench),
            deadlift: _kg(_deadlift),
          );
      if (mounted) {
        setState(() {
          _result = result;
          _error = null;
        });
      }
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    }
  }

  /// A weight back in whatever unit is being typed in.
  String _show(double kg, String Function(String, [Map<String, String>?]) t) {
    final value = _unit == 'lb' ? kg * _lbPerKg : kg;
    final rounded = (value * 10).round() / 10;
    return '$rounded ${_unit == 'lb' ? t('strength.lb') : t('common.kg')}';
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final result = _result;

    return Scaffold(
      appBar: AppBar(title: Text(t('strength.title'))),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text(t('strength.subtitle'), style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 16),

          Row(
            children: [
              Expanded(
                child: _Choice(
                  label: t('profile.gender'),
                  value: _gender,
                  options: {'male': t('profile.genderMale'), 'female': t('profile.genderFemale')},
                  onChanged: (value) => setState(() {
                    _gender = value;
                    _schedule();
                  }),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: _Choice(
                  label: t('strength.unit'),
                  value: _unit,
                  options: {'kg': t('strength.kg'), 'lb': t('strength.lb')},
                  // Only the reading changes: the weights go up in kilograms
                  // whatever is typed, so the scores do not move.
                  onChanged: (value) => setState(() {
                    _unit = value;
                    _schedule();
                  }),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          _Choice(
            label: t('strength.division'),
            value: _division,
            options: {'raw': t('strength.raw'), 'equipped': t('strength.equipped')},
            onChanged: (value) => setState(() {
              _division = value;
              _schedule();
            }),
          ),
          const SizedBox(height: 12),

          for (final field in [
            (_bodyweight, 'strength.bodyweight'),
            (_squat, 'strength.squat'),
            (_bench, 'strength.bench'),
            (_deadlift, 'strength.deadlift'),
          ]) ...[
            TextField(
              controller: field.$1,
              keyboardType: const TextInputType.numberWithOptions(decimal: true),
              decoration: InputDecoration(labelText: t(field.$2), suffixText: t('strength.$_unit')),
            ),
            const SizedBox(height: 12),
          ],

          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(_error!, style: TextStyle(color: AppColors.of(context).danger)),
          ],

          const SizedBox(height: 8),
          if (result == null)
            Text(t('strength.enterLifts'), style: Theme.of(context).textTheme.bodySmall)
          else ...[
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _Score(label: t('strength.total'), value: _show(result.total, t), big: true),
                    const SizedBox(height: 10),
                    Row(
                      children: [
                        Expanded(child: _Score(label: t('strength.dots'), value: _fmt(result.dots))),
                        Expanded(child: _Score(label: t('strength.wilks'), value: _fmt(result.wilks))),
                        Expanded(child: _Score(label: t('strength.ipfGl'), value: _fmt(result.ipfGl))),
                      ],
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: StrengthBadges(result: result),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// A two- or three-way choice, as the segmented control the rest of the app
/// uses for the same shape of question.
class _Choice extends StatelessWidget {
  final String label;
  final String value;
  final Map<String, String> options;
  final ValueChanged<String> onChanged;

  const _Choice({
    required this.label,
    required this.value,
    required this.options,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: Theme.of(context).textTheme.bodySmall),
        const SizedBox(height: 4),
        SegmentedButton<String>(
          segments: [
            for (final option in options.entries)
              ButtonSegment(value: option.key, label: Text(option.value)),
          ],
          selected: {value},
          showSelectedIcon: false,
          onSelectionChanged: (selection) => onChanged(selection.first),
        ),
      ],
    );
  }
}

class _Score extends StatelessWidget {
  final String label;
  final String value;
  final bool big;

  const _Score({required this.label, required this.value, this.big = false});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: theme.textTheme.bodySmall),
        Text(
          value,
          style: big
              ? theme.textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.bold)
              : theme.textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold),
        ),
      ],
    );
  }
}

String _fmt(double value) =>
    value % 1 == 0 ? value.toStringAsFixed(0) : value.toStringAsFixed(2);
