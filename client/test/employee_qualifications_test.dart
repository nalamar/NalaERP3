import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/employee_qualifications_section.dart';
import 'package:nalaerp_client/pages/employees_page.dart';

// Qualifikationen am Mitarbeiter im Client (Backlog F.7.3.1). InkRipple
// und Desktop-Fenster wie in employees_page_test.dart (Backlog 0.48).

class _FakeQualApi extends ApiClient {
  _FakeQualApi({
    this.permissions = const {'qualifications.read'},
    Map<String, List<dynamic>>? standing,
    this.types = const [],
    List<dynamic>? requirements,
    this.standingError,
    this.employees = const [],
  })  : standing = standing ?? {},
        requirements = requirements ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final Map<String, List<dynamic>> standing;
  final List<dynamic> types;
  List<dynamic> requirements;
  final Object? standingError;
  final List<Map<String, dynamic>> employees;

  final List<String> standingCalls = [];
  final List<MapEntry<String, List<String>>> setCalls = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> getQualificationStanding(String employeeId) async {
    standingCalls.add(employeeId);
    if (standingError != null) throw standingError!;
    return standing[employeeId] ?? const [];
  }

  @override
  Future<List<dynamic>> listQualificationTypes({bool includeInactive = false}) async => types;

  @override
  Future<List<dynamic>> listQualificationRequirements(String employeeId) async => requirements;

