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

  final List<Map<String, dynamic>> created = [];
  final List<Map<String, dynamic>> corrections = [];
  final List<MapEntry<String, String>> stornos = [];

  @override
  Future<Map<String, dynamic>> createTimeEntry(
      {required String startedAt, required String endedAt, int breakMinutes = 0, String note = '', String? employeeId}) async {
    calls.add('create');
    final e = {'started_at': startedAt, 'ended_at': endedAt, 'break_minutes': breakMinutes, 'note': note, 'employee_id': employeeId};
    created.add(e);
    return e;
  }

  @override
  Future<Map<String, dynamic>> correctTimeEntry(String id,
      {required String startedAt, required String endedAt, int breakMinutes = 0, String note = '', required String reason}) async {
    calls.add('correct');
    final e = {'id': id, 'started_at': startedAt, 'ended_at': endedAt, 'break_minutes': breakMinutes, 'note': note, 'reason': reason};
    corrections.add(e);
    return e;
  }

  @override
  Future<Map<String, dynamic>> stornoTimeEntry(String id, String reason) async {
    calls.add('storno');
    stornos.add(MapEntry(id, reason));
    return {'id': id, 'status': 'storniert'};
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
    expect(find.text('In dieser Woche keine Zeiten erfasst.'), findsOneWidget);
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
    expect(find.text('In dieser Woche keine Zeiten erfasst.'), findsNothing);
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

  group('Eingaben fuer Nachtragen und Korrigieren', () {
    test('parseClockInput', () {
      expect(parseClockInput('7:05'), 425);
      expect(parseClockInput(' 07:05 '), 425);
      expect(parseClockInput('23:59'), 1439);
      expect(parseClockInput('24:00'), isNull);
      expect(parseClockInput('7:60'), isNull);
      expect(parseClockInput('705'), isNull);
      expect(parseClockInput(''), isNull);
    });

    String utc(int y, int m, int d, int h, int min) => DateTime(y, m, d, h, min).toUtc().toIso8601String();

    test('parseEntryForm: Ortszeit nach UTC, Nachtschicht ueber Mitternacht', () {
      final day = parseEntryForm(date: '13.10.2026', start: '7:00', end: '15:30', pause: '30');
      expect(day.error, isNull);
      expect(day.times!.startedAt, utc(2026, 10, 13, 7, 0));
      expect(day.times!.endedAt, utc(2026, 10, 13, 15, 30));
      expect(day.times!.breakMinutes, 30);
      expect(day.times!.overnight, isFalse);

      final night = parseEntryForm(date: '31.12.2026', start: '22:00', end: '06:00', pause: '0');
      expect(night.times!.startedAt, utc(2026, 12, 31, 22, 0));
      expect(night.times!.endedAt, utc(2027, 1, 1, 6, 0));
      expect(night.times!.overnight, isTrue);
    });

    test('parseEntryForm: Fehler je Feld', () {
      expect(parseEntryForm(date: '32.10.2026', start: '7:00', end: '8:00', pause: '0').error, 'Bitte das Datum als TT.MM.JJJJ angeben.');
      expect(parseEntryForm(date: '13.10.2026', start: '7', end: '8:00', pause: '0').error, 'Bitte den Beginn als HH:MM angeben.');
      expect(parseEntryForm(date: '13.10.2026', start: '7:00', end: '', pause: '0').error, 'Bitte das Ende als HH:MM angeben.');
      expect(parseEntryForm(date: '13.10.2026', start: '7:00', end: '07:00', pause: '0').error, 'Beginn und Ende dürfen nicht gleich sein.');
      expect(parseEntryForm(date: '13.10.2026', start: '7:00', end: '8:00', pause: '-1').error,
          'Bitte die Pause als ganze Zahl von Minuten angeben (0 für keine).');
    });
  });

  testWidgets('Wochen blaettern: Vorwoche ohne Kommen/Gehen, zurueck zur aktuellen', (tester) async {
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Vorwoche'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list 2026-10-05 2026-10-11 - false');
    expect(find.text('Woche Mo 05.10. – So 11.10.'), findsOneWidget);
    expect(find.text('Kommen'), findsNothing);

    await tester.tap(find.byTooltip('Nächste Woche'));
    await tester.tap(find.byTooltip('Nächste Woche'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list 2026-10-19 2026-10-25 - false');

    await tester.tap(find.text('Diese Woche'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list 2026-10-12 2026-10-18 - false');
    expect(find.text('Kommen'), findsOneWidget);
    expect(find.text('Diese Woche'), findsNothing);
  });

  testWidgets('Stornierte anzeigen laedt mit include_storniert und zeigt die Begruendung', (tester) async {
    final api = _FakeApi(entries: [
      {'id': 'a', 'work_date': '2026-10-13T00:00:00Z', 'started_at': '2026-10-13T05:00:00Z', 'ended_at': '2026-10-13T06:00:00Z', 'break_minutes': 0, 'status': 'storniert', 'storno_reason': 'Fehlstempelung'},
    ]);
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    expect(find.textContaining('Fehlstempelung'), findsNothing);
    await tester.tap(find.text('Stornierte anzeigen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list 2026-10-12 2026-10-18 - true');
    expect(find.text('${_local('2026-10-13T05:00:00Z')} – ${_local('2026-10-13T06:00:00Z')}, Pause 0 min – storniert: Fehlstempelung'), findsOneWidget);
    expect(find.text('Di 13.10.'), findsOneWidget);
    // Stornierte Eintraege haben keine Aktionen.
    expect(find.byTooltip('Korrigieren'), findsNothing);
    expect(find.byTooltip('Stornieren'), findsNothing);
  });

  testWidgets('Zeit nachtragen: Pruefung im Dialog, dann Aufruf mit UTC-Zeiten', (tester) async {
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Zeit nachtragen'));
    await tester.pumpAndSettle();
    // Vorbelegt mit dem heutigen Datum.
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)')).controller!.text, '14.10.2026');

    await tester.enterText(find.widgetWithText(TextField, 'Beginn (HH:MM)'), '7');
    await tester.tap(find.widgetWithText(FilledButton, 'Nachtragen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte den Beginn als HH:MM angeben.'), findsOneWidget);
    expect(api.created, isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)'), '13.10.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Beginn (HH:MM)'), '07:00');
    await tester.enterText(find.widgetWithText(TextField, 'Ende (HH:MM, vor Beginn = Folgetag)'), '15:30');
    await tester.enterText(find.widgetWithText(TextField, 'Pause (Minuten)'), '30');
    await tester.enterText(find.widgetWithText(TextField, 'Notiz'), ' Baustelle ');
    await tester.tap(find.widgetWithText(FilledButton, 'Nachtragen'));
    await tester.pumpAndSettle();
    expect(api.created.single, {
      'started_at': DateTime(2026, 10, 13, 7).toUtc().toIso8601String(),
      'ended_at': DateTime(2026, 10, 13, 15, 30).toUtc().toIso8601String(),
      'break_minutes': 30,
      'note': 'Baustelle',
      'employee_id': null,
    });
    expect(api.calls.last, startsWith('list '));
  });

  testWidgets('Korrigieren: vorbelegt, nur mit Begruendung', (tester) async {
    final api = _FakeApi(
      entries: [
        {'id': 'k1', 'work_date': '2026-10-13T00:00:00Z', 'started_at': '2026-10-13T05:00:00Z', 'ended_at': '2026-10-13T14:00:00Z', 'break_minutes': 45, 'status': 'active', 'note': 'Montage'},
      ],
      days: [
        {'work_date': '2026-10-13', 'net_minutes': 495, 'break_minutes': 45, 'open': false, 'hinweise': []},
      ],
    );
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Korrigieren'));
    await tester.pumpAndSettle();
    String field(String label) => tester.widget<TextField>(find.widgetWithText(TextField, label)).controller!.text;
    expect(field('Beginn (HH:MM)'), _local('2026-10-13T05:00:00Z'));
    expect(field('Ende (HH:MM, vor Beginn = Folgetag)'), _local('2026-10-13T14:00:00Z'));
    expect(field('Pause (Minuten)'), '45');
    expect(field('Notiz'), 'Montage');

    await tester.enterText(find.widgetWithText(TextField, 'Pause (Minuten)'), '60');
    await tester.tap(find.widgetWithText(FilledButton, 'Korrigieren'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Korrektur begründen.'), findsOneWidget);
    expect(api.corrections, isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Begründung der Korrektur'), ' Pause vergessen ');
    await tester.tap(find.widgetWithText(FilledButton, 'Korrigieren'));
    await tester.pumpAndSettle();
    final c = api.corrections.single;
    expect(c['id'], 'k1');
    expect(c['break_minutes'], 60);
    expect(c['reason'], 'Pause vergessen');
    expect(c['note'], 'Montage');
    expect(DateTime.parse(c['started_at'] as String), DateTime.parse('2026-10-13T05:00:00Z'));
    expect(DateTime.parse(c['ended_at'] as String), DateTime.parse('2026-10-13T14:00:00Z'));
  });

  testWidgets('Stornieren nur mit Begruendung; laufender Eintrag: stornierbar, nicht korrigierbar', (tester) async {
    final api = _FakeApi(
      entries: [
        {'id': 'r1', 'work_date': '2026-10-14T00:00:00Z', 'started_at': '2026-10-14T06:00:00Z', 'break_minutes': 0, 'status': 'active'},
      ],
      days: [
        {'work_date': '2026-10-14', 'net_minutes': 0, 'break_minutes': 0, 'open': true, 'hinweise': []},
      ],
    );
    await tester.pumpWidget(_wrap(MyTimePage(api: api, now: () => _now)));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Korrigieren'), findsNothing);
    await tester.tap(find.byTooltip('Stornieren'));
    await tester.pumpAndSettle();
    final button = find.widgetWithText(FilledButton, 'Stornieren');
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), ' Fehlstempelung ');
    await tester.pumpAndSettle();
    await tester.tap(button);
    await tester.pumpAndSettle();
    expect(api.stornos.single.key, 'r1');
    expect(api.stornos.single.value, 'Fehlstempelung');
    expect(api.calls.last, startsWith('list '));
  });

  test('API: nachtragen, korrigieren, stornieren', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final status = req.url.path.endsWith('/storno') ? 200 : 201;
      return http.Response.bytes(utf8.encode(jsonEncode({'id': 'x'})), status, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.createTimeEntry(startedAt: 'S', endedAt: 'E', breakMinutes: 15, note: 'n');
      await api.createTimeEntry(startedAt: 'S', endedAt: 'E', employeeId: 'e9');
      await api.correctTimeEntry('k1', startedAt: 'S2', endedAt: 'E2', breakMinutes: 5, note: 'm', reason: 'r');
      await api.stornoTimeEntry('k2', 'falsch');
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}'), [
      'POST /api/v1/time-entries/',
      'POST /api/v1/time-entries/',
      'POST /api/v1/time-entries/k1/correct',
      'POST /api/v1/time-entries/k2/storno',
    ]);
    expect(jsonDecode(calls[0].body), {'started_at': 'S', 'ended_at': 'E', 'break_minutes': 15, 'note': 'n'});
    expect(jsonDecode(calls[1].body), {'started_at': 'S', 'ended_at': 'E', 'break_minutes': 0, 'note': '', 'employee_id': 'e9'});
    expect(jsonDecode(calls[2].body), {'started_at': 'S2', 'ended_at': 'E2', 'break_minutes': 5, 'note': 'm', 'reason': 'r'});
    expect(jsonDecode(calls[3].body), {'reason': 'falsch'});
    for (final c in calls) {
      expect(c.headers['content-type'], startsWith('application/json'));
    }

    // Nachtragen und Korrigieren erwarten 201, Stornieren 200.
    final ok200 = MockClient((req) async => http.Response('{"id":"x"}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.createTimeEntry(startedAt: 'S', endedAt: 'E'), () => ok200), throwsA(isA<ApiException>()));
    await expectLater(
        http.runWithClient(() => api.correctTimeEntry('k', startedAt: 'S', endedAt: 'E', reason: 'r'), () => ok200), throwsA(isA<ApiException>()));
    final err = MockClient((req) async =>
        http.Response.bytes(utf8.encode('{"error":{"code":"conflict","message":"Zeiteintrag ist bereits storniert"}}'), 409, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.stornoTimeEntry('k', 'r'), () => err),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Zeiteintrag ist bereits storniert')));
  });
}
