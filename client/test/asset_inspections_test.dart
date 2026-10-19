import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/asset_inspections_due_page.dart';
import 'package:nalaerp_client/pages/asset_inspections_section.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';

// Pruefungen von Betriebsmitteln im Client (Backlog F.13.4.2). Feste Uhr;
// InkRipple und Desktop-Fenster wie in den anderen Client-Tests (Backlog
// 0.48).

final _now = DateTime(2026, 10, 19, 10);

class _FakeApi extends ApiClient {
  _FakeApi({
    required this.permissions,
    this.standing = const {'status': 'nie_geprueft'},
    List<Map<String, dynamic>>? inspections,
    this.due = const [],
    this.recordError,
  })  : inspections = inspections ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  Map<String, dynamic> standing;
  final List<Map<String, dynamic>> inspections;
  final List<dynamic> due;
  final Object? recordError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> recorded = [];
  final List<String> stornos = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<Map<String, dynamic>> getAssetInspectionStanding(String assetId) async {
    calls.add('standing $assetId');
    return standing;
  }

  @override
  Future<List<dynamic>> listAssetInspections(String assetId, {bool includeStorniert = false}) async {
    calls.add('list $assetId');
    return inspections;
  }

  @override
  Future<Map<String, dynamic>> recordAssetInspection(Map<String, dynamic> body) async {
    calls.add('record');
    if (recordError != null) throw recordError!;
    recorded.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> stornoAssetInspection(String id, String reason) async {
    stornos.add('$id $reason');
    return {'id': id};
  }

  @override
  Future<List<dynamic>> listAssetInspectionsDue({int? warnDays}) async {
    calls.add('due $warnDays');
    return due;
  }
}

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: Scaffold(body: SingleChildScrollView(child: child)),
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'assets.read', 'assets.write'};

final _history = [
  {'id': 'i2', 'geprueft_am': '2026-09-01T00:00:00Z', 'ergebnis': 'maengel', 'pruefer_name': 'Elektro Meier', 'naechste_pruefung_am': '2027-03-01T00:00:00Z', 'notiz': 'Kabel'},
  {'id': 'i1', 'geprueft_am': '2026-03-01T00:00:00Z', 'ergebnis': 'bestanden', 'pruefer_name': 'Elektro Meier', 'notiz': ''},
];

