import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/providers.dart';
import '../widgets/logo.dart';
import 'coach_screen.dart';
import 'events_screen.dart';
import 'feed_screen.dart';
import 'one_rms_screen.dart';
import 'profile_screen.dart';
import 'training_screen.dart';

/// The app shell: training, maxes, feed, profile - plus a coaching tab for the
/// accounts that can author programs.
class MainTabsScreen extends ConsumerStatefulWidget {
  const MainTabsScreen({super.key});

  @override
  ConsumerState<MainTabsScreen> createState() => _MainTabsScreenState();
}

class _MainTabsScreenState extends ConsumerState<MainTabsScreen> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final isCoach = ref.watch(sessionProvider).user?.isCoach ?? false;

    // The coaching tab is inserted rather than always present, so an athlete's
    // app doesn't carry a tab they can't use.
    final tabs = <({String label, IconData icon, Widget screen})>[
      (label: t('nav.training'), icon: Icons.fitness_center, screen: const TrainingScreen()),
      (label: t('nav.oneRms'), icon: Icons.emoji_events_outlined, screen: const OneRmsScreen()),
      if (isCoach)
        (label: t('programs.title'), icon: Icons.edit_note, screen: const CoachScreen()),
      (label: t('nav.events'), icon: Icons.event_outlined, screen: const EventsScreen()),
      (label: t('nav.feed'), icon: Icons.forum_outlined, screen: const FeedScreen()),
      (label: t('nav.profile'), icon: Icons.person_outline, screen: const ProfileScreen()),
    ];

    // Losing coach rights while a later tab is selected would leave the index
    // past the end of the list.
    final index = _index.clamp(0, tabs.length - 1);
    final feedIndex = tabs.indexWhere((tab) => tab.screen is FeedScreen);

    return Scaffold(
      appBar: AppBar(
        title: Row(
          children: [
            const SfLogo(mark: true, height: 28, on: Brightness.dark),
            const SizedBox(width: 8),
            Text(tabs[index].label),
          ],
        ),
      ),
      body: IndexedStack(index: index, children: [for (final tab in tabs) tab.screen]),
      // The composer is only offered on the feed tab, where it's the obvious
      // action; elsewhere it would just be a floating button with no context.
      floatingActionButton: index == feedIndex
          ? FloatingActionButton(
              onPressed: () async {
                final posted = await Navigator.of(context).push<bool>(
                  MaterialPageRoute(builder: (context) => const ComposePostScreen()),
                );
                // The feed keeps its own paged list, so a new post is picked up
                // by rebuilding it from page 0.
                if (posted == true && mounted) setState(() {});
              },
              child: const Icon(Icons.edit),
            )
          : null,
      bottomNavigationBar: NavigationBar(
        selectedIndex: index,
        onDestinationSelected: (next) => setState(() => _index = next),
        destinations: [
          for (final tab in tabs) NavigationDestination(icon: Icon(tab.icon), label: tab.label),
        ],
      ),
    );
  }
}
