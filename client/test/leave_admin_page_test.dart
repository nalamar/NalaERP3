import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/hr_calendar_page.dart';
import 'package:nalaerp_client/pages/leave_accounts_page.dart';
import 'package:nalaerp_client/pages/leave_admin_page.dart';

// Urlaubsantraege und Krankmeldungen fuer die Personalabteilung (Backlog
// F.8.6.2.1). Feste Uhr: 14.10.2026.

final _now = DateTime(2026, 10, 14, 9, 30);

class _FakeApi extends ApiClient {
  _FakeApi({this.perms = const {'leave_requests.read', 'leave_requests.decide', 'employees.read'}, this.decideError})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> perms;
  final Object? decideError;
  final List<String> calls = [];

  @override
  bool hasPermission(String code) => perms.contains(code);

  @override
  Future<List<dynamic>> listLeaveRequests({String? employeeId, String? status, int? limit}) async {
    calls.add('requests $status $limit');
    if (status == 'pending') {
      return [
        {'id': 'p2', 'employee_id': 'e2', 'typ': 'vacation', 'status': 'pending', 'start_date': '2026-12-21T00:00:00Z', 'end_date': '2026-12-23T00:00:00Z', 'days': 3},
        {'id': 'p1', 'employee_id': 'e1', 'typ': 'unpaid', 'status': 'pending', 'start_date': '2026-11-02T00:00:00Z', 'end_date': '2026-11-02T00:00:00Z', 'days': 1},
      ];
    }
    return [
      {'id': 'a1', 'employee_id': 'e1', 'typ': 'vacation', 'status': 'approved', 'start_date': '2026-06-08T00:00:00Z', 'end_date': '2026-06-12T00:00:00Z', 'days': 5},
      {'id': 'a0', 'employee_id': 'e1', 'typ': 'vacation', 'status': 'approved', 'start_date': '2025-08-04T00:00:00Z', 'end_date': '2025-08-08T00:00:00Z', 'days': 5},
    ];
  }

