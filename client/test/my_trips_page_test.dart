import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/my_trips_page.dart';

// "Meine Fahrten" (Backlog G.3c.4b2). Erfundene Daten.

class _FakeApi extends ApiClient {
  _FakeApi(
      {required this.permissions,
      required this.assignments,
      List<Map<String, dynamic>>? trips})
      : trips = trips ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> assignments;
  final List<Map<String, dynamic>> trips;
  final List<String> calls = [];
  final List<Map<String, dynamic>> recorded = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listOwnFleetAssignments({bool nurOffen = false}) async {
    calls.add('own-assignments $nurOffen');
    return assignments;
  }

  @override
  Future<List<dynamic>> listOwnFleetTrips(
      {String? vehicleId, bool includeStorniert = false}) async {
    calls.add('own-trips ${vehicleId ?? '-'} $includeStorniert');
    return trips
        .where((t) => vehicleId == null || t['vehicle_id'] == vehicleId)
        .toList();
  }

  @override
  Future<List<dynamic>> listFleetTrips(
      {String? vehicleId,
      String? employeeId,
      String? von,
      String? bis,
      bool includeStorniert = false}) async {
    calls.add('ALL-trips');
    return const [];
  }

  @override
  Future<Map<String, dynamic>> getFleetTripChain(String vehicleId) async {
    final own = trips.where((t) => t['vehicle_id'] == vehicleId).toList();
    return {
      'km_stand': own.isEmpty ? null : own.first['km_ende'],
      'luecken': const []
    };
  }

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async {
    calls.add('employees');
    return const [];
  }

  @override
  Future<Map<String, dynamic>> recordFleetTrip(
      Map<String, dynamic> body) async {
    recorded.add(Map.of(body));
    trips.insert(0, {
      'id': 'neu', 'vehicle_id': body['vehicle_id'], 'kennzeichen': 'M-MF 1',
      'datum': '${body['datum']}T00:00:00Z', //
      'km_beginn': body['km_beginn'],
      'km_ende': body['km_ende'],
      'fahrtart': body['fahrtart'],
      'mitarbeiter_name': 'Anna Berg',
      'status': 'active',
    });
    return {'id': 'neu'};
  }
}

Map<String, dynamic> _assignment(String vehicleId, String kennzeichen,
        {bool fahrtenbuch = true, String status = 'active', String? ende}) =>
    {
      'id': 'a-$vehicleId',
      'vehicle_id': vehicleId,
      'kennzeichen': kennzeichen,
      'fahrtenbuch': fahrtenbuch,
      'status': status,
      'ende': ende
    };

List<Map<String, dynamic>> _trips() => [
      {
        'id': 't2', 'vehicle_id': 'v9', 'kennzeichen': 'M-ALT 9',
        'datum': '2026-09-02T00:00:00Z', 'km_beginn': 500, 'km_ende': 520, //
        'fahrtart': 'privat', 'mitarbeiter_name': 'Anna Berg',
        'status': 'active',
      },
      {
        'id': 't1', 'vehicle_id': 'v1', 'kennzeichen': 'M-MF 1',
        'datum': '2026-10-01T00:00:00Z', 'km_beginn': 100, 'km_ende': 150, //
        'fahrtart': 'privat', 'mitarbeiter_name': 'Anna Berg',
        'status': 'active',
      },
    ];

Future<void> _pump(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
    theme:
        ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
    home: MyTripsPage(
        key: UniqueKey(), api: api, now: () => DateTime(2026, 10, 19, 10)),
  ));
  await tester.pumpAndSettle();
}

Finder _dlgField(String label) => find.descendant(
    of: find.byType(AlertDialog),
    matching: find.widgetWithText(TextField, label));

