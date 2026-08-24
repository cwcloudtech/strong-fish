import 'ar.dart';
import 'en.dart';
import 'fr.dart';

/// Minimal i18n: one nested dictionary per locale, plus a lookup with
/// `{{var}}` interpolation and an English fallback for missing keys. It mirrors
/// sf-ui's i18n/translate.js, so a key written once works in both clients.
const Map<String, Map<String, dynamic>> dictionaries = {'en': en, 'fr': fr, 'ar': ar};

const List<AppLocale> supportedLocales = [
  AppLocale('en', 'English'),
  AppLocale('fr', 'Français'),
  AppLocale('ar', 'العربية'),
];

/// Locales written right to left.
///
/// This app carries its own dictionaries rather than Flutter's generated
/// localizations, so nothing wires the text direction up for it: Arabic *text*
/// renders right-to-left on its own, but the layout - which side a back arrow
/// sits on, which way a list tile's leading and trailing run, where a drawer
/// comes from - stays left-to-right until the whole tree is told otherwise.
/// main.dart wraps the app in a Directionality for these, the same way
/// ~/uprodit's mobile app does.
const Set<String> rtlLocales = {'ar'};

bool isRtlLocale(String locale) => rtlLocales.contains(locale);

class AppLocale {
  final String code;
  final String label;

  const AppLocale(this.code, this.label);
}

dynamic _resolve(Map<String, dynamic>? dictionary, String key) {
  dynamic value = dictionary;
  for (final part in key.split('.')) {
    if (value is Map<String, dynamic> && value.containsKey(part)) {
      value = value[part];
    } else {
      return null;
    }
  }
  return value;
}

/// Translates [key] into [locale]. A missing key falls back to English and then
/// to the key itself, which makes an untranslated string obvious rather than
/// blank.
String translate(String locale, String key, [Map<String, String>? vars]) {
  var text =
      (_resolve(dictionaries[locale], key) ?? _resolve(dictionaries['en'], key) ?? key).toString();
  if (vars != null) {
    for (final entry in vars.entries) {
      text = text.replaceAll('{{${entry.key}}}', entry.value);
    }
  }
  return text;
}

/// Implemented by ApiException - declared here rather than imported so the i18n
/// layer doesn't depend on the api layer.
abstract class ApiErrorLike {
  String? get i18nCode;
  String? get message;
}

/// Turns an API failure into a translated message: the backend sends an
/// `i18n_code` looked up in the same dictionaries the UI uses, falling back to
/// the server's English message when the code is unknown.
String translateError(String locale, Object error) {
  if (error is ApiErrorLike) {
    final code = error.i18nCode;
    if (code != null && code.isNotEmpty) {
      final translated = _resolve(dictionaries[locale], code) ?? _resolve(dictionaries['en'], code);
      if (translated != null) return translated.toString();
    }
    final message = error.message;
    if (message != null && message.isNotEmpty) return message;
  }
  return translate(locale, 'errors.internal');
}
