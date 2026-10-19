import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/employees_page.dart';
import 'package:nalaerp_client/pages/my_time_page.dart';

// Zeiten eines anderen Mitarbeiters fuer die Personalabteilung (Backlog
// F.5.3). Feste Uhr (Mittwoch, 14.10.2026); InkRipple und Desktop-Fenster
// wie in den anderen Client-Tests (Backlog 0.48).

final _now = DateTime(2026, 10, 14, 9, 30);

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, this.listError}) : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final Object? listError;
  final List<String> calls = [];
  final List<String?> createdFor = [];
  final List<String> corrected = [];
  final List<String> stornoed = [];

  final employees = <Map<String, dynamic>>[
    {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg', 'active': true},
  ];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<Map<String, dynamic>> listTimeEntries({required String from, required String to, String? employeeId, bool includeStorniert = false}) async {
    calls.add('list $from $to ${employeeId ?? '-'}');
    if (listError != null) throw listError!;
    return {
      'entries': [
        {'id': 't1', 'work_date': '2026-10-13T00:00:00Z', 'started_at': '2026-10-13T05:00:00Z', 'ended_at': '2026-10-13T13:00:00Z', 'break_minutes': 30, 'status': 'active'},
      ],
      'days': [
        {'work_date': '2026-10-13', 'net_minutes': 450, 'break_minutes': 30, 'open': false, 'hinweise': []},
      ],
    };
  }

  @override
  Future<Map<String, dynamic>> createTimeEntry(
      {required String startedAt, required String endedAt, int breakMinutes = 0, String note = '', String? employeeId}) async {
    createdFor.add(employeeId);
    return {'id': 'neu'};
  }

  @override
  Future<Map<String, dynamic>> correctTimeEntry(String id,
      {required String startedAt, required String endedAt, int breakMinutes = 0, String note = '', required String reason}) async {
    corrected.add(id);
    return {'id': 'neu'};
  }

  @override
  Future<Map<String, dynamic>> stornoTimeEntry(String id, String reason) async {
    stornoed.add(id);
    return {'id': id};
  }

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async => employees;

  @override
  Future<List<dynamic>> listTeams() async => const [];

  @override
  Future<Map<String, dynamic>> getEmployee(String id) async => employees.firstWhere((e) => e['id'] == id);
}

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

void main() {
  testWidgets('nur Leserecht: Zeiten des Mitarbeiters, ohne Kommen/Gehen und ohne Bearbeiten', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'time_entries.read'});
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now, employeeId: 'e1', employeeName: 'Anna Berg')));
    await tester.pumpAndSettle();
    expect(api.calls, ['list 2026-10-12 2026-10-18 e1']);
    expect(find.text('Arbeitszeiten: Anna Berg'), findsOneWidget);
    expect(find.text('Di 13.10.: 7:30 h (Pause 30 min)'), findsOneWidget);
    expect(find.text('Kommen'), findsNothing);
    expect(find.text('Nicht eingestempelt'), findsNothing);
    expect(find.text('Zeit nachtragen'), findsNothing);
    expect(find.byTooltip('Korrigieren'), findsNothing);
    expect(find.byTooltip('Stornieren'), findsNothing);

    // Auch beim Blaettern bleibt es der Mitarbeiter.
    await tester.tap(find.byTooltip('Vorwoche'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list 2026-10-05 2026-10-11 e1');
  });

  testWidgets('mit Schreibrecht: nachtragen fuer den Mitarbeiter, korrigieren, stornieren', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'time_entries.read', 'time_entries.write'});
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now, employeeId: 'e1', employeeName: 'Anna Berg')));
    await tester.pumpAndSettle();
    expect(find.text('Kommen'), findsNothing);

    await tester.tap(find.text('Zeit nachtragen'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Beginn (HH:MM)'), '07:00');
    await tester.enterText(find.widgetWithText(TextField, 'Ende (HH:MM, vor Beginn = Folgetag)'), '12:00');
    await tester.tap(find.widgetWithText(FilledButton, 'Nachtragen'));
    await tester.pumpAndSettle();
    expect(api.createdFor, ['e1']);

    await tester.tap(find.byTooltip('Korrigieren'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Begründung der Korrektur'), 'Ende falsch');
    await tester.tap(find.widgetWithText(FilledButton, 'Korrigieren'));
    await tester.pumpAndSettle();
    expect(api.corrected, ['t1']);

    await tester.tap(find.byTooltip('Stornieren'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), 'doppelt');
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Stornieren'));
    await tester.pumpAndSettle();
    expect(api.stornoed, ['t1']);
  });

  testWidgets('fuer einen anderen Mitarbeiter kein Hinweis auf den eigenen Login', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
      permissions: const {'time_entries.read'},
      listError: const ApiException(statusCode: 403, code: 'forbidden', message: 'Kein Mitarbeiter mit diesem Benutzer verknüpft'),
    );
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now, employeeId: 'e1', employeeName: 'Anna Berg')));
    await tester.pumpAndSettle();
    expect(find.textContaining('Ihr Login ist mit keinem Mitarbeiter verknüpft'), findsNothing);
    expect(find.text('Arbeitszeiten konnten nicht geladen werden: Kein Mitarbeiter mit diesem Benutzer verknüpft'), findsOneWidget);
  });

  testWidgets('Mitarbeiterseite: "Arbeitszeiten" nur mit time_entries.read, oeffnet die Zeiten des Mitarbeiters', (tester) async {
    await _desktop(tester);
    final none = _FakeApi(permissions: const {'employees.read'});
    await tester.pumpWidget(_wrap(EmployeesPage(api: none)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();
    expect(find.text('Arbeitszeiten'), findsNothing);

    final api = _FakeApi(permissions: const {'employees.read', 'time_entries.read'});
    await tester.pumpWidget(_wrap(EmployeesPage(key: const ValueKey('mit'), api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Arbeitszeiten'));
    await tester.pumpAndSettle();
    expect(find.text('Arbeitszeiten: Anna Berg'), findsOneWidget);
    expect(api.calls.single, endsWith(' e1'));
  });
}
