import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/production_assembly_weeks_page.dart';
import 'package:nalaerp_client/pages/production_orders_page.dart';

// Wochenübersicht der Montagetermine je Team im Client (Backlog H.3.6f).

final _a1 = <String, dynamic>{
  'id': 'a1',
  'order_id': 'o1',
  'order_nummer': 'FA-2026-0001',
  'project_nummer': 'P-7',
  'project_name': 'Hafen',
  'von': '2026-11-13T00:00:00Z',
  'bis': '2026-11-17T00:00:00Z',
  'team_id': 't1',
  'team_name': 'Montage Nord',
  'kennzeichen': 'MA-1',
  'baustelle': 'Hafenstr. 2',
  'hinweise': ['team_doppelt'],
};
final _a2 = <String, dynamic>{
  'id': 'a2',
  'order_id': 'o2',
  'order_nummer': 'FA-2026-0002',
  'project_nummer': 'P-8',
  'project_name': 'Schule',
  'von': '2026-11-16T00:00:00Z',
  'bis': '2026-11-16T00:00:00Z',
  'team_id': 't1',
  'team_name': 'Montage Nord',
  'kennzeichen': '',
  'baustelle': '',
  'hinweise': [],
};
// ohne Team und ohne Auftrag (vorbereitet für ADR 0038 Frage 6 (b))
final _a3 = <String, dynamic>{
  'id': 'a3',
  'order_id': null,
  'order_nummer': '',
  'project_nummer': 'P-9',
  'project_name': 'Halle',
  'von': '2026-11-20T00:00:00Z',
  'bis': '2026-11-20T00:00:00Z',
  'team_id': null,
  'team_name': '',
  'hinweise': [],
};

final _weeks = {
  'von': '2026-11-09T00:00:00Z',
  'bis': '2026-11-22T00:00:00Z',
  'teams': [
    {
      'team_id': 't1',
      'team_name': 'Montage Nord',
      'termine': [_a1, _a2]
    },
    {'team_id': 't2', 'team_name': 'Montage Süd', 'termine': []},
    {
      'team_id': null,
      'team_name': '',
      'termine': [_a3]
    },
  ],
};

class _FakeApi extends ApiClient {
  _FakeApi({this.empty = false, this.fail, this.gate}) : super(baseUrl: 'http://localhost:8080');

  final bool empty;
  final String? fail;
  // haelt die Antwort zurueck, bis der Test sie freigibt
  Completer<void>? gate;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permission == 'production.read';

  @override
  Future<Map<String, dynamic>> getProductionAssemblyWeeks({String? von, String? bis}) async {
    calls.add('$von $bis');
    await gate?.future;
    if (fail != null) throw ApiException(statusCode: 400, message: fail!);
    if (empty) return {'von': '2026-11-09T00:00:00Z', 'bis': '2027-01-03T00:00:00Z', 'teams': []};
    return _weeks;
  }

  @override
  Future<Map<String, dynamic>> getProductionOrder(String id) async => throw ApiException(statusCode: 404, message: 'nicht gefunden');
}

Widget _wrap(Widget child, {Key? key}) =>
    MaterialApp(key: key, theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory), home: child);

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

// Mittwoch; Montag der Woche ist der 09.11.2026.
final _today = DateTime(2026, 11, 11, 15, 30);

