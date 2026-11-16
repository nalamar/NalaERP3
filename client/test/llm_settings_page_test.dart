import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/llm_settings_page.dart';

// Sprachmodell (Backlog I.3.5b). Erfundener Schluessel.

Map<String, dynamic> _stand({
  bool aktiv = false,
  String anbieter = 'openai_compatible',
  bool gesetzt = true,
  bool lesbar = true,
  bool schluessel = true,
  int aufrufe = 3,
}) =>
    {
      'eingerichtet': true,
      'aktiv': aktiv,
      'anbieter': anbieter,
      'endpunkt': 'http://localhost:11434/v1',
      'modell': 'lokal-7b',
      'timeout_s': 300,
      'tageslimit': 500,
      'aufrufe_heute': aufrufe,
      'api_key_gesetzt': gesetzt,
      'api_key_lesbar': lesbar,
      'schluessel_eingerichtet': schluessel,
    };

class _FakeApi extends ApiClient {
  _FakeApi({Map<String, dynamic>? stand, this.saveError, this.testError})
      : stand = stand ?? _stand(),
        super(baseUrl: 'http://localhost:8080');

  Map<String, dynamic> stand;
  final ApiException? saveError;
  final ApiException? testError;
  final List<Map<String, dynamic>> saved = [];
  int tests = 0;

  @override
  Future<Map<String, dynamic>> getLlmSettings() async => stand;

  @override
  Future<Map<String, dynamic>> saveLlmSettings(
      Map<String, dynamic> body) async {
    saved.add(body);
    if (saveError != null) throw saveError!;
    final gesetzt = body.containsKey('api_key') ||
        (stand['api_key_gesetzt'] == true &&
            !body.containsKey('api_key_loeschen'));
    stand = {
      ...stand,
      ...Map.of(body)
        ..remove('api_key')
        ..remove('api_key_loeschen'),
      'api_key_gesetzt': gesetzt,
    };
    return stand;
  }

  @override
  Future<Map<String, dynamic>> testLlmSettings() async {
    tests++;
    stand = {...stand, 'aufrufe_heute': (stand['aufrufe_heute'] as int) + 1};
    if (testError != null) throw testError!;
    return {
      'ok': true,
      'modell': 'lokal-7b',
      'dauer_ms': 120,
      'eingabe_tokens': 12,
      'ausgabe_tokens': 3
    };
  }
}

Future<void> _pump(WidgetTester tester, _FakeApi api) async {
  tester.view.physicalSize = const Size(1200, 2000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
      MaterialApp(home: LlmSettingsPage(key: UniqueKey(), api: api)));
  await tester.pumpAndSettle();
}

TextField _textField(WidgetTester tester, String key) =>
    tester.widget<TextField>(find.byKey(ValueKey(key)));

