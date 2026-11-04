import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../api/api_exception.dart';
import '../theme.dart';
import 'ai_ask_button.dart';
import 'export_menu.dart';
import 'long_date.dart';
import 'markdown_text.dart';

/// The windows the API offers, in the order they widen.
const _ranges = ['24h', '48h', 'week', '14d', 'month', '90d'];

/// Ranges wide enough for a daily average to mean anything.
const _averaged = {'week', '14d', 'month', '90d'};

/// How a day's count reads at a glance, on the usual walking scale.
///
/// Applied to a day, never to a sum: ninety days of walking adds up to a
/// number that would sit in the top band whatever the member actually did,
/// which is the opposite of what colouring it is for. The same four bands as
/// the web app, so a member looking at both sees the same colour for the same
/// day.
/// Heart rate zones, the provider's own four bands. Blue is the lowest effort
/// and red the highest, the order every heart rate chart uses - the colours
/// are read before the labels are. Same values as the web app.
const _zoneColors = {
  'light': Color(0xFF3B82F6),
  'moderate': Color(0xFFEAB308),
  'vigorous': Color(0xFFF97316),
  'peak': Color(0xFFDC2626),
};

Color zoneTone(BuildContext context, String zone, {bool known = true}) {
  if (!known) return AppColors.of(context).border;
  return _zoneColors[zone] ?? _zoneColors['light']!;
}

Color stepTone(BuildContext context, int steps) {
  final colors = AppColors.of(context);
  if (steps < 6000) return colors.danger;
  if (steps < 10000) return colors.warning;
  if (steps < 15000) return const Color(0xFF65C37A);
  return colors.success;
}

/// Material 3 sizes a button for one sitting alone on a row. These share a
/// row with a two-line label on a phone, where the default padding is what
/// pushes them off the edge.
final ButtonStyle _compactButton = ButtonStyle(
  padding: WidgetStatePropertyAll(const EdgeInsets.symmetric(horizontal: 12)),
  visualDensity: VisualDensity.compact,
  // The tap target is deliberately left alone. What overflows is horizontal,
  // and shrinking the target would trade a layout problem for a button that
  // is harder to hit - under the 48dp everything else in the app respects.
);

/// A member's connected health accounts, and what they counted.
///
/// Steps are read, never stored: StrongFish is not a step tracker, and a copy
/// of somebody's daily counts would be a second source of truth to keep in
/// step with the watch that actually recorded them.
class DeviceLinksSection extends ConsumerStatefulWidget {
  const DeviceLinksSection({super.key});

  @override
  ConsumerState<DeviceLinksSection> createState() => _DeviceLinksSectionState();
}

