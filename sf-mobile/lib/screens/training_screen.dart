import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/providers.dart';
import '../widgets/common.dart';

/// The programs a coach has assigned to the connected member.
class TrainingScreen extends ConsumerWidget {
  const TrainingScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final assignments = ref.watch(assignmentsProvider);

    return assignments.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => SfErrorState(
        message: ref.read(tErrorProvider)(error),
        retryLabel: t('common.back'),
        onRetry: () => ref.invalidate(assignmentsProvider),
      ),
      data: (list) {
        if (list.isEmpty) {
          return SfEmptyState(
            icon: Icons.fitness_center,
            title: t('session.noAssignments'),
            message: t('session.noAssignmentsHelp'),
          );
        }

        return RefreshIndicator(
          onRefresh: () async => ref.invalidate(assignmentsProvider),
          child: ListView.builder(
            padding: const EdgeInsets.all(16),
            itemCount: list.length,
            itemBuilder: (context, index) {
              final assignment = list[index];
              final progress = assignment.totalSets == 0
                  ? 0.0
                  : assignment.completedSets / assignment.totalSets;

              return Card(
                child: InkWell(
                  borderRadius: BorderRadius.circular(12),
                  onTap: () => context.push('/training/${assignment.id}'),
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                assignment.programName,
                                style: Theme.of(context).textTheme.titleMedium,
                              ),
                            ),
                            Chip(
                              label: Text(t('session.status${_statusKey(assignment.status)}')),
                              visualDensity: VisualDensity.compact,
                            ),
                          ],
                        ),
                        Text(assignment.clubName, style: Theme.of(context).textTheme.bodySmall),
                        if (assignment.note.isNotEmpty) ...[
                          const SizedBox(height: 6),
                          Text(assignment.note),
                        ],
                        const SizedBox(height: 12),
                        LinearProgressIndicator(value: progress, minHeight: 6),
                        const SizedBox(height: 6),
                        Text(
                          t('session.progress', {
                            'done': '${assignment.completedSets}',
                            'total': '${assignment.totalSets}',
                          }),
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                      ],
                    ),
                  ),
                ),
              );
            },
          ),
        );
      },
    );
  }
}

/// Maps an assignment status onto its i18n key suffix.
String _statusKey(String status) => switch (status) {
      'done' => 'Done',
      'archived' => 'Archived',
      _ => 'Active',
    };
