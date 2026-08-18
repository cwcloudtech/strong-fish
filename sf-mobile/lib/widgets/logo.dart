import 'package:flutter/material.dart';

/// The wordmark, inked for the current theme.
///
/// The stock logo is drawn in navy for a light background and would all but
/// disappear on a dark one, so dark mode gets the light-inked variant instead -
/// the same pair of assets sf-ui uses.
///
/// [on] pins the variant for a surface whose colour doesn't follow the theme:
/// the app bar is navy in both themes, so its mark is always the light-inked one.
class SfLogo extends StatelessWidget {
  final bool mark;
  final double? height;
  final Brightness? on;

  const SfLogo({super.key, this.mark = false, this.height, this.on});

  @override
  Widget build(BuildContext context) {
    final brightness = on ?? Theme.of(context).brightness;
    final suffix = brightness == Brightness.dark ? '-dark' : '';
    final asset = mark ? 'assets/images/icon$suffix.png' : 'assets/images/logo$suffix.png';

    return Image.asset(asset, height: height);
  }
}
