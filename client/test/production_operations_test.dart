import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/production_operations_section.dart';
import 'package:nalaerp_client/pages/production_orders_page.dart';

// Arbeitsgänge im Detail des Fertigungsauftrags (Backlog H.2.5b).

Map<String, dynamic> _op(String id, int pos, String name,
        {String wc = 'wc1',
        String? woche,
        num plan = 70,
        List<String> hinweise = const []}) =>
    {
      'id': id,
      'pos': pos,
      'name': name,
      'work_center_id': wc,
      'work_center_name': wc == 'wc1' ? 'Zuschnitt' : 'Montage',
      'ruestzeit_min': 30,
      'minuten_je_stueck': 10,
      'geplante_woche': woche,
      'plan_minuten': plan,
      'hinweise': hinweise,
    };

final _centers = [
  {'id': 'wc1', 'name': 'Zuschnitt', 'stunden_je_tag': 8, 'aktiv': true},
  {'id': 'wc2', 'name': 'Montage', 'stunden_je_tag': 8, 'aktiv': false},
];

class _FakeApi extends ApiClient {
  _FakeApi(
      {required this.permissions,
      List<dynamic>? ops,
      this.fail,
      this.sollIst,
      this.sollIstFails = false})
      : ops = ops ?? [],
        super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  List<dynamic> ops;
  final String? fail;
  final Map<String, dynamic>? sollIst;
  final bool sollIstFails;
  final List<String> calls = [];

  @override
  Future<Map<String, dynamic>> getProductionSollIst(String orderId) async {
    calls.add('sollist $orderId');
    if (sollIstFails) throw ApiException(statusCode: 500, message: 'kaputt');
    return sollIst ??
        {
          'arbeitsgaenge': ops.length,
          'erledigt': 0,
          'soll_min': 0,
          'mit_ist': 0
        };
  }

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  // Rueckmeldung: setzt erledigt am Arbeitsgang, damit das Neuladen ihn
  // als erledigt bzw. offen zeigt.
  void _setDone(String opId, bool done) => ops = [
        for (final o in ops)
          (o as Map)['id'] == opId
              ? {
                  ...o as Map<String, dynamic>,
                  'erledigt': done,
                  'erledigt_at': done ? '2026-11-12T09:30:00Z' : null
                }
              : o
      ];

  @override
  Future<Map<String, dynamic>> reportProductionOperationDone(
      String orderId, String opId, Map<String, dynamic> body) async {
    calls.add('report $orderId $opId ${jsonEncode(body)}');
    if (fail != null) throw ApiException(statusCode: 409, message: fail!);
    _setDone(opId, true);
    return {'id': opId};
  }

  @override
  Future<Map<String, dynamic>> revertProductionOperationDone(
      String orderId, String opId, String reason) async {
    calls.add('revert $orderId $opId $reason');
    if (fail != null) throw ApiException(statusCode: 409, message: fail!);
    _setDone(opId, false);
    return {'id': opId};
  }

  @override
  Future<List<dynamic>> listProductionOperations(String orderId) async {
    calls.add('list $orderId');
    return ops;
  }

  @override
  Future<List<dynamic>> suggestProductionOperations(String orderId) async {
    calls.add('suggest $orderId');
    if (fail != null) throw ApiException(statusCode: 409, message: fail!);
    ops = [_op('a', 1, 'Sägen', woche: '2026-11-23T00:00:00Z')];
    return ops;
  }

  @override
  Future<Map<String, dynamic>> addProductionOperation(
      String orderId, Map<String, dynamic> body) async {
    calls.add('add $orderId ${jsonEncode(body)}');
    if (fail != null) throw ApiException(statusCode: 409, message: fail!);
    ops = [...ops, _op('n', ops.length + 1, body['name'] as String)];
    return ops.last as Map<String, dynamic>;
  }

  @override
  Future<Map<String, dynamic>> updateProductionOperation(
      String orderId, String opId, Map<String, dynamic> body) async {
    calls.add('update $orderId $opId ${jsonEncode(body)}');
    return {'id': opId, ...body};
  }

  @override
  Future<void> deleteProductionOperation(String orderId, String opId) async {
    calls.add('delete $orderId $opId');
    ops = [
      for (final o in ops)
        if ((o as Map)['id'] != opId) o
    ];
  }

