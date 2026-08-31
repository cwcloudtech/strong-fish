import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/providers.dart';
import '../theme.dart';

/// What somebody is on StrongFish, and what they call themselves as a lifter.
///
/// The same two badges sf-ui draws (see its ProfileBadges.jsx), in the same
/// colours: the role is the account's standing here and is granted, while the
/// specialty is a claim its owner makes about themselves. Two facts, so two
/// colours - "Coach" in grey text beside the handle was read by nobody.
///
/// The palette lives here rather than in [AppColors] because these are the only
/// widgets that use it: adding four inks to the theme extension would mean
/// touching its constructor, both themes, copyWith and lerp for colours nothing
/// else ever asks for.

/// The lifts a member may claim, in the order the picker offers them - the
/// three in competition order, then the totaler as the "no single lift is
/// mine" answer. Mirrors the API's models.Specialties.
const specialties = ['squat', 'bench', 'deadlift', 'total'];

/// One ink per badge, light and dark. Mid-tone colours would have done for
/// most of these, but a badge is small uppercase text: the dark variants are
/// lightened until they carry on a near-black card.
class _Ink {
  final Color light;
  final Color dark;
  const _Ink(this.light, this.dark);

  Color of(Brightness brightness) => brightness == Brightness.dark ? dark : light;
}

const _superadmin = _Ink(Color(0xFFB45309), Color(0xFFF0B429));

/// The pill a badge is drawn as: the ink on a wash of itself, which is what
/// keeps one colour per badge rather than a colour and a background.
class _Badge extends StatelessWidget {
  final String label;
  final Color ink;

  const _Badge({required this.label, required this.ink});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: ink.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        label.toUpperCase(),
        style: TextStyle(
          color: ink,
          fontSize: 11,
          fontWeight: FontWeight.w600,
          letterSpacing: 0.2,
        ),
      ),
    );
  }
}

/// The account's standing: athlete, coach or superadmin.
class RoleBadge extends ConsumerWidget {
  final String role;

  const RoleBadge({super.key, required this.role});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final brightness = Theme.of(context).brightness;

    // An account still waiting on its coach confirmation reads as an athlete:
    // that is what it can do here.
    final (key, ink) = switch (role) {
      'superadmin' => ('profile.superadmin', _superadmin.of(brightness)),
      'coach' => ('profile.coach', colors.success),
      _ => ('profile.athlete', colors.primaryDark),
    };
    return _Badge(label: t(key), ink: ink);
  }
}

/// The badge under a profile's name.
class ProfileBadges extends StatelessWidget {
  final String role;
  final MainAxisAlignment alignment;

  const ProfileBadges({
    super.key,
    required this.role,
    this.alignment = MainAxisAlignment.center,
  });

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 6,
      runSpacing: 6,
      alignment: alignment == MainAxisAlignment.start ? WrapAlignment.start : WrapAlignment.center,
      children: [RoleBadge(role: role)],
    );
  }
}