class _DeviceLinksSectionState extends ConsumerState<DeviceLinksSection> with WidgetsBindingObserver {
  List<DeviceConnection>? _connections;
  String _busy = '';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _load();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  /// Consent happens in the browser and finishes on the server, so the app
  /// only finds out by asking again.
  ///
  /// Every resume, not just the one after a consent was started: whether the
  /// app is even backgrounded depends on how the browser opened - an in-app
  /// sheet never backgrounds it, and a flag set before leaving is then still
  /// set when nothing happened. The list is one small request; asking for it
  /// whenever the member comes back is cheaper than being wrong about it.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) _load();
  }

  Future<void> _load() async {
    try {
      final connections = await ref.read(apiProvider).deviceConnections();
      if (mounted) setState(() => _connections = connections);
    } catch (_) {
      if (mounted) setState(() => _connections = const []);
    }
  }

  Future<void> _connect(String provider) async {
    setState(() => _busy = provider);
    try {
      final url = await ref.read(apiProvider).deviceAuthorizeUrl(provider);
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = '');
    }
  }

  /// Signs in to an account-paired provider. The password never leaves this
  /// call - only the grant the server returns is stored.
  Future<void> _pair(String provider, String email, String password) async {
    setState(() => _busy = provider);
    try {
      await ref.read(apiProvider).pairDevice(provider, email, password);
      await _load();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = '');
    }
  }

  /// Connects a key-paired provider (Garmin) with the member's personal API
  /// key, which is kept on the server.
  Future<void> _pairKey(String provider, String apiKey) async {
    setState(() => _busy = provider);
    try {
      await ref.read(apiProvider).pairDeviceKey(provider, apiKey);
      await _load();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = '');
    }
  }

  /// Opens the web app to finish connecting with Google, where the browser
  /// runs Google's own sign-in. Native Google sign-in needs a plugin the app
  /// does not bundle, so - like the other providers' consent - this step
  /// happens in the browser.
  Future<void> _connectGoogleWeb() async {
    final messenger = ScaffoldMessenger.of(context);
    final t = ref.read(tProvider);
    final base = ref.read(apiProvider).client.frontendUrl;
    try {
      final opened = await launchUrl(Uri.parse('$base/dashboard/steps'),
          mode: LaunchMode.externalApplication);
      if (!opened && mounted) {
        messenger.showSnackBar(SnackBar(content: Text(t('devices.recordFailed'))));
      }
    } catch (_) {
      if (mounted) messenger.showSnackBar(SnackBar(content: Text(t('devices.recordFailed'))));
    }
  }

  /// Opens the Stance app to record a set. Stance owns the encoder and does
  /// the capture; StrongFish reads the result back afterwards.
  Future<void> _record(String url) async {
    final messenger = ScaffoldMessenger.of(context);
    final t = ref.read(tProvider);
    try {
      final opened = await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
      if (!opened && mounted) {
        messenger.showSnackBar(SnackBar(content: Text(t('devices.recordFailed'))));
      }
    } catch (_) {
      if (mounted) messenger.showSnackBar(SnackBar(content: Text(t('devices.recordFailed'))));
    }
  }

  Future<void> _disconnect(String provider) async {
    setState(() => _busy = provider);
    try {
      await ref.read(apiProvider).disconnectDevice(provider);
      await _load();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _busy = '');
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final connections = _connections;
    // Empty only when this build knows of no integration at all. A provider
    // the operator has not configured still appears, switched off: a blank
    // space leaves a member wondering whether the feature exists.
    if (connections == null || connections.isEmpty) return const SizedBox.shrink();
    String? activitySource;
    for (final connection in connections) {
      if (connection.activitySource) activitySource = connection.provider;
    }

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Expanded(
              child: Text(t('devices.title'), style: Theme.of(context).textTheme.titleMedium),
            ),
            IconButton(
              onPressed: _load,
              tooltip: t('common.refresh'),
              icon: const Icon(Icons.refresh, size: 20),
              visualDensity: VisualDensity.compact,
            ),
          ]),
          Text(t('devices.subtitle'), style: Theme.of(context).textTheme.bodySmall),
          // The data first - the numbers a member comes back to read. Each
          // connected provider shows what it can; the controls that pair and
          // unpair the accounts are gathered at the bottom.
          for (final connection in connections)
            if (connection.connected) ...[
              const SizedBox(height: 12),
              if (connection.activitySource) _StepPanel(provider: connection.provider),
              if (connection.activitySource) _HealthFeedbackPanel(provider: connection.provider),
              if (!connection.activitySource &&
                  activitySource != null &&
                  connection.can('activity'))
                Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Text(
                    t('devices.activityFrom', {'provider': t('devices.provider.$activitySource')}),
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ),
              if (connection.can('body-composition'))
                _BodyCompositionPanel(provider: connection.provider),
              if (connection.can('bar-speed') && connection.recordUrl.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Align(
                    alignment: AlignmentDirectional.centerStart,
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      FilledButton.tonalIcon(
                        style: _compactButton,
                        onPressed: () => _record(connection.recordUrl),
                        icon: const Icon(Icons.fiber_manual_record, size: 18),
                        label: Text(t('devices.record')),
                      ),
                      Padding(
                        padding: const EdgeInsets.only(top: 4),
                        child: Text(t('devices.recordHint'),
                            style: Theme.of(context).textTheme.bodySmall),
                      ),
                    ]),
                  ),
                ),
              if (connection.can('bar-speed')) _BarSpeedPanel(provider: connection.provider),
            ],
          // The accounts last: connecting and disconnecting sit under the data
          // they produce, not above it.
          const SizedBox(height: 18),
          Text(t('devices.accounts'), style: Theme.of(context).textTheme.titleSmall),
          for (final connection in connections) ...[
            const SizedBox(height: 14),
            Row(children: [
              Expanded(
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text(t('devices.provider.${connection.provider}'),
                      style: const TextStyle(fontWeight: FontWeight.w600)),
                  Text(
                    !connection.available
                        ? t('devices.unavailable')
                        : connection.connected
                            ? t('devices.connectedSince', {
                                'date':
                                    connection.connectedAt?.toIso8601String().substring(0, 10) ?? '',
                              })
                            : t('devices.notConnected'),
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ]),
              ),
              if (_busy == connection.provider)
                const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
              else if (connection.connected)
                TextButton.icon(
                  onPressed: () => _disconnect(connection.provider),
                  style: _compactButton,
                  icon: const Icon(Icons.link_off, size: 18),
                  label: Text(t('devices.disconnect')),
                )
              else if (connection.oauth)
                FilledButton.icon(
                  onPressed:
                      connection.available ? () => _connect(connection.provider) : null,
                  style: _compactButton,
                  icon: const Icon(Icons.link, size: 18),
                  label: Text(t('devices.connect')),
                ),
            ]),
            if (!connection.connected &&
                connection.googleClientId.isNotEmpty &&
                connection.available) ...[
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Align(
                  alignment: AlignmentDirectional.centerStart,
                  child: OutlinedButton.icon(
                    onPressed: _connectGoogleWeb,
                    icon: const Icon(Icons.login, size: 18),
                    label: Text(t('devices.connectGoogle')),
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(t('devices.connectGoogleHint'),
                    style: Theme.of(context).textTheme.bodySmall),
              ),
            ],
            if (!connection.connected && connection.accountPaired && connection.available)
              _AccountPairForm(
                provider: connection.provider,
                busy: _busy == connection.provider,
                onPair: (email, password) => _pair(connection.provider, email, password),
              ),
            // Key-paired providers (Garmin): one personal API key field.
            if (!connection.connected && connection.keyPaired && connection.available)
              _KeyPairForm(
                provider: connection.provider,
                busy: _busy == connection.provider,
                onPair: (apiKey) => _pairKey(connection.provider, apiKey),
              ),
          ],
        ]),
      ),
    );
  }
}

