import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import 'multi_select.dart';

/// The member's connections to a language model, and who else may spend them.
///
/// Deliberately the storage settings over again: an account holds a
/// credential and decides who else may use it. A coach paying for one key and
/// letting their athletes ask it questions is the same arrangement as a club
/// sharing a bucket.
class AiConnectionsSection extends ConsumerWidget {
  const AiConnectionsSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final connections = ref.watch(aiConnectionsProvider);
    final providers = ref.watch(aiProvidersProvider).valueOrNull ?? const <AiProvider>[];
    final me = ref.watch(sessionProvider).user?.id ?? '';

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(t('ai.title'), style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(t('ai.subtitle'), style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 8),
          connections.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(12),
              child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
            ),
            error: (error, _) => Text(ref.read(tErrorProvider)(error),
                style: TextStyle(color: AppColors.of(context).danger)),
            data: (list) {
              // The member's own, in order. The ones shared with them follow
              // and have no arrows: the owner decides where those sit.
              final mine = list.where((connection) => connection.ownerId == me).toList();
              return Column(children: [
              if (list.isEmpty)
                Align(
                  alignment: AlignmentDirectional.centerStart,
                  child: Text(t('ai.noConnections'),
                      style: Theme.of(context).textTheme.bodySmall),
                ),
              if (mine.length > 1)
                Align(
                  alignment: AlignmentDirectional.centerStart,
                  child: Text(t('ai.orderHelp'), style: Theme.of(context).textTheme.bodySmall),
                ),
              for (final connection in list)
                _ConnectionTile(
                  connection: connection,
                  mine: connection.ownerId == me,
                  providers: providers,
                  position: mine.indexWhere((candidate) => candidate.id == connection.id),
                  of: mine.length,
                  onMove: (by) => _move(context, ref, mine, connection.id, by),
                ),
              const SizedBox(height: 8),
              Align(
                alignment: AlignmentDirectional.centerStart,
                child: FilledButton.icon(
                  onPressed: () => _edit(context, ref, null),
                  icon: const Icon(Icons.add),
                  label: Text(t('ai.addConnection')),
                ),
              ),
            ]);
            },
          ),
        ]),
      ),
    );
  }
}

/// Moving one is a swap with its neighbour, sent as the whole order: the API
/// takes a list, so there is one way for the order to be wrong rather than
/// two.
Future<void> _move(BuildContext context, WidgetRef ref, List<AiConnection> mine,
    String connectionId, int by) async {
  final ids = [for (final connection in mine) connection.id];
  final from = ids.indexOf(connectionId);
  final to = from + by;
  if (from < 0 || to < 0 || to >= ids.length) return;

  final messenger = ScaffoldMessenger.of(context);
  ids[from] = ids[to];
  ids[to] = connectionId;
  try {
    await ref.read(apiProvider).reorderAiConnections(ids);
    ref.invalidate(aiConnectionsProvider);
  } catch (error) {
    messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
  }
}

/// Opens the form for a new connection or an existing one, and reloads the
/// list when something was saved.
Future<void> _edit(BuildContext context, WidgetRef ref, AiConnection? connection) async {
  final saved = await showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    builder: (context) => Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: _ConnectionForm(connection: connection),
    ),
  );
  if (saved == true) ref.invalidate(aiConnectionsProvider);
}

class _ConnectionTile extends ConsumerWidget {
  final AiConnection connection;
  final bool mine;
  final List<AiProvider> providers;

  /// Where it sits in the member's own order, and how many there are - what
  /// the arrows need to know when to stop.
  final int position;
  final int of;
  final void Function(int by) onMove;

