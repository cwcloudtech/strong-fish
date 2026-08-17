import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../widgets/common.dart';

/// One assigned program, session by session - the screen a member actually uses
/// in the gym.
///
/// Every load shown was computed server-side from this member's current 1RMs, so
/// after logging a set the whole assignment is re-fetched rather than patched
/// locally: there is no derived state on the client to keep in sync.
class SessionScreen extends ConsumerWidget {
  final String assignmentId;

  const SessionScreen({super.key, required this.assignmentId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final locale = ref.watch(localeProvider);
    final detail = ref.watch(assignmentProvider(assignmentId));

    return Scaffold(
      appBar: AppBar(
        title: Text(detail.valueOrNull?.assignment.programName ?? t('nav.training')),
      ),
      body: detail.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => SfErrorState(
          message: ref.read(tErrorProvider)(error),
          retryLabel: t('common.back'),
          onRetry: () => ref.invalidate(assignmentProvider(assignmentId)),
        ),
        data: (data) {
          return RefreshIndicator(
            onRefresh: () async => ref.invalidate(assignmentProvider(assignmentId)),
            child: ListView(
              padding: const EdgeInsets.all(16),
              children: [
                if (data.assignment.note.isNotEmpty)
                  Card(
                    child: Padding(padding: const EdgeInsets.all(14), child: Text(data.assignment.note)),
                  ),

                // The one thing that blocks a session: without a max, the sets
                // that resolve against it have no weight to show.
                if (data.missingOneRms.isNotEmpty)
                  Card(
                    color: Theme.of(context).colorScheme.errorContainer,
                    child: Padding(
                      padding: const EdgeInsets.all(14),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(t('session.missingOneRmsList', {
                            'list': data.missingOneRms.map((e) => e.label(locale)).join(', '),
                          })),
                          const SizedBox(height: 8),
                          FilledButton(
                            onPressed: () => context.push('/one-rms'),
                            child: Text(t('session.setMyOneRms')),
                          ),
                        ],
                      ),
                    ),
                  ),

                for (final day in data.days)
                  _DayCard(
                    day: day,
                    assignmentId: assignmentId,
                    initiallyExpanded: day == data.days.first,
                  ),
              ],
            ),
          );
        },
      ),
    );
  }
}

class _DayCard extends ConsumerWidget {
  final ProgramDay day;
  final String assignmentId;
  final bool initiallyExpanded;

  const _DayCard({required this.day, required this.assignmentId, required this.initiallyExpanded});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final locale = ref.watch(localeProvider);
    final done = day.doneCount;

    return Card(
      child: ExpansionTile(
        initiallyExpanded: initiallyExpanded,
        shape: const Border(),
        title: Text(
          '${t('programs.week', {'week': '${day.week}'})} · ${t('programs.day', {'day': '${day.day}'})}',
          style: Theme.of(context).textTheme.titleMedium,
        ),
        subtitle: Text(t('session.progress', {'done': '$done', 'total': '${day.sets.length}'})),
        children: [
          for (final set in day.sets)
            _SetTile(set: set, assignmentId: assignmentId, locale: locale),
        ],
      ),
    );
  }
}

class _SetTile extends ConsumerWidget {
  final ProgramSet set;
  final String assignmentId;
  final String locale;

  const _SetTile({required this.set, required this.assignmentId, required this.locale});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final scheme = Theme.of(context).colorScheme;
    final logged = set.log?.done == true;

    return ListTile(
      dense: true,
      tileColor: logged ? scheme.primaryContainer.withValues(alpha: 0.25) : null,
      title: Row(
        children: [
          Expanded(child: Text(set.label(locale))),
          _LoadLabel(set: set),
        ],
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '${set.reps} × ${t('session.rpe')} ${set.rpe?.toStringAsFixed(set.rpe! % 1 == 0 ? 0 : 1) ?? t('common.unknown')}'
            '${set.loadKnown && set.computedPercentage > 0 ? ' · ${set.computedPercentage.toStringAsFixed(0)}%' : ''}'
            '${set.loadKnown && set.roundedLoad > 0 ? ' · ${t('session.onTheBar')} ${_fmt(set.roundedLoad)}${t('common.kg')}' : ''}',
          ),
          if (set.log?.comment.isNotEmpty == true)
            Text('“${set.log!.comment}”', style: Theme.of(context).textTheme.bodySmall),
        ],
      ),
      trailing: IconButton(
        icon: Icon(logged ? Icons.check_circle : Icons.edit_note, color: logged ? scheme.primary : null),
        tooltip: t('session.log'),
        onPressed: () => _openLogSheet(context, ref),
      ),
    );
  }

  Future<void> _openLogSheet(BuildContext context, WidgetRef ref) async {
    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      builder: (context) => Padding(
        padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
        child: _LogSheet(set: set, assignmentId: assignmentId, locale: locale),
      ),
    );
    if (saved == true) ref.invalidate(assignmentProvider(assignmentId));
  }
}

/// The prescribed weight, or the "?" that means the member hasn't recorded the
/// max it would come from.
class _LoadLabel extends ConsumerWidget {
  final ProgramSet set;

  const _LoadLabel({required this.set});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final scheme = Theme.of(context).colorScheme;

