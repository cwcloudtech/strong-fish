import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:package_info_plus/package_info_plus.dart';

import '../models/models.dart';
import '../providers/app_update_provider.dart';
import '../i18n/app_localizations.dart';
import '../providers/providers.dart';
import '../widgets/ai_connections.dart';
import '../widgets/common.dart';
import '../widgets/profile_badges.dart';
import '../widgets/social_share.dart';
import '../widgets/socials.dart';
import '../widgets/storage_connections.dart';
import 'public_profile_screen.dart';

/// The connected user's own profile and settings: their bests, their public
/// profile, and the app's language and theme.
class ProfileScreen extends ConsumerStatefulWidget {
  const ProfileScreen({super.key});

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen> {
  bool _busy = false;
  String _appVersion = '';

  @override
  void initState() {
    super.initState();
    // Both are best-effort and neither blocks the screen: the version is a
    // label, and the update check is silent when it fails.
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      await ref.read(appUpdateProvider.notifier).checkForUpdate();
      final info = await PackageInfo.fromPlatform();
      if (mounted) setState(() => _appVersion = info.version);
    });
  }

  /// Re-reads everything this screen shows.
  ///
  /// The update check is part of it deliberately: a build published while the
  /// app was open should be reachable by pulling down, not only by restarting.
  Future<void> _refresh() async {
    await ref.read(sessionProvider.notifier).refresh();
    ref.invalidate(oneRmsProvider);
    await ref.read(appUpdateProvider.notifier).checkForUpdate();
    final info = await PackageInfo.fromPlatform();
    if (mounted) setState(() => _appVersion = info.version);
  }

  Future<void> _install() async {
    final t = ref.read(tProvider);
    try {
      await ref.read(appUpdateProvider.notifier).downloadAndInstall();
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(t('update.failed'))));
      }
    }
  }

  Future<void> _changeAvatar() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery, maxWidth: 800, imageQuality: 80);
    if (picked == null) return;

    setState(() => _busy = true);
    try {
      final bytes = await picked.readAsBytes();
      await ref.read(apiProvider).updatePicture('data:image/jpeg;base64,${base64Encode(bytes)}');
      await ref.read(sessionProvider.notifier).refresh();
    } catch (error) {
      _toast(ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Saves the profile with [changes] applied over what the account already
  /// has.
  ///
  /// Every field goes with every save: the API writes the whole profile, so a
  /// payload that leaves one out clears it. Three separate copies of this map
  /// used to be spelled out in this screen, and the specialty added later was
  /// exactly the kind of field the fourth copy would have forgotten - changing
  /// the app's language would have quietly wiped somebody's badge.
  Future<void> _patchProfile(User user, Map<String, dynamic> changes) async {
    await ref.read(apiProvider).updateProfile({
      'name': user.name,
      'surname': user.surname,
      'username': user.username,
      'anonymous': user.anonymous,
      'bio': user.bio,
      'locale': user.locale,
      'profileVisibility': user.profileVisibility,
      'fitnessPublic': user.fitnessPublic,
      'birthdate': user.birthdate,
      'bodyweight': user.bodyweight,
      'height': user.height,
      'gender': user.gender,
      'effortMetric': user.effortMetric,
      'socials': user.socials,
      ...changes,
    });
    await ref.read(sessionProvider.notifier).refresh();
  }


  void _toast(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final locale = ref.watch(localeProvider);
    final themeMode = ref.watch(themeModeProvider);
    final user = ref.watch(sessionProvider).user;
    final maxes = ref.watch(oneRmsProvider).valueOrNull ?? const <OneRm>[];

    if (user == null) return const Center(child: CircularProgressIndicator());

    final bests = maxes.where((max) => const {'squat', 'bench', 'deadlift'}.contains(max.slug)).toList();
    final total = bests.fold<double>(0, (sum, best) => sum + best.value);

    // Pull to refresh, like every other screen: it re-reads the profile and
    // re-checks for a newer build, which is how the upgrade entry appears
    // without having to leave and come back.
    return RefreshIndicator(
      onRefresh: _refresh,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.all(16),
      children: [
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              children: [
                Stack(
                  alignment: Alignment.bottomRight,
                  children: [
                    SfAvatar.of(user, radius: 44),
                    if (!_busy)
                      IconButton.filled(
                        icon: const Icon(Icons.camera_alt, size: 16),
                        visualDensity: VisualDensity.compact,
                        onPressed: _changeAvatar,
                        tooltip: t('profile.changeAvatar'),
                      ),
                  ],
                ),
                const SizedBox(height: 10),
                Text(user.fullName, style: Theme.of(context).textTheme.titleLarge),
                Text('@${user.handle}', style: Theme.of(context).textTheme.bodySmall),
                const SizedBox(height: 6),
                ProfileBadges(role: user.role),
                if (user.socials.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  SocialLinks(socials: user.socials),
                ],
                if (user.bio.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(user.bio, textAlign: TextAlign.center),
                ],
              ],
            ),
          ),
        ),

        // Your own profile as everybody else reaches it. It is on this screen
        // because there was nowhere else to share it from: the share row lives
        // on somebody *else's* profile, which is no help when the profile you
        // want to hand out is your own. A private profile is left out - the
        // link would land on a 404 for whoever received it.
        if (user.handle.isNotEmpty && user.profileVisibility != 'private')
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(t('profile.myProfile'), style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 4),
                  Text(t('profile.myProfileHelp'), style: Theme.of(context).textTheme.bodySmall),
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      OutlinedButton.icon(
                        onPressed: () => Navigator.of(context).push(MaterialPageRoute(
                          builder: (_) => PublicProfileScreen(handle: user.handle),
                        )),
                        icon: const Icon(Icons.open_in_new, size: 18),
                        label: Text(t('profile.viewProfile')),
                      ),
                      const SizedBox(width: 8),
                      SocialShareRow(
                        url: '${ref.read(apiProvider).client.frontendUrl}/profile/${user.handle}',
                        text: t('share.profileText', {'name': user.fullName}),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),

        // The models this account can ask, and who else may spend them.
        const AiConnectionsSection(),

        // Where its videos and voice messages go. The same card, because it
        // is the same arrangement: a credential of the member's own, lent to
        // whoever they choose, in an order that decides which one leads.
        const StorageConnectionsSection(),

        // Where else to find this lifter. The fields are generated from the
        // same table as the links, so a network added there appears in both.
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(t('profile.socials'), style: Theme.of(context).textTheme.titleMedium),
                const SizedBox(height: 4),
                Text(t('profile.socialsHelp'), style: Theme.of(context).textTheme.bodySmall),
                const SizedBox(height: 8),
                _SocialsForm(user: user, onSave: (socials) => _patchProfile(user, {'socials': socials})),
              ],
            ),
          ),
        ),

        // The numbers that are about the lifter rather than about a lift.
        // The phone had no way to set any of them: bodyweight and birthdate
        // were sent back untouched on every save, and the AI coach is handed
        // all three with the rest of the member's file.
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(t('profile.measures'), style: Theme.of(context).textTheme.titleMedium),
                const SizedBox(height: 8),
                _MeasuresForm(
                  user: user,
                  onSave: (changes) => _patchProfile(user, changes),
                ),
              ],
            ),
          ),
        ),

        if (bests.isNotEmpty)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(t('profile.bests'), style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 12),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceAround,
                    children: [
                      for (final best in bests)
                        _Stat(value: '${_fmt(best.value)}${t('common.kg')}', label: best.label(locale)),
                      _Stat(value: '${_fmt(total)}${t('common.kg')}', label: t('profile.total')),
                    ],
                  ),
                ],
              ),
            ),
          ),

        Card(
          child: Column(
            children: [
              // Powerlifting's coefficients are fitted separately for men and
              // women - there is no unisex variant of DOTS, Wilks or IPF GL -
              // so a strength score cannot be computed without this. Male by
              // default, which is what an account never asked reads as.
              ListTile(
                leading: const Icon(Icons.wc_outlined),
                title: Text(t('profile.gender')),
                trailing: SegmentedButton<String>(
                  segments: [
                    ButtonSegment(value: 'male', label: Text(t('profile.genderMale'))),
                    ButtonSegment(value: 'female', label: Text(t('profile.genderFemale'))),
                  ],
                  selected: {user.gender.isEmpty ? 'male' : user.gender},
                  showSelectedIcon: false,
                  onSelectionChanged: (selection) async {
                    try {
                      await _patchProfile(user, {'gender': selection.first});
                    } catch (error) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(ref.read(tErrorProvider)(error))),
                        );
                      }
                    }
                  },
                ),
              ),
              // How effort reads across the app: RPE, or the reps-in-reserve it
              // implies. A display choice only - effort is stored and computed
              // as RPE and shown as 10 - RPE when RIR is picked.
              ListTile(
                leading: const Icon(Icons.speed_outlined),
                title: Text(t('profile.effortMetric')),
                subtitle: Text(t('profile.effortMetricHelp')),
                trailing: SegmentedButton<String>(
                  segments: const [
                    ButtonSegment(value: 'rpe', label: Text('RPE')),
                    ButtonSegment(value: 'rir', label: Text('RIR')),
                  ],
                  selected: {user.effortMetric.isEmpty ? 'rpe' : user.effortMetric},
                  showSelectedIcon: false,
                  onSelectionChanged: (selection) async {
                    try {
                      await _patchProfile(user, {'effortMetric': selection.first});
                    } catch (error) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(ref.read(tErrorProvider)(error))),
                        );
                      }
                    }
                  },
                ),
              ),
              ListTile(
                leading: const Icon(Icons.translate),
                title: Text(t('common.language')),
                trailing: DropdownButton<String>(
                  value: locale,
                  underline: const SizedBox.shrink(),
                  // Built from the dictionary list rather than written out:
                  // a locale added there should appear here without anybody
                  // having to remember this screen.
                  items: [
                    for (final option in supportedLocales)
                      DropdownMenuItem(value: option.code, child: Text(option.label)),
                  ],
                  onChanged: (value) async {
                    if (value == null) return;
                    await ref.read(localeProvider.notifier).set(value);
                    // Keep the account's language in step, so transactional
                    // emails arrive in the one they're reading the app in.
                    try {
                      await _patchProfile(user, {'locale': value});
                    } catch (_) {
                      // The app's own language already changed; failing to
                      // mirror it server-side isn't worth an error dialog.
                    }
                  },
                ),
              ),
              ListTile(
                leading: const Icon(Icons.brightness_6_outlined),
                title: Text(t('common.theme')),
                trailing: DropdownButton<ThemeMode>(
                  value: themeMode,
                  underline: const SizedBox.shrink(),
                  items: [
                    DropdownMenuItem(value: ThemeMode.system, child: Text(t('common.all'))),
                    DropdownMenuItem(value: ThemeMode.light, child: Text(t('common.light'))),
                    DropdownMenuItem(value: ThemeMode.dark, child: Text(t('common.dark'))),
                  ],
                  onChanged: (value) =>
                      value == null ? null : ref.read(themeModeProvider.notifier).set(value),
                ),
              ),
              // Anonymity is a switch here rather than a form: the username
              // itself is edited on the web, where a text field belongs, and
              // what an athlete wants on a phone is to turn it on or off.
              ListTile(
                leading: const Icon(Icons.masks_outlined),
                title: Text(t('profile.anonymous')),
                subtitle: Text(
                  user.username.isEmpty
                      ? t('profile.anonymousNeedsUsername')
                      : t('profile.anonymousHelp'),
                ),
                trailing: Switch(
                  value: user.anonymous,
                  // Nothing to hide behind without a username, and the API
                  // refuses it - so the switch is simply unavailable.
                  onChanged: user.username.isEmpty
                      ? null
                      : (value) async {
                          try {
                            await _patchProfile(user, {'anonymous': value});
                          } catch (error) {
                            _toast(ref.read(tErrorProvider)(error));
                          }
                        },
                ),
              ),
              // Three levels rather than a public/private switch: "my clubs"
              // is the one most athletes actually want, and a toggle cannot
              // express it.
              ListTile(
                leading: const Icon(Icons.visibility_outlined),
                title: Text(t('profile.visibility')),
                subtitle: Text(t('profile.visibilityHelp.${user.profileVisibility}')),
                trailing: DropdownButton<String>(
                  value: user.profileVisibility,
                  underline: const SizedBox.shrink(),
                  items: [
                    DropdownMenuItem(value: 'public', child: Text(t('profile.visibilityPublic'))),
                    DropdownMenuItem(value: 'clubs', child: Text(t('profile.visibilityClubs'))),
                    DropdownMenuItem(value: 'private', child: Text(t('profile.visibilityPrivate'))),
                  ],
                  onChanged: (value) async {
                    if (value == null) return;
                    try {
                      await _patchProfile(user, {'profileVisibility': value});
                    } catch (error) {
                      _toast(ref.read(tErrorProvider)(error));
                    }
                  },
                ),
              ),
              // Fitness data on the public profile, off by default. A coach or
              // nutritionist of a shared club sees it regardless; this opens it
              // to everyone, even logged out.
              SwitchListTile(
                secondary: const Icon(Icons.monitor_heart_outlined),
                title: Text(t('profile.fitnessPublicToggle')),
                subtitle: Text(t('profile.fitnessPublicHelp')),
                value: user.fitnessPublic,
                onChanged: (value) async {
                  try {
                    await _patchProfile(user, {'fitnessPublic': value});
                  } catch (error) {
                    _toast(ref.read(tErrorProvider)(error));
                  }
                },
              ),
              // What the member calls themselves as a lifter. Picked, never
              // computed: a badge worked out from the three bests would leave
              // somebody who has entered no maxes with nothing to wear, and
              // would relabel somebody the week their squat dips.
              //
              // A sheet rather than a dropdown, unlike the tiles above it: the
              // labels are long ("Spécialiste du développé couché"), and the
              // options are worth showing as the coloured badges they will
              // become rather than as four lines of text.
              // MFA enrollment needs a QR scan or a WebAuthn ceremony, neither
              // of which belongs in this screen - the web app owns it, and the
              // app just reports the state.
              ListTile(
                leading: const Icon(Icons.shield_outlined),
                title: Text(t('mfa.title')),
                subtitle: Text(user.mfaEnabled ? t('mfa.totpEnabled') : t('mfa.totpDisabled')),
              ),
              // The upgrade entry only exists when there is something to
              // upgrade to; the rest of the time this row is just the version.
              _UpdateTile(
                appVersion: _appVersion,
                onInstall: _install,
              ),
              ListTile(
                leading: const Icon(Icons.logout),
                title: Text(t('common.logout')),
                onTap: () => ref.read(sessionProvider.notifier).logout(),
              ),
            ],
          ),
        ),
      ],
      ),
    );
  }
}

