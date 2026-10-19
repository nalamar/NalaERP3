import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/leave_accounts_page.dart';

// Urlaubskonten und Resturlaub (Backlog F.8.6.2.2). Feste Uhr: 14.10.2026.

final _now = DateTime(2026, 10, 14, 9, 30);

class _FakeApi extends ApiClient {
  _FakeApi({this.perms = const {'leave_requests.read', 'leave_requests.decide', 'employees.read'}}) : super(baseUrl: 'http://localhost:8080');

  final Set<String> perms;
  final List<String> calls = [];
  final List<Map<String, dynamic>> saved = [];

  @override
  bool hasPermission(String code) => perms.contains(code);

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async {
    calls.add('employees');
    return [
      {'id': 'e2', 'first_name': 'Carl', 'last_name': 'Dorn'},
      {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg'},
      {'id': 'e3', 'first_name': 'Ex', 'last_name': 'Aus', 'active': false},
    ];
  }

  @override
  Future<List<dynamic>> listRemainingLeave({int? year}) async {
    calls.add('remaining $year');
    return [
      {
        'employee_id': 'e1',
        'first_name': 'Anna',
        'last_name': 'Berg',
        'konto': {'rest_tage': 15, 'hinweis_erteilt_am': '2026-10-01T00:00:00Z'}
      },
      {
        'employee_id': 'e2',
        'first_name': 'Carl',
        'last_name': 'Dorn',
        'konto': {'rest_tage': 2.5}
      },
    ];
  }

  @override
  Future<Map<String, dynamic>> getLeaveAccount({String? employeeId, int? year}) async {
    calls.add('account $employeeId $year');
    return {
      'employee_id': employeeId,
      'jahr': year,
      'anspruch_tage': 20,
      'uebertrag_tage': 0,
      'genommen_tage': 5,
      'verplant_tage': 0,
      'rest_tage': 15,
      'verfuegbar_tage': 15,
      'festgelegt': employeeId == 'e1',
      'notiz': employeeId == 'e1' ? 'Eintritt Juli' : '',
      if (employeeId == 'e1') 'hinweis_erteilt_am': '2026-10-01T00:00:00Z',
      if (employeeId == 'e2') 'teilurlaub': {'tage': 8.5, 'monate': 5, 'grund': 'Teilurlaub: 1/12 je vollem Beschäftigungsmonat (§ 5 BUrlG)'},
    };
  }

  @override
  Future<Map<String, dynamic>> setLeaveAccount(String employeeId, int year, Map<String, dynamic> body) async {
    calls.add('set $employeeId $year');
    saved.add(body);
    return {};
  }
}

Future<void> _pump(WidgetTester tester, ApiClient api) async {
  tester.view.physicalSize = const Size(1400, 1200);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(MaterialApp(
    theme: ThemeData(splashFactory: InkRipple.splashFactory),
    home: LeaveAccountsPage(api: api, now: () => _now),
  ));
  await tester.pumpAndSettle();
}

Finder _inDialog(Finder f) => find.descendant(of: find.byType(AlertDialog), matching: f);

void main() {
  test('Eingaben und Zeilen', () {
    expect(parseAccountDays(''), (ok: true, value: null));
    expect(parseAccountDays('0'), (ok: true, value: 0.0));
    expect(parseAccountDays('12,5'), (ok: true, value: 12.5));
    for (final bad in ['-1', '367', '1,2', 'x']) {
      expect(parseAccountDays(bad).ok, isFalse, reason: bad);
    }
    expect(buildAccountBody(anspruch: '12,5', uebertrag: '', hinweis: '01.10.2026', notiz: ' Eintritt ').body,
        {'anspruch_tage': 12.5, 'uebertrag_tage': 0, 'notiz': 'Eintritt', 'hinweis_erteilt_am': '2026-10-01'});
    expect(buildAccountBody(anspruch: '0', uebertrag: '3').body, {'anspruch_tage': 0.0, 'uebertrag_tage': 3.0, 'notiz': ''});
    expect(buildAccountBody(anspruch: '').error, 'Bitte den Anspruch in ganzen oder halben Tagen angeben (0 bis 366).');
    expect(buildAccountBody(anspruch: '20', uebertrag: '-2').error, 'Bitte den Übertrag in ganzen oder halben Tagen angeben oder leer lassen.');
    expect(buildAccountBody(anspruch: '20', hinweis: '31.02.2026').error, 'Bitte das Datum des Hinweises als TT.MM.JJJJ angeben oder leer lassen.');
    expect(
        remainingLine({
          'first_name': 'Anna',
          'last_name': 'Berg',
          'konto': {'rest_tage': 15, 'hinweis_erteilt_am': '2026-10-01T00:00:00Z'}
        }),
        'Berg, Anna · Rest 15 Tage · Hinweis erteilt am 01.10.2026');
    expect(
        remainingLine({
          'first_name': '',
          'last_name': 'Dorn',
          'konto': {'rest_tage': 2.5}
        }),
        'Dorn · Rest 2,5 Tage · Hinweis auf Verfall fehlt');
  });

  testWidgets('Resturlaub, Auswahl per Liste, Konto mit Hinweis', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(api.calls, ['employees', 'remaining 2026']);
    expect(find.text('Berg, Anna · Rest 15 Tage · Hinweis erteilt am 01.10.2026'), findsOneWidget);
    expect(find.text('Dorn, Carl · Rest 2,5 Tage · Hinweis auf Verfall fehlt'), findsOneWidget);
    await tester.tap(find.text('Berg, Anna · Rest 15 Tage · Hinweis erteilt am 01.10.2026'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'account e1 2026');
    expect(find.text('Konto festgelegt'), findsOneWidget);
    expect(find.text('Hinweis auf Resturlaub und Verfall erteilt am 01.10.2026'), findsOneWidget);
    expect(find.text('Notiz: Eintritt Juli'), findsOneWidget);
    expect(find.text('Resturlaub: 15 Tage'), findsOneWidget);
    expect(find.textContaining('Vorschlag Teilurlaub'), findsNothing);
    // Jahr wechseln laedt Konto und Liste neu.
    await tester.tap(find.byTooltip('Vorjahr'));
    await tester.pumpAndSettle();
    expect(api.calls, containsAll(['remaining 2025', 'account e1 2025']));
  });

  testWidgets('Auswahl: nur aktive, nach Namen sortiert', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    final items = tester.widgetList<Text>(find.descendant(of: find.byType(DropdownMenuItem<String>), matching: find.byType(Text))).map((t) => t.data).toList();
    expect(items.where((s) => s == 'Aus, Ex'), isEmpty);
    expect(items.toSet().toList(), ['Berg, Anna', 'Dorn, Carl']);
    await tester.tap(find.text('Dorn, Carl').last);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'account e2 2026');
    expect(find.text('Konto nicht festgelegt - es gilt der Jahresanspruch des Mitarbeiters'), findsOneWidget);
    expect(find.text('Vorschlag Teilurlaub: 8,5 Tage – Teilurlaub: 1/12 je vollem Beschäftigungsmonat (§ 5 BUrlG)'), findsOneWidget);
  });

  testWidgets('Konto festlegen: Vorschlag uebernehmen, Pruefung, Speichern', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.text('Dorn, Carl · Rest 2,5 Tage · Hinweis auf Verfall fehlt'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Konto festlegen'));
    await tester.pumpAndSettle();
    final fields = _inDialog(find.byType(TextField));
    expect(tester.widget<TextField>(fields.at(0)).controller!.text, '20');
    await tester.tap(_inDialog(find.text('Übernehmen')));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(fields.at(0)).controller!.text, '8,5');
    await tester.enterText(fields.at(2), '31.02.2026');
    await tester.tap(_inDialog(find.text('Speichern')));
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Datum des Hinweises als TT.MM.JJJJ angeben oder leer lassen.'), findsOneWidget);
    expect(api.saved, isEmpty);
    await tester.enterText(fields.at(1), '1,5');
    await tester.enterText(fields.at(2), '15.10.2026');
    await tester.enterText(fields.at(3), 'Eintritt 02.07.');
    await tester.tap(_inDialog(find.text('Speichern')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('set e2 2026'));
    expect(api.saved, [
      {'anspruch_tage': 8.5, 'uebertrag_tage': 1.5, 'notiz': 'Eintritt 02.07.', 'hinweis_erteilt_am': '2026-10-15'}
    ]);
    expect(find.text('Urlaubskonto gespeichert'), findsOneWidget);
    expect(api.calls.where((c) => c == 'account e2 2026').length, 2);
  });

  testWidgets('Ohne Entscheidungsrecht kein Festlegen', (tester) async {
    final api = _FakeApi(perms: const {'leave_requests.read', 'employees.read'});
    await _pump(tester, api);
    await tester.tap(find.text('Berg, Anna · Rest 15 Tage · Hinweis erteilt am 01.10.2026'));
    await tester.pumpAndSettle();
    expect(find.text('Resturlaub: 15 Tage'), findsOneWidget);
    expect(find.text('Konto festlegen'), findsNothing);
  });

  test('API: Resturlaub und Konto festlegen', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      return http.Response.bytes(utf8.encode(req.method == 'GET' ? '[]' : '{"jahr":2026}'), 200, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.listRemainingLeave();
      await api.listRemainingLeave(year: 2025);
      await api.setLeaveAccount('e1', 2026, {'anspruch_tage': 20});
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
      'GET /api/v1/leave-accounts/remaining?',
      'GET /api/v1/leave-accounts/remaining?year=2025',
      'PUT /api/v1/leave-accounts/e1/2026?',
    ]);
    expect(jsonDecode(calls[2].body), {'anspruch_tage': 20});
    final bad = MockClient((req) async => http.Response('{"error":{"code":"validation_error","message":"anspruch_tage erforderlich"}}', 400,
        headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.setLeaveAccount('e1', 2026, {}), () => bad), throwsA(isA<ApiException>()));
  });
}
