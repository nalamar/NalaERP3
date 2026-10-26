import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/fleet_page.dart';
import 'package:nalaerp_client/pages/vehicle_detail_page.dart';
import 'package:nalaerp_client/pages/vehicle_schedules_section.dart';

// Termine am Fahrzeug im Client (Backlog G.2.4.1). InkRipple und
// Desktop-Fenster wie in den anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, List<Map<String, dynamic>>? schedules, this.saveError})
      : schedules = schedules ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> schedules;
  final Object? saveError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> created = [];
  final List<MapEntry<String, Map<String, dynamic>>> updated = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listVehicleSchedules(String vehicleId, {bool includeInactive = false}) async {
    calls.add('list $vehicleId $includeInactive');
    // Wie der Server: stillgelegte nur auf Wunsch.
    return schedules.where((s) => includeInactive || s['aktiv'] != false).toList();
  }

  @override
  Future<Map<String, dynamic>> createSchedule(Map<String, dynamic> body) async {
    if (saveError != null) throw saveError!;
    created.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> updateSchedule(String id, Map<String, dynamic> body) async {
    if (saveError != null) throw saveError!;
    updated.add(MapEntry(id, body));
    return {'id': id, ...body};
  }

  @override
  Future<List<dynamic>> listVehicles({String? art, bool includeAbgemeldete = false}) async => [_vehicle];

  @override
  Future<List<dynamic>> listCostCenters({bool includeInactive = false}) async => const [];
}

const _vehicle = {
  'id': 'v1', 'kennzeichen': 'M-AB 1', 'art': 'lkw', 'bezeichnung': 'Pritsche', 'hersteller': 'MAN', 'modell': 'TGL', 'fin': 'WVWZZZ1JZ3W386752',
  'erstzulassung': null, 'standort': 'Hof', 'notiz': '', 'fahrtenbuch': false, 'abgemeldet_am': null,
};

Map<String, dynamic> _sched(String id, String art, Map<String, dynamic> standing,
        {String bez = '', String? start, int? iv, bool aktiv = true, int? startKm, int? ivKm, String notiz = ''}) =>
    {
      'id': id, 'vehicle_id': 'v1', 'art': art, 'bezeichnung': bez, 'starttermin': start, 'start_km': startKm, 'intervall_monate': iv,
      'intervall_km': ivKm, 'notiz': notiz, 'aktiv': aktiv, 'standing': standing,
    };

List<Map<String, dynamic>> _schedules() => [
      _sched('s1', 'hu', {'status': 'gesperrt', 'faellig_am': '2026-11-30T00:00:00Z', 'tage_bis_faellig': 35, 'letztes_ergebnis': 'maengel'},
          start: '2027-05-31T00:00:00Z', iv: 24, notiz: 'Plakette'),
      _sched('s2', 'uvv', {'status': 'ueberfaellig', 'faellig_am': '2026-10-01T00:00:00Z', 'tage_bis_faellig': -25}, iv: 12),
      _sched('s3', 'wartung', {'status': 'ohne_termin', 'faellig_km': 60000}, startKm: 60000, ivKm: 30000),
      _sched('s4', 'sonstige', {'status': 'faellig_bald', 'faellig_am': '2026-11-05T00:00:00Z', 'tage_bis_faellig': 10}, bez: 'Tachograph'),
      _sched('s5', 'leasing', {'status': 'ohne_termin'}, aktiv: false),
    ];

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: Scaffold(body: SingleChildScrollView(child: child)),
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'fleet.read', 'fleet.write'};

