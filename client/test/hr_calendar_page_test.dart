import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/hr_calendar_page.dart';

// Feiertage und Betriebsruhe (Backlog F.8.6.4.1). Feste Uhr: 20.10.2026.

final _now = DateTime(2026, 10, 20, 9, 30);

class _FakeApi extends ApiClient {
  _FakeApi({this.perms = const {'settings.manage', 'leave_requests.decide'}, this.land = 'BY'}) : super(baseUrl: 'http://localhost:8080');

  final Set<String> perms;
  final String land;
  final List<String> calls = [];
  final List<Map<String, dynamic>> savedSettings = [];

  @override
  bool hasPermission(String code) => perms.contains(code);

  @override
  Future<Map<String, dynamic>> getHrCalendarSettings() async {
    calls.add('settings');
    return {'bundesland': land, 'mariae_himmelfahrt': true, 'augsburger_friedensfest': false};
  }

  @override
  Future<Map<String, dynamic>> setHrCalendarSettings(Map<String, dynamic> body) async {
    calls.add('saveSettings');
    savedSettings.add(body);
    return body;
  }

  @override
  Future<List<dynamic>> listCompanyDays({int? year}) async {
    calls.add('days $year');
    return [
      {'id': 'd1', 'datum': '$year-12-24T00:00:00Z', 'name': 'Heiligabend'},
    ];
  }

  @override
  Future<Map<String, dynamic>> createCompanyDay(String datum, String name) async {
    calls.add('create $datum $name');
    return {'id': 'neu'};
  }

  @override
  Future<void> deleteCompanyDay(String id) async {
    calls.add('delete $id');
  }

  @override
  Future<Map<String, dynamic>> getHolidays({int? year}) async {
    calls.add('holidays $year');
    return {
      'year': year,
      'bundesland': land,
      'days': [
        {'date': '$year-01-01T00:00:00Z', 'name': 'Neujahr', 'art': 'gesetzlich'},
        {'date': '$year-12-24T00:00:00Z', 'name': 'Heiligabend', 'art': 'betrieblich'},
      ],
    };
  }
}

