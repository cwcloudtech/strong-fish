import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:open_filex/open_filex.dart';
import 'package:path_provider/path_provider.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../widgets/common.dart';
import '../widgets/multi_select.dart';

/// Authoring a program on the phone: sessions, and the sets in them.
///
/// This is the counterpart of the web's builder, and the coaching-side twin of
/// SessionScreen - that one shows an athlete the weight to lift, this one shows
/// a coach the prescription they're writing.
class ProgramEditorScreen extends ConsumerStatefulWidget {
  final String clubId;
  final String programId;

  const ProgramEditorScreen({super.key, required this.clubId, required this.programId});

  @override
  ConsumerState<ProgramEditorScreen> createState() => _ProgramEditorScreenState();
}

class _ProgramEditorScreenState extends ConsumerState<ProgramEditorScreen> {
  ProgramDetail? _detail;
  String? _error;

  /// True while the printable sheet is being fetched.
  bool _exporting = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final detail = await ref.read(apiProvider).program(widget.clubId, widget.programId);
      if (mounted) {
        setState(() {
          _detail = detail;
          _error = null;
        });
      }
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    }
  }

  void _toast(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  /// Hands the program to one or more members of the club.
  ///
  /// A group in one go, because that is how a block is started - the picker is
  /// a multi-select for the same reason the web app's is.
  Future<void> _assign() async {
    final t = ref.read(tProvider);

    List<ClubMember> members;
    try {
      members = await ref.read(apiProvider).clubMembers(widget.clubId);
    } catch (error) {
      if (mounted) _toast(ref.read(tErrorProvider)(error));
      return;
    }
    if (!mounted) return;
    if (members.isEmpty) {
      _toast(t('programs.noMembers'));
      return;
    }

    final picked = await showDialog<List<String>>(
      context: context,
      builder: (dialogContext) => _AssignDialog(members: members),
    );
    if (picked == null || picked.isEmpty) return;

    try {
      final count = await ref.read(apiProvider).assignProgram(
            widget.clubId,
            widget.programId,
            userIds: picked,
          );
      _toast(t('programs.assigned', {'count': '$count'}));
    } catch (error) {
      _toast(ref.read(tErrorProvider)(error));
    }
  }

  /// Downloads the printable sheet and hands it to whatever opens PDFs on
  /// this phone.
  ///
  /// Saved to the app's own documents directory rather than a shared one: it
  /// needs no permission, and the file is a copy of something the server can
  /// render again at any time.
  Future<void> _exportPdf() async {
    final t = ref.read(tProvider);
    setState(() => _exporting = true);
    try {
      final directory = await getApplicationDocumentsDirectory();
      final path = await ref.read(apiProvider).downloadProgramPdf(
            widget.clubId,
            widget.programId,
            directory: directory.path,
            fileName: '${_safeFileName(_detail?.program.name ?? 'program')}.pdf',
            locale: ref.read(localeProvider),
          );

      final result = await OpenFilex.open(path);
      if (result.type != ResultType.done && mounted) {
        // A phone with no PDF viewer is a real case, and "nothing happened" is
        // the worst way to find out.
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(t('programs.exportNoViewer'))));
      }
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  /// Keeps a program's name usable as a filename, since it is whatever
  /// somebody typed.
  static String _safeFileName(String name) {
    final cleaned = name.replaceAll(RegExp(r'[^A-Za-z0-9 _-]'), '').trim().replaceAll(' ', '-');
    return cleaned.isEmpty ? 'program' : cleaned;
  }

  /// Everything about the program that is not a set: its name, the line under
  /// it, and who may read it.
  ///
  /// A program's name is usually whatever the imported spreadsheet was called,
  /// or something typed in a hurry on the first day of a block, and until now
  /// neither could be changed anywhere.
  Future<void> _editProgram() async {
    final detail = _detail;
    if (detail == null) return;

    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      builder: (context) => Padding(
        padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
        child: _ProgramDetailsSheet(clubId: widget.clubId, program: detail.program),
      ),
    );
    if (saved == true) await _load();
  }

  Future<void> _addSession() async {
    final t = ref.read(tProvider);
    try {
      // Numbers left at zero continue the program's own numbering server-side.
      await ref.read(apiProvider).addDay(widget.clubId, widget.programId);
      _toast(t('programs.sessionSaved'));
      await _load();
    } catch (error) {
      _toast(ref.read(tErrorProvider)(error));
    }
  }

  Future<void> _deleteSession(ProgramDay day) async {
    final t = ref.read(tProvider);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('common.delete')),
        content: Text(t('programs.confirmDeleteSession', {'week': '${day.week}', 'day': '${day.day}'})),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: Text(t('common.cancel'))),
          FilledButton(onPressed: () => Navigator.of(context).pop(true), child: Text(t('common.delete'))),
        ],
      ),
    );
    if (confirmed != true) return;

    try {
      await ref.read(apiProvider).deleteDay(widget.clubId, widget.programId, day.id);
      _toast(t('programs.sessionDeleted'));
      await _load();
    } catch (error) {
      _toast(ref.read(tErrorProvider)(error));
    }
  }

  Future<void> _deleteSet(ProgramSet set) async {
    final t = ref.read(tProvider);
    try {
      await ref.read(apiProvider).deleteSet(widget.clubId, widget.programId, set.id);
      _toast(t('programs.setDeleted'));
      await _load();
    } catch (error) {
      _toast(ref.read(tErrorProvider)(error));
    }
  }

  Future<void> _editSet({required ProgramDay day, ProgramSet? set}) async {
    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      builder: (context) => Padding(
        padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
        child: _SetEditorSheet(
          clubId: widget.clubId,
          programId: widget.programId,
          dayId: day.id,
          set: set,
        ),
      ),
    );
    if (saved == true) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final locale = ref.watch(localeProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(_detail?.program.name ?? t('programs.title')),
        actions: [
          IconButton(
            icon: const Icon(Icons.edit_outlined),
            tooltip: t('programs.edit'),
            onPressed: _detail == null ? null : _editProgram,
          ),
          IconButton(
            icon: const Icon(Icons.group_add_outlined),
            tooltip: t('programs.assign'),
            onPressed: _detail == null || widget.clubId.isEmpty ? null : _assign,
          ),
          IconButton(
            icon: _exporting
                ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                : const Icon(Icons.picture_as_pdf_outlined),
            tooltip: t('programs.exportPdf'),
            onPressed: _detail == null || _exporting ? null : _exportPdf,
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _addSession,
        icon: const Icon(Icons.add),
        label: Text(t('programs.addSession')),
      ),
      body: _error != null
          ? SfErrorState(message: _error!, retryLabel: t('common.back'), onRetry: _load)
          : _detail == null
              ? const Center(child: CircularProgressIndicator())
              : RefreshIndicator(
                  onRefresh: _load,
                  child: _detail!.days.isEmpty
                      ? ListView(
                          children: [
                            const SizedBox(height: 80),
                            SfEmptyState(icon: Icons.event_note, title: t('programs.noSessions')),
                          ],
                        )
                      : ListView(
                          padding: const EdgeInsets.fromLTRB(16, 16, 16, 88),
                          children: [
                            for (final day in _detail!.days)
                              Card(
                                child: Column(
                                  children: [
                                    ListTile(
                                      title: Text(
                                        '${t('programs.week', {'week': '${day.week}'})} · '
                                        '${t('programs.day', {'day': '${day.day}'})}',
                                        style: Theme.of(context).textTheme.titleMedium,
                                      ),
                                      trailing: IconButton(
                                        icon: const Icon(Icons.delete_outline),
                                        tooltip: t('common.delete'),
                                        onPressed: () => _deleteSession(day),
                                      ),
                                    ),
                                    for (final set in day.sets)
                                      ListTile(
                                        dense: true,
                                        title: Text(set.label(locale)),
                                        subtitle: Text('${set.reps} × ${_describeLoad(t, set)}'),
                                        onTap: () => _editSet(day: day, set: set),
                                        trailing: IconButton(
                                          icon: const Icon(Icons.close),
                                          tooltip: t('common.delete'),
                                          onPressed: () => _deleteSet(set),
                                        ),
                                      ),
                                    Padding(
                                      padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
                                      child: Align(
                                        alignment: Alignment.centerLeft,
                                        child: OutlinedButton.icon(
                                          icon: const Icon(Icons.add, size: 18),
                                          label: Text(t('programs.addSet')),
                                          onPressed: () => _editSet(day: day),
                                        ),
                                      ),
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

/// A one-line summary of how a set is loaded, for the authoring list.
String _describeLoad(String Function(String, [Map<String, String>?]) t, ProgramSet set) {
  switch (set.loadMode) {
    case 'rpe':
      return '${t('session.rpe')} ${set.rpe ?? '—'}';
    case 'percentage':
      return '${set.percentage ?? '—'}%';
    case 'absolute':
      return '${set.absoluteLoad ?? '—'} ${t('common.kg')}';
    default:
      return t('session.bodyweight');
  }
}

/// Writes one prescribed set. Which fields appear follows from the load mode,
/// so an RPE set never collects a weight that would be ignored.
/// The program's own details, in the same kind of sheet the sets are edited in.
///
/// Name, description and audience together, because the API replaces the three
/// in one request: sending a new name without the visibility would republish a
/// public program to its club as a side effect of renaming it.
class _ProgramDetailsSheet extends ConsumerStatefulWidget {
  final String clubId;
  final Program program;

  const _ProgramDetailsSheet({required this.clubId, required this.program});

  @override
  ConsumerState<_ProgramDetailsSheet> createState() => _ProgramDetailsSheetState();
}

class _ProgramDetailsSheetState extends ConsumerState<_ProgramDetailsSheet> {
  late final TextEditingController _name;
  late final TextEditingController _description;
  late String _visibility;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _name = TextEditingController(text: widget.program.name);
    _description = TextEditingController(text: widget.program.description);
    _visibility = widget.program.visibility;
  }

  @override
  void dispose() {
    _name.dispose();
    _description.dispose();
    super.dispose();
  }

  /// The audiences on offer, which are not the same for the two kinds of
  /// program: a club's is either the club's or the world's, while one of
  /// somebody's own can also be theirs alone.
  List<({String value, String label})> _audiences(String Function(String) t) {
    if (widget.clubId.isNotEmpty) {
      return [
        (value: 'club', label: t('programs.private')),
        (value: 'public', label: t('programs.public')),
      ];
    }
    return [
      (value: 'private', label: t('programs.visibilityPrivate')),
      (value: 'club', label: t('programs.visibilityClubs')),
      (value: 'public', label: t('programs.visibilityPublic')),
    ];
  }

  Future<void> _save() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(apiProvider).updateProgram(
            widget.clubId,
            widget.program.id,
            name: _name.text.trim(),
            description: _description.text.trim(),
            visibility: _visibility,
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
    final audiences = _audiences(t);

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(t('programs.edit'), style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 16),
            TextField(
              controller: _name,
              autofocus: true,
              decoration: InputDecoration(labelText: t('programs.name')),
              // The name is what the sheet exists for, so a rename can be
              // finished from the keyboard.
              textInputAction: TextInputAction.done,
              onSubmitted: (_) => _busy || _name.text.trim().isEmpty ? null : _save(),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _description,
              maxLines: 2,
              decoration: InputDecoration(labelText: t('programs.description')),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              initialValue: audiences.any((audience) => audience.value == _visibility)
                  ? _visibility
                  : audiences.first.value,
              isExpanded: true,
              decoration: InputDecoration(labelText: t('programs.visibility')),
              items: [
                for (final audience in audiences)
                  DropdownMenuItem(value: audience.value, child: Text(audience.label)),
              ],
              onChanged: (value) => setState(() => _visibility = value ?? _visibility),
            ),
            if (_error != null) ...[
              const SizedBox(height: 10),
              Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ],
            const SizedBox(height: 16),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: _busy ? null : () => Navigator.of(context).pop(false),
                  child: Text(t('common.cancel')),
                ),
                const SizedBox(width: 8),
                // The API refuses a program with no name, so the button does
                // too rather than letting somebody find out by pressing it.
                FilledButton(
                  onPressed: _busy || _name.text.trim().isEmpty ? null : _save,
                  child: Text(t('common.save')),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SetEditorSheet extends ConsumerStatefulWidget {
  final String clubId;
  final String programId;
  final String dayId;
  final ProgramSet? set;

  const _SetEditorSheet({
    required this.clubId,
    required this.programId,
    required this.dayId,
    this.set,
  });

  @override
  ConsumerState<_SetEditorSheet> createState() => _SetEditorSheetState();
}

class _SetEditorSheetState extends ConsumerState<_SetEditorSheet> {
  late final TextEditingController _reps;
  late final TextEditingController _value;
  late final TextEditingController _notes;

  String _exerciseId = '';
  String _loadMode = 'rpe';
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final set = widget.set;
    _exerciseId = set?.exerciseId ?? '';
    _loadMode = set?.loadMode.isNotEmpty == true ? set!.loadMode : 'rpe';
    _reps = TextEditingController(text: '${set?.reps ?? 5}');
    _value = TextEditingController(text: _initialValue(set));
    _notes = TextEditingController(text: set?.notes ?? '');
  }

  String _initialValue(ProgramSet? set) {
    return switch (_loadMode) {
      'rpe' => set?.rpe != null ? _fmt(set!.rpe!) : '8',
      'percentage' => set?.percentage != null ? _fmt(set!.percentage!) : '75',
      'absolute' => set?.absoluteLoad != null ? _fmt(set!.absoluteLoad!) : '20',
      _ => '',
    };
  }

  @override
  void dispose() {
    for (final controller in [_reps, _value, _notes]) {
      controller.dispose();
    }
    super.dispose();
  }

  Future<void> _save() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final parsed = double.tryParse(_value.text.trim().replaceAll(',', '.'));
    try {
      await ref.read(apiProvider).saveSet(
            widget.clubId,
            widget.programId,
            setId: widget.set?.id,
            dayId: widget.dayId,
            exerciseId: _exerciseId,
            reps: int.tryParse(_reps.text.trim()) ?? 0,
            loadMode: _loadMode,
            rpe: parsed,
            percentage: parsed,
            absoluteLoad: parsed,
            notes: _notes.text.trim(),
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
    final locale = ref.watch(localeProvider);
    final catalog = ref.watch(exercisesProvider).valueOrNull ?? const <Exercise>[];
    final selected = catalog.where((exercise) => exercise.id == _exerciseId).firstOrNull;

    // A bodyweight movement settles the load mode: there is no weight to
    // prescribe, so the other modes would be meaningless.
    if (selected?.bodyweight == true && _loadMode != 'bodyweight') {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) setState(() => _loadMode = 'bodyweight');
      });
    }

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              widget.set != null ? t('programs.editSet') : t('programs.addSet'),
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 16),
            DropdownButtonFormField<String>(
              initialValue: _exerciseId.isEmpty ? null : _exerciseId,
              isExpanded: true,
              decoration: InputDecoration(labelText: t('session.exercise')),
              items: [
                for (final exercise in catalog)
                  DropdownMenuItem(value: exercise.id, child: Text(exercise.label(locale))),
              ],
              onChanged: (value) => setState(() => _exerciseId = value ?? ''),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              initialValue: _loadMode,
              isExpanded: true,
              decoration: InputDecoration(labelText: t('session.loadMode')),
              items: [
                DropdownMenuItem(value: 'rpe', child: Text(t('session.loadModeRpe'))),
                DropdownMenuItem(value: 'percentage', child: Text(t('session.loadModePercentage'))),
                DropdownMenuItem(value: 'absolute', child: Text(t('session.loadModeAbsolute'))),
                DropdownMenuItem(value: 'bodyweight', child: Text(t('session.loadModeBodyweight'))),
              ],
              onChanged: selected?.bodyweight == true
                  ? null
                  : (value) => setState(() {
                        _loadMode = value ?? 'rpe';
                        _value.text = _initialValue(null);
                      }),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _reps,
                    keyboardType: TextInputType.number,
                    decoration: InputDecoration(labelText: t('session.reps')),
                  ),
                ),
                if (_loadMode != 'bodyweight') ...[
                  const SizedBox(width: 10),
                  Expanded(
                    child: TextField(
                      controller: _value,
                      keyboardType: const TextInputType.numberWithOptions(decimal: true),
                      decoration: InputDecoration(
                        labelText: switch (_loadMode) {
                          'rpe' => t('session.rpe'),
                          'percentage' => t('session.percentage'),
                          _ => t('session.absoluteLoad'),
                        },
                      ),
                    ),
                  ),
                ],
              ],
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _notes,
              decoration: InputDecoration(labelText: t('session.notes')),
            ),
            if (_error != null) ...[
              const SizedBox(height: 10),
              Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ],
            const SizedBox(height: 16),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: _busy ? null : () => Navigator.of(context).pop(false),
                  child: Text(t('common.cancel')),
                ),
                const SizedBox(width: 8),
                FilledButton(
                  onPressed: _busy || _exerciseId.isEmpty ? null : _save,
                  child: Text(t('common.save')),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

String _fmt(double value) => value % 1 == 0 ? value.toStringAsFixed(0) : value.toStringAsFixed(1);

/// Picks who a program is handed to.
///
/// Its own dialog rather than a bare sheet, because the choice needs a
/// confirm step: assigning a block emails everybody picked, which is not
/// something to fire on a stray tap.
class _AssignDialog extends ConsumerStatefulWidget {
  final List<ClubMember> members;

  const _AssignDialog({required this.members});

  @override
  ConsumerState<_AssignDialog> createState() => _AssignDialogState();
}

class _AssignDialogState extends ConsumerState<_AssignDialog> {
  List<String> _picked = [];

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);

    return AlertDialog(
      title: Text(t('programs.assignTo')),
      content: SfMultiSelect(
        label: t('clubs.members'),
        placeholder: t('programs.pickMembers'),
        // The address is in the label so the search finds people by it, and so
        // two members with the same name are still told apart.
        options: [
          for (final member in widget.members)
            SelectOption(value: member.userId, label: '${member.label} (${member.email})'),
        ],
        selected: _picked,
        onChanged: (values) => setState(() => _picked = values),
        searchHint: t('common.search'),
        noResults: t('common.noResults'),
        selectedCount: t('common.selectedCount', {'count': '{{count}}'}),
        clearLabel: t('common.clear'),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(t('common.cancel')),
        ),
        FilledButton(
          onPressed: _picked.isEmpty ? null : () => Navigator.of(context).pop(_picked),
          child: Text(t('programs.assign')),
        ),
      ],
    );
  }
}
