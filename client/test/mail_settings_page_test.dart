import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/mail_settings_page.dart';

// E-Mail-Versand (Backlog F.14.3). Erfundene Zugangsdaten.

Map<String, dynamic> _stand({
  bool aktiv = false,
  bool gesetzt = true,
  bool lesbar = true,
  bool schluessel = true,
  String sicherheit = 'ssl',
  int port = 465,
}) =>
    {
      'eingerichtet': true,
      'aktiv': aktiv,
      'smtp_host': 'mail.example.test',
      'smtp_port': port,
      'sicherheit': sicherheit,
      'benutzer': 'erp@example.test',
      'absender_adresse': 'erp@example.test',
      'absender_name': 'Nala ERP',
      'passwort_gesetzt': gesetzt,
      'passwort_lesbar': lesbar,
      'schluessel_eingerichtet': schluessel,
    };

class _FakeApi extends ApiClient {
  _FakeApi({Map<String, dynamic>? stand, this.saveError, this.testError, this.umgeleitet = false})
      : stand = stand ?? _stand(),
        super(baseUrl: 'http://localhost:8080');

  Map<String, dynamic> stand;
  final ApiException? saveError;
  final ApiException? testError;
  final bool umgeleitet;
  final List<Map<String, dynamic>> saved = [];
  final List<String> tests = [];

  @override
  Future<Map<String, dynamic>> getMailSettings() async => stand;

  @override
  Future<Map<String, dynamic>> saveMailSettings(Map<String, dynamic> body) async {
    saved.add(body);
    if (saveError != null) throw saveError!;
    final gesetzt = body.containsKey('passwort') || (stand['passwort_gesetzt'] == true && !body.containsKey('passwort_loeschen'));
    stand = {
      ...stand,
      ...Map.of(body)
        ..remove('passwort')
        ..remove('passwort_loeschen'),
      'passwort_gesetzt': gesetzt,
    };
    return stand;
  }

  @override
  Future<List<dynamic>> listNoticeMailLog({int? limit}) async => const [];

  @override
  Future<Map<String, dynamic>> sendMailTest(String an) async {
    tests.add(an);
    if (testError != null) throw testError!;
    return {'gesendet': true, 'umgeleitet': umgeleitet};
  }
}

