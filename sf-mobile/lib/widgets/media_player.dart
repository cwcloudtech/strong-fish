import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../providers/providers.dart';
import '../theme.dart';

/// The provider a post's link resolves to.
///
/// A direct port of sf-ui's `webcomponents/MediaPlayer.js` detectMedia, host by
/// host, so a post looks the same on a phone as it does in a browser. The two
/// have to agree: it is the same link, and a reader switching devices should
/// not find one of them showing a player and the other a bare URL.
enum MediaKind { youtube, vimeo, dailymotion, facebook, drive, file, audio, image, link }

class DetectedMedia {
  final MediaKind kind;

  /// For an embed: the iframe src. For an image or an uploaded file: its own
  /// URL. For a plain link: the original, unchanged.
  final String embedUrl;

  const DetectedMedia(this.kind, this.embedUrl);
}

/// The kinds that play inside a provider's own iframe.
const Set<MediaKind> _framedKinds = {
  MediaKind.youtube,
  MediaKind.vimeo,
  MediaKind.dailymotion,
  MediaKind.facebook,
  MediaKind.drive,
};

final RegExp _imagePattern =
    RegExp(r'\.(png|jpe?g|gif|webp|svg|bmp|avif)(\?.*)?$', caseSensitive: false);

/// A video a browser can play from a plain URL - which is what an upload to the
/// member's own bucket ends up as.
final RegExp _videoPattern =
    RegExp(r'\.(mp4|webm|ogv|ogg|mov|m4v)(\?.*)?$', caseSensitive: false);

/// A sound file, played through the platform's own controls for the same
/// reason. Only the extensions that can be nothing else are listed: .webm and
/// .ogg are containers for either, and a recording in one of them is told
/// apart by its key in [_isVoiceObject] rather than guessed at from its name.
final RegExp _audioPattern =
    RegExp(r'\.(mp3|m4a|aac|wav|oga|opus|flac|weba)(\?.*)?$', caseSensitive: false);

/// Whether this is a voice recording served by our own media proxy.
///
/// The API names what it stores - `.../{userId}/voice/clip-1a2b.webm` for a
/// recording, `.../video/...` for a film - and the object segment of a media
/// URL is that key in base64url. Reading it is what tells an audio-only WebM
/// from a video one; the extension cannot, and guessing wrong plays a voice
/// message inside a black rectangle.
bool _isVoiceObject(Uri uri) {
  final parts = uri.pathSegments.where((segment) => segment.isNotEmpty).toList();
  final media = parts.indexOf('media');
  if (media < 1 || parts[media - 1] != 'v1' || parts.length < media + 3) return false;

  final object = parts[media + 2];
  final dot = object.lastIndexOf('.');
  final encoded = dot > 0 ? object.substring(0, dot) : object;
  try {
    final key = utf8.decode(base64Url.decode(base64Url.normalize(encoded)));
    return key.split('/').contains('voice');
  } catch (_) {
    return false;
  }
}

