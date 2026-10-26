import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/vehicle_completions.dart';
import 'package:nalaerp_client/pages/vehicle_schedules_section.dart';

// Erledigungen der Fahrzeugtermine im Client (Backlog G.2.4.2).
// InkRipple und Desktop-Fenster wie in den anderen Client-Tests.

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, required this.schedules, Map<String, List<Map<String, dynamic>>>? completions, this.recordError})
      : completions = completions ?? {},
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> schedules;
  final Map<String, List<Map<String, dynamic>>> completions;
  final Object? recordError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> recorded = [];
  final List<MapEntry<String, String>> stornos = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listVehicleSchedules(String vehicleId, {bool includeInactive = false}) async {
    calls.add('schedules');
    return schedules.where((s) => includeInactive || s['aktiv'] != false).toList();
  }

  @override
  Future<List<dynamic>> listCompletions(String scheduleId, {bool includeStorniert = false}) async {
    calls.add('completions $scheduleId $includeStorniert');
    return (completions[scheduleId] ?? const []).where((c) => includeStorniert || c['status'] == 'active').toList();
  }

  @override
  Future<Map<String, dynamic>> recordCompletion(Map<String, dynamic> body) async {
    if (recordError != null) throw recordError!;
    recorded.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> stornoCompletion(String id, String reason) async {
    stornos.add(MapEntry(id, reason));
    for (final list in completions.values) {
      for (final c in list) {
        if (c['id'] == id) {
          c['status'] = 'storniert';
          c['storno_reason'] = reason;
        }
      }
    }
    return {'id': id, 'status': 'storniert'};
  }
}

Map<String, dynamic> _sched(String id, String art, {bool aktiv = true, String bez = ''}) => {
      'id': id, 'vehicle_id': 'v1', 'art': art, 'bezeichnung': bez, 'starttermin': null, 'start_km': null, 'intervall_monate': 24,
      'intervall_km': null, 'notiz': '', 'aktiv': aktiv, 'standing': {'status': 'ok', 'faellig_am': '2027-05-31T00:00:00Z'},
    };

List<Map<String, dynamic>> _schedules() => [_sched('hu', 'hu'), _sched('w', 'wartung'), _sched('alt', 'uvv', aktiv: false)];

Map<String, List<Map<String, dynamic>>> _completions() => {
      'hu': [
        {
          'id': 'c2', 'schedule_id': 'hu', 'erledigt_am': '2026-03-10T00:00:00Z', 'ergebnis': 'maengel', 'km_stand': 42000, 'naechster_termin': '2026-04-30T00:00:00Z',
          'naechster_km': null, 'pruefer': 'TÜV', 'notiz': 'Bremse', 'status': 'active', 'storno_reason': '',
        },
        {
          'id': 'c1', 'schedule_id': 'hu', 'erledigt_am': '2024-03-01T00:00:00Z', 'ergebnis': 'bestanden', 'km_stand': null, 'naechster_termin': '2026-03-31T00:00:00Z',
          'naechster_km': null, 'pruefer': '', 'notiz': '', 'status': 'storniert', 'storno_reason': 'falsches Fahrzeug',
        },
      ],
    };