/// The social accounts, one field per entry in the table.
///
/// Saved on a button rather than on every keystroke: these are typed, not
/// picked, and a request per character would be both wasteful and impossible
/// to correct halfway through.
/// Bodyweight, height and birthdate.
///
/// Saved together behind one button rather than each on its own: they are
/// typed in one go, and three separate saves of the whole profile is three
/// chances for one of them to be written over.
class _MeasuresForm extends ConsumerStatefulWidget {
  final User user;
  final Future<void> Function(Map<String, dynamic>) onSave;

  const _MeasuresForm({required this.user, required this.onSave});

  @override
  ConsumerState<_MeasuresForm> createState() => _MeasuresFormState();
}

class _MeasuresFormState extends ConsumerState<_MeasuresForm> {
  late final TextEditingController _bodyweight;
  late final TextEditingController _height;
  late String _birthdate;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _bodyweight = TextEditingController(
        text: widget.user.bodyweight > 0 ? _fmt(widget.user.bodyweight) : '');
    _height = TextEditingController(
        text: widget.user.height > 0 ? _fmt(widget.user.height) : '');
    _birthdate = widget.user.birthdate;
  }

  @override
  void dispose() {
    _bodyweight.dispose();
    _height.dispose();
    super.dispose();
  }

  double _number(TextEditingController field) =>
      double.tryParse(field.text.trim().replaceAll(',', '.')) ?? 0;

  Future<void> _pickBirthdate() async {
    final now = DateTime.now();
    final current = DateTime.tryParse(_birthdate) ?? DateTime(now.year - 30, now.month, now.day);
    final picked = await showDatePicker(
      context: context,
      initialDate: current,
      // Nobody lifting is 120, and a date in the future is a typo.
      firstDate: DateTime(now.year - 120),
      lastDate: now,
    );
    if (picked != null) {
      setState(() => _birthdate =
          '${picked.year.toString().padLeft(4, '0')}-'
          '${picked.month.toString().padLeft(2, '0')}-'
          '${picked.day.toString().padLeft(2, '0')}');
    }
  }

  Future<void> _save() async {
    final t = ref.read(tProvider);
    setState(() => _busy = true);
    try {
      await widget.onSave({
        'bodyweight': _number(_bodyweight),
        'height': _number(_height),
        'birthdate': _birthdate,
      });
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(t('profile.saved'))));
      }
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

    return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      Row(children: [
        Expanded(
          child: TextField(
            controller: _bodyweight,
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            decoration: InputDecoration(
              labelText: '${t('profile.bodyweight')} (${t('common.kg')})',
              border: const OutlineInputBorder(),
              isDense: true,
            ),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: TextField(
            controller: _height,
            keyboardType: TextInputType.number,
            decoration: InputDecoration(
              labelText: '${t('profile.height')} (${t('common.cm')})',
              border: const OutlineInputBorder(),
              isDense: true,
            ),
          ),
        ),
      ]),
      const SizedBox(height: 8),
      ListTile(
        contentPadding: EdgeInsets.zero,
        leading: const Icon(Icons.cake_outlined),
        title: Text(t('profile.birthdate')),
        subtitle: Text(_birthdate.isEmpty ? t('common.optional') : _birthdate),
        trailing: _birthdate.isEmpty
            ? null
            : IconButton(
                onPressed: () => setState(() => _birthdate = ''),
                tooltip: t('common.clearSelection'),
                icon: const Icon(Icons.close, size: 20),
              ),
        onTap: _pickBirthdate,
      ),
      const SizedBox(height: 8),
      FilledButton(
        onPressed: _busy ? null : _save,
        child: Text(t('common.save')),
      ),
    ]);
  }
}

