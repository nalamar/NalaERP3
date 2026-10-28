import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/vehicle_assignments_section.dart';
import 'package:nalaerp_client/pages/vehicle_detail_page.dart';

// Fahrerzuordnung im Client (Backlog G.3a.3). InkRipple und
// Desktop-Fenster wie in den anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, List<Map<String, dynamic>>? assignments, this.hints = const [], this.assignError})
      : assignments = assignments ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> assignments;
  final List<String> hints;
  final Object? assignError;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listFleetAssignments({String? vehicleId, String? employeeId, bool nurOffen = false, bool includeStorniert = false}) async {
    calls.add('list $vehicleId');
    // Wie der Server: stornierte nur auf Wunsch.
    return assignments.where((a) => a['status'] == 'active').toList();
  }

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async => [
        {'id': 'e2', 'first_name': 'Zora', 'last_name': 'Zell', 'active': true},
        {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg', 'active': true},
        {'id': 'e9', 'first_name': 'Otto', 'last_name': 'Alt', 'active': false},
      ];

  @override
  Future<Map<String, dynamic>> assignVehicle({required String vehicleId, required String employeeId, required String beginn, String notiz = ''}) async {
    calls.add('assign $vehicleId $employeeId $beginn $notiz');
    if (assignError != null) throw assignError!;
    assignments.insert(0, {'id': 'neu', 'mitarbeiter_name': 'Anna Berg', 'beginn': '${beginn}T00:00:00Z', 'ende': null, 'notiz': notiz, 'status': 'active'});
    return {'id': 'neu', if (hints.isNotEmpty) 'hinweise': hints};
  }

  @override
  Future<Map<String, dynamic>> endFleetAssignment(String id, String ende) async {
    calls.add('end $id $ende');
    assignments.firstWhere((a) => a['id'] == id)['ende'] = '${ende}T00:00:00Z';
    return {'id': id};
  }

  @override
  Future<Map<String, dynamic>> stornoFleetAssignment(String id, String reason) async {
    calls.add('storno $id $reason');
    assignments.firstWhere((a) => a['id'] == id)['status'] = 'storniert';
    return {'id': id};
  }

  @override
  Future<List<dynamic>> listVehicleSchedules(String vehicleId, {bool includeInactive = false}) async => const [];
}

List<Map<String, dynamic>> _history() => [
      {'id': 'x2', 'mitarbeiter_name': 'Anna Berg', 'beginn': '2026-10-01T00:00:00Z', 'ende': null, 'notiz': 'Dienstwagen', 'status': 'active'},
      {'id': 'x1', 'mitarbeiter_name': 'Bert Cord', 'beginn': '2026-09-01T00:00:00Z', 'ende': '2026-09-20T00:00:00Z', 'notiz': '', 'status': 'active'},
    ];

final _now = DateTime(2026, 10, 19, 10);
const _rw = {'fleet.read', 'fleet.write'};

Widget _section(_FakeApi api, {bool deregistered = false, Key? key}) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: Scaffold(
        body: SingleChildScrollView(child: VehicleAssignmentsSection(key: key, api: api, vehicleId: 'v1', deregistered: deregistered, now: () => _now)),
      ),
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

// Button im Dialog - der Abschnitt hat gleichnamige Buttons.
Finder _dlg(String text) => find.descendant(of: find.byType(AlertDialog), matching: find.widgetWithText(FilledButton, text));

