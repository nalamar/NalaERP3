import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/my_leave_page.dart';
import 'package:nalaerp_client/pages/my_time_page.dart' show notLinkedMessage;

// "Mein Urlaub" (Backlog F.8.6.1). Feste Uhr: Mittwoch, 14.10.2026.

final _now = DateTime(2026, 10, 14, 9, 30);

class _FakeApi extends ApiClient {
  _FakeApi({this.accountError, Set<String> permissions = const {'leave_requests.self'}})
      : _perms = permissions,
        super(baseUrl: 'http://localhost:8080');

  final Object? accountError;
  final Set<String> _perms;
  final List<String> calls = [];
  final List<Map<String, dynamic>> created = [];
  final List<Map<String, dynamic>> sickReports = [];
  final List<String> withdrawn = [];

  List<Map<String, dynamic>> requests = [
    {'id': 'r1', 'typ': 'vacation', 'status': 'approved', 'start_date': '2026-06-08T00:00:00Z', 'end_date': '2026-06-12T00:00:00Z', 'days': 5},
    {'id': 'r2', 'typ': 'vacation', 'status': 'pending', 'start_date': '2026-12-21T00:00:00Z', 'end_date': '2026-12-23T00:00:00Z', 'days': 3},
    {'id': 'r0', 'typ': 'vacation', 'status': 'approved', 'start_date': '2025-08-04T00:00:00Z', 'end_date': '2025-08-08T00:00:00Z', 'days': 5},
    {
      'id': 'r3',
      'typ': 'unpaid',
      'status': 'storniert',
      'start_date': '2026-03-02T00:00:00Z',
      'end_date': '2026-03-02T00:00:00Z',
      'days': 0.5,
      'storno_reason': 'Fehlbuchung'
    },
  ];

  @override
  bool hasPermission(String code) => _perms.contains(code);

  @override
  Future<Map<String, dynamic>> getLeaveAccount({String? employeeId, int? year}) async {
    calls.add('account ${employeeId ?? '-'} $year');
    if (accountError != null) throw accountError!;
    return {
      'jahr': year,
      'anspruch_tage': 28,
      'uebertrag_tage': 2.5,
      'genommen_tage': 3,
      'krank_im_urlaub_tage': 2,
      'verplant_tage': 3,
      'rest_tage': 27.5,
      'verfuegbar_tage': 24.5,
    };
  }

  @override
  Future<List<dynamic>> listLeaveRequests({String? employeeId, int? limit}) async {
    calls.add('requests ${employeeId ?? '-'} $limit');
    return requests.map((e) => Map<String, dynamic>.from(e)).toList();
  }

  @override
  Future<List<dynamic>> listAbsences({required String from, required String to, String? employeeId, String? teamId, bool includeStorniert = false}) async {
    calls.add('absences $from $to');
    return [
      {'id': 'a1', 'typ': 'vacation', 'source_request_id': 'r1', 'start_date': '2026-06-08T00:00:00Z', 'end_date': '2026-06-12T00:00:00Z', 'status': 'approved'},
      {
        'id': 's1',
        'typ': 'sick',
        'start_date': '2026-06-10T00:00:00Z',
        'end_date': '2026-06-11T00:00:00Z',
        'status': 'approved',
        'attest': true,
        'gemeldet_von': 'personal',
        'bestaetigt_at': '2026-06-10T08:00:00Z'
      },
      {'id': 's2', 'typ': 'sick', 'start_date': '2026-09-01T00:00:00Z', 'end_date': '2026-09-01T00:00:00Z', 'status': 'approved', 'attest': false, 'gemeldet_von': 'selbst'},
    ];
  }

  @override
  Future<Map<String, dynamic>> createLeaveRequest(Map<String, dynamic> body) async {
    calls.add('create');
    created.add(body);
    return {...body, 'id': 'neu', 'status': 'pending', 'days': body['days'] ?? 2};
  }

  @override
  Future<Map<String, dynamic>> withdrawLeaveRequest(String id) async {
    calls.add('withdraw $id');
    withdrawn.add(id);
    return {'id': id, 'status': 'withdrawn'};
  }

