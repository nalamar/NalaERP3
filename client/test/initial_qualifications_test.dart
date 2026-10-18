import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/employees_page.dart';
import 'package:nalaerp_client/pages/initial_qualifications_dialog.dart';

// Qualifikationen gleich beim Anlegen eines Mitarbeiters (Backlog F.7.3.3).
// InkRipple und Desktop-Fenster wie in den anderen Client-Tests (Backlog
// 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({
    required this.permissions,
    this.types = const [],
    this.requirementsError,
    this.failingTypeIds = const {},
  }) : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<dynamic> types;
  final Object? requirementsError;
  final Set<String> failingTypeIds;

  final List<Map<String, dynamic>> employees = [];
  final List<MapEntry<String, List<String>>> requirementCalls = [];
  final List<Map<String, dynamic>> createdRecords = [];
  final List<String> order = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async => List<dynamic>.from(employees);

  @override
  Future<List<dynamic>> listTeams() async => const [];

  @override
  Future<List<dynamic>> listLoginUsers() async => const [];

  @override
  Future<Map<String, dynamic>> getEmployee(String id) async => employees.firstWhere((e) => e['id'] == id);

  @override
  Future<Map<String, dynamic>> createEmployee(Map<String, dynamic> body) async {
    order.add('createEmployee');
    final e = <String, dynamic>{'id': 'neu-1', 'active': true, ...body};
    employees.add(e);
    return e;
  }

  @override
  Future<List<dynamic>> listQualificationTypes({bool includeInactive = false}) async => types;

  @override
  Future<List<dynamic>> setQualificationRequirements(String employeeId, List<String> typeIds) async {
    order.add('setRequirements');
    if (requirementsError != null) throw requirementsError!;
    requirementCalls.add(MapEntry(employeeId, typeIds));
    return const [];
  }

  @override
  Future<List<dynamic>> createQualification(Map<String, dynamic> body) async {
    order.add('createQualification');
    if (failingTypeIds.contains(body['type_id'])) {
      throw const ApiException(statusCode: 400, message: 'Erwerbsdatum darf nicht in der Zukunft liegen');
    }
    createdRecords.add(body);
    return [body];
  }

  @override
  Future<List<dynamic>> getQualificationStanding(String employeeId) async => const [];

  @override
  Future<List<dynamic>> listEmployeeQualifications(String employeeId, {bool includeStorniert = false}) async => const [];
}

const _types = [
  {'id': 't-eh', 'name': 'Erste Hilfe'},
  {'id': 't-as', 'name': 'Arbeitsschutz'},
  {'id': 't-st', 'name': 'Staplerschein'},
];

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1600, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

Future<void> _openAndCreate(WidgetTester tester, _FakeApi api) async {
  await tester.pumpWidget(MaterialApp(
    theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
    home: EmployeesPage(api: api),
  ));
  await tester.pumpAndSettle();
  await tester.tap(find.byType(FloatingActionButton));
  await tester.pumpAndSettle();
  await tester.enterText(find.widgetWithText(TextField, 'Vorname'), 'Neu');
  await tester.enterText(find.widgetWithText(TextField, 'Nachname'), 'Mann');
  await tester.tap(find.text('Anlegen'));
  await tester.pumpAndSettle();
}

InitialQualificationEntry _e(String id, String name, {bool req = false, bool has = false, String date = ''}) =>
    InitialQualificationEntry(typeId: id, name: name)
      ..required = req
      ..present = has
      ..erworbenAm = date;