void main() {
  group('reine Funktionen', () {
    test('pruefstandLabel fuer alle Staende', () {
      expect(pruefstandLabel({'status': 'gesperrt', 'maengel': true}), 'gesperrt – letzte Prüfung mit Mängeln');
      expect(pruefstandLabel({'status': 'gesperrt', 'maengel': false}), 'gesperrt – letzte Prüfung nicht bestanden');
      expect(pruefstandLabel({'status': 'ueberfaellig', 'naechste_pruefung_am': '2026-09-01T00:00:00Z'}), 'Prüfung überfällig seit 01.09.2026');
      expect(pruefstandLabel({'status': 'faellig_bald', 'naechste_pruefung_am': '2026-10-29T00:00:00Z', 'tage_bis_faellig': 10}),
          'Prüfung fällig am 29.10.2026 (in 10 Tagen)');
      expect(pruefstandLabel({'status': 'faellig_bald', 'naechste_pruefung_am': '2026-10-19T00:00:00Z', 'tage_bis_faellig': 0}), 'Prüfung heute fällig');
      expect(pruefstandLabel({'status': 'nie_geprueft'}), 'noch nie geprüft');
      expect(pruefstandLabel({'status': 'nicht_pruefpflichtig'}), 'nicht prüfpflichtig');
      expect(pruefstandLabel({'status': 'geprueft', 'naechste_pruefung_am': '2027-03-01T00:00:00Z'}), 'geprüft, nächste Prüfung am 01.03.2027');
      expect(pruefstandLabel({'status': 'geprueft'}), 'geprüft');
      expect(pruefstandIsBad('gesperrt'), isTrue);
      expect(pruefstandIsBad('faellig_bald'), isFalse);
      expect(pruefstandIsBad('nie_geprueft'), isTrue);
      expect(pruefstandIsBad('ueberfaellig'), isTrue);
      expect(pruefstandIsBad('geprueft'), isFalse);
    });

    test('inspectionLine und buildInspectionBody', () {
      expect(inspectionLine(_history[0]), '01.09.2026 · Mängel · Elektro Meier · nächste 01.03.2027 · Kabel');
      expect(inspectionLine(_history[1]), '01.03.2026 · bestanden · Elektro Meier');
      expect(buildInspectionBody(assetId: 'a1', geprueftAm: '2026-10-19', ergebnis: 'bestanden', prueferName: ' Meier ', notiz: ' x '),
          {'asset_id': 'a1', 'geprueft_am': '2026-10-19', 'ergebnis': 'bestanden', 'pruefer_name': 'Meier', 'notiz': 'x'});
      expect(buildInspectionBody(assetId: 'a1', geprueftAm: 'd', ergebnis: 'maengel', naechstePruefungAm: '2027-01-01', prueferName: 'M')['naechste_pruefung_am'],
          '2027-01-01');
      expect(debugNoKey(buildInspectionBody(assetId: 'a1', geprueftAm: 'd', ergebnis: 'bestanden', naechstePruefungAm: '', prueferName: 'M')), isTrue);
    });
  });

  testWidgets('zeigt Pruefstand und Pruefungen; ohne Schreibrecht nur lesend', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'assets.read'}, standing: {'status': 'gesperrt', 'maengel': true}, inspections: _history);
    await tester.pumpWidget(_wrap(AssetInspectionsSection(api: api, assetId: 'a1', now: () => _now)));
    await tester.pumpAndSettle();
    expect(api.calls, ['standing a1', 'list a1']);
    expect(find.text('Prüfstand: gesperrt – letzte Prüfung mit Mängeln'), findsOneWidget);
    expect(find.text('01.09.2026 · Mängel · Elektro Meier · nächste 01.03.2027 · Kabel'), findsOneWidget);
    expect(find.text('Prüfung erfassen'), findsNothing);
    expect(find.byTooltip('Prüfung stornieren'), findsNothing);
  });

  testWidgets('Pruefung erfassen: Pruefung im Dialog, Hinweis auf Sperre, dann Aufruf und Neuladen', (tester) async {
    await _desktop(tester);
    var changed = 0;
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(AssetInspectionsSection(api: api, assetId: 'a1', now: () => _now, onChanged: () => changed++)));
    await tester.pumpAndSettle();
    expect(find.text('Keine Prüfungen erfasst.'), findsOneWidget);
    await tester.tap(find.text('Prüfung erfassen'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Geprüft am (TT.MM.JJJJ)')).controller!.text, '19.10.2026');

    Finder erfassen() => find.descendant(of: find.byType(AlertDialog), matching: find.widgetWithText(FilledButton, 'Erfassen'));
    await tester.tap(erfassen());
    await tester.pumpAndSettle();
    expect(find.text('Bitte den Prüfer angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Prüfer'), ' Elektro Meier ');
    await tester.enterText(find.widgetWithText(TextField, 'Nächste Prüfung (TT.MM.JJJJ, leer = aus Prüfintervall)'), 'bald');
    await tester.tap(erfassen());
    await tester.pumpAndSettle();
    expect(find.text('Bitte die nächste Prüfung als TT.MM.JJJJ angeben oder leer lassen.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Geprüft am (TT.MM.JJJJ)'), '30.02.2026');
    await tester.tap(erfassen());
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Prüfdatum als TT.MM.JJJJ angeben.'), findsOneWidget);
    expect(api.recorded, isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Geprüft am (TT.MM.JJJJ)'), '18.10.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Nächste Prüfung (TT.MM.JJJJ, leer = aus Prüfintervall)'), '18.04.2027');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Mängel').last);
    await tester.pumpAndSettle();
    expect(find.text('Das Stück ist danach für die Ausgabe gesperrt, bis eine Prüfung bestanden ist.'), findsOneWidget);
    await tester.tap(erfassen());
    await tester.pumpAndSettle();
    expect(api.recorded.single, {
      'asset_id': 'a1',
      'geprueft_am': '2026-10-18',
      'ergebnis': 'maengel',
      'naechste_pruefung_am': '2027-04-18',
      'pruefer_name': 'Elektro Meier',
      'notiz': '',
    });
    expect(api.calls.where((c) => c.startsWith('standing')).length, 2);
    expect(changed, 1);
  });

  testWidgets('nicht bestanden: Folgetermin gesperrt und nicht gesendet', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(AssetInspectionsSection(api: api, assetId: 'a1', now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Prüfung erfassen'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Nächste Prüfung (TT.MM.JJJJ, leer = aus Prüfintervall)'), '01.01.2027');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('nicht bestanden').last);
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Nächste Prüfung (TT.MM.JJJJ, leer = aus Prüfintervall)')).enabled, isFalse);
    await tester.enterText(find.widgetWithText(TextField, 'Prüfer'), 'Meier');
    await tester.tap(find.descendant(of: find.byType(AlertDialog), matching: find.widgetWithText(FilledButton, 'Erfassen')));
    await tester.pumpAndSettle();
    expect(api.recorded.single['ergebnis'], 'nicht_bestanden');
    expect(api.recorded.single.containsKey('naechste_pruefung_am'), isFalse);
  });

  testWidgets('Serverfehler beim Erfassen; ausgemustert ohne Erfassen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, recordError: const ApiException(statusCode: 409, code: 'conflict', message: 'Asset ist ausgemustert'));
    await tester.pumpWidget(_wrap(AssetInspectionsSection(api: api, assetId: 'a1', now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Prüfung erfassen'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Prüfer'), 'Meier');
    await tester.tap(find.descendant(of: find.byType(AlertDialog), matching: find.widgetWithText(FilledButton, 'Erfassen')));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Asset ist ausgemustert'), findsOneWidget);

    await tester.pumpWidget(_wrap(AssetInspectionsSection(key: const ValueKey('weg'), api: _FakeApi(permissions: _rw), assetId: 'a1', retired: true)));
    await tester.pumpAndSettle();
    expect(find.text('Prüfung erfassen'), findsNothing);
  });

  testWidgets('Pruefung stornieren nur mit Begruendung, an der richtigen Pruefung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, inspections: _history);
    await tester.pumpWidget(_wrap(AssetInspectionsSection(api: api, assetId: 'a1', now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Prüfung stornieren').at(0));
    await tester.pumpAndSettle();
    final button = find.widgetWithText(FilledButton, 'Stornieren');
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), ' falsches Gerät ');
    await tester.pumpAndSettle();
    await tester.tap(button);
    await tester.pumpAndSettle();
    expect(api.stornos, ['i2 falsches Gerät']);
    expect(api.calls.where((c) => c.startsWith('standing')).length, 2);
  });

  testWidgets('Faellige Pruefungen: Liste mit Halter, Vorwarnzeit, Leertext', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'assets.read'}, due: [
      {'inventarnummer': 'P-1', 'bezeichnung': 'Auffanggurt', 'standort': 'Lager', 'status': 'gesperrt', 'maengel': false},
      {'inventarnummer': 'E-17', 'bezeichnung': 'Bohrhammer', 'standort': '', 'mitarbeiter_name': 'Anna Berg', 'status': 'ueberfaellig', 'naechste_pruefung_am': '2026-09-01T00:00:00Z'},
    ]);
    await tester.pumpWidget(MaterialApp(theme: ThemeData(splashFactory: InkRipple.splashFactory), home: AssetInspectionsDuePage(api: api)));
    await tester.pumpAndSettle();
    expect(api.calls, ['due 30']);
    expect(find.text('P-1 – Auffanggurt · Lager'), findsOneWidget);
    expect(find.text('gesperrt – letzte Prüfung nicht bestanden'), findsOneWidget);
    expect(find.text('E-17 – Bohrhammer · bei Anna Berg'), findsOneWidget);
    expect(find.text('Prüfung überfällig seit 01.09.2026'), findsOneWidget);

    await tester.enterText(find.widgetWithText(TextField, 'Vorwarnzeit (Tage)'), 'x');
    await tester.tap(find.text('Anwenden'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Vorwarnzeit als ganze Zahl von Tagen angeben (0 oder mehr).'), findsOneWidget);
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Vorwarnzeit (Tage)'), '-1');
    await tester.tap(find.text('Anwenden'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Vorwarnzeit als ganze Zahl von Tagen angeben (0 oder mehr).'), findsOneWidget);
    expect(api.calls, ['due 30']);
    await tester.enterText(find.widgetWithText(TextField, 'Vorwarnzeit (Tage)'), '60');
    await tester.tap(find.text('Anwenden'));
    await tester.pumpAndSettle();
    expect(api.calls, ['due 30', 'due 60']);

    final empty = _FakeApi(permissions: const {'assets.read'});
    await tester.pumpWidget(MaterialApp(key: const ValueKey('leer'), home: AssetInspectionsDuePage(api: empty)));
    await tester.pumpAndSettle();
    expect(find.text('Nichts fällig – alle Prüfungen gelten länger als 30 Tage.'), findsOneWidget);
  });

  testWidgets('Dashboard: "Fällige Prüfungen" nur mit assets.read', (tester) async {
    final api = _FakeApi(permissions: const {});
    await tester.pumpWidget(MaterialApp(home: DashboardPage(api: api, onLogout: () async {}, permissions: const {'assets.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Fällige Prüfungen'), findsNothing);
    await tester.pumpWidget(MaterialApp(
        theme: ThemeData(splashFactory: InkRipple.splashFactory),
        home: DashboardPage(api: api, onLogout: () async {}, permissions: const {'assets.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fällige Prüfungen'));
    await tester.pumpAndSettle();
    expect(find.byType(AssetInspectionsDuePage), findsOneWidget);
  });

  test('API: Stand, Liste, erfassen (201), stornieren, faellige', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final isList = req.url.path == '/api/v1/asset-inspections/' && req.method == 'GET' || req.url.path.endsWith('/due');
      final status = req.method == 'POST' && req.url.path == '/api/v1/asset-inspections/' ? 201 : 200;
      return http.Response.bytes(utf8.encode(isList ? '[]' : '{"status":"geprueft"}'), status, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.getAssetInspectionStanding('a1');
      await api.listAssetInspections('a1');
      await api.listAssetInspections('a1', includeStorniert: true);
      await api.recordAssetInspection({'asset_id': 'a1'});
      await api.stornoAssetInspection('i1', 'falsch');
      await api.listAssetInspectionsDue();
      await api.listAssetInspectionsDue(warnDays: 60);
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
      'GET /api/v1/assets/a1/inspection-standing?',
      'GET /api/v1/asset-inspections/?asset_id=a1',
      'GET /api/v1/asset-inspections/?asset_id=a1&include_storniert=true',
      'POST /api/v1/asset-inspections/?',
      'POST /api/v1/asset-inspections/i1/storno?',
      'GET /api/v1/asset-inspections/due?',
      'GET /api/v1/asset-inspections/due?warn_tage=60',
    ]);
    expect(jsonDecode(calls[3].body), {'asset_id': 'a1'});
    expect(jsonDecode(calls[4].body), {'reason': 'falsch'});
    final ok200 = MockClient((req) async => http.Response('{"id":"x"}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.recordAssetInspection({}), () => ok200), throwsA(isA<ApiException>()));
  });
}

bool debugNoKey(Map<String, dynamic> body) => !body.containsKey('naechste_pruefung_am');
