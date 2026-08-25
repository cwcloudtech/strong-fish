import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import 'public_profile_screen.dart';

/// Account management, for a superadmin.
///
/// It offers the three things worth doing away from a desk: activating an
/// account that is waiting, confirming somebody who signed up as a coach, and
/// stopping an abuser. Everything else - editing a profile, resolving reports,
/// the exercise catalog - stays on the web, where the screen is big enough for
/// it.
class AdminUsersScreen extends ConsumerStatefulWidget {
  const AdminUsersScreen({super.key});

  @override
  ConsumerState<AdminUsersScreen> createState() => _AdminUsersScreenState();
}

class _AdminUsersScreenState extends ConsumerState<AdminUsersScreen> {
  final _search = TextEditingController();

  List<User> _users = [];
  List<CoachApplicant> _coachRequests = [];
  Map<String, dynamic> _stats = const {};
  bool _loading = true;
  String? _error;
  String? _busy;
  String _query = '';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final api = ref.read(apiProvider);
    try {
      // All three at once: they are shown on one screen, and three round trips
      // in sequence would make it feel slower than it is.
      final results = await Future.wait([
        api.adminUsers(),
        api.adminCoachRequests(),
        api.adminStats(),
      ]);
      if (!mounted) return;
      setState(() {
        _users = results[0] as List<User>;
        _coachRequests = results[1] as List<CoachApplicant>;
        _stats = results[2] as Map<String, dynamic>;
      });
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _run(String id, Future<void> Function() action) async {
    setState(() => _busy = id);
    try {
      await action();
      await _load();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = null);
    }
  }

