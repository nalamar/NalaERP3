import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/production_planning_page.dart';

// Arbeitsplätze und Arbeitsgangvorlagen im Client (Backlog H.2.5a).

final _centers = [
  {'id': 'wc1', 'name': 'Zuschnitt', 'stunden_je_tag': 8, 'aktiv': true},
  {'id': 'wc2', 'name': 'Montage', 'stunden_je_tag': 7.5, 'aktiv': false},
];

final _templates = [
  {
    'id': 't1',
    'name': 'Sägen',
    'work_center_id': 'wc1',
    'work_center_name': 'Zuschnitt',
    'reihenfolge': 1,
    'ruestzeit_min': 30,
    'minuten_je_stueck': 4.5,
    'aktiv': true
  },
  {
    'id': 't2',
    'name': 'Verglasung',
    'work_center_id': 'wc2',
    'work_center_name': 'Montage',
    'reihenfolge': 3,
    'ruestzeit_min': 0,
    'minuten_je_stueck': 20,
    'aktiv': true
  },
];

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, this.fail})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final String? fail;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listWorkCenters({bool includeInactive = false}) async {
    calls.add('centers $includeInactive');
    return _centers;
  }

  @override
  Future<List<dynamic>> listOperationTemplates(
      {bool includeInactive = false}) async {
    calls.add('templates $includeInactive');
    return _templates;
  }

  @override
  Future<Map<String, dynamic>> createWorkCenter(
      Map<String, dynamic> body) async {
    calls.add('createWC ${jsonEncode(body)}');
    if (fail != null) throw ApiException(statusCode: 400, message: fail!);
    return {'id': 'wc9', ...body};
  }

  @override
  Future<Map<String, dynamic>> updateWorkCenter(
      String id, Map<String, dynamic> body) async {
    calls.add('updateWC $id ${jsonEncode(body)}');
    return {'id': id, ...body};
  }

  @override
  Future<Map<String, dynamic>> createOperationTemplate(
      Map<String, dynamic> body) async {
    calls.add('createT ${jsonEncode(body)}');
    if (fail != null) throw ApiException(statusCode: 409, message: fail!);
    return {'id': 't9', ...body};
  }

  @override
  Future<Map<String, dynamic>> updateOperationTemplate(
      String id, Map<String, dynamic> body) async {
    calls.add('updateT $id ${jsonEncode(body)}');
    return {'id': id, ...body};
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

void main() {
  test('Zahlen und Minuten', () {
    expect(parseDecimalDe(' 7,5 '), 7.5);
    expect(parseDecimalDe('8'), 8);
    expect(parseDecimalDe(''), isNull);
    expect(parseDecimalDe('acht'), isNull);
    expect(formatMinutes(45), '45 min');
    expect(formatMinutes(60), '1 h');
    expect(formatMinutes(70.4), '1 h 10 min');
    expect(formatMinutes(0), '0 min');
  });

  test('Prüfung und Body Arbeitsplatz', () {
    expect(workCenterInputError('Zuschnitt', '7,5'), isNull);
    expect(workCenterInputError('x', '24'), isNull);
    expect(workCenterInputError(' ', '8'), 'Bitte einen Namen angeben.');
    for (final bad in ['0', '-1', '24,5', '', 'acht']) {
      expect(workCenterInputError('x', bad),
          'Stunden je Tag: mehr als 0 und höchstens 24.',
          reason: bad);
    }
    expect(buildWorkCenterBody(name: ' Zuschnitt ', hours: '7,5'),
        {'name': 'Zuschnitt', 'stunden_je_tag': 7.5});
    expect(buildWorkCenterBody(name: 'x', hours: '8', aktiv: false),
        {'name': 'x', 'stunden_je_tag': 8.0, 'aktiv': false});
  });

  test('Prüfung und Body Vorlage', () {
    expect(templateInputError('Sägen', 'wc1', '1', '', '4,5'), isNull);
    expect(templateInputError(' ', 'wc1', '1', '', ''),
        'Bitte einen Namen angeben.');
    expect(templateInputError('x', null, '1', '', ''),
        'Bitte einen Arbeitsplatz wählen.');
    expect(templateInputError('x', '', '1', '', ''),
        'Bitte einen Arbeitsplatz wählen.');
    for (final bad in ['0', '-2', '1,5', '']) {
      expect(templateInputError('x', 'wc1', bad, '', ''),
          'Reihenfolge: ganze Zahl ab 1.',
          reason: bad);
    }
    expect(templateInputError('x', 'wc1', '1', '-1', ''),
        'Zeiten: Minuten, nicht negativ.');
    expect(templateInputError('x', 'wc1', '1', '', 'viel'),
        'Zeiten: Minuten, nicht negativ.');
    expect(
        buildTemplateBody(
            name: ' Sägen ',
            workCenterId: 'wc1',
            reihenfolge: ' 2 ',
            ruest: '',
            jeStueck: '4,5'),
        {
          'name': 'Sägen',
          'work_center_id': 'wc1',
          'reihenfolge': 2,
          'ruestzeit_min': 0,
          'minuten_je_stueck': 4.5
        });
    expect(
        buildTemplateBody(
            name: 'x',
            workCenterId: 'wc1',
            reihenfolge: '1',
            ruest: '10',
            jeStueck: '0',
            aktiv: true)['aktiv'],
        true);
  });

  test('Auswahl der Arbeitsplätze und Zeilen', () {
    expect(templateWorkCenterChoices(_centers, null), [
      {'id': 'wc1', 'label': 'Zuschnitt'}
    ]);
    expect(templateWorkCenterChoices(_centers, 'wc2'), [
      {'id': 'wc1', 'label': 'Zuschnitt'},
      {'id': 'wc2', 'label': 'Montage (deaktiviert)'},
    ]);
    expect(workCenterLine(_centers[0]), '8 h je Arbeitstag');
    expect(workCenterLine(_centers[1]), '7,5 h je Arbeitstag · deaktiviert');
    expect(templateLine(_templates[0]),
        '1. · Zuschnitt · Rüsten 30 min · 4,5 min je Stück');
    expect(templateLine({..._templates[1], 'aktiv': false}),
        '3. · Montage · 20 min je Stück · deaktiviert');
  });

  testWidgets(
      'Liste: deaktivierte nur auf Wunsch, nur lesen ohne Schaltflächen',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.read'});
    await tester.pumpWidget(_wrap(ProductionPlanningPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('Zuschnitt'), findsOneWidget);
    expect(find.text('Montage'), findsNothing);
    expect(find.text('Sägen'), findsOneWidget);
    expect(find.byIcon(Icons.edit), findsNothing);
    expect(find.text('Arbeitsplatz'), findsNothing);
    expect(api.calls, ['centers true', 'templates false']);
    await tester.tap(find.text('Deaktivierte anzeigen'));
    await tester.pumpAndSettle();
    expect(find.text('Montage'), findsOneWidget);
    expect(find.text('7,5 h je Arbeitstag · deaktiviert'), findsOneWidget);
    expect(api.calls.last, 'templates true');
  });

  testWidgets('Arbeitsplatz anlegen mit Prüfung, ändern und deaktivieren',
      (tester) async {
    await _desktop(tester);
    final api =
        _FakeApi(permissions: const {'production.read', 'production.write'});
    await tester.pumpWidget(_wrap(ProductionPlanningPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Arbeitsplatz'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Namen angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Name'), 'Bohren');
    await tester.enterText(
        find.widgetWithText(TextField, 'Stunden je Arbeitstag'), '25');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Stunden je Tag: mehr als 0 und höchstens 24.'),
        findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('createWC')), isEmpty);
    await tester.enterText(
        find.widgetWithText(TextField, 'Stunden je Arbeitstag'), '6,5');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(
        api.calls, contains('createWC {"name":"Bohren","stunden_je_tag":6.5}'));
    expect(find.text('Arbeitsplatz angelegt'), findsOneWidget);

    await tester.tap(find.byTooltip('Arbeitsplatz bearbeiten'));
    await tester.pumpAndSettle();
    expect(find.text('Aktiv'), findsOneWidget);
    await tester.tap(find.text('Aktiv'));
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(
        api.calls,
        contains(
            'updateWC wc1 {"name":"Zuschnitt","stunden_je_tag":8.0,"aktiv":false}'));
  });

  testWidgets(
      'Vorlage anlegen: nur aktive Arbeitsplätze, Reihenfolge vorbelegt, Fehler als Meldung',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
        permissions: const {'production.read', 'production.write'},
        fail: 'Arbeitsplatz ist deaktiviert');
    await tester.pumpWidget(_wrap(ProductionPlanningPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Vorlage'));
    await tester.pumpAndSettle();
    final order =
        tester.widget<TextField>(find.widgetWithText(TextField, 'Reihenfolge'));
    expect(order.controller!.text, '4');
    await tester.enterText(
        find.widgetWithText(TextField, 'Name des Arbeitsgangs'), 'Bohren');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Arbeitsplatz wählen.'), findsOneWidget);
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    expect(find.text('Montage (deaktiviert)'), findsNothing);
    await tester.tap(find.text('Zuschnitt').last);
    await tester.pumpAndSettle();
    await tester.enterText(
        find.widgetWithText(TextField, 'Minuten je Stück'), '2,5');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(
        api.calls,
        contains(
            'createT {"name":"Bohren","work_center_id":"wc1","reihenfolge":4,"ruestzeit_min":0.0,"minuten_je_stueck":2.5}'));
    expect(find.textContaining('Arbeitsplatz ist deaktiviert'), findsOneWidget);
  });

  testWidgets('Vorlage auf deaktiviertem Arbeitsplatz bleibt änderbar',
      (tester) async {
    await _desktop(tester);
    final api =
        _FakeApi(permissions: const {'production.read', 'production.write'});
    await tester.pumpWidget(_wrap(ProductionPlanningPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Vorlage bearbeiten').last);
    await tester.pumpAndSettle();
    expect(find.text('Montage (deaktiviert)'), findsOneWidget);
    await tester.enterText(
        find.widgetWithText(TextField, 'Rüstzeit (Minuten je Auftrag)'), '5');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(
        api.calls,
        contains(
            'updateT t2 {"name":"Verglasung","work_center_id":"wc2","reihenfolge":3,"ruestzeit_min":5.0,"minuten_je_stueck":20.0,"aktiv":true}'));
  });

  testWidgets('Dashboard: Karte nur mit production.read', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.read'});
    await tester.pumpWidget(_wrap(DashboardPage(
        api: api,
        onLogout: () async {},
        permissions: const {'production.write'})));
    await tester.pumpAndSettle();
    expect(find.text('Arbeitsplätze und Vorlagen'), findsNothing);
    await tester.pumpWidget(_wrap(DashboardPage(
        key: const ValueKey('mit'),
        api: api,
        onLogout: () async {},
        permissions: const {'production.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Arbeitsplätze und Vorlagen'));
    await tester.pumpAndSettle();
    expect(find.byType(ProductionPlanningPage), findsOneWidget);
  });

  group('API-Methoden', () {
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

    test('Pfade, Parameter, Bodies', () async {
      await mock([], () => api.listWorkCenters());
      await mock([], () => api.listWorkCenters(includeInactive: true));
      await mock({'id': 'wc1'}, () => api.createWorkCenter({'name': 'x'}),
          status: 201);
      await mock(
          {'id': 'wc1'}, () => api.updateWorkCenter('wc1', {'aktiv': false}));
      await mock([], () => api.listOperationTemplates());
      await mock([], () => api.listOperationTemplates(includeInactive: true));
      await mock({'id': 't1'}, () => api.createOperationTemplate({'name': 'y'}),
          status: 201);
      await mock({'id': 't1'},
          () => api.updateOperationTemplate('t1', {'reihenfolge': 2}));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/production/work-centers/?',
        'GET /api/v1/production/work-centers/?include_inactive=true',
        'POST /api/v1/production/work-centers/?',
        'PUT /api/v1/production/work-centers/wc1?',
        'GET /api/v1/production/operation-templates/?',
        'GET /api/v1/production/operation-templates/?include_inactive=true',
        'POST /api/v1/production/operation-templates/?',
        'PUT /api/v1/production/operation-templates/t1?',
      ]);
      expect(calls[2].body, '{"name":"x"}');
      expect(calls[3].body, '{"aktiv":false}');
      expect(calls[6].body, '{"name":"y"}');
      expect(calls[7].body, '{"reihenfolge":2}');
    });

    test('Fehler als ApiException', () async {
      // Anlegen erwartet 201; 200 ist ein Fehler, ebenso 409 beim Ändern.
      await expectLater(mock({'id': 'x'}, () => api.createWorkCenter({})),
          throwsA(isA<ApiException>()));
      await expectLater(
          mock({
            'error': {
              'code': 'conflict',
              'message': 'Arbeitsplatz ist deaktiviert'
            }
          }, () => api.updateOperationTemplate('t1', {}), status: 409),
          throwsA(isA<ApiException>().having(
              (e) => e.message, 'message', 'Arbeitsplatz ist deaktiviert')));
    });
  });
}