void main() {
  group('buildInitialQualificationPlan', () {
    test('Anforderungen sortiert, Nachweise mit Datum', () {
      final plan = buildInitialQualificationPlan('e1', [
        _e('t2', 'Stapler', req: true, has: true, date: '15.03.2026'),
        _e('t1', 'Erste Hilfe', req: true),
        _e('t3', 'Leiter', has: true, date: '2025-01-10'),
        _e('t4', 'Nichts'),
      ]);
      expect(plan.error, isNull);
      expect(plan.requiredIds, ['t1', 't2']);
      expect(plan.records.map((r) => r.key), ['Stapler', 'Leiter']);
      expect(plan.records.first.value, {
        'employee_ids': ['e1'],
        'type_id': 't2',
        'erworben_am': '2026-03-15',
      });
      expect(plan.records.last.value['erworben_am'], '2025-01-10');
    });

    test('"vorhanden" ohne gueltiges Datum ist ein Fehler, nicht still uebergangen', () {
      expect(buildInitialQualificationPlan('e1', [_e('t1', 'Erste Hilfe', has: true)]).error,
          'Erste Hilfe: bitte das Erwerbsdatum als TT.MM.JJJJ angeben.');
      expect(buildInitialQualificationPlan('e1', [_e('t1', 'Erste Hilfe', has: true, date: '30.02.2026')]).error, isNotNull);
      // Ein Datum ohne "vorhanden" zaehlt nicht.
      final p = buildInitialQualificationPlan('e1', [_e('t1', 'Erste Hilfe', date: 'Unsinn')]);
      expect(p.error, isNull);
      expect(p.records, isEmpty);
    });
  });

  group('applyInitialQualifications', () {
    test('ohne Anforderungen kein Aufruf dafuer; ein Fehler haelt die uebrigen nicht auf', () async {
      final api = _FakeApi(permissions: const {}, failingTypeIds: {'t1'});
      final lines = await applyInitialQualifications(
          api,
          'e1',
          buildInitialQualificationPlan('e1', [
            _e('t1', 'Erste Hilfe', has: true, date: '01.01.2026'),
            _e('t2', 'Stapler', has: true, date: '02.01.2026'),
          ]));
      expect(api.order, ['createQualification', 'createQualification']);
      expect(lines.map((l) => l.ok), [false, true]);
      expect(lines.map((l) => l.line), [
        'Nachweis "Erste Hilfe" NICHT erfasst: Erwerbsdatum darf nicht in der Zukunft liegen',
        'Nachweis "Stapler" erfasst.',
      ]);
    });

    test('Fehler bei den Anforderungen wird gemeldet, Nachweise werden trotzdem erfasst', () async {
      final api = _FakeApi(permissions: const {}, requirementsError: const ApiException(statusCode: 404, message: 'Qualifikationsart nicht gefunden'));
      final lines = await applyInitialQualifications(
          api,
          'e1',
          buildInitialQualificationPlan('e1', [
            _e('t1', 'Erste Hilfe', req: true, has: true, date: '01.01.2026'),
          ]));
      expect(api.order, ['setRequirements', 'createQualification']);
      expect(lines.map((l) => l.ok), [false, true]);
      expect(lines.map((l) => l.line), ['Anforderungen NICHT gespeichert: Qualifikationsart nicht gefunden', 'Nachweis "Erste Hilfe" erfasst.']);
    });
  });

  testWidgets('Anlegen mit Anforderungen und vorhandenem Nachweis', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'employees.read', 'employees.write', 'qualifications.read', 'qualifications.write'}, types: _types);
    await _openAndCreate(tester, api);

    expect(find.text('Qualifikationen für Neu Mann'), findsOneWidget);
    // Nach Name sortiert, unabhaengig von der Reihenfolge des Katalogs.
    double y(String name) => tester.getTopLeft(find.text(name)).dy;
    expect(y('Arbeitsschutz'), lessThan(y('Erste Hilfe')));
    expect(y('Erste Hilfe'), lessThan(y('Staplerschein')));
    await tester.tap(find.byKey(const ValueKey('iq-req-t-eh')));
    await tester.tap(find.byKey(const ValueKey('iq-req-t-st')));
    await tester.tap(find.byKey(const ValueKey('iq-has-t-st')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('iq-date-t-st')), '15.03.2026');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.order, ['createEmployee', 'setRequirements', 'createQualification']);
    expect(api.requirementCalls.single.key, 'neu-1');
    expect(api.requirementCalls.single.value, ['t-eh', 't-st']);
    expect(api.createdRecords.single, {
      'employee_ids': ['neu-1'],
      'type_id': 't-st',
      'erworben_am': '2026-03-15',
    });
    expect(find.text('Anforderungen gespeichert (2). Nachweis "Staplerschein" erfasst.'), findsOneWidget);
    // Danach ist der neue Mitarbeiter ausgewaehlt.
    expect(find.text('Status: aktiv'), findsOneWidget);
  });

  testWidgets('Datumsfeld nur bei "vorhanden"; fehlendes Datum bleibt im Dialog', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'employees.read', 'employees.write', 'qualifications.write'}, types: _types);
    await _openAndCreate(tester, api);

    expect(tester.widget<TextField>(find.byKey(const ValueKey('iq-date-t-eh'))).enabled, isFalse);
    await tester.tap(find.byKey(const ValueKey('iq-has-t-eh')));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.byKey(const ValueKey('iq-date-t-eh'))).enabled, isTrue);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Erste Hilfe: bitte das Erwerbsdatum als TT.MM.JJJJ angeben.'), findsOneWidget);
    expect(api.order, ['createEmployee']);
  });

  testWidgets('Ueberspringen speichert nur den Mitarbeiter', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'employees.read', 'employees.write', 'qualifications.write'}, types: _types);
    await _openAndCreate(tester, api);
    await tester.tap(find.byKey(const ValueKey('iq-req-t-eh')));
    await tester.tap(find.text('Überspringen'));
    await tester.pumpAndSettle();
    expect(api.order, ['createEmployee']);
    expect(find.text('Status: aktiv'), findsOneWidget);
  });

  testWidgets('ohne qualifications.write oder ohne Katalog kein zweiter Schritt', (tester) async {
    await _desktop(tester);
    final noWrite = _FakeApi(permissions: const {'employees.read', 'employees.write', 'qualifications.read'}, types: _types);
    await _openAndCreate(tester, noWrite);
    expect(find.textContaining('Qualifikationen für'), findsNothing);
    expect(noWrite.order, ['createEmployee']);

    final emptyCatalog = _FakeApi(permissions: const {'employees.read', 'employees.write', 'qualifications.write'});
    await tester.pumpWidget(const SizedBox());
    await _openAndCreate(tester, emptyCatalog);
    expect(find.textContaining('Qualifikationen für'), findsNothing);
    expect(find.text('Status: aktiv'), findsOneWidget);
  });

  testWidgets('Teilerfolg wird in einem Dialog genau gemeldet', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
      permissions: const {'employees.read', 'employees.write', 'qualifications.write'},
      types: _types,
      failingTypeIds: {'t-eh'},
    );
    await _openAndCreate(tester, api);
    await tester.tap(find.byKey(const ValueKey('iq-has-t-eh')));
    await tester.tap(find.byKey(const ValueKey('iq-has-t-as')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('iq-date-t-eh')), '01.01.2099');
    await tester.enterText(find.byKey(const ValueKey('iq-date-t-as')), '10.01.2026');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(find.text('Nicht alles gespeichert'), findsOneWidget);
    expect(find.text('Der Mitarbeiter ist angelegt. Fehlendes bitte im Abschnitt "Qualifikationen" nachtragen.'), findsOneWidget);
    // Sortiert nach Name: Arbeitsschutz vor Erste Hilfe.
    expect(find.text('Nachweis "Arbeitsschutz" erfasst.'), findsOneWidget);
    expect(find.text('Nachweis "Erste Hilfe" NICHT erfasst: Erwerbsdatum darf nicht in der Zukunft liegen'), findsOneWidget);
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.text('Status: aktiv'), findsOneWidget);
  });
}
