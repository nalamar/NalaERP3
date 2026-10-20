import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/teams_page.dart';

// Teams pflegen (Backlog F.8.6.4.2).

class _FakeApi extends ApiClient {
  _FakeApi({this.perms = const {'employees.read', 'employees.write'}, this.deleteError}) : super(baseUrl: 'http://localhost:8080');

  final Set<String> perms;
  final Object? deleteError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> bodies = [];

  @override
  bool hasPermission(String code) => perms.contains(code);

  @override
  Future<List<dynamic>> listTeams() async {
    calls.add('teams');
    return [
      {'id': 't1', 'name': 'Montage', 'lead_employee_id': 'e1', 'mitglieder': 3},
      {'id': 't2', 'name': 'Büro', 'mitglieder': 1},
    ];
  }

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async => [
        {'id': 'e2', 'first_name': 'Carl', 'last_name': 'Dorn'},
        {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg'},
        {'id': 'e3', 'first_name': 'Ex', 'last_name': 'Aus', 'active': false},
      ];

  @override
  Future<Map<String, dynamic>> createTeam(Map<String, dynamic> body) async {
    calls.add('create');
    bodies.add(body);
    return {'id': 'neu'};
  }

  @override
  Future<Map<String, dynamic>> updateTeam(String id, Map<String, dynamic> body) async {
    calls.add('update $id');
    bodies.add(body);
    return {'id': id};
  }

  @override
  Future<void> deleteTeam(String id) async {
    calls.add('delete $id');
    if (deleteError != null) throw deleteError!;
  }
}

Future<void> _pump(WidgetTester tester, ApiClient api) async {
  await tester.binding.setSurfaceSize(const Size(1200, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(theme: ThemeData(splashFactory: InkRipple.splashFactory), home: TeamsPage(api: api)));
  await tester.pumpAndSettle();
}

Finder _inDialog(Finder f) => find.descendant(of: find.byType(AlertDialog), matching: f);

void main() {
  test('Zeile', () {
    final names = {'e1': 'Berg, Anna'};
    expect(teamLine({'name': 'Montage', 'lead_employee_id': 'e1', 'mitglieder': 3}, names), 'Montage · Teamleiter: Berg, Anna · 3 Mitglieder');
    expect(teamLine({'name': 'Büro', 'mitglieder': 1}, names), 'Büro · 1 Mitglied');
    expect(teamLine({'name': 'Lager', 'lead_employee_id': 'x'}, names), 'Lager · Teamleiter: unbekannt · 0 Mitglieder');
  });

  testWidgets('Liste, anlegen mit Teamleiter, bearbeiten ohne Teamleiter', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(find.text('Montage · Teamleiter: Berg, Anna · 3 Mitglieder'), findsOneWidget);
    expect(find.text('Büro · 1 Mitglied'), findsOneWidget);
    await tester.tap(find.byTooltip('Team anlegen'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Anlegen')));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Namen angeben.'), findsOneWidget);
    await tester.enterText(_inDialog(find.byType(TextField)), '  Lager ');
    await tester.tap(_inDialog(find.byType(DropdownButtonFormField<String?>)));
    await tester.pumpAndSettle();
    // Nur aktive, nach Namen.
    expect(find.text('Aus, Ex'), findsNothing);
    await tester.tap(find.text('Dorn, Carl').last);
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Anlegen')));
    await tester.pumpAndSettle();
    expect(api.bodies.last, {'name': 'Lager', 'lead_employee_id': 'e2'});
    expect(find.text('Team angelegt'), findsOneWidget);
    expect(api.calls.where((c) => c == 'teams').length, 2);
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(_inDialog(find.byType(TextField))).controller!.text, 'Montage');
    await tester.tap(_inDialog(find.byType(DropdownButtonFormField<String?>)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('kein Teamleiter').last);
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Speichern')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('update t1'));
    expect(api.bodies.last, {'name': 'Montage', 'lead_employee_id': null});
  });

  testWidgets('Loeschen mit Rueckfrage; Ablehnung des Servers als Meldung', (tester) async {
    final api = _FakeApi(deleteError: const ApiException(statusCode: 409, code: 'conflict', message: 'Team hat noch Mitglieder - zuerst umsetzen'));
    await _pump(tester, api);
    await tester.tap(find.byTooltip('Löschen').last);
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Abbrechen')));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('delete')), isEmpty);
    await tester.tap(find.byTooltip('Löschen').last);
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Löschen')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('delete t2'));
    expect(find.text('Fehler: Team hat noch Mitglieder - zuerst umsetzen'), findsOneWidget);
  });

  testWidgets('Ohne Schreibrecht nur lesen', (tester) async {
    final api = _FakeApi(perms: const {'employees.read'});
    await _pump(tester, api);
    expect(find.text('Montage · Teamleiter: Berg, Anna · 3 Mitglieder'), findsOneWidget);
    expect(find.byTooltip('Team anlegen'), findsNothing);
    expect(find.byTooltip('Bearbeiten'), findsNothing);
    expect(find.byTooltip('Löschen'), findsNothing);
  });

  test('API: Teams anlegen, aendern, loeschen', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final status = req.method == 'POST' ? 201 : (req.method == 'DELETE' ? 204 : 200);
      return http.Response.bytes(utf8.encode(req.method == 'DELETE' ? '' : '{"id":"t"}'), status, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.createTeam({'name': 'A'});
      await api.updateTeam('t1', {'name': 'B', 'lead_employee_id': null});
      await api.deleteTeam('t1');
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}'), [
      'POST /api/v1/employees/teams',
      'PUT /api/v1/employees/teams/t1',
      'DELETE /api/v1/employees/teams/t1',
    ]);
    expect(jsonDecode(calls[1].body), {'name': 'B', 'lead_employee_id': null});
    final ok200 = MockClient((req) async => http.Response('{}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.createTeam({}), () => ok200), throwsA(isA<ApiException>()));
    await expectLater(http.runWithClient(() => api.deleteTeam('t1'), () => ok200), throwsA(isA<ApiException>()));
    final conflict = MockClient((req) async => http.Response('{"error":{"code":"conflict","message":"x"}}', 409, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.updateTeam('t1', {}), () => conflict), throwsA(isA<ApiException>()));
  });
}
