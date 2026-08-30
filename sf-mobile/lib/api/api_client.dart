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
  final Dio dio = Dio(BaseOptions(
    // Only the *connect* phase is bounded globally. A request that is already
    // talking to the server may legitimately take minutes - a video on a gym's
    // connection - and cutting that off is what an upload timeout looks like
    // from the outside. A host that never answers should still fail rather
    // than spin forever, hence this one.
    connectTimeout: const Duration(seconds: 30),
  ));

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
      // Nothing came back at all. On an upload that is worth naming: the API
      // stops an oversized body part-way through, and what reaches the phone
      // is a dropped connection, not the 413 it sent - so dio's own "the
      // connection errored" was the whole explanation a member got for a
      // video that was simply too long.
      final uploading = error.requestOptions.path.startsWith('/media/');
      return error.copyWith(
        error: ApiException(
          i18nCode: uploading ? 'errors.uploadInterrupted' : null,
          message: error.message,
        ),
      );
    }
    final data = response.data;
    String? i18nCode;
    String? message;
    if (data is Map) {
      i18nCode = data['i18n_code'] as String?;
      message = data['message'] as String?;
    }

    // A 413 does not always come from this API. A reverse proxy in front of it
    // refuses an oversized body itself, and what it sends back is its own HTML
    // error page - no i18n code, nothing this app can read - so the member was
    // told "something went wrong" about a video that was simply too big. The
    // status alone says enough.
    if (response.statusCode == 413 && (i18nCode == null || i18nCode.isEmpty)) {
      i18nCode = 'errors.videoTooLarge';
      message = null;
    }

    return error.copyWith(
      error: ApiException(i18nCode: i18nCode, message: message, statusCode: response.statusCode),
    );
  }
}
