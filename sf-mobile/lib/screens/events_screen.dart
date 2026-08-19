import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/linkified_text.dart';

/// The calendar: meets, club sessions and camps.
///
/// It is read-only here on purpose. Adding an event is a coach's desk job -
/// dates, a location, an entry link - and the web app is where that belongs;
/// what an athlete needs on a phone is to see what is coming, which is also
/// what the ICS subscription is for.
class EventsScreen extends ConsumerWidget {
  const EventsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final events = ref.watch(eventsProvider);

    return RefreshIndicator(
      onRefresh: () async => ref.invalidate(eventsProvider),
      child: events.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => SfErrorState(
          message: ref.read(tErrorProvider)(error),
          onRetry: () => ref.invalidate(eventsProvider),
          retryLabel: t('common.back'),
        ),
        data: (list) {
          if (list.isEmpty) {
            return SfEmptyState(
              icon: Icons.event_outlined,
              title: t('events.emptyTitle'),
              message: t('events.emptyBody'),
            );
          }
          return ListView.builder(
            padding: const EdgeInsets.all(16),
            itemCount: list.length,
            itemBuilder: (context, index) => _EventCard(event: list[index]),
          );
        },
      ),
    );
  }
}

class _EventCard extends ConsumerWidget {
  final Event event;

  const _EventCard({required this.event});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                // The colour its author picked, shown as a dot rather than by
                // tinting the chip: the chip already carries the kind, and two
                // meanings in one swatch reads as neither.
                if (_eventColor() != null) ...[
                  Container(
                    width: 10,
                    height: 10,
                    decoration: BoxDecoration(color: _eventColor(), shape: BoxShape.circle),
                  ),
                  const SizedBox(width: 8),
                ],
                Chip(
                  label: Text(t('events.kind.${event.kind}')),
                  backgroundColor: _kindColor(colors, event.kind),
                  visualDensity: VisualDensity.compact,
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(event.title, style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 4),
            Text(_when(event), style: TextStyle(color: colors.textMuted)),
            if (event.location.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 2),
                child: Text(event.location, style: TextStyle(color: colors.textMuted)),
              ),
            if (event.clubName.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 2),
                child: Text(event.clubName, style: TextStyle(color: colors.textMuted, fontSize: 12)),
              ),
            if (event.description.isNotEmpty) ...[
              const SizedBox(height: 8),
              LinkifiedText(event.description),
            ],
            if (event.url.isNotEmpty) ...[
              const SizedBox(height: 8),
              TextButton.icon(
                icon: const Icon(Icons.open_in_new, size: 16),
                label: Text(t('events.moreInfo')),
                onPressed: () => launchUrl(Uri.parse(event.url), mode: LaunchMode.externalApplication),
              ),
            ],
          ],
        ),
      ),
    );
  }

  /// The event's own colour, or null when it has none or the server sent
  /// something that is not a `#rrggbb` - a bad value drops the dot rather than
  /// throwing on a card that would otherwise render.
  Color? _eventColor() {
    final value = event.color.trim();
    if (!RegExp(r'^#[0-9a-fA-F]{6}$').hasMatch(value)) return null;
    return Color(0xFF000000 | int.parse(value.substring(1), radix: 16));
  }

  Color _kindColor(AppColors colors, String kind) => switch (kind) {
        'competition' => colors.primaryTint,
        'training' => colors.backgroundMuted,
        _ => colors.backgroundMuted,
      };

  /// The date as a reader wants it: a day for an all-day event, a day and a
  /// time otherwise. Deliberately not localized through `intl` - the app
  /// carries no date-formatting dependency, and this is the one place a
  /// numeric date is clearer than a written one anyway.
  String _when(Event event) {
    final start = event.startsAt;
    final day = '${_pad(start.day)}/${_pad(start.month)}/${start.year}';
    // A whole-day entry has no hour to show: either its author marked it so,
    // or it is a birthday, which is derived from a birthdate and never had one.
    if (event.allDay || event.kind == 'birthday') return day;
    return '$day  ${_pad(start.hour)}:${_pad(start.minute)}';
  }

  String _pad(int value) => value.toString().padLeft(2, '0');
}