Widget _wrap(Widget child) => MaterialApp(
      theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: Scaffold(body: SingleChildScrollView(child: child)),
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _rw = {'fleet.read', 'fleet.write'};
DateTime _today() => DateTime(2026, 3, 5);

void main() {
  group('reine Funktionen', () {
    test('ergebnisseFor und parseKmStand', () {
      expect(ergebnisseFor('hu'), ['bestanden', 'maengel', 'nicht_bestanden']);
      expect(ergebnisseFor('uvv'), ['bestanden', 'maengel', 'nicht_bestanden']);
      expect(ergebnisseFor('wartung'), ['erledigt']);
      expect(ergebnisseFor('sonstige'), ['erledigt']);
      expect(parseKmStand(''), (ok: true, value: null));
      expect(parseKmStand('0'), (ok: true, value: 0));
      expect(parseKmStand('42.000'), (ok: true, value: 42000));
      expect(parseKmStand('-1'), (ok: false, value: null));
      expect(parseKmStand('x'), (ok: false, value: null));
    });

    test('buildCompletionBody und completionLine', () {
      expect(buildCompletionBody(scheduleId: 'hu', erledigtAm: '2026-03-05', ergebnis: 'bestanden', pruefer: ' TÜV ', notiz: ' ok '), {
        'schedule_id': 'hu', 'erledigt_am': '2026-03-05', 'ergebnis': 'bestanden', 'km_stand': null, 'naechster_termin': null, 'naechster_km': null,
        'pruefer': 'TÜV', 'notiz': 'ok',
      });
      final c = _completions()['hu']!;
      expect(completionLine('hu', c[0]), '10.03.2026 · Mängel · TÜV · 42000 km · nächste 04/2026 · Bremse');
      expect(completionLine('hu', c[1]), '01.03.2024 · bestanden · nächste 03/2026 (storniert: falsches Fahrzeug)');
      expect(completionLine('wartung', {'erledigt_am': '2026-01-02', 'ergebnis': 'erledigt', 'naechster_km': 60000, 'naechster_termin': '2027-01-02'}),
          '02.01.2026 · erledigt · nächste 02.01.2027 · bei 60000 km');
    });
  });

  testWidgets('HU erledigt mit Mängeln: Hinweis auf Sperre, Nachprüfung als Monat, danach neu geladen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, schedules: _schedules());
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1', now: _today)));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Erledigt erfassen'), findsNWidgets(2));
    // Stillgelegter Termin (sichtbar gemacht): kein "Erledigt erfassen",
    // aber seine Erledigungen sind lesbar.
    await tester.tap(find.text('Stillgelegte anzeigen'));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Erledigungen'), findsNWidgets(3));
    expect(find.byTooltip('Erledigt erfassen'), findsNWidgets(2));
    await tester.tap(find.byTooltip('Erledigt erfassen').first);
    await tester.pumpAndSettle();
    expect(find.text('Hauptuntersuchung erledigt'), findsOneWidget);
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)')).controller!.text, '05.03.2026');
    expect(find.text('Nächste HU (MM.JJJJ, optional)'), findsOneWidget);
    expect(find.text('Leer = Vorschlag aus dem Intervall des Termins.'), findsOneWidget);
    expect(find.textContaining('gesperrt'), findsNothing);

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Mängel').last);
    await tester.pumpAndSettle();
    expect(find.text('Das Fahrzeug ist danach gesperrt, bis eine bestandene Prüfung erfasst ist.'), findsOneWidget);
    expect(find.text('Nachprüfung bis (MM.JJJJ, optional)'), findsOneWidget);
    expect(find.text('Ohne Angabe gibt es keinen Folgetermin.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Nachprüfung bis (MM.JJJJ, optional)'), '4.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Kilometerstand (optional)'), '42.000');
    await tester.enterText(find.widgetWithText(TextField, 'Prüfstelle / Werkstatt'), 'TÜV');
    final loadsBefore = api.calls.where((c) => c == 'schedules').length;
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(api.recorded.single, {
      'schedule_id': 'hu', 'erledigt_am': '2026-03-05', 'ergebnis': 'maengel', 'km_stand': 42000, 'naechster_termin': '2026-04-01', 'naechster_km': null,
      'pruefer': 'TÜV', 'notiz': '',
    });
    expect(find.text('Erledigung erfasst'), findsOneWidget);
    expect(api.calls.where((c) => c == 'schedules').length, loadsBefore + 1);
  });

  testWidgets('Wartung: kein Ergebnis wählbar, Folge-km, leerer Folgetermin = Vorschlag; ungültige Eingaben bleiben im Dialog', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, schedules: _schedules());
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1', now: _today)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Erledigt erfassen').at(1));
    await tester.pumpAndSettle();
    expect(find.byType(DropdownButtonFormField<String>), findsNothing);
    expect(find.text('Folgetermin (TT.MM.JJJJ, optional)'), findsOneWidget);

    await tester.enterText(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)'), '31.02.2026');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte das Datum als TT.MM.JJJJ angeben.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Datum (TT.MM.JJJJ)'), '01.03.2026');
    await tester.enterText(find.widgetWithText(TextField, 'Kilometerstand (optional)'), '-3');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte den Kilometerstand als ganze Zahl angeben oder leer lassen.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Kilometerstand (optional)'), '31000');
    await tester.enterText(find.widgetWithText(TextField, 'Folgetermin (TT.MM.JJJJ, optional)'), '03.2027');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte den Folgetermin als TT.MM.JJJJ angeben oder leer lassen.'), findsOneWidget);
    await tester.enterText(find.widgetWithText(TextField, 'Folgetermin (TT.MM.JJJJ, optional)'), '');
    await tester.enterText(find.widgetWithText(TextField, 'Nächste Wartung bei km (optional, leer = km-Stand + Intervall)'), '0');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Kilometer als ganze Zahl angeben oder leer lassen.'), findsOneWidget);
    expect(api.recorded, isEmpty);
    await tester.enterText(find.widgetWithText(TextField, 'Nächste Wartung bei km (optional, leer = km-Stand + Intervall)'), '61.000');
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(api.recorded.single['ergebnis'], 'erledigt');
    expect(api.recorded.single['erledigt_am'], '2026-03-01');
    expect(api.recorded.single['km_stand'], 31000);
    expect(api.recorded.single['naechster_termin'], isNull);
    expect(api.recorded.single['naechster_km'], 61000);
  });

  testWidgets('Fehler des Servers beim Erfassen: Meldung, nicht neu geladen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, schedules: _schedules(), recordError: const ApiException(statusCode: 409, message: 'Fahrzeug ist abgemeldet'));
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1', now: _today)));
    await tester.pumpAndSettle();
    final loads = api.calls.length;
    await tester.tap(find.byTooltip('Erledigt erfassen').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Fahrzeug ist abgemeldet'), findsOneWidget);
    expect(api.calls.length, loads);
  });

  testWidgets('Erledigungen ansehen und stornieren; nur mit Begründung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, schedules: _schedules(), completions: _completions());
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1', now: _today)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Erledigungen').first);
    await tester.pumpAndSettle();
    expect(find.text('Erledigungen – Hauptuntersuchung'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('completions')).toList(), ['completions hu true']);
    expect(find.text('10.03.2026 · Mängel · TÜV · 42000 km · nächste 04/2026 · Bremse'), findsOneWidget);
    expect(find.text('01.03.2024 · bestanden · nächste 03/2026 (storniert: falsches Fahrzeug)'), findsOneWidget);
    // Nur die aktive ist stornierbar.
    expect(find.byTooltip('Erledigung stornieren'), findsOneWidget);

    await tester.tap(find.byTooltip('Erledigung stornieren'));
    await tester.pumpAndSettle();
    final button = find.widgetWithText(FilledButton, 'Stornieren');
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), '  ');
    await tester.pumpAndSettle();
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), ' falscher Termin ');
    await tester.pumpAndSettle();
    await tester.tap(button);
    await tester.pumpAndSettle();
    expect(api.stornos.single.key, 'c2');
    expect(api.stornos.single.value, 'falscher Termin');
    expect(find.byTooltip('Erledigung stornieren'), findsNothing);
    expect(find.textContaining('(storniert: falscher Termin)'), findsOneWidget);

    final loads = api.calls.where((c) => c == 'schedules').length;
    await tester.tap(find.text('Schließen'));
    await tester.pumpAndSettle();
    // Nach einem Storno wird der Stand neu geladen.
    expect(api.calls.where((c) => c == 'schedules').length, loads + 1);
  });

  testWidgets('ohne Erledigungen; Schließen ohne Storno lädt nicht neu', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, schedules: _schedules());
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1', now: _today)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Erledigungen').at(1));
    await tester.pumpAndSettle();
    expect(find.text('Noch keine Erledigungen.'), findsOneWidget);
    final loads = api.calls.where((c) => c == 'schedules').length;
    await tester.tap(find.text('Schließen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c == 'schedules').length, loads);
  });

  testWidgets('nur lesen: Erledigungen sichtbar, nichts erfassen oder stornieren', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.read'}, schedules: _schedules(), completions: _completions());
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1', now: _today)));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Erledigt erfassen'), findsNothing);
    await tester.tap(find.byTooltip('Erledigungen').first);
    await tester.pumpAndSettle();
    expect(find.textContaining('10.03.2026'), findsOneWidget);
    expect(find.byTooltip('Erledigung stornieren'), findsNothing);
  });

  testWidgets('abgemeldetes Fahrzeug: nichts erfassen, aber stornieren (Berichtigung)', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, schedules: _schedules(), completions: _completions());
    await tester.pumpWidget(_wrap(VehicleSchedulesSection(api: api, vehicleId: 'v1', deregistered: true, now: _today)));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Erledigt erfassen'), findsNothing);
    await tester.tap(find.byTooltip('Erledigungen').first);
    await tester.pumpAndSettle();
    expect(find.byTooltip('Erledigung stornieren'), findsOneWidget);
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
      await mock([], () => api.listCompletions('s'));
      await mock([], () => api.listCompletions('s', includeStorniert: true));
      await mock({'id': 'c'}, () => api.recordCompletion({'schedule_id': 's'}), status: 201);
      await mock({'id': 'c'}, () => api.stornoCompletion('c', 'falsch'));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/fleet/schedules/s/completions?',
        'GET /api/v1/fleet/schedules/s/completions?include_storniert=true',
        'POST /api/v1/fleet/completions/?',
        'POST /api/v1/fleet/completions/c/storno?',
      ]);
      expect(jsonDecode(calls[2].body), {'schedule_id': 's'});
      expect(jsonDecode(calls[3].body), {'reason': 'falsch'});
      expect(calls[3].headers['content-type'], startsWith('application/json'));
      await expectLater(mock({'id': 'c'}, () => api.recordCompletion({}), status: 200), throwsA(isA<ApiException>()));
      await expectLater(
        mock({'error': {'code': 'conflict', 'message': 'Erledigung ist bereits storniert'}}, () => api.stornoCompletion('c', 'x'), status: 409),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Erledigung ist bereits storniert')),
      );
    });
  });
}
