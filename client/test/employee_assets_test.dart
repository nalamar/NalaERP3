import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/employee_assets_section.dart';
import 'package:nalaerp_client/pages/employees_page.dart';

// Ausgegebene Betriebsmittel am Mitarbeiter (Backlog F.7.4). InkRipple und
// Desktop-Fenster wie in den anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, this.assignments = const {}, this.error}) : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final Map<String, List<dynamic>> assignments;
  final Object? error;
  final List<String> calls = [];

  final employees = <Map<String, dynamic>>[
    {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg', 'active': true},
    {'id': 'e2', 'first_name': 'Bert', 'last_name': 'Cord', 'active': true},
  ];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listAssetAssignments({String? assetId, String? employeeId, bool nurOffen = false, bool includeStorniert = false}) async {
    calls.add('$employeeId|$nurOffen|$includeStorniert|$assetId');
    if (error != null) throw error!;
    return assignments[employeeId] ?? const [];
  }

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async => employees;

  @override
  Future<List<dynamic>> listTeams() async => const [];

  @override
  Future<Map<String, dynamic>> getEmployee(String id) async => employees.firstWhere((e) => e['id'] == id);
}

Map<String, dynamic> _a(String nr, String name, String date) =>
    {'inventarnummer': nr, 'bezeichnung': name, 'ausgegeben_am': '${date}T00:00:00Z'};

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: Scaffold(body: child),
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

void main() {
  test('assetAssignmentLabel', () {
    expect(assetAssignmentLabel(_a('L-1', 'Leiter', '2026-01-05')), 'L-1 – Leiter, seit 05.01.2026');
  });

  testWidgets('zeigt nur die offenen Ausgaben des Mitarbeiters', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'assets.read'}, assignments: {
      'e1': [_a('L-1', 'Leiter', '2026-01-05'), _a('E-7', 'Bohrhammer', '2025-12-01')],
    });
    await tester.pumpWidget(_wrap(EmployeeAssetsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();
    // Gefragt wird nach den offenen Ausgaben genau dieses Mitarbeiters.
    expect(api.calls, ['e1|true|false|null']);
    expect(find.text('L-1 – Leiter, seit 05.01.2026'), findsOneWidget);
    expect(find.text('E-7 – Bohrhammer, seit 01.12.2025'), findsOneWidget);
    expect(find.text('Keine Betriebsmittel ausgegeben.'), findsNothing);
  });

  testWidgets('leere Liste und Ladefehler', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(EmployeeAssetsSection(api: _FakeApi(permissions: const {'assets.read'}), employeeId: 'e1')));
    await tester.pumpAndSettle();
    expect(find.text('Keine Betriebsmittel ausgegeben.'), findsOneWidget);

    await tester.pumpWidget(_wrap(EmployeeAssetsSection(
      key: const ValueKey('fehler'),
      api: _FakeApi(permissions: const {'assets.read'}, error: const ApiException(statusCode: 403, message: 'Keine Berechtigung')),
      employeeId: 'e1',
    )));
    await tester.pumpAndSettle();
    expect(find.text('Betriebsmittel konnten nicht geladen werden: Keine Berechtigung'), findsOneWidget);
    expect(find.text('Keine Betriebsmittel ausgegeben.'), findsNothing);
  });

  testWidgets('Mitarbeiterseite: Abschnitt nur mit assets.read, laedt beim Wechsel neu', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'employees.read', 'assets.read'}, assignments: {
      'e1': [_a('L-1', 'Leiter', '2026-01-05')],
      'e2': [_a('IT-3', 'Laptop', '2026-02-01')],
    });
    await tester.pumpWidget(MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: EmployeesPage(api: api),
    ));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();
    expect(find.text('L-1 – Leiter, seit 05.01.2026'), findsOneWidget);
    await tester.tap(find.text('Bert Cord'));
    await tester.pumpAndSettle();
    expect(find.text('L-1 – Leiter, seit 05.01.2026'), findsNothing);
    expect(find.text('IT-3 – Laptop, seit 01.02.2026'), findsOneWidget);

    // Ohne assets.read (das Lager hat nur assets.self) kein Abschnitt.
    final noAssets = _FakeApi(permissions: const {'employees.read', 'assets.self'});
    await tester.pumpWidget(MaterialApp(
      key: const ValueKey('ohne'),
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: EmployeesPage(api: noAssets),
    ));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();
    expect(find.text('Ausgegebene Betriebsmittel'), findsNothing);
    expect(noAssets.calls, isEmpty);
  });

  test('listAssetAssignments: Pfad und Filter', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <Uri>[];
    final client = MockClient((req) async {
      calls.add(req.url);
      return http.Response.bytes(utf8.encode('[]'), 200, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.listAssetAssignments();
      await api.listAssetAssignments(employeeId: 'e1', nurOffen: true);
      await api.listAssetAssignments(assetId: 'a1', includeStorniert: true);
    }, () => client);
    expect(calls.map((u) => '${u.path}?${u.query}'), [
      '/api/v1/asset-assignments/?',
      '/api/v1/asset-assignments/?employee_id=e1&nur_offen=true',
      '/api/v1/asset-assignments/?asset_id=a1&include_storniert=true',
    ]);
  });
}
