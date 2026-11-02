import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:share_plus/share_plus.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/ai_ask_button.dart';
import '../widgets/export_menu.dart';
import '../widgets/markdown_text.dart';

/// The member's threads with the AI coach.
///
/// A list that pushes a thread, the way the messages tab does: on a phone
/// there is room for one of the two, and stacking them is what every chat
/// app does. Every exchange is kept server-side, so this list is the
/// member's history rather than this screen's.
class AiCoachScreen extends ConsumerStatefulWidget {
  const AiCoachScreen({super.key});

  @override
  ConsumerState<AiCoachScreen> createState() => _AiCoachScreenState();
}

class _AiCoachScreenState extends ConsumerState<AiCoachScreen> {
  /// Which threads are ticked, for deleting several at once.
  final Set<String> _picked = {};

  Future<void> _reload() async {
    ref.invalidate(aiConversationsProvider);
    setState(_picked.clear);
  }

  Future<void> _open(BuildContext context, String conversationId) async {
    await Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => AiThreadScreen(conversationId: conversationId)),
    );
    await _reload();
  }

  Future<void> _deletePicked() async {
    final messenger = ScaffoldMessenger.of(context);
    final t = ref.read(tProvider);

    // Asked first, like every other deletion of something the model was paid
    // to produce.
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        content: Text(t('ai.deleteConversationConfirm')),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(context).pop(false), child: Text(t('common.cancel'))),
          FilledButton(
              onPressed: () => Navigator.of(context).pop(true), child: Text(t('common.delete'))),
        ],
      ),
    );
    if (confirmed != true) return;

    try {
      final deleted =
          await ref.read(apiProvider).deleteAiConversations(ids: _picked.toList());
      messenger.showSnackBar(
          SnackBar(content: Text(t('ai.deletedCount', {'count': '$deleted'}))));
      await _reload();
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    }
  }

  Future<void> _clearAll() async {
    final t = ref.read(tProvider);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        content: Text(t('ai.clearAllConfirm')),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(context).pop(false), child: Text(t('common.cancel'))),
          FilledButton(
              onPressed: () => Navigator.of(context).pop(true), child: Text(t('ai.clearAll'))),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    final messenger = ScaffoldMessenger.of(context);
    try {
      final deleted = await ref.read(apiProvider).deleteAiConversations(all: true);
      messenger.showSnackBar(
          SnackBar(content: Text(t('ai.deletedCount', {'count': '$deleted'}))));
      await _reload();
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final conversations = ref.watch(aiConversationsProvider);

    return Scaffold(
      body: RefreshIndicator(
        onRefresh: _reload,
        child: conversations.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => SfErrorState(
            message: ref.read(tErrorProvider)(error),
            onRetry: _reload,
            retryLabel: t('common.back'),
          ),
          data: (list) {
            if (list.isEmpty) {
              return SfRefreshableBody(
                child: SfEmptyState(
                  icon: Icons.auto_awesome_outlined,
                  title: t('ai.emptyTitle'),
                  message: t('ai.emptyBody'),
                  action: FilledButton.icon(
                    onPressed: () => _open(context, ''),
                    icon: const Icon(Icons.add),
                    label: Text(t('ai.newConversation')),
                  ),
                ),
              );
            }

            return Column(children: [
              if (_picked.isNotEmpty || list.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                  child: Row(children: [
                    if (_picked.isNotEmpty)
                      TextButton.icon(
                        onPressed: _deletePicked,
                        icon: const Icon(Icons.delete_outline, size: 18),
                        label: Text(t('ai.deletePicked', {'count': '${_picked.length}'})),
                      ),
                    const Spacer(),
                    TextButton(onPressed: _clearAll, child: Text(t('ai.clearAll'))),
                  ]),
                ),
              Expanded(
                child: ListView.separated(
                  physics: const AlwaysScrollableScrollPhysics(),
                  itemCount: list.length,
                  separatorBuilder: (context, index) => const Divider(height: 1),
                  itemBuilder: (context, index) {
                    final conversation = list[index];
                    return ListTile(
                      leading: Checkbox(
                        value: _picked.contains(conversation.id),
                        onChanged: (picked) => setState(() => picked == true
                            ? _picked.add(conversation.id)
                            : _picked.remove(conversation.id)),
                      ),
                      title: Text(conversation.title, maxLines: 2, overflow: TextOverflow.ellipsis),
                      subtitle: Text([
                        conversation.count == 1
                            ? t('ai.exchangeOne')
                            : t('ai.exchangeCount', {'count': '${conversation.count}'}),
                        if (conversation.shared) t('ai.sharedBadge'),
                      ].join(' · ')),
                      onTap: () => _open(context, conversation.id),
                    );
                  },
                ),
              ),
            ]);
          },
        ),
      ),
      floatingActionButton: FloatingActionButton(
        tooltip: t('ai.newConversation'),
        onPressed: () => _open(context, ''),
        child: const Icon(Icons.add),
      ),
    );
  }
}

