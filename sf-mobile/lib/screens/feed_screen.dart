import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/media_player.dart';
import '../widgets/social_share.dart';

/// The feed: posts from the people you follow, your own, and your clubs'.
/// "Discover" switches to every public post, which is what a new account needs
/// before it follows anyone.
class FeedScreen extends ConsumerStatefulWidget {
  const FeedScreen({super.key});

  @override
  ConsumerState<FeedScreen> createState() => _FeedScreenState();
}

class _FeedScreenState extends ConsumerState<FeedScreen> {
  final _scroll = ScrollController();
  final List<Post> _posts = [];

  bool _discover = false;
  bool _loading = true;
  bool _loadedOnce = false;
  int _page = 0;
  int _total = 0;
  String? _error;

  bool get _hasMore => !_loadedOnce || _posts.length < _total;

  @override
  void initState() {
    super.initState();
    _scroll.addListener(() {
      if (_scroll.position.pixels > _scroll.position.maxScrollExtent - 300 && !_loading && _hasMore) {
        _load();
      }
    });
    _load();
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _load({bool reset = false}) async {
    if (reset) {
      setState(() {
        _posts.clear();
        _page = 0;
        _total = 0;
        _loadedOnce = false;
      });
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final page = await ref.read(apiProvider).feed(page: _page, discover: _discover);
      if (!mounted) return;
      setState(() {
        _posts.addAll(page.results);
        _total = page.totalResults;
        _page += 1;
        _loadedOnce = true;
      });
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
          child: SegmentedButton<bool>(
            segments: [
              ButtonSegment(value: false, label: Text(t('feed.following'))),
              ButtonSegment(value: true, label: Text(t('feed.discover'))),
            ],
            selected: {_discover},
            onSelectionChanged: (selection) {
              setState(() => _discover = selection.first);
              _load(reset: true);
            },
          ),
        ),
        Expanded(
          child: _error != null && _posts.isEmpty
              ? SfErrorState(message: _error!, retryLabel: t('common.back'), onRetry: () => _load(reset: true))
              : RefreshIndicator(
                  onRefresh: () => _load(reset: true),
                  child: ListView.builder(
                    controller: _scroll,
                    padding: const EdgeInsets.all(16),
                    itemCount: _posts.length + 1,
                    itemBuilder: (context, index) {
                      if (index == _posts.length) {
                        if (_loading) {
                          return const Padding(
                            padding: EdgeInsets.all(24),
                            child: Center(child: CircularProgressIndicator()),
                          );
                        }
                        if (_posts.isEmpty && _loadedOnce) {
                          return SfEmptyState(
                            icon: Icons.forum_outlined,
                            title: t(_discover ? 'feed.emptyDiscover' : 'feed.empty'),
                          );
                        }
                        return const SizedBox(height: 40);
                      }
                      return _PostCard(
                        post: _posts[index],
                        onChanged: (updated) => setState(() => _posts[index] = updated),
                        onDeleted: () => setState(() => _posts.removeAt(index)),
                      );
                    },
                  ),
                ),
        ),
      ],
    );
  }
}

class _PostCard extends ConsumerWidget {
  final Post post;
  final void Function(Post) onChanged;
  final VoidCallback onDeleted;

