import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/production_withdrawals_section.dart';

// Entnahmen im Detail des Fertigungsauftrags (Backlog H.3.4b).

Map<String, dynamic> _order({String status = 'in_arbeit', String? lager = 'wh1'}) => {
      'id': 'o1',
      'nummer': 'FA-2026-0001',
      'status': status,
      'reservierung_lager_id': lager,
      'items': [
        {'id': 'i1', 'pos': 1, 'article_code': 'A-7', 'description': 'Griff', 'material_id': 'm1', 'material_einheit': 'Stk', 'offen': 2},
        {'id': 'i2', 'pos': 2, 'article_code': '', 'description': 'Isolierglas', 'material_id': null},
        {
          'id': 'i3',
          'pos': 3,
          'article_code': 'P-100',
          'description': 'Blendrahmen',
          'material_id': 'm3',
          'material_einheit': 'm',
          'offen': 0
        },
      ],
    };

final _warehouses = [
  {'id': 'wh1', 'code': 'HL', 'name': 'Halle'},
  {'id': 'wh2', 'code': 'AL', 'name': 'Außenlager'},
];

Map<String, dynamic> _w({String id = 'w1', bool storniert = false, String notiz = ''}) => {
      'id': id,
      'item_pos': 1,
      'material_nummer': 'mat-1',
      'material_bezeichnung': 'Griff',
      'warehouse_id': 'wh1',
      'menge': 2,
      'einheit': 'Stk',
      'notiz': notiz,
      'created_at': '2026-11-10T08:00:00Z',
      if (storniert) ...{'storniert_at': '2026-11-11T08:00:00Z', 'storno_reason': 'falsch gebucht'},
    };

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, this.list = const [], this.fail, this.hints = const []}) : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  List<dynamic> list;
  final String? fail;
  final List<String> hints;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listProductionWithdrawals(String orderId) async {
    calls.add('list $orderId');
    return list;
  }

  @override
  Future<List<dynamic>> listLocations(String warehouseId) async {
    calls.add('locations $warehouseId');
    return warehouseId == 'wh1'
        ? [
            {'id': 'l1', 'code': 'R1', 'name': 'Regal 1'}
          ]
        : const [];
  }

  @override
  Future<Map<String, dynamic>> createProductionWithdrawal(String orderId, Map<String, dynamic> body) async {
    calls.add('create $orderId ${jsonEncode(body)}');
    if (fail != null) throw ApiException(statusCode: 409, message: fail!);
    final w = {..._w(id: 'w9'), 'hinweise': hints};
    list = [w, ...list];
    return w;
  }

  @override
  Future<Map<String, dynamic>> stornoProductionWithdrawal(String orderId, String withdrawalId, String reason) async {
    calls.add('storno $orderId $withdrawalId $reason');
    if (fail != null) throw ApiException(statusCode: 409, message: fail!);
    return {..._w(id: withdrawalId, storniert: true), 'hinweise': <String>[]};
  }
}

Future<int> _pump(WidgetTester tester, _FakeApi api, Map<String, dynamic> order, {List<dynamic> warehouses = const []}) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  var changed = 0;
  await tester.pumpWidget(MaterialApp(
    home: Scaffold(
      body: SingleChildScrollView(
        child: ProductionWithdrawalsSection(api: api, order: order, warehouses: warehouses, onChanged: () async => changed++),
      ),
    ),
  ));
  await tester.pumpAndSettle();
  return changed;
}

