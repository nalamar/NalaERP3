import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/llm_profile_dialog.dart';

// Modellprofil anlegen/bearbeiten (Backlog I.8.2a). Erfundener Schluessel.

class _FakeApi extends ApiClient {
  _FakeApi({this.error}) : super(baseUrl: 'http://localhost:8080');

  final ApiException? error;
  final List<Map<String, dynamic>> created = [];
  final List<(String, Map<String, dynamic>)> updated = [];

  @override
  Future<Map<String, dynamic>> createLlmProfile(
      Map<String, dynamic> body) async {
    created.add(body);
    if (error != null) throw error!;
    return {'id': 'neu-1', ...body}..remove('api_key');
  }

  @override
  Future<Map<String, dynamic>> updateLlmProfile(
      String id, Map<String, dynamic> body) async {
    updated.add((id, body));
    if (error != null) throw error!;
    return {'id': id, ...body}..remove('api_key');
  }
}

const _profil = {
  'id': 'p-1',
  'name': 'Lokal',
  'aktiv': true,
  'standard': true,
  'anbieter': 'openai_compatible',
  'endpunkt': 'http://localhost:11434/v1',
  'modell': 'lokal-7b',
  'timeout_s': 120,
  'schwelle': 0.75,
  'api_key_gesetzt': true,
  'api_key_lesbar': true,
};

