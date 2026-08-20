import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../providers/providers.dart';
import '../theme.dart';

/// The accounts a member can show on their profile.
///
/// The same table sf-ui keeps (utils/socialProfiles.js), for the same reason:
/// the profile form and the row of links on a profile are both generated from
/// it, so adding a network is adding an entry rather than editing two screens.
///
/// The API stores the account's own name - "marie.lifts", never a URL - so
/// [link] is what turns one back into an address, and the base belongs here
/// beside the icon rather than in the payload.
class SocialProfile {
  final String key;
  final String label;
  final IconData icon;
  final String placeholder;
  final String Function(String account) link;

  /// The key of a second field, for the one entry that has one: a placing read
  /// off OpenPowerlifting, which this app never computes.
  final String? rankKey;

  const SocialProfile({
    required this.key,
    required this.label,
    required this.icon,
    required this.placeholder,
    required this.link,
    this.rankKey,
  });
}

const socialProfiles = <SocialProfile>[
  SocialProfile(
    key: 'instagram',
    label: 'Instagram',
    icon: Icons.camera_alt_outlined,
    placeholder: 'marie.lifts',
    link: _instagram,
  ),
  SocialProfile(
    key: 'tiktok',
    label: 'TikTok',
    icon: Icons.music_note_outlined,
    placeholder: 'marie.lifts',
    link: _tiktok,
  ),
  SocialProfile(
    key: 'x',
    label: 'X',
    icon: Icons.close,
    placeholder: 'marielifts',
    link: _x,
  ),
  SocialProfile(
    key: 'bluesky',
    label: 'Bluesky',
    icon: Icons.cloud_outlined,
    placeholder: 'marie.bsky.social',
    link: _bluesky,
  ),
  SocialProfile(
    key: 'openpowerlifting',
    label: 'OpenPowerlifting',
    // The strength icon: this one is the federation results database rather
    // than a social network, and a barbell says so at a glance.
    icon: Icons.fitness_center,
    placeholder: 'mariedubois',
    link: _openpowerlifting,
    rankKey: 'openpowerliftingRank',
  ),
];

String _instagram(String account) => 'https://www.instagram.com/$account';
String _tiktok(String account) => 'https://www.tiktok.com/@$account';
String _x(String account) => 'https://x.com/$account';
String _bluesky(String account) => 'https://bsky.app/profile/$account';
String _openpowerlifting(String account) => 'https://www.openpowerlifting.org/u/$account';

/// The row of links a profile carries, or nothing at all when it carries none.
class SocialLinks extends ConsumerWidget {
  final Map<String, String> socials;
  final WrapAlignment alignment;

  const SocialLinks({super.key, required this.socials, this.alignment = WrapAlignment.center});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);

    final filled = socialProfiles.where((network) => (socials[network.key] ?? '').isNotEmpty).toList();
    if (filled.isEmpty) return const SizedBox.shrink();

    return Wrap(
      spacing: 8,
      runSpacing: 8,
      alignment: alignment,
      children: [
        for (final network in filled)
          ActionChip(
            avatar: Icon(network.icon, size: 16, color: colors.textMuted),
            // The rank is what somebody wants to read on an OpenPowerlifting
            // chip; every other one says which network it is.
            label: Text(
              network.rankKey != null && (socials[network.rankKey!] ?? '').isNotEmpty
                  ? '${t('profile.rank')} ${socials[network.rankKey!]}'
                  : network.label,
              style: const TextStyle(fontSize: 12),
            ),
            visualDensity: VisualDensity.compact,
            onPressed: () => launchUrl(
              Uri.parse(network.link(socials[network.key]!)),
              mode: LaunchMode.externalApplication,
            ),
          ),
      ],
    );
  }
}
