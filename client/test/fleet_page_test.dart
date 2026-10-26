import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/fleet_page.dart';

// Fuhrpark im Client (Backlog G.1.4). InkRipple und Desktop-Fenster wie
// in den anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, List<Map<String, dynamic>>? vehicles, this.costCenters, this.saveError})
      : vehicles = vehicles ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> vehicles;
  // null = Kostenstellen nicht ladbar (ohne cost_centers.read).
  final List<dynamic>? costCenters;
  final Object? saveError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> created = [];
  final List<MapEntry<String, Map<String, dynamic>>> updated = [];
  final List<List<String>> deregistered = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listVehicles({String? art, bool includeAbgemeldete = false}) async {
    calls.add('list ${art ?? ''} $includeAbgemeldete');
    // Wie der Server: Filter Art, abgemeldete nur auf Wunsch.
    return vehicles
        .where((v) => (art == null || art.isEmpty || v['art'] == art) && (includeAbgemeldete || v['abgemeldet_am'] == null))
        .toList();
  }

  @override
  Future<List<dynamic>> listCostCenters({bool includeInactive = false}) async {
    if (costCenters == null) throw const ApiException(statusCode: 403, message: 'Keine Berechtigung');
    return costCenters!;
  }

  @override
  Future<Map<String, dynamic>> createVehicle(Map<String, dynamic> body) async {
    if (saveError != null) throw saveError!;
    created.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> updateVehicle(String id, Map<String, dynamic> body) async {
    if (saveError != null) throw saveError!;
    updated.add(MapEntry(id, body));
    return {'id': id, ...body};
  }

  @override
  Future<Map<String, dynamic>> deregisterVehicle(String id, {required String datum, required String grund}) async {
    deregistered.add([id, datum, grund]);
    return {'id': id};
  }
}

List<Map<String, dynamic>> _vehicles() => [
      {
        'id': 'v1', 'kennzeichen': 'M-AB 1234', 'art': 'transporter', 'bezeichnung': 'Montagebus', 'hersteller': 'VW', 'modell': 'Crafter',
        'fin': 'WVWZZZ1JZ3W386752', 'erstzulassung': '2021-03-15T00:00:00Z', 'zulaessige_gesamtmasse_kg': 3500, 'standort': 'Hof',
        'kostenstelle_id': 'cc1', 'fahrtenbuch': true, 'notiz': 'Dachträger', 'abgemeldet_am': null, 'abgemeldet_grund': '',
      },
      {
        'id': 'v2', 'kennzeichen': 'M-AB 99', 'art': 'anhaenger', 'bezeichnung': '', 'hersteller': '', 'modell': '', 'fin': null,
        'erstzulassung': null, 'zulaessige_gesamtmasse_kg': null, 'standort': '', 'kostenstelle_id': null, 'fahrtenbuch': false, 'notiz': '',
        'abgemeldet_am': null, 'abgemeldet_grund': '',
      },
      {
        'id': 'v3', 'kennzeichen': 'M-X 1', 'art': 'pkw', 'bezeichnung': 'Alt', 'hersteller': '', 'modell': '', 'fin': null,
        'erstzulassung': null, 'zulaessige_gesamtmasse_kg': null, 'standort': '', 'kostenstelle_id': null, 'fahrtenbuch': false, 'notiz': '',
        'abgemeldet_am': '2026-01-15T00:00:00Z', 'abgemeldet_grund': 'verkauft',
      },
    ];

const _cc = [
  {'id': 'cc1', 'code': '4100', 'name': 'Montage'},
  {'id': 'cc2', 'code': '4200', 'name': 'Werkstatt'},
];

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'fleet.read', 'fleet.write'};

