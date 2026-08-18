import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../widgets/common.dart';
import 'program_editor_screen.dart';

/// The coaching tab: the clubs the user manages, and the programs in them.
///
/// It only appears for a coach - an athlete's app has no use for it - and it
/// covers authoring a program by hand. Spreadsheet import stays on the web,
/// where picking an .xlsx off a filesystem is natural.
class CoachScreen extends ConsumerWidget {
  const CoachScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final clubs = ref.watch(clubsProvider);

    return clubs.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => SfErrorState(
        message: ref.read(tErrorProvider)(error),
        retryLabel: t('common.back'),
        onRetry: () => ref.invalidate(clubsProvider),
      ),
      data: (list) {
        final managed = list.where((club) => club.canManage).toList();
        if (managed.isEmpty) {
          return SfEmptyState(
            icon: Icons.groups_outlined,
            title: t('clubs.empty'),
            message: t('clubs.emptyCoach'),
          );
        }

        return RefreshIndicator(
          onRefresh: () async => ref.invalidate(clubsProvider),
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              for (final club in managed) _ClubPrograms(club: club),
            ],
          ),
        );
      },
    );
  }
}

class _ClubPrograms extends ConsumerStatefulWidget {
  final Club club;

  const _ClubPrograms({required this.club});

  @override
  ConsumerState<_ClubPrograms> createState() => _ClubProgramsState();
}

class _ClubProgramsState extends ConsumerState<_ClubPrograms> {
  List<Program>? _programs;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final programs = await ref.read(apiProvider).programs(widget.club.id);
      if (mounted) setState(() => _programs = programs);
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    }
  }

  Future<void> _create() async {
    final t = ref.read(tProvider);
    final name = await showDialog<String>(
      context: context,
      builder: (context) => _NameDialog(title: t('programs.create'), label: t('programs.programName')),
    );
    if (name == null || name.isEmpty) return;

    try {
      final program = await ref.read(apiProvider).createProgram(widget.club.id, name, '');
      if (!mounted) return;
      // An empty program is only useful once it has sessions, so open it
      // straight away rather than dropping back to the list.
      await Navigator.of(context).push(MaterialPageRoute(
        builder: (context) => ProgramEditorScreen(clubId: widget.club.id, programId: program.id),
      ));
      _load();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(ref.read(tErrorProvider)(error))),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(widget.club.name, style: Theme.of(context).textTheme.titleMedium),
                ),
                Chip(label: Text(t('clubs.${widget.club.role}'))),
              ],
            ),
            if (_error != null) Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
            if (_programs == null)
              const Padding(padding: EdgeInsets.all(12), child: LinearProgressIndicator())
            else if (_programs!.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 8),
                child: Text(t('programs.empty'), style: Theme.of(context).textTheme.bodySmall),
              )
            else
              for (final program in _programs!)
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text(program.name),
                  subtitle: Text(
                    '${t('programs.weeks', {'count': '${program.weeks}'})} · '
                    '${t('programs.sessions', {'count': '${program.dayCount}'})} · '
                    '${t('programs.setCount', {'count': '${program.setCount}'})}',
                  ),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () async {
                    await Navigator.of(context).push(MaterialPageRoute(
                      builder: (context) =>
                          ProgramEditorScreen(clubId: widget.club.id, programId: program.id),
                    ));
                    _load();
                  },
                ),
            const SizedBox(height: 8),
            OutlinedButton.icon(
              icon: const Icon(Icons.add),
              label: Text(t('programs.create')),
              onPressed: _create,
            ),
          ],
        ),
      ),
    );
  }
}

/// A single-field prompt, used for the handful of "just give me a name" flows.
class _NameDialog extends StatefulWidget {
  final String title;
  final String label;

  const _NameDialog({required this.title, required this.label});

  @override
  State<_NameDialog> createState() => _NameDialogState();
}

class _NameDialogState extends State<_NameDialog> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.title),
      content: TextField(
        controller: _controller,
        autofocus: true,
        decoration: InputDecoration(labelText: widget.label),
        onSubmitted: (value) => Navigator.of(context).pop(value.trim()),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(_controller.text.trim()),
          child: const Text('OK'),
        ),
      ],
    );
  }
}
