import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/my_time_page.dart';

// "Meine Arbeitszeit" (Backlog F.5.1). Die Uhr der Seite ist fest
// vorgegeben (Mittwoch, 14.10.2026), damit kein Test vom Tagesdatum
// abhaengt. InkRipple wie in den anderen Client-Tests (Backlog 0.48).

final _now = DateTime(2026, 10, 14, 9, 30);

class _FakeApi extends ApiClient {
  _FakeApi({List<Map<String, dynamic>>? entries, this.days = const [], this.listError})
      : entries = entries ?? [],
        super(baseUrl: 'http://localhost:8080');

  final List<Map<String, dynamic>> entries;
  List<dynamic> days;
  final Object? listError;
  final List<String> calls = [];

  @override
  Future<Map<String, dynamic>> listTimeEntries({required String from, required String to, String? employeeId, bool includeStorniert = false}) async {
    calls.add('list $from $to ${employeeId ?? '-'} $includeStorniert');
    if (listError != null) throw listError!;
    return {'entries': entries.map((e) => Map<String, dynamic>.from(e)).toList(), 'days': days};
  }

  @override
  Future<Map<String, dynamic>> clockIn() async {
    calls.add('clockIn');
    final e = {
      'id': 'neu',
      'work_date': '2026-10-14T00:00:00Z',
      'started_at': '2026-10-14T07:30:00Z',
      'break_minutes': 0,
      'status': 'active',
    };
    entries.add(e);
    return e;
  }

  @override
  Future<Map<String, dynamic>> clockOut(String entryId, {int breakMinutes = 0}) async {
    calls.add('clockOut $entryId $breakMinutes');
    final e = entries.firstWhere((x) => x['id'] == entryId);
    e['ended_at'] = '2026-10-14T15:30:00Z';
    e['break_minutes'] = breakMinutes;
    return e;
  }
}

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

String _local(String iso) {
  final l = DateTime.parse(iso).toLocal();
  return '${l.hour.toString().padLeft(2, '0')}:${l.minute.toString().padLeft(2, '0')}';
}

