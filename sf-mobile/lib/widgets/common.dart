import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';

import '../models/models.dart';

/// A user's avatar, falling back to their initials. Pictures arrive as base64
/// data URIs (the API stores them inline), so they're decoded here rather than
/// fetched.
class SfAvatar extends StatelessWidget {
  final String picture;
  final String name;
  final double radius;

  const SfAvatar({super.key, required this.picture, required this.name, this.radius = 20});

  factory SfAvatar.of(User user, {double radius = 20}) =>
      SfAvatar(picture: user.picture, name: user.fullName, radius: radius);

  @override
  Widget build(BuildContext context) {
    final bytes = _decode(picture);
    final scheme = Theme.of(context).colorScheme;

    return CircleAvatar(
      radius: radius,
      backgroundColor: scheme.primaryContainer,
      backgroundImage: bytes != null ? MemoryImage(bytes) : null,
      child: bytes == null
          ? Text(
              _initials(name),
              style: TextStyle(
                color: scheme.onPrimaryContainer,
                fontWeight: FontWeight.bold,
                fontSize: radius * 0.7,
              ),
            )
          : null,
    );
  }
}

/// Decodes a `data:image/...;base64,...` URI. A malformed picture yields null so
/// the avatar falls back to initials rather than throwing during a build.
Uint8List? _decode(String picture) {
  if (picture.isEmpty) return null;
  final comma = picture.indexOf(',');
  final payload = comma >= 0 ? picture.substring(comma + 1) : picture;
  try {
    return base64Decode(payload);
  } catch (_) {
    return null;
  }
}

String _initials(String name) {
  final parts = name.trim().split(RegExp(r'\s+')).where((part) => part.isNotEmpty).take(2);
  if (parts.isEmpty) return '?';
  return parts.map((part) => part[0].toUpperCase()).join();
}

/// The "nothing here" state, with an optional call to action.
class SfEmptyState extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? message;
  final Widget? action;

  const SfEmptyState({super.key, this.icon = Icons.inbox_outlined, required this.title, this.message, this.action});

  @override
  Widget build(BuildContext context) {
    final muted = Theme.of(context).textTheme.bodyMedium?.color?.withValues(alpha: 0.65);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 44, color: muted),
            const SizedBox(height: 12),
            Text(title, style: Theme.of(context).textTheme.titleMedium, textAlign: TextAlign.center),
            if (message != null) ...[
              const SizedBox(height: 6),
              Text(message!, style: TextStyle(color: muted), textAlign: TextAlign.center),
            ],
            if (action != null) ...[const SizedBox(height: 16), action!],
          ],
        ),
      ),
    );
  }
}

/// Renders a failed load with a retry, so a transient network error doesn't
/// leave the screen permanently blank.
class SfErrorState extends StatelessWidget {
  final String message;
  final VoidCallback? onRetry;
  final String retryLabel;

  const SfErrorState({super.key, required this.message, this.onRetry, this.retryLabel = 'Retry'});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 44, color: Theme.of(context).colorScheme.error),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center),
            if (onRetry != null) ...[
              const SizedBox(height: 16),
              FilledButton(onPressed: onRetry, child: Text(retryLabel)),
            ],
          ],
        ),
      ),
    );
  }
}