/// One provider's step counts, in whichever window the member picks.
class _StepPanel extends ConsumerStatefulWidget {
  final String provider;
  const _StepPanel({required this.provider});

  @override
  ConsumerState<_StepPanel> createState() => _StepPanelState();
}

class _StepPanelState extends ConsumerState<_StepPanel> {
  String _range = 'week';
  StepSeries? _series;
  HeartSeries? _heart;
  String _error = '';
  Object? _errorRaw;
  bool _loading = false;
  bool _exporting = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = '';
    });
    // Apart, deliberately. The provider reaches back ninety days for steps
    // and only fourteen for heart rate, so one of the two is expected to be
    // thinner on the wider views - and a single failed call must never blank
    // the steps the member came for.
    final api = ref.read(apiProvider);
    try {
      final steps = await api.deviceSteps(widget.provider, _range);
      if (mounted) setState(() => _series = steps);
    } catch (error) {
      if (mounted) {
        setState(() {
          _series = null;
          _errorRaw = error;
          _error = ref.read(tErrorProvider)(error);
        });
      }
    }
    try {
      final heart = await api.deviceHeartRate(widget.provider, _range);
      if (mounted) setState(() => _heart = heart);
    } catch (_) {
      if (mounted) setState(() => _heart = null);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  /// Saved to the app's own documents directory rather than a shared one: it
  /// needs no permission, and the file is a copy of something the server can
  /// render again at any time.
  Future<void> _export(ExportFormat format) async {
    setState(() => _exporting = true);
    try {
      final directory = await getApplicationDocumentsDirectory();
      final path = await ref.read(apiProvider).downloadHealth(
            widget.provider,
            directory: directory.path,
            fileName: 'health-$_range.${format.extension}',
            format: format.extension,
            range: _range,
            locale: ref.read(localeProvider),
          );
      if (mounted) await openExported(context, ref, path);
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(ref.read(tErrorProvider)(error))));
      }
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final series = _series;
    // The most recent day the watch actually recorded - "last" rather than
    // "today", because today's reading only exists once it has been synced.
    // Null when there is none: a fallback day would read as a heart rate of
    // zero, which is a worse answer than no answer.
    HeartDay? latest;
    for (final day in _heart?.days ?? const <HeartDay>[]) {
      if (day.known) latest = day;
    }

    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      const SizedBox(height: 10),
      Row(children: [
        Expanded(
          child: SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(children: [
              for (final range in _ranges)
                Padding(
                  padding: const EdgeInsets.only(right: 6),
                  child: ChoiceChip(
                    selected: _range == range,
                    label: Text(t('devices.range.$range')),
                    onSelected: (_) {
                      setState(() => _range = range);
                      _load();
                    },
                  ),
                ),
            ]),
          ),
        ),
        // Exports what is on screen: the range picked beside it, not some
        // other window the member would have to go and set again.
        SfExportButton(
          onExport: _export,
          busy: _exporting,
          tooltip: t('devices.export'),
          formats: const [ExportFormat.png, ExportFormat.pdf],
        ),
      ]),
      if (_loading && series == null)
        const Padding(
          padding: EdgeInsets.all(16),
          child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
        ),
      if (_error.isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(top: 10),
          child: _PanelError(_error, _errorRaw),
        ),
      if (series != null) ...[
        const SizedBox(height: 12),
        // A Wrap, not a Row: "4 093 000" beside "45 477" is wider than a
        // narrow phone, and a Row would overflow rather than fold.
        Wrap(spacing: 24, runSpacing: 8, children: [
          _Figure(
            label: t('devices.total'),
            value: series.total,
            // Coloured only over a single day, where the total is itself a
            // day's count.
            tone: series.days.length == 1 ? stepTone(context, series.total) : null,
          ),
          if (_averaged.contains(series.range))
            _Figure(
              label: t('devices.dailyAverage'),
              value: series.dailyAverage,
              tone: stepTone(context, series.dailyAverage),
            ),
          if (latest != null)
            _Figure(
              label: t('devices.latestBpm'),
              value: latest.avg,
              tone: zoneTone(context, latest.zone),
            ),
        ]),
        const SizedBox(height: 12),
        _StepBars(days: series.days),
        // A shortened range keeps its latest reading - that number is still
        // true - but not a chart that would be captioned with the range it
        // does not cover.
        if (_heart != null && !_heart!.clamped) _HeartSection(heart: _heart!),
        if (_heart?.clamped == true)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Text(t('devices.heartRangeTooWide'),
                style: Theme.of(context).textTheme.bodySmall),
          ),
      ],
    ]);
  }
}

