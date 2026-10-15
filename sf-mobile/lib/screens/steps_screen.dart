import 'package:flutter/material.dart';

import '../widgets/device_links.dart';

/// Steps, and the account they come from.
///
/// A screen of its own rather than a panel on the profile. Connecting a watch
/// is a setting, but reading what it counted is something a member comes back
/// to - and a chart below four other cards is one nobody finds.
class StepsScreen extends StatelessWidget {
  const StepsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(12),
      children: const [DeviceLinksSection()],
    );
  }
}
