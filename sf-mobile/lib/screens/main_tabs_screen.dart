import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/providers.dart';
import 'feed_screen.dart';
import 'one_rms_screen.dart';
import 'profile_screen.dart';
import 'training_screen.dart';

/// The app shell: training, maxes, feed, profile.
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

    final titles = [t('nav.training'), t('nav.oneRms'), t('nav.feed'), t('nav.profile')];
    const screens = [TrainingScreen(), OneRmsScreen(), FeedScreen(), ProfileScreen()];

    return Scaffold(
      appBar: AppBar(
        title: Row(
          children: [
            Image.asset('assets/images/icon.png', height: 28),
            const SizedBox(width: 8),
            Text(titles[_index]),
          ],
        ),
      ),
      body: IndexedStack(index: _index, children: screens),
      // The composer is only offered on the feed tab, where it's the obvious
      // action; elsewhere it would just be a floating button with no context.
      floatingActionButton: _index == 2
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
        selectedIndex: _index,
        onDestinationSelected: (index) => setState(() => _index = index),
        destinations: [
          NavigationDestination(icon: const Icon(Icons.fitness_center), label: t('nav.training')),
          NavigationDestination(icon: const Icon(Icons.emoji_events_outlined), label: t('nav.oneRms')),
          NavigationDestination(icon: const Icon(Icons.forum_outlined), label: t('nav.feed')),
          NavigationDestination(icon: const Icon(Icons.person_outline), label: t('nav.profile')),
        ],
      ),
    );
  }
}