class _Figure extends StatelessWidget {
  final String label;
  final int value;
  final Color? tone;

  const _Figure({required this.label, required this.value, this.tone});

  @override
  Widget build(BuildContext context) {
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text(label, style: Theme.of(context).textTheme.bodySmall),
      Text(
        _grouped(value),
        style: Theme.of(context).textTheme.titleLarge?.copyWith(color: tone),
      ),
    ]);
  }
}

/// A bar per day, drawn with containers rather than a charting package: one
/// series of plain daily numbers does not justify the dependency.
class _StepBars extends StatelessWidget {
  final List<StepDay> days;
  const _StepBars({required this.days});

  @override
  Widget build(BuildContext context) {
    if (days.isEmpty) return const SizedBox.shrink();
    final peak = days.map((day) => day.steps).fold<int>(1, (a, b) => a > b ? a : b);

    return Semantics(
      // The bars are decoration; the counts are what a screen reader is owed.
      label: days.map((day) => '${day.date}: ${day.steps}').join(', '),
      child: Container(
        height: 90,
        padding: const EdgeInsets.all(4),
        decoration: BoxDecoration(
          color: AppColors.of(context).backgroundMuted,
          borderRadius: BorderRadius.circular(6),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            for (final day in days)
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 1),
                  child: Tooltip(
                    message: '${day.date}: ${_grouped(day.steps)}',
                    child: Container(
                      // A day with no steps still draws a sliver, so the row
                      // reads as "nothing that day" rather than as a gap.
                      height: (82 * day.steps / peak).clamp(2, 82).toDouble(),
                      decoration: BoxDecoration(
                        color: stepTone(context, day.steps),
                        borderRadius: const BorderRadius.vertical(top: Radius.circular(2)),
                      ),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// Thousands separated, without pulling in a localisation package for one
/// number.
String _grouped(int value) {
  final digits = value.toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write(' ');
    buffer.write(digits[i]);
  }
  return buffer.toString();
}

/// Heart rate over the same days, each one coloured by the hardest zone it
/// reached.
///
/// The zones are the provider's own four bands, not thresholds worked out here
/// from a birthday: a zone depends on somebody's maximum heart rate, and
/// guessing it would put a confident colour on a number that had not earned
/// it.
class _HeartSection extends ConsumerWidget {
  final HeartSeries heart;
  const _HeartSection({required this.heart});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final known = heart.days.where((day) => day.known).toList();
    if (known.isEmpty) {
      return Padding(
        padding: const EdgeInsets.only(top: 14),
        child: Text(t('devices.noHeartRate'), style: Theme.of(context).textTheme.bodySmall),
      );
    }

    final peak = known
        .map((day) => day.max > 0 ? day.max : day.avg)
        .fold<int>(1, (a, b) => a > b ? a : b);

    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      const SizedBox(height: 16),
      Wrap(spacing: 24, runSpacing: 8, children: [
        _Figure(label: t('devices.averageBpm'), value: heart.averageBpm),
        _Figure(label: t('devices.peakBpm'), value: heart.peakBpm),
      ]),
      const SizedBox(height: 12),
      Container(
        height: 90,
        padding: const EdgeInsets.all(4),
        decoration: BoxDecoration(
          color: AppColors.of(context).backgroundMuted,
          borderRadius: BorderRadius.circular(6),
        ),
        child: Row(crossAxisAlignment: CrossAxisAlignment.end, children: [
          for (final day in heart.days)
            Expanded(
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 1),
                child: Tooltip(
                  message: '${day.date}: ${day.avg} bpm (${t('devices.zone.${day.zone}')})',
                  child: Container(
                    height: (82 * (day.max > 0 ? day.max : day.avg) / peak).clamp(2, 82).toDouble(),
                    decoration: BoxDecoration(
                      color: zoneTone(context, day.zone, known: day.known),
                      borderRadius: const BorderRadius.vertical(top: Radius.circular(2)),
                    ),
                  ),
                ),
              ),
            ),
        ]),
      ),
      // What the colours mean, since four bands explain nothing on their own.
      const SizedBox(height: 6),
      Wrap(spacing: 12, runSpacing: 4, children: [
        for (final zone in const ['light', 'moderate', 'vigorous', 'peak'])
          Row(mainAxisSize: MainAxisSize.min, children: [
            Container(
              width: 10,
              height: 10,
              decoration: BoxDecoration(
                color: zoneTone(context, zone),
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(width: 5),
            Text(t('devices.zone.$zone'), style: Theme.of(context).textTheme.bodySmall),
          ]),
      ]),
    ]);
  }
}

/// The latest reading of every measure the member's scale takes.
///
/// No chart and no range: this is "where am I now". Each measure carries its
/// own date, because a scale does not take every measure every time, and a
/// value from last month should say so.
/// What the AI made of the member's health data.
///
/// Ninety days, once a day: the window is wide because the question is a
/// direction rather than a day, and the wait is a day because that is how
/// often the numbers underneath move. The button says when it comes back
/// rather than refusing when pressed.
class _HealthFeedbackPanel extends ConsumerStatefulWidget {
  final String provider;
  const _HealthFeedbackPanel({required this.provider});

  @override
  ConsumerState<_HealthFeedbackPanel> createState() => _HealthFeedbackPanelState();
}

class _HealthFeedbackPanelState extends ConsumerState<_HealthFeedbackPanel> {
  HealthFeedback? _feedback;
  String _error = '';
  bool _busy = false;
  // The subscriptions this member may spend, so the button can ask which one
  // pays when there is more than one.
  List<AiConnection> _connections = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  /// A 404 is the ordinary case - nobody has asked yet - so it leaves the
  /// panel empty with a button in it rather than reporting a failure.
  Future<void> _load() async {
    try {
      final feedback = await ref.read(apiProvider).healthFeedback(widget.provider);
      if (mounted) setState(() => _feedback = feedback);
    } catch (_) {
      if (mounted) setState(() => _feedback = null);
    }
    try {
      final connections = await ref.read(apiProvider).aiConnections();
      if (mounted) setState(() => _connections = connections);
    } catch (_) {
      // The button still works with none listed: it then asks with the
      // member's default, and the API says so if there isn't one.
    }
  }

  Future<void> _generate(String connectionId) async {
    setState(() {
      _busy = true;
      _error = '';
    });
    try {
      final feedback = await ref
          .read(apiProvider)
          .generateHealthFeedback(widget.provider, connectionId: connectionId);
      if (mounted) setState(() => _feedback = feedback);
    } catch (error) {
      if (!mounted) return;
      // Too soon: the refusal carries the reading it would not replace, so
      // the panel shows that rather than an error about it.
      final body = error is DioException && error.response?.data is Map<String, dynamic>
          ? error.response!.data as Map<String, dynamic>
          : const <String, dynamic>{};
      if (body['i18n_code'] == 'errors.healthFeedbackTooSoon' &&
          body['feedback'] is Map<String, dynamic>) {
        setState(() => _feedback =
            HealthFeedback.fromJson(body['feedback'] as Map<String, dynamic>));
      }
      setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Deleting a reading asks first: it cost money, and the next one cannot be
  /// asked for until tomorrow.
  Future<void> _confirmDelete() async {
    final t = ref.read(tProvider);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(t('ai.deleteFeedback')),
        content: Text(t('ai.deleteFeedbackConfirm')),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(context).pop(false), child: Text(t('common.cancel'))),
          FilledButton(
              onPressed: () => Navigator.of(context).pop(true), child: Text(t('common.delete'))),
        ],
      ),
    );
    if (confirmed == true) await _delete();
  }

