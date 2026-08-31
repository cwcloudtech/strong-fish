import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/profile_badges.dart';
import '../widgets/strength_badges.dart';
import '../widgets/social_share.dart';
import '../widgets/socials.dart';
import 'messages_screen.dart';

/// Somebody else's profile, opened from a message, a post, or a search result.
///
/// Read-only and deliberately short: bests, clubs and a follow button. The rest
/// of what a profile carries is on the web, where there is room for it.
///
/// A profile the caller may not see answers 404, and this says so plainly
/// rather than showing an empty shell - the visibility rules run in the API,
/// and this screen only reports what they decided.
class PublicProfileScreen extends ConsumerStatefulWidget {
  final String handle;

  const PublicProfileScreen({super.key, required this.handle});

  @override
  ConsumerState<PublicProfileScreen> createState() => _PublicProfileScreenState();
}

class _PublicProfileScreenState extends ConsumerState<PublicProfileScreen> {
  PublicProfile? _profile;
  bool _loading = true;
  String? _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final profile = await ref.read(apiProvider).profile(widget.handle);
      if (mounted) setState(() => _profile = profile);
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _toggleFollow() async {
    final profile = _profile;
    if (profile == null) return;
    setState(() => _busy = true);
    try {
      await ref.read(apiProvider).follow(profile.handle, profile.followed);
      await _load();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final profile = _profile;

    return Scaffold(
      appBar: AppBar(title: Text(profile?.fullName ?? t('profile.title'))),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              // Every state pulls to refresh, not just the loaded one: a
              // profile that failed to load is exactly where somebody pulls.
              ? RefreshIndicator(
                  onRefresh: _load,
                  child: SfRefreshableBody(
                    child: SfErrorState(
                        message: _error!, onRetry: _load, retryLabel: t('common.back')),
                  ),
                )
              : profile == null
                  ? RefreshIndicator(
                      onRefresh: _load,
                      child: SfRefreshableBody(
                        child: SfEmptyState(
                            icon: Icons.person_off_outlined, title: t('profile.notVisible')),
                      ),
                    )
                  : RefreshIndicator(
                      onRefresh: _load,
                      child: ListView(
                        physics: const AlwaysScrollableScrollPhysics(),
                      padding: const EdgeInsets.all(16),
                      children: [
                        Row(
                          children: [
                            SfAvatar(picture: profile.picture, name: profile.fullName, radius: 32),
                            const SizedBox(width: 16),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(profile.fullName,
                                      style: Theme.of(context).textTheme.titleLarge),
                                  if (profile.handle.isNotEmpty)
                                    Text('@${profile.handle}',
                                        style: TextStyle(color: colors.textMuted)),
                                  const SizedBox(height: 6),
                                  // Standing and specialty, each in its own
                                  ProfileBadges(
                                    role: profile.role,
                                    alignment: MainAxisAlignment.start,
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                        if (profile.bio.isNotEmpty) ...[
                          const SizedBox(height: 12),
                          Text(profile.bio),
                        ],
                        if (profile.socials.isNotEmpty) ...[
                          const SizedBox(height: 12),
                          SocialLinks(socials: profile.socials, alignment: WrapAlignment.start),
                        ],
                        // What their own maxes are worth: the tier, where it
                        // sits among the members here, and what it has earned.
                        // Absent for somebody who has not weighed in or
                        // entered no lifts - a profile with nothing to measure
                        // wears nothing.
                        if (profile.strength != null) ...[
                          const SizedBox(height: 16),
                          // The tier is the headline; where they sit among
                          // the club is its tooltip, so it does not compete
                          // with the thing it describes.
                          StrengthTier(result: profile.strength!),
                          const SizedBox(height: 10),
                          StrengthBadges(result: profile.strength!),
                        ],
                        const SizedBox(height: 16),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                          children: [
                            _Stat(value: '${profile.followers}', label: t('profile.followers')),
                            _Stat(value: '${profile.following}', label: t('profile.following')),
                            if (profile.total > 0)
                              _Stat(value: '${profile.total.round()} kg', label: t('profile.total')),
                          ],
                        ),
                        const SizedBox(height: 16),
                        // Neither action makes sense on your own profile, and
                        // the API refuses both: you cannot follow or message
                        // yourself.
                        if (profile.id != ref.watch(sessionProvider).user?.id)
                          Row(
                            children: [
                              Expanded(
                                child: FilledButton(
                                  onPressed: _busy ? null : _toggleFollow,
                                  child: Text(profile.followed
                                      ? t('profile.unfollow')
                                      : t('profile.follow')),
                                ),
                              ),
                              const SizedBox(width: 8),
                              // Reaching this page at all means the profile is
                              // visible to the caller, which is exactly the
                              // API's condition for being allowed to message
                              // its owner - the rule the web button carries.
                              Expanded(
                                child: OutlinedButton.icon(
                                  onPressed: () => Navigator.of(context).push(MaterialPageRoute(
                                    builder: (_) => ThreadScreen(
                                      userId: profile.id,
                                      title: profile.fullName,
                                    ),
                                  )),
                                  icon: const Icon(Icons.chat_bubble_outline),
                                  label: Text(t('messages.message')),
                                ),
                              ),
                            ],
                          ),
                        if (profile.bests.isNotEmpty) ...[
                          const Divider(height: 32),
                          Text(t('profile.bests'), style: Theme.of(context).textTheme.titleMedium),
                          for (final best in profile.bests)
                            ListTile(
                              dense: true,
                              title: Text(best.label(ref.watch(localeProvider))),
                              trailing: Text('${best.value.round()} kg'),
                            ),
                        ],
                        const Divider(height: 32),
                        Text(t('share.label'), style: Theme.of(context).textTheme.titleMedium),
                        const SizedBox(height: 4),
                        SocialShareRow(
                          url: '${ref.read(apiProvider).client.frontendUrl}/profile/${profile.handle}',
                          text: t('share.profileText', {'name': profile.fullName}),
                        ),
                      ],
                    ),
                    ),
    );
  }
}

class _Stat extends StatelessWidget {
  final String value;
  final String label;

  const _Stat({required this.value, required this.label});

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Text(value,
            style: Theme.of(context)
                .textTheme
                .titleMedium
                ?.copyWith(fontWeight: FontWeight.bold, color: Theme.of(context).colorScheme.primary)),
        Text(label, style: Theme.of(context).textTheme.bodySmall),
      ],
    );
  }
}