  @override
  Future<List<dynamic>> setQualificationRequirements(String employeeId, List<String> typeIds) async {
    setCalls.add(MapEntry(employeeId, typeIds));
    return const [];
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
      home: Scaffold(body: child),
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

Map<String, dynamic> _st(String name, String status, {bool erforderlich = false, String? bis, int? tage}) => {
      'type_id': 't-$name',
      'name': name,
      'status': status,
      'erforderlich': erforderlich,
      if (bis != null) 'gueltig_bis': '${bis}T00:00:00Z',
      if (tage != null) 'tage_bis_ablauf': tage,
    };

void main() {
  group('qualificationStatusLabel', () {
    test('alle Staende', () {
      expect(qualificationStatusLabel(_st('A', 'fehlt')), 'fehlt');
      expect(qualificationStatusLabel(_st('A', 'abgelaufen', bis: '2026-03-01', tage: -10)), 'abgelaufen seit 01.03.2026');
      expect(qualificationStatusLabel(_st('A', 'laeuft_ab', bis: '2026-11-30', tage: 44)), 'läuft ab am 30.11.2026 (in 44 Tagen)');
      expect(qualificationStatusLabel(_st('A', 'laeuft_ab', bis: '2026-10-17', tage: 0)), 'läuft heute ab');
      expect(qualificationStatusLabel(_st('A', 'gueltig', bis: '2027-05-02', tage: 200)), 'gültig bis 02.05.2027');
      expect(qualificationStatusLabel(_st('A', 'gueltig')), 'gültig, unbefristet');
    });
  });

  group('requirementChoices', () {
    test('aktive Arten plus Anforderungen auf stillgelegte, nach Name sortiert', () {
      final choices = requirementChoices([
        {'id': 't2', 'name': 'Staplerschein'},
        {'id': 't1', 'name': 'Arbeitsschutz', 'pflicht_fuer_alle': true},
      ], [
        {'type_id': 't2', 'name': 'Staplerschein', 'type_aktiv': true},
        {'type_id': 't9', 'name': 'Alter Kurs', 'type_aktiv': false},
      ]);
      expect(choices.map((c) => c['type_id']), ['t9', 't1', 't2']);
      expect(choices.map((c) => c['stillgelegt']), [true, false, false]);
      expect(choices.map((c) => c['pflicht_fuer_alle']), [false, true, false]);
    });
  });

  testWidgets('zeigt den Stand mit Kennzeichen "erforderlich"', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(standing: {
      'e1': [
        _st('Erste Hilfe', 'fehlt', erforderlich: true),
        _st('Staplerschein', 'gueltig', bis: '2027-05-02'),
      ],
    });
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();
    expect(api.standingCalls, ['e1']);
    expect(find.text('Erste Hilfe (erforderlich)'), findsOneWidget);
    expect(find.text('fehlt'), findsOneWidget);
    expect(find.text('Staplerschein'), findsOneWidget);
    expect(find.text('gültig bis 02.05.2027'), findsOneWidget);
    // Ohne Schreibrecht kein Festlegen der Anforderungen.
    expect(find.text('Anforderungen festlegen'), findsNothing);
  });

  testWidgets('leerer Stand und Ladefehler', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: _FakeQualApi(), employeeId: 'e1')));
    await tester.pumpAndSettle();
    expect(find.text('Keine Anforderungen und keine Nachweise.'), findsOneWidget);

    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(
        key: const ValueKey('x'),
        api: _FakeQualApi(standingError: const ApiException(statusCode: 403, message: 'Keine Berechtigung')),
        employeeId: 'e1')));
    await tester.pumpAndSettle();
    expect(find.text('Qualifikationen konnten nicht geladen werden: Keine Berechtigung'), findsOneWidget);
    expect(find.text('Keine Anforderungen und keine Nachweise.'), findsNothing);
  });

  testWidgets('Anforderungen festlegen: vorausgewaehlt, stillgelegte bleiben erhalten, danach neu geladen', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      types: [
        {'id': 't1', 'name': 'Arbeitsschutz', 'pflicht_fuer_alle': true},
        {'id': 't2', 'name': 'Staplerschein'},
        {'id': 't3', 'name': 'Erste Hilfe'},
      ],
      requirements: [
        {'type_id': 't2', 'name': 'Staplerschein', 'type_aktiv': true},
        {'type_id': 't9', 'name': 'Alter Kurs', 'type_aktiv': false},
      ],
    );
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Anforderungen festlegen'));
    await tester.pumpAndSettle();
    expect(find.text('stillgelegt – zählt im Stand nicht mehr'), findsOneWidget);
    expect(find.text('ohnehin Pflicht für alle'), findsOneWidget);
    CheckboxListTile tile(String name) => tester.widget<CheckboxListTile>(find.widgetWithText(CheckboxListTile, name));
    expect(tile('Staplerschein').value, isTrue);
    expect(tile('Alter Kurs').value, isTrue);
    expect(tile('Erste Hilfe').value, isFalse);

    await tester.tap(find.text('Erste Hilfe'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Staplerschein'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.setCalls.single.key, 'e1');
    expect(api.setCalls.single.value, ['t3', 't9']);
    expect(api.standingCalls, ['e1', 'e1']);
  });

  testWidgets('Abbrechen speichert nichts', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      types: [
        {'id': 't1', 'name': 'Arbeitsschutz'},
      ],
    );
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anforderungen festlegen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Arbeitsschutz'));
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.setCalls, isEmpty);
  });

  testWidgets('Mitarbeiterseite: Abschnitt nur mit qualifications.read, laedt beim Wechsel neu', (tester) async {
    await _desktop(tester);
    final employees = [
      {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg', 'active': true},
      {'id': 'e2', 'first_name': 'Bert', 'last_name': 'Cord', 'active': true},
    ];
    final api = _FakeQualApi(
      permissions: const {'employees.read', 'qualifications.read'},
      employees: employees,
      standing: {
        'e1': [_st('Erste Hilfe', 'fehlt', erforderlich: true)],
        'e2': [_st('Staplerschein', 'abgelaufen', bis: '2026-01-31')],
      },
    );
    await tester.pumpWidget(MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: EmployeesPage(api: api),
    ));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();
    expect(find.text('Erste Hilfe (erforderlich)'), findsOneWidget);

    await tester.tap(find.text('Bert Cord'));
    await tester.pumpAndSettle();
    expect(find.text('Erste Hilfe (erforderlich)'), findsNothing);
    expect(find.text('abgelaufen seit 31.01.2026'), findsOneWidget);
    expect(api.standingCalls.last, 'e2');

    // Ohne qualifications.read kein Abschnitt.
    final noQual = _FakeQualApi(permissions: const {'employees.read'}, employees: employees);
    await tester.pumpWidget(MaterialApp(
      key: const ValueKey('ohne'),
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: EmployeesPage(api: noQual),
    ));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anna Berg'));
    await tester.pumpAndSettle();
    expect(find.text('Qualifikationen'), findsNothing);
    expect(noQual.standingCalls, isEmpty);
  });

  group('API-Methoden', () {
    final api = ApiClient(baseUrl: 'http://api.test');
    late List<http.Request> calls;

    Future<T> mock<T>(Object body, Future<T> Function() call, {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(utf8.encode(jsonEncode(body)), status, headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    setUp(() => calls = []);

    test('Katalog, Stand und Anforderungen lesen', () async {
      await mock([], () => api.listQualificationTypes());
      await mock([], () => api.listQualificationTypes(includeInactive: true));
      await mock([], () => api.getQualificationStanding('e1'));
      await mock([], () => api.listQualificationRequirements('e1'));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/qualification-types/?',
        'GET /api/v1/qualification-types/?include_inactive=true',
        'GET /api/v1/qualifications/standing?employee_id=e1',
        'GET /api/v1/qualifications/requirements?employee_id=e1',
      ]);
    });

    test('Anforderungen setzen: PUT mit vollstaendiger Liste, auch leer', () async {
      await mock([], () => api.setQualificationRequirements('e1', ['t1', 't2']));
      await mock([], () => api.setQualificationRequirements('e1', []));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}').toSet(), {'PUT /api/v1/qualifications/requirements?employee_id=e1'});
      expect(calls.first.headers['content-type'], startsWith('application/json'));
      expect(jsonDecode(calls[0].body), {'type_ids': ['t1', 't2']});
      // Leer heisst "keine Anforderungen" - das Feld muss trotzdem da sein.
      expect(calls[1].body, '{"type_ids":[]}');
      await expectLater(
        mock({'error': {'code': 'validation_error', 'message': 'Qualifikationsart nicht gefunden'}}, () => api.setQualificationRequirements('e1', ['x']), status: 404),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Qualifikationsart nicht gefunden')),
      );
    });
  });
}
