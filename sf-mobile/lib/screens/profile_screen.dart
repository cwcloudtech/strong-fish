import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:package_info_plus/package_info_plus.dart';

import '../models/models.dart';
import '../providers/app_update_provider.dart';
import '../providers/providers.dart';
import '../widgets/common.dart';

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
                Text(
                  '@${user.handle} · ${t(switch (user.role) {
                    'coach' => 'profile.coach',
                    'superadmin' => 'profile.superadmin',
                    _ => 'profile.athlete',
                  })}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                if (user.bio.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(user.bio, textAlign: TextAlign.center),
                ],
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
              ListTile(
                leading: const Icon(Icons.translate),
                title: Text(t('common.language')),
                trailing: DropdownButton<String>(
                  value: locale,
                  underline: const SizedBox.shrink(),
                  items: const [
                    DropdownMenuItem(value: 'en', child: Text('English')),
                    DropdownMenuItem(value: 'fr', child: Text('Français')),
                  ],
                  onChanged: (value) async {
                    if (value == null) return;
                    await ref.read(localeProvider.notifier).set(value);
                    // Keep the account's language in step, so transactional
                    // emails arrive in the one they're reading the app in.
                    try {
                      await ref.read(apiProvider).updateProfile({
                        'name': user.name,
                        'surname': user.surname,
                        'username': user.username,
                        'anonymous': user.anonymous,
                        'bio': user.bio,
                        'locale': value,
                        'profileVisibility': user.profileVisibility,
                        'birthdate': user.birthdate,
                        'bodyweight': user.bodyweight,
                      });
                      await ref.read(sessionProvider.notifier).refresh();
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
                            await ref.read(apiProvider).updateProfile({
                              'name': user.name,
                              'surname': user.surname,
                              'username': user.username,
                              'anonymous': value,
                              'bio': user.bio,
                              'locale': user.locale,
                              'profileVisibility': user.profileVisibility,
                              'birthdate': user.birthdate,
                              'bodyweight': user.bodyweight,
                            });
                            await ref.read(sessionProvider.notifier).refresh();
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
                      await ref.read(apiProvider).updateProfile({
                        'name': user.name,
                        'surname': user.surname,
                        'username': user.username,
                        'anonymous': user.anonymous,
                        'bio': user.bio,
                        'locale': user.locale,
                        'profileVisibility': value,
                        'birthdate': user.birthdate,
                        'bodyweight': user.bodyweight,
                      });
                      await ref.read(sessionProvider.notifier).refresh();
                    } catch (error) {
                      _toast(ref.read(tErrorProvider)(error));
                    }
                  },
                ),
              ),
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
