import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';
import 'multi_select.dart';

/// The member's video storage, and who else may use it.
///
/// The same card as the AI connections, because it is the same arrangement:
/// an account holds a credential, decides who else may spend it, and puts
/// them in an order that means something - an upload goes to every target,
/// and the link in the post comes from the first.
///
/// StrongFish keeps no video of its own, so this is what makes posting one
/// possible at all; until now it could only be set up on the web.
class StorageConnectionsSection extends ConsumerWidget {
  const StorageConnectionsSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final storages = ref.watch(storagesProvider);

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(t('storage.title'), style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 4),
          storages.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(12),
              child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
            ),
            error: (error, _) => Text(ref.read(tErrorProvider)(error),
                style: TextStyle(color: AppColors.of(context).danger)),
            data: (page) {
              final list = page.storages;
              final megabytes = (page.maxSize / (1024 * 1024)).round();
              return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(t('storage.subtitle', {'size': '$megabytes'}),
                    style: Theme.of(context).textTheme.bodySmall),
                const SizedBox(height: 8),
                if (list.isEmpty)
                  Text(t('storage.noTargets'), style: Theme.of(context).textTheme.bodySmall),
                if (list.length > 1)
                  Text(t('storage.orderHelp'), style: Theme.of(context).textTheme.bodySmall),
                for (final storage in list)
                  _StorageTile(
                    storage: storage,
                    position: list.indexOf(storage),
                    of: list.length,
                    onMove: (by) => _move(context, ref, list, storage.id, by),
                  ),
                const SizedBox(height: 8),
                Align(
                  alignment: AlignmentDirectional.centerStart,
                  child: FilledButton.icon(
                    onPressed: () => _edit(context, ref, null),
                    icon: const Icon(Icons.add),
                    label: Text(t('storage.addTarget')),
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
Future<void> _move(
    BuildContext context, WidgetRef ref, List<Storage> list, String storageId, int by) async {
  final ids = [for (final storage in list) storage.id];
  final from = ids.indexOf(storageId);
  final to = from + by;
  if (from < 0 || to < 0 || to >= ids.length) return;

  final messenger = ScaffoldMessenger.of(context);
  ids[from] = ids[to];
  ids[to] = storageId;
  try {
    await ref.read(apiProvider).reorderStorages(ids);
    ref.invalidate(storagesProvider);
  } catch (error) {
    messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
  }
}

/// Opens the form for a new target or an existing one, and reloads the list
/// when something was saved.
Future<void> _edit(BuildContext context, WidgetRef ref, Storage? storage) async {
  final saved = await showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    builder: (context) => Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: _StorageForm(storage: storage),
    ),
  );
  if (saved == true) ref.invalidate(storagesProvider);
}

class _StorageTile extends ConsumerWidget {
  final Storage storage;
  final int position;
  final int of;
  final void Function(int by) onMove;

  const _StorageTile({
    required this.storage,
    required this.position,
    required this.of,
    required this.onMove,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final kind = storage.connection.isDrive ? t('storage.typeDrive') : t('storage.typeS3');

    return ListTile(
      contentPadding: EdgeInsets.zero,
      title: Text(kind),
      subtitle: Text([
        storage.connection.summary,
        if (position == 0) t('storage.first'),
      ].where((part) => part.isNotEmpty).join(' · ')),
      trailing: Row(mainAxisSize: MainAxisSize.min, children: [
        IconButton(
          onPressed: position <= 0 ? null : () => onMove(-1),
          tooltip: t('storage.moveUp'),
          icon: const Icon(Icons.arrow_upward, size: 20),
          visualDensity: VisualDensity.compact,
        ),
        IconButton(
          onPressed: position >= of - 1 ? null : () => onMove(1),
          tooltip: t('storage.moveDown'),
          icon: const Icon(Icons.arrow_downward, size: 20),
          visualDensity: VisualDensity.compact,
        ),
        IconButton(
          onPressed: () => showModalBottomSheet<void>(
            context: context,
            isScrollControlled: true,
            builder: (context) => _SharesSheet(storageId: storage.id),
          ),
          tooltip: t('storage.shares'),
          icon: const Icon(Icons.group_outlined, size: 20),
          visualDensity: VisualDensity.compact,
        ),
        IconButton(
          onPressed: () => _edit(context, ref, storage),
          tooltip: t('common.edit'),
          icon: const Icon(Icons.edit_outlined, size: 20),
          visualDensity: VisualDensity.compact,
        ),
        IconButton(
          onPressed: () => _delete(context, ref),
          tooltip: t('storage.clear'),
          icon: const Icon(Icons.delete_outline, size: 20),
          visualDensity: VisualDensity.compact,
        ),
      ]),
    );
  }

  /// Removing a target asks first: the videos already in it stay where they
  /// are, but nothing else here is written down - the keys are gone.
  Future<void> _delete(BuildContext context, WidgetRef ref) async {
    final t = ref.read(tProvider);
    final messenger = ScaffoldMessenger.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('storage.clear')),
        content: Text(t('storage.clearConfirm')),
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
      await ref.read(apiProvider).deleteStorage(storage.id);
      ref.invalidate(storagesProvider);
      messenger.showSnackBar(SnackBar(content: Text(t('storage.cleared'))));
    } catch (error) {
      messenger.showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
    }
  }
}