  Future<void> _delete() async {
    setState(() => _busy = true);
    try {
      await ref.read(apiProvider).deleteHealthFeedback(widget.provider);
      if (mounted) setState(() => _feedback = null);
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final locale = ref.watch(localeProvider);
    final colors = AppColors.of(context);
    final small = Theme.of(context).textTheme.bodySmall;
    final feedback = _feedback;

    return Container(
      margin: const EdgeInsets.only(top: 14),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.backgroundMuted,
        border: Border.all(color: colors.border),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Icon(Icons.auto_awesome_outlined, size: 18, color: colors.textMuted),
          const SizedBox(width: 6),
          Expanded(
            child: Text(t('devices.aiFeedback'),
                style: const TextStyle(fontWeight: FontWeight.w600)),
          ),
          if (feedback != null)
            IconButton(
              onPressed: _busy ? null : _confirmDelete,
              tooltip: t('ai.deleteFeedback'),
              icon: const Icon(Icons.delete_outline, size: 20),
              visualDensity: VisualDensity.compact,
            ),
        ]),
        const SizedBox(height: 6),
        if (feedback != null) ...[
          MarkdownText(feedback.content),
          const SizedBox(height: 6),
          // Who generated it and when, in the corner: a signature rather than
          // part of what was said.
          Align(
            alignment: AlignmentDirectional.centerEnd,
            child: Text(
              [
                t('ai.generatedBy', {
                  'connection': feedback.credited,
                  'date': longDate(locale, feedback.updatedAt),
                }),
                if (feedback.days > 0)
                  t('devices.aiFeedbackWindow', {'days': '${feedback.days}'}),
              ].join(' · '),
              style: small?.copyWith(fontStyle: FontStyle.italic),
            ),
          ),
        ] else
          Text(t('devices.aiFeedbackEmpty'), style: small),
        if (_error.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 6),
            child: Text(_error, style: TextStyle(color: colors.danger)),
          ),
        const SizedBox(height: 8),
        // Asked for as often as the member likes, through whichever
        // subscription they pick: the reading replaces the one before it,
        // since the ninety days it describes have moved on.
        AiAskButton(
          connections: _connections,
          busy: _busy,
          label: feedback == null ? t('devices.aiFeedbackAsk') : t('devices.aiFeedbackAgain'),
          tooltip: t('devices.aiFeedbackHelp'),
          onAsk: _generate,
        ),
      ]),
    );
  }
}

