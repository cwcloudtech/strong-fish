import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'i18n/app_localizations.dart';
import 'providers/providers.dart';
import 'router.dart';
import 'theme.dart';

void main() {
  runApp(const ProviderScope(child: StrongFishApp()));
}

class StrongFishApp extends ConsumerStatefulWidget {
  const StrongFishApp({super.key});

  @override
  ConsumerState<StrongFishApp> createState() => _StrongFishAppState();
}

class _StrongFishAppState extends ConsumerState<StrongFishApp> {
  @override
  void initState() {
    super.initState();
    // Deferred past the first frame: these read shared preferences and secure
    // storage, and restoring the session also calls the API.
    Future.microtask(() async {
      await ref.read(localeProvider.notifier).load();
      await ref.read(themeModeProvider.notifier).load();
      await ref.read(sessionProvider.notifier).restore();
    });
  }

  @override
  Widget build(BuildContext context) {
    final locale = ref.watch(localeProvider);

    return MaterialApp.router(
      title: 'strong-fish',
      debugShowCheckedModeBanner: false,
      theme: buildAppTheme(Brightness.light),
      darkTheme: buildAppTheme(Brightness.dark),
      themeMode: ref.watch(themeModeProvider),
      routerConfig: ref.watch(routerProvider),
      // The app uses its own dictionaries rather than Flutter's generated
      // localizations, so the text direction is not wired up for it. Flip the
      // whole tree when an RTL locale is picked - Arabic reads right to left,
      // and so should the screen around it.
      builder: (context, child) => Directionality(
        textDirection: isRtlLocale(locale) ? TextDirection.rtl : TextDirection.ltr,
        child: child ?? const SizedBox.shrink(),
      ),
    );
  }
}
