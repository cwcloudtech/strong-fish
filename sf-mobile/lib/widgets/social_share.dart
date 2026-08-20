import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:share_plus/share_plus.dart';

import '../providers/providers.dart';

/// The share button for a post or a profile.
///
/// One button opening the system's own share sheet, rather than a row of
/// network icons: the sheet already lists every app on the phone that can take
/// a link - Instagram, TikTok, WhatsApp, a note to yourself - ordered by who
/// this person actually shares with. A hand-written row can only offer the
/// networks this app happened to think of, and on a phone it would offer them
/// beside the sheet that does the job better.
///
/// It is also the only way to reach the networks that refuse a web share
/// intent. Instagram and TikTok compose from the camera roll and cannot be
/// prefilled from a URL, which is why the web app no longer lists them - but
/// the phone's share sheet hands them the link directly.
class SocialShareRow extends ConsumerWidget {
  final String url;
  final String text;

  const SocialShareRow({super.key, required this.url, required this.text});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);

    return Builder(
      // A Builder so the render box below is this button's own: iPad anchors
      // the share popover to a rect, and the rect has to be the thing that was
      // tapped or the sheet opens in the corner.
      builder: (buttonContext) => IconButton(
        icon: const Icon(Icons.share_outlined, size: 20),
        tooltip: t('share.share'),
        onPressed: () => _share(buttonContext),
      ),
    );
  }

  /// Opens the share sheet.
  ///
  /// The text and the link travel in one field rather than as separate
  /// parameters: an app that takes only text still receives the link, and one
  /// that detects links still turns it into a card.
  Future<void> _share(BuildContext context) async {
    final box = context.findRenderObject() as RenderBox?;

    await SharePlus.instance.share(ShareParams(
      text: text.isEmpty ? url : '$text\n$url',
      sharePositionOrigin:
          box != null && box.hasSize ? box.localToGlobal(Offset.zero) & box.size : null,
    ));
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