class _BodyCompositionPanel extends ConsumerStatefulWidget {
  final String provider;
  const _BodyCompositionPanel({required this.provider});

  @override
  ConsumerState<_BodyCompositionPanel> createState() => _BodyCompositionPanelState();
}

class _BodyCompositionPanelState extends ConsumerState<_BodyCompositionPanel> {
  List<BodyMeasure>? _measures;
  String _error = '';
  Object? _errorRaw;
  bool _loading = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      final measures = await ref.read(apiProvider).deviceBodyComposition(widget.provider);
      if (mounted) setState(() => _measures = measures);
    } catch (error) {
      if (mounted) {
        setState(() {
          _measures = null;
          _errorRaw = error;
          _error = ref.read(tErrorProvider)(error);
        });
      }
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final measures = _measures;
    final small = Theme.of(context).textTheme.bodySmall;

    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      const SizedBox(height: 14),
      Row(children: [
        Expanded(
          child: Text(t('devices.bodyComposition.title'),
              style: const TextStyle(fontWeight: FontWeight.w600)),
        ),
        IconButton(
          onPressed: _loading ? null : _load,
          tooltip: t('common.refresh'),
          icon: const Icon(Icons.refresh, size: 20),
          visualDensity: VisualDensity.compact,
        ),
      ]),
      if (_loading && measures == null)
        const Padding(
          padding: EdgeInsets.all(16),
          child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
        ),
      if (_error.isNotEmpty)
        _PanelError(_error, _errorRaw),
      if (measures != null && measures.isEmpty)
        Text(t('devices.bodyComposition.empty'), style: small),
      if (measures != null && measures.isNotEmpty)
        // A Wrap of fixed-width tiles, so a narrow phone shows two per row and
        // a tablet as many as fit.
        Wrap(spacing: 8, runSpacing: 8, children: [
          for (final measure in measures)
            Container(
              width: 150,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                border: Border.all(color: AppColors.of(context).border),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(t('devices.bodyComposition.measure.${measure.type}'), style: small),
                Text.rich(
                  TextSpan(children: [
                    TextSpan(text: _decimal(measure.value)),
                    if (measure.unit.isNotEmpty)
                      TextSpan(
                        text: ' ${measure.unit == 'kg/m2' ? 'kg/m²' : measure.unit}',
                        style: small,
                      ),
                  ]),
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                if (measure.takenAt != null)
                  Text(
                    t('devices.bodyComposition.takenOn', {
                      'date': measure.takenAt!.toIso8601String().substring(0, 10),
                    }),
                    style: small?.copyWith(fontSize: 11),
                  ),
              ]),
            ),
        ]),
    ]);
  }
}