/// One thread: the exchanges, and the box to add to them.
///
/// Opened with an empty id to start a new one - the first answer creates the
/// thread server-side and this screen adopts its id, so a conversation only
/// exists once something was actually said.
class AiThreadScreen extends ConsumerStatefulWidget {
  final String conversationId;

  const AiThreadScreen({super.key, required this.conversationId});

  @override
  ConsumerState<AiThreadScreen> createState() => _AiThreadScreenState();
}

class _AiThreadScreenState extends ConsumerState<AiThreadScreen> {
  final _controller = TextEditingController();
  final _scroll = ScrollController();
  AiConversation? _conversation;
  String _pending = '';
  bool _asking = false;
  bool _exporting = false;
  String _error = '';
  // Which subscription the next question is spent on. The picker is only
  // drawn when there is a choice to make - one key is never worth a control.
  String _connectionId = '';

  @override
  void initState() {
    super.initState();
    if (widget.conversationId.isNotEmpty) _load(widget.conversationId);
  }

  @override
  void dispose() {
    _controller.dispose();
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _load(String conversationId) async {
    try {
      final conversation = await ref.read(apiProvider).aiConversation(conversationId);
      if (mounted) setState(() => _conversation = conversation);
      _toBottom();
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    }
  }

  /// The newest turn, after the frame that added it: a thread that answers
  /// below the fold looks like it did not answer.
  void _toBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scroll.hasClients) {
        _scroll.animateTo(_scroll.position.maxScrollExtent,
            duration: const Duration(milliseconds: 200), curve: Curves.easeOut);
      }
    });
  }

  Future<void> _ask() async {
    final question = _controller.text.trim();
    if (question.isEmpty || _asking) return;

    setState(() {
      _asking = true;
      _error = '';
      _pending = question;
    });
    _controller.clear();
    _toBottom();

    try {
      final answer = await ref.read(apiProvider).askAi(question,
          conversationId: _conversation?.id ?? '', connectionId: _connectionId);
      await _load(answer['conversationId']?.toString() ?? _conversation?.id ?? '');
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = ref.read(tErrorProvider)(error);
          // The question goes back in the box rather than being lost with
          // the turn that never happened.
          _controller.text = question;
        });
      }
    } finally {
      if (mounted) {
        setState(() {
          _asking = false;
          _pending = '';
        });
      }
    }
  }

  Future<void> _share() async {
    final conversation = _conversation;
    if (conversation == null) return;

    final messenger = ScaffoldMessenger.of(context);
    try {
      if (conversation.shared) {
        await ref.read(apiProvider).unshareAiConversation(conversation.id);
        await _load(conversation.id);
        return;
      }
      final shared = await ref.read(apiProvider).shareAiConversation(conversation.id);
      if (mounted) setState(() => _conversation = shared);
      await SharePlus.instance.share(
          ShareParams(text: '${shared.title}\n${shared.shareUrl}'));
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    }
  }

  Future<void> _rename() async {
    final conversation = _conversation;
    if (conversation == null) return;

    final t = ref.read(tProvider);
    final controller = TextEditingController(text: conversation.title);
    final title = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('ai.rename')),
        content: TextField(controller: controller, autofocus: true),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(context).pop(), child: Text(t('common.cancel'))),
          FilledButton(
              onPressed: () => Navigator.of(context).pop(controller.text),
              child: Text(t('common.save'))),
        ],
      ),
    );
    controller.dispose();
    if (title == null || title.trim().isEmpty || !mounted) return;

    try {
      final renamed = await ref.read(apiProvider).renameAiConversation(conversation.id, title.trim());
      if (mounted) setState(() => _conversation = renamed);
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    }
  }

  Future<void> _export(ExportFormat format) async {
    final conversation = _conversation;
    if (conversation == null) return;

    setState(() => _exporting = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final directory = await getTemporaryDirectory();
      final name = conversation.title.replaceAll(RegExp(r'[^\w-]+'), '-');
      final path = await ref.read(apiProvider).downloadConversation(
            conversation.id,
            directory: directory.path,
            fileName: '$name.${format.extension}',
            format: format.extension,
          );
      if (mounted) await openExported(context, ref, path);
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final conversation = _conversation;
    final exchanges = conversation?.exchanges ?? const <AiExchange>[];
    final connections = ref.watch(aiConnectionsProvider).valueOrNull ?? const <AiConnection>[];
    if (_connectionId.isEmpty && connections.isNotEmpty) {
      _connectionId = connections.first.id;
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(conversation?.title ?? t('ai.newConversation')),
        actions: [
          if (conversation != null) ...[
            IconButton(
              onPressed: _rename,
              tooltip: t('ai.rename'),
              icon: const Icon(Icons.edit_outlined),
            ),
            IconButton(
              onPressed: _share,
              tooltip: conversation.shared ? t('ai.unshareLink') : t('ai.shareLink'),
              icon: Icon(conversation.shared ? Icons.link_off : Icons.link),
            ),
            SfExportButton(
              onExport: _export,
              busy: _exporting,
              tooltip: t('ai.exportConversation'),
              formats: const [ExportFormat.pdf, ExportFormat.png],
            ),
          ],
        ],
      ),
      body: Column(children: [
        // The subscription this thread's next question is spent on, the way
        // ~/aristochat puts its agent picker above the conversation.
        if (connections.length > 1)
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
            child: DropdownButtonFormField<String>(
              initialValue: _connectionId,
              decoration: InputDecoration(
                labelText: t('ai.connection'),
                border: const OutlineInputBorder(),
                isDense: true,
              ),
              items: [
                for (final connection in connections)
                  DropdownMenuItem(
                    value: connection.id,
                    child: Text(aiConnectionLabel(connection), overflow: TextOverflow.ellipsis),
                  ),
              ],
              onChanged: (value) => setState(() => _connectionId = value ?? ''),
            ),
          ),
        if (_error.isNotEmpty)
          Padding(
            padding: const EdgeInsets.all(12),
            child: Text(_error, style: TextStyle(color: colors.danger)),
          ),
        Expanded(
          child: exchanges.isEmpty && _pending.isEmpty
              ? SfEmptyState(
                  icon: Icons.auto_awesome_outlined,
                  title: t('ai.emptyTitle'),
                  message: t('ai.emptyBody'),
                )
              : ListView(
                  controller: _scroll,
                  padding: const EdgeInsets.all(12),
                  children: [
                    for (final exchange in exchanges) ...[
                      _Bubble(text: exchange.question, mine: true),
                      _Bubble(text: exchange.answer, mine: false),
                    ],
                    if (_pending.isNotEmpty) ...[
                      _Bubble(text: _pending, mine: true),
                      _Bubble(text: t('ai.thinking'), mine: false),
                    ],
                  ],
                ),
        ),
        SafeArea(
          top: false,
          child: Padding(
            padding: const EdgeInsets.all(8),
            child: Row(crossAxisAlignment: CrossAxisAlignment.end, children: [
              Expanded(
                child: TextField(
                  controller: _controller,
                  minLines: 1,
                  maxLines: 4,
                  textInputAction: TextInputAction.send,
                  onSubmitted: (_) => _ask(),
                  decoration: InputDecoration(
                    hintText: t('ai.placeholder'),
                    border: const OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              FilledButton(
                onPressed: _asking ? null : _ask,
                child: _asking
                    ? const SizedBox(
                        width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.send),
              ),
            ]),
          ),
        ),
      ]),
    );
  }
}

/// One turn. The member's own on the right, the way every messenger has
/// trained people to read a thread; the answer on the left, as markdown.
class _Bubble extends StatelessWidget {
  final String text;
  final bool mine;

  const _Bubble({required this.text, required this.mine});

  @override
  Widget build(BuildContext context) {
    final colors = AppColors.of(context);

    return Align(
      alignment: mine ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        constraints: BoxConstraints(maxWidth: MediaQuery.of(context).size.width * 0.85),
        decoration: BoxDecoration(
          color: mine ? colors.primaryTint : colors.backgroundMuted,
          borderRadius: BorderRadius.circular(12),
        ),
        child: mine ? Text(text) : MarkdownText(text),
      ),
    );
  }
}