    if (set.isBodyweight) {
      return Chip(
        label: Text(t('session.bodyweight'), style: const TextStyle(fontSize: 11)),
        visualDensity: VisualDensity.compact,
        padding: EdgeInsets.zero,
      );
    }
    if (!set.loadKnown) {
      return Tooltip(
        message: t('session.missingOneRm'),
        child: Text(t('common.unknown'),
            style: TextStyle(color: scheme.error, fontWeight: FontWeight.bold, fontSize: 16)),
      );
    }
    return Text(
      '${_fmt(set.load)} ${t('common.kg')}',
      style: TextStyle(color: scheme.primary, fontWeight: FontWeight.bold, fontSize: 16),
    );
  }
}

/// The log form: what was actually done, the RPE it felt like, and a comment for
/// the coach. Fields pre-fill from the prescription, since the common case is
/// "did what was asked".
class _LogSheet extends ConsumerStatefulWidget {
  final ProgramSet set;
  final String assignmentId;
  final String locale;

  const _LogSheet({required this.set, required this.assignmentId, required this.locale});

  @override
  ConsumerState<_LogSheet> createState() => _LogSheetState();
}

class _LogSheetState extends ConsumerState<_LogSheet> {
  late final TextEditingController _reps;
  late final TextEditingController _rpe;
  late final TextEditingController _load;
  late final TextEditingController _comment;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final log = widget.set.log;
    _reps = TextEditingController(text: '${log?.actualReps ?? widget.set.reps}');
    _rpe = TextEditingController(
        text: log?.actualRpe != null ? _fmt(log!.actualRpe!) : (widget.set.rpe != null ? _fmt(widget.set.rpe!) : ''));
    _load = TextEditingController(
        text: log?.actualLoad != null
            ? _fmt(log!.actualLoad!)
            : (widget.set.loadKnown && widget.set.roundedLoad > 0 ? _fmt(widget.set.roundedLoad) : ''));
    _comment = TextEditingController(text: log?.comment ?? '');
  }

  @override
  void dispose() {
    for (final controller in [_reps, _rpe, _load, _comment]) {
      controller.dispose();
    }
    super.dispose();
  }

  Future<void> _save() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(apiProvider).logSet(
            widget.assignmentId,
            widget.set.id,
            actualReps: int.tryParse(_reps.text.trim()),
            actualRpe: double.tryParse(_rpe.text.trim().replaceAll(',', '.')),
            actualLoad: double.tryParse(_load.text.trim().replaceAll(',', '.')),
            comment: _comment.text.trim(),
          );
      if (mounted) Navigator.of(context).pop(true);
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = ref.read(tErrorProvider)(error);
          _busy = false;
        });
      }
    }
  }

  Future<void> _clear() async {
    setState(() => _busy = true);
    try {
      await ref.read(apiProvider).clearLog(widget.assignmentId, widget.set.id);
      if (mounted) Navigator.of(context).pop(true);
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
    final log = widget.set.log;

    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(widget.set.label(widget.locale), style: Theme.of(context).textTheme.titleMedium),
          if (widget.set.loadKnown && widget.set.oneRm > 0)
            Text(
              t('session.from1rm', {
                'lift': widget.set.exerciseOneRmRef.isNotEmpty
                    ? widget.set.exerciseOneRmRef
                    : widget.set.exerciseSlug,
                'value': _fmt(widget.set.oneRm),
              }),
              style: Theme.of(context).textTheme.bodySmall,
            ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _reps,
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(labelText: t('session.actualReps')),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: TextField(
                  controller: _rpe,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(labelText: t('session.actualRpe')),
                ),
              ),
              if (!widget.set.isBodyweight) ...[
                const SizedBox(width: 10),
                Expanded(
                  child: TextField(
                    controller: _load,
                    keyboardType: const TextInputType.numberWithOptions(decimal: true),
                    decoration: InputDecoration(labelText: t('session.actualLoad')),
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _comment,
            maxLines: 2,
            decoration: InputDecoration(labelText: t('session.comment')),
          ),
          if (log?.e1rm != null && log!.e1rm > 0) ...[
            const SizedBox(height: 10),
            Text(
              t('session.yourE1rm', {'value': _fmt(log.e1rm)}) +
                  (widget.set.oneRm > 0 && log.e1rm > widget.set.oneRm ? ' ${t('session.beatsMax')}' : ''),
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ],
          if (_error != null) ...[
            const SizedBox(height: 10),
            Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          ],
          const SizedBox(height: 16),
          Row(
            children: [
              if (log != null)
                TextButton(onPressed: _busy ? null : _clear, child: Text(t('session.clearLog'))),
              const Spacer(),
              TextButton(
                onPressed: _busy ? null : () => Navigator.of(context).pop(false),
                child: Text(t('common.cancel')),
              ),
              const SizedBox(width: 8),
              FilledButton(onPressed: _busy ? null : _save, child: Text(t('session.done'))),
            ],
          ),
        ],
      ),
    );
  }
}

/// Formats a weight without a trailing ".0" - "77.5kg" but "80kg".
String _fmt(double value) =>
    value % 1 == 0 ? value.toStringAsFixed(0) : value.toStringAsFixed(1);
