import 'package:flutter/material.dart';

/// Always white regardless of theme - used as a foreground colour on top of a
/// coloured (primary/danger/navy) background, in both light and dark mode.
const Color kWhite = Color(0xFFFFFFFF);

/// A small shared palette, not a full design system.
///
/// The shape mirrors ~/cwclock's lib/theme.dart - a [ThemeExtension] so every
/// widget reading a token via `AppColors.of(context)` picks up the current
/// light/dark theme automatically - and the values mirror sf-ui's index.css,
/// so the web and mobile apps read as one product. The hues come from the logo:
/// a deep navy for chrome and a steel blue for action.
@immutable
class AppColors extends ThemeExtension<AppColors> {
  final Color primary;
  final Color primaryDark;
  final Color primaryTint;
  final Color navy;
  final Color danger;
  final Color success;
  final Color warning;
  final Color text;
  final Color textMuted;
  final Color border;
  final Color surface;
  final Color background;
  final Color backgroundMuted;

  const AppColors({
    required this.primary,
    required this.primaryDark,
    required this.primaryTint,
    required this.navy,
    required this.danger,
    required this.success,
    required this.warning,
    required this.text,
    required this.textMuted,
    required this.border,
    required this.surface,
    required this.background,
    required this.backgroundMuted,
  });

  /// Mirrors sf-ui's `:root` tokens.
  static const light = AppColors(
    primary: Color(0xFF0E5E9B),
    primaryDark: Color(0xFF0A4A7D),
    primaryTint: Color(0xFFEEF6FC),
    navy: Color(0xFF062A4E),
    danger: Color(0xFFDC2626),
    success: Color(0xFF16A34A),
    warning: Color(0xFFD97706),
    text: Color(0xFF0F172A),
    textMuted: Color(0xFF64748B),
    border: Color(0xFFE2E8F0),
    surface: Color(0xFFFFFFFF),
    background: Color(0xFFF8FAFC),
    backgroundMuted: Color(0xFFF1F5F9),
  );

  /// Mirrors sf-ui's `[data-theme="dark"]` tokens. The primary lightens in dark
  /// mode: the light theme's navy-leaning blue doesn't carry enough contrast
  /// against a near-black background.
  static const dark = AppColors(
    primary: Color(0xFF2B8FD4),
    primaryDark: Color(0xFF4AA6E4),
    primaryTint: Color(0x292B8FD4),
    navy: Color(0xFF0D1A2C),
    danger: Color(0xFFF87171),
    success: Color(0xFF4ADE80),
    warning: Color(0xFFFBBF24),
    text: Color(0xFFF1F5F9),
    textMuted: Color(0xFF94A3B8),
    border: Color(0xFF2A3750),
    surface: Color(0xFF141D2E),
    background: Color(0xFF0B1220),
    backgroundMuted: Color(0xFF182234),
  );

  static AppColors of(BuildContext context) => Theme.of(context).extension<AppColors>()!;

  @override
  AppColors copyWith({
    Color? primary,
    Color? primaryDark,
    Color? primaryTint,
    Color? navy,
    Color? danger,
    Color? success,
    Color? warning,
    Color? text,
    Color? textMuted,
    Color? border,
    Color? surface,
    Color? background,
    Color? backgroundMuted,
  }) {
    return AppColors(
      primary: primary ?? this.primary,
      primaryDark: primaryDark ?? this.primaryDark,
      primaryTint: primaryTint ?? this.primaryTint,
      navy: navy ?? this.navy,
      danger: danger ?? this.danger,
      success: success ?? this.success,
      warning: warning ?? this.warning,
      text: text ?? this.text,
      textMuted: textMuted ?? this.textMuted,
      border: border ?? this.border,
      surface: surface ?? this.surface,
      background: background ?? this.background,
      backgroundMuted: backgroundMuted ?? this.backgroundMuted,
    );
  }

  @override
  AppColors lerp(ThemeExtension<AppColors>? other, double t) {
    if (other is! AppColors) return this;
    return t < 0.5 ? this : other;
  }
}

/// Spacing on an 8pt grid, matching cwclock's AppSpacing.
class AppSpacing {
  const AppSpacing._();

  static double of(num n) => n * 8.0;
}

class AppRadius {
  const AppRadius._();

  static const value = 8.0;
}

ThemeData buildAppTheme(Brightness brightness) {
  final colors = brightness == Brightness.dark ? AppColors.dark : AppColors.light;

  return ThemeData(
    useMaterial3: true,
    brightness: brightness,
    scaffoldBackgroundColor: colors.background,
    extensions: [colors],
    colorScheme: ColorScheme.fromSeed(
      seedColor: colors.primary,
      brightness: brightness,
      primary: colors.primary,
      error: colors.danger,
      surface: colors.surface,
    ),
    appBarTheme: AppBarTheme(
      // The app bar is the navy chrome the sidebar is on the web, in both
      // themes - so its foreground is pinned white rather than following the
      // text token.
      backgroundColor: colors.navy,
      foregroundColor: kWhite,
      elevation: 0,
      centerTitle: false,
    ),
    cardTheme: CardThemeData(
      elevation: 0,
      margin: EdgeInsets.only(bottom: AppSpacing.of(1.5)),
      color: colors.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.value + 2),
        side: BorderSide(color: colors.border),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppRadius.value),
        borderSide: BorderSide(color: colors.border),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppRadius.value),
        borderSide: BorderSide(color: colors.border),
      ),
      filled: true,
      fillColor: colors.surface,
      isDense: true,
      labelStyle: TextStyle(color: colors.textMuted),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.value)),
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.of(2.5), vertical: AppSpacing.of(1.75)),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.value)),
        side: BorderSide(color: colors.border),
        foregroundColor: colors.text,
      ),
    ),
    dividerTheme: DividerThemeData(color: colors.border, space: 1),
    chipTheme: ChipThemeData(
      side: BorderSide.none,
      backgroundColor: colors.backgroundMuted,
      labelStyle: TextStyle(color: colors.textMuted, fontSize: 11),
    ),
    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: colors.surface,
      indicatorColor: colors.primaryTint,
      elevation: 0,
    ),
    snackBarTheme: SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: colors.navy,
      contentTextStyle: const TextStyle(color: kWhite),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.value)),
    ),
    textTheme: TextTheme(
      bodyMedium: TextStyle(color: colors.text),
      bodySmall: TextStyle(color: colors.textMuted),
    ),
  );
}
