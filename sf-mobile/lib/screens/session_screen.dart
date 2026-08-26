import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:path_provider/path_provider.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/export_menu.dart';

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
        actions: [
          // The block with this member's feedback on it. Offered to whoever
          // may open the screen: the member sends it to their coach, and the
          // coach exports their athlete's block to read away from the app.
          if (detail.valueOrNull != null)
            _AssignmentExport(
              assignmentId: assignmentId,
              programName: detail.valueOrNull!.assignment.programName,
              tooltip: t('session.exportBlock'),
            ),
        ],
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
              physics: const AlwaysScrollableScrollPhysics(),
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

                // Grouped by week, because that is the unit a week's work is
                // discussed in: one export per week, and its sessions under
                // it. The same grouping the exports use server-side.
                for (final week in _weeksOf(data.days)) ...[
                  Padding(
                    padding: const EdgeInsets.fromLTRB(4, 16, 4, 4),
                    child: Row(
                      children: [
                        Expanded(
                          child: Text(
                            t('programs.week', {'week': '${week.number}'}),
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                        ),
                        _AssignmentExport(
                          assignmentId: assignmentId,
                          programName: data.assignment.programName,
                          week: week.number,
                          tooltip: t('session.exportWeek'),
                        ),
                      ],
                    ),
                  ),
                  for (final day in week.days)
                    _DayCard(
                      day: day,
                      assignmentId: assignmentId,
                      initiallyExpanded: week == _weeksOf(data.days).first && day == week.days.first,
                    ),
                ],
              ],
            ),
          );
        },
      ),
    );
  }
}

/// The sessions grouped by week, in order.
///
/// The same grouping the API's exports use (programsheet.Weeks): a week number
/// can be skipped by an imported spreadsheet, so the weeks are read off the
/// sessions rather than assumed contiguous.
List<({int number, List<ProgramDay> days})> _weeksOf(List<ProgramDay> days) {
  final byNumber = <int, List<ProgramDay>>{};
  for (final day in days) {
    byNumber.putIfAbsent(day.week, () => []).add(day);
  }
  final numbers = byNumber.keys.toList()..sort();
  return [for (final number in numbers) (number: number, days: byNumber[number]!)];
}

/// Exports an assigned block - or one week of it - with the member's feedback.
class _AssignmentExport extends ConsumerStatefulWidget {
  final String assignmentId;
  final String programName;
  final int week;
  final String tooltip;

  const _AssignmentExport({
    required this.assignmentId,
    required this.programName,
    required this.tooltip,
    this.week = 0,
  });

  @override
  ConsumerState<_AssignmentExport> createState() => _AssignmentExportState();
}

class _AssignmentExportState extends ConsumerState<_AssignmentExport> {
  bool _busy = false;

  Future<void> _export(ExportFormat format) async {
    setState(() => _busy = true);
    try {
      final directory = await getApplicationDocumentsDirectory();
      final name = [
        _safeFileName(widget.programName),
        if (widget.week > 0) 'w${widget.week}',
      ].join('-');

      final path = await ref.read(apiProvider).downloadAssignment(
            widget.assignmentId,
            directory: directory.path,
            fileName: '$name.${format.extension}',
            format: format.extension,
            week: widget.week,
            locale: ref.read(localeProvider),
          );
      if (mounted) await openExported(context, ref, path);
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SfExportButton(busy: _busy, onExport: _export, tooltip: widget.tooltip);
  }
}

/// Keeps a program's name usable as a filename, since it is whatever somebody
/// typed.
String _safeFileName(String name) {
  final cleaned = name.replaceAll(RegExp(r'[^A-Za-z0-9 _-]'), '').trim().replaceAll(' ', '-');
  return cleaned.isEmpty ? 'program' : cleaned;
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
    final allDone = day.sets.isNotEmpty && done == day.sets.length;

    return Card(
      child: ExpansionTile(
        initiallyExpanded: initiallyExpanded,
        shape: const Border(),
        title: Row(
          children: [
            Expanded(
              // The session's own name follows the numbers when it has one:
              // it was saved and then shown nowhere, so a titled session
              // looked exactly like an untitled one.
              child: Text.rich(
                TextSpan(children: [
                  TextSpan(
                    text: '${t('programs.week', {'week': '${day.week}'})} · ${t('programs.day', {'day': '${day.day}'})}',
                  ),
                  if (day.name.isNotEmpty)
                    TextSpan(
                      text: ' · ${day.name}',
                      style: TextStyle(color: AppColors.of(context).textMuted, fontWeight: FontWeight.w500),
                    ),
                ]),
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            // The whole session in one tap, on the right of the panel. It sits
            // in the title rather than in `trailing`, which the expand arrow
            // already owns.
            _DoneToggle(
              done: allDone,
              tooltip: allDone ? t('session.markDayUndone') : t('session.markDayDone'),
              onPressed: () => _setDayDone(context, ref, !allDone),
            ),
            const SizedBox(width: 4),
          ],
        ),
        subtitle: Text(t('session.progress', {'done': '$done', 'total': '${day.sets.length}'})),
        children: [
          for (final set in day.sets)
            _SetTile(set: set, assignmentId: assignmentId, locale: locale),
        ],
      ),
    );
  }

  Future<void> _setDayDone(BuildContext context, WidgetRef ref, bool done) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(apiProvider).setDayDone(assignmentId, day.id, done);
      ref.invalidate(assignmentProvider(assignmentId));
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    }
  }
}