// Startet eine Seite mit Knopf, der den Dialog oeffnet; Ergebnis in result.
Future<List<Map<String, dynamic>?>> _pump(WidgetTester tester, _FakeApi api,
    {Map<String, dynamic>? profil}) async {
  tester.view.physicalSize = const Size(1200, 2000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final result = <Map<String, dynamic>?>[];
  await tester.pumpWidget(MaterialApp(
    home: Builder(
      builder: (context) => TextButton(
        onPressed: () async => result
            .add(await showLlmProfileDialog(context, api, profil: profil)),
        child: const Text('öffnen'),
      ),
    ),
  ));
  await tester.tap(find.text('öffnen'));
  await tester.pumpAndSettle();
  return result;
}

TextField _textField(WidgetTester tester, String key) =>
    tester.widget<TextField>(find.byKey(ValueKey(key)));

void main() {
  group('buildLlmProfileBody', () {
    ({Map<String, dynamic>? body, String? error}) build(
            {String name = ' Claude ',
            String timeout = '300',
            String schwelle = '0,8',
            String key = '',
            bool loeschen = false}) =>
        buildLlmProfileBody(
          name: name,
          aktiv: true,
          anbieter: 'anthropic',
          endpunkt: ' https://api.example.test/v1 ',
          modell: ' modell-1 ',
          apiKey: key,
          apiKeyLoeschen: loeschen,
          timeout: timeout,
          schwelle: schwelle,
        );

    test('vollstaendig, Komma als Dezimalzeichen', () {
      final r = build();
      expect(r.error, isNull);
      expect(r.body, {
        'name': 'Claude',
        'aktiv': true,
        'anbieter': 'anthropic',
        'endpunkt': 'https://api.example.test/v1',
        'modell': 'modell-1',
        'timeout_s': 300,
        'schwelle': 0.8,
      });
    });

    test('Grenzwerte, leer = Standard, Schluessel unveraendert', () {
      expect(build(schwelle: '0').body!['schwelle'], 0);
      expect(build(schwelle: '1').body!['schwelle'], 1);
      expect(build(schwelle: '0.95').body!['schwelle'], 0.95);
      expect(build(schwelle: ' ').body!.containsKey('schwelle'), isFalse);
      expect(build(timeout: '').body!['timeout_s'], 0);
      expect(build(timeout: '1800').error, isNull);
      expect(build(name: 'ä' * 100).error, isNull);
      // gezaehlt in Zeichen wie der Server, nicht in UTF-16-Einheiten
      expect(build(name: '\u{1F600}' * 100).error, isNull);
      expect(build(name: '\u{1F600}' * 101).error, isNotNull);
      expect(build(key: ' sk-erfunden ').body!['api_key'], ' sk-erfunden ');
      expect(build(loeschen: true).body!['api_key_loeschen'], true);
      expect(build().body!.containsKey('api_key'), isFalse);
    });

    test('abgelehnt', () {
      for (final n in ['', '   ', 'ä' * 101]) {
        expect(build(name: n).error, 'Name muss 1 bis 100 Zeichen lang sein',
            reason: n);
      }
      for (final s in ['-0,01', '1,01', 'hoch', 'NaN', '90 %']) {
        expect(build(schwelle: s).error,
            'Schwelle muss eine Zahl zwischen 0 und 1 sein (z. B. 0,9)',
            reason: s);
      }
      for (final t in ['0', '1801', 'x']) {
        expect(build(timeout: t).error,
            'Zeitlimit muss eine Zahl zwischen 1 und 1800 Sekunden sein',
            reason: t);
      }
      expect(build(key: 'x', loeschen: true).error,
          'API-Schlüssel entweder neu setzen oder löschen');
    });
  });

  test('llmSchwelleText', () {
    expect(llmSchwelleText(0.75), '0,75');
    expect(llmSchwelleText(1), '1');
    expect(llmSchwelleText(null), '');
  });

  testWidgets('neues Profil: Standardwerte, Schluessel gesendet',
      (tester) async {
    final api = _FakeApi();
    final result = await _pump(tester, api);
    expect(find.text('Profil anlegen'), findsOneWidget);
    expect(find.text('Gespeicherten API-Schlüssel löschen'), findsNothing);
    await tester.enterText(find.byKey(const ValueKey('profilName')), 'Lokal');
    await tester.enterText(find.byKey(const ValueKey('profilEndpunkt')),
        'http://localhost:11434/v1');
    await tester.enterText(
        find.byKey(const ValueKey('profilApiKey')), 'sk-erfunden-42');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.created.single['name'], 'Lokal');
    expect(api.created.single['aktiv'], false);
    expect(api.created.single['api_key'], 'sk-erfunden-42');
    expect(api.created.single.containsKey('schwelle'), isFalse);
    expect(api.updated, isEmpty);
    expect(result.single!['id'], 'neu-1');
    expect(find.text('Profil anlegen'), findsNothing);
  });

  testWidgets('bearbeiten: Stand ohne Schluessel, behaelt Schluessel',
      (tester) async {
    final api = _FakeApi();
    final result = await _pump(tester, api, profil: _profil);
    expect(find.text('Profil bearbeiten'), findsOneWidget);
    expect(_textField(tester, 'profilName').controller!.text, 'Lokal');
    expect(_textField(tester, 'profilSchwelle').controller!.text, '0,75');
    expect(_textField(tester, 'profilTimeout').controller!.text, '120');
    expect(_textField(tester, 'profilApiKey').controller!.text, '');
    expect(_textField(tester, 'profilApiKey').obscureText, isTrue);
    expect(find.text('Gespeichert – leer lassen, um ihn zu behalten'),
        findsOneWidget);
    await tester.enterText(find.byKey(const ValueKey('profilSchwelle')), '0,6');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    final (id, body) = api.updated.single;
    expect(id, 'p-1');
    expect(body['schwelle'], 0.6);
    expect(body['aktiv'], true);
    expect(body.containsKey('api_key'), isFalse);
    expect(body.containsKey('api_key_loeschen'), isFalse);
    expect(api.created, isEmpty);
    expect(result.single!['schwelle'], 0.6);
  });

  testWidgets('Schluessel loeschen sperrt das Feld', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api, profil: _profil);
    await tester.enterText(
        find.byKey(const ValueKey('profilApiKey')), 'tippfehler');
    await tester.tap(find.text('Gespeicherten API-Schlüssel löschen'));
    await tester.pumpAndSettle();
    expect(_textField(tester, 'profilApiKey').enabled, isFalse);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.$2['api_key_loeschen'], true);
    expect(api.updated.single.$2.containsKey('api_key'), isFalse);
  });

  testWidgets('Fehler: Eingabe und Server, Dialog bleibt offen',
      (tester) async {
    final api = _FakeApi(
        error: const ApiException(
            statusCode: 400,
            message: 'Ein Profil mit diesem Namen ist bereits vorhanden'));
    final result = await _pump(tester, api);
    await tester.enterText(find.byKey(const ValueKey('profilName')), 'X');
    await tester.enterText(find.byKey(const ValueKey('profilSchwelle')), '1,5');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(
        find.text('Schwelle muss eine Zahl zwischen 0 und 1 sein (z. B. 0,9)'),
        findsOneWidget);
    expect(api.created, isEmpty);

    await tester.enterText(find.byKey(const ValueKey('profilSchwelle')), '');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Ein Profil mit diesem Namen ist bereits vorhanden'),
        findsOneWidget);
    expect(find.text('Profil anlegen'), findsOneWidget);
    expect(result, isEmpty);

    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(result, [null]);
  });
}