DetectedMedia detectMedia(String rawUrl) {
  final uri = Uri.tryParse(rawUrl);
  if (uri == null || !uri.hasScheme || uri.host.isEmpty) {
    return DetectedMedia(MediaKind.link, rawUrl);
  }

  final host = uri.host.toLowerCase().replaceFirst(RegExp(r'^www\.'), '');
  final parts = uri.pathSegments.where((segment) => segment.isNotEmpty).toList();
  final last = parts.isNotEmpty ? parts.last : null;

  if (host == 'youtube.com' || host == 'm.youtube.com') {
    final query = uri.queryParameters['v'];
    final id = (query != null && query.isNotEmpty) ? query : last;
    if (id != null && id.isNotEmpty) {
      return DetectedMedia(MediaKind.youtube, 'https://www.youtube.com/embed/$id');
    }
  }
  if (host == 'youtu.be' && last != null) {
    return DetectedMedia(MediaKind.youtube, 'https://www.youtube.com/embed/$last');
  }

  if ((host == 'vimeo.com' || host == 'player.vimeo.com') &&
      last != null &&
      RegExp(r'^\d+$').hasMatch(last)) {
    return DetectedMedia(MediaKind.vimeo, 'https://player.vimeo.com/video/$last');
  }

  if (host == 'dailymotion.com') {
    final index = parts.indexOf('video');
    final id = (index >= 0 && index + 1 < parts.length) ? parts[index + 1] : last;
    if (id != null && id.isNotEmpty) {
      return DetectedMedia(MediaKind.dailymotion, 'https://www.dailymotion.com/embed/video/$id');
    }
  }
  if (host == 'dai.ly' && last != null) {
    return DetectedMedia(MediaKind.dailymotion, 'https://www.dailymotion.com/embed/video/$last');
  }

  if (host == 'facebook.com' || host == 'fb.watch') {
    return DetectedMedia(
      MediaKind.facebook,
      'https://www.facebook.com/plugins/video.php'
          '?href=${Uri.encodeComponent(rawUrl)}&show_text=false',
    );
  }

  // A Drive file shared by link. Its direct-download URL serves an interstitial
  // for anything but a small file, which no player can get past, so the
  // /preview player is what gets framed.
  if (host == 'drive.google.com') {
    final index = parts.indexOf('d');
    final id = index >= 0 && index + 1 < parts.length ? parts[index + 1] : uri.queryParameters['id'];
    if (id != null && id.isNotEmpty) {
      return DetectedMedia(MediaKind.drive, 'https://drive.google.com/file/d/$id/preview');
    }
  }

  if (_audioPattern.hasMatch(uri.path) || _isVoiceObject(uri)) {
    return DetectedMedia(MediaKind.audio, rawUrl);
  }
  if (_videoPattern.hasMatch(uri.path)) return DetectedMedia(MediaKind.file, rawUrl);
  if (_imagePattern.hasMatch(uri.path)) return DetectedMedia(MediaKind.image, rawUrl);

  return DetectedMedia(MediaKind.link, rawUrl);
}

String _providerLabel(MediaKind kind) {
  switch (kind) {
    case MediaKind.youtube:
      return 'YouTube';
    case MediaKind.vimeo:
      return 'Vimeo';
    case MediaKind.dailymotion:
      return 'Dailymotion';
    case MediaKind.facebook:
      return 'Facebook';
    case MediaKind.drive:
      return 'Google Drive';
    default:
      return '';
  }
}

/// A minimal page embedding the provider's iframe, with the same markup and
/// `allow` attributes the web component uses.
///
/// The embed is loaded as an iframe inside a document with a real https base
/// URL, rather than as the top-level page. That is what YouTube's player needs:
/// opened bare, with no embedding origin, it refuses with a configuration
/// error.
/// A URL as an HTML attribute value.
///
/// Signed media links carry a query string, so the ampersands and quotes that
/// would otherwise end the attribute early have to go in escaped.
String _attribute(String url) => url
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');

String _frameHtml(String embedUrl) =>
    '<!DOCTYPE html><html><head>'
    '<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1">'
    '<style>html,body{margin:0;padding:0;height:100%;background:#000;overflow:hidden}'
    '.frame{position:relative;width:100%;height:100%}'
    'iframe{position:absolute;inset:0;width:100%;height:100%;border:0}</style></head>'
    '<body><div class="frame"><iframe src="${_attribute(embedUrl)}" '
    'allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" '
    'allowfullscreen></iframe></div></body></html>';

/// Whether this URL is an object served by a StrongFish media proxy -
/// `.../v1/media/{storage}/{object}`.
///
/// Recognised by the shape of the path, not by comparing the whole address
/// against the configured API URL. The server builds these links from its own
/// `SF_API_URL`; the phone was told an address by hand or by a scanned QR
/// code, and the two only have to reach the same server, not be spelled the
/// same way. A `https` against a `http`, a `www.`, an explicit `:443` or a
/// trailing slash was enough to make a phone treat its own deployment's video
/// as a stranger's link and hand the player a URL that answers 401 - which is
/// why an upload played in a browser and showed nothing on a phone.
///
/// Everything else - a public bucket's object, a Drive /preview page, a
/// YouTube video - goes to the player untouched, as it always has.
bool isPrivateMedia(String url) {
  final uri = Uri.tryParse(url);
  if (uri == null || !uri.hasScheme || uri.host.isEmpty) return false;
  final parts = uri.pathSegments.where((segment) => segment.isNotEmpty).toList();
  final media = parts.indexOf('media');
  // `v1/media/{storage}/{object}`: the storage id and the object have to be
  // there, or this is some other endpoint that merely contains the word.
  return media > 0 && parts[media - 1] == 'v1' && parts.length >= media + 3;
}

