import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/llm_settings_page.dart';

// Sprachmodell (Backlog I.3.5b, umgebaut in I.8.2b). Erfundener Schluessel.

Map<String, dynamic> _mandant(
        {bool aktiv = false, bool schluessel = true, int aufrufe = 3}) =>
    {
      'aktiv': aktiv,
      'tageslimit': 500,
      'aufrufe_heute': aufrufe,
      'schluessel_eingerichtet': schluessel,
    };

Map<String, dynamic> _profil(String id, String name,
        {bool standard = false,
        bool aktiv = true,
        String anbieter = 'openai_compatible',
        bool lesbar = true}) =>
    {
      'id': id,
      'name': name,
      'aktiv': aktiv,
      'standard': standard,
      'anbieter': anbieter,
      'endpunkt': 'http://localhost:11434/v1',
      'modell': 'lokal-7b',
      'timeout_s': 300,
      'schwelle': 0.9,
      'api_key_gesetzt': true,
      'api_key_lesbar': lesbar,
    };

class _FakeApi extends ApiClient {
  _FakeApi(
      {Map<String, dynamic>? mandant,
      List<Map<String, dynamic>>? profile,
      this.saveError,
      this.testError})
      : mandant = mandant ?? _mandant(),
        profile = profile ??
            [
              _profil('p-1', 'Lokal', standard: true),
              _profil('p-2', 'Claude', anbieter: 'anthropic', aktiv: false),
            ],
        super(baseUrl: 'http://localhost:8080');

  Map<String, dynamic> mandant;
  List<Map<String, dynamic>> profile;
  final ApiException? saveError;
  final ApiException? testError;
  final List<Map<String, dynamic>> saved = [];
  final List<String> tests = [];
  final List<String> standards = [];
  final List<Map<String, dynamic>> created = [];

  @override
  Future<Map<String, dynamic>> getLlmMandant() async => mandant;

  @override
  Future<Map<String, dynamic>> saveLlmMandant(Map<String, dynamic> body) async {
    saved.add(body);
    if (saveError != null) throw saveError!;
    mandant = {...mandant, ...body};
    return mandant;
  }

  @override
  Future<List<dynamic>> listLlmProfiles() async => [...profile];

  @override
  Future<Map<String, dynamic>> setLlmStandardProfile(String id) async {
    standards.add(id);
    profile = [
      for (final p in profile) {...p, 'standard': p['id'] == id}
    ]..sort(
        (a, b) => a['standard'] == true ? -1 : (b['standard'] == true ? 1 : 0));
    return profile.first;
  }

  @override
  Future<Map<String, dynamic>> createLlmProfile(
      Map<String, dynamic> body) async {
    created.add(body);
    final p = {
      ..._profil('p-${profile.length + 1}', body['name'].toString()),
      'aktiv': body['aktiv'],
      'standard': profile.isEmpty,
    };
    profile = [...profile, p];
    return p;
  }

