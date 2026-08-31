import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';

/// What a total is worth in one line: the tier it falls in, and where it sits
/// among the lifters here.
///
/// Split from the badges because the two answer different questions. This one
/// is about a number somebody just typed - the calculator's whole job - while
/// a badge is something a member has *earned*, which belongs on the profile
/// that earned it.
class StrengthSummary extends ConsumerWidget {
  final StrengthResult result;

  const StrengthSummary({super.key, required this.result});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        StrengthTier(result: result),

        // Where this score sits among the members of this deployment - not a
        // published curve fitted on international meets, which would tell a
        // beginners' gym that everybody in it is bottom decile.
        if (result.sample > 0) ...[
          const SizedBox(height: 12),
          Row(
            crossAxisAlignment: CrossAxisAlignment.baseline,
            textBaseline: TextBaseline.alphabetic,
            children: [
              Text(t('strength.strongerThan'), style: Theme.of(context).textTheme.bodySmall),
              const SizedBox(width: 8),
              Text(
                '${result.percentile}%',
                style: Theme.of(context)
                    .textTheme
                    .titleLarge
                    ?.copyWith(fontWeight: FontWeight.bold, color: colors.primary),
              ),
            ],
          ),
          const SizedBox(height: 6),
          ClipRRect(
            borderRadius: BorderRadius.circular(999),
            child: LinearProgressIndicator(
              value: result.percentile / 100,
              minHeight: 8,
              backgroundColor: colors.backgroundMuted,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            t('strength.percentile', {
              'value': '${result.percentile}',
              'sample': '${result.sample}',
            }),
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ] else ...[
          const SizedBox(height: 8),
          Text(t('strength.noPopulation'), style: Theme.of(context).textTheme.bodySmall),
        ],
      ],
    );
  }
}

/// What a lifter has earned.
///
/// Shown on a profile, not on the calculator: a badge is something a member won
/// with their own recorded maxes, and awarding one for a number somebody typed
/// into a form would make it worth nothing.
class StrengthBadges extends ConsumerWidget {
  final StrengthResult result;

  /// A profile shows what has been won; pass false to show the locked ones too.
  final bool earnedOnly;

  const StrengthBadges({super.key, required this.result, this.earnedOnly = true});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final shown = earnedOnly
        ? result.earned
        : [
            ...result.earned,
            ...result.badges.where((badge) => !badge.earned && badge.progress > 0),
          ];

    if (shown.isEmpty) {
      return Text(
        ref.watch(tProvider)('strength.noBadges'),
        style: Theme.of(context).textTheme.bodySmall,
      );
    }
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [for (final badge in shown) _BadgeChip(badge: badge)],
    );
  }
}

/// The tier a DOTS score falls in - "Platform Contender", "Titan" - as the one
/// badge that gets a colour of its own.
///
/// Where the lifter sits among the members here is its tooltip rather than a
/// line of its own: on a profile the tier is the headline, and a percentile
/// printed beside it competes with the thing it is describing. The calculator
/// draws the same number as a bar, because there it *is* the answer.
class StrengthTier extends ConsumerWidget {
  final StrengthResult result;
  final bool showScore;

  const StrengthTier({super.key, required this.result, this.showScore = true});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (result.tierKey.isEmpty) return const SizedBox.shrink();

    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final ink = switch (result.tierKey) {
      'intermediate' => colors.primary,
      'advanced' => colors.success,
      'elite' => colors.warning,
      'worldClass' => colors.danger,
      _ => colors.textMuted,
    };
    final percentile = result.sample > 0
        ? t('strength.percentile', {
            'value': '${result.percentile}',
            'sample': '${result.sample}',
          })
        : t('strength.noPopulation');

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Tooltip(
          message: percentile,
          triggerMode: TooltipTriggerMode.tap,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(999),
              color: ink.withValues(alpha: 0.16),
            ),
            child: Text(
              t('strength.tiers.${result.tierKey}'),
              style: TextStyle(color: ink, fontWeight: FontWeight.w700, fontSize: 12, letterSpacing: 0.3),
            ),
          ),
        ),
        if (showScore) ...[
          const SizedBox(width: 8),
          Text(
            t('strength.tierFrom', {'value': _fmt(result.dots)}),
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      ],
    );
  }
}

class _BadgeChip extends ConsumerWidget {
  final StrengthBadge badge;

  const _BadgeChip({required this.badge});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = AppColors.of(context);
    final ink = badge.earned ? colors.success : colors.textMuted;

    return Container(
      constraints: const BoxConstraints(minWidth: 130),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(10),
        color: badge.earned ? colors.success.withValues(alpha: 0.16) : colors.backgroundMuted,
        border: Border.all(color: badge.earned ? Colors.transparent : colors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            ref.watch(tProvider)('strength.badges.${badge.key}'),
            style: TextStyle(color: ink, fontWeight: FontWeight.w700, fontSize: 12),
          ),
          // A locked badge is a distance to close, and the bar is the distance.
          if (!badge.earned) ...[
            const SizedBox(height: 5),
            ClipRRect(
              borderRadius: BorderRadius.circular(999),
              child: LinearProgressIndicator(
                value: badge.progress,
                minHeight: 4,
                backgroundColor: colors.border,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

String _fmt(double value) =>
    value % 1 == 0 ? value.toStringAsFixed(0) : value.toStringAsFixed(2);