/// Done, or not done.
///
/// Green when the set (or the session) is finished, red when it is not, and it
/// flips on tap - the two states a member is ever in while running a program.
/// There is no third state to clear back to: a set they have not done yet is
/// simply not done.
class _DoneToggle extends StatelessWidget {
  final bool done;
  final String tooltip;
  final VoidCallback? onPressed;

  const _DoneToggle({required this.done, required this.tooltip, this.onPressed});

  @override
  Widget build(BuildContext context) {
    final colors = AppColors.of(context);
    final ink = done ? colors.success : colors.danger;

    return IconButton(
      icon: Icon(done ? Icons.check_circle : Icons.cancel_outlined),
      color: ink,
      iconSize: 22,
      visualDensity: VisualDensity.compact,
      tooltip: tooltip,
      onPressed: onPressed,
    );
  }
}

/// The perceived-RPE values on offer, as the chart is written: half points
/// from 6 up, and nothing below - "it moved" is not an RPE, and the chart has
/// no row for one.
const _perceivedRpe = [6.0, 6.5, 7.0, 7.5, 8.0, 8.5, 9.0, 9.5, 10.0];

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

    // What was actually lifted, and what the bar reads because of it.
    final usedLoad = set.log?.actualLoad;
    final onTheBar = usedLoad ?? (set.loadKnown && set.roundedLoad > 0 ? set.roundedLoad : null);

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
          // "On the bar" is what the bar actually read, so a logged load
          // replaces the computed one here - a set logged at 130 next to
          // 122.5 on the bar describes a session nobody did. The colour is
          // what says which of the two you are looking at, and the plan moves
          // into a long-press rather than being lost.
          Tooltip(
            message: usedLoad == null
                ? ''
                : (set.loadKnown && set.roundedLoad > 0
                    ? t('session.loggedLoadPlanned', {'value': '${_fmt(set.roundedLoad)} ${t('common.kg')}'})
                    : t('session.loggedLoad')),
            triggerMode: usedLoad == null ? TooltipTriggerMode.manual : TooltipTriggerMode.longPress,
            child: Text.rich(TextSpan(children: [
              TextSpan(
                text: '${set.reps} × ${t('session.rpe')} ${set.rpe?.toStringAsFixed(set.rpe! % 1 == 0 ? 0 : 1) ?? t('common.unknown')}'
                    '${set.loadKnown && set.computedPercentage > 0 ? ' · ${set.computedPercentage.toStringAsFixed(0)}%' : ''}',
              ),
              if (onTheBar != null) ...[
                TextSpan(text: ' · ${t('session.onTheBar')} '),
                TextSpan(
                  text: '${_fmt(onTheBar)}${t('common.kg')}',
                  style: usedLoad != null
                      ? TextStyle(color: AppColors.of(context).success, fontWeight: FontWeight.w600)
                      : null,
                ),
              ],
            ])),
          ),
          if (set.log?.comment.isNotEmpty == true)
            Text('“${set.log!.comment}”', style: Theme.of(context).textTheme.bodySmall),
        ],
      ),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Pick how the set actually felt and it saves there and then, and
          // the rest of the session is re-resolved against the max that effort
          // demonstrates.
          _PerceivedRpe(set: set, assignmentId: assignmentId),
          // Done or not done, and nothing else: the two states a set is in
          // while a session is being run.
          _DoneToggle(
            done: logged,
            tooltip: logged ? t('session.markUndone') : t('session.markDone'),
            onPressed: () => _setDone(context, ref, !logged),
          ),
          // Everything the two controls above do not carry: the reps and the
          // weight when they differed from the prescription, and a note.
          IconButton(
            icon: const Icon(Icons.edit_note),
            tooltip: t('session.log'),
            visualDensity: VisualDensity.compact,
            onPressed: () => _openLogSheet(context, ref),
          ),
        ],
      ),
    );
  }

  /// Ticks this set off, or puts it back.
  ///
  /// The API replaces a set's log wholesale, so the flag has to travel with
  /// whatever else was already logged: ticking a set off must not delete the
  /// RPE the member picked for it.
  Future<void> _setDone(BuildContext context, WidgetRef ref, bool done) async {
    final messenger = ScaffoldMessenger.of(context);
    final log = set.log;
    try {
      await ref.read(apiProvider).logSet(
            assignmentId,
            set.id,
            actualReps: log?.actualReps,
            actualRpe: log?.actualRpe,
            actualLoad: log?.actualLoad,
            comment: log?.comment ?? '',
            done: done,
          );
      ref.invalidate(assignmentProvider(assignmentId));
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    }
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
    final load = Text(
      '${_fmt(set.load)} ${t('common.kg')}',
      style: TextStyle(color: scheme.primary, fontWeight: FontWeight.bold, fontSize: 16),
    );
    if (!set.autoregulated) return load;

    // A load that moved because of what was just lifted has to say so, or the
    // weight looks like it changed on its own.
    return Tooltip(
      message: t('session.fromTodaysE1rm', {'value': _fmt(set.oneRm)}),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.autorenew, size: 14, color: scheme.primary),
          const SizedBox(width: 3),
          load,
        ],
      ),
    );
  }
}

