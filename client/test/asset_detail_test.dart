import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/asset_detail_page.dart';
import 'package:nalaerp_client/pages/assets_page.dart';

// Detailseite eines Betriebsmittels: Ausgaben (Backlog F.13.4.1). Feste
// Uhr; InkRipple und Desktop-Fenster wie in den anderen Client-Tests
// (Backlog 0.48).

final _now = DateTime(2026, 10, 19, 10);

class _FakeApi extends ApiClient {
  _FakeApi({
    required this.permissions,
    List<Map<String, dynamic>>? assignments,
    this.issueHints = const [],
    this.issueError,
  })  : assignments = assignments ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> assignments;
  final List<String> issueHints;
  final Object? issueError;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listAssetAssignments({String? assetId, String? employeeId, bool nurOffen = false, bool includeStorniert = false}) async {
    calls.add('list $assetId');
    return assignments.where((a) => a['status'] == 'active').toList();
  }

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async => [
        {'id': 'e2', 'first_name': 'Zora', 'last_name': 'Zell', 'active': true},
        {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg', 'active': true},
        {'id': 'e9', 'first_name': 'Otto', 'last_name': 'Alt', 'active': false},
      ];

  @override
  Future<Map<String, dynamic>> issueAsset({required String assetId, required String employeeId, required String ausgegebenAm, String notiz = ''}) async {
    calls.add('issue $assetId $employeeId $ausgegebenAm $notiz');
    if (issueError != null) throw issueError!;
    assignments.add({'id': 'neu', 'mitarbeiter_name': 'Anna Berg', 'ausgegeben_am': '${ausgegebenAm}T00:00:00Z', 'status': 'active'});
    return {'id': 'neu', if (issueHints.isNotEmpty) 'hinweise': issueHints};
  }

  @override
  Future<Map<String, dynamic>> returnAsset(String assignmentId, {required String datum, String zustand = ''}) async {
    calls.add('return $assignmentId $datum $zustand');
    assignments.firstWhere((a) => a['id'] == assignmentId)['zurueckgegeben_am'] = '${datum}T00:00:00Z';
    return {'id': assignmentId};
  }

  @override
  Future<Map<String, dynamic>> stornoAssetAssignment(String assignmentId, String reason) async {
    calls.add('storno $assignmentId $reason');
    return {'id': assignmentId};
  }

  @override
  Future<List<dynamic>> listAssets({String? kategorie, bool includeAusgemustert = false}) async => [_asset()];

  @override
  Future<List<dynamic>> listCostCenters({bool includeInactive = false}) async => const [];

  // Pruefungen (F.13.4.2) - hier ohne Belang, aber ohne Netzwerk.
  @override
  Future<Map<String, dynamic>> getAssetInspectionStanding(String assetId) async => {'status': 'nie_geprueft'};

  @override
  Future<List<dynamic>> listAssetInspections(String assetId, {bool includeStorniert = false}) async => const [];
}

Map<String, dynamic> _asset({bool retired = false}) => {
      'id': 'a1',
      'inventarnummer': 'L-1',
      'bezeichnung': 'Leiter',
      'kategorie': 'leiter',
      'standort': 'Halle 2',
      'pruefintervall_monate': 12,
      'seriennummer': 'S9',
      'ausgemustert_am': retired ? '2026-09-01T00:00:00Z' : null,
      'ausgemustert_grund': retired ? 'Holm gebrochen' : '',
    };

List<Map<String, dynamic>> _history() => [
      {'id': 'x2', 'mitarbeiter_name': 'Anna Berg', 'ausgegeben_am': '2026-10-01T00:00:00Z', 'status': 'active'},
      {'id': 'x1', 'mitarbeiter_name': 'Bert Cord', 'ausgegeben_am': '2026-09-01T00:00:00Z', 'zurueckgegeben_am': '2026-09-20T00:00:00Z', 'rueckgabe_zustand': 'Holm verbogen', 'status': 'active'},
    ];

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'assets.read', 'assets.write'};

// Button im Dialog - die Seite hat gleichnamige Buttons.
Finder _dlg(String text) => find.descendant(of: find.byType(AlertDialog), matching: find.widgetWithText(FilledButton, text));

