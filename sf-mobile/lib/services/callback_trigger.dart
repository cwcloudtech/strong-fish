import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:mqtt_client/mqtt_client.dart';
import 'package:mqtt_client/mqtt_server_client.dart';
import 'package:web_socket_channel/io.dart';

import '../models/models.dart';

/// What a recording callback carries: the lift and the load - the way a brand's
/// app signals its sensor - plus whether this is the start or the stop of the
/// set, so a device can begin and end its own recording.
class BarSpeedTrigger {
  final String lift;
  final double load;
  final String unit;
  final String action;

  const BarSpeedTrigger({
    required this.lift,
    required this.load,
    required this.action,
    this.unit = 'kg',
  });

  Map<String, dynamic> toPayload() => {
        'event': 'bar-speed',
        'action': action,
        'lift': lift,
        'load': load,
        'unit': unit,
        'source': 'strongfish',
        'firedAt': DateTime.now().toUtc().toIso8601String(),
      };
}

/// The outcome of firing one callback, so the screen can say which device it
/// could not reach.
class CallbackResult {
  final RecordingCallback callback;
  final bool ok;
  final String error;

  const CallbackResult({required this.callback, required this.ok, this.error = ''});
}

/// Fires recording callbacks from the phone, over whichever transport each one
/// uses: an HTTP webhook (a plain POST), a WebSocket message, or an MQTT
/// publish. The body is the same JSON whichever way it travels - only the
/// carriage differs - which is the whole point of cwcloud's "callback".
///
/// These go to the member's own device, not to StrongFish, so each transport
/// uses a fresh connection carrying none of StrongFish's session auth. It never
/// throws - a device being off cannot be allowed to fail the recording the
/// member is doing on the phone - and reports each endpoint's outcome instead.
class CallbackTrigger {
  CallbackTrigger._();
  static final CallbackTrigger instance = CallbackTrigger._();

  static const Duration _timeout = Duration(seconds: 5);

  final Dio _dio = Dio(BaseOptions(
    connectTimeout: _timeout,
    sendTimeout: _timeout,
    receiveTimeout: _timeout,
  ));

  /// Fires every enabled callback in [callbacks], one by one, and returns one
  /// result per endpoint it tried (disabled ones are skipped).
  Future<List<CallbackResult>> fire(
    List<RecordingCallback> callbacks,
    BarSpeedTrigger trigger,
  ) async {
    final results = <CallbackResult>[];
    final payload = trigger.toPayload();
    final body = jsonEncode(payload);

    for (final callback in callbacks) {
      if (!callback.enabled || callback.url.trim().isEmpty) continue;
      try {
        switch (callback.type) {
          case 'websocket':
            await _fireWebSocket(callback, body);
            break;
          case 'mqtt':
            await _fireMqtt(callback, body);
            break;
          case 'http':
          default:
            await _fireHttp(callback, payload);
        }
        results.add(CallbackResult(callback: callback, ok: true));
      } catch (error) {
        results.add(CallbackResult(callback: callback, ok: false, error: error.toString()));
      }
    }
    return results;
  }

  /// The request headers for an HTTP or WebSocket callback: the member's custom
  /// headers, then an Authorization header - a bearer token when set, otherwise
  /// HTTP basic auth from the username and password.
  Map<String, String> _headers(RecordingCallback callback) {
    final headers = <String, String>{};
    for (final header in callback.headers) {
      if (header.name.trim().isNotEmpty) headers[header.name.trim()] = header.value;
    }
    if (callback.token.trim().isNotEmpty) {
      headers['Authorization'] = callback.token.trim();
    } else if (callback.username.trim().isNotEmpty) {
      final basic = base64Encode(utf8.encode('${callback.username.trim()}:${callback.password}'));
      headers['Authorization'] = 'Basic $basic';
    }
    return headers;
  }

  Future<void> _fireHttp(RecordingCallback callback, Map<String, dynamic> payload) async {
    await _dio.post(callback.url.trim(), data: payload, options: Options(headers: _headers(callback)));
  }

  Future<void> _fireWebSocket(RecordingCallback callback, String body) async {
    final channel = IOWebSocketChannel.connect(
      Uri.parse(callback.url.trim()),
      headers: _headers(callback),
      connectTimeout: _timeout,
    );
    try {
      await channel.ready.timeout(_timeout);
      channel.sink.add(body);
    } finally {
      await channel.sink.close();
    }
  }

  Future<void> _fireMqtt(RecordingCallback callback, String body) async {
    final uri = Uri.tryParse(callback.url.trim()) ?? Uri();
    final scheme = uri.scheme.toLowerCase();
    final secure = scheme == 'mqtts' || scheme == 'ssl' || scheme == 'wss';
    final overWebSocket = scheme == 'ws' || scheme == 'wss';
    final defaultPort = overWebSocket ? (secure ? 443 : 80) : (secure ? 8883 : 1883);
    final port = uri.hasPort ? uri.port : defaultPort;
    final host = uri.host.isNotEmpty ? uri.host : callback.url.trim();
    final clientId = callback.clientId.trim().isNotEmpty
        ? callback.clientId.trim()
        : 'strongfish-${DateTime.now().millisecondsSinceEpoch}';

    // Over WebSocket the broker wants the full ws(s):// URL as its server; over
    // plain TCP it wants the bare host.
    final client = MqttServerClient.withPort(
      overWebSocket ? callback.url.trim() : host,
      clientId,
      port,
    );
    client.secure = secure && !overWebSocket;
    client.useWebSocket = overWebSocket;
    client.logging(on: false);
    client.keepAlivePeriod = 20;
    client.connectTimeoutPeriod = _timeout.inMilliseconds;
    client.autoReconnect = false;

    final username = callback.username.trim().isEmpty ? null : callback.username.trim();
    final password = callback.password.isEmpty ? null : callback.password;
    try {
      await client.connect(username, password);
      final builder = MqttClientPayloadBuilder()..addString(body);
      final qos = MqttQos.values[callback.qos.clamp(0, 2)];
      client.publishMessage(callback.topic.trim(), qos, builder.payload!);
      // A beat for a QoS>0 ack, or for the socket to flush a QoS0 publish,
      // before the disconnect tears it down.
      await Future<void>.delayed(const Duration(milliseconds: 200));
    } finally {
      client.disconnect();
    }
  }
}
