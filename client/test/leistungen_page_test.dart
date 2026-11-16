import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/leistungen_page.dart';
import 'package:nalaerp_client/web/browser.dart' as browser;

// Leistungskatalog im Client (Backlog I.4.5). Erfundene Leistungen.

class _FakeApi extends ApiClient {
  _FakeApi(
      {required this.permissions,
      List<Map<String, dynamic>>? leistungen,
      this.saveError,
      this.importError})
      : leistungen = leistungen ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> leistungen;
  final ApiException? saveError;
  final ApiException? importError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> created = [];
  final List<MapEntry<String, Map<String, dynamic>>> updated = [];
  final List<String> imported = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listLeistungen(
      {String q = '', bool nurAktive = false}) async {
    calls.add('list "$q" $nurAktive');
    return leistungen
        .where((l) => !nurAktive || l['aktiv'] != false)
        .where((l) =>
            q.isEmpty ||
            '${l['code']} ${l['kurztext']}'
                .toLowerCase()
                .contains(q.toLowerCase()))
        .toList();
  }

  @override
  Future<Map<String, dynamic>> createLeistung(Map<String, dynamic> body) async {
    if (saveError != null) throw saveError!;
    created.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> updateLeistung(
      String id, Map<String, dynamic> body) async {
    if (saveError != null) throw saveError!;
    updated.add(MapEntry(id, body));
    return {'id': id, ...body};
  }

  @override
  Future<Map<String, dynamic>> importLeistungenCsv(
      String filename, Uint8List bytes) async {
    imported.add('$filename ${bytes.length}');
    if (importError != null) throw importError!;
    return {'neu': 3, 'geaendert': 1};
  }
}

final _katalog = [
  {
    'id': 'l1',
    'code': 'FE-ALU-1F',
    'kurztext': 'Aluminiumfenster einflügelig',
    'langtext': 'liefern und montieren',
    'einheit': 'Stk',
    'gewerk': 'fenster',
    'profilserie': 'Serie A75',
    'verglasung': '3-fach',
    'rc_klasse': 'RC2',
    'u_wert': 0.9,
    'brandschutzklasse': '',
    'aktiv': true,
  },
  {
    'id': 'l2',
    'code': 'TU-BS-T30',
    'kurztext': 'Feuerschutztür',
    'langtext': '',
    'einheit': 'Stk',
    'gewerk': '',
    'profilserie': '',
    'verglasung': '',
    'rc_klasse': '',
    'u_wert': null,
    'brandschutzklasse': 'T30',
    'aktiv': true,
  },
  {
    'id': 'l3',
    'code': 'ALT',
    'kurztext': 'Alte Leistung',
    'langtext': '',
    'einheit': 'm',
    'gewerk': '',
    'profilserie': '',
    'verglasung': '',
    'rc_klasse': '',
    'u_wert': null,
    'brandschutzklasse': '',
    'aktiv': false,
  },
];

List<Map<String, dynamic>> _copy() =>
    _katalog.map((l) => Map<String, dynamic>.from(l)).toList();

// Eigener Schluessel: jede Seite bekommt einen frischen ScaffoldMessenger,
// sonst warten Snackbars hinter denen des vorigen Aufbaus.
Widget _wrap(Widget child) => MaterialApp(
      key: UniqueKey(),
      theme:
          ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'quotes.read', 'leistungen.write'};

Future<browser.PickedFile?> Function({String? accept}) _picker(
        browser.PickedFile? file, List<String?> accepts) =>
    ({String? accept}) async {
      accepts.add(accept);
      return file;
    };

void main() {
  group('reine Funktionen', () {
    test('parseUWert und formatUWert', () {
      expect(parseUWert('').wert, isNull);
      expect(parseUWert(' ').ok, isTrue);
      expect(parseUWert('0,9').wert, 0.9);
      expect(parseUWert('1.25').wert, 1.25);
      expect(parseUWert('10').wert, 10);
      for (final bad in ['0', '-1', '10,01', 'x', '1,2,3']) {
        expect(parseUWert(bad).ok, isFalse, reason: bad);
      }
      expect(formatUWert(0.9), '0,9');
      expect(formatUWert(1.25), '1,25');
      expect(formatUWert(1), '1');
      expect(formatUWert(1.1234), '1,123');
      expect(formatUWert(null), '');
      expect(formatUWert('0.9'), '');
    });

    test('buildLeistungBody', () {
      final r = buildLeistungBody(
          code: ' FE-1 ',
          kurztext: ' Fenster ',
          langtext: ' lang ',
          einheit: ' Stk ',
          gewerk: ' fenster ',
          profilserie: ' A75 ',
          verglasung: ' 3-fach ',
          rcKlasse: 'RC2',
          uWert: '0,9',
          brandschutzklasse: ' T30 ');
      expect(r.error, isNull);
      expect(r.body, {
        'code': 'FE-1',
        'kurztext': 'Fenster',
        'langtext': 'lang',
        'einheit': 'Stk',
        'gewerk': 'fenster',
        'profilserie': 'A75',
        'verglasung': '3-fach',
        'rc_klasse': 'RC2',
        'u_wert': 0.9,
        'brandschutzklasse': 'T30',
      });
      expect(
          buildLeistungBody(code: 'X', kurztext: 'k', einheit: 'm')
              .body!['u_wert'],
          isNull);
      expect(
          buildLeistungBody(code: 'X', kurztext: 'k', einheit: 'm')
              .body!
              .containsKey('aktiv'),
          isFalse);
      expect(
          buildLeistungBody(
                  code: 'X', kurztext: 'k', einheit: 'm', aktiv: false)
              .body!['aktiv'],
          false);
      for (final c in [
        (
          buildLeistungBody(code: ' ', kurztext: 'k', einheit: 'm'),
          'Bitte einen Code angeben.'
        ),
        (
          buildLeistungBody(code: 'A B', kurztext: 'k', einheit: 'm'),
          'Der Code darf keine Leerzeichen enthalten.'
        ),
        (
          buildLeistungBody(code: 'X', kurztext: ' ', einheit: 'm'),
          'Bitte einen Kurztext angeben.'
        ),
        (
          buildLeistungBody(code: 'X', kurztext: 'k', einheit: ' '),
          'Bitte eine Einheit angeben.'
        ),
        (
          buildLeistungBody(code: 'X', kurztext: 'k', einheit: 'm', uWert: '0'),
          'Der U-Wert muss eine Zahl über 0 bis 10 sein (z. B. 0,9).'
        ),
      ]) {
        expect(c.$1.error, c.$2);
        expect(c.$1.body, isNull);
      }
    });

    test('leistungDetails', () {
      expect(leistungDetails(_katalog[0]),
          'Stk · fenster · Serie A75 · 3-fach · RC2 · U 0,9');
      expect(leistungDetails(_katalog[1]), 'Stk · T30');
      expect(leistungDetails(_katalog[2]), 'm · abgeschaltet');
    });
  });

  testWidgets('nur lesen: Liste ohne Bearbeiten, Anlegen und Import',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: {'quotes.read'}, leistungen: _copy());
    await tester.pumpWidget(_wrap(LeistungenPage(api: api)));
    await tester.pumpAndSettle();
    expect(api.calls, ['list "" true']);
    expect(
        find.text('FE-ALU-1F – Aluminiumfenster einflügelig'), findsOneWidget);
    expect(find.text('Stk · fenster · Serie A75 · 3-fach · RC2 · U 0,9'),
        findsOneWidget);
    expect(find.text('ALT – Alte Leistung'), findsNothing);
    expect(find.byIcon(Icons.edit), findsNothing);
    expect(find.byType(FloatingActionButton), findsNothing);
    expect(find.text('CSV importieren'), findsNothing);
  });

  testWidgets('Suche und abgeschaltete Leistungen laden neu', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: {'quotes.read'}, leistungen: _copy());
    await tester.pumpWidget(_wrap(LeistungenPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abgeschaltete anzeigen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list "" false');
    expect(find.text('ALT – Alte Leistung'), findsOneWidget);
    expect(find.text('m · abgeschaltet'), findsOneWidget);
    await tester.enterText(find.byKey(const ValueKey('suche')), 'tür');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list "tür" false');
    expect(find.text('TU-BS-T30 – Feuerschutztür'), findsOneWidget);
    expect(find.text('FE-ALU-1F – Aluminiumfenster einflügelig'), findsNothing);
    // leer: Hinweis je nach Recht
    await tester.enterText(find.byKey(const ValueKey('suche')), 'gibtsnicht');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
    expect(find.text('Keine Leistungen.'), findsOneWidget);
  });

  testWidgets('anlegen: Prüfung im Dialog, dann Body mit Systemvorgaben',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(LeistungenPage(api: api)));
    await tester.pumpAndSettle();
    expect(
        find.text(
            'Keine Leistungen. Legen Sie Leistungen an oder importieren Sie eine CSV-Datei.'),
        findsOneWidget);
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();
    expect(find.text('Aktiv'), findsNothing);
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Code angeben.'), findsOneWidget);
    await tester.enterText(find.byKey(const ValueKey('code')), 'FB-ALU');
    await tester.enterText(
        find.byKey(const ValueKey('kurztext')), 'Fensterbank außen');
    await tester.enterText(find.byKey(const ValueKey('einheit')), 'm');
    await tester.enterText(find.byKey(const ValueKey('uwert')), '11');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(
        find.text('Der U-Wert muss eine Zahl über 0 bis 10 sein (z. B. 0,9).'),
        findsOneWidget);
    expect(api.created, isEmpty);
    await tester.enterText(find.byKey(const ValueKey('uwert')), '1,3');
    await tester.enterText(
        find.byKey(const ValueKey('profilserie')), 'Serie T75');
    await tester.tap(find.byKey(const ValueKey('rc')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('RC3').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created.single['code'], 'FB-ALU');
    expect(api.created.single['u_wert'], 1.3);
    expect(api.created.single['rc_klasse'], 'RC3');
    expect(api.created.single['profilserie'], 'Serie T75');
    expect(api.created.single.containsKey('aktiv'), isFalse);
    expect(find.text('Leistung angelegt'), findsOneWidget);
  });

  testWidgets('bearbeiten und abschalten; Fehler des Servers als Meldung',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, leistungen: _copy());
    await tester.pumpWidget(_wrap(LeistungenPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.edit).first);
    await tester.pumpAndSettle();
    expect(find.text('Leistung bearbeiten'), findsOneWidget);
    expect(
        tester
            .widget<TextField>(find.byKey(const ValueKey('uwert')))
            .controller!
            .text,
        '0,9');
    expect(
        tester
            .widget<TextField>(find.byKey(const ValueKey('langtext')))
            .controller!
            .text,
        'liefern und montieren');
    await tester.tap(find.text('Aktiv'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.key, 'l1');
    expect(api.updated.single.value['aktiv'], false);
    expect(api.updated.single.value['rc_klasse'], 'RC2');
    expect(api.updated.single.value['u_wert'], 0.9);
    expect(find.text('Leistung gespeichert'), findsOneWidget);

    final failing = _FakeApi(
        permissions: _rw,
        leistungen: _copy(),
        saveError: const ApiException(
            statusCode: 400, message: 'Code "TU-BS-T30" bereits vorhanden'));
    await tester
        .pumpWidget(_wrap(LeistungenPage(key: UniqueKey(), api: failing)));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.edit).first);
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('code')), 'TU-BS-T30');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Nicht gespeichert: Code "TU-BS-T30" bereits vorhanden'),
        findsOneWidget);
  });

  testWidgets('CSV-Import: abgebrochen, leer, zu groß, Erfolg, abgelehnt',
      (tester) async {
    await _desktop(tester);
    final accepts = <String?>[];
    Future<void> pump(_FakeApi api, browser.PickedFile? file) async {
      await tester.pumpWidget(_wrap(LeistungenPage(
          key: UniqueKey(), api: api, filePicker: _picker(file, accepts))));
      await tester.pumpAndSettle();
      await tester.tap(find.text('CSV importieren'));
      await tester.pumpAndSettle();
    }

    final api = _FakeApi(permissions: _rw, leistungen: _copy());
    await pump(api, null);
    expect(api.imported, isEmpty);
    expect(accepts.single, '.csv,text/csv');

    await pump(api, browser.PickedFile(Uint8List(0), 'leer.csv', 'text/csv'));
    expect(find.text('Die Datei ist leer.'), findsOneWidget);
    await pump(
        api,
        browser.PickedFile(
            Uint8List(maxLeistungenImportBytes + 1), 'gross.csv', 'text/csv'));
    expect(find.text('Die Datei ist zu groß (5,0 MB, höchstens 5 MB).'),
        findsOneWidget);
    expect(api.imported, isEmpty);

    final callsVorher = api.calls.length;
    await pump(
        api,
        browser.PickedFile(
            Uint8List.fromList(utf8.encode('code;kurztext;einheit\nX;x;Stk\n')),
            'katalog.csv',
            'text/csv'));
    expect(api.imported, ['katalog.csv 30']);
    expect(
        find.text('Import abgeschlossen: 3 neu, 1 geändert.'), findsOneWidget);
    expect(api.calls.length,
        callsVorher + 2); // Laden beim Öffnen und nach dem Import

    final abgelehnt = _FakeApi(
        permissions: _rw,
        importError: const ApiException(
            statusCode: 400, message: 'Zeile 4: Kurztext erforderlich'));
    await pump(abgelehnt,
        browser.PickedFile(Uint8List.fromList([1, 2, 3]), 'k.csv', 'text/csv'));
    expect(
        find.text(
            'Import abgelehnt, nichts gespeichert: Zeile 4: Kurztext erforderlich'),
        findsOneWidget);
  });

  testWidgets('Ladefehler wird angezeigt', (tester) async {
    await _desktop(tester);
    final api = _ErrApi();
    await tester.pumpWidget(_wrap(LeistungenPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('Katalog konnte nicht geladen werden: Keine Berechtigung'),
        findsOneWidget);
  });

  testWidgets('Dashboard: Karte nur mit quotes.read', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: {'quotes.read'});
    await tester.pumpWidget(_wrap(DashboardPage(
        api: api,
        onLogout: () async {},
        permissions: const {'sales_orders.read'})));
    await tester.pumpAndSettle();
    expect(find.text('Leistungskatalog'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(
        api: api, onLogout: () async {}, permissions: const {'quotes.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Leistungskatalog'));
    await tester.pumpAndSettle();
    expect(find.byType(LeistungenPage), findsOneWidget);
  });

  test(
      'API: Liste mit Filtern, anlegen (201), ändern (PUT), Import als Upload im Feld file',
      () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.BaseRequest>[];
    final bodies = <String>[];
    final client = MockClient((req) async {
      calls.add(req);
      bodies.add(req.body);
      final path = req.url.path;
      if (req.method == 'GET')
        return http.Response.bytes(utf8.encode('[{"id":"l1"}]'), 200,
            headers: {'content-type': 'application/json'});
      if (path.endsWith('/import'))
        return http.Response.bytes(utf8.encode('{"neu":1,"geaendert":0}'), 200,
            headers: {'content-type': 'application/json'});
      return http.Response.bytes(
          utf8.encode('{"id":"x"}'), req.method == 'POST' ? 201 : 200,
          headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      expect((await api.listLeistungen()).length, 1);
      await api.listLeistungen(q: ' tür ', nurAktive: true);
      await api.createLeistung({'code': 'X'});
      await api.updateLeistung('l1', {'code': 'Y'});
      final r = await api.importLeistungenCsv(
          'k.csv', Uint8List.fromList(utf8.encode('code;kurztext;einheit\n')));
      expect(r['neu'], 1);
    }, () => client);
    expect(calls[0].url.toString(),
        'http://api.test/api/v1/leistungen/?limit=1000');
    expect(calls[1].url.queryParameters,
        {'q': 'tür', 'aktiv': '1', 'limit': '1000'});
    expect(
        '${calls[2].method} ${calls[2].url.path}', 'POST /api/v1/leistungen/');
    expect(jsonDecode(bodies[2]), {'code': 'X'});
    expect(
        '${calls[3].method} ${calls[3].url.path}', 'PUT /api/v1/leistungen/l1');
    expect('${calls[4].method} ${calls[4].url.path}',
        'POST /api/v1/leistungen/import');
    expect(calls[4].headers['content-type'], startsWith('multipart/form-data'));
    expect(bodies[4], contains('name="file"; filename="k.csv"'));
    expect(bodies[4], contains('content-type: text/csv'));

    // Fehlerstatus wird zur ApiException
    final failing = MockClient((req) async => http.Response.bytes(
        utf8.encode(
            '{"error":{"code":"validation_error","message":"Zeile 2: Einheit erforderlich"}}'),
        400,
        headers: {'content-type': 'application/json'}));
    await http.runWithClient(() async {
      await expectLater(
          api.importLeistungenCsv('k.csv', Uint8List(1)),
          throwsA(isA<ApiException>().having(
              (e) => e.message, 'message', 'Zeile 2: Einheit erforderlich')));
      await expectLater(api.createLeistung({}), throwsA(isA<ApiException>()));
      await expectLater(
          api.updateLeistung('l1', {}), throwsA(isA<ApiException>()));
    }, () => failing);
  });
}

class _ErrApi extends ApiClient {
  _ErrApi() : super(baseUrl: 'http://localhost:8080');

  @override
  bool hasPermission(String permission) => permission == 'quotes.read';

  @override
  Future<List<dynamic>> listLeistungen(
          {String q = '', bool nurAktive = false}) async =>
      throw const ApiException(statusCode: 403, message: 'Keine Berechtigung');
}