  @override
  Future<Map<String, dynamic>> reportSick(Map<String, dynamic> body) async {
    calls.add('sick');
    sickReports.add(body);
    return {...body, 'id': 'neu', 'typ': 'sick'};
  }
}

Future<void> _pump(WidgetTester tester, ApiClient api) async {
  tester.view.physicalSize = const Size(1400, 1000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(MaterialApp(
    theme: ThemeData(splashFactory: InkRipple.splashFactory),
    home: MyLeavePage(api: api, now: () => _now),
  ));
  await tester.pumpAndSettle();
}

Finder _inDialog(Finder f) => find.descendant(of: find.byType(AlertDialog), matching: f);

void main() {
  test('Formate und Zeilen', () {
    expect(formatLeaveDays(1), '1 Tag');
    expect(formatLeaveDays(5), '5 Tage');
    expect(formatLeaveDays(0.5), '0,5 Tage');
    expect(formatLeaveDays(4.5), '4,5 Tage');
    expect(formatLeaveDays(null), '0 Tage');
    expect(
        leaveRequestLine({'typ': 'vacation', 'status': 'approved', 'start_date': '2026-06-08T00:00:00Z', 'end_date': '2026-06-12T00:00:00Z', 'days': 5}),
        '08.06.2026 – 12.06.2026 · 5 Tage · Urlaub · genehmigt');
    expect(
        leaveRequestLine({
          'typ': 'unpaid',
          'status': 'storniert',
          'start_date': '2026-03-02T00:00:00Z',
          'end_date': '2026-03-02T00:00:00Z',
          'days': 0.5,
          'reason': 'Umzug',
          'storno_reason': 'Fehlbuchung'
        }),
        '02.03.2026 · 0,5 Tage · unbezahlter Urlaub · storniert · Umzug · Storno: Fehlbuchung');
    expect(leaveRequestLine({'typ': 'other', 'status': 'withdrawn', 'start_date': '2026-03-02', 'end_date': '2026-03-03', 'days': 2}),
        '02.03.2026 – 03.03.2026 · 2 Tage · Sonstiges · zurückgezogen');
    expect(sickLine({'start_date': '2026-06-10', 'end_date': '2026-06-11', 'attest': true, 'status': 'approved', 'gemeldet_von': 'personal'}),
        '10.06.2026 – 11.06.2026 · Attest liegt vor');
    expect(sickLine({'start_date': '2026-09-01', 'end_date': '2026-09-01', 'attest': false, 'status': 'approved', 'gemeldet_von': 'selbst'}),
        '01.09.2026 · kein Attest erfasst · noch nicht bestätigt');
    expect(sickLine({'start_date': '2026-09-01', 'end_date': '2026-09-01', 'status': 'storniert', 'gemeldet_von': 'selbst'}),
        '01.09.2026 · kein Attest erfasst · storniert');
    expect(
        accountLines({
          'anspruch_tage': 20,
          'uebertrag_tage': 0,
          'genommen_tage': 5,
          'krank_im_urlaub_tage': 0,
          'verplant_tage': 2,
          'rest_tage': 15,
          'verfuegbar_tage': 13
        }),
        ['Anspruch: 20 Tage', 'Genommen: 5 Tage', 'Beantragt, noch offen: 2 Tage', 'Resturlaub: 15 Tage', 'Noch verfügbar: 13 Tage']);
    expect(overlapsYear({'start_date': '2025-12-29', 'end_date': '2026-01-02'}, 2026), isTrue);
    expect(overlapsYear({'start_date': '2025-12-29', 'end_date': '2026-01-02'}, 2025), isTrue);
    expect(overlapsYear({'start_date': '2025-12-29', 'end_date': '2025-12-31'}, 2026), isFalse);
    expect(overlapsYear({'start_date': '2027-01-04', 'end_date': '2027-01-05'}, 2026), isFalse);
  });

  test('Eingaben fuer Antrag und Krankmeldung', () {
    expect(parseLeaveDays(''), (ok: true, value: null));
    expect(parseLeaveDays('4,5'), (ok: true, value: 4.5));
    expect(parseLeaveDays('3'), (ok: true, value: 3.0));
    for (final bad in ['0', '-1', '1,3', 'x']) {
      expect(parseLeaveDays(bad).ok, isFalse, reason: bad);
    }
    var r = buildLeaveRequestBody(typ: 'vacation', from: '08.06.2026', to: '12.06.2026', reason: '  Sommer ');
    expect(r.body, {'typ': 'vacation', 'start_date': '2026-06-08', 'end_date': '2026-06-12', 'reason': 'Sommer'});
    r = buildLeaveRequestBody(typ: 'unpaid', from: '08.06.2026', to: '08.06.2026', daysText: '0,5');
    expect(r.body, {'typ': 'unpaid', 'start_date': '2026-06-08', 'end_date': '2026-06-08', 'days': 0.5});
    expect(buildLeaveRequestBody(typ: 'vacation', from: '31.02.2026', to: '12.06.2026').error, 'Bitte den ersten Tag als TT.MM.JJJJ angeben.');
    expect(buildLeaveRequestBody(typ: 'vacation', from: '08.06.2026', to: '').error, 'Bitte den letzten Tag als TT.MM.JJJJ angeben.');
    expect(buildLeaveRequestBody(typ: 'vacation', from: '08.06.2026', to: '07.06.2026').error, 'Der letzte Tag liegt vor dem ersten.');
    expect(buildLeaveRequestBody(typ: 'vacation', from: '08.06.2026', to: '09.06.2026', daysText: '1,2').error,
        'Bitte die Tage als ganze oder halbe Zahl angeben oder leer lassen.');
    expect(buildSickBody(from: '10.06.2026', to: '11.06.2026').body, {'start_date': '2026-06-10', 'end_date': '2026-06-11'});
    expect(buildSickBody(from: 'x', to: '11.06.2026').error, 'Bitte den ersten Krankheitstag als TT.MM.JJJJ angeben.');
    expect(buildSickBody(from: '10.06.2026', to: 'x').error, 'Bitte den letzten Krankheitstag als TT.MM.JJJJ angeben.');
    expect(buildSickBody(from: '11.06.2026', to: '10.06.2026').error, 'Der letzte Tag liegt vor dem ersten.');
    expect(requestableLeaveTypes.contains('sick'), isFalse);
  });

  testWidgets('Konto, Antraege des Jahres und Krankmeldungen', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(api.calls, ['account - 2026', 'requests - 200', 'absences 2026-01-01 2026-12-31']);
    expect(find.text('Urlaubsjahr 2026'), findsOneWidget);
    expect(find.text('Anspruch: 28 Tage'), findsOneWidget);
    expect(find.text('Übertrag aus dem Vorjahr: 2,5 Tage'), findsOneWidget);
    expect(find.text('Krank im Urlaub (gutgeschrieben): 2 Tage'), findsOneWidget);
    expect(find.text('Noch verfügbar: 24,5 Tage'), findsOneWidget);
    // Nur 2026, nach Beginn sortiert; der Antrag aus 2025 fehlt.
    final lines = tester.widgetList<Text>(find.textContaining(' · ')).map((t) => t.data!).where((s) => s.contains('Tag')).toList();
    expect(lines, [
      '02.03.2026 · 0,5 Tage · unbezahlter Urlaub · storniert · Storno: Fehlbuchung',
      '08.06.2026 – 12.06.2026 · 5 Tage · Urlaub · genehmigt',
      '21.12.2026 – 23.12.2026 · 3 Tage · Urlaub · offen',
    ]);
    // Zurueckziehen nur beim offenen Antrag.
    expect(find.text('Zurückziehen'), findsOneWidget);
    // Krankmeldungen ohne die Urlaubsabwesenheit.
    expect(find.text('10.06.2026 – 11.06.2026 · Attest liegt vor'), findsOneWidget);
    expect(find.text('01.09.2026 · kein Attest erfasst · noch nicht bestätigt'), findsOneWidget);
    // Die Urlaubsabwesenheit erscheint nicht als Krankmeldung.
    expect(find.textContaining('08.06.2026 – 12.06.2026 · kein Attest'), findsNothing);
  });

  testWidgets('Jahr wechseln', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(find.text('Dieses Jahr'), findsNothing);
    await tester.tap(find.byTooltip('Vorjahr'));
    await tester.pumpAndSettle();
    expect(find.text('Urlaubsjahr 2025'), findsOneWidget);
    expect(api.calls, contains('account - 2025'));
    expect(api.calls, contains('absences 2025-01-01 2025-12-31'));
    expect(find.text('04.08.2025 – 08.08.2025 · 5 Tage · Urlaub · genehmigt'), findsOneWidget);
    await tester.tap(find.text('Dieses Jahr'));
    await tester.pumpAndSettle();
    expect(find.text('Urlaubsjahr 2026'), findsOneWidget);
    await tester.tap(find.byTooltip('Folgejahr'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('account - 2027'));
  });

  testWidgets('Urlaub beantragen mit Pruefung im Dialog', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.text('Urlaub beantragen'));
    await tester.pumpAndSettle();
    final fields = _inDialog(find.byType(TextField));
    expect(tester.widget<TextField>(fields.at(0)).controller!.text, '14.10.2026');
    await tester.enterText(fields.at(1), '13.10.2026');
    await tester.tap(_inDialog(find.text('Beantragen')));
    await tester.pumpAndSettle();
    expect(find.text('Der letzte Tag liegt vor dem ersten.'), findsOneWidget);
    expect(api.created, isEmpty);
    await tester.enterText(fields.at(1), '16.10.2026');
    await tester.enterText(fields.at(2), '2,5');
    await tester.enterText(fields.at(3), 'Herbst');
    await tester.tap(_inDialog(find.byType(DropdownButtonFormField<String>)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('unbezahlter Urlaub').last);
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Beantragen')));
    await tester.pumpAndSettle();
    expect(api.created, [
      {'typ': 'unpaid', 'start_date': '2026-10-14', 'end_date': '2026-10-16', 'days': 2.5, 'reason': 'Herbst'}
    ]);
    expect(find.text('Antrag gestellt: 2,5 Tage'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('account')).length, 2);
  });

  testWidgets('Antrag zurueckziehen und abbrechen', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.text('Zurückziehen'));
    await tester.pumpAndSettle();
    expect(_inDialog(find.text('21.12.2026 – 23.12.2026 · 3 Tage · Urlaub · offen')), findsOneWidget);
    await tester.tap(_inDialog(find.text('Abbrechen')));
    await tester.pumpAndSettle();
    expect(api.withdrawn, isEmpty);
    await tester.tap(find.text('Zurückziehen'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Zurückziehen')));
    await tester.pumpAndSettle();
    expect(api.withdrawn, ['r2']);
    expect(find.text('Antrag zurückgezogen'), findsOneWidget);
  });

  testWidgets('Krank melden sendet nur den Zeitraum', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.text('Krank melden'));
    await tester.pumpAndSettle();
    expect(find.textContaining('keine Diagnose'), findsOneWidget);
    final fields = _inDialog(find.byType(TextField));
    expect(fields, findsNWidgets(2));
    await tester.enterText(fields.at(1), 'morgen');
    await tester.tap(_inDialog(find.text('Melden')));
    await tester.pumpAndSettle();
    expect(find.text('Bitte den letzten Krankheitstag als TT.MM.JJJJ angeben.'), findsOneWidget);
    await tester.enterText(fields.at(1), '15.10.2026');
    await tester.tap(_inDialog(find.text('Melden')));
    await tester.pumpAndSettle();
    expect(api.sickReports, [
      {'start_date': '2026-10-14', 'end_date': '2026-10-15'}
    ]);
    expect(find.text('Krankmeldung übermittelt - gute Besserung'), findsOneWidget);
  });

  testWidgets('Ohne verknuepften Mitarbeiter: Hinweis, keine Buttons', (tester) async {
    final api = _FakeApi(accountError: ApiException(statusCode: 403, code: 'forbidden', message: notLinkedMessage));
    await _pump(tester, api);
    expect(find.text('Ihr Login ist mit keinem Mitarbeiter verknüpft. Bitte wenden Sie sich an die Personalabteilung.'), findsOneWidget);
    expect(find.text('Urlaub beantragen'), findsNothing);
    expect(find.text('Krank melden'), findsNothing);
    final other = _FakeApi(accountError: ApiException(statusCode: 500, code: 'internal_error', message: 'kaputt'));
    // Neue Seite statt Wiederverwendung des Zustands.
    await tester.pumpWidget(const SizedBox());
    await _pump(tester, other);
    expect(find.text('Urlaubsdaten konnten nicht geladen werden: kaputt'), findsOneWidget);
    expect(find.text('Urlaub beantragen'), findsOneWidget);
  });

  testWidgets('Dashboard: "Mein Urlaub" nur mit leave_requests.self', (tester) async {
    final api = _FakeApi();
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(MaterialApp(home: DashboardPage(api: api, onLogout: () async {}, permissions: const {'time_entries.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Mein Urlaub'), findsNothing);
    await tester.pumpWidget(MaterialApp(
        theme: ThemeData(splashFactory: InkRipple.splashFactory),
        home: DashboardPage(api: api, onLogout: () async {}, permissions: const {'leave_requests.self'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Mein Urlaub'));
    await tester.pumpAndSettle();
    expect(find.byType(MyLeavePage), findsOneWidget);
  });

  test('API: Pfade, Parameter, Bodies, Statuscodes', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final isList = req.method == 'GET' && (req.url.path == '/api/v1/leave-requests/' || req.url.path == '/api/v1/absences/');
      final created = req.method == 'POST' && (req.url.path == '/api/v1/leave-requests/' || req.url.path == '/api/v1/absences/sick');
      return http.Response.bytes(utf8.encode(isList ? '[]' : '{"id":"x"}'), created ? 201 : 200, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.listLeaveRequests();
      await api.listLeaveRequests(employeeId: 'e1', limit: 50);
      await api.createLeaveRequest({'typ': 'vacation'});
      await api.withdrawLeaveRequest('r1');
      await api.getLeaveAccount();
      await api.getLeaveAccount(employeeId: 'e1', year: 2026);
      await api.reportSick({'start_date': '2026-06-10'});
      await api.listAbsences(from: '2026-01-01', to: '2026-12-31');
      await api.listAbsences(from: '2026-01-01', to: '2026-01-31', employeeId: 'e1', teamId: 't1', includeStorniert: true);
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
      'GET /api/v1/leave-requests/?',
      'GET /api/v1/leave-requests/?employee_id=e1&limit=50',
      'POST /api/v1/leave-requests/?',
      'POST /api/v1/leave-requests/r1/withdraw?',
      'GET /api/v1/leave-accounts/?',
      'GET /api/v1/leave-accounts/?employee_id=e1&year=2026',
      'POST /api/v1/absences/sick?',
      'GET /api/v1/absences/?from=2026-01-01&to=2026-12-31',
      'GET /api/v1/absences/?from=2026-01-01&to=2026-01-31&employee_id=e1&team_id=t1&include_storniert=true',
    ]);
    expect(jsonDecode(calls[2].body), {'typ': 'vacation'});
    expect(jsonDecode(calls[6].body), {'start_date': '2026-06-10'});
    final ok200 = MockClient((req) async => http.Response('{"id":"x"}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.createLeaveRequest({}), () => ok200), throwsA(isA<ApiException>()));
    await expectLater(http.runWithClient(() => api.reportSick({}), () => ok200), throwsA(isA<ApiException>()));
    final conflict = MockClient((req) async => http.Response('{"error":{"code":"conflict","message":"Urlaubsantrag ist bereits entschieden"}}', 409,
        headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.withdrawLeaveRequest('r1'), () => conflict), throwsA(isA<ApiException>()));
  });
}
