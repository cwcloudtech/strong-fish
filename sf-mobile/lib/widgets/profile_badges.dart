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

const _squat = _Ink(Color(0xFF7C3AED), Color(0xFFA78BFA));
const _bench = _Ink(Color(0xFF0E7490), Color(0xFF22D3EE));
const _deadlift = _Ink(Color(0xFFBE123C), Color(0xFFFB7185));
const _superadmin = _Ink(Color(0xFFB45309), Color(0xFFF0B429));

const _lifts = {'squat': _squat, 'bench': _bench, 'deadlift': _deadlift};

/// The pill both badges are drawn as: the ink on a wash of itself, which is
/// what keeps one colour per badge rather than a colour and a background.
class _Badge extends StatelessWidget {
  final String label;
  final Color ink;

  /// A background of its own, for the totaler - the one badge that is not a
  /// single colour.
  final Decoration? decoration;

  const _Badge({required this.label, required this.ink, this.decoration});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: decoration ??
          BoxDecoration(
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

/// The lift the member claims as their own. Nothing at all when they claim
/// none, which is the default and a perfectly good answer.
class SpecialtyBadge extends ConsumerWidget {
  final String specialty;

  const SpecialtyBadge({super.key, required this.specialty});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final brightness = Theme.of(context).brightness;

    if (specialty == 'total') {
      // The balanced totaler is the three lifts at once, and the badge says so
      // in the three colours rather than in a fourth hue that would mean
      // nothing on its own.
      return _Badge(
        label: t('profile.specialties.total'),
        ink: colors.text,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(999),
          gradient: LinearGradient(
            colors: [
              _squat.of(brightness).withValues(alpha: 0.24),
              _bench.of(brightness).withValues(alpha: 0.24),
              _deadlift.of(brightness).withValues(alpha: 0.24),
            ],
          ),
        ),
      );
    }

    final ink = _lifts[specialty];
    if (ink == null) return const SizedBox.shrink();
    return _Badge(label: t('profile.specialties.$specialty'), ink: ink.of(brightness));
  }
}

/// Both badges in the row they share, under a profile's name.
class ProfileBadges extends StatelessWidget {
  final String role;
  final String specialty;
  final MainAxisAlignment alignment;

  const ProfileBadges({
    super.key,
    required this.role,
    this.specialty = '',
    this.alignment = MainAxisAlignment.center,
  });

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 6,
      runSpacing: 6,
      alignment: alignment == MainAxisAlignment.start ? WrapAlignment.start : WrapAlignment.center,
      children: [
        RoleBadge(role: role),
        SpecialtyBadge(specialty: specialty),
      ],
    );
  }
}
