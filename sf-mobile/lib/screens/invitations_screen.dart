import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';

/// The clubs that asked this account to join them.
///
/// Invitations are matched by email address, not by user id, so one sent
/// before somebody registered is waiting here the first time they open the
/// app - which is the whole point of being able to invite a stranger.
class InvitationsScreen extends ConsumerStatefulWidget {
  const InvitationsScreen({super.key});

  @override
  ConsumerState<InvitationsScreen> createState() => _InvitationsScreenState();
}

class _InvitationsScreenState extends ConsumerState<InvitationsScreen> {
  String? _busy;

  Future<void> _act(Invitation invitation, {required bool accept}) async {
    setState(() => _busy = invitation.id);
    final t = ref.read(tProvider);
    try {
      final api = ref.read(apiProvider);
      if (accept) {
        await api.acceptInvitation(invitation.id);
      } else {
        await api.declineInvitation(invitation.id);
      }
      ref.invalidate(invitationsProvider);
      // Accepting changes which clubs this member is in, which is what the
      // training and calendar screens are scoped by.
      ref.invalidate(clubsProvider);
      ref.invalidate(eventsProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(accept
              ? t('invitations.accepted', {'club': invitation.clubName})
              : t('invitations.declined')),
        ));
      }
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final invitations = ref.watch(invitationsProvider);

    return RefreshIndicator(
      onRefresh: () async => ref.invalidate(invitationsProvider),
      child: invitations.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => SfErrorState(
          message: ref.read(tErrorProvider)(error),
          onRetry: () => ref.invalidate(invitationsProvider),
          retryLabel: t('common.back'),
        ),
        data: (list) {
          if (list.isEmpty) {
            return SfEmptyState(
              icon: Icons.mail_outline,
              title: t('invitations.emptyTitle'),
              message: t('invitations.emptyBody'),
            );
          }
          return ListView.builder(
            padding: const EdgeInsets.all(16),
            itemCount: list.length,
            itemBuilder: (context, index) {
              final invitation = list[index];
              final colors = AppColors.of(context);

              return Card(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(invitation.clubName, style: Theme.of(context).textTheme.titleMedium),
                      const SizedBox(height: 4),
                      Text(
                        t('invitations.from', {'name': invitation.inviterName}),
                        style: TextStyle(color: colors.textMuted),
                      ),
                      if (invitation.message.isNotEmpty) ...[
                        const SizedBox(height: 8),
                        Text(invitation.message),
                      ],
                      const SizedBox(height: 4),
                      Text(
                        t('invitations.asRole.${invitation.role}'),
                        style: TextStyle(color: colors.textMuted, fontSize: 12),
                      ),
                      const SizedBox(height: 12),
                      Row(
                        children: [
                          FilledButton(
                            onPressed: _busy == invitation.id
                                ? null
                                : () => _act(invitation, accept: true),
                            child: Text(t('invitations.accept')),
                          ),
                          const SizedBox(width: 8),
                          OutlinedButton(
                            onPressed: _busy == invitation.id
                                ? null
                                : () => _act(invitation, accept: false),
                            child: Text(t('invitations.decline')),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              );
            },
          );
        },
      ),
    );
  }
}
