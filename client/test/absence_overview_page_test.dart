import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/absence_overview_page.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';

// Abwesenheitsuebersicht (Backlog F.8.6.3). Feste Uhr: 14.10.2026.

final _now = DateTime(2026, 10, 14, 9, 30);

Map<String, dynamic> _abs(String emp, String first, String last, String typ, String from, String to) => {
      'employee_id': emp,
      'first_name': first,
      'last_name': last,
      'typ': typ,
      'start_date': '${from}T00:00:00Z',
      'end_date': '${to}T00:00:00Z',
    };

class _FakeApi extends ApiClient {
  _FakeApi({this.perms = const {'leave_requests.read', 'employees.read'}, this.bundesland = 'NW', this.error})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> perms;
  final String bundesland;
  final Object? error;
  final List<String> calls = [];

  @override
  bool hasPermission(String code) => perms.contains(code);

  @override
  Future<List<dynamic>> listTeams() async {
    calls.add('teams');
    return [
      {'id': 't1', 'name': 'Montage'},
    ];
  }

  @override
  Future<List<dynamic>> listAbsences({required String from, required String to, String? employeeId, String? teamId, bool includeStorniert = false}) async {
    calls.add('absences $from $to ${teamId ?? '-'}');
    if (error != null) throw error!;
    return [
      _abs('e2', 'Carl', 'Dorn', 'vacation', '2026-10-12', '2026-10-16'),
      _abs('e2', 'Carl', 'Dorn', 'sick', '2026-10-14', '2026-10-15'),
      _abs('e1', 'Anna', 'Berg', 'unpaid', '2026-09-28', '2026-10-02'),
    ];
  }

  @override
  Future<Map<String, dynamic>> getHolidays({int? year}) async {
    calls.add('holidays $year');
    return {
      'year': year,
      'bundesland': bundesland,
      'days': [
        {'date': '$year-10-03T00:00:00Z', 'name': 'Tag der Deutschen Einheit', 'art': 'gesetzlich'},
        {'date': '$year-11-01T00:00:00Z', 'name': 'Allerheiligen', 'art': 'gesetzlich'},
      ],
    };
  }
}

Future<void> _pump(WidgetTester tester, ApiClient api, {DateTime? now}) async {
  tester.view.physicalSize = const Size(1600, 1200);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(MaterialApp(
    theme: ThemeData(splashFactory: InkRipple.splashFactory),
    home: AbsenceOverviewPage(api: api, now: () => now ?? _now),
  ));
  await tester.pumpAndSettle();
}