/// Whether [url] is served by the very API this phone is talking to, compared
/// host to host so the spellings above don't matter.
bool isOwnApiHost(String url, String apiUrl) {
  String hostOf(String value) {
    final uri = Uri.tryParse(value.contains('://') ? value : 'https://$value');
    return uri?.host.toLowerCase().replaceFirst(RegExp(r'^www\.'), '') ?? '';
  }

  final own = hostOf(apiUrl);
  return own.isNotEmpty && hostOf(url) == own;
}

/// Renders a post's link the way the web `<media-player>` does: a 16:9 embed for
/// the hosted providers, a native player for an uploaded video, the platform's
/// audio controls for a sound file, a picture for an image, and a link card for
/// anything else.
///
/// Nothing loads until it is tapped. A feed is a list, and spinning up a WebView
/// per post would cost memory and battery for players nobody asked to watch.
class MediaPlayer extends ConsumerStatefulWidget {
  final String url;

  /// Where the framed document claims to be served from - the app's own
  /// frontend, so the embed has an origin to report.
  final String baseUrl;

  /// Play this as sound even when the name says nothing.
  ///
  /// Set by the caller that already knows - a voice message carries its
  /// recording in its own field, so there is nothing to detect. It matters for
  /// a bucket that serves its files publicly: the address is then the member's
  /// own, `.webm` and `.ogg` are containers for either kind, and without this
  /// a voice note would be drawn as a video with nothing to show.
  final bool preferAudio;

  const MediaPlayer({
    super.key,
    required this.url,
    required this.baseUrl,
    this.preferAudio = false,
  });

  @override
  ConsumerState<MediaPlayer> createState() => _MediaPlayerState();
}

class _MediaPlayerState extends ConsumerState<MediaPlayer> {
  WebViewController? _controller;

  /// The address to hand the player: the post's own, or - for an object in a
  /// private bucket - the signed one the API gives back. Null while that
  /// exchange is in flight, empty when it was refused.
  String? _resolved;

  @override
  void initState() {
    super.initState();
    _resolve();
  }

  @override
  void didUpdateWidget(MediaPlayer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.url != widget.url) _resolve();
  }

  /// The caller's hint, honoured for anything that is not somebody else's
  /// player: a YouTube or Drive link stays framed, since there is no file to
  /// hand to an audio element in the first place.
  bool _preferAudio(MediaKind kind) =>
      widget.preferAudio && !_framedKinds.contains(kind) && kind != MediaKind.image;

  Future<void> _resolve() async {
    final api = ref.read(apiProvider);
    if (!isPrivateMedia(widget.url)) {
      setState(() => _resolved = widget.url);
      return;
    }

    setState(() => _resolved = null);
    try {
      final signed = await api.mediaLink(widget.url);
      if (mounted) setState(() => _resolved = signed.isEmpty ? widget.url : signed);
    } catch (_) {
      // A file this member may not read is not an error to shout about: the
      // post shows nothing where the video would be, which is what somebody
      // else's private video should look like.
      //
      // A media link from *another* instance is a different matter - this API
      // has no such object and never will - so that one falls back to the URL
      // as posted, which still plays if it points at something public.
      if (!mounted) return;
      final ours = isOwnApiHost(widget.url, api.client.apiUrl);
      setState(() => _resolved = ours ? '' : widget.url);
    }
  }

  void _play(String embedUrl) {
    setState(() {
      _controller = WebViewController()
        ..setJavaScriptMode(JavaScriptMode.unrestricted)
        ..setBackgroundColor(Colors.black)
        ..loadHtmlString(_frameHtml(embedUrl), baseUrl: widget.baseUrl);
    });
  }

  @override
  Widget build(BuildContext context) {
    final url = _resolved;
    if (url == null) {
      return const AspectRatio(
        aspectRatio: 16 / 9,
        child: Center(child: CircularProgressIndicator()),
      );
    }
    if (url.isEmpty) return const SizedBox.shrink();

    final media = detectMedia(url);

    if (_framedKinds.contains(media.kind)) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(10),
        child: AspectRatio(
          aspectRatio: 16 / 9,
          child: _controller != null
              ? WebViewWidget(controller: _controller!)
              : _Poster(label: _providerLabel(media.kind), onTap: () => _play(media.embedUrl)),
        ),
      );
    }

    // An uploaded file has no provider page to frame, so it plays in a bare
    // document with a <video> tag - the phone's own controls, no iframe.
    if (media.kind == MediaKind.file) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(10),
        child: AspectRatio(
          aspectRatio: 16 / 9,
          child: _controller != null
              ? WebViewWidget(controller: _controller!)
              : _Poster(label: '', onTap: () => _playFile(media.embedUrl)),
        ),
      );
    }

    // Audio needs no poster and no 16:9 box: the controls are the whole of it,
    // and at metadata-only preload nothing is fetched until it is played.
    if (media.kind == MediaKind.audio || _preferAudio(media.kind)) {
      return SizedBox(height: 56, child: AudioWebView(url: media.embedUrl));
    }

    if (media.kind == MediaKind.image) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(10),
        child: Image.network(
          media.embedUrl,
          fit: BoxFit.cover,
          width: double.infinity,
          errorBuilder: (_, _, _) => const SizedBox.shrink(),
        ),
      );
    }

    return _LinkCard(url: url);
  }

  void _playFile(String url) {
    const style = 'html,body{margin:0;height:100%;background:#000}'
        'video{width:100%;height:100%;object-fit:contain}';
    setState(() {
      _controller = WebViewController()
        ..setJavaScriptMode(JavaScriptMode.unrestricted)
        ..setBackgroundColor(Colors.black)
        ..loadHtmlString(
          '<!DOCTYPE html><html><head>'
          '<meta name="viewport" content="width=device-width, initial-scale=1">'
          '<style>$style</style></head><body>'
          '<video src="$url" controls autoplay playsinline></video></body></html>',
          baseUrl: widget.baseUrl,
        );
    });
  }
}

