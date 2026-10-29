import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/vehicle_detail_page.dart';
import 'package:nalaerp_client/pages/vehicle_trips_section.dart';

// Fahrtenbuch am Fahrzeug (Backlog G.3c.4a). Erfundene Daten. InkRipple
// und Desktop-Fenster wie in den anderen Client-Tests (Backlog 0.48).

class _FakeApi extends ApiClient {
  _FakeApi(
      {required this.permissions,
      List<Map<String, dynamic>>? trips,
      this.recordError})
      : trips = trips ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> trips;
  final Object? recordError;
  final List<String> calls = [];
  final List<Map<String, dynamic>> recorded = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listFleetTrips(
      {String? vehicleId,
      String? employeeId,
      String? von,
      String? bis,
      bool includeStorniert = false}) async {
    calls.add('list $vehicleId $includeStorniert');
    return List.of(trips);
  }

  @override
  Future<Map<String, dynamic>> getFleetTripChain(String vehicleId) async {
    calls.add('chain $vehicleId');
    final active = trips.where((t) => t['status'] == 'active').toList()
      ..sort(
          (a, b) => (a['km_beginn'] as int).compareTo(b['km_beginn'] as int));
    final gaps = <Map<String, dynamic>>[];
    for (var i = 1; i < active.length; i++) {
      if (active[i]['km_beginn'] as int > (active[i - 1]['km_ende'] as int)) {
        gaps.add({
          'von_km': active[i - 1]['km_ende'],
          'bis_km': active[i]['km_beginn']
        });
      }
    }
    return {
      'km_stand': active.isEmpty ? null : active.last['km_ende'],
      'luecken': gaps
    };
  }

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async => [
        {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Berg', 'active': true},
        {'id': 'e9', 'first_name': 'Otto', 'last_name': 'Alt', 'active': false},
      ];

  @override
  Future<Map<String, dynamic>> recordFleetTrip(
      Map<String, dynamic> body) async {
    recorded.add(Map.of(body));
    if (recordError != null) throw recordError!;
    trips.insert(0, {
      'id': 'neu',
      'datum': '${body['datum']}T00:00:00Z',
      'km_beginn': body['km_beginn'],
      'km_ende': body['km_ende'],
      'fahrtart': body['fahrtart'],
      'mitarbeiter_name': 'Anna Berg',
      'ziel': body['ziel'],
      'zweck': body['zweck'],
      'geschaeftspartner': body['geschaeftspartner'],
      'status': 'active',
    });
    return {'id': 'neu'};
  }

  @override
  Future<Map<String, dynamic>> stornoFleetTrip(String id, String reason) async {
    calls.add('storno $id $reason');
    final t = trips.firstWhere((t) => t['id'] == id);
    t['status'] = 'storniert';
    t['storno_reason'] = reason;
    return {'id': id};
  }

  @override
  Future<List<dynamic>> listFleetAssignments(
          {String? vehicleId,
          String? employeeId,
          bool nurOffen = false,
          bool includeStorniert = false}) async =>
      const [];

  @override
  Future<List<dynamic>> listVehicleSchedules(String vehicleId,
          {bool includeInactive = false}) async =>
      const [];
}

List<Map<String, dynamic>> _trips() => [
      {
        'id': 't2', 'datum': '2026-10-02T00:00:00Z', 'km_beginn': 12050,
        'km_ende': 12100, 'fahrtart': 'privat', //
        'mitarbeiter_name': 'Bert Cord', 'ziel': '', 'zweck': '',
        'geschaeftspartner': '', 'status': 'active',
      },
      {
        'id': 't1', 'datum': '2026-10-01T00:00:00Z', 'km_beginn': 12000,
        'km_ende': 12050, 'fahrtart': 'dienstlich', //
        'mitarbeiter_name': 'Anna Berg', 'ziel': 'Augsburg', 'zweck': 'Aufmaß',
        'geschaeftspartner': 'Kunde A', 'status': 'active',
      },
    ];

final _now = DateTime(2026, 10, 19, 10);
const _rw = {'fleet.read', 'fleet.write'};

Widget _section(_FakeApi api) => MaterialApp(
      theme:
          ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
      home: Scaffold(
          body: SingleChildScrollView(
              child: VehicleTripsSection(
                  api: api, vehicleId: 'v1', now: () => _now))),
    );

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

Finder _dlg(String text) => find.descendant(
    of: find.byType(AlertDialog),
    matching: find.widgetWithText(FilledButton, text));
Finder _dlgField(String label) => find.descendant(
    of: find.byType(AlertDialog),
    matching: find.widgetWithText(TextField, label));

void main() {
  group('buildTripBody', () {
    ({Map<String, dynamic>? body, String? error}) build({
      String? employeeId = 'e1',
      String datum = '02.03.2026',
      String beginn = '1000',
      String ende = '1050',
      String fahrtart = 'dienstlich',
      String ziel = ' Augsburg ',
      String zweck = ' Aufmaß ',
    }) =>
        buildTripBody(
          vehicleId: 'v1',
          employeeId: employeeId,
          datum: datum,
          kmBeginn: beginn,
          kmEnde: ende,
          fahrtart: fahrtart,
          ziel: ziel,
          zweck: zweck,
          geschaeftspartner: ' Kunde ',
          notiz: ' Stau ',
        );

    test('gueltig', () {
      expect(build().body, {
        'vehicle_id': 'v1',
        'employee_id': 'e1',
        'datum': '2026-03-02',
        'km_beginn': 1000,
        'km_ende': 1050,
        'fahrtart': 'dienstlich',
        'ziel': 'Augsburg',
        'zweck': 'Aufmaß',
        'geschaeftspartner': 'Kunde',
        'notiz': 'Stau',
      });
      expect(build(employeeId: null).body!.containsKey('employee_id'), isFalse);
      expect(build(fahrtart: 'privat', ziel: '', zweck: '').error, isNull);
      expect(build(beginn: ' 0 ', ende: '1').body!['km_beginn'], 0);
    });

    test('abgelehnt', () {
      // Liste statt Map: gleiche Ergebnisse waeren als Schluessel
      // zusammengefallen.
      final cases = [
        (build(datum: '2.3.26'), 'Bitte das Datum als TT.MM.JJJJ angeben.'),
        (
          build(beginn: 'abc'),
          'Kilometerstand Beginn muss eine ganze Zahl ab 0 sein.'
        ),
        (
          build(beginn: '-1'),
          'Kilometerstand Beginn muss eine ganze Zahl ab 0 sein.'
        ),
        (
          build(ende: ''),
          'Kilometerstand Ende muss eine ganze Zahl ab 0 sein.'
        ),
        (
          build(ende: '-5', beginn: '0'),
          'Kilometerstand Ende muss eine ganze Zahl ab 0 sein.'
        ),
        (
          build(ende: '1000'),
          'Kilometerstand Ende muss über dem Beginn liegen.'
        ),
        (build(fahrtart: 'urlaub'), 'Bitte eine Fahrtart wählen.'),
        (build(ziel: ' '), 'Reiseziel ist bei Dienstfahrten erforderlich.'),
        (build(zweck: ''), 'Reisezweck ist bei Dienstfahrten erforderlich.'),
      ];
      for (final (r, msg) in cases) {
        expect(r.error, msg);
        expect(r.body, isNull);
      }
    });
  });

  test('formatKm und tripLine', () {
    expect(formatKm(0), '0');
    expect(formatKm(999), '999');
    expect(formatKm(1000), '1.000');
    expect(formatKm(1234567), '1.234.567');
    final t = _trips();
    expect(tripLine(t[1]),
        '01.10.2026 · 12.000–12.050 km (50 km) · Dienstfahrt · Anna Berg · Augsburg, Aufmaß, Kunde A');
    expect(tripLine(t[0]),
        '02.10.2026 · 12.050–12.100 km (50 km) · Privatfahrt · Bert Cord');
    expect(
        tripLine({...t[0], 'status': 'storniert', 'storno_reason': 'doppelt'}),
        'STORNIERT: 02.10.2026 · 12.050–12.100 km (50 km) · Privatfahrt · Bert Cord (doppelt)');
  });

  testWidgets('nur lesen: Stand, Fahrten, Luecke, keine Aktionen',
      (tester) async {
    await _desktop(tester);
    final trips = _trips()..[0]['km_beginn'] = 12070;
    final api = _FakeApi(permissions: const {'fleet.read'}, trips: trips);
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(api.calls, ['list v1 true', 'chain v1']);
    expect(find.text('km-Stand: 12.100 km'), findsOneWidget);
    expect(find.text('⚠ Lücke im km-Stand: 12.050–12.070 km'), findsOneWidget);
    expect(find.textContaining('Anna Berg · Augsburg'), findsOneWidget);
    expect(find.text('Fahrt erfassen'), findsNothing);
    expect(find.byTooltip('Fahrt stornieren'), findsNothing);
  });

  testWidgets(
      'erfassen mit fleet.write: Fahrer Pflicht, km-Stand vorbelegt, Aufruf',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, trips: _trips());
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(find.textContaining('Lücke'), findsNothing);
    await tester.tap(find.text('Fahrt erfassen'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(_dlgField('km Beginn')).controller!.text,
        '12100');
    expect(
        tester
            .widget<TextField>(_dlgField('Datum (TT.MM.JJJJ)'))
            .controller!
            .text,
        '19.10.2026');
    await tester.enterText(_dlgField('km Ende'), '12180');
    await tester.tap(_dlg('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte einen Fahrer auswählen.'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('tripEmployee')));
    await tester.pumpAndSettle();
    expect(find.text('Alt, Otto'), findsNothing);
    await tester.tap(find.text('Berg, Anna').last);
    await tester.pumpAndSettle();
    await tester.tap(_dlg('Erfassen'));
    await tester.pumpAndSettle();
    expect(find.text('Reiseziel ist bei Dienstfahrten erforderlich.'),
        findsOneWidget);
    expect(api.recorded, isEmpty);

    await tester.enterText(_dlgField('Reiseziel'), 'Ulm');
    await tester.enterText(_dlgField('Reisezweck'), 'Montage');
    await tester.tap(_dlg('Erfassen'));
    await tester.pumpAndSettle();
    expect(api.recorded.single['employee_id'], 'e1');
    expect(api.recorded.single['km_beginn'], 12100);
    expect(api.recorded.single['km_ende'], 12180);
    expect(api.recorded.single['datum'], '2026-10-19');
    expect(api.recorded.single['ziel'], 'Ulm');
    expect(find.byType(AlertDialog), findsNothing);
    expect(find.text('Fahrt erfasst'), findsOneWidget);
    expect(find.text('km-Stand: 12.180 km'), findsOneWidget);
  });

