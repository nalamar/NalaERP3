import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/production_assembly_section.dart';

// Montagetermine im Detail des Fertigungsauftrags (Backlog H.3.6e1).

Map<String, dynamic> _a({
  String id = 'a1',
  String von = '2030-03-04T00:00:00Z',
  String bis = '2030-03-06T00:00:00Z',
  String team = 'Montage Nord',
  String kennzeichen = 'MA-1',
  String baustelle = 'Hafenstr. 2',
  String notiz = '',
  List<String> hinweise = const [],
  bool storniert = false,
}) =>
    {
      'id': id,
      'von': von,
      'bis': bis,
      'team_name': team,
      'kennzeichen': kennzeichen,
      'baustelle': baustelle,
      'notiz': notiz,
      'hinweise': hinweise,
      'storniert_at': storniert ? '2026-11-12T08:00:00Z' : null,
      'storno_reason': storniert ? 'verschoben' : '',
    };

class _FakeApi extends ApiClient {
  _FakeApi({this.list = const [], this.fail, this.permissions = const {'production.read'}, this.failWrite, this.failChoices = false})
      : super(baseUrl: 'http://localhost:8080');

  final List<dynamic> list;
  final String? fail;
  final Set<String> permissions;
  final String? failWrite;
  final bool failChoices;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listProductionAssembly(String orderId) async {
    calls.add('list $orderId');
    if (fail != null) throw ApiException(statusCode: 404, message: fail!);
    return list;
  }

  @override
  Future<Map<String, dynamic>> getProductionAssemblyChoices() async {
    calls.add('choices');
    if (failChoices) throw ApiException(statusCode: 403, message: 'Keine Berechtigung');
    return {
      'teams': [
        {'id': 't1', 'name': 'Montage Nord'},
        {'id': 't2', 'name': 'Montage Süd'},
      ],
      'fahrzeuge': [
        {'id': 'v1', 'kennzeichen': 'MA-1', 'bezeichnung': 'Sprinter', 'abgemeldet_am': null},
      ],
    };
  }

  Map<String, dynamic> _written(String call) {
    calls.add(call);
    if (failWrite != null) throw ApiException(statusCode: 409, message: failWrite!);
    return _a(hinweise: ['team_doppelt']);
  }

  @override
  Future<Map<String, dynamic>> createProductionAssembly(String orderId, Map<String, dynamic> body) async =>
      _written('create $orderId ${jsonEncode(body)}');

  @override
  Future<Map<String, dynamic>> updateProductionAssembly(String orderId, String id, Map<String, dynamic> body) async =>
      _written('update $orderId $id ${jsonEncode(body)}');

  @override
  Future<Map<String, dynamic>> cancelProductionAssembly(String orderId, String id, String reason) async {
    _written('cancel $orderId $id $reason');
    return _a(storniert: true);
  }
}

const _write = {'production.read', 'production.write'};

Future<void> _pump(WidgetTester tester, _FakeApi api, {String status = 'freigegeben'}) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
    home: Scaffold(body: SingleChildScrollView(child: ProductionAssemblySection(api: api, orderId: 'o1', status: status))),
  ));
  await tester.pumpAndSettle();
}

Finder _field(String label) => find.widgetWithText(TextField, label);

