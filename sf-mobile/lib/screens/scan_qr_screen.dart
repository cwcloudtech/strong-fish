import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

import '../api/config_parser.dart';
import '../providers/providers.dart';

/// Signs this device in by scanning the QR code shown on the web app's API
/// keys screen.
///
/// A QR code is only a container for text, so there is no QR-specific format
/// here: the scanner hands back the same `api_url = ...` / `api_key = ...`
/// lines the downloadable config file holds, and parseConfigText reads both.
///
/// [MobileScanner] is used directly, with no separate permission pre-check -
/// it requests and manages camera permission itself as its controller starts,
/// and layering a second request in front of it leaves the preview running
/// while the controller never initialises, so nothing is ever detected.
class ScanQrScreen extends ConsumerStatefulWidget {
  const ScanQrScreen({super.key});

  @override
  ConsumerState<ScanQrScreen> createState() => _ScanQrScreenState();
}

class _ScanQrScreenState extends ConsumerState<ScanQrScreen> {
  bool _handled = false;
  bool _connecting = false;
  String? _error;

  Future<void> _onDetect(BarcodeCapture capture) async {
    if (_handled || _connecting) return;

    String? raw;
    for (final barcode in capture.barcodes) {
      final value = barcode.rawValue;
      if (value != null && value.trim().isNotEmpty) {
        raw = value;
        break;
      }
    }
    if (raw == null) return;

    final t = ref.read(tProvider);
    final config = parseConfigText(raw);
    if (!config.isComplete) {
      // Not our code - a URL, a boarding pass, anything. Say so and keep the
      // camera running rather than closing the screen.
      setState(() => _error = t('auth.scanFailed'));
      return;
    }

    _handled = true;
    setState(() {
      _connecting = true;
      _error = null;
    });
    try {
      await ref.read(sessionProvider.notifier).connectWithConfig(config);
      if (mounted) Navigator.of(context).pop();
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = ref.read(tErrorProvider)(error);
          _handled = false;
        });
      }
    } finally {
      if (mounted) setState(() => _connecting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);

    return Scaffold(
      appBar: AppBar(title: Text(t('auth.scanQr'))),
      body: Stack(
        children: [
          MobileScanner(onDetect: _onDetect),
          if (_connecting) const LinearProgressIndicator(),
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: Container(
              padding: const EdgeInsets.all(16),
              color: Colors.black.withValues(alpha: 0.55),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    t('auth.scanQrHelp'),
                    textAlign: TextAlign.center,
                    style: const TextStyle(color: Colors.white, fontSize: 14),
                  ),
                  if (_error != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(
                        _error!,
                        textAlign: TextAlign.center,
                        style: const TextStyle(color: Color(0xFFF87171), fontSize: 13),
                      ),
                    ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