  @override
  Future<List<dynamic>> listNewSickReports() async {
    calls.add('newSick');
    return [
      {'id': 's1', 'employee_id': 'e2', 'typ': 'sick', 'start_date': '2026-10-12T00:00:00Z', 'end_date': '2026-10-13T00:00:00Z', 'status': 'approved', 'attest': false, 'gemeldet_von': 'selbst'},
      {'id': 's2', 'employee_id': 'e9', 'typ': 'sick', 'start_date': '2026-10-14T00:00:00Z', 'end_date': '2026-10-14T00:00:00Z', 'status': 'approved', 'attest': true, 'gemeldet_von': 'selbst'},
    ];
  }

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async {
    calls.add('employees $limit');
    return [
      {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg'},
      {'id': 'e2', 'first_name': 'Carl', 'last_name': 'Dorn'},
    ];
  }

  @override
  Future<Map<String, dynamic>> decideLeaveRequest(String id, bool approve) async {
    calls.add('decide $id $approve');
    if (decideError != null) throw decideError!;
    return {'id': id};
  }

  @override
  Future<Map<String, dynamic>> stornoLeaveRequest(String id, String reason) async {
    calls.add('storno $id $reason');
    return {'id': id};
  }

  @override
  Future<Map<String, dynamic>> confirmAbsence(String id) async {
    calls.add('confirm $id');
    return {'id': id};
  }

  @override
  Future<Map<String, dynamic>> setAbsenceAttest(String id, bool attest) async {
    calls.add('attest $id $attest');
    return {'id': id};
  }

  final List<Map<String, dynamic>> sickReports = [];

  @override
  Future<Map<String, dynamic>> reportSick(Map<String, dynamic> body) async {
    calls.add('reportSick');
    sickReports.add(body);
    return {'id': 'neu'};
  }

  @override
  Future<List<dynamic>> listCompanyDays({int? year}) async => [];

  @override
  Future<Map<String, dynamic>> getHolidays({int? year}) async => {'days': []};

  @override
  Future<List<dynamic>> listRemainingLeave({int? year}) async {
    calls.add('remaining $year');
    return [];
  }

  @override
  Future<Map<String, dynamic>> stornoAbsence(String id, String reason) async {
    calls.add('stornoAbsence $id $reason');
    return {'id': id};
  }
}

Future<void> _pump(WidgetTester tester, ApiClient api) async {
  tester.view.physicalSize = const Size(1600, 1200);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(MaterialApp(
    theme: ThemeData(splashFactory: InkRipple.splashFactory),
    home: LeaveAdminPage(api: api, now: () => _now),
  ));
  await tester.pumpAndSettle();
}

Finder _inDialog(Finder f) => find.descendant(of: find.byType(AlertDialog), matching: f);

// Button in der Zeile, deren Text mit prefix beginnt.
Finder _rowButton(String prefix, String label) => find.descendant(
      of: find.ancestor(of: find.textContaining(prefix), matching: find.byType(Wrap)).first,
      matching: find.text(label),
    );

void main() {
  test('Namen und Auswahl', () {
    final names = employeeNames([
      {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg'},
      {'id': 'e2', 'first_name': '', 'last_name': 'Dorn'},
    ]);
    expect(names, {'e1': 'Berg, Anna', 'e2': 'Dorn'});
    expect(withName(names, {'employee_id': 'e1'}, 'x'), 'Berg, Anna: x');
    expect(withName(names, {'employee_id': 'e7'}, 'x'), 'Mitarbeiter: x');
    final sel = approvedFromYear([
      {'id': 'b', 'start_date': '2026-06-08', 'end_date': '2026-06-12'},
      {'id': 'old', 'start_date': '2025-08-04', 'end_date': '2025-08-08'},
      {'id': 'cross', 'start_date': '2025-12-29', 'end_date': '2026-01-02'},
      {'id': 'a', 'start_date': '2026-02-02', 'end_date': '2026-02-03'},
    ], 2026);
    expect(sel.map((m) => m['id']), ['cross', 'a', 'b']);
  });

  testWidgets('Uebersicht mit Namen, sortiert, nur ab diesem Jahr', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(api.calls, ['requests pending 500', 'requests approved 500', 'newSick', 'employees 500']);
    final texts = tester.widgetList<Text>(find.byType(Text)).map((t) => t.data ?? '').where((s) => s.contains(': ')).toList();
    expect(texts, [
      'Berg, Anna: 02.11.2026 · 1 Tag · unbezahlter Urlaub · offen',
      'Dorn, Carl: 21.12.2026 – 23.12.2026 · 3 Tage · Urlaub · offen',
      'Dorn, Carl: 12.10.2026 – 13.10.2026 · kein Attest erfasst · noch nicht bestätigt',
      'Mitarbeiter: 14.10.2026 · Attest liegt vor · noch nicht bestätigt',
      'Berg, Anna: 08.06.2026 – 12.06.2026 · 5 Tage · Urlaub · genehmigt',
    ]);
    expect(find.text('Genehmigen'), findsNWidgets(2));
    expect(_rowButton('Dorn, Carl: 12.10.2026', 'Attest liegt vor'), findsOneWidget);
    expect(_rowButton('Mitarbeiter: 14.10.2026', 'Attest entfernen'), findsOneWidget);
  });

  testWidgets('Nur lesen: keine Buttons, ohne Mitarbeiterliste', (tester) async {
    final api = _FakeApi(perms: const {'leave_requests.read'});
    await _pump(tester, api);
    expect(api.calls.where((c) => c.startsWith('employees')), isEmpty);
    expect(find.text('Mitarbeiter: 02.11.2026 · 1 Tag · unbezahlter Urlaub · offen'), findsOneWidget);
    for (final label in ['Genehmigen', 'Ablehnen', 'Bestätigen', 'Stornieren', 'Attest liegt vor']) {
      expect(find.text(label), findsNothing, reason: label);
    }
  });

  testWidgets('Genehmigen und Ablehnen mit Rueckfrage', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(_rowButton('Dorn, Carl: 21.12.2026', 'Ablehnen'));
    await tester.pumpAndSettle();
    expect(_inDialog(find.text('Dorn, Carl: 21.12.2026 – 23.12.2026 · 3 Tage · Urlaub · offen')), findsOneWidget);
    await tester.tap(_inDialog(find.text('Abbrechen')));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('decide')), isEmpty);
    await tester.tap(_rowButton('Dorn, Carl: 21.12.2026', 'Ablehnen'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Ablehnen')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('decide p2 false'));
    expect(find.text('Antrag abgelehnt'), findsOneWidget);
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    await tester.tap(_rowButton('Berg, Anna: 02.11.2026', 'Genehmigen'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Genehmigen')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('decide p1 true'));
    // Nach jeder Aktion neu geladen.
    expect(api.calls.where((c) => c == 'requests pending 500').length, 3);
  });

  testWidgets('Ablehnung des Servers wird gemeldet', (tester) async {
    final api = _FakeApi(decideError: const ApiException(statusCode: 403, code: 'forbidden', message: 'Keine Berechtigung'));
    await _pump(tester, api);
    await tester.tap(_rowButton('Berg, Anna: 02.11.2026', 'Genehmigen'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Genehmigen')));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Keine Berechtigung'), findsOneWidget);
  });

  testWidgets('Genehmigten Urlaub stornieren nur mit Begruendung', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(_rowButton('Berg, Anna: 08.06.2026', 'Stornieren'));
    await tester.pumpAndSettle();
    expect(_inDialog(find.textContaining('Die Tage gehen auf das Urlaubskonto zurück.')), findsOneWidget);
    final button = _inDialog(find.widgetWithText(FilledButton, 'Stornieren'));
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(_inDialog(find.byType(TextField)), '  Projekt verschoben ');
    await tester.pumpAndSettle();
    await tester.tap(button);
    await tester.pumpAndSettle();
    expect(api.calls, contains('storno a1 Projekt verschoben'));
    expect(find.text('Urlaub storniert'), findsOneWidget);
  });

  testWidgets('Neue Krankmeldungen: bestaetigen, Attest, stornieren', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    // Die zweite Meldung bestaetigen - es muss genau diese sein.
    await tester.tap(_rowButton('Mitarbeiter: 14.10.2026', 'Bestätigen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('confirm')), ['confirm s2']);
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    await tester.tap(_rowButton('Dorn, Carl: 12.10.2026', 'Attest liegt vor'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('attest s1 true'));
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    await tester.tap(_rowButton('Mitarbeiter: 14.10.2026', 'Attest entfernen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('attest s2 false'));
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    await tester.tap(_rowButton('Mitarbeiter: 14.10.2026', 'Stornieren'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Abbrechen')));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('stornoAbsence')), isEmpty);
    await tester.tap(_rowButton('Mitarbeiter: 14.10.2026', 'Stornieren'));
    await tester.pumpAndSettle();
    await tester.enterText(_inDialog(find.byType(TextField)), 'doppelt gemeldet');
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.widgetWithText(FilledButton, 'Stornieren')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('stornoAbsence s2 doppelt gemeldet'));
  });

  testWidgets('Krankmeldung fuer einen Mitarbeiter erfassen', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.text('Krankmeldung erfassen'));
    await tester.pumpAndSettle();
    expect(_inDialog(find.text('Erfasst wird nur der Zeitraum - keine Diagnose.')), findsOneWidget);
    await tester.tap(_inDialog(find.text('Erfassen')));
    await tester.pumpAndSettle();
    expect(find.text('Bitte den Mitarbeiter wählen.'), findsOneWidget);
    await tester.tap(_inDialog(find.byType(DropdownButtonFormField<String>)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Dorn, Carl').last);
    await tester.pumpAndSettle();
    final fields = _inDialog(find.byType(TextField));
    expect(tester.widget<TextField>(fields.at(0)).controller!.text, '14.10.2026');
    await tester.enterText(fields.at(1), '13.10.2026');
    await tester.tap(_inDialog(find.text('Erfassen')));
    await tester.pumpAndSettle();
    expect(find.text('Der letzte Tag liegt vor dem ersten.'), findsOneWidget);
    await tester.enterText(fields.at(1), 'x');
    await tester.tap(_inDialog(find.text('Erfassen')));
    await tester.pumpAndSettle();
    expect(find.text('Bitte beide Tage als TT.MM.JJJJ angeben.'), findsOneWidget);
    expect(api.sickReports, isEmpty);
    await tester.enterText(fields.at(1), '16.10.2026');
    await tester.tap(_inDialog(find.text('Attest liegt vor')));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Erfassen')));
    await tester.pumpAndSettle();
    expect(api.sickReports, [
      {'employee_id': 'e2', 'start_date': '2026-10-14', 'end_date': '2026-10-16', 'attest': true}
    ]);
    expect(find.text('Krankmeldung erfasst'), findsOneWidget);
  });

  testWidgets('Weg zu den Urlaubskonten; ohne Rechte keine Buttons oben', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.text('Urlaubskonten und Resturlaub'));
    await tester.pumpAndSettle();
    expect(find.byType(LeaveAccountsPage), findsOneWidget);
    expect(api.calls, contains('remaining 2026'));
    await tester.pageBack();
    await tester.pumpAndSettle();
    await tester.tap(find.text('Feiertage und Betriebsruhe'));
    await tester.pumpAndSettle();
    expect(find.byType(HrCalendarPage), findsOneWidget);
    // Mit Mitarbeiterliste, aber ohne Entscheidungsrecht: Konten ja,
    // Krankmeldung erfassen nein.
    final reader = _FakeApi(perms: const {'leave_requests.read', 'employees.read'});
    await tester.pumpWidget(const SizedBox());
    await _pump(tester, reader);
    expect(find.text('Krankmeldung erfassen'), findsNothing);
    expect(find.text('Urlaubskonten und Resturlaub'), findsOneWidget);
    expect(find.text('Feiertage und Betriebsruhe'), findsNothing);
    final readOnly = _FakeApi(perms: const {'leave_requests.read'});
    await tester.pumpWidget(const SizedBox());
    await _pump(tester, readOnly);
    expect(find.text('Krankmeldung erfassen'), findsNothing);
    expect(find.text('Urlaubskonten und Resturlaub'), findsNothing);
  });

  testWidgets('Dashboard: "Urlaubsanträge" nur mit leave_requests.read', (tester) async {
    final api = _FakeApi();
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(MaterialApp(home: DashboardPage(api: api, onLogout: () async {}, permissions: const {'leave_requests.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Urlaubsanträge'), findsNothing);
    await tester.pumpWidget(MaterialApp(
        theme: ThemeData(splashFactory: InkRipple.splashFactory),
        home: DashboardPage(api: api, onLogout: () async {}, permissions: const {'leave_requests.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Urlaubsanträge'));
    await tester.pumpAndSettle();
    expect(find.byType(LeaveAdminPage), findsOneWidget);
  });

  test('API: Pfade, Bodies, Statuscodes', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final isList = req.method == 'GET';
      return http.Response.bytes(utf8.encode(isList ? '[]' : '{"id":"x"}'), 200, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.listLeaveRequests(status: 'pending', limit: 500);
      await api.decideLeaveRequest('r1', true);
      await api.decideLeaveRequest('r2', false);
      await api.stornoLeaveRequest('r3', 'Grund');
      await api.listNewSickReports();
      await api.setAbsenceAttest('a1', true);
      await api.confirmAbsence('a2');
      await api.stornoAbsence('a3', 'falsch');
      await api.setAbsenceAttest('a4', false);
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
      'GET /api/v1/leave-requests/?status=pending&limit=500',
      'POST /api/v1/leave-requests/r1/decide?',
      'POST /api/v1/leave-requests/r2/decide?',
      'POST /api/v1/leave-requests/r3/storno?',
      'GET /api/v1/absences/sick/new?',
      'POST /api/v1/absences/a1/attest?',
      'POST /api/v1/absences/a2/confirm?',
      'POST /api/v1/absences/a3/storno?',
      'POST /api/v1/absences/a4/attest?',
    ]);
    expect(jsonDecode(calls[8].body), {'attest': false});
    expect(jsonDecode(calls[1].body), {'approve': true});
    expect(jsonDecode(calls[2].body), {'approve': false});
    expect(jsonDecode(calls[3].body), {'reason': 'Grund'});
    expect(jsonDecode(calls[5].body), {'attest': true});
    expect(jsonDecode(calls[7].body), {'reason': 'falsch'});
    final conflict = MockClient((req) async => http.Response('{"error":{"code":"conflict","message":"Urlaubsantrag ist bereits entschieden"}}', 409,
        headers: {'content-type': 'application/json'}));
    for (final call in <Future<Object?> Function()>[
      () => api.decideLeaveRequest('r1', true),
      () => api.stornoLeaveRequest('r1', 'x'),
      () => api.setAbsenceAttest('a1', true),
      () => api.confirmAbsence('a1'),
      () => api.stornoAbsence('a1', 'x'),
    ]) {
      await expectLater(http.runWithClient(call, () => conflict), throwsA(isA<ApiException>()));
    }
  });
}