  const _ConnectionTile({
    required this.connection,
    required this.mine,
    required this.providers,
    required this.position,
    required this.of,
    required this.onMove,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    // The vendor's own name, from the server's catalogue: branding rather
    // than a translated string, so a copy in three dictionaries would be a
    // second list to keep in step with the table.
    final provider = providerLabel(providers, connection.provider);

    return ListTile(
      contentPadding: EdgeInsets.zero,
      title: Text(connection.name.isEmpty ? provider : connection.name),
      subtitle: Text([
        provider,
        if (connection.model.isNotEmpty) connection.model,
        if (mine && position == 0) t('ai.first'),
        if (!mine) t('ai.sharedBy', {'name': connection.ownerName}),
      ].join(' · ')),
      trailing: mine
          ? Row(mainAxisSize: MainAxisSize.min, children: [
              IconButton(
                onPressed: position <= 0 ? null : () => onMove(-1),
                tooltip: t('ai.moveUp'),
                icon: const Icon(Icons.arrow_upward, size: 20),
                visualDensity: VisualDensity.compact,
              ),
              IconButton(
                onPressed: position < 0 || position >= of - 1 ? null : () => onMove(1),
                tooltip: t('ai.moveDown'),
                icon: const Icon(Icons.arrow_downward, size: 20),
                visualDensity: VisualDensity.compact,
              ),
              IconButton(
                onPressed: () => _shares(context, ref, connection),
                tooltip: t('ai.shares'),
                icon: const Icon(Icons.group_outlined, size: 20),
                visualDensity: VisualDensity.compact,
              ),
              IconButton(
                onPressed: () => _edit(context, ref, connection),
                tooltip: t('common.edit'),
                icon: const Icon(Icons.edit_outlined, size: 20),
                visualDensity: VisualDensity.compact,
              ),
              IconButton(
                onPressed: () => _delete(context, ref),
                tooltip: t('ai.clear'),
                icon: const Icon(Icons.delete_outline, size: 20),
                visualDensity: VisualDensity.compact,
              ),
            ])
          : Text(t('ai.role.user'), style: Theme.of(context).textTheme.bodySmall),
    );
  }

  Future<void> _delete(BuildContext context, WidgetRef ref) async {
    final messenger = ScaffoldMessenger.of(context);
    final t = ref.read(tProvider);
    try {
      await ref.read(apiProvider).deleteAiConnection(connection.id);
      ref.invalidate(aiConnectionsProvider);
      messenger.showSnackBar(SnackBar(content: Text(t('ai.cleared'))));
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    }
  }

  Future<void> _shares(BuildContext context, WidgetRef ref, AiConnection connection) =>
      showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        builder: (context) => _SharesSheet(connectionId: connection.id),
      );
}

/// The connection form.
///
/// The address is a field rather than a constant because that is the point:
/// picking Anthropic and then aiming it at a LiteLLM on the next machine is
/// the arrangement this was built for. Picking a provider fills in the
/// address and the model, so somebody who only has a key is one paste away.
class _ConnectionForm extends ConsumerStatefulWidget {
  final AiConnection? connection;

  const _ConnectionForm({required this.connection});

  @override
  ConsumerState<_ConnectionForm> createState() => _ConnectionFormState();
}

class _ConnectionFormState extends ConsumerState<_ConnectionForm> {
  late final TextEditingController _name;
  late final TextEditingController _baseUrl;
  late final TextEditingController _modelField;
  late final TextEditingController _maxTokens;
  final _token = TextEditingController();
  final _modelFocus = FocusNode();
  late String _provider;
  List<AiProvider> _providers = const [];
  bool _busy = false;
  String _error = '';
  /// The random half of the suggested name, minted once for this form: it
  /// must not change under the member while they look at it, and two
  /// connections to the same model have to be tellable apart.
  final String _suffix = _randomSuffix();

  @override
  void initState() {
    super.initState();
    final connection = widget.connection;
    _name = TextEditingController(text: connection?.name ?? '');
    _baseUrl = TextEditingController(text: connection?.baseUrl ?? '');
    _modelField = TextEditingController(text: connection?.model ?? '');
    _maxTokens = TextEditingController(
      text: (connection?.maxTokens ?? 0) > 0 ? '${connection!.maxTokens}' : '',
    );
    _provider = connection?.provider ?? 'mistral';
    _loadProviders();
  }

  /// The defaults the picker fills in. A provider list that cannot be
  /// fetched leaves the form usable with whatever the member types.
  Future<void> _loadProviders() async {
    try {
      final providers = await ref.read(apiProvider).aiProviders();
      if (mounted) setState(() => _providers = providers);
    } catch (_) {
      // Left empty on purpose.
    }
  }

  @override
  void dispose() {
    _name.dispose();
    _baseUrl.dispose();
    _modelField.dispose();
    _maxTokens.dispose();
    _token.dispose();
    _modelFocus.dispose();
    super.dispose();
  }

  AiProvider? get _defaults =>
      _providers.where((provider) => provider.name == _provider).firstOrNull;

  /// Switching provider rewrites the address and the model unless the member
  /// typed their own: changing vendor and keeping the old vendor's URL is
  /// never what was meant, but neither is losing a hand-typed gateway.
  void _pickProvider(String provider) {
    final previous = _defaults;
    final next = _providers.where((entry) => entry.name == provider).firstOrNull;
    setState(() {
      if (_baseUrl.text.isEmpty || _baseUrl.text == previous?.defaultUrl) {
        _baseUrl.text = next?.defaultUrl ?? '';
      }
      if (_modelField.text.isEmpty || _modelField.text == previous?.defaultModel) {
        _modelField.text = next?.defaultModel ?? '';
      }
      _provider = provider;
    });
  }