void main() {
  test('Wochen des Zeitraums', () {
    expect(assemblyWeekStarts('2026-11-09T00:00:00Z', '2026-11-22T00:00:00Z'), [DateTime(2026, 11, 9), DateTime(2026, 11, 16)]);
    expect(assemblyWeekStarts('2026-12-28', '2027-01-10'), [DateTime(2026, 12, 28), DateTime(2027, 1, 4)]);
    // Beginn mitten in der Woche zählt ab Montag
    expect(assemblyWeekStarts('2026-11-11', '2026-11-11'), [DateTime(2026, 11, 9)]);
    // Ende an einem Montag schließt dessen Woche ein
    expect(assemblyWeekStarts('2026-11-09', '2026-11-16'), [DateTime(2026, 11, 9), DateTime(2026, 11, 16)]);
    // über die Zeitumstellung in Kalendertagen
    expect(assemblyWeekStarts('2027-03-22', '2027-04-04'), [DateTime(2027, 3, 22), DateTime(2027, 3, 29)]);
    expect(assemblyWeekStarts('2026-11-16', '2026-11-08'), isEmpty);
    expect(assemblyWeekStarts('kaputt', '2026-11-22'), isEmpty);
    expect(assemblyWeekStarts('2026-11-09', null), isEmpty);
  });

  test('Termin in der Woche von Montag bis Sonntag', () {
    final mo = DateTime(2026, 11, 16);
    Map<String, dynamic> t(String von, String? bis) => {'von': von, 'bis': bis};
    expect(assemblyInWeek(_a1, mo), isTrue);
    expect(assemblyInWeek(_a1, DateTime(2026, 11, 9)), isTrue);
    expect(assemblyInWeek(_a1, DateTime(2026, 11, 23)), isFalse);
    expect(assemblyInWeek(t('2026-11-10', '2026-11-16'), mo), isTrue);
    expect(assemblyInWeek(t('2026-11-22', '2026-11-25'), mo), isTrue);
    // eine Uhrzeit zählt nicht: Sonntag bleibt in der Woche
    expect(assemblyInWeek(t('2026-11-22T10:00:00', '2026-11-25T10:00:00'), mo), isTrue);
    expect(assemblyInWeek(t('2026-11-01', '2026-12-01'), mo), isTrue);
    expect(assemblyInWeek(t('2026-11-10', '2026-11-15'), mo), isFalse);
    expect(assemblyInWeek(t('2026-11-23', '2026-11-23'), mo), isFalse);
    expect(assemblyInWeek(t('2026-11-16', null), mo), isFalse);
  });

  test('Titel, Tooltip und Zeile des Teams', () {
    expect(assemblyWeekTitle(_a1), 'FA-2026-0001 · P-7 Hafen');
    expect(assemblyWeekTitle(_a3), 'P-9 Halle');
    expect(assemblyWeekTitle({'order_nummer': 'FA-1', 'project_nummer': 'P-7', 'project_name': ''}), 'FA-1 · P-7');
    expect(assemblyWeekTitle({}), '');
    expect(assemblyWeekTooltip(_a1),
        '13.11.2026 – 17.11.2026 (5 Tage)\nFahrzeug: MA-1\nBaustelle: Hafenstr. 2\nHinweis: Team ist im Zeitraum doppelt eingeplant');
    expect(assemblyWeekTooltip(_a2), '16.11.2026 (1 Tag)');
    final teams = _weeks['teams'] as List;
    expect(assemblyTeamLine(teams[0]), '2 Termine');
    expect(assemblyTeamLine(teams[1]), 'keine Termine');
    expect(assemblyTeamLine(teams[2]), '1 Termin');
    expect(assemblyTeamLine({}), 'keine Termine');
  });

  testWidgets('Tabelle Team x Woche, Termin über zwei Wochen, Hinweise, ohne Team zuletzt', (tester) async {
    await _desktop(tester);
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(ProductionAssemblyWeeksPage(api: api, today: _today)));
    await tester.pumpAndSettle();
    expect(api.calls, ['2026-11-09 2026-12-28']);
    expect(find.text('KW 46\n09.11.2026'), findsOneWidget);
    expect(find.text('KW 47\n16.11.2026'), findsOneWidget);
    expect(find.text('KW 48\n23.11.2026'), findsNothing);
    expect(find.text('FA-2026-0001 · P-7 Hafen'), findsNWidgets(2));
    expect(find.text('13.11.2026 – 17.11.2026 (5 Tage)'), findsNWidgets(2));
    expect(find.text('FA-2026-0002 · P-8 Schule'), findsOneWidget);
    expect(find.text('P-9 Halle'), findsOneWidget);
    expect(find.text('2 Termine'), findsOneWidget);
    expect(find.text('keine Termine'), findsOneWidget);
    expect(find.text('1 Termin'), findsOneWidget);
    // Süd in beiden Wochen leer, ohne Team in KW 46
    expect(find.text('–'), findsNWidgets(3));
    expect(find.byIcon(Icons.warning_amber_rounded), findsNWidgets(2));
    expect(find.byTooltip(assemblyWeekTooltip(_a1)), findsNWidgets(2));
    final nord = tester.getTopLeft(find.text('Montage Nord')).dy;
    final sued = tester.getTopLeft(find.text('Montage Süd')).dy;
    final ohne = tester.getTopLeft(find.text('ohne Team')).dy;
    expect(nord < sued && sued < ohne, isTrue);
    // ohne Hinweis kein Symbol in der Zeile von Süd und ohne Team
    final row = find.ancestor(of: find.text('P-9 Halle'), matching: find.byType(Row)).first;
    expect(find.descendant(of: row, matching: find.byIcon(Icons.warning_amber_rounded)), findsNothing);
  });

  testWidgets('Tipp öffnet den Auftrag und lädt danach neu, ohne Auftrag nichts', (tester) async {
    await _desktop(tester);
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(ProductionAssemblyWeeksPage(api: api, today: _today)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('P-9 Halle'));
    await tester.pumpAndSettle();
    expect(find.byType(ProductionOrderDetailPage), findsNothing);
    expect(api.calls, hasLength(1));
    await tester.tap(find.text('FA-2026-0002 · P-8 Schule'));
    await tester.pumpAndSettle();
    final detail = tester.widget<ProductionOrderDetailPage>(find.byType(ProductionOrderDetailPage));
    expect(detail.orderId, 'o2');
    Navigator.of(tester.element(find.byType(ProductionOrderDetailPage))).pop();
    await tester.pumpAndSettle();
    expect(find.byType(ProductionOrderDetailPage), findsNothing);
    expect(api.calls, hasLength(2));
  });

  testWidgets('Blättern um acht Wochen und zurück zur aktuellen Woche', (tester) async {
    await _desktop(tester);
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(ProductionAssemblyWeeksPage(api: api, today: _today)));
    await tester.pumpAndSettle();
    expect(find.text('09.11.2026 bis 28.12.2026 (Wochen ab Montag)'), findsOneWidget);
    await tester.tap(find.byTooltip('Spätere Wochen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, '2027-01-04 2027-02-22');
    await tester.tap(find.byTooltip('Frühere Wochen'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Frühere Wochen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, '2026-09-14 2026-11-02');
    await tester.tap(find.text('Aktuelle Woche'));
    await tester.pumpAndSettle();
    expect(api.calls.last, '2026-11-09 2026-12-28');
    await tester.tap(find.byTooltip('Neu laden'));
    await tester.pumpAndSettle();
    expect(api.calls, hasLength(6));
  });

  testWidgets('Ohne Teams ein Hinweis', (tester) async {
    await _desktop(tester);
    final gate = Completer<void>();
    final api = _FakeApi(empty: true, gate: gate);
    await tester.pumpWidget(_wrap(ProductionAssemblyWeeksPage(api: api, today: _today)));
    await tester.pump();
    // während des Ladens noch kein Hinweis
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
    expect(find.textContaining('Noch kein Team'), findsNothing);
    gate.complete();
    await tester.pumpAndSettle();
    expect(find.byType(LinearProgressIndicator), findsNothing);
    expect(find.textContaining('Noch kein Team'), findsOneWidget);
    expect(find.byType(DataTable), findsNothing);
    // beim Neuladen verschwindet der Hinweis bis zur Antwort
    final again = api.gate = Completer<void>();
    await tester.tap(find.byTooltip('Neu laden'));
    await tester.pump();
    expect(find.textContaining('Noch kein Team'), findsNothing);
    again.complete();
    await tester.pumpAndSettle();
    expect(find.textContaining('Noch kein Team'), findsOneWidget);
  });

  testWidgets('Fehler als Meldung, keine Tabelle', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(ProductionAssemblyWeeksPage(api: _FakeApi(fail: 'Zeitraum ist zu groß'), today: _today)));
    await tester.pumpAndSettle();
    expect(find.text('Montagetermine konnten nicht geladen werden: Zeitraum ist zu groß'), findsOneWidget);
    expect(find.textContaining('Noch kein Team'), findsNothing);
    expect(find.byType(DataTable), findsNothing);
  });

  testWidgets('Dashboard: Karte nur mit production.read', (tester) async {
    await _desktop(tester);
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'production.write'})));
    await tester.pumpAndSettle();
    expect(find.text('Montageplanung'), findsNothing);
    await tester.pumpWidget(_wrap(
        DashboardPage(key: const ValueKey('mit'), api: api, onLogout: () async {}, permissions: const {'production.read'}),
        key: const ValueKey('mit')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Montageplanung'));
    await tester.pumpAndSettle();
    expect(find.byType(ProductionAssemblyWeeksPage), findsOneWidget);
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

    test('Pfad und Parameter', () async {
      final out = await mock(_weeks, () => api.getProductionAssemblyWeeks(von: '2026-11-09', bis: '2026-12-28'));
      expect(out['teams'], hasLength(3));
      await mock(_weeks, () => api.getProductionAssemblyWeeks());
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/production/montage?von=2026-11-09&bis=2026-12-28',
        'GET /api/v1/production/montage?',
      ]);
    });

    test('Fehler als ApiException', () async {
      await expectLater(
          mock({
            'error': {'code': 'validation_error', 'message': 'Zeitraum ist zu groß: höchstens 26 Wochen'}
          }, () => api.getProductionAssemblyWeeks(von: '2026-01-05', bis: '2026-12-28'), status: 400),
          throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Zeitraum ist zu groß: höchstens 26 Wochen')));
    });
  });
}
