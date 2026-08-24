import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../widgets/common.dart';

/// The member's maxes - the single input every prescribed load is computed
/// from. Saving one here recalculates every program they're running, which is
/// why the training screens are invalidated alongside.
class OneRmsScreen extends ConsumerWidget {
  const OneRmsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final locale = ref.watch(localeProvider);
    final exercises = ref.watch(exercisesProvider);
    final maxes = ref.watch(oneRmsProvider);

    if (exercises.isLoading || maxes.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (exercises.hasError || maxes.hasError) {
      return SfErrorState(
        message: ref.read(tErrorProvider)(exercises.error ?? maxes.error!),
        retryLabel: t('common.back'),
        onRetry: () {
          ref.invalidate(exercisesProvider);
          ref.invalidate(oneRmsProvider);
        },
      );
    }

    final catalog = exercises.value ?? const <Exercise>[];
    final byExercise = {for (final max in maxes.value ?? const <OneRm>[]) max.exerciseId: max};

    // The competition lifts are always listed, even without a value: they're
    // what most prescriptions resolve against, so an empty one is exactly the
    // thing worth prompting for. Other movements appear once recorded.
    final mainLifts = catalog.where((exercise) => exercise.main).toList();
    final others = catalog.where((exercise) => !exercise.main && byExercise.containsKey(exercise.id)).toList();

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(oneRmsProvider);
        ref.invalidate(exercisesProvider);
      },
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(16),
        children: [
          Text(t('oneRms.subtitle'), style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 16),
          Text(t('oneRms.mainLifts'), style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          for (final exercise in mainLifts)
            _OneRmRow(exercise: exercise, current: byExercise[exercise.id], locale: locale),
          if (others.isNotEmpty) ...[
            const SizedBox(height: 20),
            Text(t('oneRms.otherLifts'), style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            for (final exercise in others)
              _OneRmRow(exercise: exercise, current: byExercise[exercise.id], locale: locale),
          ],
          const SizedBox(height: 20),
          _AddLiftButton(
            catalog: catalog.where((exercise) => !exercise.main && !exercise.bodyweight).toList(),
            recorded: byExercise.keys.toSet(),
            locale: locale,
          ),
        ],
      ),
    );
  }
}

class _OneRmRow extends ConsumerStatefulWidget {
  final Exercise exercise;
  final OneRm? current;
  final String locale;

  const _OneRmRow({required this.exercise, required this.current, required this.locale});

  @override
  ConsumerState<_OneRmRow> createState() => _OneRmRowState();
}

class _OneRmRowState extends ConsumerState<_OneRmRow> {
  late final TextEditingController _value;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _value = TextEditingController(
      text: widget.current != null ? _fmt(widget.current!.value) : '',
    );
  }

  @override
  void dispose() {
    _value.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final parsed = double.tryParse(_value.text.trim().replaceAll(',', '.'));
    final t = ref.read(tProvider);
    if (parsed == null || parsed <= 0) {
      _toast(t('errors.invalidOneRm'));
      return;
    }

    setState(() => _busy = true);
    try {
      await ref.read(apiProvider).setOneRm(widget.exercise.id, parsed);
      // Both the list and every assignment's resolved loads are now stale.
      ref.invalidate(oneRmsProvider);
      ref.invalidate(assignmentsProvider);
      _toast(t('oneRms.saved'));
    } catch (error) {
      _toast(ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _toast(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);

    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(widget.exercise.label(widget.locale)),
                  if (widget.current != null)
                    Text(
                      '${t('oneRms.updated')}: ${widget.current!.updatedAt.toLocal().toString().split(' ').first}',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                ],
              ),
            ),
            SizedBox(
              width: 92,
              child: TextField(
                controller: _value,
                keyboardType: const TextInputType.numberWithOptions(decimal: true),
                textAlign: TextAlign.right,
                decoration: InputDecoration(suffixText: t('common.kg')),
                onSubmitted: (_) => _save(),
              ),
            ),
            IconButton(
              icon: _busy
                  ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.check),
              tooltip: t('common.save'),
              onPressed: _busy ? null : _save,
            ),
          ],
        ),
      ),
    );
  }
}

/// Lets a member record a max for a movement outside the three competition
/// lifts - useful when they've actually tested, say, their paused deadlift.
class _AddLiftButton extends ConsumerWidget {
  final List<Exercise> catalog;
  final Set<String> recorded;
  final String locale;

  const _AddLiftButton({required this.catalog, required this.recorded, required this.locale});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final addable = catalog.where((exercise) => !recorded.contains(exercise.id)).toList();
    if (addable.isEmpty) return const SizedBox.shrink();

    return OutlinedButton.icon(
      icon: const Icon(Icons.add),
      label: Text(t('oneRms.addLift')),
      onPressed: () async {
        final picked = await showModalBottomSheet<Exercise>(
          context: context,
          isScrollControlled: true,
          builder: (context) => SafeArea(
            child: ListView(
              shrinkWrap: true,
              children: [
                for (final exercise in addable)
                  ListTile(
                    title: Text(exercise.label(locale)),
                    onTap: () => Navigator.of(context).pop(exercise),
                  ),
              ],
            ),
          ),
        );
        if (picked == null || !context.mounted) return;

        // Seeding it at zero would be refused by the API (a max must be
        // positive), so the value is asked for up front.
        await showDialog<void>(
          context: context,
          builder: (context) => _AddLiftDialog(exercise: picked, locale: locale),
        );
      },
    );
  }
}

class _AddLiftDialog extends ConsumerStatefulWidget {
  final Exercise exercise;
  final String locale;

  const _AddLiftDialog({required this.exercise, required this.locale});

  @override
  ConsumerState<_AddLiftDialog> createState() => _AddLiftDialogState();
}

class _AddLiftDialogState extends ConsumerState<_AddLiftDialog> {
  final _value = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _value.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final parsed = double.tryParse(_value.text.trim().replaceAll(',', '.'));
    if (parsed == null || parsed <= 0) {
      setState(() => _error = ref.read(tProvider)('errors.invalidOneRm'));
      return;
    }
    setState(() => _busy = true);
    try {
      await ref.read(apiProvider).setOneRm(widget.exercise.id, parsed);
      ref.invalidate(oneRmsProvider);
      ref.invalidate(assignmentsProvider);
      if (mounted) Navigator.of(context).pop();
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = ref.read(tErrorProvider)(error);
          _busy = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    return AlertDialog(
      title: Text(widget.exercise.label(widget.locale)),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            controller: _value,
            autofocus: true,
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            decoration: InputDecoration(labelText: t('oneRms.value'), suffixText: t('common.kg')),
          ),
          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          ],
        ],
      ),
      actions: [
        TextButton(onPressed: _busy ? null : () => Navigator.of(context).pop(), child: Text(t('common.cancel'))),
        FilledButton(onPressed: _busy ? null : _save, child: Text(t('common.save'))),
      ],
    );
  }
}

String _fmt(double value) => value % 1 == 0 ? value.toStringAsFixed(0) : value.toStringAsFixed(1);