  /// What the name field offers, and what a blank one becomes: the model it
  /// points at, plus the suffix. The API fills a blank name exactly this way,
  /// so what the placeholder promised is what gets saved.
  String get _model =>
      _modelField.text.trim().isEmpty ? (_defaults?.defaultModel ?? '') : _modelField.text.trim();

  String get _suggestedName => _model.isEmpty ? '' : '$_model-$_suffix';

  Future<void> _save() async {
    setState(() {
      _busy = true;
      _error = '';
    });
    final payload = {
      'name': _name.text.trim().isEmpty ? _suggestedName : _name.text.trim(),
      'provider': _provider,
      'baseUrl': _baseUrl.text.trim().isEmpty ? (_defaults?.defaultUrl ?? '') : _baseUrl.text.trim(),
      'model': _model,
      // Blank or non-numeric leaves it to the API, which fills in the default.
      'maxTokens': int.tryParse(_maxTokens.text.trim()) ?? 0,
      'token': _token.text.trim(),
    };
    try {
      final api = ref.read(apiProvider);
      final existing = widget.connection;
      if (existing == null) {
        await api.addAiConnection(payload);
      } else {
        await api.setAiConnection(existing.id, payload);
      }
      if (mounted) Navigator.of(context).pop(true);
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final connection = widget.connection;
    // Until the catalogue arrives, the one the connection already points at:
    // a dropdown with nothing in it cannot show its own value.
    final names = _providers.isEmpty
        ? [AiProvider(name: _provider, label: _provider)]
        : _providers;

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(
            controller: _name,
            decoration: InputDecoration(
              labelText: t('ai.name'),
              // The name it will be given if this is left alone.
              hintText: _suggestedName.isEmpty
                  ? t('ai.namePlaceholder', {'provider': _defaults?.label ?? _provider})
                  : _suggestedName,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 12),
          DropdownButtonFormField<String>(
            initialValue: _provider,
            decoration: InputDecoration(
              labelText: t('ai.providerLabel'),
              border: const OutlineInputBorder(),
            ),
            items: [
              for (final provider in names)
                DropdownMenuItem(value: provider.name, child: Text(provider.label)),
            ],
            onChanged: (value) => value == null ? null : _pickProvider(value),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _baseUrl,
            keyboardType: TextInputType.url,
            decoration: InputDecoration(
              labelText: t('ai.baseUrl'),
              helperText: t('ai.baseUrlHelp'),
              helperMaxLines: 3,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 12),
          // Suggestions, not a list of permitted values: a provider ships a
          // model the day before the catalogue hears about it, and the API
          // takes whatever is typed.
          RawAutocomplete<String>(
            textEditingController: _modelField,
            focusNode: _modelFocus,
            optionsBuilder: (value) {
              final needle = value.text.trim().toLowerCase();
              final models = _defaults?.models ?? const <String>[];
              if (needle.isEmpty) return models;
              return models.where((model) => model.toLowerCase().contains(needle));
            },
            onSelected: (model) => setState(() => _modelField.text = model),
            fieldViewBuilder: (context, controller, focusNode, onSubmitted) => TextField(
              controller: controller,
              focusNode: focusNode,
              onSubmitted: (_) => onSubmitted(),
              decoration: InputDecoration(
                labelText: t('ai.model'),
                hintText: _defaults?.defaultModel,
                helperText: t('ai.modelHelp'),
                helperMaxLines: 3,
                border: const OutlineInputBorder(),
              ),
            ),
            optionsViewBuilder: (context, onSelected, options) => Align(
              alignment: Alignment.topLeft,
              child: Material(
                elevation: 4,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxHeight: 220, maxWidth: 320),
                  child: ListView(
                    padding: EdgeInsets.zero,
                    shrinkWrap: true,
                    children: [
                      for (final option in options)
                        ListTile(
                          dense: true,
                          title: Text(option),
                          onTap: () => onSelected(option),
                        ),
                    ],
                  ),
                ),
              ),
            ),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _maxTokens,
            keyboardType: TextInputType.number,
            decoration: InputDecoration(
              labelText: t('ai.maxTokens'),
              hintText: '4096',
              helperText: t('ai.maxTokensHelp'),
              helperMaxLines: 3,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _token,
            obscureText: true,
            decoration: InputDecoration(
              labelText: t('ai.token'),
              hintText: connection?.hasToken == true
                  ? t('ai.tokenSet')
                  : (_defaults?.keyHint.isNotEmpty == true ? _defaults!.keyHint : null),
              helperText: connection?.hasToken == true ? t('ai.tokenKeep') : t('ai.tokenHelp'),
              helperMaxLines: 3,
              border: const OutlineInputBorder(),
            ),
          ),
          if (_error.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Text(_error, style: TextStyle(color: AppColors.of(context).danger)),
            ),
          const SizedBox(height: 16),
          Row(children: [
            Expanded(
              child: FilledButton(
                onPressed: _busy ? null : _save,
                child: Text(t('common.save')),
              ),
            ),
            const SizedBox(width: 8),
            TextButton(
              onPressed: () => Navigator.of(context).pop(false),
              child: Text(t('common.cancel')),
            ),
          ]),
        ]),
      ),
    );
  }
}

