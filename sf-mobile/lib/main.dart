import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

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
    return MaterialApp.router(
      title: 'strong-fish',
      debugShowCheckedModeBanner: false,
      theme: buildAppTheme(Brightness.light),
      darkTheme: buildAppTheme(Brightness.dark),
      themeMode: ref.watch(themeModeProvider),
      routerConfig: ref.watch(routerProvider),
    );
  }
}
