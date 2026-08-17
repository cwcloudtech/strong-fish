import 'package:dio/dio.dart';

import 'api_exception.dart';

/// The shared HTTP client: base URL `${apiUrl}/v1`, with the session's bearer
/// token injected on every request.
///
/// The API URL is configurable at runtime rather than baked in, so the same
/// build can point at a self-hosted instance - which is the normal case for a
/// club running its own server.
class ApiClient {
  final Dio dio = Dio();

  String _apiUrl = '';
  String? _token;

  ApiClient() {
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          if (_token != null && _token!.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $_token';
          }
          handler.next(options);
        },
        onError: (error, handler) => handler.reject(_toApiError(error)),
      ),
    );
  }

  String get apiUrl => _apiUrl;

  bool get hasSession => _token != null && _token!.isNotEmpty;

  void setApiUrl(String url) {
    _apiUrl = url.replaceFirst(RegExp(r'/+$'), '');
    dio.options.baseUrl = _apiUrl.isNotEmpty ? '$_apiUrl/v1' : '';
  }

  void setToken(String? token) => _token = token;

  void clearSession() {
    _token = null;
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
