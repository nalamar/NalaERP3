import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/qualifications_due_page.dart';

// Faellige Qualifikationen (Backlog F.13.2). InkRipple und Desktop-Fenster
// wie in den anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({this.entries = const [], this.error}) : super(baseUrl: 'http://localhost:8080');
  final List<dynamic> entries;
  final Object? error;
  final List<int?> calls = [];

  @override
  Future<List<dynamic>> listQualificationsDue({int? warnDays}) async {
    calls.add(warnDays);
    if (error != null) throw error!;
    return entries;
  }
}

final _entries = [
  {'employee_id': 'e1', 'personalnummer': 'P-17', 'first_name': 'Anna', 'last_name': 'Berg', 'code': 'ERSTHELFER', 'name': 'Erste Hilfe', 'status': 'fehlt', 'erforderlich': true},
  {'employee_id': 'e2', 'first_name': 'Bert', 'last_name': 'Cord', 'code': 'STAPLER_UNTERWEISUNG', 'name': 'Stapler – Unterweisung', 'status': 'abgelaufen', 'gueltig_bis': '2026-09-30T00:00:00Z', 'tage_bis_ablauf': -19},
  {'employee_id': 'e1', 'personalnummer': 'P-17', 'first_name': 'Anna', 'last_name': 'Berg', 'code': 'ARBEITSSCHUTZ', 'name': 'Arbeitsschutz', 'status': 'laeuft_ab', 'gueltig_bis': '2026-11-30T00:00:00Z', 'tage_bis_ablauf': 42},
];

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

void main() {
  test('dueCounts und dueEmployeeLabel', () {
    expect(dueCounts(_entries), {'fehlt': 1, 'abgelaufen': 1, 'laeuft_ab': 1});
    // Mehrfach vorkommende Staende werden gezaehlt, nicht nur markiert.
    expect(dueCounts([..._entries, {'status': 'fehlt'}, {'status': 'fehlt'}]), {'fehlt': 3, 'abgelaufen': 1, 'laeuft_ab': 1});
    expect(dueCounts(const []), {'fehlt': 0, 'abgelaufen': 0, 'laeuft_ab': 0});
    expect(dueEmployeeLabel(_entries[0]), 'Berg, Anna (P-17)');
    expect(dueEmployeeLabel(_entries[1]), 'Cord, Bert');
  });

  testWidgets('zeigt die Liste in Reihenfolge des Servers mit Zaehlern und Statustexten', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(entries: _entries);
    await tester.pumpWidget(_wrap(QualificationsDuePage(api: api)));
    await tester.pumpAndSettle();
    expect(api.calls, [60]);
    expect(find.text('fehlt: 1 · abgelaufen: 1 · läuft ab: 1'), findsOneWidget);
    final titles = [
      'Berg, Anna (P-17) – Erste Hilfe',
      'Cord, Bert – Stapler – Unterweisung',
      'Berg, Anna (P-17) – Arbeitsschutz',
    ];
    for (final t in titles) {
      expect(find.text(t), findsOneWidget);
    }
    // Reihenfolge wie geliefert.
    final ys = titles.map((t) => tester.getTopLeft(find.text(t)).dy).toList();
    expect(ys[0], lessThan(ys[1]));
    expect(ys[1], lessThan(ys[2]));
    expect(find.text('fehlt'), findsOneWidget);
    expect(find.text('abgelaufen seit 30.09.2026'), findsOneWidget);
    expect(find.text('läuft ab am 30.11.2026 (in 42 Tagen)'), findsOneWidget);
  });

  testWidgets('Vorwarnzeit aendern; ungueltige Eingabe abgelehnt', (tester) async {
    await _desktop(tester);
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(QualificationsDuePage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('Nichts fällig – alle erforderlichen Qualifikationen gelten länger als 60 Tage.'), findsOneWidget);

    await tester.enterText(find.widgetWithText(TextField, 'Vorwarnzeit (Tage)'), '-1');
    await tester.tap(find.text('Anwenden'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Vorwarnzeit als ganze Zahl von Tagen angeben (0 oder mehr).'), findsOneWidget);
    expect(api.calls, [60]);

    await tester.enterText(find.widgetWithText(TextField, 'Vorwarnzeit (Tage)'), ' 14 ');
    await tester.tap(find.text('Anwenden'));
    await tester.pumpAndSettle();
    expect(api.calls, [60, 14]);
    expect(find.text('Nichts fällig – alle erforderlichen Qualifikationen gelten länger als 14 Tage.'), findsOneWidget);
  });

  testWidgets('Ladefehler', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(QualificationsDuePage(api: _FakeApi(error: const ApiException(statusCode: 403, message: 'Keine Berechtigung')))));
    await tester.pumpAndSettle();
    expect(find.text('Liste konnte nicht geladen werden: Keine Berechtigung'), findsOneWidget);
    expect(find.textContaining('Nichts fällig'), findsNothing);
    expect(find.textContaining('fehlt:'), findsNothing);
  });

  testWidgets('Dashboard: Karte nur mit qualifications.read', (tester) async {
    final api = _FakeApi();
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'qualifications.self'})));
    await tester.pumpAndSettle();
    expect(find.text('Fällige Qualifikationen'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'qualifications.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fällige Qualifikationen'));
    await tester.pumpAndSettle();
    expect(find.byType(QualificationsDuePage), findsOneWidget);
  });

  test('API: Pfad und Vorwarnzeit', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <Uri>[];
    final client = MockClient((req) async {
      calls.add(req.url);
      return http.Response.bytes(utf8.encode('[]'), 200, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.listQualificationsDue();
      await api.listQualificationsDue(warnDays: 30);
    }, () => client);
    expect(calls.map((u) => '${u.path}?${u.query}'), ['/api/v1/qualifications/due?', '/api/v1/qualifications/due?warn_tage=30']);
  });
}