void main() {
  group('reine Funktionen', () {
    test('assignmentLine', () {
      final h = _history();
      expect(assignmentLine(h[0]), 'Anna Berg · seit 01.10.2026');
      expect(assignmentLine(h[1]), 'Bert Cord · 01.09.2026 – 20.09.2026 (Holm verbogen)');
      expect(assignmentLine({...h[1], 'rueckgabe_zustand': ''}), 'Bert Cord · 01.09.2026 – 20.09.2026');
    });

    test('openAssignment: nur aktiv und nicht zurueckgegeben', () {
      expect(openAssignment(_history())?['id'], 'x2');
      expect(openAssignment([_history()[1]]), isNull);
      expect(openAssignment([{..._history()[0], 'status': 'storniert'}]), isNull);
    });

    test('activeEmployees: ohne inaktive, nach Nachname', () {
      final out = activeEmployees([
        {'id': 'e2', 'first_name': 'Zora', 'last_name': 'Zell', 'active': true},
        {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg'},
        {'id': 'e9', 'first_name': 'Otto', 'last_name': 'Alt', 'active': false},
      ]);
      expect(out.map((e) => e['id']), ['e1', 'e2']);
    });
  });

  testWidgets('zeigt Halter und Verlauf; ohne Schreibrecht keine Aktionen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'assets.read'}, assignments: _history());
    await tester.pumpWidget(_wrap(AssetDetailPage(api: api, asset: _asset(), now: () => _now)));
    await tester.pumpAndSettle();
    expect(api.calls, ['list a1']);
    expect(find.text('L-1 – Leiter'), findsOneWidget);
    expect(find.text('Leiter · Standort Halle 2 · Prüfung alle 12 Monate · SN S9'), findsOneWidget);
    expect(find.text('Ausgegeben an Anna Berg · seit 01.10.2026'), findsOneWidget);
    expect(find.text('Bert Cord · 01.09.2026 – 20.09.2026 (Holm verbogen)'), findsOneWidget);
    // Abschnitt Pruefungen (F.13.4.2) auf derselben Seite.
    expect(find.text('Prüfungen'), findsOneWidget);
    expect(find.text('Prüfstand: noch nie geprüft'), findsOneWidget);
    expect(find.text('Zurücknehmen'), findsNothing);
    expect(find.text('Ausgeben'), findsNothing);
    expect(find.byTooltip('Stornieren'), findsNothing);
  });

  testWidgets('ausgeben: nur aktive Mitarbeiter, Pruefung im Dialog, Aufruf mit ISO-Datum', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(AssetDetailPage(api: api, asset: _asset(), now: () => _now)));
    await tester.pumpAndSettle();
    expect(find.text('Nicht ausgegeben'), findsOneWidget);
    expect(find.text('Noch nie ausgegeben.'), findsOneWidget);
    await tester.tap(find.text('Ausgeben'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Ausgegeben am (TT.MM.JJJJ)')).controller!.text, '19.10.2026');

    await tester.tap(_dlg('Ausgeben'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Mitarbeiter auswählen.'), findsOneWidget);

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    expect(find.text('Alt, Otto'), findsNothing);
    final berg = tester.getTopLeft(find.text('Berg, Anna').last).dy;
    final zell = tester.getTopLeft(find.text('Zell, Zora').last).dy;
    expect(berg, lessThan(zell));
    await tester.tap(find.text('Berg, Anna').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Ausgegeben am (TT.MM.JJJJ)'), '32.10.2026');
    await tester.tap(_dlg('Ausgeben'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Datum als TT.MM.JJJJ angeben.'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('issue')), isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Ausgegeben am (TT.MM.JJJJ)'), '18.10.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Notiz'), ' mit Haken ');
    await tester.tap(_dlg('Ausgeben'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('issue')), ['issue a1 e1 2026-10-18 mit Haken']);
    expect(find.text('Ausgegeben'), findsOneWidget);
    expect(find.text('Ausgegeben an Anna Berg · seit 18.10.2026'), findsOneWidget);
  });

  testWidgets('Hinweise nach der Ausgabe als Dialog; Ablehnung bei Sperre als Meldung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, issueHints: ['Prüfung überfällig seit 01.01.2026']);
    await tester.pumpWidget(_wrap(AssetDetailPage(api: api, asset: _asset(), now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Ausgeben'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Berg, Anna').last);
    await tester.pumpAndSettle();
    await tester.tap(_dlg('Ausgeben'));
    await tester.pumpAndSettle();
    expect(find.text('Ausgegeben – bitte beachten'), findsOneWidget);
    expect(find.text('⚠ Prüfung überfällig seit 01.01.2026'), findsOneWidget);
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();

    final locked = _FakeApi(
      permissions: _rw,
      issueError: const ApiException(statusCode: 409, code: 'conflict', message: 'Asset ist gesperrt: letzte Prüfung mit Mängeln'),
    );
    await tester.pumpWidget(_wrap(AssetDetailPage(key: const ValueKey('gesperrt'), api: locked, asset: _asset(), now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Ausgeben'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Berg, Anna').last);
    await tester.pumpAndSettle();
    await tester.tap(_dlg('Ausgeben'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Asset ist gesperrt: letzte Prüfung mit Mängeln'), findsOneWidget);
    expect(find.text('Nicht ausgegeben'), findsOneWidget);
  });

  testWidgets('ausgemustert: kein Ausgeben', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(AssetDetailPage(api: api, asset: _asset(retired: true), now: () => _now)));
    await tester.pumpAndSettle();
    expect(find.text('Ausgemustert am 01.09.2026 (Holm gebrochen)'), findsOneWidget);
    expect(find.text('Ausgeben'), findsNothing);
  });

  testWidgets('zuruecknehmen mit Datum und Zustand', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, assignments: _history());
    await tester.pumpWidget(_wrap(AssetDetailPage(api: api, asset: _asset(), now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Zurücknehmen'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Zurückgegeben am (TT.MM.JJJJ)')).controller!.text, '19.10.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Zustand (optional)'), ' Sprosse locker ');
    await tester.tap(_dlg('Zurücknehmen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('return')), ['return x2 2026-10-19 Sprosse locker']);
    expect(find.text('Nicht ausgegeben'), findsOneWidget);
  });

  testWidgets('stornieren nur mit Begruendung, an der richtigen Ausgabe', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, assignments: _history());
    await tester.pumpWidget(_wrap(AssetDetailPage(api: api, asset: _asset(), now: () => _now)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Stornieren').at(1));
    await tester.pumpAndSettle();
    final button = find.widgetWithText(FilledButton, 'Stornieren');
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), ' falscher Mitarbeiter ');
    await tester.pumpAndSettle();
    await tester.tap(button);
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('storno')), ['storno x1 falscher Mitarbeiter']);
    expect(api.calls.last, 'list a1');
  });

  testWidgets('Liste der Betriebsmittel oeffnet die Detailseite', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, assignments: _history());
    await tester.pumpWidget(_wrap(AssetsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('L-1 – Leiter'));
    await tester.pumpAndSettle();
    expect(find.byType(AssetDetailPage), findsOneWidget);
    expect(find.text('Ausgegeben an Anna Berg · seit 01.10.2026'), findsOneWidget);
  });

  test('API: ausgeben (201), zuruecknehmen, stornieren', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final status = req.url.path == '/api/v1/asset-assignments/' ? 201 : 200;
      return http.Response.bytes(utf8.encode('{"id":"x","hinweise":["h"]}'), status, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      final out = await api.issueAsset(assetId: 'a1', employeeId: 'e1', ausgegebenAm: '2026-10-18', notiz: 'n');
      expect(out['hinweise'], ['h']);
      await api.returnAsset('x1', datum: '2026-10-19', zustand: 'ok');
      await api.stornoAssetAssignment('x2', 'falsch');
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}'), [
      'POST /api/v1/asset-assignments/',
      'POST /api/v1/asset-assignments/x1/return',
      'POST /api/v1/asset-assignments/x2/storno',
    ]);
    expect(jsonDecode(calls[0].body), {'asset_id': 'a1', 'employee_id': 'e1', 'ausgegeben_am': '2026-10-18', 'notiz': 'n'});
    expect(jsonDecode(calls[1].body), {'datum': '2026-10-19', 'zustand': 'ok'});
    expect(jsonDecode(calls[2].body), {'reason': 'falsch'});
    final ok200 = MockClient((req) async => http.Response('{"id":"x"}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(
        http.runWithClient(() => api.issueAsset(assetId: 'a', employeeId: 'e', ausgegebenAm: 'd'), () => ok200), throwsA(isA<ApiException>()));
  });
}
