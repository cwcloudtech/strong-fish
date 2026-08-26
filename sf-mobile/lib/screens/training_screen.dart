import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
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

        // What is still being run first; what is finished stays below it
        // rather than disappearing, because the comments left on a block are
        // the record of how it went. Sorted, not filtered - and the sort is
        // stable, so the newest block is still first within each group.
        final ordered = _byStatus(list);

        return RefreshIndicator(
          onRefresh: () async => ref.invalidate(assignmentsProvider),
          child: ListView.builder(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.all(16),
            itemCount: ordered.length,
            itemBuilder: (context, index) {
              final assignment = ordered[index];
              final progress = assignment.totalSets == 0
                  ? 0.0
                  : assignment.completedSets / assignment.totalSets;
              final colors = AppColors.of(context);
              final history = assignment.status.isNotEmpty && assignment.status != 'active';

              return Card(
                // A block no longer being run is coloured as history: still
                // readable and still open-able, which is the point of keeping
                // it, but visibly not the one to train today.
                color: history ? colors.backgroundMuted : null,
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
                                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                                      color: history ? colors.textMuted : null,
                                    ),
                              ),
                            ),
                            Chip(
                              label: Text(t('session.status${_statusKey(assignment.status)}')),
                              visualDensity: VisualDensity.compact,
                              backgroundColor: _statusTint(assignment.status, colors),
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

/// Active blocks first, then finished, then archived.
List<Assignment> _byStatus(List<Assignment> assignments) {
  const known = ['active', 'done', 'archived'];
  // Every assignment lands in exactly one bucket, including one carrying a
  // status this build has never heard of - it trains like an active block, so
  // it belongs with them rather than being dropped or listed twice.
  String bucket(String status) => known.contains(status) ? status : 'active';

  // Grouped rather than sorted: List.sort is not stable, and a sort that
  // shuffled the newest block out of the top of its own group would undo the
  // order the API already put them in.
  return [
    for (final status in known) ...assignments.where((a) => bucket(a.status) == status),
  ];
}

/// The colour a status wears: the one in progress stands out, the finished one
/// reads as done, the archived one recedes.
Color? _statusTint(String status, AppColors colors) => switch (status) {
      'done' => colors.success.withValues(alpha: 0.18),
      'archived' => null,
      _ => colors.primary.withValues(alpha: 0.18),
    };

/// Maps an assignment status onto its i18n key suffix.
String _statusKey(String status) => switch (status) {
      'done' => 'Done',
      'archived' => 'Archived',
      _ => 'Active',
    };
