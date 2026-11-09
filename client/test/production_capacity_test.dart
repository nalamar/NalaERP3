import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/production_capacity_page.dart';

// Auslastung je Arbeitsplatz und Woche im Client (Backlog H.2.5c).

final _capacity = {
  'von': '2026-11-09T00:00:00Z',
  'bis': '2026-11-16T00:00:00Z',
  'arbeitsplaetze': [
    {
      'work_center_id': 'wc1',
      'name': 'Zuschnitt',
      'stunden_je_tag': 8,
      'aktiv': true,
      'ungeplant_min': 90,
      'wochen': [
        {
          'woche': '2026-11-09T00:00:00Z',
          'arbeitstage': 5,
          'verfuegbar_min': 2400,
          'geplant_min': 600,
          'hinweise': []
        },
        {
          'woche': '2026-11-16T00:00:00Z',
          'arbeitstage': 2,
          'verfuegbar_min': 960,
          'geplant_min': 1500,
          'hinweise': ['ueberlast']
        },
      ],
    },
    {
      'work_center_id': 'wc2',
      'name': 'Montage',
      'stunden_je_tag': 7.5,
      'aktiv': false,
      'ungeplant_min': 0,
      'wochen': [
        {
          'woche': '2026-11-09T00:00:00Z',
          'arbeitstage': 0,
          'verfuegbar_min': 0,
          'geplant_min': 0,
          'hinweise': []
        },
        {
          'woche': '2026-11-16T00:00:00Z',
          'arbeitstage': 2,
          'verfuegbar_min': 900,
          'geplant_min': 60,
          'hinweise': []
        },
      ],
    },
  ],
};

class _FakeApi extends ApiClient {
  _FakeApi({this.empty = false, this.fail, this.gate})
      : super(baseUrl: 'http://localhost:8080');

  final bool empty;
  final String? fail;
  // haelt die Antwort zurueck, bis der Test sie freigibt
  final Completer<void>? gate;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permission == 'production.read';

  @override
  Future<Map<String, dynamic>> getProductionCapacity(
      {String? von, String? bis}) async {
    calls.add('$von $bis');
    await gate?.future;
    if (fail != null) throw ApiException(statusCode: 400, message: fail!);
    if (empty) return {'arbeitsplaetze': []};
    return _capacity;
  }
}

Widget _wrap(Widget child, {Key? key}) => MaterialApp(
    key: key,
    theme:
        ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
    home: child);

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

// Mittwoch; Montag der Woche ist der 09.11.2026.
final _today = DateTime(2026, 11, 11, 15, 30);

