import 'package:flutter/material.dart';

/// The palette is taken from the logo: a deep navy for structure and a steel
/// blue for action - the same two colours sf-ui is built on, so the web and
/// mobile apps read as one product.
const Color sfNavy = Color(0xFF062A4E);
const Color sfPrimary = Color(0xFF0E5E9B);

ThemeData buildAppTheme(Brightness brightness) {
  final isDark = brightness == Brightness.dark;
  final scheme = ColorScheme.fromSeed(
    seedColor: sfPrimary,
    brightness: brightness,
    primary: sfPrimary,
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: isDark ? const Color(0xFF0A1729) : const Color(0xFFF1F5F9),
    appBarTheme: AppBarTheme(
      backgroundColor: isDark ? const Color(0xFF0F2135) : sfNavy,
      foregroundColor: Colors.white,
      elevation: 0,
      centerTitle: false,
    ),
    cardTheme: CardThemeData(
      elevation: 0,
      margin: const EdgeInsets.only(bottom: 12),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: isDark ? const Color(0xFF1E3A56) : const Color(0xFFDBE3EC)),
      ),
      color: isDark ? const Color(0xFF0F2135) : Colors.white,
    ),
    inputDecorationTheme: InputDecorationTheme(
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(8)),
      isDense: true,
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
      ),
    ),
    chipTheme: const ChipThemeData(side: BorderSide.none),
  );
}
