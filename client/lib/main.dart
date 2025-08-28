import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';

void main() {
  // Stelle sicher, dass deutsch als Standard gilt
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const NalaERPApp());
}

class NalaERPApp extends StatelessWidget {
  const NalaERPApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'NalaERP3',
      debugShowCheckedModeBanner: false,
      locale: const Locale('de', 'DE'),
      supportedLocales: const [Locale('de', 'DE')],
      home: const HomePage(),
      theme: ThemeData(useMaterial3: true, brightness: Brightness.light),
    );
  }
}

class HomePage extends StatelessWidget {
  const HomePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('NalaERP3 – Materialverwaltung (MVP)'),
      ),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text('Hallo Welt! Flutter Web läuft auf Port 3000.'),
            if (kIsWeb) const Text('Modus: Web'),
          ],
        ),
      ),
    );
  }
}

