import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/profile_badges.dart';
import 'messages_screen.dart';
import 'public_profile_screen.dart';

/// Finding somebody to talk to - the phone's version of sf-ui's search page.
///
/// Two ways out of every row, because a stranger's name is not enough to write
/// to them on: their profile, which is what the web opens, and the thread
/// itself for somebody already recognised.
///
/// Nothing is filtered here. Which profiles a caller may see is decided inside
/// the API's query, so a hidden member is not merely absent from this list -
/// they were never counted in the total either.
///
/// It opens on results rather than on an empty prompt: with no criteria the
/// API answers with everybody the caller may see, so the screen is useful
/// before anything is typed and narrows as it is.
class MemberSearchScreen extends ConsumerStatefulWidget {
  const MemberSearchScreen({super.key});

  @override
  ConsumerState<MemberSearchScreen> createState() => _MemberSearchScreenState();
}

class _MemberSearchScreenState extends ConsumerState<MemberSearchScreen> {
  final _scroll = ScrollController();
  final _terms = TextEditingController();
  final _name = TextEditingController();
  final _surname = TextEditingController();
  final _username = TextEditingController();
  final _email = TextEditingController();

  /// The criteria the results on screen were fetched with - a snapshot taken
  /// when the search is submitted, not what is currently in the fields. Paging
  /// has to keep asking with the query that produced page 0, or the next page
  /// would come from a different search.
  Map<String, String> _applied = const {};

  final List<MemberResult> _results = [];
  bool _advanced = false;
  bool _loading = true;
  bool _loadedOnce = false;
  int _page = 0;
  int _total = 0;
  String? _error;

  bool get _hasMore => !_loadedOnce || _results.length < _total;

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
    _terms.dispose();
    _name.dispose();
    _surname.dispose();
    _username.dispose();
    _email.dispose();
    super.dispose();
  }

  Future<void> _load({bool reset = false}) async {
    if (reset) {
      setState(() {
        _results.clear();
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
      final page = await ref.read(apiProvider).searchMembers(
            terms: _applied['terms'] ?? '',
            name: _applied['name'] ?? '',
            surname: _applied['surname'] ?? '',
            username: _applied['username'] ?? '',
            email: _applied['email'] ?? '',
            page: _page,
          );
      if (!mounted) return;
      setState(() {
        // Merged by id rather than appended blindly: a member added between
        // two requests shifts the window, and the same row can arrive twice.
        final seen = _results.map((member) => member.id).toSet();
        _results.addAll(page.results.where((member) => !seen.contains(member.id)));
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

  void _submit() {
    FocusScope.of(context).unfocus();
    setState(() {
      _applied = {
        'terms': _terms.text,
        'name': _name.text,
        'surname': _surname.text,
        'username': _username.text,
        'email': _email.text,
      }..removeWhere((_, value) => value.trim().isEmpty);
    });
    _load(reset: true);
  }

  void _clear() {
    for (final field in [_terms, _name, _surname, _username, _email]) {
      field.clear();
    }
    setState(() => _applied = const {});
    _load(reset: true);
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);

    return Scaffold(
      appBar: AppBar(title: Text(t('search.title'))),
      body: Column(
        children: [
          _form(t),
          const Divider(height: 1),
          Expanded(child: _results.isEmpty ? _empty(t) : _list(t)),
        ],
      ),
    );
  }

  Widget _form(String Function(String, [Map<String, String>?]) t) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            controller: _terms,
            autofocus: true,
            textInputAction: TextInputAction.search,
            onSubmitted: (_) => _submit(),
            decoration: InputDecoration(
              labelText: t('search.terms'),
              hintText: t('search.termsPlaceholder'),
              prefixIcon: const Icon(Icons.search),
              suffixIcon: _applied.isEmpty
                  ? null
                  : IconButton(
                      icon: const Icon(Icons.close),
                      tooltip: t('search.clear'),
                      onPressed: _clear,
                    ),
            ),
          ),
          if (_advanced) ...[
            const SizedBox(height: 8),
            // One field per line: four side by side is what the web does with
            // the room it has, and on a phone it would leave none of them
            // wide enough to read what was typed.
            for (final field in [
              (controller: _name, label: t('auth.name'), keyboard: TextInputType.name),
              (controller: _surname, label: t('auth.surname'), keyboard: TextInputType.name),
              (controller: _username, label: t('profile.username'), keyboard: TextInputType.text),
              (controller: _email, label: t('auth.email'), keyboard: TextInputType.emailAddress),
            ])
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: TextField(
                  controller: field.controller,
                  keyboardType: field.keyboard,
                  textInputAction: TextInputAction.search,
                  onSubmitted: (_) => _submit(),
                  decoration: InputDecoration(labelText: field.label, isDense: true),
                ),
              ),
          ],
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: FilledButton.icon(
                  onPressed: _loading ? null : _submit,
                  icon: const Icon(Icons.search),
                  label: Text(t('search.submit')),
                ),
              ),
              const SizedBox(width: 8),
              TextButton(
                onPressed: () => setState(() => _advanced = !_advanced),
                child: Text(_advanced ? t('search.simple') : t('search.advanced')),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _empty(String Function(String, [Map<String, String>?]) t) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      // Refreshable like every other state of this screen: after a failure a
      // pull is the first thing anybody tries, and it should run the search
      // again rather than need the button underneath.
      return RefreshIndicator(
        onRefresh: () => _load(reset: true),
        child: SfRefreshableBody(
          child: SfErrorState(
            message: _error!,
            onRetry: () => _load(reset: true),
            retryLabel: t('common.back'),
          ),
        ),
      );
    }
    // Scrollable and refreshable like the list it replaces: a pull on an
    // empty result is somebody asking again, and it should run the search
    // rather than sit there.
    return RefreshIndicator(
      onRefresh: () => _load(reset: true),
      child: SfRefreshableBody(
        child: SfEmptyState(
          icon: Icons.person_search_outlined,
          title: t('search.noneTitle'),
          message: t('search.noneBody'),
        ),
      ),
    );
  }

  Widget _list(String Function(String, [Map<String, String>?]) t) {
    return RefreshIndicator(
      onRefresh: () => _load(reset: true),
      child: ListView.separated(
        controller: _scroll,
        physics: const AlwaysScrollableScrollPhysics(),
        // The count above the rows, and - only while there is another page -
        // the spinner that both reports it and, by coming into view, asks for
        // it. Dropped at the end of the results so the list doesn't finish on
        // a divider under nothing.
        itemCount: _results.length + 1 + (_loading || _hasMore ? 1 : 0),
        separatorBuilder: (context, index) => const Divider(height: 1),
        itemBuilder: (context, index) {
          if (index == 0) {
            return Padding(
              padding: const EdgeInsets.fromLTRB(16, 10, 16, 6),
              child: Text(
                t('search.results', {'count': '$_total'}),
                style: TextStyle(color: AppColors.of(context).textMuted),
              ),
            );
          }
          if (index <= _results.length) {
            return _MemberRow(member: _results[index - 1]);
          }
          return Padding(
            padding: const EdgeInsets.all(24),
            child: Center(
              child: _loading
                  ? const CircularProgressIndicator()
                  : const SizedBox.shrink(),
            ),
          );
        },
      ),
    );
  }
}

