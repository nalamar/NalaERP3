import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/employees_page.dart';

// Mitarbeiterverwaltung im Client (Backlog F.7.1).
//
// Die Tests setzen InkRipple statt des Material-3-Standards InkSparkle:
// dessen Shader laesst sich in dieser Testumgebung nicht laden
// ("Unsupported runtime stages format version") - das betrifft auch
// vorbestehende Tests und ist als eigener Backlog-Punkt erfasst.

class _FakeEmployeeApi extends ApiClient {
  _FakeEmployeeApi({
    List<Map<String, dynamic>>? employees,
    this.teams = const [],
    this.permissions = const {'employees.read'},
    this.listError,
  })  : employees = employees ?? [],
        super(baseUrl: 'http://localhost:8080');

  final List<Map<String, dynamic>> employees;
  final List<dynamic> teams;
  final Set<String> permissions;
  final Object? listError;

  final List<Map<String, dynamic>> createdBodies = [];
  final List<MapEntry<String, Map<String, dynamic>>> patches = [];
  int listCalls = 0;

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async {
    listCalls++;
    if (listError != null) throw listError!;
    return List<dynamic>.from(employees);
  }

  @override
  Future<List<dynamic>> listTeams() async => teams;

  @override
  Future<Map<String, dynamic>> getEmployee(String id) async =>
      Map<String, dynamic>.from(employees.firstWhere((e) => e['id'] == id));

  @override
  Future<Map<String, dynamic>> createEmployee(Map<String, dynamic> body) async {
    createdBodies.add(body);
    final created = <String, dynamic>{'id': 'neu-1', 'active': true, ...body};
    employees.add(created);
    return created;
  }

  @override
  Future<Map<String, dynamic>> updateEmployee(String id, Map<String, dynamic> patch) async {
    patches.add(MapEntry(id, patch));
    final e = employees.firstWhere((e) => e['id'] == id);
    e.addAll(patch);
    return Map<String, dynamic>.from(e);
  }
}

Map<String, dynamic> _emp(String id, String first, String last, {bool active = true, String role = '', String? teamId}) => {
      'id': id,
      'first_name': first,
      'last_name': last,
      'email': '$first@example.com'.toLowerCase(),
      'phone': '',
      'role': role,
      'location': '',
      'cost_center': '',
      'active': active,
      if (teamId != null) 'team_id': teamId,
    };

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

// Desktop-Fenster: die Seite ist zweispaltig (Liste und Detail).
Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