  /// Rejecting a coach needs a reason: it is emailed to the applicant, and the
  /// API refuses the decision without one.
  Future<void> _rejectCoach(CoachApplicant applicant) async {
    final t = ref.read(tProvider);
    final controller = TextEditingController();

    final motive = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('admin.rejectCoachTitle')),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(t('admin.rejectCoachHelp', {'name': applicant.fullName})),
            const SizedBox(height: 12),
            TextField(
              controller: controller,
              maxLines: 3,
              autofocus: true,
              decoration: InputDecoration(hintText: t('admin.rejectCoachPlaceholder')),
            ),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text(t('common.cancel'))),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text.trim()),
            child: Text(t('admin.rejectCoach')),
          ),
        ],
      ),
    );
    controller.dispose();

    if (motive == null || motive.isEmpty) return;
    await _run(applicant.id,
        () => ref.read(apiProvider).adminDecideCoachRequest(applicant.id, 'rejected', motive));
  }

  /// Deleting an account takes their posts, programs and messages with it, so
  /// it asks first - and it is the one action here that cannot be undone by
  /// setting a role back.
  Future<void> _confirmDelete(User user) async {
    final t = ref.read(tProvider);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('common.delete')),
        content: Text(t('admin.confirmDeleteUser', {'email': user.email})),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: Text(t('common.cancel'))),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: AppColors.of(context).danger),
            onPressed: () => Navigator.pop(context, true),
            child: Text(t('common.delete')),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await _run(user.id, () => ref.read(apiProvider).adminDeleteUser(user.id));
  }

  Future<void> _showIps(User user) async {
    final t = ref.read(tProvider);
    List<ConnectionIp> ips;
    try {
      ips = await ref.read(apiProvider).adminUserIps(user.id);
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
      return;
    }
    if (!mounted) return;

    await showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          children: [
            Text(t('admin.ipsTitle', {'name': user.fullName}),
                style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            if (ips.isEmpty)
              Text(t('admin.noIpsBody'))
            else
              for (final entry in ips)
                ListTile(
                  dense: true,
                  title: Text(entry.ip),
                  subtitle: Text('${t('admin.ipCount')}: ${entry.count}'),
                  trailing: Text(_date(entry.lastSeen)),
                ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);

    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      return SfErrorState(message: _error!, onRetry: _load, retryLabel: t('common.back'));
    }

    final query = _query.toLowerCase();
    final visible = query.isEmpty
        ? _users
        : _users
            .where((user) =>
                user.fullName.toLowerCase().contains(query) ||
                user.email.toLowerCase().contains(query) ||
                user.handle.toLowerCase().contains(query))
            .toList();

    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(16),
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceEvenly,
            children: [
              _Stat(value: '${_stats['users'] ?? 0}', label: t('admin.totalUsers')),
              _Stat(value: '${_stats['openReports'] ?? 0}', label: t('admin.openReports')),
              _Stat(value: '${_stats['coachRequests'] ?? 0}', label: t('admin.coachRequests')),
            ],
          ),
          const SizedBox(height: 16),

          // Coaches waiting on a decision come first: somebody is blocked on
          // this, and the account list is not.
          if (_coachRequests.isNotEmpty) ...[
            Text(t('admin.coachRequests'), style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            for (final applicant in _coachRequests)
              Card(
                child: ListTile(
                  leading: _ProfileAvatar(
                    avatar: SfAvatar(picture: applicant.picture, name: applicant.fullName),
                    target: applicant.handle.isNotEmpty ? applicant.handle : applicant.id,
                  ),
                  title: Text(applicant.fullName),
                  subtitle: Text(applicant.email),
                  trailing: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      IconButton(
                        icon: const Icon(Icons.check),
                        tooltip: t('admin.confirmCoach'),
                        onPressed: _busy == applicant.id
                            ? null
                            : () => _run(
                                applicant.id,
                                () => ref
                                    .read(apiProvider)
                                    .adminDecideCoachRequest(applicant.id, 'approved', '')),
                      ),
                      IconButton(
                        icon: const Icon(Icons.close),
                        tooltip: t('admin.rejectCoach'),
                        onPressed: _busy == applicant.id ? null : () => _rejectCoach(applicant),
                      ),
                    ],
                  ),
                ),
              ),
            const SizedBox(height: 16),
          ],

          TextField(
            controller: _search,
            decoration: InputDecoration(
              hintText: t('admin.searchUsers'),
              prefixIcon: const Icon(Icons.search),
            ),
            onChanged: (value) => setState(() => _query = value),
          ),
          const SizedBox(height: 8),

          if (visible.isEmpty)
            SfEmptyState(icon: Icons.person_off_outlined, title: t('search.noneTitle'))
          else
            for (final user in visible)
              Card(
                child: ListTile(
                  leading: _ProfileAvatar(
                    avatar: SfAvatar.of(user),
                    target: user.handle.isNotEmpty ? user.handle : user.id,
                  ),
                  title: Text(user.fullName.isEmpty ? user.email : user.fullName),
                  subtitle: Text(
                    '${user.email}\n${t('admin.role${_capitalize(user.role)}')}',
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                  ),
                  isThreeLine: true,
                  trailing: _busy == user.id
                      ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
                      : PopupMenuButton<String>(
                          onSelected: (action) {
                            switch (action) {
                              case 'ips':
                                _showIps(user);
                              case 'delete':
                                _confirmDelete(user);
                              default:
                                _run(user.id,
                                    () => ref.read(apiProvider).adminSetRole(user.id, action));
                            }
                          },
                          itemBuilder: (context) => [
                            for (final role in const ['confirmed', 'coach', 'disabled', 'ban'])
                              if (role != user.role)
                                PopupMenuItem(
                                  value: role,
                                  child: Text(t('admin.makeRole', {
                                    'role': t('admin.role${_capitalize(role)}'),
                                  })),
                                ),
                            PopupMenuItem(value: 'ips', child: Text(t('admin.ips'))),
                            // Superadmins are not offered to themselves: an
                            // account cannot delete itself, and the API says so
                            // too rather than trusting this list.
                            if (user.id != ref.read(sessionProvider).user?.id)
                              PopupMenuItem(
                                value: 'delete',
                                child: Text(
                                  t('common.delete'),
                                  style: TextStyle(color: AppColors.of(context).danger),
                                ),
                              ),
                          ],
                        ),
                ),
              ),
        ],
      ),
    );
  }

  String _capitalize(String value) =>
      value.isEmpty ? value : value[0].toUpperCase() + value.substring(1);

  String _date(DateTime at) =>
      '${at.day.toString().padLeft(2, '0')}/${at.month.toString().padLeft(2, '0')}/${at.year}';
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
        Text(label, style: Theme.of(context).textTheme.bodySmall, textAlign: TextAlign.center),
      ],
    );
  }
}

/// An avatar in the admin lists that opens the person behind it.
///
/// Only the picture is tappable, not the whole row: the row's own controls
/// change somebody's role or delete their account, and those should stay the
/// deliberate act they are.
///
/// Addressed by handle when there is one and by id otherwise - the API's
/// profile route takes either - so a member who never picked a profile name
/// opens just the same. Visibility is the API's decision as everywhere else,
/// and it lets a superadmin read every profile, so from this screen the page
/// always opens.
class _ProfileAvatar extends ConsumerWidget {
  final Widget avatar;
  final String target;

  const _ProfileAvatar({required this.avatar, required this.target});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Tooltip(
      message: ref.watch(tProvider)('search.openProfile'),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: () => Navigator.of(context).push(
          MaterialPageRoute(builder: (_) => PublicProfileScreen(handle: target)),
        ),
        child: avatar,
      ),
    );
  }
}