void main() {
  group('reine Funktionen', () {
    test('parseMasse', () {
      expect(parseMasse(''), (ok: true, kg: null));
      expect(parseMasse(' 3500 '), (ok: true, kg: 3500));
      expect(parseMasse('3.500'), (ok: true, kg: 3500));
      expect(parseMasse('0'), (ok: false, kg: null));
      expect(parseMasse('-1'), (ok: false, kg: null));
      expect(parseMasse('3,5'), (ok: false, kg: null));
    });

    test('buildVehicleBody: immer alle zwölf Felder, leere FIN und Kostenstelle als null', () {
      final b = buildVehicleBody(kennzeichen: ' M-AB 1 ', art: 'pkw', fin: ' wvwzzz1jz3w386752 ', kostenstelleId: 'cc1', fahrtenbuch: true, notiz: ' x ');
      expect(b.keys.toSet(), {
        'kennzeichen', 'art', 'bezeichnung', 'hersteller', 'modell', 'fin', 'erstzulassung', 'zulaessige_gesamtmasse_kg', 'standort', 'kostenstelle_id', 'fahrtenbuch', 'notiz',
      });
      expect(b['kennzeichen'], 'M-AB 1');
      expect(b['fin'], 'WVWZZZ1JZ3W386752');
      expect(b['kostenstelle_id'], 'cc1');
      expect(b['fahrtenbuch'], isTrue);
      expect(b['notiz'], 'x');
      final empty = buildVehicleBody(kennzeichen: 'A', art: 'lkw', fin: ' ', kostenstelleId: '');
      expect(empty['fin'], isNull);
      expect(empty['kostenstelle_id'], isNull);
      expect(empty['fahrtenbuch'], isFalse);
      expect(empty.containsKey('erstzulassung') && empty['erstzulassung'] == null, isTrue);
    });

    test('vehicleArtLabel und vehicleDetails', () {
      expect(vehicleArtLabel('anhaenger'), 'Anhänger');
      expect(vehicleArtLabel('x'), 'x');
      final v = _vehicles();
      expect(vehicleDetails(v[0]), 'Transporter · VW Crafter · Erstzulassung 15.03.2021 · Fahrtenbuch');
      expect(vehicleDetails(v[1]), 'Anhänger');
      expect(vehicleDetails(v[2]), 'Pkw · abgemeldet am 15.01.2026 (verkauft)');
    });
  });

  testWidgets('listet aktive Fahrzeuge; Filter Art und Abgemeldete; ohne Schreibrecht nur lesen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, vehicles: _vehicles());
    await tester.pumpWidget(_wrap(FleetPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('M-AB 1234 – Montagebus'), findsOneWidget);
    expect(find.text('M-AB 99'), findsOneWidget);
    expect(find.textContaining('M-X 1'), findsNothing);
    expect(find.byTooltip('Bearbeiten'), findsNothing);
    expect(find.byType(FloatingActionButton), findsNothing);

    await tester.tap(find.text('Abgemeldete anzeigen'));
    await tester.pumpAndSettle();
    expect(find.text('M-X 1 – Alt'), findsOneWidget);
    expect(api.calls.last, 'list  true');

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anhänger').last);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list anhaenger true');
    expect(find.text('M-AB 99'), findsOneWidget);
    expect(find.textContaining('M-AB 1234'), findsNothing);
  });

  testWidgets('leer', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(FleetPage(api: _FakeApi(permissions: _rw, costCenters: const []))));
    await tester.pumpAndSettle();
    expect(find.text('Keine Fahrzeuge gefunden.'), findsOneWidget);
  });

  testWidgets('legt ein Fahrzeug an; ungültige Eingaben bleiben im Dialog', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, costCenters: _cc);
    await tester.pumpWidget(_wrap(FleetPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte ein Kennzeichen angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Kennzeichen (z. B. M-AB 1234)'), 'm-ab 5');
    await tester.enterText(find.widgetWithText(TextField, 'Erstzulassung (TT.MM.JJJJ, optional)'), '31.02.2021');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Erstzulassung als TT.MM.JJJJ angeben oder leer lassen.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Erstzulassung (TT.MM.JJJJ, optional)'), '15.03.2021');
    await tester.enterText(find.widgetWithText(TextField, 'Zulässige Gesamtmasse in kg (optional)'), 'viel');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Gesamtmasse als ganze Zahl in kg angeben oder leer lassen.'), findsOneWidget);
    expect(api.created, isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Zulässige Gesamtmasse in kg (optional)'), '3.500');
    await tester.tap(find.widgetWithText(DropdownButtonFormField<String>, 'Pkw').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Transporter').last);
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(DropdownButtonFormField<String?>, 'Keine'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('4200 – Werkstatt').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fahrtenbuch führen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created.single, {
      'kennzeichen': 'm-ab 5',
      'art': 'transporter',
      'bezeichnung': '',
      'hersteller': '',
      'modell': '',
      'fin': null,
      'erstzulassung': '2021-03-15',
      'zulaessige_gesamtmasse_kg': 3500,
      'standort': '',
      'kostenstelle_id': 'cc2',
      'fahrtenbuch': true,
      'notiz': '',
    });
    expect(find.text('Fahrzeug angelegt'), findsOneWidget);
  });

  testWidgets('bearbeitet mit allen Werten vorbelegt', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, vehicles: _vehicles(), costCenters: _cc);
    await tester.pumpWidget(_wrap(FleetPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    expect(find.text('Fahrzeug bearbeiten'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Notiz'), 'neu');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.key, 'v1');
    expect(api.updated.single.value, {
      'kennzeichen': 'M-AB 1234',
      'art': 'transporter',
      'bezeichnung': 'Montagebus',
      'hersteller': 'VW',
      'modell': 'Crafter',
      'fin': 'WVWZZZ1JZ3W386752',
      'erstzulassung': '2021-03-15',
      'zulaessige_gesamtmasse_kg': 3500,
      'standort': 'Hof',
      'kostenstelle_id': 'cc1',
      'fahrtenbuch': true,
      'notiz': 'neu',
    });
  });

  testWidgets('Fehler des Servers beim Speichern wird angezeigt', (tester) async {
    await _desktop(tester);
    final failing = _FakeApi(
        permissions: _rw,
        vehicles: _vehicles(),
        costCenters: _cc,
        saveError: const ApiException(statusCode: 400, message: 'Fahrzeug mit diesem Kennzeichen existiert bereits'));
    await tester.pumpWidget(_wrap(FleetPage(api: failing)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Fahrzeug mit diesem Kennzeichen existiert bereits'), findsOneWidget);
  });

  testWidgets('Kostenstellen nicht ladbar: bestehende Zuordnung bleibt beim Speichern', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, vehicles: _vehicles());
    await tester.pumpWidget(_wrap(FleetPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    expect(find.text('Kostenstelle zugeordnet – sie bleibt beim Speichern erhalten.'), findsOneWidget);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.value['kostenstelle_id'], 'cc1');
  });

  testWidgets('Kostenstelle nicht in der Liste (z. B. stillgelegt): Hinweis, Zuordnung bleibt', (tester) async {
    await _desktop(tester);
    final vehicles = _vehicles();
    vehicles[0]['kostenstelle_id'] = 'cc-alt';
    final api = _FakeApi(permissions: _rw, vehicles: vehicles, costCenters: _cc);
    await tester.pumpWidget(_wrap(FleetPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    expect(find.text('Kostenstelle zugeordnet – sie bleibt beim Speichern erhalten.'), findsOneWidget);
    expect(find.widgetWithText(DropdownButtonFormField<String?>, 'Keine'), findsNothing);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.value['kostenstelle_id'], 'cc-alt');
  });

  testWidgets('abmelden mit Datum und Grund; abgemeldete ohne Abmelde-Knopf', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, vehicles: _vehicles(), costCenters: _cc);
    await tester.pumpWidget(_wrap(FleetPage(api: api, now: () => DateTime(2026, 3, 5))));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Abmelden'), findsNWidgets(2));
    await tester.tap(find.byTooltip('Abmelden').first);
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)')).controller!.text, '05.03.2026');
    await tester.tap(find.widgetWithText(FilledButton, 'Abmelden'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Grund angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)'), '1.2.26');
    await tester.enterText(find.widgetWithText(TextField, 'Grund (z. B. verkauft)'), 'verkauft');
    await tester.tap(find.widgetWithText(FilledButton, 'Abmelden'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Datum als TT.MM.JJJJ angeben.'), findsOneWidget);
    expect(api.deregistered, isEmpty);
    await tester.enterText(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)'), '20.02.2026');
    await tester.tap(find.widgetWithText(FilledButton, 'Abmelden'));
    await tester.pumpAndSettle();
    expect(api.deregistered, [
      ['v1', '2026-02-20', 'verkauft']
    ]);
    expect(find.text('Fahrzeug abgemeldet'), findsOneWidget);

    await tester.tap(find.text('Abgemeldete anzeigen'));
    await tester.pumpAndSettle();
    // Der abgemeldete Pkw hat Bearbeiten, aber kein Abmelden.
    expect(find.byTooltip('Bearbeiten'), findsNWidgets(3));
    expect(find.byTooltip('Abmelden'), findsNWidgets(2));
  });

  testWidgets('Dashboard: Karte "Fuhrpark" nur mit fleet.read', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'});
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'fleet.self', 'employees.read'})));
    await tester.pumpAndSettle();
    expect(find.text('Fuhrpark'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(key: const ValueKey('mit'), api: api, onLogout: () async {}, permissions: const {'fleet.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fuhrpark'));
    await tester.pumpAndSettle();
    expect(find.byType(FleetPage), findsOneWidget);
  });

  group('API-Methoden', () {
    final api = ApiClient(baseUrl: 'http://api.test');
    late List<http.Request> calls;

    Future<T> mock<T>(Object body, Future<T> Function() call, {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(utf8.encode(jsonEncode(body)), status, headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    setUp(() => calls = []);

    test('Pfade, Methoden, Filter und erwartete Statuscodes', () async {
      await mock([], () => api.listVehicles());
      await mock([], () => api.listVehicles(art: 'lkw', includeAbgemeldete: true));
      await mock({'id': 'v'}, () => api.createVehicle({'kennzeichen': 'A'}), status: 201);
      await mock({'id': 'v'}, () => api.updateVehicle('v', {'kennzeichen': 'A'}));
      await mock({'id': 'v'}, () => api.deregisterVehicle('v', datum: '2026-01-01', grund: 'x'));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/fleet/vehicles/?',
        'GET /api/v1/fleet/vehicles/?art=lkw&include_abgemeldete=true',
        'POST /api/v1/fleet/vehicles/?',
        'PUT /api/v1/fleet/vehicles/v?',
        'POST /api/v1/fleet/vehicles/v/deregister?',
      ]);
      expect(jsonDecode(calls[4].body), {'datum': '2026-01-01', 'grund': 'x'});
      for (final i in [2, 3, 4]) {
        expect(calls[i].headers['content-type'], startsWith('application/json'));
      }
    });

    test('falscher Status wird zur ApiException', () async {
      await expectLater(mock({'id': 'v'}, () => api.createVehicle({}), status: 200), throwsA(isA<ApiException>()));
      await expectLater(
        mock({'error': {'code': 'conflict', 'message': 'Fahrzeug ist bereits abgemeldet'}}, () => api.deregisterVehicle('v', datum: '2026-01-01', grund: 'x'), status: 409),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Fahrzeug ist bereits abgemeldet')),
      );
      await expectLater(mock({}, () => api.updateVehicle('v', {}), status: 400), throwsA(isA<ApiException>()));
    });
  });
}