/// One decimal at most, and none when the value is whole: 72.5 kg, 1650 kcal.
String _decimal(double value) {
  final rounded = (value * 10).round() / 10;
  return rounded == rounded.truncateToDouble()
      ? _grouped(rounded.toInt())
      : rounded.toStringAsFixed(1);
}

/// Signing in to an account-paired provider (Stance): an email and a password,
/// exchanged once for a grant. The password is never stored - only the tokens
/// the server returns are - so the field is write-only.
class _AccountPairForm extends StatefulWidget {
  final String provider;
  final bool busy;
  final void Function(String email, String password) onPair;

  const _AccountPairForm({required this.provider, required this.busy, required this.onPair});

  @override
  State<_AccountPairForm> createState() => _AccountPairFormState();
}

class _AccountPairFormState extends State<_AccountPairForm> {
  final _email = TextEditingController();
  final _password = TextEditingController();

  @override
  void dispose() {
    _email.dispose();
    _password.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer(builder: (context, ref, _) {
      final t = ref.watch(tProvider);
      final ready = _email.text.trim().isNotEmpty && _password.text.isNotEmpty;
      return Padding(
        padding: const EdgeInsets.only(top: 8),
        child: Column(children: [
          TextField(
            controller: _email,
            keyboardType: TextInputType.emailAddress,
            autocorrect: false,
            onChanged: (_) => setState(() {}),
            decoration: InputDecoration(
              labelText: t('devices.pair.${widget.provider}.email'),
              border: const OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _password,
            obscureText: true,
            onChanged: (_) => setState(() {}),
            decoration: InputDecoration(
              labelText: t('devices.pair.password'),
              border: const OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: AlignmentDirectional.centerEnd,
            child: FilledButton.icon(
              onPressed: widget.busy || !ready
                  ? null
                  : () => widget.onPair(_email.text.trim(), _password.text),
              icon: widget.busy
                  ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.link, size: 18),
              label: Text(t('devices.connect')),
            ),
          ),
        ]),
      );
    });
  }
}

/// A member's own personal API key for a key-paired provider (Garmin): one
/// field, masked, with a hint on where to get it.
class _KeyPairForm extends StatefulWidget {
  final String provider;
  final bool busy;
  final void Function(String apiKey) onPair;

  const _KeyPairForm({required this.provider, required this.busy, required this.onPair});

  @override
  State<_KeyPairForm> createState() => _KeyPairFormState();
}

class _KeyPairFormState extends State<_KeyPairForm> {
  final _key = TextEditingController();

  @override
  void dispose() {
    _key.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer(builder: (context, ref, _) {
      final t = ref.watch(tProvider);
      final ready = _key.text.trim().isNotEmpty;
      return Padding(
        padding: const EdgeInsets.only(top: 8),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          TextField(
            controller: _key,
            obscureText: true,
            autocorrect: false,
            onChanged: (_) => setState(() {}),
            decoration: InputDecoration(
              labelText: t('devices.pairKey.${widget.provider}.label'),
              border: const OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 4),
          Text(t('devices.pairKey.${widget.provider}.hint'),
              style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 8),
          Align(
            alignment: AlignmentDirectional.centerEnd,
            child: FilledButton.icon(
              onPressed:
                  widget.busy || !ready ? null : () => widget.onPair(_key.text.trim()),
              icon: widget.busy
                  ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.link, size: 18),
              label: Text(t('devices.connect')),
            ),
          ),
        ]),
      );
    });
  }
}

/// The last set of each lift the member tracked on a bar-speed provider: one
/// small chart a lift, a bar a rep, under the load and the two RPEs.
class _BarSpeedPanel extends ConsumerStatefulWidget {
  final String provider;
  const _BarSpeedPanel({required this.provider});

  @override
  ConsumerState<_BarSpeedPanel> createState() => _BarSpeedPanelState();
}

class _BarSpeedPanelState extends ConsumerState<_BarSpeedPanel> {
  List<StanceSet>? _sets;
  String _error = '';
  Object? _errorRaw;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final sets = await ref.read(apiProvider).barSpeed(widget.provider);
      if (mounted) setState(() => _sets = sets);
    } catch (error) {
      if (mounted) {
        setState(() {
          _errorRaw = error;
          _error = ref.read(tErrorProvider)(error);
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);
    final small = Theme.of(context).textTheme.bodySmall;

    if (_error.isNotEmpty) {
      return Padding(
        padding: const EdgeInsets.only(top: 8),
        child: _PanelError(_error, _errorRaw),
      );
    }
    final sets = _sets;
    if (sets == null) return const SizedBox.shrink();
    if (sets.isEmpty) {
      return Padding(
        padding: const EdgeInsets.only(top: 8),
        child: Text(t('devices.barSpeed.empty'), style: small),
      );
    }

    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      const SizedBox(height: 12),
      Text(t('devices.barSpeed.title'), style: const TextStyle(fontWeight: FontWeight.w600)),
      for (final set in sets) _BarSpeedSet(set: set),
    ]);
  }
}