void main() {
  test('Auswahl der Zeilen und Vorschlag der Menge', () {
    final c = withdrawalItemChoices(_order());
    expect(c.map((x) => x.id), ['i1', 'i3']);
    expect(c.map((x) => x.label), ['Pos. 1 A-7 Griff · offen 2 Stk', 'Pos. 3 P-100 Blendrahmen · offen 0 m']);
    expect(withdrawalItemChoices({'items': null}), isEmpty);
    expect(withdrawalDefaultQty(_order(), 'i1'), '2');
    // Alles entnommen, ohne Material oder unbekannt: kein Vorschlag.
    expect(withdrawalDefaultQty(_order(), 'i3'), '');
    expect(withdrawalDefaultQty(_order(), 'i2'), '');
    expect(withdrawalDefaultQty(_order(), null), '');
  });

  test('buildWithdrawalBody', () {
    final ok = buildWithdrawalBody(itemId: 'i1', warehouseId: 'wh1', locationId: 'l1', menge: '2,5', notiz: ' Rest ');
    expect(ok.error, isNull);
    expect(ok.body, {'item_id': 'i1', 'warehouse_id': 'wh1', 'location_id': 'l1', 'menge': 2.5, 'notiz': 'Rest'});
    // Leerer Platz geht nicht mit.
    expect(buildWithdrawalBody(itemId: 'i1', warehouseId: 'wh1', locationId: '', menge: '1').body!.containsKey('location_id'), isFalse);
    expect(buildWithdrawalBody(itemId: 'i1', warehouseId: 'wh1', menge: '1000000').error, isNull);
    for (final (item, wh, menge, notiz, msg) in [
      (null, 'wh1', '1', '', 'Bitte eine Zeile der Stückliste wählen.'),
      ('', 'wh1', '1', '', 'Bitte eine Zeile der Stückliste wählen.'),
      ('i1', null, '1', '', 'Bitte ein Lager wählen.'),
      ('i1', 'wh1', '', '', 'Menge größer 0 bis 1.000.000 angeben.'),
      ('i1', 'wh1', '0', '', 'Menge größer 0 bis 1.000.000 angeben.'),
      ('i1', 'wh1', '-1', '', 'Menge größer 0 bis 1.000.000 angeben.'),
      ('i1', 'wh1', 'zwei', '', 'Menge größer 0 bis 1.000.000 angeben.'),
      ('i1', 'wh1', '1000000,1', '', 'Menge größer 0 bis 1.000.000 angeben.'),
      ('i1', 'wh1', '1', 'x' * 501, 'Notiz höchstens 500 Zeichen.'),
    ]) {
      final r = buildWithdrawalBody(itemId: item, warehouseId: wh, menge: menge, notiz: notiz);
      expect(r.error, msg, reason: '$item $wh $menge');
      expect(r.body, isNull);
    }
    expect(buildWithdrawalBody(itemId: 'i1', warehouseId: 'wh1', menge: '1', notiz: 'x' * 500).error, isNull);
  });

  test('Zeile und Angaben einer Entnahme', () {
    expect(withdrawalLine(_w(), _warehouses), '10.11.2026 · Pos. 1 · mat-1 Griff · 2 Stk · HL – Halle');
    // Ohne Zeile und ohne Lagerliste: ohne Pos., mit Lager-ID.
    expect(withdrawalLine({..._w(), 'item_pos': null, 'menge': 1.25, 'einheit': ''}, const []), '10.11.2026 · mat-1 Griff · 1,25 · wh1');
    expect(withdrawalDetails(_w()), isNull);
    expect(withdrawalDetails(_w(notiz: 'Rest')), 'Notiz: Rest');
    expect(withdrawalDetails(_w(storniert: true, notiz: 'Rest')), 'Notiz: Rest\nStorniert 11.11.2026: falsch gebucht');
    expect(withdrawalCancelled(_w()), isFalse);
    expect(withdrawalCancelled(_w(storniert: true)), isTrue);
  });

  testWidgets('Nur lesen: Liste mit Storno, ohne Schaltflächen', (tester) async {
    final api = _FakeApi(permissions: const {'production.read'}, list: [_w(id: 'w2'), _w(storniert: true)]);
    await _pump(tester, api, _order(), warehouses: _warehouses);
    expect(api.calls, ['list o1']);
    final lines = tester.widgetList<Text>(find.text('10.11.2026 · Pos. 1 · mat-1 Griff · 2 Stk · HL – Halle')).toList();
    expect(lines, hasLength(2));
    expect(lines[0].style?.decoration, isNull);
    expect(lines[1].style?.decoration, TextDecoration.lineThrough);
    expect(find.text('Storniert 11.11.2026: falsch gebucht'), findsOneWidget);
    expect(find.text('Entnehmen'), findsNothing);
    expect(find.byTooltip('Entnahme stornieren'), findsNothing);
  });

  testWidgets('Leer; nach fertig nur noch stornieren', (tester) async {
    final api = _FakeApi(permissions: const {'production.read', 'production.write'});
    await _pump(tester, api, _order());
    expect(find.text('Noch keine Entnahmen.'), findsOneWidget);
    expect(find.text('Entnehmen'), findsOneWidget);

    final done = _FakeApi(permissions: const {'production.read', 'production.write'}, list: [_w(), _w(id: 'w0', storniert: true)]);
    await tester.pumpWidget(const SizedBox());
    await _pump(tester, done, _order(status: 'fertig'));
    expect(find.text('Entnehmen'), findsNothing);
    // Nur die nicht stornierte Entnahme lässt sich stornieren.
    expect(find.byTooltip('Entnahme stornieren'), findsOneWidget);
  });

  testWidgets('Entnehmen: vorbelegt, mit Platz, Hinweis, danach neu laden', (tester) async {
    final api = _FakeApi(permissions: const {'production.write', 'warehouses.read'}, hints: const ['mehrentnahme']);
    var changed = 0;
    await tester.binding.setSurfaceSize(const Size(1400, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: ProductionWithdrawalsSection(api: api, order: _order(), warehouses: _warehouses, onChanged: () async => changed++),
      ),
    ));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Entnehmen'));
    await tester.pumpAndSettle();
    // Erste Zeile mit Material, ihre offene Menge, Reservierungslager.
    expect(find.text('Pos. 1 A-7 Griff · offen 2 Stk'), findsOneWidget);
    expect(find.text('HL – Halle'), findsOneWidget);
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Menge (Einheit des Lagermaterials)')).controller!.text, '2');
    expect(api.calls, contains('locations wh1'));
    await tester.tap(find.text('Ohne Platz'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('R1 – Regal 1').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Menge (Einheit des Lagermaterials)'), '2,5');
    await tester.enterText(find.widgetWithText(TextField, 'Notiz (optional)'), 'Rest');
    await tester.tap(find.widgetWithText(FilledButton, 'Entnehmen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('create o1 {"item_id":"i1","warehouse_id":"wh1","location_id":"l1","menge":2.5,"notiz":"Rest"}'));
    expect(find.text('Entnahme gebucht – Mehr entnommen als der Bedarf der Zeile'), findsOneWidget);
    expect(api.calls.where((c) => c == 'list o1'), hasLength(2));
    expect(changed, 1);
  });

  testWidgets('Entnehmen: Zeile wechseln, Prüfung, Fehler des Servers', (tester) async {
    final api = _FakeApi(
        permissions: const {'production.write', 'warehouses.read'}, fail: 'Entnahme nur bei freigegebenen oder begonnenen Aufträgen');
    final changed = await _pump(tester, api, _order(), warehouses: _warehouses);
    await tester.tap(find.text('Entnehmen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Pos. 1 A-7 Griff · offen 2 Stk'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Pos. 3 P-100 Blendrahmen · offen 0 m').last);
    await tester.pumpAndSettle();
    // Alles entnommen: keine Menge vorbelegt, Speichern mit Meldung.
    expect(tester.widget<TextField>(find.widgetWithText(TextField, 'Menge (Einheit des Lagermaterials)')).controller!.text, '');
    await tester.tap(find.widgetWithText(FilledButton, 'Entnehmen'));
    await tester.pumpAndSettle();
    expect(find.text('Menge größer 0 bis 1.000.000 angeben.'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('create')), isEmpty);
    await tester.enterText(find.widgetWithText(TextField, 'Menge (Einheit des Lagermaterials)'), '1');
    await tester.tap(find.widgetWithText(FilledButton, 'Entnehmen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('create o1 {"item_id":"i3","warehouse_id":"wh1","menge":1.0,"notiz":""}'));
    expect(find.text('Fehler: Entnahme nur bei freigegebenen oder begonnenen Aufträgen'), findsOneWidget);
    expect(changed, 0);
  });

  testWidgets('Entnehmen ohne Leserecht für Lager', (tester) async {
    // Mit Reservierungslager: dieses Lager, ohne Platz.
    final api = _FakeApi(permissions: const {'production.write'});
    await _pump(tester, api, _order());
    await tester.tap(find.text('Entnehmen'));
    await tester.pumpAndSettle();
    expect(find.text('Lager: Reservierungslager wh1'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('locations')), isEmpty);
    await tester.tap(find.widgetWithText(FilledButton, 'Entnehmen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('create o1 {"item_id":"i1","warehouse_id":"wh1","menge":2.0,"notiz":""}'));

    // Ohne Reservierungslager: keine Entnahme möglich.
    final none = _FakeApi(permissions: const {'production.write'});
    await tester.pumpWidget(const SizedBox());
    await _pump(tester, none, _order(lager: null));
    await tester.tap(find.text('Entnehmen'));
    await tester.pumpAndSettle();
    expect(find.text('Ohne Leserecht für Lager und ohne Reservierungslager ist keine Entnahme möglich.'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Entnehmen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte ein Lager wählen.'), findsOneWidget);
    expect(none.calls.where((c) => c.startsWith('create')), isEmpty);
  });

  testWidgets('Storno nur mit Begründung, Fehler als Meldung', (tester) async {
    final api = _FakeApi(permissions: const {'production.write'}, list: [_w()]);
    await _pump(tester, api, _order());
    await tester.tap(find.byTooltip('Entnahme stornieren'));
    await tester.pumpAndSettle();
    final button = find.widgetWithText(FilledButton, 'Stornieren');
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), '  falsch gebucht ');
    await tester.pumpAndSettle();
    await tester.tap(button);
    await tester.pumpAndSettle();
    expect(api.calls, contains('storno o1 w1 falsch gebucht'));
    expect(find.text('Entnahme storniert'), findsOneWidget);

    final failing = _FakeApi(permissions: const {'production.write'}, list: [_w()], fail: 'Entnahme ist bereits storniert');
    await tester.pumpWidget(const SizedBox());
    await _pump(tester, failing, _order());
    await tester.tap(find.byTooltip('Entnahme stornieren'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), 'x');
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Stornieren'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Entnahme ist bereits storniert'), findsOneWidget);
  });
}
