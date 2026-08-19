import 'package:dio/dio.dart';

import 'api_exception.dart';

/// The shared HTTP client: base URL `${apiUrl}/v1`, with the session's
/// credential injected on every request - a bearer token from a password
/// login, or an `X-Api-Key` when the device was enrolled by scanning a QR
/// code.
///
/// The API URL is configurable at runtime rather than baked in, so the same
/// build can point at a self-hosted instance - which is the normal case for a
/// club running its own server.
class ApiClient {
  final Dio dio = Dio();

  String _apiUrl = '';
  String? _token;
  String? _apiKey;

  ApiClient() {
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          // An API key is authoritative when the device was enrolled by
          // scanning one: the API rejects a bad key outright rather than
          // falling back, so sending both would only be ambiguous.
          if (_apiKey != null && _apiKey!.isNotEmpty) {
            options.headers['X-Api-Key'] = _apiKey;
          } else if (_token != null && _token!.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $_token';
          }
          handler.next(options);
        },
        onError: (error, handler) => handler.reject(_toApiError(error)),
      ),
    );
  }

  String get apiUrl => _apiUrl;

  /// Where the web app lives, derived from the API's own address.
  ///
  /// Shared links have to open the web app, not the API - and the two are the
  /// same deployment, so "api." becoming "www." is the convention rather than
  /// another setting for somebody to get wrong. A URL that does not follow it
  /// falls back to itself, which is at least the right deployment.
  String get frontendUrl {
    final uri = Uri.tryParse(_apiUrl);
    if (uri == null || uri.host.isEmpty) return _apiUrl;
    if (!uri.host.startsWith('api.')) return _apiUrl;
    return uri.replace(host: 'www.${uri.host.substring(4)}').toString();
  }

  bool get hasSession =>
      (_token != null && _token!.isNotEmpty) || (_apiKey != null && _apiKey!.isNotEmpty);

  void setApiUrl(String url) {
    _apiUrl = url.replaceFirst(RegExp(r'/+$'), '');
    dio.options.baseUrl = _apiUrl.isNotEmpty ? '$_apiUrl/v1' : '';
  }

  void setToken(String? token) => _token = token;

  void setApiKey(String? apiKey) => _apiKey = apiKey;

  void clearSession() {
    _token = null;
    _apiKey = null;
  }

  DioException _toApiError(DioException error) {
    final response = error.response;
    if (response == null) {
      return error.copyWith(error: ApiException(message: error.message));
    }
    final data = response.data;
    String? i18nCode;
    String? message;
    if (data is Map) {
      i18nCode = data['i18n_code'] as String?;
      message = data['message'] as String?;
    }
    return error.copyWith(
      error: ApiException(i18nCode: i18nCode, message: message, statusCode: response.statusCode),
    );
  }
}
