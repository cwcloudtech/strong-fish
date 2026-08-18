import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';

/// The list of private conversations.
///
/// A thread is pushed rather than shown beside the list: on a phone there is
/// only ever room for one of the two, and stacking them is what every
/// messenger does.
class MessagesScreen extends ConsumerWidget {
  const MessagesScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final conversations = ref.watch(conversationsProvider);
    final colors = AppColors.of(context);

    return RefreshIndicator(
      onRefresh: () async => ref.invalidate(conversationsProvider),
      child: conversations.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => SfErrorState(
          message: ref.read(tErrorProvider)(error),
          onRetry: () => ref.invalidate(conversationsProvider),
          retryLabel: t('common.back'),
        ),
        data: (list) {
          if (list.isEmpty) {
            return SfEmptyState(
              icon: Icons.chat_bubble_outline,
              title: t('messages.emptyTitle'),
              message: t('messages.emptyBody'),
            );
          }
          return ListView.separated(
            itemCount: list.length,
            separatorBuilder: (context, index) => const Divider(height: 1),
            itemBuilder: (context, index) {
              final conversation = list[index];
              return ListTile(
                leading: SfAvatar.of(conversation.other),
                title: Text('${conversation.other.name} ${conversation.other.surname}'.trim()),
                subtitle: Text(
                  conversation.lastMessage,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: colors.textMuted),
                ),
                trailing: conversation.unread > 0
                    ? Badge(label: Text('${conversation.unread}'))
                    : null,
                onTap: () => Navigator.of(context).push(MaterialPageRoute(
                  builder: (_) => ThreadScreen(
                    userId: conversation.other.id,
                    title: '${conversation.other.name} ${conversation.other.surname}'.trim(),
                  ),
                )),
              );
            },
          );
        },
      ),
    );
  }
}

/// One open conversation.
class ThreadScreen extends ConsumerStatefulWidget {
  final String userId;
  final String title;

  const ThreadScreen({super.key, required this.userId, this.title = ''});

  @override
  ConsumerState<ThreadScreen> createState() => _ThreadScreenState();
}

class _ThreadScreenState extends ConsumerState<ThreadScreen> {
  final _draft = TextEditingController();
  final _scroll = ScrollController();
  final List<PrivateMessage> _sent = [];
  bool _busy = false;

  @override
  void dispose() {
    _draft.dispose();
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final content = _draft.text.trim();
    if (content.isEmpty) return;

    setState(() => _busy = true);
    try {
      final message = await ref.read(apiProvider).sendMessage(widget.userId, content);
      _draft.clear();
      // Appended locally rather than refetching: the thread is already on
      // screen, and a full reload would lose the scroll position mid-sentence.
      setState(() => _sent.add(message));
      ref.invalidate(conversationsProvider);
      _scrollToBottom();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _block() async {
    final t = ref.read(tProvider);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('blocks.blockTitle')),
        content: Text(t('blocks.blockBody', {'name': widget.title})),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: Text(t('common.cancel'))),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: Text(t('blocks.block'))),
        ],
      ),
    );
    if (confirmed != true) return;

    try {
      await ref.read(apiProvider).blockMember(widget.userId);
      ref.invalidate(conversationsProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(t('blocks.blocked'))));
        Navigator.of(context).pop();
      }
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    }
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scroll.hasClients) _scroll.jumpTo(_scroll.position.maxScrollExtent);
    });
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final thread = ref.watch(threadProvider(widget.userId));

    return Scaffold(
      appBar: AppBar(
        title: Text(widget.title.isEmpty ? t('messages.title') : widget.title),
        actions: [
          IconButton(
            icon: const Icon(Icons.block),
            tooltip: t('blocks.block'),
            onPressed: _block,
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: thread.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => SfErrorState(
                message: ref.read(tErrorProvider)(error),
                onRetry: () => ref.invalidate(threadProvider(widget.userId)),
                retryLabel: t('common.back'),
              ),
              data: (data) {
                final messages = [...data.messages, ..._sent];
                if (messages.isEmpty) {
                  return Center(
                    child: Padding(
                      padding: const EdgeInsets.all(32),
                      child: Text(t('messages.threadEmpty'), textAlign: TextAlign.center),
                    ),
                  );
                }
                _scrollToBottom();
                return ListView.builder(
                  controller: _scroll,
                  padding: const EdgeInsets.all(16),
                  itemCount: messages.length,
                  itemBuilder: (context, index) => _Bubble(message: messages[index]),
                );
              },
            ),
          ),
          SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Expanded(
                    child: TextField(
                      controller: _draft,
                      minLines: 1,
                      maxLines: 4,
                      decoration: InputDecoration(hintText: t('messages.placeholder')),
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton.filled(
                    icon: const Icon(Icons.send),
                    onPressed: _busy ? null : _send,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Bubble extends StatelessWidget {
  final PrivateMessage message;

  const _Bubble({required this.message});

  @override
  Widget build(BuildContext context) {
    final colors = AppColors.of(context);

    return Align(
      alignment: message.mine ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        constraints: BoxConstraints(maxWidth: MediaQuery.of(context).size.width * 0.78),
        decoration: BoxDecoration(
          color: message.mine ? colors.primaryTint : colors.backgroundMuted,
          borderRadius: BorderRadius.circular(AppRadius.value),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(message.content),
            const SizedBox(height: 2),
            Text(
              _time(message.createdAt),
              style: TextStyle(color: colors.textMuted, fontSize: 11),
            ),
          ],
        ),
      ),
    );
  }

  String _time(DateTime at) =>
      '${at.day.toString().padLeft(2, '0')}/${at.month.toString().padLeft(2, '0')} '
      '${at.hour.toString().padLeft(2, '0')}:${at.minute.toString().padLeft(2, '0')}';
}