  const _PostCard({required this.post, required this.onChanged, required this.onDeleted});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                SfAvatar.of(post.author, radius: 18),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(post.author.fullName, style: const TextStyle(fontWeight: FontWeight.bold)),
                      Text(
                        '${post.createdAt.toLocal().toString().split('.').first}'
                        '${post.clubName.isNotEmpty ? ' · ${post.clubName}' : ''}',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
                Chip(
                  label: Text(
                    t(post.visibility == 'club' ? 'feed.visibilityClub' : 'feed.visibilityPublic'),
                    style: const TextStyle(fontSize: 11),
                  ),
                  visualDensity: VisualDensity.compact,
                ),
              ],
            ),
            if (post.content.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text(post.content),
            ],
            for (final picture in post.pictures) ...[
              const SizedBox(height: 10),
              _Base64Image(data: picture),
            ],
            // The same players the web app's <media-player> renders, so a post
            // looks the same on a phone as it does in a browser. Nothing loads
            // until it is tapped.
            for (final link in post.links) ...[
              const SizedBox(height: 8),
              // The framed document claims to be served from this deployment's
              // own origin: an embed with no origin to report is what YouTube
              // refuses with a configuration error.
              MediaPlayer(url: link, baseUrl: ref.read(apiProvider).client.apiUrl),
            ],
            const Divider(height: 20),
            Row(
              children: [
                // A like says how a post landed with other people, so its
                // author only ever sees the count. The API refuses a self-like
                // too; this keeps a dead control off the screen.
                if (post.author.id == ref.watch(sessionProvider).user?.id)
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.favorite_border, size: 18, color: AppColors.of(context).textMuted),
                        const SizedBox(width: 6),
                        Text('${post.likes}',
                            style: TextStyle(color: AppColors.of(context).textMuted)),
                      ],
                    ),
                  )
                else
                  TextButton.icon(
                    icon: Icon(post.liked ? Icons.favorite : Icons.favorite_border,
                        color: post.liked ? Theme.of(context).colorScheme.error : null),
                    label: Text('${post.likes}'),
                    onPressed: () async {
                      try {
                        onChanged(await ref.read(apiProvider).like(post.id, post.liked));
                      } catch (error) {
                        if (context.mounted) {
                          ScaffoldMessenger.of(context)
                              .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
                        }
                      }
                    },
                  ),
                // Only a public post can be shared: a club-only post's link
                // would 404 for whoever opened it.
                if (post.visibility == 'public')
                  IconButton(
                    icon: const Icon(Icons.share_outlined),
                    tooltip: ref.read(tProvider)('share.label'),
                    onPressed: () => showModalBottomSheet<void>(
                      context: context,
                      showDragHandle: true,
                      builder: (_) => SafeArea(
                        child: Padding(
                          padding: const EdgeInsets.all(16),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Text(ref.read(tProvider)('share.label')),
                              const SizedBox(height: 8),
                              SocialShareRow(
                                url: '${ref.read(apiProvider).client.frontendUrl}/posts/${post.id}',
                                text: shareTextFor(post.content, ref.read(tProvider)('share.postText')),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                TextButton.icon(
                  icon: const Icon(Icons.mode_comment_outlined),
                  label: Text('${post.comments}'),
                  onPressed: () => _openComments(context, ref),
                ),
                const Spacer(),
                if (post.deletable)
                  IconButton(
                    icon: const Icon(Icons.delete_outline),
                    tooltip: t('common.delete'),
                    onPressed: () => _confirmDelete(context, ref),
                  )
                else
                  IconButton(
                    icon: const Icon(Icons.flag_outlined),
                    tooltip: t('feed.report'),
                    onPressed: () => _report(context, ref),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _openComments(BuildContext context, WidgetRef ref) async {
    final added = await showModalBottomSheet<int>(
      context: context,
      isScrollControlled: true,
      builder: (context) => Padding(
        padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
        child: _CommentsSheet(post: post),
      ),
    );
    if (added != null && added != 0) onChanged(post.copyWith(comments: post.comments + added));
  }

  Future<void> _confirmDelete(BuildContext context, WidgetRef ref) async {
    final t = ref.read(tProvider);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('common.delete')),
        content: Text(t('feed.confirmDeletePost')),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: Text(t('common.cancel'))),
          FilledButton(onPressed: () => Navigator.of(context).pop(true), child: Text(t('common.delete'))),
        ],
      ),
    );
    if (confirmed != true) return;

    try {
      await ref.read(apiProvider).deletePost(post.id);
      onDeleted();
    } catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    }
  }

  Future<void> _report(BuildContext context, WidgetRef ref) async {
    final t = ref.read(tProvider);
    final reason = await showModalBottomSheet<String>(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(title: Text(t('feed.reportReason')), dense: true),
            for (final entry in const {
              'spam': 'feed.reportSpam',
              'abuse': 'feed.reportAbuse',
              'inappropriate': 'feed.reportInappropriate',
              'other': 'feed.reportOther',
            }.entries)
              ListTile(title: Text(t(entry.value)), onTap: () => Navigator.of(context).pop(entry.key)),
          ],
        ),
      ),
    );
    if (reason == null) return;

    try {
      await ref.read(apiProvider).report(targetType: 'post', targetId: post.id, reason: reason);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(t('feed.reported'))));
      }
    } catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    }
  }
}

