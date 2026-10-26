import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/qualification_catalog_page.dart';

// Qualifikationskatalog im Client (Backlog F.13.1). InkRipple und
// Desktop-Fenster wie in den anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, List<Map<String, dynamic>>? types, this.adoptResult = const []})
      : types = types ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> types;
  final List<dynamic> adoptResult;
  final List<String> calls = [];
  final List<Map<String, dynamic>> created = [];
  final List<MapEntry<String, Map<String, dynamic>>> updated = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listQualificationTypes({bool includeInactive = false}) async {
    calls.add('list $includeInactive');
    return types.where((t) => includeInactive || t['aktiv'] != false).toList();
  }

  @override
  Future<Map<String, dynamic>> createQualificationType(Map<String, dynamic> body) async {
    calls.add('create');
    created.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> updateQualificationType(String id, Map<String, dynamic> body) async {
    calls.add('update');
    updated.add(MapEntry(id, body));
    return {'id': id, ...body};
  }

  @override
  Future<List<dynamic>> adoptDefaultQualificationTypes() async {
    calls.add('adopt');
    return adoptResult;
  }
}

final _catalog = [
  {'id': 't1', 'code': 'ARBEITSSCHUTZ', 'name': 'Arbeitsschutz', 'kategorie': 'unterweisung', 'gueltigkeit_monate': 12, 'pflicht_fuer_alle': true, 'aktiv': true, 'hinweis': 'DGUV'},
  {'id': 't2', 'code': 'STAPLER_SCHEIN', 'name': 'Staplerschein', 'kategorie': 'qualifikation', 'gueltigkeit_monate': null, 'pflicht_fuer_alle': false, 'aktiv': true, 'hinweis': ''},
  {'id': 't4', 'code': 'SCHWEISSERPRUEFUNG', 'name': 'Schweißerprüfung', 'kategorie': 'qualifikation', 'gueltigkeit_monate': 36, 'bestaetigung_monate': 6, 'pflicht_fuer_alle': false, 'aktiv': true, 'hinweis': ''},
  {'id': 't3', 'code': 'ALT', 'name': 'Alter Kurs', 'kategorie': 'beauftragung', 'gueltigkeit_monate': 1, 'pflicht_fuer_alle': false, 'aktiv': false, 'hinweis': ''},
];

List<Map<String, dynamic>> _copy() => _catalog.map((t) => Map<String, dynamic>.from(t)).toList();

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'qualifications.read', 'qualifications.write'};

