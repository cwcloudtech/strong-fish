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
Color stepTone(BuildContext context, int steps) {
  final colors = AppColors.of(context);
  if (steps < 6000) return colors.danger;
  if (steps < 10000) return colors.warning;
  if (steps < 15000) return const Color(0xFF65C37A);
  return colors.success;
}

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
  bool _awaitingConsent = false;

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

  /// Consent happens in the system browser, so the app finds out it worked by
  /// being brought back to the front. Only after a consent was actually
  /// started: reloading on every resume would re-fetch the list every time the
  /// member glanced at a notification.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && _awaitingConsent) {
      _awaitingConsent = false;
      _load();
    }
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
      _awaitingConsent = true;
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (error) {
      _awaitingConsent = false;
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
          Text(t('devices.title'), style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 4),
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
                  icon: const Icon(Icons.link_off, size: 18),
                  label: Text(t('devices.disconnect')),
                )
              else
                FilledButton.icon(
                  // Offered but not pressable when the server has no
                  // credentials for it: there is nothing to consent to.
                  onPressed:
                      connection.available ? () => _connect(connection.provider) : null,
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
      final series = await ref.read(apiProvider).deviceSteps(widget.provider, _range);
      if (mounted) setState(() => _series = series);
    } catch (error) {
      if (mounted) {
        setState(() {
          _series = null;
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
        Row(children: [
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
    return Padding(
      padding: const EdgeInsetsDirectional.only(end: 22),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text(label, style: Theme.of(context).textTheme.bodySmall),
        Text(
          _grouped(value),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(color: tone),
        ),
      ]),
    );
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
