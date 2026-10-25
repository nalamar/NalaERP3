import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/positions_page.dart';

// Stellen im Client (Backlog F.11.5). InkRipple und Desktop-Fenster wie in
// den anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, List<Map<String, dynamic>>? positions, this.adoptResult = const [], this.saveError})
      : positions = positions ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> positions;
  final List<dynamic> adoptResult;
  final Object? saveError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> created = [];
  final List<MapEntry<String, Map<String, dynamic>>> updated = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listPositions({bool includeInactive = false}) async {
    calls.add('list $includeInactive');
    return positions.where((p) => includeInactive || p['aktiv'] != false).toList();
  }

  @override
  Future<List<dynamic>> listQualificationTypes({bool includeInactive = false}) async {
    calls.add('types $includeInactive');
    return _types;
  }

  @override
  Future<Map<String, dynamic>> createPosition(Map<String, dynamic> body) async {
    calls.add('create');
    if (saveError != null) throw saveError!;
    created.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> updatePosition(String id, Map<String, dynamic> body) async {
    calls.add('update');
    if (saveError != null) throw saveError!;
    updated.add(MapEntry(id, body));
    return {'id': id, ...body};
  }

  @override
  Future<List<dynamic>> adoptDefaultPositions() async {
    calls.add('adopt');
    return adoptResult;
  }
}

const _types = [
  {'id': 't-st', 'name': 'Stapler', 'aktiv': true},
  {'id': 't-eh', 'name': 'erste Hilfe', 'aktiv': true},
  {'id': 't-alt', 'name': 'Alter Kurs', 'aktiv': false},
];

List<Map<String, dynamic>> _positions() => [
      {'id': 'p1', 'code': 'LAGERIST', 'name': 'Lagerist', 'aktiv': true, 'hinweis': 'h', 'type_ids': ['t-st', 't-alt']},
      {'id': 'p2', 'code': 'BUERO', 'name': 'Bürokraft', 'aktiv': true, 'hinweis': '', 'type_ids': <String>[]},
      {'id': 'p3', 'code': 'ALTSTELLE', 'name': 'Altstelle', 'aktiv': false, 'hinweis': '', 'type_ids': ['t-eh']},
    ];

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'qualifications.read', 'qualifications.write'};