/// One person: who they are, and the two things you came here to do.
class _MemberRow extends ConsumerWidget {
  final MemberResult member;

  const _MemberRow({required this.member});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    // A member with no profile name has no profile page to open - the page is
    // addressed by handle - so the row only offers the conversation.
    final openable = member.handle.isNotEmpty;

    return ListTile(
      leading: SfAvatar(picture: member.picture, name: member.fullName),
      title: Text(member.fullName, style: const TextStyle(fontWeight: FontWeight.w600)),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (openable || member.sharesClub)
            Row(
              children: [
                if (openable)
                  Flexible(
                    child: Text(
                      '@${member.handle}',
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: colors.textMuted, fontSize: 13),
                    ),
                  ),
                if (member.sharesClub) ...[
                  if (openable) const SizedBox(width: 8),
                  Icon(Icons.groups_outlined, size: 14, color: colors.textMuted),
                  const SizedBox(width: 4),
                  Flexible(
                    child: Text(
                      t('search.sharesClub'),
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: colors.textMuted, fontSize: 13),
                    ),
                  ),
                ],
              ],
            ),
          const SizedBox(height: 4),
          ProfileBadges(
            role: member.role,
            specialty: member.specialty,
            alignment: MainAxisAlignment.start,
          ),
        ],
      ),
      isThreeLine: true,
      trailing: IconButton(
        icon: const Icon(Icons.chat_bubble_outline),
        tooltip: t('messages.message'),
        onPressed: () => Navigator.of(context).push(MaterialPageRoute(
          builder: (_) => ThreadScreen(userId: member.id, title: member.fullName),
        )),
      ),
      onTap: openable
          ? () => Navigator.of(context).push(MaterialPageRoute(
                builder: (_) => PublicProfileScreen(handle: member.handle),
              ))
          : null,
    );
  }
}