/// Who else may put questions through this connection.
class _SharesSheet extends ConsumerStatefulWidget {
  final String connectionId;

  const _SharesSheet({required this.connectionId});

  @override
  ConsumerState<_SharesSheet> createState() => _SharesSheetState();
}

class _SharesSheetState extends ConsumerState<_SharesSheet> {
  List<AccessGrant> _grants = const [];
  List<MemberResult> _members = const [];
  List<String> _picked = const [];
  bool _busy = false;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final api = ref.read(apiProvider);
      final grants = await api.aiShares(widget.connectionId);
      final page = await api.searchMembers();
      if (mounted) {
        setState(() {
          _grants = grants;
          _members = page.results;
        });
      }
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    }
  }

  Future<void> _share() async {
    setState(() {
      _busy = true;
      _error = '';
    });
    try {
      // One request per person: granting is rare, and a partial failure then
      // names who it failed for.
      for (final userId in _picked) {
        _grants = await ref.read(apiProvider).shareAiConnection(widget.connectionId, userId);
      }
      if (mounted) setState(() => _picked = const []);
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _revoke(String userId) async {
    setState(() => _busy = true);
    try {
      await ref.read(apiProvider).unshareAiConnection(widget.connectionId, userId);
      await _load();
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final shared = _grants.where((grant) => grant.role != 'owner').toList();
    final granted = {for (final grant in _grants) grant.userId};

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(t('ai.shares'), style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(t('ai.sharesHelp'), style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 8),
          if (shared.isEmpty)
            Text(t('ai.noShares'), style: Theme.of(context).textTheme.bodySmall),
          for (final grant in shared)
            ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(grant.name),
              subtitle: grant.handle.isEmpty ? null : Text('@${grant.handle}'),
              trailing: IconButton(
                onPressed: _busy ? null : () => _revoke(grant.userId),
                tooltip: t('ai.unshare'),
                icon: const Icon(Icons.delete_outline, size: 20),
              ),
            ),
          const SizedBox(height: 8),
          SfMultiSelect(
            label: t('ai.shareWith'),
            placeholder: t('ai.pickMembers'),
            options: [
              for (final member in _members.where((member) => !granted.contains(member.id)))
                SelectOption(
                  value: member.id,
                  label: '${member.name} ${member.surname}'.trim().isEmpty
                      ? member.handle
                      : '${member.name} ${member.surname}'.trim(),
                ),
            ],
            selected: _picked,
            onChanged: (values) => setState(() => _picked = values),
            searchHint: t('common.search'),
            noResults: t('common.noResults'),
            selectedCount: t('common.selectedCount', {'count': '{{count}}'}),
            clearLabel: t('common.clearSelection'),
          ),
          if (_error.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(_error, style: TextStyle(color: AppColors.of(context).danger)),
            ),
          const SizedBox(height: 12),
          FilledButton(
            onPressed: _busy || _picked.isEmpty ? null : _share,
            child: Text(t('ai.shareAction')),
          ),
        ]),
      ),
    );
  }
}

/// A provider's display name, from the catalogue the server serves. Falls
/// back to the stored name for a connection whose provider has since been
/// taken out of the table: it still works, and showing "deepseek" beats
/// showing nothing.
String providerLabel(List<AiProvider> providers, String name) {
  for (final provider in providers) {
    if (provider.name == name) return provider.label;
  }
  return name;
}

/// Eight random characters, the tail of a connection's suggested name. Not an
/// id and not a secret: it exists so the same model twice over reads as two
/// different connections in a menu.
String _randomSuffix() {
  const alphabet = '0123456789abcdef';
  final random = Random();
  return List.generate(8, (_) => alphabet[random.nextInt(alphabet.length)]).join();
}
