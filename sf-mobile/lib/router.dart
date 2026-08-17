import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'providers/providers.dart';
import 'screens/login_screen.dart';
import 'screens/main_tabs_screen.dart';
import 'screens/one_rms_screen.dart';
import 'screens/session_screen.dart';

/// Routing reacts to the session: the redirect below is what sends someone to
/// the login screen when their token stops working mid-use, without every
/// screen having to check.
final routerProvider = Provider<GoRouter>((ref) {
  final notifier = ValueNotifier(ref.read(sessionProvider).status);
  ref.onDispose(notifier.dispose);
  ref.listen(sessionProvider, (_, next) => notifier.value = next.status);

  return GoRouter(
    initialLocation: '/training',
    refreshListenable: notifier,
    redirect: (context, state) {
      final status = ref.read(sessionProvider).status;
      final onLogin = state.matchedLocation == '/login';
      final onSplash = state.matchedLocation == '/';

      return switch (status) {
        SessionStatus.restoring => onSplash ? null : '/',
        SessionStatus.missing => onLogin ? null : '/login',
        SessionStatus.connected => (onLogin || onSplash) ? '/training' : null,
      };
    },
    routes: [
      GoRoute(path: '/', builder: (context, state) => const _SplashScreen()),
      GoRoute(path: '/login', builder: (context, state) => const LoginScreen()),
      GoRoute(path: '/training', builder: (context, state) => const MainTabsScreen()),
      GoRoute(
        path: '/training/:assignmentId',
        builder: (context, state) =>
            SessionScreen(assignmentId: state.pathParameters['assignmentId']!),
      ),
      GoRoute(
        path: '/one-rms',
        builder: (context, state) => const _OneRmsPage(),
      ),
    ],
  );
});

class _SplashScreen extends StatelessWidget {
  const _SplashScreen();

  @override
  Widget build(BuildContext context) {
    return const Scaffold(body: Center(child: CircularProgressIndicator()));
  }
}

/// The maxes screen reached from a session's "enter my 1RMs" prompt - the same
/// screen as the tab, but pushed with its own app bar so it can be dismissed
/// back to the session.
class _OneRmsPage extends ConsumerWidget {
  const _OneRmsPage();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      appBar: AppBar(title: Text(ref.watch(tProvider)('oneRms.title'))),
      body: const OneRmsScreen(),
    );
  }
}