class _SocialsForm extends ConsumerStatefulWidget {
  final User user;
  final Future<void> Function(Map<String, String>) onSave;

  const _SocialsForm({required this.user, required this.onSave});

  @override
  ConsumerState<_SocialsForm> createState() => _SocialsFormState();
}

class _SocialsFormState extends ConsumerState<_SocialsForm> {
  late final Map<String, TextEditingController> _fields;
  bool _busy = false;
  bool _dirty = false;

  @override
  void initState() {
    super.initState();
    _fields = {
      for (final network in socialProfiles) ...{
        network.key: TextEditingController(text: widget.user.socials[network.key] ?? ''),
        if (network.rankKey != null)
          network.rankKey!: TextEditingController(text: widget.user.socials[network.rankKey!] ?? ''),
      },
    };
    for (final field in _fields.values) {
      field.addListener(() {
        if (!_dirty && mounted) setState(() => _dirty = true);
      });
    }
  }

  @override
  void dispose() {
    for (final field in _fields.values) {
      field.dispose();
    }
    super.dispose();
  }

  Future<void> _save() async {
    final t = ref.read(tProvider);
    setState(() => _busy = true);
    try {
      await widget.onSave({
        for (final entry in _fields.entries) entry.key: entry.value.text.trim(),
      });
      if (mounted) {
        setState(() => _dirty = false);
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(t('profile.saved'))));
      }
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

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final network in socialProfiles) ...[
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Expanded(
                flex: network.rankKey == null ? 1 : 2,
                child: TextField(
                  controller: _fields[network.key],
                  decoration: InputDecoration(
                    // The icon is the label: five written labels one after
                    // another say what five marks already do.
                    icon: Icon(network.icon),
                    labelText: network.label,
                    hintText: network.placeholder,
                  ),
                ),
              ),
              if (network.rankKey != null) ...[
                const SizedBox(width: 10),
                Expanded(
                  child: TextField(
                    controller: _fields[network.rankKey!],
                    decoration: InputDecoration(labelText: t('profile.rank')),
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: 8),
        ],
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton(
            onPressed: _busy || !_dirty ? null : _save,
            child: Text(t('common.save')),
          ),
        ),
      ],
    );
  }
}