void main() {
  test('vehicleAssignmentLine und openVehicleAssignment', () {
    final h = _history();
    expect(vehicleAssignmentLine(h[0]), 'Anna Berg · seit 01.10.2026 (Dienstwagen)');
    expect(vehicleAssignmentLine(h[1]), 'Bert Cord · 01.09.2026 – 20.09.2026');
    expect(openVehicleAssignment(h)?['id'], 'x2');
    expect(openVehicleAssignment([h[1]]), isNull);
    expect(openVehicleAssignment([{...h[0], 'status': 'storniert'}]), isNull);
  });

  testWidgets('zeigt Fahrer und Verlauf; ohne Schreibrecht keine Aktionen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, assignments: _history());
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(api.calls, ['list v1']);
    expect(find.text('Zugeordnet: Anna Berg · seit 01.10.2026 (Dienstwagen)'), findsOneWidget);
    expect(find.text('Bert Cord · 01.09.2026 – 20.09.2026'), findsOneWidget);
    expect(find.text('Beenden'), findsNothing);
    expect(find.text('Fahrer zuordnen'), findsNothing);
    expect(find.byTooltip('Zuordnung stornieren'), findsNothing);
  });

  testWidgets('zuordnen: nur aktive Mitarbeiter, Pruefung im Dialog, Aufruf mit ISO-Datum', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(find.text('Kein fester Fahrer'), findsOneWidget);
    expect(find.text('Noch nie zugeordnet.'), findsOneWidget);
    await tester.tap(find.text('Fahrer zuordnen'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Beginn (TT.MM.JJJJ)')).controller!.text, '19.10.2026');

    await tester.tap(_dlg('Zuordnen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Mitarbeiter auswählen.'), findsOneWidget);
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    expect(find.text('Alt, Otto'), findsNothing);
    await tester.tap(find.text('Berg, Anna').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Beginn (TT.MM.JJJJ)'), '32.10.2026');
    await tester.tap(_dlg('Zuordnen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Datum als TT.MM.JJJJ angeben.'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('assign')), isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Beginn (TT.MM.JJJJ)'), '18.10.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Notiz'), ' Dienstwagen ');
    await tester.tap(_dlg('Zuordnen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('assign')), ['assign v1 e1 2026-10-18 Dienstwagen']);
    expect(find.text('Zugeordnet'), findsOneWidget);
    expect(find.text('Zugeordnet: Anna Berg · seit 18.10.2026 (Dienstwagen)'), findsOneWidget);
    expect(find.text('Fahrer zuordnen'), findsNothing);
    expect(find.text('Beenden'), findsOneWidget);
  });

  testWidgets('Hinweise als Dialog; Ablehnung bei Sperre als Meldung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, hints: ['UVV-Prüfung überfällig seit 30.09.2026', 'Keine gültige Führerscheinkontrolle erfasst']);
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fahrer zuordnen'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Zell, Zora').last);
    await tester.pumpAndSettle();
    await tester.tap(_dlg('Zuordnen'));
    await tester.pumpAndSettle();
    expect(find.text('Zugeordnet – bitte beachten'), findsOneWidget);
    expect(find.text('⚠ UVV-Prüfung überfällig seit 30.09.2026'), findsOneWidget);
    expect(find.text('⚠ Keine gültige Führerscheinkontrolle erfasst'), findsOneWidget);
    await tester.tap(_dlg('OK'));
    await tester.pumpAndSettle();
    expect(find.byType(AlertDialog), findsNothing);

    final blocked = _FakeApi(
        permissions: _rw, assignError: const ApiException(statusCode: 409, message: 'Fahrzeug ist gesperrt: Hauptuntersuchung mit Mängeln'));
    await tester.pumpWidget(_section(blocked, key: const ValueKey('gesperrt')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fahrer zuordnen'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Berg, Anna').last);
    await tester.pumpAndSettle();
    await tester.tap(_dlg('Zuordnen'));
    await tester.pumpAndSettle();
    expect(find.textContaining('Fahrzeug ist gesperrt: Hauptuntersuchung mit Mängeln'), findsOneWidget);
    expect(find.text('Kein fester Fahrer'), findsOneWidget);
  });

  testWidgets('beenden mit Datum; stornieren nur mit Begruendung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, assignments: _history());
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Beenden'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Ende (TT.MM.JJJJ)')).controller!.text, '19.10.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Ende (TT.MM.JJJJ)'), '1.13.2026');
    await tester.tap(_dlg('Beenden'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Datum als TT.MM.JJJJ angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Ende (TT.MM.JJJJ)'), '18.10.2026');
    await tester.tap(_dlg('Beenden'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('end')), ['end x2 2026-10-18']);
    expect(api.calls.where((c) => c.startsWith('list')).length, 2);
    expect(find.text('Kein fester Fahrer'), findsOneWidget);
    expect(find.text('Anna Berg · 01.10.2026 – 18.10.2026 (Dienstwagen)'), findsOneWidget);

    await tester.tap(find.byTooltip('Zuordnung stornieren').last);
    await tester.pumpAndSettle();
    expect(tester.widget<FilledButton>(_dlg('Stornieren')).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), ' falscher Fahrer ');
    await tester.pump();
    await tester.tap(_dlg('Stornieren'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('storno')), ['storno x1 falscher Fahrer']);
    expect(find.text('Bert Cord · 01.09.2026 – 20.09.2026'), findsNothing);
  });

  testWidgets('abgemeldetes Fahrzeug: kein Zuordnen, Stornieren bleibt', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, assignments: [_history()[1]]);
    await tester.pumpWidget(_section(api, deregistered: true));
    await tester.pumpAndSettle();
    expect(find.text('Kein fester Fahrer'), findsOneWidget);
    expect(find.text('Fahrer zuordnen'), findsNothing);
    expect(find.byTooltip('Zuordnung stornieren'), findsOneWidget);
  });

  testWidgets('Detailseite zeigt den Abschnitt Fahrer', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, assignments: _history());
    await tester.pumpWidget(MaterialApp(
        home: VehicleDetailPage(api: api, vehicle: const {'id': 'v1', 'kennzeichen': 'M-AB 1', 'art': 'pkw', 'bezeichnung': '', 'abgemeldet_am': null})));
    await tester.pumpAndSettle();
    expect(find.text('Fahrer'), findsOneWidget);
    expect(api.calls, ['list v1']);
    expect(find.text('Zugeordnet: Anna Berg · seit 01.10.2026 (Dienstwagen)'), findsOneWidget);
  });

  test('API: Pfade, Methoden, Bodies; Fehler werden zur ApiException', () async {
    final calls = <http.Request>[];
    final api = ApiClient(baseUrl: 'http://api.test');
    Future<T> mock<T>(Object body, Future<T> Function() call, {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(utf8.encode(jsonEncode(body)), status, headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    await mock([], () => api.listFleetAssignments(vehicleId: 'v1', employeeId: 'e1', nurOffen: true, includeStorniert: true));
    expect(calls.last.method, 'GET');
    expect(calls.last.url.path, '/api/v1/fleet/assignments/');
    expect(calls.last.url.queryParameters, {'vehicle_id': 'v1', 'employee_id': 'e1', 'nur_offen': 'true', 'include_storniert': 'true'});
    await mock([], () => api.listFleetAssignments());
    expect(calls.last.url.queryParameters, isEmpty);

    final out = await mock({'id': 'a1', 'hinweise': ['x']}, () => api.assignVehicle(vehicleId: 'v1', employeeId: 'e1', beginn: '2026-10-18', notiz: 'n'), status: 201);
    expect(out['hinweise'], ['x']);
    expect(calls.last.method, 'POST');
    expect(calls.last.url.path, '/api/v1/fleet/assignments/');
    expect(jsonDecode(calls.last.body), {'vehicle_id': 'v1', 'employee_id': 'e1', 'beginn': '2026-10-18', 'notiz': 'n'});
    await expectLater(mock({'id': 'a1'}, () => api.assignVehicle(vehicleId: 'v1', employeeId: 'e1', beginn: '2026-10-18')), throwsA(isA<ApiException>()));
    await expectLater(
        mock({'error': {'code': 'conflict', 'message': 'Fahrzeug ist bereits zugeordnet'}}, () => api.assignVehicle(vehicleId: 'v1', employeeId: 'e1', beginn: '2026-10-18'),
            status: 409),
        throwsA(isA<ApiException>()));

    await mock({'id': 'a1'}, () => api.endFleetAssignment('a1', '2026-10-19'));
    expect(calls.last.method, 'POST');
    expect(calls.last.url.path, '/api/v1/fleet/assignments/a1/end');
    expect(jsonDecode(calls.last.body), {'ende': '2026-10-19'});
    await expectLater(mock({'error': {'code': 'conflict', 'message': 'x'}}, () => api.endFleetAssignment('a1', '2026-10-19'), status: 409), throwsA(isA<ApiException>()));

    await mock({'id': 'a1'}, () => api.stornoFleetAssignment('a1', 'falsch'));
    expect(calls.last.url.path, '/api/v1/fleet/assignments/a1/storno');
    expect(jsonDecode(calls.last.body), {'reason': 'falsch'});
    await expectLater(mock({'error': {'code': 'conflict', 'message': 'x'}}, () => api.stornoFleetAssignment('a1', 'falsch'), status: 409), throwsA(isA<ApiException>()));
  });
}
