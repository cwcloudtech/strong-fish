import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/logo.dart';
import 'admin_users_screen.dart';
import 'ai_coach_screen.dart';
import 'coach_screen.dart';
import 'feed_screen.dart';
import 'invitations_screen.dart';
import 'member_search_screen.dart';
import 'messages_screen.dart';
import 'one_rms_screen.dart';
import 'steps_screen.dart';
import 'strength_screen.dart';
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
    final colors = AppColors.of(context);
    final user = ref.watch(sessionProvider).user;
    final isCoach = user?.isCoach ?? false;
    final isSuperadmin = user?.role == 'superadmin';

    final tabs = <({String label, IconData icon, Widget screen})>[
      (label: t('nav.feed'), icon: Icons.forum_outlined, screen: const FeedScreen()),
      (label: t('nav.training'), icon: Icons.fitness_center, screen: const TrainingScreen()),
      (label: t('nav.oneRms'), icon: Icons.emoji_events_outlined, screen: const OneRmsScreen()),
      (label: t('nav.healthData'), icon: Icons.favorite_outline, screen: const StepsScreen()),
      (label: t('nav.strength'), icon: Icons.leaderboard_outlined, screen: const StrengthScreen()),
      if (isCoach)
        (label: t('programs.title'), icon: Icons.edit_note, screen: const CoachScreen()),
      (label: t('nav.invitations'), icon: Icons.group_add_outlined, screen: const InvitationsScreen()),
      (label: t('nav.messages'), icon: Icons.chat_bubble_outline, screen: const MessagesScreen()),
      (label: t('nav.coach'), icon: Icons.auto_awesome_outlined, screen: const AiCoachScreen()),
      if (isSuperadmin)
        (label: t('nav.admin'), icon: Icons.admin_panel_settings_outlined, screen: const AdminUsersScreen()),
      (label: t('nav.profile'), icon: Icons.person_outline, screen: const ProfileScreen()),
    ];

    final index = _index.clamp(0, tabs.length - 1);
    final messagesIndex = tabs.indexWhere((tab) => tab.screen is MessagesScreen);

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
      floatingActionButton: index == messagesIndex
          ? FloatingActionButton(
        tooltip: t('messages.findSomeone'),
        onPressed: () => Navigator.of(context).push(
          MaterialPageRoute(builder: (context) => const MemberSearchScreen()),
        ),
        child: const Icon(Icons.person_search),
      )
          : null,
      bottomNavigationBar: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const _ConversationStrip(),
          Container(
            decoration: BoxDecoration(
              color: colors.surface,
              border: Border(top: BorderSide(color: colors.border)),
            ),
            child: SafeArea(
              top: false,
              child: SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: ConstrainedBox(
                  constraints: BoxConstraints(minWidth: MediaQuery.of(context).size.width),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                    children: [
                      for (var i = 0; i < tabs.length; i++)
                        _NavItem(
                          label: tabs[i].label,
                          icon: tabs[i].icon,
                          active: i == index,
                          badge: tabs[i].screen is MessagesScreen,
                          onTap: () => setState(() => _index = i),
                        ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// The conversations already open, as a scrollable row of avatars pinned above
/// the navigation bar.
///
/// It renders nothing at all when there are none: an empty strip would be a
/// permanent band of dead space at the bottom of every screen.
class _ConversationStrip extends ConsumerWidget {
  const _ConversationStrip();

  /// Enough to be a shortcut, few enough that it does not become a second
  /// messages screen glued to the bottom of the app.
  static const _max = 8;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final conversations = ref.watch(conversationsProvider).valueOrNull ?? const <Conversation>[];
    if (conversations.isEmpty) return const SizedBox.shrink();

    final shown = conversations.take(_max).toList();
    final colors = AppColors.of(context);

    return Container(
      height: 62,
      decoration: BoxDecoration(
        color: colors.surface,
        border: Border(top: BorderSide(color: colors.border)),
      ),
      child: ListView.builder(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        itemCount: shown.length,
        itemBuilder: (context, index) {
          final conversation = shown[index];
          final name = '${conversation.other.name} ${conversation.other.surname}'.trim();

          return Padding(
            padding: const EdgeInsets.only(right: 12),
            child: Tooltip(
              message: name,
              child: InkWell(
                onTap: () => Navigator.of(context).push(MaterialPageRoute(
                  builder: (_) => ThreadScreen(userId: conversation.other.id, title: name),
                )),
                child: Badge(
                  isLabelVisible: conversation.unread > 0,
                  label: Text('${conversation.unread}'),
                  child: SfAvatar.of(conversation.other, radius: 20),
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}

/// The unread count, on the messages destination.
class _UnreadBadge extends ConsumerWidget {
  final Widget child;

  const _UnreadBadge({required this.child});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final unread = ref.watch(conversationsProvider).valueOrNull?.fold<int>(
      0,
          (total, conversation) => total + conversation.unread,
    ) ??
        0;

    return Badge(
      isLabelVisible: unread > 0,
      label: Text('$unread'),
      child: child,
    );
  }
}

/// One destination in the sliding bar.
///
/// A fixed width is what makes the row scroll rather than compress: give the
/// items flexible widths and they shrink to fit instead of overflowing, which
/// is exactly the squeezed, unreadable bar this replaces.
class _NavItem extends ConsumerWidget {
  final String label;
  final IconData icon;
  final bool active;
  final bool badge;
  final VoidCallback onTap;

  const _NavItem({
    required this.label,
    required this.icon,
    required this.active,
    required this.badge,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = AppColors.of(context);
    final color = active ? colors.primary : colors.textMuted;

    return InkWell(
      onTap: onTap,
      child: SizedBox(
        width: 74,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              badge
                  ? _UnreadBadge(child: Icon(icon, color: color, size: 24))
                  : Icon(icon, color: color, size: 24),
              const SizedBox(height: 4),
              Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  color: color,
                  fontSize: 11,
                  fontWeight: active ? FontWeight.w600 : FontWeight.w400,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}