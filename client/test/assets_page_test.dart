import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/assets_page.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';

// Betriebsmittel-Bestand im Client (Backlog F.13.3). Feste Uhr; InkRipple
// und Desktop-Fenster wie in den anderen Client-Tests (Backlog 0.48).

final _now = DateTime(2026, 10, 19, 10);

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, List<Map<String, dynamic>>? assets, this.costCenters, this.costCenterError = false})
      : assets = assets ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> assets;
  final List<dynamic>? costCenters;
  final bool costCenterError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> created = [];
  final List<MapEntry<String, Map<String, dynamic>>> updated = [];
  final List<String> retired = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listAssets({String? kategorie, bool includeAusgemustert = false}) async {
    calls.add('list ${kategorie ?? ''} $includeAusgemustert');
    return assets
        .where((a) => (kategorie == null || kategorie.isEmpty || a['kategorie'] == kategorie) && (includeAusgemustert || a['ausgemustert_am'] == null))
        .toList();
  }

  @override
  Future<List<dynamic>> listCostCenters({bool includeInactive = false}) async {
    calls.add('costCenters');
    if (costCenterError) throw const ApiException(statusCode: 403, message: 'Keine Berechtigung');
    return costCenters ?? const [];
  }

  @override
  Future<Map<String, dynamic>> createAsset(Map<String, dynamic> body) async {
    created.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> updateAsset(String id, Map<String, dynamic> body) async {
    updated.add(MapEntry(id, body));
    return {'id': id, ...body};
  }

  @override
  Future<Map<String, dynamic>> retireAsset(String id, {required String datum, required String grund}) async {
    retired.add('$id $datum $grund');
    return {'id': id};
  }
}

Map<String, dynamic> _bohrer() => {
      'id': 'a1',
      'inventarnummer': 'E-17',
      'bezeichnung': 'Bohrhammer',
      'kategorie': 'elektrowerkzeug',
      'seriennummer': 'SN1',
      'hersteller': 'Hilti',
      'modell': 'TE 30',
      'standort': 'Werkstatt',
      'anschaffungsdatum': '2025-03-01T00:00:00Z',
      'anschaffungspreis': 499.9,
      'kostenstelle_id': 'cc1',
      'pruefintervall_monate': 6,
      'notiz': 'Koffer rot',
      'ausgemustert_am': null,
      'ausgemustert_grund': '',
    };

Map<String, dynamic> _leiter() => {
      'id': 'a2',
      'inventarnummer': 'L-1',
      'bezeichnung': 'Leiter',
      'kategorie': 'leiter',
      'standort': '',
      'pruefintervall_monate': null,
      'ausgemustert_am': '2026-01-10T00:00:00Z',
      'ausgemustert_grund': 'Holm gebrochen',
    };

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1100));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'assets.read', 'assets.write'};