/// The perceived RPE, saved the moment it is picked.
///
/// The API replaces a set's log wholesale, so this carries whatever else was
/// already logged with it - otherwise choosing "8" would quietly delete the
/// note the member left and the weight they typed.
class _PerceivedRpe extends ConsumerStatefulWidget {
  final ProgramSet set;
  final String assignmentId;

  const _PerceivedRpe({required this.set, required this.assignmentId});

  @override
  ConsumerState<_PerceivedRpe> createState() => _PerceivedRpeState();
}

class _PerceivedRpeState extends ConsumerState<_PerceivedRpe> {
  bool _busy = false;

  Future<void> _pick(double value) async {
    final log = widget.set.log;
    setState(() => _busy = true);
    try {
      await ref.read(apiProvider).logSet(
            widget.assignmentId,
            widget.set.id,
            actualReps: log?.actualReps,
            actualRpe: value,
            actualLoad: log?.actualLoad,
            comment: log?.comment ?? '',
            // Saying how a set felt is also saying you did it: nobody rates a
            // set they have not run.
            done: true,
          );
      ref.invalidate(assignmentProvider(widget.assignmentId));
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final current = widget.set.log?.actualRpe;

    if (_busy) {
      return const SizedBox(width: 42, height: 18, child: Center(
        child: SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2)),
      ));
    }

    return DropdownButton<double>(
      value: _perceivedRpe.contains(current) ? current : null,
      hint: Text(t('session.rpePlaceholder'), style: const TextStyle(fontSize: 12)),
      underline: const SizedBox.shrink(),
      isDense: true,
      items: [
        for (final value in _perceivedRpe)
          DropdownMenuItem(value: value, child: Text(_fmt(value), style: const TextStyle(fontSize: 13))),
      ],
      onChanged: (value) => value == null ? null : _pick(value),
    );
  }
}

/// The log form: what was actually done, the RPE it felt like, and a comment for
/// the coach. Reps and RPE pre-fill from the prescription, since the common
/// case is "did what was asked".
///
/// The load does not. A pre-filled weight is saved as if it had been typed the
/// moment the set is ticked off, so every set came back claiming a load the
/// member never entered - and the row then shows it as the weight they used.
/// The prescription is offered as the field's hint instead: visible, and not a
/// value.
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
    _load = TextEditingController(text: log?.actualLoad != null ? _fmt(log!.actualLoad!) : '');
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
                    decoration: InputDecoration(
                      labelText: t('session.actualLoad'),
                      hintText: widget.set.loadKnown && widget.set.roundedLoad > 0
                          ? _fmt(widget.set.roundedLoad)
                          : null,
                    ),
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
