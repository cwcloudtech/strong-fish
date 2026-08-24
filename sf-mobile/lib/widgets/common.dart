import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';

import '../models/models.dart';
import '../theme.dart';
import 'media_player.dart';

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
/// Makes a non-scrolling body pullable inside a RefreshIndicator.
///
/// A RefreshIndicator only sees the gesture when its child actually scrolls,
/// so an empty state - a centred icon and two lines - swallows the pull: the
/// one screen somebody most wants to refresh is the one that says there is
/// nothing yet. This gives it a scrollable of viewport height to overscroll.
class SfRefreshableBody extends StatelessWidget {
  final Widget child;

  const SfRefreshableBody({super.key, required this.child});

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) => SingleChildScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        child: ConstrainedBox(
          constraints: BoxConstraints(minHeight: constraints.maxHeight),
          child: child,
        ),
      ),
    );
  }
}

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

/// A picture carried inline as a base64 data URI.
///
/// Promoted out of the feed when private messages started carrying pictures
/// too: the decoding and its failure case are the same in both places, and two
/// copies would only drift.
class SfBase64Image extends StatelessWidget {
  final String data;

  const SfBase64Image({super.key, required this.data});

  @override
  Widget build(BuildContext context) {
    final comma = data.indexOf(',');
    Uint8List bytes;
    try {
      bytes = base64Decode(comma >= 0 ? data.substring(comma + 1) : data);
    } catch (_) {
      // A malformed payload is not worth an error state inside a message
      // bubble; it simply has no picture to show.
      return const SizedBox.shrink();
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular(AppRadius.value),
      child: Image.memory(bytes, fit: BoxFit.cover, width: double.infinity),
    );
  }
}

/// A voice message, played in place.
///
/// Deliberately not a waveform or a scrubber: what a spoken message needs is
/// play, pause, and how long it is. Anything more is a media player nobody
/// asked for in a chat bubble.
class AudioBubble extends StatefulWidget {
  final String url;

  /// Where a framed player claims to be served from - needed only when the
  /// recording lives somewhere that has to be embedded rather than played.
  final String baseUrl;

  const AudioBubble({super.key, required this.url, this.baseUrl = ''});

  @override
  State<AudioBubble> createState() => _AudioBubbleState();
}

class _AudioBubbleState extends State<AudioBubble> {
  bool _open = false;

  @override
  Widget build(BuildContext context) {
    final colors = AppColors.of(context);

    // The platform's own controls, in a minimal page - the same approach the
    // media player takes for an uploaded video, and for the same reason: there
    // is no audio element in Flutter, and a plugin for play/pause alone would
    // be a dependency to carry forever.
    if (_open) {
      // A voice note in Google Drive is not a file to play: the API stores
      // Drive's /preview address, which is an HTML player page. Feeding that
      // to an <audio src> loads a web page and plays nothing - silently, which
      // is exactly how a recording that uploaded perfectly came out mute.
      // Framed through Drive's own player in that case; played natively when
      // the storage serves the file itself, as an S3 bucket does.
      final detected = detectMedia(widget.url);
      if (detected.kind == MediaKind.drive) {
        return SizedBox(
          height: 120,
          child: MediaPlayer(url: widget.url, baseUrl: widget.baseUrl),
        );
      }
      return SizedBox(
        height: 56,
        child: AudioWebView(url: widget.url),
      );
    }

    return InkWell(
      onTap: () => setState(() => _open = true),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.play_circle_outline, color: colors.primary),
          const SizedBox(width: 8),
          Text('🎤', style: TextStyle(color: colors.textMuted)),
        ],
      ),
    );
  }
}
