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
  _FakeApi({this.list = const [], this.fail}) : super(baseUrl: 'http://localhost:8080');

  final List<dynamic> list;
  final String? fail;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permission == 'production.read';

  @override
  Future<List<dynamic>> listProductionAssembly(String orderId) async {
    calls.add('list $orderId');
    if (fail != null) throw ApiException(statusCode: 404, message: fail!);
    return list;
  }
}

Future<void> _pump(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
    home: Scaffold(body: SingleChildScrollView(child: ProductionAssemblySection(api: api, orderId: 'o1', status: 'freigegeben'))),
  ));
  await tester.pumpAndSettle();
}

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
