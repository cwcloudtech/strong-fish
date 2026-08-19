import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';

import '../theme.dart';
import 'media_player.dart';

/// Any http(s) run up to the next whitespace - the same pattern the web
/// frontend linkifies with, so a post reads the same on both.
final _urlPattern = RegExp(r'https?://[^\s]+');

/// Punctuation that ends a sentence rather than a URL.
const _trailing = '.,;:!?)]}"\'';

/// Renders [text] with any http(s) URL turned into a tappable link.
///
/// Tapping opens the URL in the in-app web view rather than handing it to the
/// browser: it is the same page the media player opens, and staying in the app
/// means the reader comes back to the feed where they left it.
///
/// Trailing punctuation is kept out of the target, so "see https://x.com."
/// doesn't try to open a URL ending in a period.
class LinkifiedText extends StatefulWidget {
  final String text;
  final TextStyle? style;

  const LinkifiedText(this.text, {super.key, this.style});

  @override
  State<LinkifiedText> createState() => _LinkifiedTextState();
}

class _LinkifiedTextState extends State<LinkifiedText> {
  final List<TapGestureRecognizer> _recognizers = [];

  @override
  void dispose() {
    _disposeRecognizers();
    super.dispose();
  }

  /// A recognizer holds a gesture arena entry; dropping one without disposing
  /// it leaks, which is why the previous batch goes before every rebuild.
  void _disposeRecognizers() {
    for (final recognizer in _recognizers) {
      recognizer.dispose();
    }
    _recognizers.clear();
  }

  @override
  Widget build(BuildContext context) {
    _disposeRecognizers();
    final colors = AppColors.of(context);
    final text = widget.text;
    final spans = <InlineSpan>[];
    var start = 0;

    for (final match in _urlPattern.allMatches(text)) {
      if (match.start > start) {
        spans.add(TextSpan(text: text.substring(start, match.start)));
      }

      var url = match.group(0)!;
      var tail = '';
      while (url.isNotEmpty && _trailing.contains(url[url.length - 1])) {
        tail = url[url.length - 1] + tail;
        url = url.substring(0, url.length - 1);
      }

      // An empty match is punctuation only - nothing to link to.
      if (url.isEmpty) {
        spans.add(TextSpan(text: tail));
        start = match.end;
        continue;
      }

      final target = url;
      final recognizer = TapGestureRecognizer()
        ..onTap = () => Navigator.of(context).push(
              MaterialPageRoute(builder: (_) => WebViewPage(url: target)),
            );
      _recognizers.add(recognizer);

      spans.add(TextSpan(
        text: url,
        // Colour alone, no underline: a feed of posts full of underlined URLs
        // reads as noise, and this matches how the web frontend draws them.
        style: TextStyle(color: colors.primary),
        recognizer: recognizer,
      ));
      if (tail.isNotEmpty) spans.add(TextSpan(text: tail));
      start = match.end;
    }

    if (start < text.length) spans.add(TextSpan(text: text.substring(start)));

    return Text.rich(TextSpan(style: widget.style ?? DefaultTextStyle.of(context).style, children: spans));
  }
}