void main() {
  test('Zeitraum, Zeile und Tage', () {
    expect(assemblyDays(_a()), 3);
    expect(assemblyRange(_a()), '04.03.2030 – 06.03.2030 (3 Tage)');
    expect(assemblyRange(_a(bis: '2030-03-04T00:00:00Z')), '04.03.2030 (1 Tag)');
    // Über den Monatswechsel und mit Uhrzeit: Kalendertage zählen.
    expect(assemblyDays(_a(von: '2030-02-27T23:00:00Z', bis: '2030-03-01T01:00:00Z')), 3);
    expect(assemblyDays({'von': null, 'bis': '2030-03-01'}), 0);
    expect(assemblyLine(_a()), '04.03.2030 – 06.03.2030 (3 Tage) · Montage Nord · MA-1 · Hafenstr. 2');
    expect(assemblyLine(_a(team: '', kennzeichen: '', baustelle: '')), '04.03.2030 – 06.03.2030 (3 Tage) · ohne Team');
  });

  test('Hinweise und Details', () {
    expect(assemblyHints(_a(hinweise: ['team_doppelt', 'nach_liefertermin', 'neu'])),
        ['Team ist im Zeitraum doppelt eingeplant', 'Montage endet nach dem Liefertermin', 'neu']);
    expect(assemblyHints({'hinweise': null}), isEmpty);
    for (final code in ['team_doppelt', 'team_abwesend', 'fahrzeug_gesperrt', 'vor_fertigstellung', 'nach_liefertermin']) {
      expect(assemblyHintLabels[code], isNotNull, reason: code);
    }
    expect(assemblyDetails(_a()), isNull);
    expect(assemblyDetails(_a(hinweise: ['team_abwesend'], notiz: 'Kran bestellt')), 'Hinweis: Abwesenheit im Team\nNotiz: Kran bestellt');
    expect(assemblyDetails(_a(storniert: true)), 'Storniert 12.11.2026: verschoben');
    expect(assemblyCancelled(_a()), isFalse);
    expect(assemblyCancelled(_a(storniert: true)), isTrue);
  });

  testWidgets('Liste mit Hinweisen und Storno', (tester) async {
    final api = _FakeApi(list: [
      _a(hinweise: ['fahrzeug_gesperrt']),
      _a(id: 'a2', von: '2030-03-09T00:00:00Z', bis: '2030-03-09T00:00:00Z', team: ''),
      _a(id: 'a3', storniert: true),
    ]);
    await _pump(tester, api);
    expect(api.calls, ['list o1']);
    expect(find.text('Montage'), findsOneWidget);
    expect(find.text('04.03.2030 – 06.03.2030 (3 Tage) · Montage Nord · MA-1 · Hafenstr. 2'), findsNWidgets(2));
    expect(find.text('09.03.2030 (1 Tag) · ohne Team · MA-1 · Hafenstr. 2'), findsOneWidget);
    expect(find.text('Hinweis: Fahrzeug ist gesperrt (HU/UVV)'), findsOneWidget);
    expect(find.text('Storniert 12.11.2026: verschoben'), findsOneWidget);
    expect(find.byIcon(Icons.warning_amber_rounded), findsOneWidget);
    expect(find.byIcon(Icons.event_available_outlined), findsOneWidget);
    // Ohne Hinweis: Kalender statt Warnung in genau dieser Zeile.
    final plain = find.ancestor(of: find.text('09.03.2030 (1 Tag) · ohne Team · MA-1 · Hafenstr. 2'), matching: find.byType(ListTile));
    expect(find.descendant(of: plain, matching: find.byIcon(Icons.event_available_outlined)), findsOneWidget);
    expect(find.byIcon(Icons.event_busy_outlined), findsOneWidget);
    final struck = tester.widgetList<Text>(find.text('04.03.2030 – 06.03.2030 (3 Tage) · Montage Nord · MA-1 · Hafenstr. 2')).toList();
    expect(struck.map((t) => t.style?.decoration), [null, TextDecoration.lineThrough]);
    expect(find.text('Noch keine Montagetermine.'), findsNothing);
  });

  testWidgets('leer', (tester) async {
    await _pump(tester, _FakeApi());
    expect(find.text('Noch keine Montagetermine.'), findsOneWidget);
  });

  testWidgets('Fehler beim Laden', (tester) async {
    await _pump(tester, _FakeApi(fail: 'Fertigungsauftrag nicht gefunden'));
    expect(find.text('Montagetermine konnten nicht geladen werden: Fertigungsauftrag nicht gefunden'), findsOneWidget);
    expect(find.text('Noch keine Montagetermine.'), findsNothing);
  });

  test('Eingaben prüfen wie der Server (H.3.6e2)', () {
    final ok =
        buildAssemblyBody(von: '4.3.2030', bis: '2030-03-06', teamId: 't1', vehicleId: 'v1', baustelle: ' Hafenstr. 2 ', notiz: ' Kran ');
    expect(ok.error, isNull);
    expect(ok.body,
        {'von': '2030-03-04', 'bis': '2030-03-06', 'team_id': 't1', 'vehicle_id': 'v1', 'baustelle': 'Hafenstr. 2', 'notiz': 'Kran'});
    expect(buildAssemblyBody(von: '04.03.2030', bis: '04.03.2030', teamId: '', vehicleId: null).body,
        {'von': '2030-03-04', 'bis': '2030-03-04', 'baustelle': '', 'notiz': ''});
    // 60 Tage gehen, 61 nicht.
    expect(buildAssemblyBody(von: '01.03.2030', bis: '29.04.2030').error, isNull);
    expect(buildAssemblyBody(von: '01.03.2030', bis: '30.04.2030').error, 'Montage höchstens 60 Tage.');
    for (final c in [
      ('', '06.03.2030', 'Bitte Beginn und Ende als TT.MM.JJJJ angeben.'),
      ('04.03.2030', '', 'Bitte Beginn und Ende als TT.MM.JJJJ angeben.'),
      ('31.02.2030', '06.03.2030', 'Bitte Beginn und Ende als TT.MM.JJJJ angeben.'),
      ('06.03.2030', '05.03.2030', 'Das Ende liegt vor dem Beginn.'),
    ]) {
      final r = buildAssemblyBody(von: c.$1, bis: c.$2);
      expect((r.body, r.error), (null, c.$3), reason: '${c.$1}–${c.$2}');
    }
    // Zeichen zählen, nicht UTF-16-Einheiten (wie der Server).
    expect(buildAssemblyBody(von: '04.03.2030', bis: '04.03.2030', baustelle: '🏗' * 300).error, isNull);
    expect(buildAssemblyBody(von: '04.03.2030', bis: '04.03.2030', baustelle: '🏗' * 301).error, 'Baustelle: höchstens 300 Zeichen.');
    expect(buildAssemblyBody(von: '04.03.2030', bis: '04.03.2030', notiz: 'ä' * 500).error, isNull);
    expect(buildAssemblyBody(von: '04.03.2030', bis: '04.03.2030', notiz: 'ä' * 501).error, 'Notiz: höchstens 500 Zeichen.');
    expect(assemblyCancelReasonError(' '), 'Bitte eine Begründung angeben.');
    expect(assemblyCancelReasonError('x' * 501), 'Begründung: höchstens 500 Zeichen.');
    expect(assemblyCancelReasonError(' verschoben '), isNull);
    expect(assemblyAllowed('fertig'), isTrue);
    expect(assemblyAllowed('entwurf'), isTrue);
    expect(assemblyAllowed('storniert'), isFalse);
  });

  test('Auswahl von Team und Fahrzeug', () {
    expect(assemblyVehicleLabel({'kennzeichen': 'MA-3', 'bezeichnung': 'Sprinter', 'abgemeldet_am': '2031-01-31T00:00:00Z'}),
        'MA-3 Sprinter (abgemeldet ab 31.01.2031)');
    expect(assemblyVehicleLabel({'kennzeichen': 'MA-1', 'bezeichnung': ''}), 'MA-1');
    final teams = [
      {'id': 't1', 'name': 'Nord'},
      {'name': 'ohne ID'},
    ];
    String name(Map<String, dynamic> t) => t['name'].toString();
    expect(assemblyChoiceOptions(teams, name, 'ohne Team', null, '').map((o) => '${o.id}=${o.label}'), ['=ohne Team', 't1=Nord']);
    expect(assemblyChoiceOptions(teams, name, 'ohne Team', 't1', 'Nord'), hasLength(2));
    // Nicht mehr zur Auswahl: bleibt mit Beschriftung, sonst mit ID.
    expect(assemblyChoiceOptions(teams, name, 'ohne Team', 't9', 'Alt').last, (id: 't9', label: 'Alt'));
    expect(assemblyChoiceOptions(teams, name, 'ohne Team', 't9', '').last, (id: 't9', label: 't9'));
  });

  testWidgets('nur lesen: keine Schaltflächen', (tester) async {
    await _pump(tester, _FakeApi(list: [_a()]));
    expect(find.text('Termin anlegen'), findsNothing);
    expect(find.byType(IconButton), findsNothing);
  });

  testWidgets('stornierter Auftrag: nur stornieren, stornierte Termine ohne Schaltflächen', (tester) async {
    await _pump(tester, _FakeApi(list: [_a(), _a(id: 'a2', storniert: true)], permissions: _write), status: 'storniert');
    expect(find.text('Termin anlegen'), findsNothing);
    expect(find.byTooltip('Termin ändern'), findsNothing);
    expect(find.byTooltip('Termin stornieren'), findsOneWidget);
  });

  testWidgets('anlegen mit Prüfung, Team wählen, Hinweise in der Meldung', (tester) async {
    final api = _FakeApi(permissions: _write);
    await _pump(tester, api);
    expect(find.byTooltip('Termin ändern'), findsNothing);
    await tester.tap(find.text('Termin anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Montagetermin anlegen'), findsOneWidget);
    await tester.enterText(_field('Beginn (TT.MM.JJJJ)'), '06.03.2030');
    await tester.enterText(_field('Ende (TT.MM.JJJJ)'), '05.03.2030');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Das Ende liegt vor dem Beginn.'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('create')), isEmpty);

    await tester.enterText(_field('Ende (TT.MM.JJJJ)'), '07.03.2030');
    await tester.tap(find.text('ohne Team'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Montage Süd').last);
    await tester.pumpAndSettle();
    await tester.enterText(_field('Baustelle (optional)'), 'Hafenstr. 2');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.calls, [
      'list o1',
      'choices',
      'create o1 {"von":"2030-03-06","bis":"2030-03-07","team_id":"t2","baustelle":"Hafenstr. 2","notiz":""}',
      'list o1',
    ]);
    expect(find.text('Montagetermin angelegt – Team ist im Zeitraum doppelt eingeplant'), findsOneWidget);
  });

  testWidgets('ändern: vorbelegt, Fahrzeug außerhalb der Auswahl bleibt', (tester) async {
    final api = _FakeApi(permissions: _write, list: [
      {..._a(notiz: 'Kran'), 'team_id': 't1', 'vehicle_id': 'v9', 'kennzeichen': 'MA-9'}
    ]);
    await _pump(tester, api);
    await tester.tap(find.byTooltip('Termin ändern'));
    await tester.pumpAndSettle();
    expect(find.text('Montagetermin ändern'), findsOneWidget);
    expect(find.text('MA-9'), findsWidgets);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.calls[2],
        'update o1 a1 {"von":"2030-03-04","bis":"2030-03-06","team_id":"t1","vehicle_id":"v9","baustelle":"Hafenstr. 2","notiz":"Kran"}');
    expect(find.text('Montagetermin geändert – Team ist im Zeitraum doppelt eingeplant'), findsOneWidget);
  });

  testWidgets('stornieren nur mit Begründung', (tester) async {
    final api = _FakeApi(permissions: _write, list: [_a()]);
    await _pump(tester, api);
    await tester.tap(find.byTooltip('Termin stornieren'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Stornieren'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte eine Begründung angeben.'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('cancel')), isEmpty);
    await tester.enterText(_field('Begründung'), ' verschoben ');
    await tester.tap(find.widgetWithText(FilledButton, 'Stornieren'));
    await tester.pumpAndSettle();
    expect(api.calls.sublist(1), ['cancel o1 a1 verschoben', 'list o1']);
    expect(find.text('Montagetermin storniert'), findsOneWidget);
  });

  testWidgets('Fehler beim Speichern und beim Laden der Auswahl als Meldung', (tester) async {
    final api = _FakeApi(permissions: _write, list: [_a()], failWrite: 'Montagetermin ist bereits storniert');
    await _pump(tester, api);
    await tester.tap(find.byTooltip('Termin stornieren'));
    await tester.pumpAndSettle();
    await tester.enterText(_field('Begründung'), 'x');
    await tester.tap(find.widgetWithText(FilledButton, 'Stornieren'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Montagetermin ist bereits storniert'), findsOneWidget);
    expect(api.calls.last, 'cancel o1 a1 x');
  });

  testWidgets('Auswahl nicht ladbar: kein Dialog', (tester) async {
    final api = _FakeApi(permissions: _write, failChoices: true);
    await _pump(tester, api);
    await tester.tap(find.text('Termin anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Montagetermin anlegen'), findsNothing);
    expect(find.text('Teams und Fahrzeuge konnten nicht geladen werden: Keine Berechtigung'), findsOneWidget);
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

    test('Pfade und Bodies', () async {
      final list = await mock([_a()], () => api.listProductionAssembly('o1'));
      await mock(_a(), () => api.createProductionAssembly('o1', {'von': '2030-03-04', 'bis': '2030-03-06'}), status: 201);
      await mock(_a(), () => api.updateProductionAssembly('o1', 'a1', {'von': '2030-03-05', 'bis': '2030-03-06'}));
      await mock(_a(storniert: true), () => api.cancelProductionAssembly('o1', 'a1', 'verschoben'));
      final ch = await mock({'teams': [], 'fahrzeuge': []}, () => api.getProductionAssemblyChoices());
      expect(calls.map((c) => '${c.method} ${c.url.path}'), [
        'GET /api/v1/production/orders/o1/montage',
        'POST /api/v1/production/orders/o1/montage',
        'PUT /api/v1/production/orders/o1/montage/a1',
        'POST /api/v1/production/orders/o1/montage/a1/stornieren',
        'GET /api/v1/production/montage/auswahl',
      ]);
      expect(list, hasLength(1));
      expect(ch['teams'], isEmpty);
      expect(calls[1].body, '{"von":"2030-03-04","bis":"2030-03-06"}');
      expect(calls[2].body, '{"von":"2030-03-05","bis":"2030-03-06"}');
      expect(calls[3].body, '{"reason":"verschoben"}');
    });

    test('Fehler als ApiException', () async {
      // Anlegen erwartet 201; 200 ist ein Fehler.
      await expectLater(mock(_a(), () => api.createProductionAssembly('o1', {})), throwsA(isA<ApiException>()));
      await expectLater(
          mock({
            'error': {'code': 'conflict', 'message': 'Montagetermin ist bereits storniert'}
          }, () => api.cancelProductionAssembly('o1', 'a1', 'x'), status: 409),
          throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Montagetermin ist bereits storniert')));
    });
  });
}