/// The connection form, for a new target or one being edited.
///
/// The same fields either way: what differs between adding and editing is
/// only which request the save makes.
class _StorageForm extends ConsumerStatefulWidget {
  final Storage? storage;

  const _StorageForm({required this.storage});

  @override
  ConsumerState<_StorageForm> createState() => _StorageFormState();
}

class _StorageFormState extends ConsumerState<_StorageForm> {
  final _fields = <String, TextEditingController>{};
  late String _type;
  late bool _private;
  bool _busy = false;
  String _error = '';

  StorageConnection get _connection => widget.storage?.connection ?? const StorageConnection();

  @override
  void initState() {
    super.initState();
    final conn = _connection;
    _type = conn.type;
    _private = conn.private;
    for (final entry in {
      'endpoint': conn.endpoint,
      'region': conn.region,
      'bucketName': conn.bucketName,
      'accessKey': conn.accessKey,
      'publicBaseUrl': conn.publicBaseUrl,
      'folderId': conn.folderId,
      'path': conn.path,
      // The two credentials start empty whatever is stored: the API never
      // sends a secret back, and a field showing its marker would be a field
      // that writes the marker.
      'secretKey': '',
      'serviceAccount': '',
    }.entries) {
      _fields[entry.key] = TextEditingController(text: entry.value);
    }
  }

  @override
  void dispose() {
    for (final field in _fields.values) {
      field.dispose();
    }
    super.dispose();
  }

  String _text(String name) => _fields[name]!.text.trim();

  /// A blank credential keeps the stored one, which is what the API's marker
  /// means. The Drive key is pasted as the JSON Google hands out and encoded
  /// here, so nobody has to run base64 in a terminal first.
  String _secret(String typed, String stored) {
    if (typed.isEmpty) return stored.isEmpty ? '' : secretSet;
    return typed;
  }

