import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/vehicle_costs_section.dart';
import 'package:nalaerp_client/pages/vehicle_detail_page.dart';

// Fahrzeugkosten im Client (Backlog G.3d.3). Erfundene Daten.

class _FakeApi extends ApiClient {
  _FakeApi(
      {required this.permissions,
      List<Map<String, dynamic>>? costs,
      this.recordError})
      : costs = costs ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> costs;
  final Object? recordError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> recorded = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listFleetCosts(
      {String? vehicleId,
      String? kostenart,
      String? von,
      String? bis,
      bool includeStorniert = false}) async {
    calls.add('list $vehicleId $includeStorniert');
    return List.of(costs);
  }

  @override
  Future<Map<String, dynamic>> getFleetCostSummary(
      {String? vehicleId, String? kostenart, String? von, String? bis}) async {
    calls.add('summary $vehicleId');
    final byArt = <String, Map<String, dynamic>>{};
    var cents = 0;
    for (final c in costs.where((c) => c['status'] == 'active')) {
      final s = byArt.putIfAbsent(
          c['kostenart'] as String,
          () =>
              {'kostenart': c['kostenart'], 'anzahl': 0, 'betrag_netto': 0.0});
      s['anzahl'] = (s['anzahl'] as int) + 1;
      s['betrag_netto'] =
          (s['betrag_netto'] as double) + (c['betrag_netto'] as num);
      cents += ((c['betrag_netto'] as num) * 100).round();
    }
    return {'summen': byArt.values.toList(), 'gesamt': cents / 100};
  }

  @override
  Future<List<dynamic>> listCostCenters({bool includeInactive = false}) async =>
      [
        {'id': 'cc1', 'code': '100', 'name': 'Montage'},
      ];

  @override
  Future<Map<String, dynamic>> recordFleetCost(
      Map<String, dynamic> body) async {
    recorded.add(Map.of(body));
    if (recordError != null) throw recordError!;
    costs.insert(0, {
      ...body,
      'id': 'neu',
      'datum': '${body['datum']}T00:00:00Z',
      'status': 'active'
    });
    return {'id': 'neu'};
  }

  @override
  Future<Map<String, dynamic>> stornoFleetCost(String id, String reason) async {
    calls.add('storno $id $reason');
    final c = costs.firstWhere((c) => c['id'] == id);
    c['status'] = 'storniert';
    c['storno_reason'] = reason;
    return {'id': id};
  }

  @override
  Future<List<dynamic>> listFleetAssignments(
          {String? vehicleId,
          String? employeeId,
          bool nurOffen = false,
          bool includeStorniert = false}) async =>
      const [];

  @override
  Future<List<dynamic>> listVehicleSchedules(String vehicleId,
          {bool includeInactive = false}) async =>
      const [];
}

List<Map<String, dynamic>> _costs() => [
      {
        'id': 'k2', 'datum': '2026-10-05T00:00:00Z', 'kostenart': 'reparatur',
        'bezeichnung': '', 'betrag_netto': 300, //
        'lieferant': 'Werkstatt X', 'belegnummer': 'R-7', 'status': 'active',
      },
      {
        'id': 'k1', 'datum': '2026-10-02T00:00:00Z', 'kostenart': 'kraftstoff',
        'bezeichnung': '', 'betrag_netto': 80.12, //
        'liter': 45.5, 'km_stand': 12000, 'lieferant': 'Tanke',
        'belegnummer': '', 'status': 'active',
      },
    ];

Widget _section(_FakeApi api) => MaterialApp(
      theme:
          ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: Scaffold(
          body: SingleChildScrollView(
              child: VehicleCostsSection(
                  api: api,
                  vehicleId: 'v1',
                  now: () => DateTime(2026, 10, 19, 10)))),
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1400));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

Finder _dlg(String text) => find.descendant(
    of: find.byType(AlertDialog),
    matching: find.widgetWithText(FilledButton, text));
Finder _dlgField(String label) => find.descendant(
    of: find.byType(AlertDialog),
    matching: find.widgetWithText(TextField, label));

