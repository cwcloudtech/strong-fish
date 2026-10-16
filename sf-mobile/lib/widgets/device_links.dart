import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/models.dart';
import '../providers/providers.dart';
import '../theme.dart';

/// The windows the API offers, in the order they widen.
const _ranges = ['24h', '48h', 'week', 'month', '90d'];

/// Ranges wide enough for a daily average to mean anything.
const _averaged = {'week', 'month', '90d'};

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
              else
                FilledButton.icon(
                  // Offered but not pressable when the server has no
                  // credentials for it: there is nothing to consent to.
                  onPressed:
                      connection.available ? () => _connect(connection.provider) : null,
                  style: _compactButton,
                  icon: const Icon(Icons.link, size: 18),
                  label: Text(t('devices.connect')),
                ),
            ]),
            if (connection.connected) _StepPanel(provider: connection.provider),
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
      // Together: they are the same days of the same range, and two panels
      // that could disagree about which week they show would be worse than
      // one that is briefly empty.
      final api = ref.read(apiProvider);
      final results = await Future.wait([
        api.deviceSteps(widget.provider, _range),
        api.deviceHeartRate(widget.provider, _range),
      ]);
      if (mounted) {
        setState(() {
          _series = results[0] as StepSeries;
          _heart = results[1] as HeartSeries;
        });
      }
    } catch (error) {
      if (mounted) {
        setState(() {
          _series = null;
          _heart = null;
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
    final series = _series;

    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      const SizedBox(height: 10),
      SingleChildScrollView(
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
      if (_loading && series == null)
        const Padding(
          padding: EdgeInsets.all(16),
          child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
        ),
      if (_error.isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(top: 10),
          child: Text(_error, style: TextStyle(color: AppColors.of(context).danger)),
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
        ]),
        const SizedBox(height: 12),
        _StepBars(days: series.days),
        if (_heart != null) _HeartSection(heart: _heart!),
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