/// One lift's last set: its heading, and a bar per rep.
class _BarSpeedSet extends ConsumerWidget {
  final StanceSet set;
  const _BarSpeedSet({required this.set});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);
    final small = Theme.of(context).textTheme.bodySmall;

    final parts = <String>[t('devices.lift.${set.lift}')];
    if (set.weightKg > 0 && set.reps > 0) {
      parts.add(t('devices.barSpeed.load', {'weight': _fmt(set.weightKg), 'reps': '${set.reps}'}));
    }
    if (set.givenRpe > 0 && set.predictedRpe > 0) {
      parts.add(t('devices.barSpeed.rpePair',
          {'given': _fmt(set.givenRpe), 'real': _fmt(set.predictedRpe)}));
    } else if (set.predictedRpe > 0) {
      parts.add(t('devices.barSpeed.rpeReal', {'real': _fmt(set.predictedRpe)}));
    }

    return Padding(
      padding: const EdgeInsets.only(top: 10),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text(parts.join('  ·  '), style: small),
        const SizedBox(height: 4),
        _VelocityBars(velocities: set.repVelocities),
      ]),
    );
  }
}

/// A bar per rep, its height the rep's velocity, captioned with the figure.
class _VelocityBars extends StatelessWidget {
  final List<double> velocities;
  const _VelocityBars({required this.velocities});

  @override
  Widget build(BuildContext context) {
    if (velocities.isEmpty) return const SizedBox.shrink();
    final peak = velocities.fold<double>(0.01, (a, b) => a > b ? a : b);

    return Container(
      height: 96,
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        color: AppColors.of(context).backgroundMuted,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          for (final velocity in velocities)
            Expanded(
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 3),
                child: Column(mainAxisSize: MainAxisSize.min,
                    mainAxisAlignment: MainAxisAlignment.end, children: [
                  Text(velocity.toStringAsFixed(2),
                      style: Theme.of(context).textTheme.labelSmall),
                  const SizedBox(height: 2),
                  Container(
                    height: (74 * velocity / peak).clamp(2, 74).toDouble(),
                    decoration: BoxDecoration(
                      color: AppColors.of(context).primary,
                      borderRadius: const BorderRadius.vertical(top: Radius.circular(2)),
                    ),
                  ),
                ]),
              ),
            ),
        ],
      ),
    );
  }
}

/// A measurement without a pointless decimal.
String _fmt(double value) =>
    value == value.roundToDouble() ? value.toStringAsFixed(0) : value.toStringAsFixed(1);

/// A panel's error: the translated message, and the troubleshooting console
/// below it when the failure carried an upstream detail.
class _PanelError extends StatelessWidget {
  final String message;
  final Object? raw;
  const _PanelError(this.message, this.raw);

  @override
  Widget build(BuildContext context) {
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text(message, style: TextStyle(color: AppColors.of(context).danger)),
      _DebugConsole(raw),
    ]);
  }
}

/// The raw upstream error (status and body) the API sent when a provider could
/// not be reached: collapsed by default, with a button to copy it so it can be
/// pasted into a report. Renders nothing for any other failure.
class _DebugConsole extends ConsumerWidget {
  final Object? error;
  const _DebugConsole(this.error);

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (error == null) return const SizedBox.shrink();
    final ex = asApiException(error!);
    final detail = ex.detail;
    if (detail == null || detail.isEmpty) return const SizedBox.shrink();
    final t = ref.watch(tProvider);
    final text = (ex.statusCode != null ? 'HTTP ${ex.statusCode}\n' : '') + detail;
    return Theme(
      data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
      child: ExpansionTile(
        tilePadding: EdgeInsets.zero,
        childrenPadding: const EdgeInsets.only(bottom: 8),
        dense: true,
        title: Text(t('common.troubleshoot'), style: Theme.of(context).textTheme.bodySmall),
        children: [
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: TextButton.icon(
              onPressed: () {
                Clipboard.setData(ClipboardData(text: text));
                ScaffoldMessenger.of(context)
                    .showSnackBar(SnackBar(content: Text(t('common.copied'))));
              },
              icon: const Icon(Icons.copy, size: 16),
              label: Text(t('common.copy')),
            ),
          ),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: const Color(0x14808080),
              borderRadius: BorderRadius.circular(6),
            ),
            child: SelectableText(
              text,
              style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
            ),
          ),
        ],
      ),
    );
  }
}
