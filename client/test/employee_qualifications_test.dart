import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/employee_qualifications_section.dart';
import 'package:nalaerp_client/pages/employees_page.dart';
import 'package:nalaerp_client/web/browser.dart' as browser;

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
    Map<String, List<dynamic>>? records,
    this.allTypes,
    this.createError,
    Map<String, List<dynamic>>? documents,
    this.uploadError,
  })  : standing = standing ?? {},
        records = records ?? {},
        documents = documents ?? {},
        requirements = requirements ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final Map<String, List<dynamic>> standing;
  final List<dynamic> types;
  List<dynamic> requirements;
  final Object? standingError;
  final List<Map<String, dynamic>> employees;
  final Map<String, List<dynamic>> records;
  // Katalog inklusive stillgelegter (nur fuer Namen); null = wie types.
  final List<dynamic>? allTypes;
  final Object? createError;
  final List<Map<String, dynamic>> created = [];
  final List<MapEntry<String, String>> stornos = [];
  // Nachweisdokumente je Nachweis (F.10.2).
  final Map<String, List<dynamic>> documents;
  final Object? uploadError;
  final List<String> documentListCalls = [];
  final List<Map<String, Object?>> uploads = [];
  final List<List<String?>> downloads = [];

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
  Future<List<dynamic>> listQualificationTypes({bool includeInactive = false}) async =>
      includeInactive ? (allTypes ?? types) : types;

  @override
  Future<List<dynamic>> listEmployeeQualifications(String employeeId, {bool includeStorniert = false}) async =>
      records[employeeId] ?? const [];

  @override
  Future<List<dynamic>> createQualification(Map<String, dynamic> body) async {
    if (createError != null) throw createError!;
    created.add(body);
    return [
      {'id': 'neu', ...body}
    ];
  }

  @override
  Future<Map<String, dynamic>> stornoQualification(String id, String reason) async {
    stornos.add(MapEntry(id, reason));
    return {'id': id, 'status': 'storniert'};
  }

  @override
  Future<List<dynamic>> listQualificationRequirements(String employeeId) async => requirements;

  @override
  Future<List<dynamic>> listQualificationDocuments(String qualificationId) async {
    documentListCalls.add(qualificationId);
    return List<dynamic>.from(documents[qualificationId] ?? const []);
  }

  @override
  Future<Map<String, dynamic>> uploadQualificationDocument(String qualificationId, String filename, Uint8List bytes,
      {String? contentType}) async {
    if (uploadError != null) throw uploadError!;
    uploads.add({'qual': qualificationId, 'filename': filename, 'length': bytes.length, 'contentType': contentType});
    final doc = <String, dynamic>{
      'id': 'd-neu',
      'qualification_id': qualificationId,
      'filename': filename,
      'content_type': contentType,
      'length': bytes.length,
      'uploaded_at': '2026-10-22T09:00:00Z',
    };
    documents.putIfAbsent(qualificationId, () => []).add(doc);
    return doc;
  }

  @override
  Future<void> downloadQualificationDocument(String qualificationId, String documentId, {String? filename}) async {
    downloads.add([qualificationId, documentId, filename]);
  }

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

  group('parseDateInput', () {
    test('deutsches und ISO-Format, ungueltige Tage abgelehnt', () {
      expect(parseDateInput('15.03.2026'), '2026-03-15');
      expect(parseDateInput(' 5.3.2026 '), '2026-03-05');
      expect(parseDateInput('2026-03-15'), '2026-03-15');
      expect(parseDateInput('29.02.2028'), '2028-02-29');
      expect(parseDateInput('29.02.2026'), isNull);
      expect(parseDateInput('31.04.2026'), isNull);
      expect(parseDateInput('13.13.2026'), isNull);
      expect(parseDateInput('2026-3-15'), isNull);
      expect(parseDateInput('15.03.26'), isNull);
      expect(parseDateInput(''), isNull);
    });
  });

  group('buildQualificationBody', () {
    test('Pflichtangaben und nur gefuellte optionale', () {
      expect(buildQualificationBody(employeeId: 'e1', typeId: 't1', erworbenAm: '2026-03-15', aussteller: ' ', note: ' x '), {
        'employee_ids': ['e1'],
        'type_id': 't1',
        'erworben_am': '2026-03-15',
        'note': 'x',
      });
      expect(
        buildQualificationBody(
            employeeId: 'e1', typeId: 't1', erworbenAm: '2026-03-15', gueltigBis: '2027-03-15', aussteller: ' DRK ', nachweisNr: ' N-1 '),
        {
          'employee_ids': ['e1'],
          'type_id': 't1',
          'erworben_am': '2026-03-15',
          'gueltig_bis': '2027-03-15',
          'aussteller': 'DRK',
          'nachweis_nr': 'N-1',
        },
      );
    });

    test('qualificationRecordLabel', () {
      expect(qualificationRecordLabel({'erworben_am': '2026-03-15T00:00:00Z', 'gueltig_bis': '2028-03-15T00:00:00Z', 'aussteller': 'DRK'}),
          'erworben 15.03.2026 · gültig bis 15.03.2028 · DRK');
      expect(qualificationRecordLabel({'erworben_am': '2026-03-15T00:00:00Z', 'aussteller': ''}), 'erworben 15.03.2026 · unbefristet');
    });
  });

  testWidgets('listet Nachweise mit Namen aus dem Katalog, auch stillgelegter Arten', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      standing: {'e1': [_st('Erste Hilfe', 'gueltig', bis: '2028-03-15')]},
      types: [
        {'id': 't1', 'name': 'Erste Hilfe'},
      ],
      allTypes: [
        {'id': 't1', 'name': 'Erste Hilfe'},
        {'id': 't9', 'name': 'Alter Kurs', 'aktiv': false},
      ],
      records: {
        'e1': [
          {'id': 'q1', 'type_id': 't1', 'erworben_am': '2026-03-15T00:00:00Z', 'gueltig_bis': '2028-03-15T00:00:00Z', 'aussteller': 'DRK'},
          {'id': 'q2', 'type_id': 't9', 'erworben_am': '2020-01-10T00:00:00Z'},
        ],
      },
    );
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();
    expect(find.text('Nachweise'), findsOneWidget);
    expect(find.text('Erste Hilfe: erworben 15.03.2026 · gültig bis 15.03.2028 · DRK'), findsOneWidget);
    expect(find.text('Alter Kurs: erworben 10.01.2020 · unbefristet'), findsOneWidget);
    // Ohne Schreibrecht weder Erfassen noch Storno.
    expect(find.text('Nachweis erfassen'), findsNothing);
    expect(find.byTooltip('Stornieren'), findsNothing);
  });

  testWidgets('erfasst einen Nachweis; ungueltige Eingaben bleiben im Dialog', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      types: [
        {'id': 't1', 'name': 'Erste Hilfe'},
        {'id': 't2', 'name': 'Staplerschein'},
      ],
    );
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();
    final loadsBefore = api.standingCalls.length;

    await tester.tap(find.text('Nachweis erfassen'));
    await tester.pumpAndSettle();
    // Ohne Auswahl.
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte eine Qualifikation auswählen.'), findsOneWidget);

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Staplerschein').last);
    await tester.pumpAndSettle();
    // Ungueltiges Erwerbsdatum.
    await tester.enterText(find.widgetWithText(TextField, 'Erworben am (TT.MM.JJJJ)'), '31.02.2026');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Erwerbsdatum als TT.MM.JJJJ angeben.'), findsOneWidget);
    // Ungueltiges Gueltig-bis.
    await tester.enterText(find.widgetWithText(TextField, 'Erworben am (TT.MM.JJJJ)'), '15.03.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Gültig bis (TT.MM.JJJJ, leer = nach Frist der Art)'), 'bald');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte "Gültig bis" als TT.MM.JJJJ angeben oder leer lassen.'), findsOneWidget);
    expect(api.created, isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Gültig bis (TT.MM.JJJJ, leer = nach Frist der Art)'), '');
    await tester.enterText(find.widgetWithText(TextField, 'Aussteller'), 'BG Holz und Metall');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(api.created, [
      {
        'employee_ids': ['e1'],
        'type_id': 't2',
        'erworben_am': '2026-03-15',
        'aussteller': 'BG Holz und Metall',
      }
    ]);
    expect(find.byType(AlertDialog), findsNothing);
    expect(api.standingCalls.length, greaterThan(loadsBefore));
    expect(find.text('Nachweis erfasst'), findsOneWidget);
  });

  testWidgets('Fehler des Servers beim Erfassen wird angezeigt', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      types: [
        {'id': 't1', 'name': 'Erste Hilfe'},
      ],
      createError: const ApiException(statusCode: 400, message: 'Erwerbsdatum darf nicht in der Zukunft liegen'),
    );
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Nachweis erfassen'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Erste Hilfe').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Erworben am (TT.MM.JJJJ)'), '01.01.2099');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Erwerbsdatum darf nicht in der Zukunft liegen'), findsOneWidget);
  });

  testWidgets('storniert einen Nachweis nur mit Begruendung', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      standing: {'e1': [_st('Erste Hilfe', 'gueltig')]},
      types: [
        {'id': 't1', 'name': 'Erste Hilfe'},
      ],
      records: {
        'e1': [
          {'id': 'q1', 'type_id': 't1', 'erworben_am': '2026-03-15T00:00:00Z'},
          {'id': 'q2', 'type_id': 't1', 'erworben_am': '2024-03-15T00:00:00Z'},
        ],
      },
    );
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();
    final loadsBefore = api.standingCalls.length;

    // Den zweiten (aelteren) Nachweis stornieren.
    await tester.tap(find.byTooltip('Stornieren').at(1));
    await tester.pumpAndSettle();
    final button = find.widgetWithText(FilledButton, 'Stornieren');
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), '  ');
    await tester.pumpAndSettle();
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), ' falsches Datum ');
    await tester.pumpAndSettle();
    await tester.tap(button);
    await tester.pumpAndSettle();

    expect(api.stornos.single.key, 'q2');
    expect(api.stornos.single.value, 'falsches Datum');
    expect(api.standingCalls.length, greaterThan(loadsBefore));
  });

  testWidgets('Abbrechen beim Storno storniert nichts', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      standing: {'e1': [_st('Erste Hilfe', 'gueltig')]},
      records: {
        'e1': [
          {'id': 'q1', 'type_id': 't1', 'erworben_am': '2026-03-15T00:00:00Z'},
        ],
      },
    );
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Stornieren'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), 'x');
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.stornos, isEmpty);
  });

  group('API-Methoden der Nachweise', () {
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

    test('Nachweise lesen, erfassen, stornieren', () async {
      await mock([], () => api.listEmployeeQualifications('e1'));
      await mock([], () => api.listEmployeeQualifications('e1', includeStorniert: true));
      final created = await mock([
        {'id': 'q1'}
      ], () => api.createQualification({'employee_ids': ['e1'], 'type_id': 't1', 'erworben_am': '2026-03-15'}), status: 201);
      final storno = await mock({'id': 'q1', 'status': 'storniert'}, () => api.stornoQualification('q1', 'falsch'));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/qualifications/?employee_id=e1',
        'GET /api/v1/qualifications/?employee_id=e1&include_storniert=true',
        'POST /api/v1/qualifications/?',
        'POST /api/v1/qualifications/q1/storno?',
      ]);
      expect(jsonDecode(calls[2].body), {'employee_ids': ['e1'], 'type_id': 't1', 'erworben_am': '2026-03-15'});
      expect(jsonDecode(calls[3].body), {'reason': 'falsch'});
      expect(calls[2].headers['content-type'], startsWith('application/json'));
      expect(calls[3].headers['content-type'], startsWith('application/json'));
      expect(created.single['id'], 'q1');
      expect(storno['status'], 'storniert');
    });

    test('Erfassen erwartet 201, Fehler werden zur ApiException', () async {
      await expectLater(mock([], () => api.createQualification({}), status: 200), throwsA(isA<ApiException>()));
      await expectLater(
        mock({'error': {'code': 'conflict', 'message': 'Nachweis ist bereits storniert'}}, () => api.stornoQualification('q1', 'x'), status: 409),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Nachweis ist bereits storniert')),
      );
    });
  });

  group('Nachweisdokumente: Hilfsfunktionen', () {
    test('formatFileSize', () {
      expect(formatFileSize(0), '0 B');
      expect(formatFileSize(1023), '1023 B');
      expect(formatFileSize(1024), '1 KB');
      expect(formatFileSize(250 * 1024 + 300), '250 KB');
      expect(formatFileSize(1024 * 1024), '1,0 MB');
      expect(formatFileSize(1536 * 1024), '1,5 MB');
    });

    test('qualificationUploadProblem: leer und ueber 10 MB abgelehnt, genau 10 MB erlaubt', () {
      expect(qualificationUploadProblem(0), 'Die Datei ist leer.');
      expect(qualificationUploadProblem(1), isNull);
      expect(qualificationUploadProblem(10 * 1024 * 1024), isNull);
      expect(qualificationUploadProblem(10 * 1024 * 1024 + 1), 'Die Datei ist zu groß (10,0 MB, höchstens 10 MB).');
      expect(qualificationUploadProblem(12 * 1024 * 1024), 'Die Datei ist zu groß (12,0 MB, höchstens 10 MB).');
    });

    test('qualificationDocumentLabel', () {
      expect(qualificationDocumentLabel({'filename': 'scan.pdf', 'length': 2048, 'uploaded_at': '2026-10-21T15:00:00Z'}),
          'scan.pdf · 2 KB · hochgeladen 21.10.2026');
      expect(qualificationDocumentLabel({'filename': 'liste.png'}), 'liste.png');
    });
  });

  Map<String, List<dynamic>> recordsWithOne() => {
        'e1': [
          {'id': 'q1', 'type_id': 't1', 'erworben_am': '2026-03-15T00:00:00Z'},
          {'id': 'q2', 'type_id': 't1', 'erworben_am': '2024-03-15T00:00:00Z'},
        ],
      };

  testWidgets('Dokumente ansehen und herunterladen, ohne Schreibrecht kein Hochladen', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      types: [
        {'id': 't1', 'name': 'Erste Hilfe'},
      ],
      records: recordsWithOne(),
      documents: {
        'q2': [
          {'id': 'd1', 'filename': 'zertifikat.pdf', 'length': 2048, 'uploaded_at': '2026-10-21T15:00:00Z'},
          {'id': 'd2', 'filename': 'liste.png', 'length': 1536 * 1024, 'uploaded_at': '2026-10-20T08:00:00Z'},
        ],
      },
    );
    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1')));
    await tester.pumpAndSettle();

    // Zum ersten Nachweis gibt es keine Dokumente.
    await tester.tap(find.byTooltip('Dokumente').first);
    await tester.pumpAndSettle();
    expect(api.documentListCalls, ['q1']);
    expect(find.text('Keine Dokumente.'), findsOneWidget);
    expect(find.text('Datei hochladen'), findsNothing);
    await tester.tap(find.text('Schließen'));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Dokumente').at(1));
    await tester.pumpAndSettle();
    expect(api.documentListCalls, ['q1', 'q2']);
    expect(find.text('Erste Hilfe – erworben 15.03.2024 · unbefristet'), findsOneWidget);
    expect(find.text('zertifikat.pdf · 2 KB · hochgeladen 21.10.2026'), findsOneWidget);
    expect(find.text('liste.png · 1,5 MB · hochgeladen 20.10.2026'), findsOneWidget);
    expect(find.text('Keine Dokumente.'), findsNothing);
    expect(find.text('Datei hochladen'), findsNothing);

    await tester.tap(find.byTooltip('Herunterladen').at(1));
    await tester.pumpAndSettle();
    expect(api.downloads, [
      ['q2', 'd2', 'liste.png']
    ]);
  });

  testWidgets('Dokument hochladen: Dateiauswahl, Upload, Liste neu geladen', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      types: [
        {'id': 't1', 'name': 'Erste Hilfe'},
      ],
      records: recordsWithOne(),
    );
    final accepts = <String?>[];
    browser.PickedFile? next = browser.PickedFile(Uint8List.fromList([1, 2, 3]), 'scan.pdf', 'application/pdf');
    Future<browser.PickedFile?> picker({String? accept}) async {
      accepts.add(accept);
      return next;
    }

    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1', filePicker: picker)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Dokumente').first);
    await tester.pumpAndSettle();
    expect(find.text('PDF, JPEG oder PNG, höchstens 10 MB. Dokumente werden nicht gelöscht.'), findsOneWidget);

    await tester.tap(find.text('Datei hochladen'));
    await tester.pumpAndSettle();
    expect(accepts, [qualificationDocumentAccept]);
    expect(api.uploads, [
      {'qual': 'q1', 'filename': 'scan.pdf', 'length': 3, 'contentType': 'application/pdf'}
    ]);
    expect(api.documentListCalls, ['q1', 'q1']);
    expect(find.text('scan.pdf · 3 B · hochgeladen 22.10.2026'), findsOneWidget);

    // Auswahl abgebrochen: nichts passiert.
    next = null;
    await tester.tap(find.text('Datei hochladen'));
    await tester.pumpAndSettle();
    expect(api.uploads.length, 1);
    expect(api.documentListCalls.length, 2);
  });

  testWidgets('zu grosse und leere Dateien werden nicht hochgeladen', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      records: recordsWithOne(),
    );
    var next = browser.PickedFile(Uint8List(10 * 1024 * 1024 + 1), 'riesig.pdf', 'application/pdf');
    Future<browser.PickedFile?> picker({String? accept}) async => next;

    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1', filePicker: picker)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Dokumente').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Datei hochladen'));
    await tester.pumpAndSettle();
    expect(find.text('Die Datei ist zu groß (10,0 MB, höchstens 10 MB).'), findsOneWidget);

    next = browser.PickedFile(Uint8List(0), 'leer.pdf', 'application/pdf');
    await tester.tap(find.text('Datei hochladen'));
    await tester.pumpAndSettle();
    expect(find.text('Die Datei ist leer.'), findsOneWidget);
    expect(api.uploads, isEmpty);
    // Der Dialog bleibt offen.
    expect(find.text('Nachweisdokumente'), findsOneWidget);
  });

  testWidgets('Fehler des Servers beim Hochladen wird im Dialog angezeigt', (tester) async {
    await _desktop(tester);
    final api = _FakeQualApi(
      permissions: const {'qualifications.read', 'qualifications.write'},
      records: recordsWithOne(),
      uploadError: const ApiException(statusCode: 400, message: 'Nur PDF, JPEG oder PNG erlaubt'),
    );
    Future<browser.PickedFile?> picker({String? accept}) async =>
        browser.PickedFile(Uint8List.fromList([1]), 'notiz.txt', 'text/plain');

    await tester.pumpWidget(_wrap(EmployeeQualificationsSection(api: api, employeeId: 'e1', filePicker: picker)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Dokumente').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Datei hochladen'));
    await tester.pumpAndSettle();
    expect(find.text('Upload fehlgeschlagen: Nur PDF, JPEG oder PNG erlaubt'), findsOneWidget);
    // Nach dem Fehler bleibt der Knopf bedienbar.
    final button = tester.widget<TextButton>(find.ancestor(of: find.text('Datei hochladen'), matching: find.byWidgetPredicate((w) => w is TextButton)));
    expect(button.onPressed, isNotNull);
    expect(api.documentListCalls, ['q1']);
  });

  group('API-Methoden der Nachweisdokumente', () {
    final api = ApiClient(baseUrl: 'http://api.test');
    late List<http.Request> calls;

    Future<T> mock<T>(Object body, Future<T> Function() call, {int status = 200, Map<String, String>? headers}) {
      final client = MockClient((req) async {
        calls.add(req);
        final bytes = body is List<int> ? body : utf8.encode(jsonEncode(body));
        return http.Response.bytes(bytes, status, headers: headers ?? {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    setUp(() => calls = []);

    test('auflisten und herunterladen ueber die eigenen Routen', () async {
      final list = await mock([
        {'id': 'd1'}
      ], () => api.listQualificationDocuments('q1'));
      await mock(<int>[37, 80, 68, 70], () => api.downloadQualificationDocument('q1', 'd1', filename: 'scan.pdf'),
          headers: {'content-type': 'application/pdf'});
      expect(calls.map((c) => '${c.method} ${c.url.path}'), [
        'GET /api/v1/qualifications/q1/documents',
        'GET /api/v1/qualifications/q1/documents/d1',
      ]);
      expect(list.single['id'], 'd1');
    });

    test('hochladen als Multipart im Feld file, erwartet 201', () async {
      final out = await mock({'id': 'd1', 'filename': 'scan.pdf'},
          () => api.uploadQualificationDocument('q1', 'scan.pdf', Uint8List.fromList(utf8.encode('%PDF-1.4')), contentType: 'application/pdf'),
          status: 201);
      expect(out['id'], 'd1');
      final req = calls.single;
      expect('${req.method} ${req.url.path}', 'POST /api/v1/qualifications/q1/documents');
      expect(req.headers['content-type'], startsWith('multipart/form-data'));
      final body = utf8.decode(req.bodyBytes);
      expect(body, contains('name="file"; filename="scan.pdf"'));
      expect(body, contains('content-type: application/pdf'));
      expect(body, contains('%PDF-1.4'));
    });

    test('Fehler beim Hochladen und Herunterladen werden zur ApiException', () async {
      await expectLater(
        mock({'error': {'code': 'validation_error', 'message': 'Nur PDF, JPEG oder PNG erlaubt'}},
            () => api.uploadQualificationDocument('q1', 'x.txt', Uint8List.fromList([1])),
            status: 400),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Nur PDF, JPEG oder PNG erlaubt')),
      );
      // 200 statt 201 ist kein Erfolg.
      await expectLater(mock({'id': 'd1'}, () => api.uploadQualificationDocument('q1', 'x.pdf', Uint8List.fromList([1])), status: 200),
          throwsA(isA<ApiException>()));
      await expectLater(
        mock({'error': {'code': 'not_found', 'message': 'Dokument nicht gefunden'}}, () => api.downloadQualificationDocument('q1', 'dx'),
            status: 404),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Dokument nicht gefunden')),
      );
    });
  });
}