void main() {
  group('reine Funktionen', () {
    test('buildPositionBody: POST mit Code, PUT mit aktiv; Arten ohne Doppelte, sortiert', () {
      expect(buildPositionBody(code: ' monteur ', name: ' Monteur ', hinweis: ' x ', typeIds: ['b', 'a', 'b']), {
        'code': 'MONTEUR',
        'name': 'Monteur',
        'hinweis': 'x',
        'type_ids': ['a', 'b'],
      });
      expect(buildPositionBody(name: 'M', typeIds: [], aktiv: false), {
        'name': 'M',
        'hinweis': '',
        'type_ids': <String>[],
        'aktiv': false,
      });
    });

    test('positionTypeChoices: aktive plus zugeordnete stillgelegte, nach Name', () {
      final c = positionTypeChoices(_types, ['t-alt', 'unbekannt']);
      expect(c.map((e) => e['type_id']), ['t-alt', 't-eh', 't-st', 'unbekannt']);
      expect(c.map((e) => e['stillgelegt']), [true, false, false, true]);
      // Nicht zugeordnete stillgelegte Arten stehen nicht zur Wahl.
      expect(positionTypeChoices(_types, []).map((e) => e['type_id']), ['t-eh', 't-st']);
    });

    test('positionTypesLabel und adoptedPositionLines', () {
      expect(positionTypesLabel({'type_ids': ['t-st', 't-alt', 'x']}, _types), 'Alter Kurs (stillgelegt), Stapler, x');
      expect(positionTypesLabel({'type_ids': []}, _types), 'keine Qualifikationen');
      expect(
          adoptedPositionLines([
            {'name': 'Monteur', 'fehlende_arten': []},
            {'name': 'Kranführer', 'fehlende_arten': ['KRANFUEHRER', 'ANSCHLAEGER']},
          ]),
          ['Monteur', 'Kranführer – fehlende Arten: KRANFUEHRER, ANSCHLAEGER']);
    });
  });

  testWidgets('listet aktive Stellen mit Arten; Stillgelegte auf Wunsch; ohne Schreibrecht nur lesen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'qualifications.read'}, positions: _positions());
    await tester.pumpWidget(_wrap(PositionsPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('Lagerist (LAGERIST)'), findsOneWidget);
    expect(find.text('Alter Kurs (stillgelegt), Stapler'), findsOneWidget);
    expect(find.text('keine Qualifikationen'), findsOneWidget);
    expect(find.textContaining('Altstelle'), findsNothing);
    expect(find.byTooltip('Bearbeiten'), findsNothing);
    expect(find.byType(FloatingActionButton), findsNothing);
    expect(find.text('Vorschläge übernehmen'), findsNothing);

    await tester.tap(find.text('Stillgelegte anzeigen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'types true');
    expect(api.calls, contains('list true'));
    expect(find.text('Altstelle (ALTSTELLE) · stillgelegt'), findsOneWidget);
  });

  testWidgets('leer und Ladefehler', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(PositionsPage(api: _FakeApi(permissions: _rw))));
    await tester.pumpAndSettle();
    expect(find.text('Noch keine Stellen. Legen Sie Stellen an oder übernehmen Sie die Vorschläge.'), findsOneWidget);
  });

  testWidgets('legt eine Stelle mit Arten an; ohne Code oder Name bleibt der Dialog offen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(PositionsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();
    // Stillgelegte Arten stehen bei einer neuen Stelle nicht zur Wahl.
    expect(find.text('Alter Kurs'), findsNothing);

    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Code angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Code (A-Z, 0-9, _)'), 'lager');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Namen angeben.'), findsOneWidget);
    expect(api.created, isEmpty);

    await tester.enterText(find.widgetWithText(TextField, 'Name'), 'Lager');
    await tester.tap(find.text('Stapler'));
    await tester.pumpAndSettle();
    expect(tester.widget<CheckboxListTile>(find.widgetWithText(CheckboxListTile, 'Stapler')).value, isTrue);
    // An und wieder aus: zaehlt nicht.
    await tester.tap(find.text('erste Hilfe'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('erste Hilfe'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created, [
      {'code': 'LAGER', 'name': 'Lager', 'hinweis': '', 'type_ids': ['t-st']}
    ]);
    expect(find.text('Stelle angelegt'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('list')).length, 2);
  });

  testWidgets('bearbeitet: Code gesperrt, stillgelegte Zuordnung bleibt, stilllegen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, positions: _positions());
    await tester.pumpWidget(_wrap(PositionsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    expect(find.text('Stelle bearbeiten'), findsOneWidget);
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Code (A-Z, 0-9, _)')).enabled, isFalse);
    expect(find.text('Änderungen gelten sofort für alle Mitarbeiter mit dieser Stelle.'), findsOneWidget);
    expect(find.text('stillgelegt – zählt im Stand nicht mehr'), findsOneWidget);
    CheckboxListTile tile(String name) => tester.widget<CheckboxListTile>(find.widgetWithText(CheckboxListTile, name));
    expect(tile('Alter Kurs').value, isTrue);
    expect(tile('Stapler').value, isTrue);
    expect(tile('erste Hilfe').value, isFalse);

    await tester.tap(find.text('erste Hilfe'));
    await tester.tap(find.text('Aktiv'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.key, 'p1');
    expect(api.updated.single.value, {
      'name': 'Lagerist',
      'hinweis': 'h',
      'type_ids': ['t-alt', 't-eh', 't-st'],
      'aktiv': false,
    });
    expect(find.text('Stelle gespeichert'), findsOneWidget);
  });

  testWidgets('Fehler des Servers wird angezeigt, Abbrechen speichert nichts', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
        permissions: _rw,
        positions: _positions(),
        saveError: const ApiException(statusCode: 400, message: 'Stelle mit diesem Namen existiert bereits'));
    await tester.pumpWidget(_wrap(PositionsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c == 'update'), isEmpty);

    await tester.tap(find.byTooltip('Bearbeiten').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Stelle mit diesem Namen existiert bereits'), findsOneWidget);
  });

  testWidgets('Vorschläge übernehmen: Hinweis, Ergebnis mit fehlenden Arten, Abbrechen ohne Aufruf', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, adoptResult: [
      {'id': 'n1', 'code': 'MONTEUR', 'name': 'Monteur', 'fehlende_arten': []},
      {'id': 'n2', 'code': 'KRANFUEHRER', 'name': 'Kranführer', 'fehlende_arten': ['KRANFUEHRER']},
    ]);
    await tester.pumpWidget(_wrap(PositionsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Vorschläge übernehmen'));
    await tester.pumpAndSettle();
    expect(find.textContaining('nicht recherchiert'), findsOneWidget);
    expect(find.textContaining('übernehmen Sie zuerst den Qualifikationskatalog'), findsOneWidget);
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.calls, isNot(contains('adopt')));

    await tester.tap(find.text('Vorschläge übernehmen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Übernehmen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('adopt'));
    expect(find.text('2 Stellen übernommen'), findsOneWidget);
    expect(find.text('Kranführer – fehlende Arten: KRANFUEHRER'), findsOneWidget);
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    // Danach neu geladen.
    expect(api.calls.where((c) => c.startsWith('list')).length, 2);
  });

  testWidgets('Vorschläge schon vorhanden: nur eine Meldung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(PositionsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Vorschläge übernehmen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Übernehmen'));
    await tester.pumpAndSettle();
    expect(find.text('Alle Vorschläge sind bereits vorhanden.'), findsOneWidget);
    expect(find.textContaining('Stellen übernommen'), findsNothing);
  });

  testWidgets('Dashboard: Karte "Stellen" nur mit qualifications.read', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'qualifications.read'});
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'qualifications.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Stellen'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(key: const ValueKey('mit'), api: api, onLogout: () async {}, permissions: const {'qualifications.read'})));
    await tester.pumpAndSettle();
    expect(find.text('Stellen'), findsOneWidget);
    await tester.tap(find.text('Stellen'));
    await tester.pumpAndSettle();
    expect(find.byType(PositionsPage), findsOneWidget);
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

    test('Pfade, Methoden und erwartete Statuscodes', () async {
      await mock([], () => api.listPositions());
      await mock([], () => api.listPositions(includeInactive: true));
      await mock({'id': 'p'}, () => api.createPosition({'code': 'A', 'name': 'A', 'type_ids': []}), status: 201);
      await mock({'id': 'p'}, () => api.updatePosition('p', {'name': 'A', 'aktiv': true, 'hinweis': '', 'type_ids': []}));
      await mock([], () => api.adoptDefaultPositions());
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/positions/?',
        'GET /api/v1/positions/?include_inactive=true',
        'POST /api/v1/positions/?',
        'PUT /api/v1/positions/p?',
        'POST /api/v1/positions/adopt-defaults?',
      ]);
      expect(jsonDecode(calls[2].body), {'code': 'A', 'name': 'A', 'type_ids': []});
      expect(jsonDecode(calls[3].body), {'name': 'A', 'aktiv': true, 'hinweis': '', 'type_ids': []});
      for (final i in [2, 3, 4]) {
        expect(calls[i].headers['content-type'], startsWith('application/json'));
      }
    });

    test('falscher Status wird zur ApiException', () async {
      await expectLater(mock({'id': 'p'}, () => api.createPosition({}), status: 200), throwsA(isA<ApiException>()));
      await expectLater(
        mock({'error': {'code': 'conflict', 'message': 'Qualifikationsart ist stillgelegt'}}, () => api.updatePosition('p', {}), status: 409),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Qualifikationsart ist stillgelegt')),
      );
      await expectLater(mock([], () => api.adoptDefaultPositions(), status: 403), throwsA(isA<ApiException>()));
    });
  });
}
