import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import 'public_profile_screen.dart';
import '../widgets/media_player.dart';
import '../widgets/linkified_text.dart';

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
              final unread = conversation.unread > 0;
              return ListTile(
                // An unread thread carries weight and a tint, not just a
                // badge: a badge down the right is easy to miss when scanning
                // a long list for what still needs you.
                tileColor: unread ? colors.primaryTint.withValues(alpha: 0.35) : null,
                leading: SfAvatar.of(conversation.other),
                title: Text(
                  '${conversation.other.name} ${conversation.other.surname}'.trim(),
                  style: TextStyle(fontWeight: unread ? FontWeight.w700 : FontWeight.w500),
                ),
                subtitle: Text(
                  conversation.lastMessage,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: unread ? colors.text : colors.textMuted,
                    fontWeight: unread ? FontWeight.w500 : FontWeight.w400,
                  ),
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
  final _recorder = AudioRecorder();
  bool _busy = false;

  /// Pictures attached to the message being written, as base64 data URIs -
  /// the same form the feed's composer uses, and what the API stores inline.
  final List<String> _pictures = [];

  /// A recorded voice message, already uploaded and waiting to be sent.
  String _audio = '';
  bool _recording = false;

  /// Upload progress, 0..1, or null when nothing is uploading. A voice note or
  /// a video goes to the member's own storage over their phone's connection,
  /// which is slow often enough to need saying.
  double? _uploading;

  @override
  void dispose() {
    _draft.dispose();
    _scroll.dispose();
    _recorder.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final content = _draft.text.trim();
    if (content.isEmpty && _pictures.isEmpty && _audio.isEmpty) return;

    setState(() => _busy = true);
    try {
      final message = await ref.read(apiProvider).sendMessage(
            widget.userId,
            content: content,
            pictures: _pictures,
            audio: _audio,
          );
      _draft.clear();
      // Appended locally rather than refetching: the thread is already on
      // screen, and a full reload would lose the scroll position mid-sentence.
      setState(() {
        _sent.add(message);
        _pictures.clear();
        _audio = '';
      });
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

  /// Attaches a photo, carried inline as base64 the way a post's pictures are.
  Future<void> _addPicture() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery, maxWidth: 1600, imageQuality: 80);
    if (picked == null) return;
    final bytes = await picked.readAsBytes();
    if (!mounted) return;
    setState(() => _pictures.add('data:image/jpeg;base64,${base64Encode(bytes)}'));
  }

  /// Uploads a video and appends its URL to the draft.
  ///
  /// Appended to the text rather than stored beside the message: from there it
  /// is an ordinary link, and the same detection that renders a pasted YouTube
  /// URL plays it. Exactly what the web composer does.
  Future<void> _addVideo() async {
    final picked = await ImagePicker().pickVideo(source: ImageSource.gallery);
    if (picked == null) return;

    setState(() => _uploading = 0);
    try {
      final url = await ref.read(apiProvider).uploadVideo(
            picked.path,
            onProgress: (value) {
              if (mounted) setState(() => _uploading = value);
            },
          );
      if (!mounted || url.isEmpty) return;
      final text = _draft.text.trim();
      _draft.text = text.isEmpty ? url : '$text\n$url';
    } catch (error) {
      // A member with no storage configured gets a 405, which carries its own
      // i18n code and reads as "set up your storage first".
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _uploading = null);
    }
  }

  /// Starts or stops recording a voice message.
  ///
  /// The recording is uploaded on stop rather than on send, so the wait happens
  /// while the sender is still deciding, and the message goes the instant they
  /// press send.
  Future<void> _toggleRecording() async {
    final t = ref.read(tProvider);

    if (_recording) {
      final path = await _recorder.stop();
      setState(() => _recording = false);
      if (path == null) return;

      setState(() => _uploading = 0);
      try {
        final url = await ref.read(apiProvider).uploadAudio(
              path,
              onProgress: (value) {
                if (mounted) setState(() => _uploading = value);
              },
            );
        if (mounted) setState(() => _audio = url);
      } catch (error) {
        if (mounted) {
          ScaffoldMessenger.of(context)
              .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
        }
      } finally {
        if (mounted) setState(() => _uploading = null);
      }
      return;
    }

    if (!await _recorder.hasPermission()) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(t('messages.micDenied'))));
      }
      return;
    }
    final directory = await getTemporaryDirectory();
    // Named from the conversation rather than the clock: this file is
    // overwritten by the next recording in the same thread, and never needs to
    // outlive the upload.
    final path = '${directory.path}/voice-${widget.userId}.m4a';
    await _recorder.start(const RecordConfig(), path: path);
    if (mounted) setState(() => _recording = true);
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
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (_uploading != null) ...[
                    LinearProgressIndicator(value: _uploading),
                    const SizedBox(height: 8),
                  ],

                  // What is already attached, so nothing is sent unseen.
                  if (_pictures.isNotEmpty || _audio.isNotEmpty) ...[
                    Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        for (var i = 0; i < _pictures.length; i++)
                          _Attachment(
                            label: t('feed.addPicture'),
                            icon: Icons.image_outlined,
                            onRemove: () => setState(() => _pictures.removeAt(i)),
                          ),
                        if (_audio.isNotEmpty)
                          _Attachment(
                            label: t('messages.voiceMessage'),
                            icon: Icons.mic,
                            onRemove: () => setState(() => _audio = ''),
                          ),
                      ],
                    ),
                    const SizedBox(height: 8),
                  ],

                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      IconButton(
                        icon: const Icon(Icons.image_outlined),
                        tooltip: t('feed.addPicture'),
                        onPressed: _busy || _uploading != null || _pictures.length >= 4 ? null : _addPicture,
                      ),
                      IconButton(
                        icon: const Icon(Icons.videocam_outlined),
                        tooltip: t('feed.addVideo'),
                        onPressed: _busy || _uploading != null ? null : _addVideo,
                      ),
                      IconButton(
                        // Red while recording: the one control here with a
                        // running state, and the only way to tell the mic is
                        // live without a waveform.
                        icon: Icon(_recording ? Icons.stop : Icons.mic_none),
                        color: _recording ? Theme.of(context).colorScheme.error : null,
                        tooltip: t('messages.recordVoice'),
                        onPressed: _busy || _uploading != null ? null : _toggleRecording,
                      ),
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
                        onPressed: _busy || _uploading != null ? null : _send,
                      ),
                    ],
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