void main() {
  group('buildLlmSettingsBody', () {
    ({Map<String, dynamic>? body, String? error}) build(
            {String timeout = '300',
            String limit = '500',
            String key = '',
            bool loeschen = false}) =>
        buildLlmSettingsBody(
          aktiv: true,
          anbieter: 'mistral',
          endpunkt: ' https://api.example.test/v1 ',
          modell: ' modell-1 ',
          apiKey: key,
          apiKeyLoeschen: loeschen,
          timeout: timeout,
          tageslimit: limit,
        );

    test('ohne Schluessel behaelt den gespeicherten', () {
      final r = build();
      expect(r.error, isNull);
      expect(r.body, {
        'aktiv': true,
        'anbieter': 'mistral',
        'endpunkt': 'https://api.example.test/v1',
        'modell': 'modell-1',
        'timeout_s': 300,
        'tageslimit': 500,
      });
    });

    test('Schluessel unveraendert, Loeschen eigens, leer = Standard', () {
      expect(build(key: ' sk-erfunden ').body!['api_key'], ' sk-erfunden ');
      expect(build(loeschen: true).body!['api_key_loeschen'], true);
      expect(build(loeschen: true).body!.containsKey('api_key'), isFalse);
      expect(build(timeout: ' ', limit: '').body!['timeout_s'], 0);
      expect(build(timeout: ' ', limit: '').body!['tageslimit'], 0);
      expect(build(timeout: '1800', limit: '100000').error, isNull);
    });

    test('abgelehnt', () {
      for (final t in ['abc', '0', '1801', '-5', '2.5']) {
        expect(build(timeout: t).error,
            'Zeitlimit muss eine Zahl zwischen 1 und 1800 Sekunden sein',
            reason: t);
      }
      for (final l in ['x', '0', '100001']) {
        expect(build(limit: l).error,
            'Tageslimit muss eine Zahl zwischen 1 und 100000 sein',
            reason: l);
      }
      expect(build(key: 'x', loeschen: true).error,
          'API-Schlüssel entweder neu setzen oder löschen');
    });
  });

  test('llmTestSummary', () {
    expect(
        llmTestSummary({
          'modell': 'm-1',
          'dauer_ms': 80,
          'eingabe_tokens': 10,
          'ausgabe_tokens': 2
        }),
        'Verbindung in Ordnung – Modell m-1, 80 ms, 10 Token gesendet, 2 Token erhalten');
    expect(llmTestSummary({}),
        'Verbindung in Ordnung – Modell unbekannt, 0 ms, 0 Token gesendet, 0 Token erhalten');
  });

  testWidgets('zeigt den Stand ohne Schluessel und speichert ohne Schluessel',
      (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(find.text('http://localhost:11434/v1'), findsOneWidget);
    expect(find.text('lokal-7b'), findsOneWidget);
    expect(find.text('Heute: 3 Aufrufe'), findsOneWidget);
    expect(find.text('Gespeichert – leer lassen, um ihn zu behalten'),
        findsOneWidget);
    expect(_textField(tester, 'apiKey').controller!.text, '');
    expect(_textField(tester, 'apiKey').obscureText, isTrue);
    expect(find.byIcon(Icons.warning_amber), findsNothing);

    await tester.tap(find.text('Sprachmodell eingeschaltet'));
    await tester.enterText(find.byKey(const ValueKey('timeout')), '120');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved, hasLength(1));
    expect(api.saved.single['aktiv'], true);
    expect(api.saved.single['timeout_s'], 120);
    expect(api.saved.single.containsKey('api_key'), isFalse);
    expect(find.text('Einstellungen des Sprachmodells gespeichert'),
        findsOneWidget);
  });

  testWidgets(
      'Anbieter wechseln: Endpunkt und Modell optional, Schluessel gesendet und geleert',
      (tester) async {
    final api = _FakeApi(stand: _stand(gesetzt: false));
    await _pump(tester, api);
    expect(find.text('Endpunkt'), findsOneWidget);
    expect(find.text('Gespeicherten API-Schlüssel löschen'), findsNothing);
    await tester.tap(find.byKey(const ValueKey('anbieter')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anthropic').last);
    await tester.pumpAndSettle();
    expect(find.text('Endpunkt (optional)'), findsOneWidget);
    expect(find.text('Modell (optional)'), findsOneWidget);
    await tester.enterText(find.byKey(const ValueKey('endpunkt')), '');
    await tester.enterText(
        find.byKey(const ValueKey('apiKey')), 'sk-erfunden-42');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved.single['anbieter'], 'anthropic');
    expect(api.saved.single['endpunkt'], '');
    expect(api.saved.single['api_key'], 'sk-erfunden-42');
    expect(_textField(tester, 'apiKey').controller!.text, '');
    expect(find.text('Gespeichert – leer lassen, um ihn zu behalten'),
        findsOneWidget);
  });

  testWidgets('Schluessel loeschen', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.enterText(find.byKey(const ValueKey('apiKey')), 'tippfehler');
    await tester.tap(find.text('Gespeicherten API-Schlüssel löschen'));
    await tester.pumpAndSettle();
    expect(_textField(tester, 'apiKey').enabled, isFalse);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved.single['api_key_loeschen'], true);
    expect(api.saved.single.containsKey('api_key'), isFalse);
  });

  testWidgets(
      'ungueltiges Zeitlimit und Fehler des Servers: Meldung, Eingaben bleiben',
      (tester) async {
    final api = _FakeApi(
        saveError: const ApiException(
            statusCode: 400,
            message: 'Für das Sprachmodell fehlt: Endpunkt und Modell'));
    await _pump(tester, api);
    await tester.enterText(find.byKey(const ValueKey('timeout')), '9999');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(
        find.text('Zeitlimit muss eine Zahl zwischen 1 und 1800 Sekunden sein'),
        findsOneWidget);
    expect(api.saved, isEmpty);

    await tester.enterText(find.byKey(const ValueKey('timeout')), '300');
    await tester.enterText(find.byKey(const ValueKey('modell')), '');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Für das Sprachmodell fehlt: Endpunkt und Modell'),
        findsOneWidget);
    expect(_textField(tester, 'modell').controller!.text, '');
    expect(
        find.text('Einstellungen des Sprachmodells gespeichert'), findsNothing);
  });

  testWidgets('Warnungen: kein Schluessel, API-Schluessel nicht lesbar',
      (tester) async {
    await _pump(
        tester,
        _FakeApi(
            stand: _stand(schluessel: false, gesetzt: false, lesbar: false)));
    expect(find.textContaining('SETTINGS_ENCRYPTION_KEY'), findsOneWidget);
    expect(find.textContaining('nicht mehr entschlüsseln'), findsNothing);

    await _pump(tester, _FakeApi(stand: _stand(lesbar: false)));
    expect(find.textContaining('nicht mehr entschlüsseln'), findsOneWidget);
    expect(find.textContaining('SETTINGS_ENCRYPTION_KEY'), findsNothing);
  });

  testWidgets('Verbindungstest: Erfolg zaehlt mit, Fehler mit Meldung',
      (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.enterText(
        find.byKey(const ValueKey('modell')), 'ungespeichert');
    await tester.tap(find.text('Verbindung testen'));
    await tester.pumpAndSettle();
    expect(api.tests, 1);
    expect(
        find.text(
            'Verbindung in Ordnung – Modell lokal-7b, 120 ms, 12 Token gesendet, 3 Token erhalten'),
        findsOneWidget);
    expect(find.text('Heute: 4 Aufrufe'), findsOneWidget);
    expect(_textField(tester, 'modell').controller!.text, 'ungespeichert');

    final failing = _FakeApi(
        testError: const ApiException(
            statusCode: 502,
            message:
                'Test des Sprachmodells fehlgeschlagen: Anfrage abgelehnt (401)',
            code: 'llm_test_failed'));
    await _pump(tester, failing);
    await tester.tap(find.text('Verbindung testen'));
    await tester.pumpAndSettle();
    expect(
        find.text(
            'Test des Sprachmodells fehlgeschlagen: Anfrage abgelehnt (401)'),
        findsOneWidget);

    final limit = _FakeApi(
        testError: const ApiException(
            statusCode: 429,
            message: 'Tageslimit des Sprachmodells erreicht (500 Aufrufe)'));
    await _pump(tester, limit);
    await tester.tap(find.text('Verbindung testen'));
    await tester.pumpAndSettle();
    expect(
        find.text(
            'Test fehlgeschlagen: Tageslimit des Sprachmodells erreicht (500 Aufrufe)'),
        findsOneWidget);
  });
}
