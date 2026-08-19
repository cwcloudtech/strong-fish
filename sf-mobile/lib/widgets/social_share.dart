import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import '../providers/providers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// One network something can be shared to.
///
/// A list of these rather than a switch, so adding a network is adding an entry
/// - the same shape the web app's SOCIAL_NETWORKS uses, and for the same
/// reason: nothing else in the app knows these names.
class SocialNetwork {
  final String id;
  final String label;
  final IconData icon;

  /// Builds the share URL, or null for a network that has no web share intent.
  ///
  /// Instagram and TikTok compose from the device's camera roll and offer no
  /// way to prefill a link, so theirs is null and the button copies instead. A
  /// button opening a composer that silently drops the link would look like it
  /// worked.
  final String Function(String url, String text)? share;

  const SocialNetwork({
    required this.id,
    required this.label,
    required this.icon,
    this.share,
  });
}

String _e(String value) => Uri.encodeComponent(value);

const socialNetworks = <SocialNetwork>[
  SocialNetwork(
    id: 'facebook',
    label: 'Facebook',
    icon: Icons.facebook,
    share: _facebook,
  ),
  SocialNetwork(id: 'x', label: 'X', icon: Icons.alternate_email, share: _x),
  SocialNetwork(id: 'bluesky', label: 'Bluesky', icon: Icons.cloud_outlined, share: _bluesky),
  SocialNetwork(id: 'instagram', label: 'Instagram', icon: Icons.camera_alt_outlined),
  SocialNetwork(id: 'tiktok', label: 'TikTok', icon: Icons.music_note_outlined),
];

String _facebook(String url, String text) => 'https://www.facebook.com/sharer/sharer.php?u=${_e(url)}';
String _x(String url, String text) => 'https://twitter.com/intent/tweet?url=${_e(url)}&text=${_e(text)}';
String _bluesky(String url, String text) => 'https://bsky.app/intent/compose?text=${_e('$text $url')}';

/// The row of share buttons, shared by profiles and posts.
class SocialShareRow extends ConsumerWidget {
  final String url;
  final String text;

  const SocialShareRow({super.key, required this.url, required this.text});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);

    return Wrap(
      spacing: 4,
      children: [
        for (final network in socialNetworks)
          IconButton(
            icon: Icon(network.icon, size: 20),
            tooltip: network.label,
            onPressed: () async {
              final target = network.share?.call(url, text);
              if (target == null) {
                // No web share intent: the link goes to the clipboard and the
                // message says to paste it, which is the honest version of
                // what these networks actually allow.
                await Clipboard.setData(ClipboardData(text: url));
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(t('share.copiedFor', {'network': network.label}))),
                  );
                }
                return;
              }
              final uri = Uri.tryParse(target);
              if (uri != null) await launchUrl(uri, mode: LaunchMode.externalApplication);
            },
          ),
      ],
    );
  }
}

final _urlPattern = RegExp(r'https?://\S+');

/// A post's text, ready to travel with a share.
///
/// The links come out: one of them is the post's own embed, and sending a
/// reader to that instead of to the post is exactly what sharing should not do.
/// The same rule the web app applies, so a post shared from either reads the
/// same.
String shareTextFor(String content, String fallback) {
  final stripped = content.replaceAll(_urlPattern, ' ').replaceAll(RegExp(r'\s+'), ' ').trim();
  if (stripped.isEmpty) return fallback;
  if (stripped.length <= 180) return stripped;

  final cut = stripped.substring(0, 180);
  final lastSpace = cut.lastIndexOf(' ');
  return '${(lastSpace > 150 ? cut.substring(0, lastSpace) : cut).trimRight()}…';
}