Future<void> _pump(WidgetTester tester, _FakeApi api) async {
  tester.view.physicalSize = const Size(1200, 2000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(MaterialApp(home: MailSettingsPage(key: UniqueKey(), api: api)));
  await tester.pumpAndSettle();
}

Finder _field(String label) => find.widgetWithText(TextField, label);

void main() {
  group('buildMailSettingsBody', () {
    ({Map<String, dynamic>? body, String? error}) build({String port = '465', String passwort = '', bool loeschen = false}) => buildMailSettingsBody(
          aktiv: true,
          host: ' mail.example.test ',
          sicherheit: 'starttls',
          port: port,
          benutzer: ' erp@example.test ',
          passwort: passwort,
          passwortLoeschen: loeschen,
          absenderAdresse: ' erp@example.test ',
          absenderName: ' ERP ',
        );

    test('ohne Passwort behaelt das gespeicherte', () {
      final r = build();
      expect(r.error, isNull);
      expect(r.body, {
        'aktiv': true,
        'smtp_host': 'mail.example.test',
        'smtp_port': 465,
        'sicherheit': 'starttls',
        'benutzer': 'erp@example.test',
        'absender_adresse': 'erp@example.test',
        'absender_name': 'ERP',
      });
    });

    test('Passwort unveraendert, Loeschen eigens', () {
      expect(build(passwort: ' geheim ').body!['passwort'], ' geheim ');
      expect(build(loeschen: true).body!['passwort_loeschen'], true);
      expect(build(loeschen: true).body!.containsKey('passwort'), isFalse);
      expect(build(port: '').body!['smtp_port'], 0);
    });

    test('abgelehnt', () {
      for (final port in ['abc', '65536', '-1', '4.5']) {
        expect(build(port: port).error, 'Port muss eine Zahl zwischen 1 und 65535 sein', reason: port);
      }
      expect(build(passwort: 'x', loeschen: true).error, 'Passwort entweder neu setzen oder löschen');
      expect(build(port: '65535').error, isNull);
    });
  });

  testWidgets('zeigt den Stand ohne Passwort und speichert ohne Passwort', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(find.text('mail.example.test'), findsOneWidget);
    expect(find.text('Gespeichert – leer lassen, um es zu behalten'), findsOneWidget);
    expect(tester.widget<TextField>(find.byKey(const ValueKey('passwort'))).controller!.text, '');
    expect(find.byIcon(Icons.warning_amber), findsNothing);
    expect(tester.widget<TextField>(find.byKey(const ValueKey('passwort'))).obscureText, isTrue);

    await tester.tap(find.text('E-Mail-Versand eingeschaltet'));
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved, hasLength(1));
    expect(api.saved.single['aktiv'], true);
    expect(api.saved.single.containsKey('passwort'), isFalse);
    expect(find.text('E-Mail-Einstellungen gespeichert'), findsOneWidget);
  });

  testWidgets('neues Passwort wird gesendet und danach geleert', (tester) async {
    final api = _FakeApi(stand: _stand(gesetzt: false));
    await _pump(tester, api);
    expect(find.text('Noch kein Passwort gespeichert'), findsOneWidget);
    expect(find.text('Gespeichertes Passwort löschen'), findsNothing);
    await tester.enterText(find.byKey(const ValueKey('passwort')), 'neu-42');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved.single['passwort'], 'neu-42');
    expect(tester.widget<TextField>(find.byKey(const ValueKey('passwort'))).controller!.text, '');
    expect(find.text('Gespeichert – leer lassen, um es zu behalten'), findsOneWidget);
  });

  testWidgets('Passwort loeschen', (tester) async {
    final api = _FakeApi(stand: _stand(aktiv: true));
    await _pump(tester, api);
    await tester.enterText(find.byKey(const ValueKey('passwort')), 'tippfehler');
    await tester.tap(find.text('Gespeichertes Passwort löschen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved.single['passwort_loeschen'], true);
    expect(api.saved.single['aktiv'], true);
    expect(api.saved.single.containsKey('passwort'), isFalse);
  });

  testWidgets('Verschluesselung nimmt den Standard-Port mit, ein eigener bleibt', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('sicherheit')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('STARTTLS (Port 587)').last);
    await tester.pumpAndSettle();
    expect(find.text('587'), findsOneWidget);
    await tester.enterText(_field('Port'), '2525');
    await tester.tap(find.byKey(const ValueKey('sicherheit')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('SSL/TLS (Port 465)').last);
    await tester.pumpAndSettle();
    expect(find.text('2525'), findsOneWidget);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.saved.single['sicherheit'], 'ssl');
    expect(api.saved.single['smtp_port'], 2525);
  });

  testWidgets('ungueltiger Port und Fehler des Servers: Meldung, Eingaben bleiben', (tester) async {
    final api = _FakeApi(saveError: const ApiException(statusCode: 400, message: 'Für den Versand fehlt: Passwort'));
    await _pump(tester, api);
    await tester.enterText(_field('Port'), 'abc');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Port muss eine Zahl zwischen 1 und 65535 sein'), findsOneWidget);
    expect(api.saved, isEmpty);

    await tester.enterText(_field('Port'), '465');
    await tester.enterText(_field('Mail-Server (z. B. mail.example.de)'), 'anderer.example.test');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Für den Versand fehlt: Passwort'), findsOneWidget);
    expect(find.text('anderer.example.test'), findsOneWidget);
    expect(find.text('E-Mail-Einstellungen gespeichert'), findsNothing);
  });

  testWidgets('Warnungen: kein Schluessel, Passwort nicht lesbar', (tester) async {
    await _pump(tester, _FakeApi(stand: _stand(schluessel: false, gesetzt: false, lesbar: false)));
    expect(find.textContaining('SETTINGS_ENCRYPTION_KEY'), findsOneWidget);
    expect(find.textContaining('nicht mehr entschlüsseln'), findsNothing);

    await _pump(tester, _FakeApi(stand: _stand(lesbar: false)));
    expect(find.textContaining('nicht mehr entschlüsseln'), findsOneWidget);
    expect(find.textContaining('SETTINGS_ENCRYPTION_KEY'), findsNothing);
  });

  testWidgets('Testversand: Pflichtfeld, Erfolg, Umleitung, Fehler', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.text('Testnachricht senden'));
    await tester.pumpAndSettle();
    expect(find.text('Empfänger erforderlich'), findsOneWidget);
    expect(api.tests, isEmpty);

    await tester.enterText(find.byKey(const ValueKey('testAn')), ' test@example.test ');
    await tester.tap(find.text('Testnachricht senden'));
    await tester.pumpAndSettle();
    expect(api.tests, ['test@example.test']);
    expect(find.text('Testnachricht an test@example.test gesendet'), findsOneWidget);

    final redirected = _FakeApi(umgeleitet: true);
    await _pump(tester, redirected);
    await tester.enterText(find.byKey(const ValueKey('testAn')), 'test@example.test');
    await tester.tap(find.text('Testnachricht senden'));
    await tester.pumpAndSettle();
    expect(find.textContaining('umgeleitet an die eingerichtete Entwicklungsadresse'), findsOneWidget);

    final failing = _FakeApi(
        testError:
            const ApiException(statusCode: 502, message: 'Anmeldung am Mail-Server fehlgeschlagen (Benutzer oder Passwort prüfen)', code: 'mail_send_failed'));
    await _pump(tester, failing);
    await tester.enterText(find.byKey(const ValueKey('testAn')), 'test@example.test');
    await tester.tap(find.text('Testnachricht senden'));
    await tester.pumpAndSettle();
    expect(find.text('Testversand fehlgeschlagen: Anmeldung am Mail-Server fehlgeschlagen (Benutzer oder Passwort prüfen)'), findsOneWidget);
  });
}