void main() {
  test('logbookAssignments: nur offene, nicht stornierte mit Fahrtenbuch', () {
    final out = logbookAssignments([
      _assignment('v1', 'A'),
      _assignment('v2', 'B', fahrtenbuch: false),
      _assignment('v3', 'C', status: 'storniert'),
      _assignment('v4', 'D', ende: '2026-10-01T00:00:00Z'),
    ]);
    expect(out.map((a) => a['vehicle_id']), ['v1']);
  });

  testWidgets(
      'zeigt eigene Fahrzeuge mit Fahrtenbuch und alle eigenen Fahrten; nur eigene Abfragen',
      (tester) async {
    // Auch mit fleet.write: auf "Meine Fahrten" kein Storno, keine Fahrerauswahl.
    final api = _FakeApi(
      permissions: const {'fleet.self', 'fleet.read', 'fleet.write'},
      assignments: [
        _assignment('v1', 'M-MF 1'),
        _assignment('v2', 'M-OHNE 2', fahrtenbuch: false)
      ],
      trips: _trips(),
    );
    await _pump(tester, api);
    expect(
        api.calls,
        containsAll(
            ['own-assignments true', 'own-trips - true', 'own-trips v1 true']));
    expect(api.calls, isNot(contains('ALL-trips')));
    expect(find.text('Fahrtenbuch M-MF 1'), findsOneWidget);
    expect(find.text('Fahrtenbuch M-OHNE 2'), findsNothing);
    expect(find.text('km-Stand: 150 km'), findsOneWidget);
    expect(find.textContaining('M-ALT 9: 02.09.2026'), findsOneWidget);
    expect(find.byTooltip('Fahrt stornieren'), findsNothing);

    await tester.tap(find.text('Fahrt erfassen'));
    await tester.pumpAndSettle();
    expect(api.calls, isNot(contains('employees')));
    expect(find.byKey(const ValueKey('tripEmployee')), findsNothing);
    await tester.enterText(_dlgField('km Ende'), '180');
    await tester.tap(find.byKey(const ValueKey('tripFahrtart')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Privatfahrt').last);
    await tester.pumpAndSettle();
    await tester.tap(find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(FilledButton, 'Erfassen')));
    await tester.pumpAndSettle();
    expect(api.recorded.single['vehicle_id'], 'v1');
    expect(api.recorded.single['km_beginn'], 150);
    expect(api.recorded.single.containsKey('employee_id'), isFalse);
    // Die Gesamtliste wird nachgeladen.
    expect(find.textContaining('M-MF 1: 19.10.2026'), findsOneWidget);
  });

  testWidgets(
      'ohne zugeordnetes Fahrzeug mit Fahrtenbuch: Hinweis, kein Erfassen',
      (tester) async {
    final api = _FakeApi(
        permissions: const {'fleet.self'},
        assignments: [_assignment('v2', 'M-OHNE 2', fahrtenbuch: false)]);
    await _pump(tester, api);
    expect(find.textContaining('kein Fahrzeug mit Fahrtenbuch zugeordnet'),
        findsOneWidget);
    expect(find.text('Fahrt erfassen'), findsNothing);
    expect(find.text('Noch keine Fahrt erfasst.'), findsOneWidget);
  });

  testWidgets('Dashboard: Karte nur mit fleet.self', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1400, 1200));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api =
        _FakeApi(permissions: const {'fleet.self'}, assignments: const []);
    await tester.pumpWidget(MaterialApp(
        home: DashboardPage(
            api: api,
            onLogout: () async {},
            permissions: const {'fleet.read', 'fleet.write'})));
    await tester.pumpAndSettle();
    expect(find.text('Meine Fahrten'), findsNothing);
    await tester.pumpWidget(MaterialApp(
        home: DashboardPage(
            key: const ValueKey('mit'),
            api: api,
            onLogout: () async {},
            permissions: const {'fleet.self'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Meine Fahrten'));
    await tester.pumpAndSettle();
    expect(find.byType(MyTripsPage), findsOneWidget);
  });

  test('API: nur_eigene in beiden Listen', () async {
    final calls = <http.Request>[];
    final api = ApiClient(baseUrl: 'http://api.test');
    Future<T> mock<T>(Future<T> Function() call, {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(
            utf8.encode(status == 200
                ? '[]'
                : jsonEncode({
                    'error': {'code': 'forbidden', 'message': 'x'}
                  })),
            status,
            headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    await mock(
        () => api.listOwnFleetTrips(vehicleId: 'v1', includeStorniert: true));
    expect(calls.last.url.path, '/api/v1/fleet/trips/');
    expect(calls.last.url.queryParameters, {
      'nur_eigene': 'true',
      'vehicle_id': 'v1',
      'include_storniert': 'true'
    });
    await mock(() => api.listOwnFleetTrips());
    expect(calls.last.url.queryParameters, {'nur_eigene': 'true'});
    await mock(() => api.listOwnFleetAssignments(nurOffen: true));
    expect(calls.last.url.path, '/api/v1/fleet/assignments/');
    expect(calls.last.url.queryParameters,
        {'nur_eigene': 'true', 'nur_offen': 'true'});
    await mock(() => api.listOwnFleetAssignments());
    expect(calls.last.url.queryParameters, {'nur_eigene': 'true'});
    await expectLater(mock(() => api.listOwnFleetTrips(), status: 403),
        throwsA(isA<ApiException>()));
  });

  // Nachgeholt aus G.3c.4a: Pfade, Methoden und Bodies des Fahrtenbuchs.
  test('API: Fahrtenbuch-Aufrufe', () async {
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
        () => api.listFleetTrips(
            vehicleId: 'v1',
            employeeId: 'e1',
            von: '2026-10-01',
            bis: '2026-10-31',
            includeStorniert: true));
    expect(calls.last.method, 'GET');
    expect(calls.last.url.path, '/api/v1/fleet/trips/');
    expect(calls.last.url.queryParameters, {
      'vehicle_id': 'v1',
      'employee_id': 'e1',
      'von': '2026-10-01',
      'bis': '2026-10-31',
      'include_storniert': 'true'
    });
    await mock([], () => api.listFleetTrips());
    expect(calls.last.url.queryParameters, isEmpty);

    final chain = await mock(
        {'km_stand': 150, 'luecken': []}, () => api.getFleetTripChain('v1'));
    expect(chain['km_stand'], 150);
    expect(calls.last.url.path, '/api/v1/fleet/trips/chain');
    expect(calls.last.url.queryParameters, {'vehicle_id': 'v1'});

    await mock({'id': 't1'},
        () => api.recordFleetTrip({'vehicle_id': 'v1', 'km_beginn': 1}),
        status: 201);
    expect(calls.last.method, 'POST');
    expect(calls.last.url.path, '/api/v1/fleet/trips/');
    expect(jsonDecode(calls.last.body), {'vehicle_id': 'v1', 'km_beginn': 1});
    await expectLater(mock({'id': 't1'}, () => api.recordFleetTrip(const {})),
        throwsA(isA<ApiException>()));
    await expectLater(
        mock({
          'error': {'code': 'conflict', 'message': 'x'}
        }, () => api.recordFleetTrip(const {}), status: 409),
        throwsA(isA<ApiException>()));

    await mock({'id': 't1'}, () => api.stornoFleetTrip('t1', 'falsch'));
    expect(calls.last.method, 'POST');
    expect(calls.last.url.path, '/api/v1/fleet/trips/t1/storno');
    expect(jsonDecode(calls.last.body), {'reason': 'falsch'});
    await expectLater(
        mock({
          'error': {'code': 'conflict', 'message': 'x'}
        }, () => api.stornoFleetTrip('t1', 'x'), status: 409),
        throwsA(isA<ApiException>()));
  });
}