void main() {
  test('Monat, Raster, freie Tage, Zeilen', () {
    expect(daysInMonth(2026, 2), 28);
    expect(daysInMonth(2028, 2), 29);
    expect(daysInMonth(2026, 12), 31);
    expect(monthRange(2026, 2), (from: '2026-02-01', to: '2026-02-28'));
    expect(monthRange(2026, 10), (from: '2026-10-01', to: '2026-10-31'));
    final rows = buildAbsenceRows([
      _abs('e2', 'Carl', 'Dorn', 'sick', '2026-10-14', '2026-10-15'),
      _abs('e2', 'Carl', 'Dorn', 'vacation', '2026-10-12', '2026-10-16'),
      _abs('e1', 'Anna', 'Berg', 'unpaid', '2026-09-28', '2026-10-02'),
      _abs('e3', '', 'Ende', 'other', '2026-10-30', '2026-11-03'),
    ], 2026, 10);
    expect(rows.map((r) => r.name), ['Berg, Anna', 'Dorn, Carl', 'Ende']);
    expect(rows[0].days, {1: 'B', 2: 'B'});
    // Krankheit gewinnt - unabhaengig von der Reihenfolge.
    expect(rows[1].days, {12: 'U', 13: 'U', 14: 'K', 15: 'K', 16: 'U'});
    expect(rows[2].days, {30: 'S', 31: 'S'});
    expect(
        freeDaysOfMonth({
          'days': [
            {'date': '2026-10-03T00:00:00Z', 'name': 'Einheit'},
            {'date': '2026-11-01T00:00:00Z', 'name': 'Allerheiligen'},
            {'date': '2026-10-31', 'name': 'Reformationstag'},
          ]
        }, 2026, 10),
        {3: 'Einheit', 31: 'Reformationstag'});
    expect(freeDaysOfMonth(null, 2026, 10), isEmpty);
    expect(absenceLine(_abs('e1', 'Anna', 'Berg', 'vacation', '2026-06-08', '2026-06-12')), 'Berg, Anna: 08.06.2026 – 12.06.2026 · Urlaub');
    expect(absenceLine(_abs('e1', 'Anna', 'Berg', 'sick', '2026-06-08', '2026-06-08')), 'Berg, Anna: 08.06.2026 · Krankheit');
  });

  testWidgets('Oktober: Raster, Liste, freie Tage', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(api.calls, containsAll(['teams', 'absences 2026-10-01 2026-10-31 -', 'holidays 2026']));
    expect(find.text('Oktober 2026'), findsOneWidget);
    expect(find.text('Berg, Anna'), findsOneWidget);
    expect(find.text('Dorn, Carl'), findsOneWidget);
    expect(find.text('K'), findsNWidgets(2));
    expect(find.text('U'), findsNWidgets(3));
    expect(find.text('B'), findsNWidgets(2));
    final lines = tester.widgetList<Text>(find.textContaining(': ')).map((t) => t.data).toList();
    expect(lines, [
      'Berg, Anna: 28.09.2026 – 02.10.2026 · unbezahlter Urlaub',
      'Dorn, Carl: 12.10.2026 – 16.10.2026 · Urlaub',
      'Dorn, Carl: 14.10.2026 – 15.10.2026 · Krankheit',
    ]);
    expect(find.text('03.10.2026 Tag der Deutschen Einheit'), findsOneWidget);
    expect(find.text('01.11.2026 Allerheiligen'), findsNothing);
    expect(find.textContaining('Kein Bundesland eingestellt'), findsNothing);
  });

  testWidgets('Monate blaettern, auch ueber den Jahreswechsel', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api, now: DateTime(2027, 1, 10));
    expect(find.text('Januar 2027'), findsOneWidget);
    await tester.tap(find.byTooltip('Vormonat'));
    await tester.pumpAndSettle();
    expect(find.text('Dezember 2026'), findsOneWidget);
    expect(api.calls, containsAll(['absences 2026-12-01 2026-12-31 -', 'holidays 2026']));
    await tester.tap(find.byTooltip('Folgemonat'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Folgemonat'));
    await tester.pumpAndSettle();
    expect(find.text('Februar 2027'), findsOneWidget);
    expect(api.calls, contains('absences 2027-02-01 2027-02-28 -'));
  });

  testWidgets('Team-Filter', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Montage').last);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'holidays 2026');
    expect(api.calls, contains('absences 2026-10-01 2026-10-31 t1'));
  });

  testWidgets('Ohne Mitarbeiterrecht kein Team-Filter; ohne Bundesland Hinweis', (tester) async {
    final api = _FakeApi(perms: const {'leave_requests.read'}, bundesland: '');
    await _pump(tester, api);
    expect(api.calls.contains('teams'), isFalse);
    expect(find.byType(DropdownButtonFormField<String>), findsNothing);
    expect(find.text('Hinweis: Kein Bundesland eingestellt - es sind nur die bundesweiten Feiertage hinterlegt.'), findsOneWidget);
  });

  testWidgets('Fehler beim Laden', (tester) async {
    final api = _FakeApi(error: const ApiException(statusCode: 403, code: 'forbidden', message: 'Keine Berechtigung'));
    await _pump(tester, api);
    expect(find.text('Abwesenheiten konnten nicht geladen werden: Keine Berechtigung'), findsOneWidget);
    expect(find.textContaining('U = Urlaub'), findsNothing);
  });

  testWidgets('Dashboard: "Abwesenheiten" nur mit leave_requests.read', (tester) async {
    final api = _FakeApi();
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(MaterialApp(home: DashboardPage(api: api, onLogout: () async {}, permissions: const {'leave_requests.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Abwesenheiten'), findsNothing);
    await tester.pumpWidget(MaterialApp(
        theme: ThemeData(splashFactory: InkRipple.splashFactory),
        home: DashboardPage(api: api, onLogout: () async {}, permissions: const {'leave_requests.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abwesenheiten'));
    await tester.pumpAndSettle();
    expect(find.byType(AbsenceOverviewPage), findsOneWidget);
  });

  test('API: Feiertage', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      return http.Response.bytes(utf8.encode('{"days":[]}'), 200, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.getHolidays();
      await api.getHolidays(year: 2027);
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), ['GET /api/v1/holidays/?', 'GET /api/v1/holidays/?year=2027']);
  });
}