class _Base64Image extends StatelessWidget {
  final String data;

  const _Base64Image({required this.data});

  @override
  Widget build(BuildContext context) {
    final comma = data.indexOf(',');
    Uint8List bytes;
    try {
      bytes = base64Decode(comma >= 0 ? data.substring(comma + 1) : data);
    } catch (_) {
      return const SizedBox.shrink();
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular(10),
      child: Image.memory(bytes, fit: BoxFit.cover, width: double.infinity),
    );
  }
}

class _CommentsSheet extends ConsumerStatefulWidget {
  final Post post;

  const _CommentsSheet({required this.post});

  @override
  ConsumerState<_CommentsSheet> createState() => _CommentsSheetState();
}

class _CommentsSheetState extends ConsumerState<_CommentsSheet> {
  final _input = TextEditingController();
  List<Comment>? _comments;
  int _delta = 0;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    ref
        .read(apiProvider)
        .comments(widget.post.id)
        .then((page) => mounted ? setState(() => _comments = page.results) : null)
        .catchError((_) => mounted ? setState(() => _comments = <Comment>[]) : null);
  }

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  /// Edits one comment, in a dialog rather than inline: a bottom sheet's list
  /// is already scrolling inside a constrained box, and growing a row into a
  /// form inside it fights the keyboard for what little height is left.
  Future<void> _edit(Comment comment) async {
    final t = ref.read(tProvider);
    final controller = TextEditingController(text: comment.content);

    final content = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('common.edit')),
        content: TextField(controller: controller, maxLines: 4, autofocus: true),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text(t('common.cancel'))),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text.trim()),
            child: Text(t('common.save')),
          ),
        ],
      ),
    );
    controller.dispose();

    if (content == null || content.isEmpty || content == comment.content) return;
    try {
      final updated = await ref.read(apiProvider).updateComment(widget.post.id, comment.id, content);
      if (mounted) {
        setState(() => _comments = [
              for (final item in _comments ?? <Comment>[])
                if (item.id == updated.id) updated else item,
            ]);
      }
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    }
  }

  Future<void> _submit() async {
    final content = _input.text.trim();
    if (content.isEmpty) return;
    setState(() => _busy = true);
    try {
      final comment = await ref.read(apiProvider).addComment(widget.post.id, content);
      setState(() {
        _comments = [...?_comments, comment];
        _delta += 1;
        _input.clear();
      });
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (_comments == null)
              const Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator())
            else
              ConstrainedBox(
                constraints: BoxConstraints(maxHeight: MediaQuery.of(context).size.height * 0.5),
                child: ListView(
                  shrinkWrap: true,
                  children: [
                    for (final comment in _comments!)
                      ListTile(
                        leading: SfAvatar.of(comment.author, radius: 16),
                        title: Row(
                          children: [
                            Flexible(child: Text(comment.author.fullName, overflow: TextOverflow.ellipsis)),
                            // Says so when it has been changed since: a comment
                            // somebody replied to may not be the one on screen.
                            if (comment.wasEdited) ...[
                              const SizedBox(width: 6),
                              Text(
                                t('feed.edited'),
                                style: TextStyle(fontSize: 11, color: AppColors.of(context).textMuted),
                              ),
                            ],
                          ],
                        ),
                        subtitle: Text(comment.content),
                        dense: true,
                        trailing: comment.editable
                            ? IconButton(
                                icon: const Icon(Icons.edit_outlined, size: 18),
                                tooltip: t('common.edit'),
                                onPressed: () => _edit(comment),
                              )
                            : null,
                      ),
                  ],
                ),
              ),
            const Divider(),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _input,
                    decoration: InputDecoration(hintText: t('feed.writeComment')),
                    onSubmitted: (_) => _submit(),
                  ),
                ),
                IconButton(icon: const Icon(Icons.send), onPressed: _busy ? null : _submit),
              ],
            ),
            TextButton(
              onPressed: () => Navigator.of(context).pop(_delta),
              child: Text(t('common.close')),
            ),
          ],
        ),
      ),
    );
  }
}

