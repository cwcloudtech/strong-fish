import 'package:flutter/material.dart';

import '../theme.dart';

/// The model's answers, which arrive as markdown.
///
/// A small renderer rather than a markdown package: what the coach skill
/// produces is headings, bullets, bold and the odd italic, and the answer is
/// read inside a chat bubble. Pulling in a full CommonMark engine to draw
/// four constructs would be a dependency to keep up to date for tables
/// nobody writes.
///
/// What is not understood is shown as what it says: an unknown marker is
/// stripped rather than printed, so the worst case is plain prose.
class MarkdownText extends StatelessWidget {
  final String text;
  final TextStyle? style;

  const MarkdownText(this.text, {super.key, this.style});

  @override
  Widget build(BuildContext context) {
    final colors = AppColors.of(context);
    final base = style ?? Theme.of(context).textTheme.bodyMedium ?? const TextStyle();

    final blocks = <Widget>[];
    for (final raw in text.split('\n')) {
      final line = raw.trim();
      if (line.isEmpty) {
        // One gap between paragraphs, never a run of them: a model that
        // leaves three blank lines should not leave three on screen.
        if (blocks.isNotEmpty) blocks.add(const SizedBox(height: 8));
        continue;
      }

      if (line.startsWith('#')) {
        blocks.add(Padding(
          padding: const EdgeInsets.only(top: 2, bottom: 2),
          child: Text(
            _inline(line.replaceFirst(RegExp(r'^#+\s*'), '')),
            style: base.copyWith(fontWeight: FontWeight.w700, color: colors.text),
          ),
        ));
        continue;
      }

      final bullet = RegExp(r'^[-*]\s+').firstMatch(line);
      if (bullet != null) {
        blocks.add(Padding(
          padding: const EdgeInsets.only(left: 2, bottom: 2),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('•  ', style: base),
              Expanded(child: _rich(_inline(line.substring(bullet.end)), base, colors)),
            ],
          ),
        ));
        continue;
      }

      blocks.add(_rich(_inline(line), base, colors));
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: blocks,
    );
  }

  /// Strips the markers this renderer does not draw as emphasis, so none of
  /// them is left on screen as punctuation.
  String _inline(String value) => value.replaceAll('`', '').replaceAll('~~', '');

  /// A line with its **bold** runs drawn bold. Split on the marker rather than
  /// parsed: bold is the only inline the skill asks for, and an unmatched
  /// marker simply leaves the rest of the line plain.
  Widget _rich(String line, TextStyle base, AppColors colors) {
    final parts = line.split('**');
    return Text.rich(
      TextSpan(
        children: [
          for (var i = 0; i < parts.length; i++)
            TextSpan(
              text: parts[i],
              style: i.isOdd ? base.copyWith(fontWeight: FontWeight.w700) : base,
            ),
        ],
      ),
      style: base.copyWith(color: colors.text),
    );
  }
}