/// The installed version, and the button that replaces it when a newer build
/// is published.
class _UpdateTile extends ConsumerWidget {
  final String appVersion;
  final Future<void> Function() onInstall;

  const _UpdateTile({required this.appVersion, required this.onInstall});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final update = ref.watch(appUpdateProvider);

    // iOS gets no upgrade control at all: an app there cannot install its own
    // code, so the version is all there is to say. Offering a button that
    // downloads and opens a package is what App Review rejects.
    if (!AppUpdateNotifier.selfUpdateAllowed) {
      return ListTile(
        leading: const Icon(Icons.info_outline),
        title: Text(t('update.storeManaged')),
        subtitle: appVersion.isEmpty ? null : Text('v$appVersion'),
      );
    }

    if (update.downloading) {
      return ListTile(
        leading: const Icon(Icons.system_update),
        title: Text(t('update.downloading')),
        subtitle: LinearProgressIndicator(value: update.progress),
      );
    }

    if (update.availableVersion == null) {
      return ListTile(
        leading: const Icon(Icons.info_outline),
        title: Text(t('update.upToDate')),
        subtitle: appVersion.isEmpty ? null : Text('v$appVersion'),
      );
    }

    return ListTile(
      leading: const Icon(Icons.system_update),
      title: Text(t('update.available', {'version': update.availableVersion!})),
      subtitle: Text(t('update.help')),
      trailing: FilledButton(onPressed: onInstall, child: Text(t('update.install'))),
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
                .titleLarge
                ?.copyWith(color: Theme.of(context).colorScheme.primary, fontWeight: FontWeight.bold)),
        Text(label, style: Theme.of(context).textTheme.bodySmall),
      ],
    );
  }
}

String _fmt(double value) => value % 1 == 0 ? value.toStringAsFixed(0) : value.toStringAsFixed(1);
