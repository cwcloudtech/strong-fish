import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';

/// A lifter's tier, where they sit among the members here, and their badges.
///
/// The API decides all of it (internal/strength); this renders what it was
/// handed and translates the keys. Locked badges are shown alongside the earned
/// ones with how far along they are - a target you can see is worth more than
/// one that appears out of nowhere the day you hit it.
class StrengthBadges extends ConsumerWidget {
  final StrengthResult result;

  /// A profile shows what has been won; the calculator shows what is left too.
  final bool earnedOnly;

  const StrengthBadges({super.key, required this.result, this.earnedOnly = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final shown = earnedOnly
        ? result.earned
        : [
            ...result.earned,
            ...result.badges.where((badge) => !badge.earned && badge.progress > 0),
          ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (result.tierKey.isNotEmpty)
          Row(
            children: [
              _Tier(tierKey: result.tierKey),
              const SizedBox(width: 8),
              Text(
                t('strength.tierFrom', {'value': _fmt(result.dots)}),
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
          ),

        // Where this score sits among the members of this deployment - not a
        // published curve fitted on international meets, which would tell a
        // beginners' gym that everybody in it is bottom decile.
        if (result.sample > 0) ...[
          const SizedBox(height: 10),
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
        ],

        const SizedBox(height: 12),
        if (shown.isEmpty)
          Text(t('strength.noBadges'), style: Theme.of(context).textTheme.bodySmall)
        else
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [for (final badge in shown) _BadgeChip(badge: badge)],
          ),
      ],
    );
  }
}

/// The tier, which is the one badge that gets a colour of its own: it is a
/// thing to be proud of rather than a row in a list.
class _Tier extends ConsumerWidget {
  final String tierKey;

  const _Tier({required this.tierKey});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = AppColors.of(context);
    final ink = switch (tierKey) {
      'intermediate' => colors.primary,
      'advanced' => colors.success,
      'elite' => colors.warning,
      'worldClass' => colors.danger,
      _ => colors.textMuted,
    };

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(999),
        color: ink.withValues(alpha: 0.16),
      ),
      child: Text(
        ref.watch(tProvider)('strength.tiers.$tierKey'),
        style: TextStyle(color: ink, fontWeight: FontWeight.w700, fontSize: 12, letterSpacing: 0.3),
      ),
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