void main() {
  group('reine Funktionen', () {
    test('scheduleTitle und dueDateLabel', () {
      expect(scheduleTitle({'art': 'hu'}), 'Hauptuntersuchung');
      expect(scheduleTitle({'art': 'sonstige', 'bezeichnung': 'Tachograph'}), 'Tachograph');
      expect(dueDateLabel('hu', '2027-05-31T00:00:00Z'), '05/2027');
      expect(dueDateLabel('uvv', '2027-05-31T00:00:00Z'), '31.05.2027');
      expect(dueDateLabel('hu', null), '');
    });

    test('terminstandLabel', () {
      expect(terminstandLabel('hu', {'status': 'gesperrt', 'faellig_am': '2026-11-30', 'letztes_ergebnis': 'maengel'}),
          'gesperrt – letzte Prüfung mit Mängeln, Nachprüfung bis 11/2026');
      expect(terminstandLabel('uvv', {'status': 'gesperrt', 'letztes_ergebnis': 'nicht_bestanden'}), 'gesperrt – letzte Prüfung nicht bestanden');
      expect(terminstandLabel('uvv', {'status': 'ueberfaellig', 'faellig_am': '2026-10-01'}), 'überfällig seit 01.10.2026');
      expect(terminstandLabel('hu', {'status': 'faellig_bald', 'faellig_am': '2026-11-30', 'tage_bis_faellig': 5}), 'fällig im 11/2026 (in 5 Tagen)');
      expect(terminstandLabel('uvv', {'status': 'faellig_bald', 'faellig_am': '2026-10-26', 'tage_bis_faellig': 0}), 'heute fällig');
      expect(terminstandLabel('uvv', {'status': 'ok', 'faellig_am': '2027-10-01'}), 'fällig am 01.10.2027');
      expect(terminstandLabel('wartung', {'status': 'ok', 'faellig_am': '2027-10-01', 'faellig_km': 60000}), 'fällig am 01.10.2027 oder bei 60000 km');
      expect(terminstandLabel('wartung', {'status': 'ohne_termin', 'faellig_km': 60000}), 'fällig bei 60000 km');
      expect(terminstandLabel('leasing', {'status': 'ohne_termin'}), 'kein Termin');
      expect(terminstandIsBad('gesperrt'), isTrue);
      expect(terminstandIsBad('ueberfaellig'), isTrue);
      expect(terminstandIsBad('faellig_bald'), isFalse);
    });

    test('parseMonthInput und parsePositiveInt', () {
      expect(parseMonthInput('05.2027'), '2027-05-01');
      expect(parseMonthInput(' 5/2027 '), '2027-05-01');
      expect(parseMonthInput('12.2027'), '2027-12-01');
      expect(parseMonthInput('13.2027'), isNull);
      expect(parseMonthInput('0.2027'), isNull);
      expect(parseMonthInput('15.05.2027'), '2027-05-15');
      expect(parseMonthInput('Mai'), isNull);
      expect(parsePositiveInt(''), (ok: true, value: null));
      expect(parsePositiveInt('30.000'), (ok: true, value: 30000));
      expect(parsePositiveInt('0'), (ok: false, value: null));
      expect(parsePositiveInt('x'), (ok: false, value: null));
    });

    test('Bodies: POST ohne Bezeichnung ausser bei sonstige; PUT immer alle sechs Felder', () {
      expect(buildScheduleCreateBody(vehicleId: 'v1', art: 'hu', bezeichnung: 'egal', notiz: ' x '), {
        'vehicle_id': 'v1', 'art': 'hu', 'bezeichnung': '', 'starttermin': null, 'start_km': null, 'intervall_monate': null, 'intervall_km': null, 'notiz': 'x',
      });
      expect(buildScheduleCreateBody(vehicleId: 'v1', art: 'sonstige', bezeichnung: ' Tacho ')['bezeichnung'], 'Tacho');
      expect(buildScheduleUpdateBody(aktiv: false), {
        'starttermin': null, 'start_km': null, 'intervall_monate': null, 'intervall_km': null, 'notiz': '', 'aktiv': false,
      });
    });
  });

  testWidgets('zeigt Termine mit Stand; Überfälliges und Gesperrtes rot; ohne Schreibrecht nur lesen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, schedules: _schedules());
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1')));
    await tester.pumpAndSettle();
    expect(find.text('gesperrt – letzte Prüfung mit Mängeln, Nachprüfung bis 11/2026'), findsOneWidget);
    expect(find.text('überfällig seit 01.10.2026'), findsOneWidget);
    expect(find.text('fällig bei 60000 km'), findsOneWidget);
    expect(find.text('Tachograph'), findsOneWidget);
    expect(find.text('Leasingende'), findsNothing);
    Color? color(String text) => tester.widget<Text>(find.text(text)).style?.color;
    expect(color('überfällig seit 01.10.2026'), const Color(0xFFB00020));
    expect(color('fällig am 05.11.2026 (in 10 Tagen)'), isNull);
    expect(find.text('Termin anlegen'), findsNothing);
    expect(find.byTooltip('Termin bearbeiten'), findsNothing);

    await tester.tap(find.text('Stillgelegte anzeigen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list v1 true');
    expect(find.text('Leasingende · stillgelegt'), findsOneWidget);
  });

  testWidgets('abgemeldetes Fahrzeug: auch mit Schreibrecht nichts zu planen', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: _FakeApi(permissions: _rw, schedules: _schedules()), vehicleId: 'v1', deregistered: true)));
    await tester.pumpAndSettle();
    expect(find.text('Termin anlegen'), findsNothing);
    expect(find.byTooltip('Termin bearbeiten'), findsNothing);
  });

  testWidgets('leer', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: _FakeApi(permissions: _rw), vehicleId: 'v1')));
    await tester.pumpAndSettle();
    expect(find.text('Keine Termine.'), findsOneWidget);
  });

  testWidgets('HU anlegen mit Monat; ungültiger Monat bleibt im Dialog', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Termin anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Leer = Vorschlag (HU 24, UVV und Versicherung 12).'), findsOneWidget);
    expect(find.text('Nächste Wartung bei km (optional)'), findsNothing);
    await tester.enterText(find.widgetWithText(TextField, 'Nächste HU (MM.JJJJ, laut Plakette)'), '13.2027');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die HU als MM.JJJJ angeben oder leer lassen.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Nächste HU (MM.JJJJ, laut Plakette)'), '05.2027');
    await tester.enterText(find.widgetWithText(TextField, 'Intervall in Monaten'), '0');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Intervall als ganze Zahl von Monaten angeben oder leer lassen.'), findsOneWidget);
    expect(api.created, isEmpty);
    await tester.enterText(find.widgetWithText(TextField, 'Intervall in Monaten'), '');
    await tester.enterText(find.widgetWithText(TextField, 'Notiz'), 'Plakette');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created.single, {
      'vehicle_id': 'v1', 'art': 'hu', 'bezeichnung': '', 'starttermin': '2027-05-01', 'start_km': null, 'intervall_monate': null, 'intervall_km': null, 'notiz': 'Plakette',
    });
    expect(find.text('Termin angelegt'), findsOneWidget);
    expect(api.calls.length, 2);
  });

  testWidgets('Wartung nach km und sonstiger Termin mit Bezeichnung anlegen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Termin anlegen'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Wartung').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Nächster Termin (TT.MM.JJJJ, optional)'), '31.02.2027');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte den Termin als TT.MM.JJJJ angeben oder leer lassen.'), findsOneWidget);
    // Ein Monat genuegt nur bei der HU.
    await tester.enterText(find.widgetWithText(TextField, 'Nächster Termin (TT.MM.JJJJ, optional)'), '11.2026');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte den Termin als TT.MM.JJJJ angeben oder leer lassen.'), findsOneWidget);
    expect(api.created, isEmpty);
    await tester.enterText(find.widgetWithText(TextField, 'Nächster Termin (TT.MM.JJJJ, optional)'), '');
    await tester.enterText(find.widgetWithText(TextField, 'Nächste Wartung bei km (optional)'), 'viel');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte Kilometer als ganze Zahl angeben oder leer lassen.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Nächste Wartung bei km (optional)'), '60.000');
    // Nur das Intervall ungueltig.
    await tester.enterText(find.widgetWithText(TextField, 'Intervall in km (optional)'), '-5');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte Kilometer als ganze Zahl angeben oder leer lassen.'), findsOneWidget);
    expect(api.created, isEmpty);
    await tester.enterText(find.widgetWithText(TextField, 'Intervall in km (optional)'), '30000');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created.single['art'], 'wartung');
    expect(api.created.single['start_km'], 60000);
    expect(api.created.single['intervall_km'], 30000);
    expect(api.created.single['starttermin'], isNull);
  });

  testWidgets('sonstiger Termin braucht eine Bezeichnung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw);
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Termin anlegen'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Sonstiges').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte eine Bezeichnung angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Bezeichnung (z. B. Tachograph)'), ' Tachograph ');
    await tester.enterText(find.widgetWithText(TextField, 'Nächster Termin (TT.MM.JJJJ, optional)'), '5.11.2026');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created.single['bezeichnung'], 'Tachograph');
    expect(api.created.single['starttermin'], '2026-11-05');
  });

  testWidgets('bearbeiten: vorbelegt, HU als Monat, PUT mit allen Feldern, stilllegen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, schedules: _schedules());
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Termin bearbeiten').first);
    await tester.pumpAndSettle();
    expect(find.text('Hauptuntersuchung bearbeiten'), findsOneWidget);
    expect(find.byType(DropdownButtonFormField<String>), findsNothing);
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Nächste HU (MM.JJJJ, laut Plakette)')).controller!.text, '05.2027');
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Intervall in Monaten')).controller!.text, '24');
    await tester.tap(find.text('Aktiv'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.updated.single.key, 's1');
    expect(api.updated.single.value, {
      'starttermin': '2027-05-01', 'start_km': null, 'intervall_monate': 24, 'intervall_km': null, 'notiz': 'Plakette', 'aktiv': false,
    });
    expect(find.text('Termin gespeichert'), findsOneWidget);

    // Wartung: km-Felder vorbelegt.
    await tester.tap(find.byTooltip('Termin bearbeiten').at(2));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Nächste Wartung bei km (optional)')).controller!.text, '60000');
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Intervall in km (optional)')).controller!.text, '30000');
  });

  testWidgets('Fehler des Servers wird angezeigt', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, saveError: const ApiException(statusCode: 400, message: 'Termin dieser Art existiert bereits für das Fahrzeug'));
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Termin anlegen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Termin dieser Art existiert bereits für das Fahrzeug'), findsOneWidget);
  });

  testWidgets('Fuhrpark-Liste öffnet die Detailseite mit Terminen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, schedules: _schedules());
    await tester.pumpWidget(MaterialApp(theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory), home: FleetPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('M-AB 1 – Pritsche'));
    await tester.pumpAndSettle();
    expect(find.byType(VehicleDetailPage), findsOneWidget);
    expect(find.text('FIN: WVWZZZ1JZ3W386752'), findsOneWidget);
    expect(find.text('Standort: Hof'), findsOneWidget);
    expect(find.text('überfällig seit 01.10.2026'), findsOneWidget);
    expect(api.calls.last, 'list v1 false');
  });

  testWidgets('Detailseite eines abgemeldeten Fahrzeugs: keine Planung', (tester) async {
    await _desktop(tester);
    final gone = Map<String, dynamic>.from(_vehicle)..['abgemeldet_am'] = '2026-01-15T00:00:00Z';
    await tester.pumpWidget(MaterialApp(home: VehicleDetailPage(api: _FakeApi(permissions: _rw, schedules: _schedules()), vehicle: gone)));
    await tester.pumpAndSettle();
    expect(find.text('überfällig seit 01.10.2026'), findsOneWidget);
    expect(find.text('Termin anlegen'), findsNothing);
    expect(find.byTooltip('Termin bearbeiten'), findsNothing);
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
      await mock([], () => api.listVehicleSchedules('v1'));
      await mock([], () => api.listVehicleSchedules('v1', includeInactive: true));
      await mock({'id': 's'}, () => api.createSchedule({'art': 'hu'}), status: 201);
      await mock({'id': 's'}, () => api.updateSchedule('s', {'aktiv': true}));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/fleet/vehicles/v1/schedules?',
        'GET /api/v1/fleet/vehicles/v1/schedules?include_inactive=true',
        'POST /api/v1/fleet/schedules/?',
        'PUT /api/v1/fleet/schedules/s?',
      ]);
      expect(jsonDecode(calls[2].body), {'art': 'hu'});
      expect(jsonDecode(calls[3].body), {'aktiv': true});
      expect(calls[2].headers['content-type'], startsWith('application/json'));
      await expectLater(mock({'id': 's'}, () => api.createSchedule({}), status: 200), throwsA(isA<ApiException>()));
      await expectLater(
        mock({'error': {'code': 'conflict', 'message': 'Fahrzeug ist abgemeldet'}}, () => api.updateSchedule('s', {}), status: 409),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Fahrzeug ist abgemeldet')),
      );
    });
  });
}
