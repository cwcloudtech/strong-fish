/// A date the way a credit line reads it: "October 19th of 2026",
/// "19 octobre 2026".
///
/// Long rather than the numeric form used for timestamps elsewhere: this one
/// sits under a piece of writing, where "19/10/2026" reads as a reference
/// number and a month's name reads as a date.
///
/// The month names live here rather than in the i18n dictionaries, which are
/// generated from the web's (see tools/gen_dart_i18n.mjs) - the web gets
/// these from Intl and would carry thirty-six strings it never reads. The
/// phone has no Intl, and pulling a localization package in to name twelve
/// months would be a dependency for a table.
const _months = {
  'en': [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
  ],
  'fr': [
    'janvier', 'février', 'mars', 'avril', 'mai', 'juin',
    'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre',
  ],
  'ar': [
    'يناير', 'فبراير', 'مارس', 'أبريل', 'مايو', 'يونيو',
    'يوليو', 'أغسطس', 'سبتمبر', 'أكتوبر', 'نوفمبر', 'ديسمبر',
  ],
};

String longDate(String locale, DateTime at) {
  final local = at.toLocal();
  final months = _months[locale] ?? _months['en']!;
  final month = months[local.month - 1];

  // English takes the ordinal and the "of", which is how the line is written
  // there; the others put the day first.
  if (locale == 'en') {
    return '$month ${_ordinal(local.day)} of ${local.year}';
  }
  return '${local.day} $month ${local.year}';
}

/// 1st, 2nd, 3rd, 4th - and the three teens that look like exceptions and are
/// not.
String _ordinal(int day) {
  if (day < 11 || day > 13) {
    switch (day % 10) {
      case 1:
        return '${day}st';
      case 2:
        return '${day}nd';
      case 3:
        return '${day}rd';
    }
  }
  return '${day}th';
}