void main() {
  group('reine Funktionen', () {
    test('parsePrice', () {
      expect(parsePrice(''), (ok: true, value: null));
      expect(parsePrice('499,9'), (ok: true, value: 499.9));
      expect(parsePrice('1.234,56'), (ok: true, value: 1234.56));
      expect(parsePrice('499.90'), (ok: true, value: 499.9));
      expect(parsePrice(' 12 € '), (ok: true, value: 12.0));
      expect(parsePrice('-5').ok, isFalse);
      expect(parsePrice('1,234').ok, isFalse);
      expect(parsePrice('abc').ok, isFalse);
    });

    test('isoToGerman, formatPrice, intervallLabel', () {
      expect(isoToGerman('2025-03-01T00:00:00Z'), '01.03.2025');
      expect(isoToGerman(null), '');
      expect(formatPrice(499.9), '499,90');
      expect(formatPrice(null), '');
      expect(intervallLabel(6), 'Prüfung alle 6 Monate');
      expect(intervallLabel(1), 'Prüfung jeden Monat');
      expect(intervallLabel(null), 'ohne Prüfintervall');
    });

    test('buildAssetBody: immer alle zwoelf Felder, leere Kostenstelle = null', () {
      final b = buildAssetBody(inventarnummer: ' L-2 ', bezeichnung: ' Leiter ', kategorie: 'leiter', kostenstelleId: '');
      expect(b.keys.toSet(), {
        'inventarnummer', 'bezeichnung', 'kategorie', 'seriennummer', 'hersteller', 'modell', 'standort',
        'anschaffungsdatum', 'anschaffungspreis', 'kostenstelle_id', 'pruefintervall_monate', 'notiz',
      });
      expect(b['inventarnummer'], 'L-2');
      expect(b['kostenstelle_id'], isNull);
      expect(b['pruefintervall_monate'], isNull);
    });
  });

  testWidgets('Liste, Filter, Ausgemusterte; ohne Schreibrecht nur lesend', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'assets.read'}, assets: [_bohrer(), _leiter()]);
    await tester.pumpWidget(_wrap(AssetsPage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    expect(api.calls, ['list  false']);
    expect(find.text('E-17 – Bohrhammer'), findsOneWidget);
    expect(find.text('Elektrowerkzeug · Werkstatt · Prüfung alle 6 Monate'), findsOneWidget);
    expect(find.text('L-1 – Leiter'), findsNothing);
    expect(find.byType(FloatingActionButton), findsNothing);
    expect(find.byTooltip('Bearbeiten'), findsNothing);
    expect(find.byTooltip('Ausmustern'), findsNothing);

    await tester.tap(find.text('Ausgemusterte anzeigen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list  true');
    expect(find.text('Leiter · ohne Prüfintervall · ausgemustert am 10.01.2026 (Holm gebrochen)'), findsOneWidget);

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Leiter').last);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list leiter true');
    expect(find.text('E-17 – Bohrhammer'), findsNothing);
    // Ohne Schreibrecht werden keine Kostenstellen geladen.
    expect(api.calls.where((c) => c == 'costCenters'), isEmpty);
  });

  testWidgets('neues Betriebsmittel: Pruefung im Dialog, dann POST mit allen Feldern', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, costCenters: [
      {'id': 'cc1', 'code': 'W1', 'name': 'Werkstatt'},
    ]);
    await tester.pumpWidget(_wrap(AssetsPage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    expect(find.text('Keine Betriebsmittel gefunden.'), findsOneWidget);
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();

    Future<void> submitExpect(String msg) async {
      await tester.tap(find.text('Anlegen'));
      await tester.pumpAndSettle();
      expect(find.text(msg), findsOneWidget);
    }

    await submitExpect('Bitte eine Inventarnummer angeben.');
    await tester.enterText(find.widgetWithText(TextField, 'Inventarnummer'), ' P-1 ');
    await submitExpect('Bitte eine Bezeichnung angeben.');
    await tester.enterText(find.widgetWithText(TextField, 'Bezeichnung'), 'Auffanggurt');
    await tester.enterText(find.widgetWithText(TextField, 'Anschaffungsdatum (TT.MM.JJJJ, optional)'), '31.02.2026');
    await submitExpect('Bitte das Anschaffungsdatum als TT.MM.JJJJ angeben oder leer lassen.');
    await tester.enterText(find.widgetWithText(TextField, 'Anschaffungsdatum (TT.MM.JJJJ, optional)'), '15.03.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Anschaffungspreis in € (optional)'), 'teuer');
    await submitExpect('Bitte den Preis als Betrag angeben (z. B. 499,90) oder leer lassen.');
    await tester.enterText(find.widgetWithText(TextField, 'Anschaffungspreis in € (optional)'), '1.249,00');
    await tester.enterText(find.widgetWithText(TextField, 'Prüfintervall in Monaten (leer = keine wiederkehrende Prüfung)'), '0');
    await submitExpect('Bitte das Prüfintervall als ganze Zahl von Monaten angeben oder leer lassen.');
    expect(api.created, isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Prüfintervall in Monaten (leer = keine wiederkehrende Prüfung)'), '12');
    await tester.tap(find.widgetWithText(DropdownButtonFormField<String>, 'Werkzeug'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('PSA').last);
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(DropdownButtonFormField<String?>, 'Keine'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('W1 – Werkstatt').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created.single, {
      'inventarnummer': 'P-1',
      'bezeichnung': 'Auffanggurt',
      'kategorie': 'psa',
      'seriennummer': '',
      'hersteller': '',
      'modell': '',
      'standort': '',
      'anschaffungsdatum': '2026-03-15',
      'anschaffungspreis': 1249.0,
      'kostenstelle_id': 'cc1',
      'pruefintervall_monate': 12,
      'notiz': '',
    });
  });

  testWidgets('bearbeiten: vorbelegt, PUT mit allen Feldern, Kostenstelle entfernbar', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, assets: [_bohrer()], costCenters: [
      {'id': 'cc1', 'code': 'W1', 'name': 'Werkstatt'},
    ]);
    await tester.pumpWidget(_wrap(AssetsPage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten'));
    await tester.pumpAndSettle();
    String field(String label) => tester.widget<TextField>(find.widgetWithText(TextField, label)).controller!.text;
    expect(field('Inventarnummer'), 'E-17');
    expect(field('Anschaffungsdatum (TT.MM.JJJJ, optional)'), '01.03.2025');
    expect(field('Anschaffungspreis in € (optional)'), '499,90');
    expect(field('Prüfintervall in Monaten (leer = keine wiederkehrende Prüfung)'), '6');

    await tester.enterText(find.widgetWithText(TextField, 'Prüfintervall in Monaten (leer = keine wiederkehrende Prüfung)'), '3');
    await tester.tap(find.widgetWithText(DropdownButtonFormField<String?>, 'W1 – Werkstatt'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Keine').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.key, 'a1');
    expect(api.updated.single.value, {
      'inventarnummer': 'E-17',
      'bezeichnung': 'Bohrhammer',
      'kategorie': 'elektrowerkzeug',
      'seriennummer': 'SN1',
      'hersteller': 'Hilti',
      'modell': 'TE 30',
      'standort': 'Werkstatt',
      'anschaffungsdatum': '2025-03-01',
      'anschaffungspreis': 499.9,
      'kostenstelle_id': null,
      'pruefintervall_monate': 3,
      'notiz': 'Koffer rot',
    });
  });

  testWidgets('ohne Kostenstellen-Recht bleibt die bestehende Zuordnung beim Speichern erhalten', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, assets: [_bohrer()], costCenterError: true);
    await tester.pumpWidget(_wrap(AssetsPage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten'));
    await tester.pumpAndSettle();
    expect(find.text('Kostenstelle zugeordnet – sie bleibt beim Speichern erhalten.'), findsOneWidget);
    expect(find.byType(DropdownButtonFormField<String?>), findsNothing);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.value['kostenstelle_id'], 'cc1');
  });

  testWidgets('ausmustern: Datum vorbelegt, Grund Pflicht; ausgemusterte ohne Ausmustern-Button', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, assets: [_bohrer(), _leiter()]);
    await tester.pumpWidget(_wrap(AssetsPage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Ausgemusterte anzeigen'));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Ausmustern'), findsOneWidget);
    expect(find.byTooltip('Bearbeiten'), findsNWidgets(2));

    await tester.tap(find.byTooltip('Ausmustern'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)')).controller!.text, '19.10.2026');
    await tester.tap(find.widgetWithText(FilledButton, 'Ausmustern'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Grund angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)'), '18.10.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Grund'), ' Kabel defekt ');
    await tester.tap(find.widgetWithText(FilledButton, 'Ausmustern'));
    await tester.pumpAndSettle();
    expect(api.retired, ['a1 2026-10-18 Kabel defekt']);
  });

  testWidgets('Dashboard: Karte nur mit assets.read', (tester) async {
    final api = _FakeApi(permissions: const {});
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'assets.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Betriebsmittel'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'assets.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Betriebsmittel'));
    await tester.pumpAndSettle();
    expect(find.byType(AssetsPage), findsOneWidget);
  });

  test('API: Bestand, anlegen, aendern, ausmustern, Kostenstellen', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final isList = req.method == 'GET';
      final status = req.method == 'POST' && req.url.path == '/api/v1/assets/' ? 201 : 200;
      return http.Response.bytes(utf8.encode(isList ? '[]' : '{"id":"x"}'), status, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.listAssets();
      await api.listAssets(kategorie: 'leiter', includeAusgemustert: true);
      await api.createAsset({'inventarnummer': 'X'});
      await api.updateAsset('a1', {'inventarnummer': 'Y'});
      await api.retireAsset('a1', datum: '2026-10-18', grund: 'kaputt');
      await api.listCostCenters();
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
      'GET /api/v1/assets/?',
      'GET /api/v1/assets/?kategorie=leiter&include_ausgemustert=true',
      'POST /api/v1/assets/?',
      'PUT /api/v1/assets/a1?',
      'POST /api/v1/assets/a1/retire?',
      'GET /api/v1/cost-centers/?',
    ]);
    expect(jsonDecode(calls[2].body), {'inventarnummer': 'X'});
    expect(jsonDecode(calls[3].body), {'inventarnummer': 'Y'});
    expect(jsonDecode(calls[4].body), {'datum': '2026-10-18', 'grund': 'kaputt'});
    final ok200 = MockClient((req) async => http.Response('{"id":"x"}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.createAsset({}), () => ok200), throwsA(isA<ApiException>()));
    final err = MockClient((req) async => http.Response.bytes(
        utf8.encode('{"error":{"code":"conflict","message":"Asset ist noch ausgegeben - erst zurücknehmen"}}'), 409,
        headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.retireAsset('a1', datum: 'd', grund: 'g'), () => err),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Asset ist noch ausgegeben - erst zurücknehmen')));
  });
}
