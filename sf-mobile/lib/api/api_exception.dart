import 'package:dio/dio.dart';

import '../i18n/app_localizations.dart';

/// A failed API call, carrying the backend's `i18n_code` so the UI can show the
/// message in the reader's language rather than the server's English fallback.
class ApiException implements Exception, ApiErrorLike {
  @override
  final String? i18nCode;
  @override
  final String? message;
  final int? statusCode;

  /// An upstream error (a status and a body) surfaced for troubleshooting when
  /// a provider could not be reached. Shown in a console, never as the message.
  final String? detail;

  /// Which integration an upstream detail is from (garmin, stance, ...).
  final String? provider;

  ApiException({this.i18nCode, this.message, this.statusCode, this.detail, this.provider});

  bool get isUnauthorized => statusCode == 401;

  @override
  String toString() => message ?? i18nCode ?? 'ApiException';
}

/// Unwraps the [ApiException] a failure carries, whether it came from the Dio
/// interceptor (a [DioException] wrapping one) or was thrown directly.
ApiException asApiException(Object error) {
  if (error is ApiException) return error;
  if (error is DioException && error.error is ApiException) {
    return error.error as ApiException;
  }
  return ApiException(message: error.toString());
}