void main() {
  group('reine Funktionen', () {
    test('fristLabel, kategorieLabel', () {
      expect(fristLabel(12), 'alle 12 Monate');
      expect(fristLabel(1), 'jeden Monat');
      expect(fristLabel(null), 'unbefristet');
      expect(kategorieLabel('unterweisung'), 'Unterweisung');
      expect(kategorieLabel('beauftragung'), 'Beauftragung');
    });

    test('parseFrist', () {
      expect(parseFrist(''), (ok: true, months: null));
      expect(parseFrist(' 24 '), (ok: true, months: 24));
      expect(parseFrist('0').ok, isFalse);
      expect(parseFrist('-3').ok, isFalse);
      expect(parseFrist('zwölf').ok, isFalse);
    });

    test('buildQualificationTypeBody: POST mit Code, PUT mit aktiv und allen sieben Feldern', () {
      expect(
        buildQualificationTypeBody(code: ' leiter_pruefer ', name: ' Leiterprüfer ', kategorie: 'beauftragung', pflichtFuerAlle: false, hinweis: ' TRBS '),
        {'code': 'LEITER_PRUEFER', 'name': 'Leiterprüfer', 'kategorie': 'beauftragung', 'gueltigkeit_monate': null, 'bestaetigung_monate': null, 'pflicht_fuer_alle': false, 'hinweis': 'TRBS'},
      );
      final put = buildQualificationTypeBody(name: 'X', kategorie: 'unterweisung', gueltigkeitMonate: 12, bestaetigungMonate: 6, pflichtFuerAlle: true, aktiv: false);
      expect(put.keys.toSet(), {'name', 'kategorie', 'gueltigkeit_monate', 'bestaetigung_monate', 'pflicht_fuer_alle', 'aktiv', 'hinweis'});
      expect(put['aktiv'], isFalse);
      expect(put['gueltigkeit_monate'], 12);
      expect(put['bestaetigung_monate'], 6);
    });

    test('bestaetigungLabel', () {
      expect(bestaetigungLabel(6), 'Bestätigung alle 6 Monate');
      expect(bestaetigungLabel(1), 'Bestätigung jeden Monat');
      expect(bestaetigungLabel(null), isNull);
    });
  });

  testWidgets('zeigt den Katalog; Stillgelegte nur auf Wunsch; ohne Schreibrecht nur lesend', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'qualifications.read'}, types: _copy());
    await tester.pumpWidget(_wrap(QualificationCatalogPage(api: api)));
    await tester.pumpAndSettle();
    expect(api.calls, ['list false']);
    expect(find.text('Arbeitsschutz (ARBEITSSCHUTZ)'), findsOneWidget);
    expect(find.text('Unterweisung · alle 12 Monate · Pflicht für alle'), findsOneWidget);
    expect(find.text('Qualifikation · unbefristet'), findsOneWidget);
    expect(find.text('Qualifikation · alle 36 Monate · Bestätigung alle 6 Monate'), findsOneWidget);
    expect(find.text('Alter Kurs (ALT)'), findsNothing);
    expect(find.byType(FloatingActionButton), findsNothing);
    expect(find.text('Vorschlagskatalog übernehmen'), findsNothing);
    expect(find.byTooltip('Bearbeiten'), findsNothing);

    await tester.tap(find.text('Stillgelegte anzeigen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list true');
    expect(find.text('Beauftragung · jeden Monat · stillgelegt'), findsOneWidget);
  });

  testWidgets('leerer Katalog: Hinweis je nach Recht', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(QualificationCatalogPage(api: _FakeApi(permissions: const {'qualifications.read'}))));
    await tester.pumpAndSettle();
    expect(find.text('Der Katalog ist leer.'), findsOneWidget);
    await tester.pumpWidget(_wrap(QualificationCatalogPage(key: const ValueKey('rw'), api: _FakeApi(permissions: _rw))));
    await tester.pumpAndSettle();
    expect(find.text('Der Katalog ist leer. Legen Sie Arten an oder übernehmen Sie den Vorschlagskatalog.'), findsOneWidget);
  });

  testWidgets('neue Art: Pruefung im Dialog, dann POST', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, types: _copy());
    await tester.pumpWidget(_wrap(QualificationCatalogPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Code angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Code (A-Z, 0-9, _)'), 'schweissen');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Namen angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Name'), 'Schweißerprüfung');
    await tester.enterText(find.widgetWithText(TextField, 'Gültigkeit in Monaten (leer = unbefristet)'), '0');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Gültigkeit als ganze Zahl von Monaten angeben oder leer lassen.'), findsOneWidget);
    expect(api.created, isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Gültigkeit in Monaten (leer = unbefristet)'), '36');
    await tester.enterText(find.widgetWithText(TextField, 'Bestätigung alle … Monate (leer = keine)'), 'sechs');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Bestätigung als ganze Zahl von Monaten angeben oder leer lassen.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Bestätigung alle … Monate (leer = keine)'), '36');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Die Bestätigung muss kürzer als die Gültigkeit sein.'), findsOneWidget);
    expect(api.created, isEmpty);
    await tester.enterText(find.widgetWithText(TextField, 'Bestätigung alle … Monate (leer = keine)'), '6');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Qualifikation').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created.single, {
      'code': 'SCHWEISSEN',
      'name': 'Schweißerprüfung',
      'kategorie': 'qualifikation',
      'gueltigkeit_monate': 36,
      'bestaetigung_monate': 6,
      'pflicht_fuer_alle': false,
      'hinweis': '',
    });
    expect(api.calls.last, 'list false');
  });

  testWidgets('Art bearbeiten: vorbelegt, Code gesperrt, stilllegen und Frist entfernen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, types: _copy());
    await tester.pumpWidget(_wrap(QualificationCatalogPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    final code = tester.widget<TextField>(find.widgetWithText(TextField, 'Code (A-Z, 0-9, _)'));
    expect(code.enabled, isFalse);
    expect(code.controller!.text, 'ARBEITSSCHUTZ');
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Gültigkeit in Monaten (leer = unbefristet)')).controller!.text, '12');

    await tester.enterText(find.widgetWithText(TextField, 'Gültigkeit in Monaten (leer = unbefristet)'), '');
    await tester.tap(find.text('Aktiv'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.key, 't1');
    expect(api.updated.single.value, {
      'name': 'Arbeitsschutz',
      'kategorie': 'unterweisung',
      'gueltigkeit_monate': null,
      'bestaetigung_monate': null,
      'pflicht_fuer_alle': true,
      'aktiv': false,
      'hinweis': 'DGUV',
    });
  });

  testWidgets('Art bearbeiten: Bestaetigung vorbelegt und unveraendert mitgesendet', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, types: _copy());
    await tester.pumpWidget(_wrap(QualificationCatalogPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').at(2));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Bestätigung alle … Monate (leer = keine)')).controller!.text, '6');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.key, 't4');
    expect(api.updated.single.value['bestaetigung_monate'], 6);
    expect(api.updated.single.value['gueltigkeit_monate'], 36);
  });

  testWidgets('Vorschlagskatalog: Rueckfrage, Ergebnis gemeldet, Abbrechen ohne Aufruf', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, adoptResult: [{'id': 'a'}, {'id': 'b'}]);
    await tester.pumpWidget(_wrap(QualificationCatalogPage(api: api)));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Vorschlagskatalog übernehmen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c == 'adopt'), isEmpty);

    await tester.tap(find.text('Vorschlagskatalog übernehmen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Übernehmen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c == 'adopt').length, 1);
    expect(find.text('2 Arten übernommen.'), findsOneWidget);
    expect(api.calls.last, 'list false');

    // Die erste Meldung ablaufen lassen (Snackbars stehen in einer Warteschlange).
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    final empty = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(QualificationCatalogPage(key: const ValueKey('leer'), api: empty)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Vorschlagskatalog übernehmen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Übernehmen'));
    await tester.pumpAndSettle();
    expect(find.text('Alle Vorschläge sind bereits im Katalog.'), findsOneWidget);
  });

  testWidgets('Dashboard: Karte nur mit qualifications.read', (tester) async {
    final api = _FakeApi(permissions: const {});
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'qualifications.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Qualifikationskatalog'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'qualifications.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Qualifikationskatalog'));
    await tester.pumpAndSettle();
    expect(find.byType(QualificationCatalogPage), findsOneWidget);
  });

  test('API: anlegen (201), aendern (PUT), Vorschlaege uebernehmen', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      if (req.url.path.endsWith('adopt-defaults')) {
        return http.Response.bytes(utf8.encode('[{"id":"a"}]'), 200, headers: {'content-type': 'application/json'});
      }
      return http.Response.bytes(utf8.encode('{"id":"x"}'), req.method == 'POST' ? 201 : 200, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.createQualificationType({'code': 'X'});
      await api.updateQualificationType('t1', {'name': 'N'});
      final adopted = await api.adoptDefaultQualificationTypes();
      expect(adopted.length, 1);
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}'), [
      'POST /api/v1/qualification-types/',
      'PUT /api/v1/qualification-types/t1',
      'POST /api/v1/qualification-types/adopt-defaults',
    ]);
    expect(jsonDecode(calls[0].body), {'code': 'X'});
    expect(jsonDecode(calls[1].body), {'name': 'N'});
    for (final c in calls) {
      expect(c.headers['content-type'], startsWith('application/json'));
    }
    final ok200 = MockClient((req) async => http.Response('{"id":"x"}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.createQualificationType({}), () => ok200), throwsA(isA<ApiException>()));
    final err = MockClient((req) async => http.Response.bytes(
        utf8.encode('{"error":{"code":"validation_error","message":"Feld erforderlich: aktiv"}}'), 400,
        headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.updateQualificationType('t1', {}), () => err),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Feld erforderlich: aktiv')));
  });
}