void main() {
  test('Montag, Zeitraum und Kopf der Woche', () {
    expect(mondayOf(_today), DateTime(2026, 11, 9));
    expect(mondayOf(DateTime(2026, 11, 15)), DateTime(2026, 11, 9));
    expect(mondayOf(DateTime(2026, 11, 9)), DateTime(2026, 11, 9));
    expect(mondayOf(DateTime(2027, 1, 1)), DateTime(2026, 12, 28));
    expect(isoDay(DateTime(2027, 3, 4)), '2027-03-04');
    // über die Zeitumstellung (25.10.2026, 28.03.2027) in Kalendertagen
    expect(addDays(DateTime(2026, 10, 19), 7), DateTime(2026, 10, 26));
    expect(addDays(DateTime(2027, 4, 5), -14), DateTime(2027, 3, 22));
    expect(capacityQuery(DateTime(2026, 10, 21)),
        {'von': '2026-10-19', 'bis': '2026-12-07'});
    expect(capacityQuery(_today), {'von': '2026-11-09', 'bis': '2026-12-28'});
    expect(capacityQuery(DateTime(2026, 12, 31)),
        {'von': '2026-12-28', 'bis': '2027-02-15'});
    expect(capacityWeekHeader('2026-11-09T00:00:00Z'), 'KW 46\n09.11.2026');
    expect(capacityWeekHeader('2026-12-28T00:00:00Z'), 'KW 53\n28.12.2026');
    expect(capacityWeekHeader('2027-01-04'), 'KW 1\n04.01.2027');
    expect(capacityWeekHeader('kaputt'), 'kaputt');
  });

  test('Zellen, Überlast und Zeile des Arbeitsplatzes', () {
    final rows = _capacity['arbeitsplaetze'] as List;
    final zuschnitt = rows[0] as Map<String, dynamic>;
    final montage = rows[1] as Map<String, dynamic>;
    final w = zuschnitt['wochen'] as List;
    expect(capacityCell(w[0]), '10 h von 40 h');
    expect(capacityCell(w[1]), '25 h von 16 h');
    expect(capacityOverloaded(w[0]), isFalse);
    expect(capacityOverloaded(w[1]), isTrue);
    expect(capacityOverloaded({'geplant_min': 5}), isFalse);
    expect(
        capacityOverloaded({
          'hinweise': ['sonstiges']
        }),
        isFalse);
    expect(capacityCell((montage['wochen'] as List)[0]), 'frei');
    // geplant ohne Arbeitstage ist nicht "frei"
    expect(capacityCell({'verfuegbar_min': 0, 'geplant_min': 60}),
        '1 h von 0 min');
    expect(
        capacityRowLine(zuschnitt), '8 h je Arbeitstag · ungeplant 1 h 30 min');
    expect(capacityRowLine(montage), '7,5 h je Arbeitstag · deaktiviert');
  });

  testWidgets('Tabelle mit Überlast-Hinweis nur in der überlasteten Woche',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi();
    await tester
        .pumpWidget(_wrap(ProductionCapacityPage(api: api, today: _today)));
    await tester.pumpAndSettle();
    expect(api.calls, ['2026-11-09 2026-12-28']);
    expect(find.text('KW 46\n09.11.2026'), findsOneWidget);
    expect(find.text('KW 47\n16.11.2026'), findsOneWidget);
    expect(find.text('Zuschnitt'), findsOneWidget);
    expect(
        find.text('8 h je Arbeitstag · ungeplant 1 h 30 min'), findsOneWidget);
    expect(find.text('7,5 h je Arbeitstag · deaktiviert'), findsOneWidget);
    expect(find.text('10 h von 40 h'), findsOneWidget);
    expect(find.text('frei'), findsOneWidget);
    expect(find.byIcon(Icons.warning_amber_rounded), findsOneWidget);
    expect(
        find.byTooltip('Überlast: mehr geplant als verfügbar'), findsOneWidget);
    final over = tester.widget<Text>(find.text('25 h von 16 h'));
    final normal = tester.widget<Text>(find.text('10 h von 40 h'));
    expect(over.style?.fontWeight, FontWeight.bold);
    expect(normal.style, isNull);
  });

  testWidgets('Blättern um acht Wochen und zurück zur aktuellen Woche',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi();
    await tester
        .pumpWidget(_wrap(ProductionCapacityPage(api: api, today: _today)));
    await tester.pumpAndSettle();
    expect(find.text('09.11.2026 bis 28.12.2026 (Wochen ab Montag)'),
        findsOneWidget);
    await tester.tap(find.byTooltip('Spätere Wochen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, '2027-01-04 2027-02-22');
    await tester.tap(find.byTooltip('Frühere Wochen'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Frühere Wochen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, '2026-09-14 2026-11-02');
    expect(find.text('14.09.2026 bis 02.11.2026 (Wochen ab Montag)'),
        findsOneWidget);
    await tester.tap(find.text('Aktuelle Woche'));
    await tester.pumpAndSettle();
    expect(api.calls.last, '2026-11-09 2026-12-28');
    expect(api.calls, hasLength(5));
  });

  testWidgets('Ohne Arbeitsplätze ein Hinweis, Fehler als Meldung',
      (tester) async {
    await _desktop(tester);
    final gate = Completer<void>();
    await tester.pumpWidget(_wrap(ProductionCapacityPage(
        api: _FakeApi(empty: true, gate: gate), today: _today)));
    await tester.pump();
    // während des Ladens noch kein Hinweis
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
    expect(find.textContaining('Noch kein aktiver Arbeitsplatz'), findsNothing);
    gate.complete();
    await tester.pumpAndSettle();
    expect(find.byType(LinearProgressIndicator), findsNothing);
    expect(
        find.textContaining('Noch kein aktiver Arbeitsplatz'), findsOneWidget);
    expect(find.byType(DataTable), findsNothing);
    await tester.pumpWidget(_wrap(
        ProductionCapacityPage(
            api: _FakeApi(fail: 'Zeitraum ist zu groß'), today: _today),
        key: const ValueKey('fehler')));
    await tester.pumpAndSettle();
    expect(find.textContaining('Auslastung konnte nicht geladen werden'),
        findsOneWidget);
    expect(find.textContaining('Zeitraum ist zu groß'), findsOneWidget);
    expect(find.textContaining('Noch kein aktiver Arbeitsplatz'), findsNothing);
    expect(find.byType(DataTable), findsNothing);
  });

  testWidgets('Dashboard: Karte nur mit production.read', (tester) async {
    await _desktop(tester);
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(DashboardPage(
        api: api,
        onLogout: () async {},
        permissions: const {'production.write'})));
    await tester.pumpAndSettle();
    expect(find.text('Auslastung'), findsNothing);
    await tester.pumpWidget(_wrap(
        DashboardPage(
            key: const ValueKey('mit'),
            api: api,
            onLogout: () async {},
            permissions: const {'production.read'}),
        key: const ValueKey('mit')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Auslastung'));
    await tester.pumpAndSettle();
    expect(find.byType(ProductionCapacityPage), findsOneWidget);
  });

  group('API-Methode', () {
    final api = ApiClient(baseUrl: 'http://api.test');
    late List<http.Request> calls;

    Future<T> mock<T>(Object body, Future<T> Function() call,
        {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(utf8.encode(jsonEncode(body)), status,
            headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    setUp(() => calls = []);

    test('Pfad und Parameter', () async {
      final out = await mock(
          _capacity,
          () =>
              api.getProductionCapacity(von: '2026-11-09', bis: '2026-12-28'));
      expect(out['arbeitsplaetze'], hasLength(2));
      await mock(_capacity, () => api.getProductionCapacity());
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/production/capacity?von=2026-11-09&bis=2026-12-28',
        'GET /api/v1/production/capacity?',
      ]);
    });

    test('Fehler als ApiException', () async {
      await expectLater(
          mock(
              {
                'error': {
                  'code': 'validation_error',
                  'message': 'Ungültiger Zeitraum: bis liegt vor von'
                }
              },
              () => api.getProductionCapacity(
                  von: '2026-12-01', bis: '2026-11-01'),
              status: 400),
          throwsA(isA<ApiException>().having((e) => e.message, 'message',
              'Ungültiger Zeitraum: bis liegt vor von')));
    });
  });
}