  @override
  Future<List<dynamic>> listWorkCenters({bool includeInactive = false}) async {
    calls.add('centers $includeInactive');
    return _centers;
  }

  @override
  Future<Map<String, dynamic>> getProductionOrder(String id) async => {
        'id': id,
        'nummer': 'FA-2026-0001',
        'project_nummer': 'P-1',
        'project_name': 'Testprojekt',
        'stueckzahl': 4,
        'status': 'freigegeben',
        'position_nummer': '3',
        'position_name': 'Fenster',
        'variante_name': 'links',
        'created_at': '2026-11-02T08:00:00Z',
        'freigegeben_at': '2026-11-03T08:00:00Z',
        'hinweise': const [],
        'items': const [],
      };
}

Widget _wrap(Widget child, {Key? key}) => MaterialApp(
    key: key,
    theme:
        ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory),
    home: Scaffold(body: SingleChildScrollView(child: child)));

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

const _write = {'production.read', 'production.write'};

void main() {
  test('Kalenderwoche und Zeilen', () {
    expect(isoWeekNumber(DateTime(2026, 11, 23)), 48);
    expect(isoWeekNumber(DateTime(2026, 12, 28)), 53);
    expect(isoWeekNumber(DateTime(2027, 1, 4)), 1);
    expect(isoWeekNumber(DateTime(2027, 1, 1)), 53);
    expect(operationWeekLabel('2026-11-23T00:00:00Z'), 'KW 48 (ab 23.11.2026)');
    expect(operationWeekLabel(null), 'ungeplant');
    expect(operationWeekLabel('kaputt'), 'ungeplant');
    expect(operationLine(_op('a', 1, 'Sägen', woche: '2026-11-23T00:00:00Z')),
        'Zuschnitt · KW 48 (ab 23.11.2026) · 1 h 10 min');
    expect(operationLine(_op('a', 1, 'Sägen', plan: 45)),
        'Zuschnitt · ungeplant · 45 min');
    expect(
        operationsTotal(
            [_op('a', 1, 'x', plan: 70), _op('b', 2, 'y', plan: 50)]),
        120);
    expect(operationsTotal(const []), 0);
    expect(operationsEditable('entwurf'), isTrue);
    expect(operationsEditable('in_arbeit'), isTrue);
    expect(operationsEditable('fertig'), isFalse);
    expect(operationsEditable('storniert'), isFalse);
    expect(operationsEditable(null), isFalse);
  });

  test('Rückmeldung und Soll/Ist als Zeile', () {
    final done = {
      ..._op('a', 1, 'Sägen'),
      'erledigt': true,
      'erledigt_at': '2026-11-12T09:30:00Z',
      'ist_minuten': 95.46,
      'rueckmeldung_notiz': 'Rest morgen',
    };
    expect(operationDoneLine(done),
        'Erledigt am 12.11.2026 · Ist 1 h 35 min · Rest morgen');
    expect(
        operationDoneLine(
            {...done, 'ist_minuten': null, 'rueckmeldung_notiz': ''}),
        'Erledigt am 12.11.2026');
    expect(operationDoneLine(_op('a', 1, 'Sägen')), isNull);
    expect(operationDoneLine({...done, 'erledigt': false}), isNull);

    final s = {
      'arbeitsgaenge': 3,
      'erledigt': 2,
      'soll_min': 210,
      'soll_erledigt_min': 140,
      'mit_ist': 1,
      'soll_mit_ist_min': 70,
      'ist_min': 95.46,
    };
    expect(sollIstLine(s),
        'Erledigt 2 von 3 · Soll erledigt 2 h 20 min von 3 h 30 min · Ist 1 h 35 min zu Soll 1 h 10 min (1 Arbeitsgang mit Ist-Zeit)');
    expect(sollIstLine({...s, 'mit_ist': 2, 'soll_mit_ist_min': 140}),
        endsWith('zu Soll 2 h 20 min (2 Arbeitsgänge mit Ist-Zeit)'));
    expect(sollIstLine({...s, 'mit_ist': 0}),
        'Erledigt 2 von 3 · Soll erledigt 2 h 20 min von 3 h 30 min');
    expect(sollIstLine({...s, 'erledigt': 0}), isNull);
    expect(sollIstLine(null), isNull);
  });

  testWidgets('Erledigte mit Haken und Rückmeldung, Soll/Ist im Kopf',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {
      'production.read'
    }, ops: [
      {
        ..._op('a', 1, 'Sägen'),
        'erledigt': true,
        'erledigt_at': '2026-11-12T09:30:00Z',
        'ist_minuten': 80,
        'rueckmeldung_notiz': '',
      },
      _op('b', 2, 'Verglasung'),
    ], sollIst: {
      'arbeitsgaenge': 2,
      'erledigt': 1,
      'soll_min': 140,
      'soll_erledigt_min': 70,
      'mit_ist': 1,
      'soll_mit_ist_min': 70,
      'ist_min': 80,
    });
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'in_arbeit')));
    await tester.pumpAndSettle();
    expect(api.calls, containsAll(['list o1', 'sollist o1']));
    expect(
        find.text('Erledigt am 12.11.2026 · Ist 1 h 20 min'), findsOneWidget);
    expect(find.byIcon(Icons.check_circle), findsOneWidget);
    expect(
        find.text(
            'Erledigt 1 von 2 · Soll erledigt 1 h 10 min von 2 h 20 min · Ist 1 h 20 min zu Soll 1 h 10 min (1 Arbeitsgang mit Ist-Zeit)'),
        findsOneWidget);
  });

  testWidgets('Soll/Ist nicht ladbar: Liste bleibt, keine Zeile',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
        permissions: const {'production.read'},
        ops: [_op('a', 1, 'Sägen')],
        sollIstFails: true);
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'in_arbeit')));
    await tester.pumpAndSettle();
    expect(find.text('1. Sägen'), findsOneWidget);
    expect(find.textContaining('Erledigt'), findsNothing);
    expect(find.textContaining('konnten nicht geladen'), findsNothing);
    expect(find.byIcon(Icons.check_circle), findsNothing);
  });

  test('buildOperationBody', () {
    final ok = buildOperationBody(
        name: ' Sägen ',
        workCenterId: 'wc1',
        ruest: '30',
        jeStueck: '4,5',
        woche: '25.11.2026');
    expect(ok.error, isNull);
    expect(ok.body, {
      'name': 'Sägen',
      'work_center_id': 'wc1',
      'ruestzeit_min': 30.0,
      'minuten_je_stueck': 4.5,
      'geplante_woche': '2026-11-25'
    });
    expect(
        buildOperationBody(name: 'x', workCenterId: 'wc1')
            .body!['geplante_woche'],
        '');
    expect(buildOperationBody(name: ' ', workCenterId: 'wc1').error,
        'Bitte einen Namen angeben.');
    expect(buildOperationBody(name: 'x', workCenterId: null).error,
        'Bitte einen Arbeitsplatz wählen.');
    expect(
        buildOperationBody(name: 'x', workCenterId: 'wc1', ruest: '-1').error,
        'Zeiten: Minuten, nicht negativ.');
    expect(
        buildOperationBody(name: 'x', workCenterId: 'wc1', woche: '31.02.2026')
            .error,
        'Bitte die Woche als Datum TT.MM.JJJJ angeben.');
  });

  testWidgets('Liste mit Planzeit und Hinweis, nur lesen ohne Schaltflächen',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {
      'production.read'
    }, ops: [
      _op('a', 1, 'Sägen', woche: '2026-11-23T00:00:00Z'),
      _op('b', 2, 'Endkontrolle',
          woche: '2026-12-07T00:00:00Z',
          plan: 8,
          hinweise: const ['nach_liefertermin']),
    ]);
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'freigegeben')));
    await tester.pumpAndSettle();
    expect(find.text('1. Sägen'), findsOneWidget);
    expect(find.text('Planzeit gesamt 1 h 18 min'), findsOneWidget);
    expect(find.text('Geplant nach dem Liefertermin'), findsOneWidget);
    expect(find.text('Arbeitsgang hinzufügen'), findsNothing);
    expect(find.byTooltip('Arbeitsgang bearbeiten'), findsNothing);
  });

  test('Rückmeldung: Eingaben prüfen', () {
    expect(feedbackAllowed('in_arbeit'), isTrue);
    for (final st in ['entwurf', 'freigegeben', 'fertig', 'storniert', null]) {
      expect(feedbackAllowed(st), isFalse, reason: '$st');
    }
    expect(buildFeedbackBody(ist: ' 95,5 ', notiz: ' Rest morgen ').body,
        {'notiz': 'Rest morgen', 'ist_minuten': 95.5});
    expect(buildFeedbackBody().body, {'notiz': ''});
    expect(buildFeedbackBody(ist: '100000').body!['ist_minuten'], 100000);
    expect(buildFeedbackBody(notiz: 'ä' * 500).error, isNull);
    for (final ist in ['0', '-1', '100000,01', 'abc', 'NaN', 'Infinity']) {
      expect(buildFeedbackBody(ist: ist).error,
          'Ist-Zeit: Minuten größer als 0, höchstens 100.000.',
          reason: ist);
    }
    expect(buildFeedbackBody(notiz: 'ä' * 501).error,
        'Notiz: höchstens 500 Zeichen.');
    expect(feedbackRevertReasonError(' versehentlich '), isNull);
    expect(feedbackRevertReasonError('x' * 500), isNull);
    expect(feedbackRevertReasonError('  '), 'Bitte eine Begründung angeben.');
    expect(feedbackRevertReasonError('x' * 501),
        'Begründung: höchstens 500 Zeichen.');
  });

  testWidgets('Erledigt melden mit Prüfung, danach zurücknehmen',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _write, ops: [_op('a', 1, 'Sägen')]);
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'in_arbeit')));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Rückmeldung zurücknehmen'), findsNothing);
    expect(find.byTooltip('Arbeitsgang entfernen'), findsOneWidget);

    await tester.tap(find.byTooltip('Erledigt melden'));
    await tester.pumpAndSettle();
    final fields = find.descendant(
        of: find.byType(AlertDialog), matching: find.byType(TextField));
    await tester.enterText(fields.at(0), '0');
    await tester.tap(find.widgetWithText(FilledButton, 'Erledigt melden'));
    await tester.pumpAndSettle();
    expect(find.text('Ist-Zeit: Minuten größer als 0, höchstens 100.000.'),
        findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('report')), isEmpty);
    await tester.enterText(fields.at(0), '95,5');
    await tester.enterText(fields.at(1), ' Rest morgen ');
    await tester.tap(find.widgetWithText(FilledButton, 'Erledigt melden'));
    await tester.pumpAndSettle();
    expect(api.calls,
        contains('report o1 a {"notiz":"Rest morgen","ist_minuten":95.5}'));
    expect(find.text('Arbeitsgang erledigt gemeldet'), findsOneWidget);
    expect(api.calls.where((c) => c == 'sollist o1'), hasLength(2));
    expect(find.byIcon(Icons.check_circle), findsOneWidget);
    expect(find.byTooltip('Erledigt melden'), findsNothing);
    expect(find.byTooltip('Arbeitsgang entfernen'), findsNothing);
    ScaffoldMessenger.of(tester.element(find.byType(Scaffold)))
        .removeCurrentSnackBar();
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Rückmeldung zurücknehmen'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Zurücknehmen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte eine Begründung angeben.'), findsOneWidget);
    await tester.enterText(
        find.descendant(
            of: find.byType(AlertDialog), matching: find.byType(TextField)),
        ' versehentlich ');
    await tester.tap(find.widgetWithText(FilledButton, 'Zurücknehmen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('revert o1 a versehentlich'));
    expect(find.text('Rückmeldung zurückgenommen'), findsOneWidget);
    expect(find.byIcon(Icons.check_circle), findsNothing);
    expect(find.byTooltip('Erledigt melden'), findsOneWidget);
  });

  testWidgets('Melden: Abbrechen ohne Aufruf, Serverfehler als Meldung',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
        permissions: _write,
        ops: [_op('a', 1, 'Sägen')],
        fail: 'Rückmeldung nur bei begonnenen Aufträgen möglich');
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'in_arbeit')));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Erledigt melden'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('report')), isEmpty);
    await tester.tap(find.byTooltip('Erledigt melden'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Erledigt melden'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('report o1 a {"notiz":""}'));
    expect(
        find.text('Fehler: Rückmeldung nur bei begonnenen Aufträgen möglich'),
        findsOneWidget);
    expect(find.byIcon(Icons.check_circle), findsNothing);
  });

  testWidgets('Melden nur in Arbeit und mit Schreibrecht', (tester) async {
    await _desktop(tester);
    final done = {
      ..._op('a', 1, 'Sägen'),
      'erledigt': true,
      'erledigt_at': '2026-11-12T09:30:00Z'
    };
    for (final (perms, status) in [
      (_write, 'freigegeben'),
      (_write, 'fertig'),
      (const {'production.read'}, 'in_arbeit'),
    ]) {
      final api =
          _FakeApi(permissions: perms, ops: [_op('b', 2, 'Glas'), done]);
      await tester.pumpWidget(_wrap(
          ProductionOperationsSection(api: api, orderId: 'o1', status: status),
          key: UniqueKey()));
      await tester.pumpAndSettle();
      expect(find.byTooltip('Erledigt melden'), findsNothing, reason: status);
      expect(find.byTooltip('Rückmeldung zurücknehmen'), findsNothing,
          reason: status);
    }
  });

  testWidgets('Fertig: nur lesen auch mit Schreibrecht', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _write, ops: [_op('a', 1, 'Sägen')]);
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'fertig')));
    await tester.pumpAndSettle();
    expect(find.text('1. Sägen'), findsOneWidget);
    expect(find.text('Arbeitsgang hinzufügen'), findsNothing);
    expect(find.byTooltip('Arbeitsgang entfernen'), findsNothing);
  });

  testWidgets('Vorschlag nur ohne Arbeitsgänge, Fehler als Meldung',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _write);
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'entwurf')));
    await tester.pumpAndSettle();
    expect(find.text('Noch keine Arbeitsgänge.'), findsOneWidget);
    await tester.tap(find.text('Aus Vorlagen vorschlagen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('suggest o1'));
    expect(find.text('1. Sägen'), findsOneWidget);
    expect(find.text('Aus Vorlagen vorschlagen'), findsNothing);

    final failing = _FakeApi(
        permissions: _write,
        fail: 'Keine aktiven Arbeitsgangvorlagen vorhanden');
    await tester.pumpWidget(_wrap(
        ProductionOperationsSection(
            api: failing, orderId: 'o2', status: 'entwurf'),
        key: const ValueKey('f')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Aus Vorlagen vorschlagen'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Keine aktiven Arbeitsgangvorlagen vorhanden'),
        findsOneWidget);
  });

  testWidgets('Hinzufügen mit Prüfung, nur aktive Arbeitsplätze',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _write, ops: [_op('a', 1, 'Sägen')]);
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'in_arbeit')));
    await tester.pumpAndSettle();
    expect(find.text('Aus Vorlagen vorschlagen'), findsNothing);
    await tester.tap(find.text('Arbeitsgang hinzufügen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('centers true'));
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
        find.widgetWithText(
            TextField, 'Geplante Woche (ein Tag als TT.MM.JJJJ)'),
        'morgen');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte die Woche als Datum TT.MM.JJJJ angeben.'),
        findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('add')), isEmpty);
    await tester.enterText(
        find.widgetWithText(
            TextField, 'Geplante Woche (ein Tag als TT.MM.JJJJ)'),
        '25.11.2026');
    await tester.enterText(
        find.widgetWithText(TextField, 'Minuten je Stück'), '2,5');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(
        api.calls,
        contains(
            'add o1 {"name":"Bohren","work_center_id":"wc1","ruestzeit_min":0.0,"minuten_je_stueck":2.5,"geplante_woche":"2026-11-25"}'));
    expect(find.text('2. Bohren'), findsOneWidget);
    expect(find.text('Arbeitsgang angelegt'), findsOneWidget);
  });

  testWidgets(
      'Ändern auf deaktiviertem Arbeitsplatz, Woche leeren; Entfernen nur nach Rückfrage',
      (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _write, ops: [
      _op('a', 1, 'Verglasung', wc: 'wc2', woche: '2026-11-23T00:00:00Z')
    ]);
    await tester.pumpWidget(_wrap(ProductionOperationsSection(
        api: api, orderId: 'o1', status: 'freigegeben')));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Arbeitsgang bearbeiten'));
    await tester.pumpAndSettle();
    expect(find.text('Montage (deaktiviert)'), findsOneWidget);
    final woche = tester.widget<TextField>(find.widgetWithText(
        TextField, 'Geplante Woche (ein Tag als TT.MM.JJJJ)'));
    expect(woche.controller!.text, '23.11.2026');
    await tester.enterText(
        find.widgetWithText(
            TextField, 'Geplante Woche (ein Tag als TT.MM.JJJJ)'),
        '');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(
        api.calls,
        contains(
            'update o1 a {"name":"Verglasung","work_center_id":"wc2","ruestzeit_min":30.0,"minuten_je_stueck":10.0,"geplante_woche":""}'));

    await tester.tap(find.byTooltip('Arbeitsgang entfernen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('delete')), isEmpty);
    await tester.tap(find.byTooltip('Arbeitsgang entfernen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Entfernen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('delete o1 a'));
    expect(find.text('Noch keine Arbeitsgänge.'), findsOneWidget);
  });

  testWidgets('Detail des Auftrags zeigt den Abschnitt', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: _write, ops: [_op('a', 1, 'Sägen')]);
    await tester.pumpWidget(
        MaterialApp(home: ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(find.text('Arbeitsgänge'), findsOneWidget);
    expect(find.text('1. Sägen'), findsOneWidget);
    expect(api.calls, contains('list o1'));
  });

  group('API-Methoden', () {
    final api = ApiClient(baseUrl: 'http://api.test');
    late List<http.Request> calls;

    Future<T> mock<T>(Object? body, Future<T> Function() call,
        {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(
            body == null ? const [] : utf8.encode(jsonEncode(body)), status,
            headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    setUp(() => calls = []);

    test('Pfade und Bodies', () async {
      await mock([], () => api.listProductionOperations('o1'));
      await mock([], () => api.suggestProductionOperations('o1'), status: 201);
      await mock(
          {'id': 'a'}, () => api.addProductionOperation('o1', {'name': 'x'}),
          status: 201);
      await mock({'id': 'a'},
          () => api.updateProductionOperation('o1', 'a', {'name': 'y'}));
      await mock(null, () => api.deleteProductionOperation('o1', 'a'),
          status: 204);
      expect(calls.map((c) => '${c.method} ${c.url.path}'), [
        'GET /api/v1/production/orders/o1/operations',
        'POST /api/v1/production/orders/o1/operations/vorschlagen',
        'POST /api/v1/production/orders/o1/operations',
        'PUT /api/v1/production/orders/o1/operations/a',
        'DELETE /api/v1/production/orders/o1/operations/a',
      ]);
      expect(calls[2].body, '{"name":"x"}');
      expect(calls[3].body, '{"name":"y"}');
    });

    test('Rückmeldung und Soll/Ist: Pfade, Bodies, Status', () async {
      final x = await mock(
          {'id': 'a', 'erledigt': true},
          () => api
              .reportProductionOperationDone('o1', 'a', {'ist_minuten': 95.5}));
      await mock({'id': 'a'},
          () => api.revertProductionOperationDone('o1', 'a', 'versehentlich'));
      final s =
          await mock({'erledigt': 1}, () => api.getProductionSollIst('o1'));
      expect(calls.map((c) => '${c.method} ${c.url.path}'), [
        'POST /api/v1/production/orders/o1/operations/a/erledigt',
        'POST /api/v1/production/orders/o1/operations/a/zuruecknehmen',
        'GET /api/v1/production/orders/o1/soll-ist',
      ]);
      expect(calls[0].body, '{"ist_minuten":95.5}');
      expect(calls[1].body, '{"reason":"versehentlich"}');
      expect(x['erledigt'], isTrue);
      expect(s['erledigt'], 1);
      // Melden erwartet 200; 201 und 409 sind Fehler.
      await expectLater(
          mock({'id': 'a'},
              () => api.reportProductionOperationDone('o1', 'a', {}),
              status: 201),
          throwsA(isA<ApiException>()));
      await expectLater(
          mock({
            'error': {
              'code': 'conflict',
              'message': 'Arbeitsgang ist bereits erledigt gemeldet'
            }
          }, () => api.reportProductionOperationDone('o1', 'a', {}),
              status: 409),
          throwsA(isA<ApiException>().having((e) => e.message, 'message',
              'Arbeitsgang ist bereits erledigt gemeldet')));
    });

    test('Fehler als ApiException', () async {
      await expectLater(mock([], () => api.suggestProductionOperations('o1')),
          throwsA(isA<ApiException>()));
      await expectLater(
          mock({
            'error': {'code': 'conflict', 'message': 'gesperrt'}
          }, () => api.deleteProductionOperation('o1', 'a'), status: 409),
          throwsA(isA<ApiException>()
              .having((e) => e.message, 'message', 'gesperrt')));
      await expectLater(
          mock({'id': 'a'}, () => api.addProductionOperation('o1', {})),
          throwsA(isA<ApiException>()));
    });
  });
}
