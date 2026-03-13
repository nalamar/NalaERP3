import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'api.dart';
import 'pages/dashboard_page.dart';

void main() {
  // Stelle sicher, dass deutsch als Standard gilt
  WidgetsFlutterBinding.ensureInitialized();
  FlutterError.onError = (FlutterErrorDetails details) {
    debugPrint('FlutterError: \n${details.exceptionAsString()}');
    FlutterError.dumpErrorToConsole(details);
  };
  ErrorWidget.builder = (FlutterErrorDetails details) {
    return Directionality(
      textDirection: TextDirection.ltr,
      child: Material(
        color: const Color(0xFFFFEAEA),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Text('Fehler: ${details.exceptionAsString()}', style: const TextStyle(color: Color(0xFFB00020))),
        ),
      ),
    );
  };
  runApp(const NalaERPApp());
}

class NalaERPApp extends StatelessWidget {
  const NalaERPApp({super.key});

  @override
  Widget build(BuildContext context) {
    final colorScheme = ColorScheme.fromSeed(seedColor: const Color(0xFF1E88E5), brightness: Brightness.light);
    return MaterialApp(
      title: 'NalaERP3',
      debugShowCheckedModeBanner: false,
      locale: const Locale('de', 'DE'),
      supportedLocales: const [Locale('de', 'DE')],
      localizationsDelegates: const [
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      home: const AuthGate(),
      theme: ThemeData(useMaterial3: true, colorScheme: colorScheme, scaffoldBackgroundColor: const Color(0xFFF7FAFF)),
    );
  }
}

class AuthGate extends StatefulWidget {
  const AuthGate({super.key});

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  final ApiClient _api = ApiClient();
  bool _loading = true;
  Map<String, dynamic>? _identity;
  String? _error;

  @override
  void initState() {
    super.initState();
    _api.authState.addListener(_handleAuthStateChanged);
    _restore();
  }

  @override
  void dispose() {
    _api.authState.removeListener(_handleAuthStateChanged);
    super.dispose();
  }

  void _handleAuthStateChanged() {
    if (!mounted) return;
    if (_api.authState.value) return;
    setState(() {
      _identity = null;
      _loading = false;
      _error = null;
    });
  }

  Future<void> _restore() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final ok = await _api.restoreSession();
    if (!mounted) return;
    if (!ok) {
      setState(() {
        _loading = false;
        _identity = null;
      });
      return;
    }
    try {
      final me = await _api.getCurrentUser();
      if (!mounted) return;
      setState(() {
        _identity = me;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _handleLogin(String login, String password) async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await _api.login(login, password);
      final me = await _api.getCurrentUser();
      if (!mounted) return;
      setState(() {
        _identity = me;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _handleLogout() async {
    await _api.logout();
    if (!mounted) return;
    setState(() {
      _identity = null;
      _error = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (_identity == null) {
      return LoginPage(
        error: _error,
        onLogin: _handleLogin,
      );
    }
    final rawPermissions = (_identity!['permissions'] as List?) ?? const [];
    final permissions = rawPermissions.map((e) => e.toString()).toSet();
    return DashboardPage(
      api: _api,
      onLogout: _handleLogout,
      permissions: permissions,
    );
  }
}

class LoginPage extends StatefulWidget {
  const LoginPage({super.key, required this.onLogin, this.error});
  final Future<void> Function(String login, String password) onLogin;
  final String? error;

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _loginCtrl = TextEditingController();
  final _passwordCtrl = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _loginCtrl.dispose();
    _passwordCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final login = _loginCtrl.text.trim();
    final password = _passwordCtrl.text;
    if (login.isEmpty || password.isEmpty) return;
    setState(() => _submitting = true);
    try {
      await widget.onLogin(login, password);
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            colors: [color.withOpacity(0.10), const Color(0xFFF7FAFF), Colors.white],
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
          ),
        ),
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Card(
              elevation: 8,
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text('NalaERP3', style: Theme.of(context).textTheme.headlineSmall),
                    const SizedBox(height: 8),
                    const Text('Bitte anmelden, um das ERP zu verwenden.'),
                    const SizedBox(height: 16),
                    TextField(
                      controller: _loginCtrl,
                      decoration: const InputDecoration(labelText: 'E-Mail oder Benutzername'),
                      onSubmitted: (_) => _submit(),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: _passwordCtrl,
                      obscureText: true,
                      decoration: const InputDecoration(labelText: 'Passwort'),
                      onSubmitted: (_) => _submit(),
                    ),
                    if (widget.error != null && widget.error!.trim().isNotEmpty) ...[
                      const SizedBox(height: 12),
                      Text(widget.error!, style: const TextStyle(color: Color(0xFFB00020))),
                    ],
                    const SizedBox(height: 20),
                    FilledButton(
                      onPressed: _submitting ? null : _submit,
                      child: _submitting
                          ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
                          : const Text('Anmelden'),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