Future<void> _pump(WidgetTester tester, ApiClient api) async {
  tester.view.physicalSize = const Size(1400, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(MaterialApp(
    theme: ThemeData(splashFactory: InkRipple.splashFactory),
    home: HrCalendarPage(api: api, now: () => _now),
  ));
  await tester.pumpAndSettle();
}

Finder _inDialog(Finder f) => find.descendant(of: find.byType(AlertDialog), matching: f);

void main() {
  test('Laender, Zeilen, Eingaben', () {
    final codes = sortedBundeslaender();
    expect(codes.length, 16);
    expect(codes.first, 'BW');
    expect(codes.last, 'TH');
    expect(codes.indexOf('NW') < codes.indexOf('RP'), isTrue);
    expect(calendarDayLine({'date': '2026-10-03T00:00:00Z', 'name': 'Tag der Deutschen Einheit', 'art': 'gesetzlich'}),
        '03.10.2026 Tag der Deutschen Einheit');
    expect(calendarDayLine({'date': '2026-12-24', 'name': 'Heiligabend', 'art': 'betrieblich'}), '24.12.2026 Heiligabend (betrieblich)');
    expect(buildCompanyDay('24.12.2026', ' Heiligabend '), (datum: '2026-12-24', name: 'Heiligabend', error: null));
    expect(buildCompanyDay('31.02.2026', 'x').error, 'Bitte das Datum als TT.MM.JJJJ angeben.');
    expect(buildCompanyDay('24.12.2026', '  ').error, 'Bitte eine Bezeichnung angeben.');
  });

  testWidgets('Bundesland mit Schaltern; Wechsel speichert ohne Schalter', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    expect(api.calls, ['settings', 'days 2026', 'holidays 2026']);
    expect(find.text('Bayern'), findsOneWidget);
    expect(find.textContaining('Kein Bundesland eingestellt'), findsNothing);
    final mariae = find.widgetWithText(CheckboxListTile, 'Mariä Himmelfahrt (15.08.) - nur in überwiegend katholischen Gemeinden');
    expect(tester.widget<CheckboxListTile>(mariae).value, isTrue);
    await tester.tap(find.text('Augsburger Friedensfest (08.08.) - nur in Augsburg'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.savedSettings.last, {'bundesland': 'BY', 'mariae_himmelfahrt': true, 'augsburger_friedensfest': true});
    expect(find.text('Feiertagskalender gespeichert'), findsOneWidget);
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    // Wechsel nach NRW: Schalter verschwinden und werden nicht gespeichert.
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Nordrhein-Westfalen').last);
    await tester.pumpAndSettle();
    expect(find.textContaining('Mariä Himmelfahrt'), findsNothing);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.savedSettings.last, {'bundesland': 'NW', 'mariae_himmelfahrt': false, 'augsburger_friedensfest': false});
  });

  testWidgets('Betriebliche Tage eintragen und entfernen; Vorschau', (tester) async {
    final api = _FakeApi(land: '');
    await _pump(tester, api);
    expect(find.text('24.12.2026 Heiligabend'), findsOneWidget);
    expect(find.text('24.12.2026 Heiligabend (betrieblich)'), findsOneWidget);
    expect(find.text('01.01.2026 Neujahr'), findsOneWidget);
    expect(find.text('Kein Bundesland eingestellt - nur bundesweite Feiertage.'), findsOneWidget);
    await tester.tap(find.text('Freien Tag eintragen'));
    await tester.pumpAndSettle();
    final fields = _inDialog(find.byType(TextField));
    await tester.enterText(fields.at(0), '31.12.2026');
    await tester.tap(_inDialog(find.text('Eintragen')));
    await tester.pumpAndSettle();
    expect(find.text('Bitte eine Bezeichnung angeben.'), findsOneWidget);
    await tester.enterText(fields.at(1), ' Silvester ');
    await tester.tap(_inDialog(find.text('Eintragen')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('create 2026-12-31 Silvester'));
    // Nach dem Eintragen neu geladen.
    expect(api.calls.where((c) => c == 'days 2026').length, 2);
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Entfernen'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Abbrechen')));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('delete')), isEmpty);
    await tester.tap(find.byTooltip('Entfernen'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Entfernen')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('delete d1'));
  });

  testWidgets('Rechte: nur Betriebsruhe bzw. nur Bundesland', (tester) async {
    final decideOnly = _FakeApi(perms: const {'leave_requests.decide'});
    await _pump(tester, decideOnly);
    expect(decideOnly.calls.contains('settings'), isFalse);
    expect(find.byType(DropdownButtonFormField<String>), findsNothing);
    expect(find.text('Freien Tag eintragen'), findsOneWidget);
    final settingsOnly = _FakeApi(perms: const {'settings.manage'});
    await tester.pumpWidget(const SizedBox());
    await _pump(tester, settingsOnly);
    expect(settingsOnly.calls.where((c) => c.startsWith('days')), isEmpty);
    expect(find.text('Freien Tag eintragen'), findsNothing);
    expect(find.byType(DropdownButtonFormField<String>), findsOneWidget);
  });

  testWidgets('Jahr wechseln', (tester) async {
    final api = _FakeApi();
    await _pump(tester, api);
    await tester.tap(find.byTooltip('Vorjahr'));
    await tester.pumpAndSettle();
    expect(api.calls, containsAll(['days 2025', 'holidays 2025']));
    expect(find.text('Alle freien Tage 2025'), findsOneWidget);
  });

  test('API: Kalender-Einstellung und betriebliche Tage', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final status = req.method == 'POST' ? 201 : (req.method == 'DELETE' ? 204 : 200);
      final body = req.method == 'DELETE' ? '' : (req.url.path.endsWith('company-days') && req.method == 'GET' ? '[]' : '{}');
      return http.Response.bytes(utf8.encode(body), status, headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      await api.getHrCalendarSettings();
      await api.setHrCalendarSettings({'bundesland': 'NW'});
      await api.listCompanyDays(year: 2026);
      await api.createCompanyDay('2026-12-24', 'Heiligabend');
      await api.deleteCompanyDay('d1');
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
      'GET /api/v1/settings/hr-calendar/?',
      'PUT /api/v1/settings/hr-calendar/?',
      'GET /api/v1/holidays/company-days?year=2026',
      'POST /api/v1/holidays/company-days?',
      'DELETE /api/v1/holidays/company-days/d1?',
    ]);
    expect(jsonDecode(calls[1].body), {'bundesland': 'NW'});
    expect(jsonDecode(calls[3].body), {'datum': '2026-12-24', 'name': 'Heiligabend'});
    final conflict = MockClient((req) async => http.Response('{"error":{"code":"conflict","message":"x"}}', 409, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.createCompanyDay('2026-12-24', 'x'), () => conflict), throwsA(isA<ApiException>()));
    await expectLater(http.runWithClient(() => api.deleteCompanyDay('d1'), () => conflict), throwsA(isA<ApiException>()));
    await expectLater(http.runWithClient(() => api.setHrCalendarSettings({}), () => conflict), throwsA(isA<ApiException>()));
    // Anlegen erwartet 201, Loeschen 204 - 200 gilt jeweils als Fehler.
    final ok200 = MockClient((req) async => http.Response('{}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.createCompanyDay('2026-12-24', 'x'), () => ok200), throwsA(isA<ApiException>()));
    await expectLater(http.runWithClient(() => api.deleteCompanyDay('d1'), () => ok200), throwsA(isA<ApiException>()));
  });
}