  testWidgets(
      'erfassen nur mit fleet.self: ohne Fahrerauswahl, Privatfahrt ohne Ziel',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'fleet.self'});
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(find.text('Noch keine Fahrt erfasst'), findsOneWidget);
    await tester.tap(find.text('Fahrt erfassen'));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('tripEmployee')), findsNothing);
    expect(
        tester.widget<TextField>(_dlgField('km Beginn')).controller!.text, '');
    await tester.tap(find.byKey(const ValueKey('tripFahrtart')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Privatfahrt').last);
    await tester.pumpAndSettle();
    expect(_dlgField('Reiseziel (optional)'), findsOneWidget);
    await tester.enterText(_dlgField('km Beginn'), '0');
    await tester.enterText(_dlgField('km Ende'), '15');
    await tester.tap(_dlg('Erfassen'));
    await tester.pumpAndSettle();
    expect(api.recorded.single.containsKey('employee_id'), isFalse);
    expect(api.recorded.single['fahrtart'], 'privat');
    expect(find.byTooltip('Fahrt stornieren'), findsNothing);
  });

  testWidgets('Ablehnung des Servers erscheint als Meldung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
      permissions: const {'fleet.self'},
      trips: _trips(),
      recordError: const ApiException(
          statusCode: 400,
          message:
              'Kilometerstand Beginn muss 12100 km sein (Ende der vorigen Fahrt)'),
    );
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fahrt erfassen'));
    await tester.pumpAndSettle();
    await tester.enterText(_dlgField('km Beginn'), '12110');
    await tester.enterText(_dlgField('km Ende'), '12180');
    await tester.tap(find.byKey(const ValueKey('tripFahrtart')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Arbeitsweg').last);
    await tester.pumpAndSettle();
    await tester.tap(_dlg('Erfassen'));
    await tester.pumpAndSettle();
    expect(api.recorded.single['fahrtart'], 'arbeitsweg');
    expect(
        find.text(
            'Fehler: Kilometerstand Beginn muss 12100 km sein (Ende der vorigen Fahrt)'),
        findsOneWidget);
    expect(find.text('km-Stand: 12.100 km'), findsOneWidget);
  });

  testWidgets('stornieren: Begruendung Pflicht, danach als storniert gezeigt',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _rw, trips: _trips());
    await tester.pumpWidget(_section(api));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Fahrt stornieren'), findsNWidgets(2));
    await tester.tap(find.byTooltip('Fahrt stornieren').first);
    await tester.pumpAndSettle();
    expect(tester.widget<FilledButton>(_dlg('Stornieren')).onPressed, isNull);
    await tester.enterText(
        find.descendant(
            of: find.byType(AlertDialog), matching: find.byType(TextField)),
        ' doppelt ');
    await tester.pump();
    await tester.tap(_dlg('Stornieren'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('storno t2 doppelt'));
    expect(find.text('Fahrt storniert'), findsOneWidget);
    expect(find.textContaining('STORNIERT: 02.10.2026'), findsOneWidget);
    expect(find.byTooltip('Fahrt stornieren'), findsOneWidget);
    expect(find.text('km-Stand: 12.050 km'), findsOneWidget);
  });

  testWidgets('Detailseite zeigt das Fahrtenbuch nur, wenn es gefuehrt wird',
      (tester) async {
    await _desktop(tester);
    for (final fb in [true, false]) {
      final api = _FakeApi(permissions: const {'fleet.read'}, trips: _trips());
      await tester.pumpWidget(MaterialApp(
        key: UniqueKey(),
        home: VehicleDetailPage(api: api, vehicle: {
          'id': 'v1',
          'kennzeichen': 'M-FB 1',
          'art': 'pkw',
          'bezeichnung': '',
          'abgemeldet_am': null,
          'fahrtenbuch': fb
        }),
      ));
      await tester.pumpAndSettle();
      expect(find.text('Fahrtenbuch'), fb ? findsOneWidget : findsNothing,
          reason: '$fb');
      expect(api.calls.contains('chain v1'), fb, reason: '$fb');
    }
  });
}
