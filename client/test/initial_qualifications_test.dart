import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/employees_page.dart';
import 'package:nalaerp_client/pages/initial_qualifications_dialog.dart';

// Qualifikationen gleich beim Anlegen eines Mitarbeiters (Backlog F.7.3.3,
// seit F.10.5.2 ein Aufruf). InkRipple und Desktop-Fenster wie in den
// anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({
    required this.permissions,
    this.types = const [],
    this.typesError,
    this.combinedError,
  }) : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<dynamic> types;
  final Object? typesError;
  // Wirft der kombinierte Aufruf einmal, gelingt der naechste.
  Object? combinedError;

  final List<Map<String, dynamic>> employees = [];
  final List<Map<String, dynamic>> combinedCalls = [];
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
  Future<Map<String, dynamic>> createEmployeeWithQualifications(Map<String, dynamic> body) async {
    order.add('createEmployeeWithQualifications');
    combinedCalls.add(body);
    final err = combinedError;
    if (err != null) {
      combinedError = null;
      throw err;
    }
    final e = <String, dynamic>{'id': 'neu-1', 'active': true, ...(body['employee'] as Map<String, dynamic>)};
    employees.add(e);
    return {
      'employee': e,
      'requirements': [
        for (final id in body['required_type_ids'] as List) {'type_id': id}
      ],
      'qualifications': [
        for (final q in body['qualifications'] as List) {'id': 'q', ...(q as Map<String, dynamic>)}
      ],
    };
  }

  @override
  Future<List<dynamic>> listQualificationTypes({bool includeInactive = false}) async {
    if (typesError != null) throw typesError!;
    return types;
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

const _allWrite = {'employees.read', 'employees.write', 'qualifications.read', 'qualifications.write'};

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1600, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

// _openDialog oeffnet "Mitarbeiter anlegen" und traegt den Namen ein.
Future<void> _openDialog(WidgetTester tester, _FakeApi api) async {
  await tester.pumpWidget(MaterialApp(
    theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
    home: EmployeesPage(api: api),
  ));
  await tester.pumpAndSettle();
  await tester.tap(find.byType(FloatingActionButton));
  await tester.pumpAndSettle();
  await tester.enterText(find.widgetWithText(TextField, 'Vorname'), 'Neu');
  await tester.enterText(find.widgetWithText(TextField, 'Nachname'), 'Mann');
}

Future<void> _create(WidgetTester tester) async {
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
      final plan = buildInitialQualificationPlan([
        _e('t2', 'Stapler', req: true, has: true, date: '15.03.2026'),
        _e('t1', 'Erste Hilfe', req: true),
        _e('t3', 'Leiter', has: true, date: '2025-01-10'),
        _e('t4', 'Nichts'),
      ]);
      expect(plan.error, isNull);
      expect(plan.requiredIds, ['t1', 't2']);
      expect(plan.records, [
        {'type_id': 't2', 'erworben_am': '2026-03-15'},
        {'type_id': 't3', 'erworben_am': '2025-01-10'},
      ]);
    });

    test('"vorhanden" ohne gueltiges Datum ist ein Fehler, nicht still uebergangen', () {
      expect(buildInitialQualificationPlan([_e('t1', 'Erste Hilfe', has: true)]).error,
          'Erste Hilfe: bitte das Erwerbsdatum als TT.MM.JJJJ angeben.');
      expect(buildInitialQualificationPlan([_e('t1', 'Erste Hilfe', has: true, date: '30.02.2026')]).error, isNotNull);
      // Ein Datum ohne "vorhanden" zaehlt nicht.
      final p = buildInitialQualificationPlan([_e('t1', 'Erste Hilfe', date: 'Unsinn')]);
      expect(p.error, isNull);
      expect(p.records, isEmpty);
    });
  });

  test('initialQualificationEntries sortiert nach Name, ohne Gross-/Kleinschreibung', () {
    final entries = initialQualificationEntries([
      {'id': 'b', 'name': 'Stapler'},
      {'id': 'a', 'name': 'arbeitsschutz'},
      {'id': 'c', 'name': 'Erste Hilfe'},
    ]);
    // Mit Beachtung der Schreibung stuende "arbeitsschutz" am Ende.
    expect(entries.map((e) => e.typeId), ['a', 'c', 'b']);
  });

  test('buildEmployeeWithQualificationsBody und initialQualificationSummary', () {
    final body = buildEmployeeWithQualificationsBody(
        {'first_name': 'Neu', 'last_name': 'Mann'},
        buildInitialQualificationPlan([_e('t1', 'Erste Hilfe', req: true, has: true, date: '01.02.2026')]));
    expect(body, {
      'employee': {'first_name': 'Neu', 'last_name': 'Mann'},
      'required_type_ids': ['t1'],
      'qualifications': [
        {'type_id': 't1', 'erworben_am': '2026-02-01'}
      ],
    });
    // Leere Listen werden mitgeschickt, nicht weggelassen.
    expect(buildEmployeeWithQualificationsBody({}, buildInitialQualificationPlan([])), {
      'employee': {},
      'required_type_ids': [],
      'qualifications': [],
    });

    expect(initialQualificationSummary({'requirements': [], 'qualifications': []}), isNull);
    expect(initialQualificationSummary({'requirements': [1, 2], 'qualifications': [1]}), 'Mitarbeiter angelegt (Anforderungen: 2, 1 Nachweis).');
    expect(initialQualificationSummary({'requirements': [], 'qualifications': [1, 2]}), 'Mitarbeiter angelegt (2 Nachweise).');
    expect(initialQualificationSummary({'requirements': [1]}), 'Mitarbeiter angelegt (Anforderungen: 1).');
  });

  testWidgets('Anlegen mit Anforderungen und vorhandenem Nachweis in einem Aufruf', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _allWrite, types: _types);
    await _openDialog(tester, api);

    expect(find.text('Welche Qualifikationen braucht die Stelle, und welche sind schon vorhanden?'), findsOneWidget);
    // Nach Name sortiert, unabhaengig von der Reihenfolge des Katalogs.
    double y(String name) => tester.getTopLeft(find.text(name)).dy;
    expect(y('Arbeitsschutz'), lessThan(y('Erste Hilfe')));
    expect(y('Erste Hilfe'), lessThan(y('Staplerschein')));
    // Jedes Kaestchen zeigt den neuen Stand sofort.
    await tester.tap(find.byKey(const ValueKey('iq-req-t-eh')));
    await tester.pumpAndSettle();
    expect(tester.widget<Checkbox>(find.byKey(const ValueKey('iq-req-t-eh'))).value, isTrue);
    await tester.tap(find.byKey(const ValueKey('iq-req-t-st')));
    await tester.tap(find.byKey(const ValueKey('iq-has-t-st')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('iq-date-t-st')), '15.03.2026');
    await _create(tester);

    expect(api.order, ['createEmployeeWithQualifications']);
    expect(api.combinedCalls.single, {
      'employee': {'first_name': 'Neu', 'last_name': 'Mann'},
      'required_type_ids': ['t-eh', 't-st'],
      'qualifications': [
        {'type_id': 't-st', 'erworben_am': '2026-03-15'}
      ],
    });
    expect(find.byType(AlertDialog), findsNothing);
    expect(find.text('Mitarbeiter angelegt (Anforderungen: 2, 1 Nachweis).'), findsOneWidget);
    // Danach ist der neue Mitarbeiter ausgewaehlt (Kopf der Detailansicht).
    expect(find.text('Status: aktiv'), findsOneWidget);
    expect(find.byWidgetPredicate((w) => w is Text && w.data == 'Neu Mann' && w.style?.fontSize == 20), findsOneWidget);
  });

  testWidgets('ohne Auswahl ebenfalls ein Aufruf, keine Meldung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _allWrite, types: _types);
    await _openDialog(tester, api);
    await _create(tester);
    expect(api.order, ['createEmployeeWithQualifications']);
    expect(api.combinedCalls.single['required_type_ids'], isEmpty);
    expect(api.combinedCalls.single['qualifications'], isEmpty);
    expect(find.textContaining('Mitarbeiter angelegt'), findsNothing);
    expect(find.text('Status: aktiv'), findsOneWidget);
  });

  testWidgets('Datumsfeld nur bei "vorhanden"; fehlendes Datum bleibt im Dialog, nichts gesendet', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
      permissions: _allWrite,
      types: _types,
      combinedError: const ApiException(statusCode: 400, message: 'Personalnummer existiert bereits'),
    );
    await _openDialog(tester, api);

    expect(tester.widget<TextField>(find.byKey(const ValueKey('iq-date-t-eh'))).enabled, isFalse);
    await tester.tap(find.byKey(const ValueKey('iq-has-t-eh')));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.byKey(const ValueKey('iq-date-t-eh'))).enabled, isTrue);
    await _create(tester);
    expect(find.text('Erste Hilfe: bitte das Erwerbsdatum als TT.MM.JJJJ angeben.'), findsOneWidget);
    expect(api.order, isEmpty);
    expect(find.byType(AlertDialog), findsOneWidget);

    // Nach dem Nachtragen wird gesendet und der Hinweis verschwindet -
    // auch wenn der Server dann ablehnt und der Dialog offen bleibt.
    await tester.enterText(find.byKey(const ValueKey('iq-date-t-eh')), '01.02.2026');
    await _create(tester);
    expect(api.order, ['createEmployeeWithQualifications']);
    expect(find.byType(AlertDialog), findsOneWidget);
    expect(find.text('Erste Hilfe: bitte das Erwerbsdatum als TT.MM.JJJJ angeben.'), findsNothing);
  });

  testWidgets('Fehler des Servers: nichts gespeichert, Dialog und Eingaben bleiben', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
      permissions: _allWrite,
      types: _types,
      combinedError: const ApiException(statusCode: 400, message: 'Erwerbsdatum darf nicht in der Zukunft liegen'),
    );
    await _openDialog(tester, api);
    await tester.tap(find.byKey(const ValueKey('iq-req-t-as')));
    await tester.tap(find.byKey(const ValueKey('iq-has-t-eh')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('iq-date-t-eh')), '01.01.2099');
    await _create(tester);

    expect(find.text('Fehler: Erwerbsdatum darf nicht in der Zukunft liegen'), findsOneWidget);
    expect(find.byType(AlertDialog), findsOneWidget);
    expect(api.employees, isEmpty);
    // Kein Teilerfolg-Dialog mehr.
    expect(find.text('Nicht alles gespeichert'), findsNothing);
    expect(tester.widget<Checkbox>(find.byKey(const ValueKey('iq-req-t-as'))).value, isTrue);

    // Datum korrigiert, zweiter Versuch mit denselben uebrigen Angaben.
    await tester.enterText(find.byKey(const ValueKey('iq-date-t-eh')), '01.01.2026');
    await _create(tester);
    expect(api.combinedCalls.length, 2);
    expect(api.combinedCalls.last['required_type_ids'], ['t-as']);
    expect(api.combinedCalls.last['qualifications'], [
      {'type_id': 't-eh', 'erworben_am': '2026-01-01'}
    ]);
    expect(find.byType(AlertDialog), findsNothing);
    expect(find.text('Status: aktiv'), findsOneWidget);
  });

  testWidgets('ohne qualifications.write oder ohne Katalog: einfache Anlage ohne Auswahl', (tester) async {
    await _desktop(tester);
    final noWrite = _FakeApi(permissions: const {'employees.read', 'employees.write', 'qualifications.read'}, types: _types);
    await _openDialog(tester, noWrite);
    expect(find.text('Qualifikationen'), findsNothing);
    await _create(tester);
    expect(noWrite.order, ['createEmployee']);

    final emptyCatalog = _FakeApi(permissions: _allWrite);
    await tester.pumpWidget(const SizedBox());
    await _openDialog(tester, emptyCatalog);
    expect(find.byKey(const ValueKey('iq-req-t-eh')), findsNothing);
    await _create(tester);
    expect(emptyCatalog.order, ['createEmployee']);
    expect(find.text('Status: aktiv'), findsOneWidget);
  });

  testWidgets('Katalog nicht ladbar: Hinweis, Anlage ohne Qualifikationen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _allWrite, typesError: const ApiException(statusCode: 500, message: 'kaputt'));
    await _openDialog(tester, api);
    expect(
        find.text('Qualifikationen konnten nicht geladen werden (kaputt). Der Mitarbeiter wird ohne sie angelegt; '
            'nachtragen im Abschnitt "Qualifikationen".'),
        findsOneWidget);
    await _create(tester);
    expect(api.order, ['createEmployee']);
  });

  group('API-Methode', () {
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

    test('POST /employees/with-qualifications, erwartet 201', () async {
      final body = {
        'employee': {'first_name': 'A', 'last_name': 'B'},
        'required_type_ids': ['t1'],
        'qualifications': [],
      };
      final out = await mock({'employee': {'id': 'e1'}, 'requirements': [], 'qualifications': []},
          () => api.createEmployeeWithQualifications(body),
          status: 201);
      expect(out['employee'], {'id': 'e1'});
      expect('${calls.single.method} ${calls.single.url.path}', 'POST /api/v1/employees/with-qualifications');
      expect(calls.single.headers['content-type'], startsWith('application/json'));
      expect(jsonDecode(calls.single.body), body);

      await expectLater(mock({'employee': {}}, () => api.createEmployeeWithQualifications(body), status: 200), throwsA(isA<ApiException>()));
      await expectLater(
        mock({'error': {'code': 'forbidden', 'message': 'Keine Berechtigung'}}, () => api.createEmployeeWithQualifications(body), status: 403),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Keine Berechtigung')),
      );
    });
  });
}
