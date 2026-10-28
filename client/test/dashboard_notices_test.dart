import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/qualifications_due_page.dart';
import 'package:nalaerp_client/widgets/dashboard_notices.dart';

// Hinweise auf dem Dashboard (Backlog F.10.4.2).

class _FakeApi extends ApiClient {
  _FakeApi({this.notices = const [], this.error}) : super(baseUrl: 'http://localhost:8080');

  final List<dynamic> notices;
  final Object? error;
  int calls = 0;

  @override
  bool hasPermission(String permission) => true;

  @override
  Future<List<dynamic>> listNotices() async {
    calls++;
    if (error != null) throw error!;
    return notices;
  }

  @override
  Future<List<dynamic>> listQualificationsDue({int? warnDays}) async => const [];
}

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: child,
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 900));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

void main() {
  test('noticeLine: Staende in fester Reihenfolge, Nullen weggelassen, Unbekanntes am Ende', () {
    expect(
        noticeLine({
          'title': 'Fällige Qualifikationen',
          'total': 3,
          'counts': {'laeuft_ab': 2, 'fehlt': 1, 'abgelaufen': 0},
        }),
        'Fällige Qualifikationen: 3 (1 fehlt, 2 läuft ab)');
    expect(
        noticeLine({
          'title': 'Fällige Prüfungen der Betriebsmittel',
          'total': 4,
          'counts': {'faellig_bald': 1, 'nie_geprueft': 1, 'ueberfaellig': 1, 'gesperrt': 1},
        }),
        'Fällige Prüfungen der Betriebsmittel: 4 (1 gesperrt, 1 überfällig, 1 nie geprüft, 1 bald fällig)');
    expect(noticeLine({'title': 'X', 'total': 1, 'counts': {'neu': 1}}), 'X: 1 (1 neu)');
    expect(noticeLine({'kind': 'x', 'total': 0}), 'x: 0');
  });

  test('noticeDestination: Listen je Art, eigene Qualifikationen ohne Seite', () {
    final api = _FakeApi();
    expect(noticeDestination('qualifications_due', api), isA<QualificationsDuePage>());
    expect(noticeDestination('asset_inspections_due', api).runtimeType.toString(), 'AssetInspectionsDuePage');
    expect(noticeDestination('fleet_schedules_due', api).runtimeType.toString(), 'FleetDuePage');
    expect(noticeDestination('own_qualifications', api), isNull);
    expect(noticeDestination('unbekannt', api), isNull);
  });

  testWidgets('Dashboard zeigt die Hinweise; Klick öffnet die Liste, eigene ohne Ziel', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(notices: [
      {'kind': 'qualifications_due', 'title': 'Fällige Qualifikationen', 'severity': 'kritisch', 'total': 2, 'counts': {'fehlt': 1, 'abgelaufen': 1}},
      {'kind': 'own_qualifications', 'title': 'Meine Qualifikationen', 'severity': 'hinweis', 'total': 1, 'counts': {'laeuft_ab': 1}},
    ]);
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {})));
    await tester.pumpAndSettle();
    expect(api.calls, 1);
    expect(find.text('Hinweise'), findsOneWidget);
    expect(find.text('Fällige Qualifikationen: 2 (1 fehlt, 1 abgelaufen)'), findsOneWidget);
    expect(find.text('Meine Qualifikationen: 1 (1 läuft ab)'), findsOneWidget);
    // Die Farbe folgt der Dringlichkeit.
    expect(tester.widget<Icon>(find.byIcon(Icons.error_outline)).color, const Color(0xFFB00020));
    expect(tester.widget<Icon>(find.byIcon(Icons.schedule)).color, const Color(0xFFB26A00));

    // Ohne eigene Seite ist die Zeile nicht anklickbar.
    expect(tester.widget<ListTile>(find.widgetWithText(ListTile, 'Meine Qualifikationen: 1 (1 läuft ab)')).onTap, isNull);
    await tester.tap(find.text('Meine Qualifikationen: 1 (1 läuft ab)'));
    await tester.pumpAndSettle();
    expect(find.byType(QualificationsDuePage), findsNothing);

    await tester.tap(find.text('Fällige Qualifikationen: 2 (1 fehlt, 1 abgelaufen)'));
    await tester.pumpAndSettle();
    expect(find.byType(QualificationsDuePage), findsOneWidget);
  });

  testWidgets('ohne Hinweise erscheint nichts, bei Fehler ein kurzer Satz', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(Scaffold(body: DashboardNotices(api: _FakeApi()))));
    await tester.pumpAndSettle();
    expect(find.text('Hinweise'), findsNothing);
    expect(find.text('Hinweise aktuell nicht verfügbar'), findsNothing);

    await tester.pumpWidget(_wrap(Scaffold(
        body: DashboardNotices(key: const ValueKey('fehler'), api: _FakeApi(error: const ApiException(statusCode: 500, message: 'Interner Fehler'))))));
    await tester.pumpAndSettle();
    expect(find.text('Hinweise aktuell nicht verfügbar'), findsOneWidget);
    expect(find.text('Hinweise'), findsNothing);
  });

  test('API: GET /api/v1/notices, Fehler werden zur ApiException', () async {
    final calls = <http.Request>[];
    final api = ApiClient(baseUrl: 'http://api.test');
    Future<T> mock<T>(Object body, Future<T> Function() call, {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(utf8.encode(jsonEncode(body)), status, headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    final out = await mock([
      {'kind': 'fleet_schedules_due', 'total': 1}
    ], () => api.listNotices());
    expect(calls.single.method, 'GET');
    expect(calls.single.url.path, '/api/v1/notices');
    expect(out.single['kind'], 'fleet_schedules_due');
    await expectLater(
      mock({'error': {'code': 'internal_error', 'message': 'Interner Fehler'}}, () => api.listNotices(), status: 500),
      throwsA(isA<ApiException>()),
    );
  });
}
