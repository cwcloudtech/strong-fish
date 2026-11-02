import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/models.dart';
import '../providers/providers.dart';

/// The button that asks the model, and - when the member has more than one
/// subscription - which one to spend.
///
/// One connection is the common case and must stay one tap: somebody with a
/// single key should never be made to pick it. Two or more is a coach's key
/// beside their own, and then *which one pays* is a real question, so the
/// button becomes a menu of them. The API applies the same rule from the
/// other side: no connection named means the member's default.
class AiAskButton extends ConsumerWidget {
  final List<AiConnection> connections;
  final void Function(String connectionId) onAsk;
  final bool busy;
  final bool disabled;
  final String label;
  final String? tooltip;

  const AiAskButton({
    super.key,
    required this.connections,
    required this.onAsk,
    required this.label,
    this.busy = false,
    this.disabled = false,
    this.tooltip,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final icon = busy
        ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
        : const Icon(Icons.auto_awesome_outlined, size: 18);
    final enabled = !busy && !disabled;

    if (connections.length > 1) {
      return PopupMenuButton<String>(
        enabled: enabled,
        tooltip: tooltip ?? t('ai.askWith'),
        onSelected: onAsk,
        itemBuilder: (context) => [
          PopupMenuItem(enabled: false, child: Text(t('ai.askWith'))),
          for (final connection in connections)
            PopupMenuItem(
              value: connection.id,
              child: Text(aiConnectionLabel(connection)),
            ),
        ],
        child: AbsorbPointer(
          child: FilledButton.tonalIcon(
            onPressed: enabled ? () {} : null,
            icon: icon,
            label: Text(label),
          ),
        ),
      );
    }

    return Tooltip(
      message: tooltip ?? '',
      child: FilledButton.tonalIcon(
        onPressed: enabled ? () => onAsk(connections.isEmpty ? '' : connections.first.id) : null,
        icon: icon,
        label: Text(label),
      ),
    );
  }
}

/// One connection in a line: what the member called it, or its provider, and
/// whose it is when it is not theirs - "Claude (Marie)" is the only way to
/// tell a lent key from your own in a menu.
String aiConnectionLabel(AiConnection connection) {
  final name = connection.name.isNotEmpty ? connection.name : connection.provider;
  if (connection.role.isNotEmpty && connection.role != 'owner' && connection.ownerName.isNotEmpty) {
    return '$name (${connection.ownerName})';
  }
  return name;
}
