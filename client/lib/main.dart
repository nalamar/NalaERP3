import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
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
      home: const DashboardPage(),
      theme: ThemeData(useMaterial3: true, colorScheme: colorScheme, scaffoldBackgroundColor: const Color(0xFFF7FAFF)),
    );
  }
}