/// What stands in for the player until it is asked for.
class _Poster extends StatelessWidget {
  final String label;
  final VoidCallback onTap;

  const _Poster({required this.label, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.black,
      child: InkWell(
        onTap: onTap,
        child: Stack(
          fit: StackFit.expand,
          children: [
            Center(
              child: Container(
                width: 56,
                height: 56,
                decoration: const BoxDecoration(color: Colors.black54, shape: BoxShape.circle),
                child: const Icon(Icons.play_arrow, color: Colors.white, size: 36),
              ),
            ),
            if (label.isNotEmpty)
              Positioned(
                left: 10,
                bottom: 8,
                child: Text(
                  label,
                  style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w600),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// Anything with no player of its own: shown as a card that opens the link.
class _LinkCard extends StatelessWidget {
  final String url;

  const _LinkCard({required this.url});

  @override
  Widget build(BuildContext context) {
    final colors = AppColors.of(context);
    return Material(
      color: colors.backgroundMuted,
      borderRadius: BorderRadius.circular(10),
      child: InkWell(
        borderRadius: BorderRadius.circular(10),
        onTap: () => Navigator.of(context).push(
          MaterialPageRoute(builder: (_) => WebViewPage(url: url)),
        ),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          child: Row(
            children: [
              Icon(Icons.link, size: 18, color: colors.textMuted),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  url,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: colors.primary, fontSize: 13),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The in-app browser a link card opens, so following a link does not throw the
/// reader out of the app.
class WebViewPage extends StatefulWidget {
  final String url;

  const WebViewPage({super.key, required this.url});

  @override
  State<WebViewPage> createState() => _WebViewPageState();
}

class _WebViewPageState extends State<WebViewPage> {
  late final WebViewController _controller = WebViewController()
    ..setJavaScriptMode(JavaScriptMode.unrestricted)
    ..loadRequest(Uri.parse(widget.url));

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(Uri.tryParse(widget.url)?.host ?? widget.url, overflow: TextOverflow.ellipsis),
      ),
      body: WebViewWidget(controller: _controller),
    );
  }
}

/// The platform's own audio controls, in a minimal page.
///
/// Flutter has no audio element, and a plugin for play/pause alone would be a
/// dependency to carry forever - the WebView is already here for video.
class AudioWebView extends StatefulWidget {
  final String url;

  const AudioWebView({super.key, required this.url});

  @override
  State<AudioWebView> createState() => _AudioWebViewState();
}

class _AudioWebViewState extends State<AudioWebView> {
  late final WebViewController _controller = WebViewController()
    ..setJavaScriptMode(JavaScriptMode.unrestricted)
    ..setBackgroundColor(Colors.transparent)
    ..loadHtmlString(
      '<!DOCTYPE html><html><head>'
      '<meta name="viewport" content="width=device-width, initial-scale=1">'
      '<style>html,body{margin:0;background:transparent}'
      'audio{width:100%;height:44px}</style></head>'
      '<body><audio src="${_attribute(widget.url)}" controls preload="metadata"></audio></body></html>',
    );

  @override
  Widget build(BuildContext context) => WebViewWidget(controller: _controller);
}
