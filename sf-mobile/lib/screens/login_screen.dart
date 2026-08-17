import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/api_exception.dart';
import '../providers/providers.dart';
import '../theme.dart';

/// Login, sign-up, and the second-factor step, in one screen.
///
/// Security keys are deliberately not offered here: the WebAuthn ceremony needs
/// a browser, so an account enrolled with a key only is pointed at the web app
/// rather than shown a button that cannot work.
class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

enum _Mode { login, signup, mfa }

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _email = TextEditingController();
  final _password = TextEditingController();
  final _name = TextEditingController();
  final _surname = TextEditingController();
  final _code = TextEditingController();
  final _apiUrl = TextEditingController();

  _Mode _mode = _Mode.login;
  String _challengeToken = '';
  bool _hasTotp = false;
  bool _busy = false;
  String? _error;
  String? _notice;
  bool _showServer = false;

  @override
  void initState() {
    super.initState();
    _apiUrl.text = ref.read(sessionProvider).apiUrl;
  }

  @override
  void dispose() {
    for (final controller in [_email, _password, _name, _surname, _code, _apiUrl]) {
      controller.dispose();
    }
    super.dispose();
  }

  Future<void> _run(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
    } catch (error) {
      if (mounted) setState(() => _error = ref.read(tErrorProvider)(error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _login() => _run(() async {
        final api = ref.read(apiProvider);
        final result = await api.login(_email.text.trim(), _password.text);
        if (result.mfaRequired) {
          setState(() {
            _mode = _Mode.mfa;
            _challengeToken = result.challengeToken;
            _hasTotp = result.hasTotp;
            _error = _hasTotp ? null : ref.read(tProvider)('mfa.unsupported');
          });
          return;
        }
        await ref.read(sessionProvider.notifier).completeLogin(result.token);
      });

  Future<void> _signup() => _run(() async {
        final t = ref.read(tProvider);
        final result = await ref.read(apiProvider).register(
              email: _email.text.trim(),
              password: _password.text,
              name: _name.text.trim(),
              surname: _surname.text.trim(),
              locale: ref.read(localeProvider),
            );
        // A new account starts disabled until it's activated - the very first
        // account on a fresh instance is the exception, and gets a session.
        if (result.token.isEmpty) {
          setState(() {
            _mode = _Mode.login;
            _notice = t('auth.checkEmail');
          });
          return;
        }
        try {
          await ref.read(sessionProvider.notifier).completeLogin(result.token);
        } on Object catch (error) {
          // A disabled account gets a token it can't use yet; say so instead of
          // failing with a bare error.
          if (asApiException(error).statusCode == 403) {
            setState(() {
              _mode = _Mode.login;
              _notice = t('auth.checkEmail');
            });
            return;
          }
          rethrow;
        }
      });

  Future<void> _verifyTotp() => _run(() async {
        final result = await ref.read(apiProvider).loginTotp(_challengeToken, _code.text.trim());
        await ref.read(sessionProvider.notifier).completeLogin(result.token);
      });

  Future<void> _forgotPassword() => _run(() async {
        await ref.read(apiProvider).forgotPassword(_email.text.trim());
        setState(() => _notice = ref.read(tProvider)('auth.resetLinkSent'));
      });

  @override
  Widget build(BuildContext context) {
    final t = ref.watch(tProvider);

    return Scaffold(
      body: Container(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [sfNavy, Color(0xFF0B3A63)],
          ),
        ),
        child: SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 420),
                child: Card(
                  margin: EdgeInsets.zero,
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: _body(t),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  List<Widget> _body(String Function(String, [Map<String, String>?]) t) {
    return [
      Image.asset('assets/images/logo.png', height: 90),
      const SizedBox(height: 16),
      Text(
        switch (_mode) {
          _Mode.login => t('auth.login'),
          _Mode.signup => t('auth.signup'),
          _Mode.mfa => t('auth.mfaTitle'),
        },
        style: Theme.of(context).textTheme.headlineSmall,
        textAlign: TextAlign.center,
      ),
      const SizedBox(height: 16),
      if (_notice != null) ...[
        _Banner(text: _notice!, color: Theme.of(context).colorScheme.primaryContainer),
        const SizedBox(height: 12),
      ],
      ..._fields(t),
      if (_error != null) ...[
        const SizedBox(height: 12),
        Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
      ],
      const SizedBox(height: 16),
      FilledButton(
        onPressed: _busy ? null : _submit,
        child: Text(_busy ? t('common.loading') : _submitLabel(t)),
      ),
      const SizedBox(height: 8),
      ..._footer(t),
    ];
  }

  List<Widget> _fields(String Function(String, [Map<String, String>?]) t) {
    if (_mode == _Mode.mfa) {
      return [
        TextField(
          controller: _code,
          keyboardType: TextInputType.number,
          maxLength: 6,
          autofocus: true,
          enabled: _hasTotp,
          decoration: InputDecoration(
            labelText: t('auth.mfaCode'),
            helperText: t('auth.mfaCodeHelp'),
            counterText: '',
          ),
        ),
      ];
    }

    return [
      if (_mode == _Mode.signup) ...[
        TextField(controller: _name, decoration: InputDecoration(labelText: t('auth.name'))),
        const SizedBox(height: 12),
        TextField(controller: _surname, decoration: InputDecoration(labelText: t('auth.surname'))),
        const SizedBox(height: 12),
      ],
      TextField(
        controller: _email,
        keyboardType: TextInputType.emailAddress,
        autocorrect: false,
        decoration: InputDecoration(labelText: t('auth.email')),
      ),
      const SizedBox(height: 12),
      TextField(
        controller: _password,
        obscureText: true,
        decoration: InputDecoration(labelText: t('auth.password')),
      ),
      if (_showServer) ...[
        const SizedBox(height: 12),
        TextField(
          controller: _apiUrl,
          keyboardType: TextInputType.url,
          autocorrect: false,
          decoration: const InputDecoration(labelText: 'Server URL'),
          onSubmitted: (value) => ref.read(sessionProvider.notifier).setApiUrl(value.trim()),
        ),
      ],
    ];
  }

  List<Widget> _footer(String Function(String, [Map<String, String>?]) t) {
    if (_mode == _Mode.mfa) {
      return [
        TextButton(
          onPressed: _busy ? null : () => setState(() => _mode = _Mode.login),
          child: Text(t('common.back')),
        ),
      ];
    }

    return [
      Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          if (_mode == _Mode.login)
            TextButton(onPressed: _busy ? null : _forgotPassword, child: Text(t('auth.forgotPassword')))
          else
            const SizedBox.shrink(),
          TextButton(
            onPressed: _busy
                ? null
                : () => setState(() {
                      _mode = _mode == _Mode.login ? _Mode.signup : _Mode.login;
                      _error = null;
                      _notice = null;
                    }),
            child: Text(_mode == _Mode.login ? t('auth.noAccount') : t('auth.hasAccount')),
          ),
        ],
      ),
      // The server field is tucked away: most people use the default instance,
      // but a club running its own needs to be able to point at it.
      TextButton(
        onPressed: () => setState(() => _showServer = !_showServer),
        child: Text(_showServer ? t('common.close') : 'Server settings'),
      ),
    ];
  }

  String _submitLabel(String Function(String, [Map<String, String>?]) t) => switch (_mode) {
        _Mode.login => t('auth.login'),
        _Mode.signup => t('auth.signup'),
        _Mode.mfa => t('auth.mfaVerify'),
      };

  Future<void> _submit() async {
    if (_showServer && _apiUrl.text.trim().isNotEmpty) {
      await ref.read(sessionProvider.notifier).setApiUrl(_apiUrl.text.trim());
    }
    switch (_mode) {
      case _Mode.login:
        await _login();
      case _Mode.signup:
        await _signup();
      case _Mode.mfa:
        await _verifyTotp();
    }
  }
}

class _Banner extends StatelessWidget {
  final String text;
  final Color color;

  const _Banner({required this.text, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(color: color, borderRadius: BorderRadius.circular(8)),
      child: Text(text),
    );
  }
}