void main() {
  group('reine Funktionen', () {
    test('weekRange: Montag bis Sonntag, auch ueber Monats- und Jahresgrenzen', () {
      expect(weekRange(DateTime(2026, 10, 14, 23, 59)), (from: '2026-10-12', to: '2026-10-18'));
      expect(weekRange(DateTime(2026, 10, 12)), (from: '2026-10-12', to: '2026-10-18'));
      expect(weekRange(DateTime(2026, 10, 18, 22)), (from: '2026-10-12', to: '2026-10-18'));
      expect(weekRange(DateTime(2026, 1, 1)), (from: '2025-12-29', to: '2026-01-04'));
      expect(weekRange(DateTime(2026, 3, 1)), (from: '2026-02-23', to: '2026-03-01'));
    });

    test('formatMinutes, formatWorkDay', () {
      expect(formatMinutes(0), '0:00 h');
      expect(formatMinutes(465), '7:45 h');
      expect(formatMinutes(600), '10:00 h');
      expect(formatWorkDay('2026-10-13'), 'Di 13.10.');
      expect(formatWorkDay('2026-10-18T00:00:00Z'), 'So 18.10.');
    });

    test('formatClock: Serverzeit in Ortszeit', () {
      expect(formatClock('2026-10-14T05:30:00Z'), _local('2026-10-14T05:30:00Z'));
      expect(formatClock(null), '');
      expect(formatClock('kaputt'), '');
    });

    test('findRunningEntry: nur aktiv und ohne Ende', () {
      expect(findRunningEntry([
        {'id': 'a', 'status': 'active', 'ended_at': '2026-10-14T10:00:00Z'},
        {'id': 'b', 'status': 'storniert'},
      ]), isNull);
      expect(findRunningEntry([
        {'id': 'a', 'status': 'active', 'ended_at': '2026-10-14T10:00:00Z'},
        {'id': 'c', 'status': 'active'},
      ])?['id'], 'c');
    });
  });

  testWidgets('laedt die Woche des Stichtags und stempelt ein', (tester) async {
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    expect(api.calls, ['list 2026-10-12 2026-10-18 - false']);
    expect(find.text('Nicht eingestempelt'), findsOneWidget);
    expect(find.text('Diese Woche noch keine Zeiten erfasst.'), findsOneWidget);
    expect(find.text('Woche Mo 12.10. – So 18.10.'), findsOneWidget);

    await tester.tap(find.text('Kommen'));
    await tester.pumpAndSettle();
    expect(api.calls, ['list 2026-10-12 2026-10-18 - false', 'clockIn', 'list 2026-10-12 2026-10-18 - false']);
    expect(find.text('Eingestempelt seit ${_local('2026-10-14T07:30:00Z')}'), findsOneWidget);
    expect(find.text('Gehen'), findsOneWidget);
  });

  testWidgets('Gehen fragt die Pause ab und prueft die Eingabe', (tester) async {
    final api = _FakeApi(entries: [
      {'id': 'r1', 'work_date': '2026-10-14T00:00:00Z', 'started_at': '2026-10-14T06:00:00Z', 'break_minutes': 0, 'status': 'active'},
    ]);
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Gehen'));
    await tester.pumpAndSettle();

    for (final bad in ['x', '-5', '']) {
      await tester.enterText(find.widgetWithText(TextField, 'Pause (Minuten)'), bad);
      await tester.tap(find.text('Ausstempeln'));
      await tester.pumpAndSettle();
      expect(find.text('Bitte die Pause als ganze Zahl von Minuten angeben (0 für keine).'), findsOneWidget);
    }
    expect(api.calls.where((c) => c.startsWith('clockOut')), isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Pause (Minuten)'), ' 30 ');
    await tester.tap(find.text('Ausstempeln'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('clockOut')), ['clockOut r1 30']);
    expect(api.calls.last, startsWith('list '));
    expect(find.text('Nicht eingestempelt'), findsOneWidget);
  });

  testWidgets('Abbrechen beim Gehen stempelt nicht aus', (tester) async {
    final api = _FakeApi(entries: [
      {'id': 'r1', 'work_date': '2026-10-14T00:00:00Z', 'started_at': '2026-10-14T06:00:00Z', 'break_minutes': 0, 'status': 'active'},
    ]);
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Gehen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('clockOut')), isEmpty);
    expect(find.text('Gehen'), findsOneWidget);
  });

  testWidgets('Wochenansicht: Tagessumme, Eintraege, Hinweise; stornierte ausgeblendet', (tester) async {
    final api = _FakeApi(
      entries: [
        {'id': 'a', 'work_date': '2026-10-13T00:00:00Z', 'started_at': '2026-10-13T05:00:00Z', 'ended_at': '2026-10-13T16:00:00Z', 'break_minutes': 30, 'status': 'active', 'note': 'Montage'},
        {'id': 'b', 'work_date': '2026-10-13T00:00:00Z', 'started_at': '2026-10-13T04:00:00Z', 'ended_at': '2026-10-13T05:00:00Z', 'break_minutes': 0, 'status': 'storniert'},
      ],
      days: [
        {'work_date': '2026-10-13', 'net_minutes': 630, 'break_minutes': 30, 'open': false, 'hinweise': ['Arbeitszeit über 10 Stunden']},
        {'work_date': '2026-10-14', 'net_minutes': 0, 'break_minutes': 0, 'open': true, 'hinweise': []},
      ],
    );
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    expect(find.text('Di 13.10.: 10:30 h (Pause 30 min)'), findsOneWidget);
    expect(find.text('${_local('2026-10-13T05:00:00Z')} – ${_local('2026-10-13T16:00:00Z')}, Pause 30 min · Montage'), findsOneWidget);
    expect(find.textContaining(_local('2026-10-13T04:00:00Z')), findsNothing);
    expect(find.text('⚠ Arbeitszeit über 10 Stunden'), findsOneWidget);
    expect(find.text('Mi 14.10.: 0:00 h (Pause 0 min) – läuft noch'), findsOneWidget);
    expect(find.text('Diese Woche noch keine Zeiten erfasst.'), findsNothing);
  });

  testWidgets('ohne verknuepften Mitarbeiter ein klarer Hinweis statt Buttons', (tester) async {
    final api = _FakeApi(listError: const ApiException(statusCode: 403, code: 'forbidden', message: 'Kein Mitarbeiter mit diesem Benutzer verknüpft'));
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    expect(find.text('Ihr Login ist mit keinem Mitarbeiter verknüpft. Bitte wenden Sie sich an die Personalabteilung.'), findsOneWidget);
    expect(find.text('Kommen'), findsNothing);
  });

  testWidgets('anderer Ladefehler wird mit Meldung angezeigt', (tester) async {
    final api = _FakeApi(listError: const ApiException(statusCode: 403, code: 'forbidden', message: 'Keine Berechtigung'));
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    expect(find.text('Arbeitszeiten konnten nicht geladen werden: Keine Berechtigung'), findsOneWidget);
  });

  testWidgets('Dashboard zeigt "Meine Arbeitszeit" nur mit time_entries.self', (tester) async {
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {})));
    await tester.pumpAndSettle();
    expect(find.text('Meine Arbeitszeit'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'time_entries.self'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Meine Arbeitszeit'));
    await tester.pumpAndSettle();
    expect(find.byType(MyTimePage), findsOneWidget);
  });

  test('API: Liste mit Zeitraum, Kommen ohne Zeit, Gehen mit Pause', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final status = req.method == 'POST' && req.url.path == '/api/v1/time-entries/' ? 201 : 200;
      final body = req.method == 'GET' ? {'entries': [], 'days': []} : {'id': 'e1'};
      return http.Response.bytes(utf8.encode(jsonEncode(body)), status, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.listTimeEntries(from: '2026-10-12', to: '2026-10-18');
      await api.listTimeEntries(from: '2026-10-12', to: '2026-10-18', employeeId: 'e9', includeStorniert: true);
      await api.clockIn();
      await api.clockOut('e1', breakMinutes: 45);
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
      'GET /api/v1/time-entries/?from=2026-10-12&to=2026-10-18',
      'GET /api/v1/time-entries/?from=2026-10-12&to=2026-10-18&employee_id=e9&include_storniert=true',
      'POST /api/v1/time-entries/?',
      'POST /api/v1/time-entries/e1/stop?',
    ]);
    // Kommen sendet KEINE Zeit - es gilt die Uhr des Servers.
    expect(jsonDecode(calls[2].body), <String, dynamic>{});
    expect(jsonDecode(calls[3].body), {'break_minutes': 45});
    expect(calls[2].headers['content-type'], startsWith('application/json'));

    // Kommen erwartet 201.
    final wrong = MockClient((req) async => http.Response('{"id":"x"}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.clockIn(), () => wrong), throwsA(isA<ApiException>()));
  });
}