  Future<void> _save() async {
    setState(() {
      _busy = true;
      _error = '';
    });

    final pasted = _text('serviceAccount');
    final payload = StorageConnection(
      type: _type,
      endpoint: _text('endpoint'),
      region: _text('region'),
      bucketName: _text('bucketName'),
      accessKey: _text('accessKey'),
      secretKey: _secret(_text('secretKey'), _connection.secretKey),
      serviceAccountBase64: _secret(
        pasted.isEmpty ? '' : base64Encode(utf8.encode(pasted)),
        _connection.serviceAccountBase64,
      ),
      folderId: _text('folderId'),
      path: _text('path'),
      publicBaseUrl: _text('publicBaseUrl'),
      private: _private,
    ).toJson();

    try {
      final api = ref.read(apiProvider);
      final existing = widget.storage;
      if (existing == null) {
        await api.addStorage(payload);
      } else {
        await api.setStorage(existing.id, payload);
      }
      if (mounted) Navigator.of(context).pop(true);
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Widget _field(String name, String label, {String? hint, String? help, bool obscure = false}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: TextField(
        controller: _fields[name],
        obscureText: obscure,
        decoration: InputDecoration(
          labelText: label,
          hintText: hint,
          helperText: help,
          helperMaxLines: 3,
          border: const OutlineInputBorder(),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final conn = _connection;

    return SafeArea(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          DropdownButtonFormField<String>(
            initialValue: _type,
            decoration: InputDecoration(
              labelText: t('storage.type'),
              border: const OutlineInputBorder(),
            ),
            items: [
              DropdownMenuItem(value: 's3', child: Text(t('storage.typeS3'))),
              DropdownMenuItem(value: 'google_drive', child: Text(t('storage.typeDrive'))),
            ],
            onChanged: (value) => setState(() => _type = value ?? 's3'),
          ),
          const SizedBox(height: 12),
          if (_type == 's3') ...[
            _field('endpoint', t('storage.endpoint'), hint: 'https://s3.eu-west-3.amazonaws.com'),
            _field('region', t('storage.region')),
            _field('bucketName', t('storage.bucket')),
            _field('accessKey', t('storage.accessKey')),
            _field('secretKey', t('storage.secretKey'),
                obscure: true,
                hint: conn.secretKey.isEmpty ? null : t('ai.tokenSet'),
                help: conn.secretKey.isEmpty ? null : t('ai.tokenKeep')),
            _field('publicBaseUrl', t('storage.publicBaseUrl'), help: t('storage.publicBaseUrlHelp')),
          ] else ...[
            _field('folderId', t('storage.folderId')),
            // Pasted rather than picked: the phone has no file browser of its
            // own here, and the JSON Google hands out is a paste away. It is
            // encoded on save, so the stored value is the same base64 the web
            // sends.
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: TextField(
                controller: _fields['serviceAccount'],
                minLines: 2,
                maxLines: 5,
                decoration: InputDecoration(
                  labelText: t('storage.serviceAccount'),
                  hintText: conn.serviceAccountBase64.isEmpty
                      ? t('storage.serviceAccountPaste')
                      : t('storage.serviceAccountSet'),
                  helperText: t('storage.serviceAccountHelp'),
                  helperMaxLines: 4,
                  border: const OutlineInputBorder(),
                ),
              ),
            ),
          ],
          _field('path', t('storage.path'),
              help: _type == 'google_drive' ? t('storage.pathHelpDrive') : t('storage.pathHelpS3')),
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            value: _private,
            onChanged: (value) => setState(() => _private = value),
            title: Text(t('storage.private')),
            subtitle: Text(t('storage.privateHelp'),
                style: Theme.of(context).textTheme.bodySmall),
          ),
          if (_error.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Text(_error, style: TextStyle(color: AppColors.of(context).danger)),
            ),
          const SizedBox(height: 12),
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

/// Who else may use this target, and what they may do with it.
class _SharesSheet extends ConsumerStatefulWidget {
  final String storageId;

  const _SharesSheet({required this.storageId});

  @override
  ConsumerState<_SharesSheet> createState() => _SharesSheetState();
}

class _SharesSheetState extends ConsumerState<_SharesSheet> {
  List<AccessGrant> _grants = const [];
  List<MemberResult> _members = const [];
  List<String> _picked = const [];
  String _role = 'reader';
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
      final grants = await api.storageShares(widget.storageId);
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
        _grants = await ref.read(apiProvider).shareStorage(widget.storageId, userId, _role);
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
      await ref.read(apiProvider).unshareStorage(widget.storageId, userId);
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
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(t('storage.shares'), style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 4),
              Text(t('storage.sharesHelp'), style: Theme.of(context).textTheme.bodySmall),
              const SizedBox(height: 8),
              if (shared.isEmpty)
                Text(t('storage.noShares'), style: Theme.of(context).textTheme.bodySmall),
              for (final grant in shared)
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text(grant.name),
                  subtitle: Text([
                    if (grant.handle.isNotEmpty) '@${grant.handle}',
                    t('storage.role.${grant.role}'),
                  ].join(' · ')),
                  trailing: IconButton(
                    onPressed: _busy ? null : () => _revoke(grant.userId),
                    tooltip: t('storage.unshare'),
                    icon: const Icon(Icons.delete_outline, size: 20),
                  ),
                ),
              const SizedBox(height: 8),
              SfMultiSelect(
                label: t('storage.shareWith'),
                placeholder: t('storage.pickMembers'),
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
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                initialValue: _role,
                decoration: InputDecoration(
                  labelText: t('storage.roleLabel'),
                  border: const OutlineInputBorder(),
                  isDense: true,
                ),
                items: [
                  DropdownMenuItem(value: 'reader', child: Text(t('storage.role.reader'))),
                  DropdownMenuItem(value: 'writer', child: Text(t('storage.role.writer'))),
                ],
                onChanged: (value) => setState(() => _role = value ?? 'reader'),
              ),
              if (_error.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Text(_error, style: TextStyle(color: AppColors.of(context).danger)),
                ),
              const SizedBox(height: 12),
              FilledButton(
                onPressed: _busy || _picked.isEmpty ? null : _share,
                child: Text(t('storage.shareAction')),
              ),
            ]),
      ),
    );
  }
}