void main() {
  group('buildEmployeeCreateBody', () {
    test('laesst leere optionale Angaben weg und setzt kein active', () {
      final body = buildEmployeeCreateBody(firstName: ' Anna ', lastName: ' Berg ', email: '  ', role: 'Monteur');
      expect(body, {'first_name': 'Anna', 'last_name': 'Berg', 'role': 'Monteur'});
      expect(body.containsKey('active'), isFalse);
    });

    test('uebernimmt alle Angaben getrimmt', () {
      final body = buildEmployeeCreateBody(
        firstName: 'Anna',
        lastName: 'Berg',
        personalNr: ' P-7 ',
        email: ' anna@example.com ',
        phone: ' 0123 ',
        role: ' Monteurin ',
        teamId: 't1',
      );
      expect(body, {
        'first_name': 'Anna',
        'last_name': 'Berg',
        'personalnummer': 'P-7',
        'email': 'anna@example.com',
        'phone': '0123',
        'role': 'Monteurin',
        'team_id': 't1',
      });
    });
  });

  testWidgets('listet Mitarbeiter, kennzeichnet inaktive und zeigt Teamnamen im Detail', (tester) async {
    await _desktop(tester);
    final api = _FakeEmployeeApi(
      employees: [
        _emp('e1', 'Anna', 'Berg', role: 'Monteurin', teamId: 't1'),
        _emp('e2', 'Otto', 'Alt', active: false, role: 'Schlosser'),
      ],
      teams: [
        {'id': 't1', 'name': 'Montage'},
      ],
    );
    await tester.pumpWidget(_wrap(EmployeesPage(api: api)));
    await tester.pumpAndSettle();

    expect(find.text('Anna Berg'), findsOneWidget);
    expect(find.text('Monteurin'), findsOneWidget);
    expect(find.text('Schlosser · inaktiv'), findsOneWidget);
    expect(find.text('2 Einträge'), findsOneWidget);

    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();
    expect(find.text('Team: Montage'), findsOneWidget);
    expect(find.text('Status: aktiv'), findsOneWidget);
  });

  testWidgets('ohne employees.write keine Schreibaktionen', (tester) async {
    await _desktop(tester);
    final api = _FakeEmployeeApi(employees: [_emp('e1', 'Anna', 'Berg')]);
    await tester.pumpWidget(_wrap(EmployeesPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.byType(FloatingActionButton), findsNothing);

    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();
    expect(find.text('Status: aktiv'), findsOneWidget);
    expect(find.text('Bearbeiten'), findsNothing);
    expect(find.text('Deaktivieren'), findsNothing);
  });

  testWidgets('legt einen Mitarbeiter an und waehlt ihn aus', (tester) async {
    await _desktop(tester);
    final api = _FakeEmployeeApi(
      permissions: const {'employees.read', 'employees.write'},
      teams: [
        {'id': 't1', 'name': 'Montage'},
      ],
    );
    await tester.pumpWidget(_wrap(EmployeesPage(api: api)));
    await tester.pumpAndSettle();
    final listCallsBefore = api.listCalls;

    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Personalnummer (optional)'), 'P-1');
    await tester.enterText(find.widgetWithText(TextField, 'Vorname'), 'Neu');
    await tester.enterText(find.widgetWithText(TextField, 'Nachname'), 'Mann');
    await tester.enterText(find.widgetWithText(TextField, 'Stelle / Rolle'), 'Monteur');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();

    expect(api.createdBodies, [
      {'first_name': 'Neu', 'last_name': 'Mann', 'personalnummer': 'P-1', 'role': 'Monteur'},
    ]);
    expect(find.byType(AlertDialog), findsNothing);
    expect(api.listCalls, greaterThan(listCallsBefore));
    expect(find.text('Personalnummer: P-1'), findsOneWidget);
    expect(find.text('Neu Mann'), findsWidgets);
  });

  testWidgets('deaktiviert und reaktiviert ueber einen Patch nur auf active', (tester) async {
    await _desktop(tester);
    final api = _FakeEmployeeApi(
      employees: [_emp('e1', 'Anna', 'Berg')],
      permissions: const {'employees.read', 'employees.write'},
    );
    await tester.pumpWidget(_wrap(EmployeesPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Deaktivieren'));
    await tester.pumpAndSettle();
    expect(api.patches.last.key, 'e1');
    expect(api.patches.last.value, {'active': false});
    expect(find.text('Status: inaktiv'), findsOneWidget);
    expect(find.text('inaktiv'), findsOneWidget); // Untertitel in der Liste

    await tester.tap(find.text('Reaktivieren'));
    await tester.pumpAndSettle();
    expect(api.patches.last.value, {'active': true});
    expect(find.text('Status: aktiv'), findsOneWidget);
  });

  testWidgets('Bearbeiten sendet alle bearbeitbaren Felder, ohne Team als null', (tester) async {
    await _desktop(tester);
    final api = _FakeEmployeeApi(
      employees: [_emp('e1', 'Anna', 'Berg', role: 'Monteurin', teamId: 'gibt-es-nicht')],
      permissions: const {'employees.read', 'employees.write'},
    );
    await tester.pumpWidget(_wrap(EmployeesPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Bearbeiten'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Telefon'), ' 0151 ');
    await tester.enterText(find.widgetWithText(TextField, 'Standort'), 'Halle 2');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.patches.single.value, {
      'first_name': 'Anna',
      'last_name': 'Berg',
      'email': 'anna@example.com',
      'phone': '0151',
      'role': 'Monteurin',
      'location': 'Halle 2',
      'team_id': null,
    });
    expect(find.text('Standort: Halle 2'), findsOneWidget);
  });

  testWidgets('zeigt einen Ladefehler an', (tester) async {
    await _desktop(tester);
    final api = _FakeEmployeeApi(listError: const ApiException(statusCode: 403, message: 'Keine Berechtigung'));
    await tester.pumpWidget(_wrap(EmployeesPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('Mitarbeiter konnten nicht geladen werden: Keine Berechtigung'), findsOneWidget);
  });

  testWidgets('Dashboard zeigt "Personal" nur mit employees.read und oeffnet die Seite', (tester) async {
    await _desktop(tester);
    final api = _FakeEmployeeApi(employees: [_emp('e1', 'Anna', 'Berg')]);
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {})));
    await tester.pumpAndSettle();
    expect(find.text('Personal'), findsNothing);

    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'employees.read'})));
    await tester.pumpAndSettle();
    expect(find.text('Personal'), findsOneWidget);
    await tester.tap(find.text('Personal'));
    await tester.pumpAndSettle();
    expect(find.byType(EmployeesPage), findsOneWidget);
    expect(find.text('Anna Berg'), findsOneWidget);
  });
}