/// One attached picture or voice note in the composer, with a way to drop it.
class _Attachment extends StatelessWidget {
  final String label;
  final IconData icon;
  final VoidCallback onRemove;

  const _Attachment({required this.label, required this.icon, required this.onRemove});

  @override
  Widget build(BuildContext context) {
    return Chip(
      avatar: Icon(icon, size: 16),
      label: Text(label),
      onDeleted: onRemove,
      visualDensity: VisualDensity.compact,
    );
  }
}

class _Bubble extends ConsumerWidget {
  final PrivateMessage message;

  const _Bubble({required this.message});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = AppColors.of(context);

    final bubble = Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      constraints: BoxConstraints(maxWidth: MediaQuery.of(context).size.width * 0.72),
      decoration: BoxDecoration(
        color: message.mine ? colors.primaryTint : colors.backgroundMuted,
        borderRadius: BorderRadius.circular(AppRadius.value),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (message.content.isNotEmpty) LinkifiedText(message.content),
          for (final picture in message.pictures) ...[
            const SizedBox(height: 6),
            SfBase64Image(data: picture),
          ],
          if (message.audio.isNotEmpty) ...[
            const SizedBox(height: 6),
            AudioBubble(url: message.audio),
          ],
          // The same players the feed renders, so a link shared in a thread
          // behaves like one shared publicly.
          for (final link in message.links) ...[
            const SizedBox(height: 6),
            MediaPlayer(url: link, baseUrl: ref.read(apiProvider).client.apiUrl),
          ],
          const SizedBox(height: 2),
          Text(
            _time(message.createdAt),
            style: TextStyle(color: colors.textMuted, fontSize: 11),
          ),
        ],
      ),
    );

    // Your own messages need no avatar - you know who you are. The other
    // side's carries one on every message, and tapping it opens their profile,
    // which is the gesture every messenger has trained people to expect.
    if (message.mine) {
      return Align(alignment: Alignment.centerRight, child: bubble);
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(right: 8, bottom: 8),
          child: GestureDetector(
            onTap: () => _openProfile(context, ref),
            child: SfAvatar.of(message.sender, radius: 16),
          ),
        ),
        Flexible(child: bubble),
      ],
    );
  }

  /// Opens the sender's profile, when there is one to open.
  ///
  /// A handle is the app's own signal that a profile is reachable: the API
  /// leaves it out of a summary the caller may not read, so its absence is the
  /// answer rather than something to discover by navigating and failing.
  void _openProfile(BuildContext context, WidgetRef ref) {
    if (message.sender.handle.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(ref.read(tProvider)('profile.notVisible'))),
      );
      return;
    }
    Navigator.of(context).push(MaterialPageRoute(
      builder: (_) => PublicProfileScreen(handle: message.sender.handle),
    ));
  }

  String _time(DateTime at) =>
      '${at.day.toString().padLeft(2, '0')}/${at.month.toString().padLeft(2, '0')} '
      '${at.hour.toString().padLeft(2, '0')}:${at.minute.toString().padLeft(2, '0')}';
}