  @override
  Future<Map<String, dynamic>> testLlmProfile(String id) async {
    tests.add(id);
    mandant = {
      ...mandant,
      'aufrufe_heute': (mandant['aufrufe_heute'] as int) + 1
    };
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

void main() {
  test('buildLlmMandantBody', () {
    expect(buildLlmMandantBody(aktiv: true, tageslimit: ' 20 ').body,
        {'aktiv': true, 'tageslimit': 20});
    expect(buildLlmMandantBody(aktiv: false, tageslimit: '').body,
        {'aktiv': false, 'tageslimit': 0});
    expect(buildLlmMandantBody(aktiv: true, tageslimit: '1').error, isNull);
    expect(
        buildLlmMandantBody(aktiv: true, tageslimit: '100000').error, isNull);
    for (final l in ['x', '0', '100001', '-1', '2.5']) {
      expect(buildLlmMandantBody(aktiv: true, tageslimit: l).error,
          'Tageslimit muss eine Zahl zwischen 1 und 100000 sein',
          reason: l);
    }
  });

  test('llmTestSummary und llmProfilZeile', () {
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
    expect(llmProfilZeile(_profil('p', 'x')),
        'OpenAI-kompatibel · lokal-7b · Schwelle 0,9 · eingeschaltet');
    expect(
        llmProfilZeile({
          ..._profil('p', 'x', anbieter: 'anthropic', aktiv: false),
          'modell': ''
        }),
        'Anthropic · Schwelle 0,9 · ausgeschaltet');
  });

  testWidgets('zeigt Hauptschalter, Tageslimit und Profile ohne Schluessel',
      (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(find.text('Heute: 3 Aufrufe'), findsOneWidget);
    expect(find.text('Lokal'), findsOneWidget);
    expect(find.text('Claude'), findsOneWidget);
    expect(find.text('Standard'), findsOneWidget);
    expect(find.text('Anthropic · lokal-7b · Schwelle 0,9 · ausgeschaltet'),
        findsOneWidget);
    // Standardprofil ohne "Als Standard"
    expect(find.byKey(const ValueKey('standard-p-1')), findsNothing);
    expect(find.byKey(const ValueKey('standard-p-2')), findsOneWidget);
    expect(find.byIcon(Icons.warning_amber), findsNothing);
    expect(find.text('Noch kein Profil angelegt – das erste wird Standard.'),
        findsNothing);
  });

  testWidgets('Hauptschalter speichern, ungueltiges Tageslimit',
      (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.text('Sprachmodell eingeschaltet'));
    await tester.enterText(find.byKey(const ValueKey('tageslimit')), '50');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved.single, {'aktiv': true, 'tageslimit': 50});
    expect(find.text('Einstellungen des Sprachmodells gespeichert'),
        findsOneWidget);

    await tester.enterText(find.byKey(const ValueKey('tageslimit')), '0');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Tageslimit muss eine Zahl zwischen 1 und 100000 sein'),
        findsOneWidget);
    expect(api.saved, hasLength(1));
  });

  testWidgets(
      'Einschalten ohne Profil: Meldung des Servers, keine Erfolgsmeldung',
      (tester) async {
    final failing = _FakeApi(
        profile: [],
        saveError: const ApiException(
            statusCode: 400,
            message: 'Zum Einschalten fehlt ein eingeschaltetes Profil'));
    await _pump(tester, failing);
    expect(find.text('Noch kein Profil angelegt – das erste wird Standard.'),
        findsOneWidget);
    await tester.tap(find.text('Sprachmodell eingeschaltet'));
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Zum Einschalten fehlt ein eingeschaltetes Profil'),
        findsOneWidget);
    expect(
        find.text('Einstellungen des Sprachmodells gespeichert'), findsNothing);
  });

  testWidgets('eingeschalteter Hauptschalter wird gezeigt und beibehalten',
      (tester) async {
    final api = _FakeApi(mandant: _mandant(aktiv: true));
    await _pump(tester, api);
    expect(tester.widget<SwitchListTile>(find.byType(SwitchListTile)).value,
        isTrue);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved.single, {'aktiv': true, 'tageslimit': 500});
  });

  testWidgets('Standard wechseln laedt die Liste neu', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('standard-p-2')));
    await tester.pumpAndSettle();
    expect(api.standards, ['p-2']);
    expect(find.text('„Claude“ ist jetzt Standard'), findsOneWidget);
    expect(find.byKey(const ValueKey('standard-p-2')), findsNothing);
    expect(find.byKey(const ValueKey('standard-p-1')), findsOneWidget);
  });

  testWidgets('Profil anlegen ueber den Dialog', (tester) async {
    final api = _FakeApi(profile: []);
    await _pump(tester, api);
    await tester.tap(find.text('Profil anlegen'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('profilName')), 'Neu');
    await tester.enterText(find.byKey(const ValueKey('profilEndpunkt')),
        'http://localhost:11434/v1');
    await tester.tap(find.text('Speichern').last);
    await tester.pumpAndSettle();
    expect(api.created.single['name'], 'Neu');
    expect(find.text('Profil angelegt'), findsOneWidget);
    expect(find.text('Neu'), findsOneWidget);
    expect(find.text('Standard'), findsOneWidget);
  });

  testWidgets(
      'Bearbeiten oeffnet den Dialog mit dem Stand, Abbrechen aendert nichts',
      (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('bearbeiten-p-2')));
    await tester.pumpAndSettle();
    expect(find.text('Profil bearbeiten'), findsOneWidget);
    expect(
        tester
            .widget<TextField>(find.byKey(const ValueKey('profilName')))
            .controller!
            .text,
        'Claude');
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(find.text('Profil bearbeiten'), findsNothing);
    expect(find.text('Profil gespeichert'), findsNothing);
  });

  testWidgets('Warnungen: kein Schluessel, API-Schluessel nicht lesbar',
      (tester) async {
    await _pump(tester, _FakeApi(mandant: _mandant(schluessel: false)));
    expect(find.textContaining('SETTINGS_ENCRYPTION_KEY'), findsOneWidget);
    expect(find.textContaining('nicht mehr entschlüsseln'), findsNothing);

    await _pump(
        tester,
        _FakeApi(profile: [
          _profil('p-1', 'Lokal', standard: true),
          _profil('p-2', 'Claude', lesbar: false),
        ]));
    expect(find.textContaining('nicht mehr entschlüsseln'), findsOneWidget);
    expect(find.textContaining('SETTINGS_ENCRYPTION_KEY'), findsNothing);
  });

  testWidgets(
      'Verbindungstest je Profil: Erfolg zaehlt mit, Fehler mit Meldung',
      (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.enterText(find.byKey(const ValueKey('tageslimit')), '77');
    await tester.tap(find.byKey(const ValueKey('testen-p-2')));
    await tester.pumpAndSettle();
    expect(api.tests, ['p-2']);
    expect(
        find.text(
            'Verbindung in Ordnung – Modell lokal-7b, 120 ms, 12 Token gesendet, 3 Token erhalten'),
        findsOneWidget);
    expect(find.text('Heute: 4 Aufrufe'), findsOneWidget);
    // Eingaben bleiben stehen
    expect(
        tester
            .widget<TextField>(find.byKey(const ValueKey('tageslimit')))
            .controller!
            .text,
        '77');

    final failing = _FakeApi(
        testError: const ApiException(
            statusCode: 502,
            message:
                'Test des Sprachmodells fehlgeschlagen: Anfrage abgelehnt (401)',
            code: 'llm_test_failed'));
    await _pump(tester, failing);
    await tester.tap(find.byKey(const ValueKey('testen-p-1')));
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
    await tester.tap(find.byKey(const ValueKey('testen-p-1')));
    await tester.pumpAndSettle();
    expect(
        find.text(
            'Test fehlgeschlagen: Tageslimit des Sprachmodells erreicht (500 Aufrufe)'),
        findsOneWidget);
  });

  testWidgets('Fehler beim Laden', (tester) async {
    await _pump(tester, _LoadFailApi());
    expect(
        find.text(
            'Einstellungen konnten nicht geladen werden: Keine Berechtigung'),
        findsOneWidget);
  });
}

class _LoadFailApi extends _FakeApi {
  @override
  Future<Map<String, dynamic>> getLlmMandant() async =>
      throw const ApiException(statusCode: 403, message: 'Keine Berechtigung');
}
