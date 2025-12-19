import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:google_fonts/google_fonts.dart';
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
          child: Text('Fehler: ${details.exceptionAsString()}',
              style: const TextStyle(color: Color(0xFFB00020))),
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
    final colorScheme = ColorScheme.fromSeed(
      seedColor: const Color(0xFF2EDBBD),
      brightness: Brightness.light,
    );
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
      builder: (context, child) {
        return Stack(
          children: [
            Container(
              decoration: const BoxDecoration(
                gradient: LinearGradient(
                  colors: [
                    Color(0xFF0B1024),
                    Color(0xFF0F1E38),
                    Color(0xFF0E2D3F),
                  ],
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                ),
              ),
            ),
            Positioned(
                top: -120,
                left: -60,
                child: _GlowBlob(colors: [
                  const Color(0xFF2EDBBD).withValues(alpha: 0.35),
                  const Color(0xFF1EA7FF).withValues(alpha: 0.25)
                ])),
            Positioned(
                bottom: -140,
                right: -40,
                child: _GlowBlob(colors: [
                  const Color(0xFF4B64FF).withValues(alpha: 0.22),
                  const Color(0xFF20B0A4).withValues(alpha: 0.28)
                ])),
            child ?? const SizedBox.shrink(),
          ],
        );
      },
      theme: ThemeData(
        useMaterial3: true,
        colorScheme: colorScheme,
        scaffoldBackgroundColor: Colors.transparent,
        textTheme: GoogleFonts.outfitTextTheme(),
        appBarTheme: AppBarTheme(
          backgroundColor: Colors.transparent,
          surfaceTintColor: Colors.transparent,
          elevation: 0,
          foregroundColor: colorScheme.onSurface,
          toolbarHeight: 64,
        ),
        cardTheme: CardThemeData(
          color: Colors.white.withValues(alpha: 0.06),
          shadowColor: colorScheme.primary.withValues(alpha: 0.28),
          elevation: 10,
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
        ),
        inputDecorationTheme: InputDecorationTheme(
          filled: true,
          fillColor: Colors.white.withValues(alpha: 0.06),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(14),
            borderSide: BorderSide(color: Colors.white.withValues(alpha: 0.12)),
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(14),
            borderSide: BorderSide(color: Colors.white.withValues(alpha: 0.12)),
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(14),
            borderSide: BorderSide(color: colorScheme.primary, width: 1.6),
          ),
          hintStyle: TextStyle(color: Colors.white.withValues(alpha: 0.62)),
          labelStyle: TextStyle(color: Colors.white.withValues(alpha: 0.8)),
        ),
        floatingActionButtonTheme: FloatingActionButtonThemeData(
          backgroundColor: colorScheme.primary,
          foregroundColor: Colors.black,
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
        ),
        filledButtonTheme: FilledButtonThemeData(
          style: FilledButton.styleFrom(
            backgroundColor: colorScheme.primary,
            foregroundColor: Colors.black,
            shape:
                RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            shadowColor: colorScheme.primary.withValues(alpha: 0.25),
            elevation: 3,
          ),
        ),
      ),
    );
  }
}

class _GlowBlob extends StatelessWidget {
  const _GlowBlob({required this.colors});
  final List<Color> colors;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 360,
      height: 360,
      decoration: BoxDecoration(
        gradient: RadialGradient(colors: colors),
        borderRadius: BorderRadius.circular(240),
        boxShadow: [
          BoxShadow(
            color: colors.first.withValues(alpha: 0.35),
            blurRadius: 60,
            spreadRadius: 40,
          )
        ],
      ),
    );
  }
}