void main() {
  group('buildCostBody', () {
    ({Map<String, dynamic>? body, String? error}) build({
      String datum = '02.10.2026',
      String kostenart = 'kraftstoff',
      String betrag = '1.234,56',
      String bezeichnung = '',
      String km = '',
      String liter = '',
      String? kostenstelleId,
    }) =>
        buildCostBody(
          vehicleId: 'v1',
          datum: datum,
          kostenart: kostenart,
          betrag: betrag,
          bezeichnung: bezeichnung,
          lieferant: ' Tanke ',
          belegnummer: ' B-1 ',
          kmStand: km,
          liter: liter,
          kostenstelleId: kostenstelleId,
          notiz: ' n ',
        );

    test('gueltig', () {
      expect(build(km: ' 12000 ', liter: '45,5', kostenstelleId: 'cc1').body, {
        'vehicle_id': 'v1',
        'datum': '2026-10-02',
        'kostenart': 'kraftstoff',
        'bezeichnung': '',
        'betrag_netto': 1234.56,
        'lieferant': 'Tanke',
        'belegnummer': 'B-1',
        'km_stand': 12000,
        'liter': 45.5,
        'kostenstelle_id': 'cc1',
        'notiz': 'n',
      });
      final plain = build(kostenart: 'wartung', betrag: '0,01').body!;
      expect(plain['betrag_netto'], 0.01);
      expect(plain.containsKey('km_stand'), isFalse);
      expect(plain.containsKey('liter'), isFalse);
      expect(plain.containsKey('kostenstelle_id'), isFalse);
      expect(
          build(kostenart: 'sonstiges', bezeichnung: ' Vignette ')
              .body!['bezeichnung'],
          'Vignette');
      expect(build(km: '0').body!['km_stand'], 0);
    });

    test('abgelehnt', () {
      final cases = [
        (build(datum: '2.10.26'), 'Bitte das Datum als TT.MM.JJJJ angeben.'),
        (build(kostenart: 'bussgeld'), 'Bitte eine Kostenart wählen.'),
        (
          build(kostenart: 'sonstiges', bezeichnung: ' '),
          'Bezeichnung ist bei sonstigen Kosten erforderlich.'
        ),
        (
          build(betrag: ''),
          'Betrag als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'
        ),
        (
          build(betrag: '0'),
          'Betrag als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'
        ),
        (
          build(betrag: '1,234'),
          'Betrag als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'
        ),
        (
          build(betrag: 'abc'),
          'Betrag als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'
        ),
        (build(km: '-1'), 'km-Stand muss eine ganze Zahl ab 0 sein.'),
        (build(km: 'x'), 'km-Stand muss eine ganze Zahl ab 0 sein.'),
        (build(kostenart: 'wartung', liter: '5'), 'Liter nur bei Kraftstoff.'),
        (
          build(liter: '0'),
          'Liter als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'
        ),
        (
          build(liter: '1,005'),
          'Liter als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'
        ),
      ];
      for (final (r, msg) in cases) {
        expect(r.error, msg);
        expect(r.body, isNull);
      }
    });
  });

  test('costLine', () {
    final c = _costs();
    expect(costLine(c[1]),
        '02.10.2026 · Kraftstoff · 80,12 € · 45,50 l · km 12000 · Tanke');
    expect(costLine(c[0]),
        '05.10.2026 · Reparatur · 300,00 € · Werkstatt X · Beleg R-7');
    expect(
        costLine({
          ...c[0],
          'kostenart': 'sonstiges',
          'bezeichnung': 'Vignette',
          'status': 'storniert',
          'storno_reason': 'doppelt'
        }),
        'STORNIERT: 05.10.2026 · Sonstiges: Vignette · 300,00 € · Werkstatt X · Beleg R-7 (doppelt)');
  });

  testWidgets('nur lesen: Summen und Belege, keine Aktionen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, costs: _costs());
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(api.calls, ['list v1 true', 'summary v1']);
    expect(find.text('Gesamt netto: 380,12 €'), findsOneWidget);
    expect(find.text('Reparatur: 300,00 € (1 Belege)'), findsOneWidget);
    expect(find.text('Kraftstoff: 80,12 € (1 Belege)'), findsOneWidget);
    expect(find.textContaining('Werkstatt X'), findsOneWidget);
    expect(find.text('Kosten erfassen'), findsNothing);
    expect(find.byTooltip('Kosten stornieren'), findsNothing);
  });

  testWidgets(
      'erfassen: Pruefung im Dialog, Liter nur bei Kraftstoff, Kostenstelle',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read', 'fleet.write'});
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(find.text('Noch keine Kosten erfasst'), findsOneWidget);
    await tester.tap(find.text('Kosten erfassen'));
    await tester.pumpAndSettle();
    expect(
        tester
            .widget<TextField>(_dlgField('Belegdatum (TT.MM.JJJJ)'))
            .controller!
            .text,
        '19.10.2026');
    expect(_dlgField('Liter (optional)'), findsOneWidget);
    await tester.enterText(_dlgField('Liter (optional)'), '40');
    await tester.tap(_dlg('Erfassen'));
    await tester.pumpAndSettle();
    expect(
        find.text(
            'Betrag als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'),
        findsOneWidget);

    // Wechsel auf Wartung: Literfeld weg, eingegebene Liter werden nicht gesendet.
    await tester.tap(find.byKey(const ValueKey('costKostenart')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Wartung').last);
    await tester.pumpAndSettle();
    expect(_dlgField('Liter (optional)'), findsNothing);
    await tester.enterText(_dlgField('Betrag netto (€)'), '199,90');
    await tester.tap(find.byKey(const ValueKey('costKostenstelle')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('100 – Montage').last);
    await tester.pumpAndSettle();
    await tester.tap(_dlg('Erfassen'));
    await tester.pumpAndSettle();
    expect(api.recorded.single['kostenart'], 'wartung');
    expect(api.recorded.single['betrag_netto'], 199.9);
    expect(api.recorded.single['datum'], '2026-10-19');
    expect(api.recorded.single['kostenstelle_id'], 'cc1');
    expect(api.recorded.single.containsKey('liter'), isFalse);
    expect(find.byType(AlertDialog), findsNothing);
    expect(find.text('Kosten erfasst'), findsOneWidget);
    expect(find.text('Gesamt netto: 199,90 €'), findsOneWidget);
  });

  testWidgets('Ablehnung des Servers erscheint als Meldung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
      permissions: const {'fleet.read', 'fleet.write'},
      recordError: const ApiException(
          statusCode: 400, message: 'Datum darf nicht in der Zukunft liegen'),
    );
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Kosten erfassen'));
    await tester.pumpAndSettle();
    await tester.enterText(_dlgField('Betrag netto (€)'), '10');
    await tester.tap(_dlg('Erfassen'));
    await tester.pumpAndSettle();
    expect(api.recorded.single.containsKey('kostenstelle_id'), isFalse);
    expect(find.text('Fehler: Datum darf nicht in der Zukunft liegen'),
        findsOneWidget);
    expect(find.text('Noch keine Kosten erfasst'), findsOneWidget);
  });

  testWidgets('stornieren: Begruendung Pflicht, danach als storniert gezeigt',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
        permissions: const {'fleet.read', 'fleet.write'}, costs: _costs());
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Kosten stornieren'), findsNWidgets(2));
    await tester.tap(find.byTooltip('Kosten stornieren').first);
    await tester.pumpAndSettle();
    expect(tester.widget<FilledButton>(_dlg('Stornieren')).onPressed, isNull);
    await tester.enterText(
        find.descendant(
            of: find.byType(AlertDialog), matching: find.byType(TextField)),
        ' doppelt ');
    await tester.pump();
    await tester.tap(_dlg('Stornieren'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('storno k2 doppelt'));
    expect(find.text('Kosten storniert'), findsOneWidget);
    expect(find.textContaining('STORNIERT: 05.10.2026'), findsOneWidget);
    expect(find.byTooltip('Kosten stornieren'), findsOneWidget);
    expect(find.text('Gesamt netto: 80,12 €'), findsOneWidget);
  });

  testWidgets('Detailseite zeigt den Abschnitt Kosten', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, costs: _costs());
    await tester.pumpWidget(MaterialApp(
      home: VehicleDetailPage(api: api, vehicle: const {
        'id': 'v1',
        'kennzeichen': 'M-K 1',
        'art': 'pkw',
        'bezeichnung': '',
        'abgemeldet_am': null
      }),
    ));
    await tester.pumpAndSettle();
    await tester.scrollUntilVisible(find.text('Gesamt netto: 380,12 €'), 200);
    expect(find.text('Kosten'), findsOneWidget);
    expect(api.calls, contains('summary v1'));
  });

  test('API: Pfade, Methoden, Bodies; Fehler werden zur ApiException',
      () async {
    final calls = <http.Request>[];
    final api = ApiClient(baseUrl: 'http://api.test');
    Future<T> mock<T>(Object body, Future<T> Function() call,
        {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(utf8.encode(jsonEncode(body)), status,
            headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    await mock(
        [],
        () => api.listFleetCosts(
            vehicleId: 'v1',
            kostenart: 'wartung',
            von: '2026-10-01',
            bis: '2026-10-31',
            includeStorniert: true));
    expect(calls.last.method, 'GET');
    expect(calls.last.url.path, '/api/v1/fleet/costs/');
    expect(calls.last.url.queryParameters, {
      'vehicle_id': 'v1',
      'kostenart': 'wartung',
      'von': '2026-10-01',
      'bis': '2026-10-31',
      'include_storniert': 'true'
    });
    await mock([], () => api.listFleetCosts());
    expect(calls.last.url.queryParameters, isEmpty);

    final sum = await mock({'summen': [], 'gesamt': 0},
        () => api.getFleetCostSummary(vehicleId: 'v1', von: '2026-01-01'));
    expect(sum['gesamt'], 0);
    expect(calls.last.url.path, '/api/v1/fleet/costs/summary');
    expect(calls.last.url.queryParameters,
        {'vehicle_id': 'v1', 'von': '2026-01-01'});

    await mock({'id': 'k1'}, () => api.recordFleetCost({'vehicle_id': 'v1'}),
        status: 201);
    expect(calls.last.method, 'POST');
    expect(calls.last.url.path, '/api/v1/fleet/costs/');
    expect(jsonDecode(calls.last.body), {'vehicle_id': 'v1'});
    await expectLater(mock({'id': 'k1'}, () => api.recordFleetCost(const {})),
        throwsA(isA<ApiException>()));

    await mock({'id': 'k1'}, () => api.stornoFleetCost('k1', 'falsch'));
    expect(calls.last.method, 'POST');
    expect(calls.last.url.path, '/api/v1/fleet/costs/k1/storno');
    expect(jsonDecode(calls.last.body), {'reason': 'falsch'});
    await expectLater(
        mock({
          'error': {'code': 'conflict', 'message': 'x'}
        }, () => api.stornoFleetCost('k1', 'x'), status: 409),
        throwsA(isA<ApiException>()));
  });
}
