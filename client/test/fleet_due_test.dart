import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/fleet_due_page.dart';
import 'package:nalaerp_client/pages/vehicle_detail_page.dart';

// Faellige Fahrzeugtermine im Client (Backlog G.2.4.3).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, this.entries = const [], this.vehicleError}) : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<dynamic> entries;
  final Object? vehicleError;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listScheduleDue({int? warnDays}) async {
    calls.add('due $warnDays');
    return entries;
  }

  @override
  Future<Map<String, dynamic>> getVehicle(String id) async {
    calls.add('vehicle $id');
    if (vehicleError != null) throw vehicleError!;
    return {'id': id, 'kennzeichen': 'M-AB 1', 'art': 'lkw', 'bezeichnung': 'Pritsche', 'abgemeldet_am': null, 'fahrtenbuch': false};
  }

  @override
  Future<List<dynamic>> listVehicleSchedules(String vehicleId, {bool includeInactive = false}) async {
    calls.add('schedules $vehicleId');
    return const [];
  }

  @override
  Future<List<dynamic>> listFleetAssignments({String? vehicleId, String? employeeId, bool nurOffen = false, bool includeStorniert = false}) async => const [];
}

Map<String, dynamic> _entry(String kz, String art, String status, {String bez = '', String vbez = '', String? faellig, int? tage}) => {
      'vehicle_id': 'v-$kz', 'kennzeichen': kz, 'vehicle_bezeichnung': vbez, 'schedule_id': 's-$kz-$art', 'art': art, 'bezeichnung': bez,
      'standing': {'status': status, 'faellig_am': faellig, 'tage_bis_faellig': tage, 'letztes_ergebnis': status == 'gesperrt' ? 'maengel' : null},
    };

final _entries = [
  _entry('M-AB 1', 'hu', 'gesperrt', vbez: 'Pritsche', faellig: '2026-11-30T00:00:00Z', tage: 35),
  _entry('M-B 1', 'uvv', 'ueberfaellig', faellig: '2026-10-01T00:00:00Z', tage: -25),
  _entry('M-C 1', 'sonstige', 'faellig_bald', bez: 'Tachograph', faellig: '2026-11-05T00:00:00Z', tage: 10),
];

Widget _wrap(Widget child) => MaterialApp(theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory), home: child);

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

void main() {
  test('dueVehicleLabel und dueStatusCounts', () {
    expect(dueVehicleLabel(_entries[0]), 'M-AB 1 – Pritsche · Hauptuntersuchung');
    expect(dueVehicleLabel(_entries[2]), 'M-C 1 · Tachograph');
    expect(dueStatusCounts(_entries), {'gesperrt': 1, 'ueberfaellig': 1, 'faellig_bald': 1});
    expect(dueStatusCounts([..._entries, _entries[1]]), {'gesperrt': 1, 'ueberfaellig': 2, 'faellig_bald': 1});
    expect(dueStatusCounts([]), {'gesperrt': 0, 'ueberfaellig': 0, 'faellig_bald': 0});
  });

  testWidgets('listet die Einträge in der Reihenfolge des Servers mit Stand und Zählern', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, entries: _entries);
    await tester.pumpWidget(_wrap(FleetDuePage(api: api)));
    await tester.pumpAndSettle();
    expect(api.calls, ['due 30']);
    double y(String t) => tester.getTopLeft(find.text(t)).dy;
    expect(y('M-AB 1 – Pritsche · Hauptuntersuchung'), lessThan(y('M-B 1 · UVV-Prüfung')));
    expect(y('M-B 1 · UVV-Prüfung'), lessThan(y('M-C 1 · Tachograph')));
    expect(find.text('gesperrt – letzte Prüfung mit Mängeln, Nachprüfung bis 11/2026'), findsOneWidget);
    expect(find.text('gesperrt: 1 · überfällig: 1 · bald fällig: 1'), findsOneWidget);
    Color? color(String t) => tester.widget<Text>(find.text(t)).style?.color;
    expect(color('überfällig seit 01.10.2026'), const Color(0xFFB00020));
    expect(color('fällig am 05.11.2026 (in 10 Tagen)'), const Color(0xFFB26A00));
  });

  testWidgets('Vorwarnzeit ändern; ungültige Eingabe lädt nicht neu; leere Liste', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'});
    await tester.pumpWidget(_wrap(FleetDuePage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('Nichts fällig – alle Termine liegen weiter als 30 Tage entfernt.'), findsOneWidget);
    expect(find.textContaining('gesperrt:'), findsNothing);
    await tester.enterText(find.widgetWithText(TextField, 'Vorwarnzeit (Tage)'), '-1');
    await tester.tap(find.text('Anwenden'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Vorwarnzeit als ganze Zahl von Tagen angeben (0 oder mehr).'), findsOneWidget);
    expect(api.calls, ['due 30']);
    await tester.enterText(find.widgetWithText(TextField, 'Vorwarnzeit (Tage)'), '90');
    await tester.tap(find.text('Anwenden'));
    await tester.pumpAndSettle();
    expect(api.calls, ['due 30', 'due 90']);
    expect(find.text('Nichts fällig – alle Termine liegen weiter als 90 Tage entfernt.'), findsOneWidget);
  });

  testWidgets('Zeile öffnet das Fahrzeug; danach neu geladen; Fehler beim Laden des Fahrzeugs', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, entries: _entries);
    await tester.pumpWidget(_wrap(FleetDuePage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('M-B 1 · UVV-Prüfung'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('vehicle v-M-B 1'));
    expect(find.byType(VehicleDetailPage), findsOneWidget);
    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('due')).length, 2);

    final failing = _FakeApi(permissions: const {'fleet.read'}, entries: _entries, vehicleError: const ApiException(statusCode: 404, message: 'Fahrzeug nicht gefunden'));
    await tester.pumpWidget(_wrap(FleetDuePage(key: const ValueKey('f'), api: failing)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('M-B 1 · UVV-Prüfung'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Fahrzeug nicht gefunden'), findsOneWidget);
    expect(find.byType(VehicleDetailPage), findsNothing);
  });

  testWidgets('Dashboard: Karte nur mit fleet.read', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'});
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'fleet.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Fällige Fahrzeugtermine'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(key: const ValueKey('mit'), api: api, onLogout: () async {}, permissions: const {'fleet.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fällige Fahrzeugtermine'));
    await tester.pumpAndSettle();
    expect(find.byType(FleetDuePage), findsOneWidget);
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

    test('Pfade und Parameter', () async {
      await mock([], () => api.listScheduleDue());
      await mock([], () => api.listScheduleDue(warnDays: 7));
      final v = await mock({'id': 'v1'}, () => api.getVehicle('v1'));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/fleet/schedules/due?',
        'GET /api/v1/fleet/schedules/due?warn_tage=7',
        'GET /api/v1/fleet/vehicles/v1?',
      ]);
      expect(v['id'], 'v1');
      await expectLater(
        mock({'error': {'code': 'forbidden', 'message': 'Keine Berechtigung'}}, () => api.listScheduleDue(), status: 403),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Keine Berechtigung')),
      );
    });
  });
}