/// Composes a post. Pictures are read into base64 data URIs and carried inline
/// in the payload, the same way the web app does it - there is no separate
/// upload step.
class ComposePostScreen extends ConsumerStatefulWidget {
  const ComposePostScreen({super.key});

  @override
  ConsumerState<ComposePostScreen> createState() => _ComposePostScreenState();
}

class _ComposePostScreenState extends ConsumerState<ComposePostScreen> {
  final _content = TextEditingController();
  final List<String> _pictures = [];

  String _visibility = 'public';
  String _clubId = '';
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _content.dispose();
    super.dispose();
  }

  Future<void> _addPicture() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery, maxWidth: 1600, imageQuality: 80);
    if (picked == null) return;
    final bytes = await picked.readAsBytes();
    if (!mounted) return;
    setState(() => _pictures.add('data:image/jpeg;base64,${base64Encode(bytes)}'));
  }

  Future<void> _submit() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(apiProvider).createPost(
            content: _content.text.trim(),
            pictures: _pictures,
            visibility: _visibility,
            clubId: _visibility == 'club' ? _clubId : '',
          );
      if (mounted) Navigator.of(context).pop(true);
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = ref.read(tErrorProvider)(error);
          _busy = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final clubs = ref.watch(clubsProvider).valueOrNull ?? const <Club>[];

    return Scaffold(
      appBar: AppBar(
        title: Text(t('feed.post')),
        actions: [
          TextButton(
            onPressed: _busy ? null : _submit,
            child: Text(t('feed.post'), style: const TextStyle(color: Colors.white)),
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          TextField(
            controller: _content,
            maxLines: 6,
            autofocus: true,
            decoration: InputDecoration(hintText: t('feed.compose')),
          ),
          // No link field: paste the URL into the text above and the API
          // picks it up, the same way it does on the web.
          const SizedBox(height: 12),
          for (final picture in _pictures) ...[
            _Base64Image(data: picture),
            const SizedBox(height: 8),
          ],
          OutlinedButton.icon(
            icon: const Icon(Icons.image_outlined),
            label: Text(t('feed.addPicture')),
            onPressed: _addPicture,
          ),
          const SizedBox(height: 16),
          DropdownButtonFormField<String>(
            initialValue: _visibility,
            decoration: InputDecoration(labelText: t('feed.visibility')),
            items: [
              DropdownMenuItem(value: 'public', child: Text(t('feed.visibilityPublic'))),
              if (clubs.isNotEmpty) DropdownMenuItem(value: 'club', child: Text(t('feed.visibilityClub'))),
            ],
            onChanged: (value) => setState(() => _visibility = value ?? 'public'),
          ),
          if (_visibility == 'club') ...[
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              initialValue: _clubId.isEmpty ? null : _clubId,
              decoration: InputDecoration(labelText: t('feed.pickClub')),
              items: [
                for (final club in clubs) DropdownMenuItem(value: club.id, child: Text(club.name)),
              ],
              onChanged: (value) => setState(() => _clubId = value ?? ''),
            ),
          ],
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          ],
        ],
      ),
    );
  }
}
