import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/providers.dart';
import '../theme.dart';

/// The sensor's battery level as a small icon-and-percent chip, the way a
/// brand's own app shows it: green when there is plenty, amber getting low, red
/// near empty. Renders nothing until a reading has arrived (percent < 0).
class BatteryBadge extends ConsumerWidget {
  final int percent;
  final double size;
  const BatteryBadge({super.key, required this.percent, this.size = 16});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (percent < 0) return const SizedBox.shrink();
    final t = ref.watch(tProvider);
    final colors = AppColors.of(context);
    final tone = percent <= 15
        ? colors.danger
        : percent <= 30
            ? colors.warning
            : colors.success;

    return Row(mainAxisSize: MainAxisSize.min, children: [
      Icon(_icon(percent), size: size, color: tone),
      const SizedBox(width: 3),
      Text(
        t('devices.sensor.battery', {'percent': '$percent'}),
        style: TextStyle(color: tone, fontSize: 12, fontWeight: FontWeight.w600),
      ),
    ]);
  }

  IconData _icon(int percent) {
    if (percent >= 90) return Icons.battery_full;
    if (percent >= 70) return Icons.battery_5_bar;
    if (percent >= 55) return Icons.battery_4_bar;
    if (percent >= 40) return Icons.battery_3_bar;
    if (percent >= 25) return Icons.battery_2_bar;
    if (percent >= 10) return Icons.battery_1_bar;
    return Icons.battery_alert;
  }
}
