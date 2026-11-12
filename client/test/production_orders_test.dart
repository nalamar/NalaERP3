import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/production_orders_page.dart';

// Fertigungsaufträge im Client (Backlog H.1.4a).

Map<String, dynamic> _order({String status = 'entwurf', List<String> hinweise = const [], List<Map<String, dynamic>>? items}) => {
      'id': 'o1',
      'nummer': 'FA-2026-0001',
      'project_nummer': 'P-1',
      'project_name': 'Testprojekt',
      'stueckzahl': 4,
      'liefertermin': '2026-12-01T00:00:00Z',
      'ausfuehrungsklasse': 'EXC2',
      'notiz': '',
      'status': status,
      'phase_name': 'Haus A',
      'position_nummer': '3',
      'position_name': 'Fenster',
      'variante_name': 'links',
      'width_mm': 1230,
      'height_mm': 1480,
      'created_at': '2026-11-02T08:00:00Z',
      if (status != 'entwurf') 'freigegeben_at': '2026-11-03T08:00:00Z',
      if (status == 'storniert') ...{'storniert_at': '2026-11-04T08:00:00Z', 'storno_reason': 'doppelt'},
      'hinweise': hinweise,
      if (items != null) 'items': items,
    };

final _items = [
  {
    'pos': 1,
    'art': 'artikel',
    'article_code': 'A-7',
    'description': '',
    'configuration': '',
    'menge_je_stueck': 1.5,
    'menge_gesamt': 6,
    'unit': ''
  },
  {
    'pos': 2,
    'art': 'glas',
    'article_code': '',
    'description': 'Isolierglas',
    'configuration': '4/16/4',
    'width_mm': 1100,
    'height_mm': 1350,
    'area_m2': 1.485,
    'menge_je_stueck': 1,
    'menge_gesamt': 4,
    'unit': '',
  },
  {
    'pos': 3,
    'art': 'profil',
    'article_code': 'P-100',
    'description': 'Blendrahmen',
    'configuration': '',
    'length_mm': 1230.4,
    'menge_je_stueck': 2,
    'menge_gesamt': 8,
    'unit': 'Stk',
  },
];

class _FakeApi extends ApiClient {
  _FakeApi(
      {required this.permissions,
      required this.order,
      this.list = const [],
      this.failStep,
      this.failCreate,
      this.warehouses = const [],
      this.failWarehouses = false,
      this.failPickList})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  Map<String, dynamic> order;
  List<dynamic> list;
  final String? failStep;
  final String? failCreate;
  final List<dynamic> warehouses;
  final bool failWarehouses;
  final String? failPickList;
  final List<String> calls = [];
  String? listScope;

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listProductionOrders(
      {String? projectId, String? elevationId, String? status, bool includeStorniert = false}) async {
    calls.add('list $status');
    listScope = '$projectId/$elevationId';
    return list;
  }

  @override
  Future<Map<String, dynamic>> createProductionOrder(Map<String, dynamic> body) async {
    calls.add('create ${jsonEncode(body)}');
    if (failCreate != null) throw ApiException(statusCode: 400, message: failCreate!);
    list = [...list, order];
    return order;
  }

  @override
  Future<Map<String, dynamic>> getProductionOrder(String id) async {
    calls.add('get $id');
    return order;
  }

  @override
  Future<Map<String, dynamic>> productionOrderStep(String id, String aktion) async {
    calls.add('step $id $aktion');
    if (failStep != null) throw ApiException(statusCode: 409, message: failStep!);
    order = {
      ...order,
      'status': {'freigeben': 'freigegeben', 'beginnen': 'in_arbeit', 'fertigmelden': 'fertig'}[aktion]
    };
    return order;
  }

  @override
  Future<Map<String, dynamic>> releaseProductionOrder(String id, {String? lagerId}) async {
    calls.add('release $id ${lagerId ?? '-'}');
    if (failStep != null) throw ApiException(statusCode: 409, message: failStep!);
    order = {...order, 'status': 'freigegeben', 'reservierung_lager_id': lagerId};
    return order;
  }

  // Arbeitsgänge (H.2.5b) prüft production_operations_test.dart.
  @override
  Future<List<dynamic>> listProductionOperations(String orderId) async => const [];

  // Entnahmen (H.3.4b) prüft production_withdrawals_test.dart.
  @override
  Future<List<dynamic>> listProductionWithdrawals(String orderId) async => const [];

  @override
  Future<Map<String, dynamic>> getProductionSollIst(String orderId) async => {'arbeitsgaenge': 0, 'erledigt': 0};

  @override
  Future<Map<String, dynamic>> createProductionWithdrawal(String orderId, Map<String, dynamic> body) async {
    calls.add('withdraw $orderId ${jsonEncode(body)}');
    return {'id': 'w1', 'hinweise': <String>[]};
  }

  @override
  Future<List<dynamic>> listWarehouses() async {
    calls.add('warehouses');
    if (failWarehouses) throw ApiException(statusCode: 500, message: 'kaputt');
    return warehouses;
  }

  @override
  Future<Map<String, dynamic>> updateProductionOrderDraft(String id, Map<String, dynamic> body) async {
    calls.add('update $id ${jsonEncode(body)}');
    order = {...order, ...body};
    return order;
  }

  @override
  Future<void> downloadProductionPickListPdf(String orderId, {String? filename}) async {
    calls.add('picklist $orderId $filename');
    if (failPickList != null) throw ApiException(statusCode: 409, message: failPickList!);
  }

  @override
  Future<Map<String, dynamic>> stornoProductionOrder(String id, String reason) async {
    calls.add('storno $id $reason');
    order = {...order, 'status': 'storniert', 'storno_reason': reason, 'storniert_at': '2026-11-05T00:00:00Z'};
    return order;
  }
}

Widget _wrap(Widget child, {Key? key}) =>
    MaterialApp(key: key, theme: ThemeData(useMaterial3: true, splashFactory: InkRipple.splashFactory), home: child);

Future<void> _desktop(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

void main() {
  test('formatQty', () {
    expect(formatQty(1.5), '1,5');
    expect(formatQty(2), '2');
    expect(formatQty(1.4851), '1,485');
    expect(formatQty(10.0), '10');
    expect(formatQty(1230.4), '1230,4');
    expect(formatQty(0), '0');
    expect(formatQty(null), '');
    expect(formatQty('3'), '');
  });

  test('Titel, Untertitel, Hinweise, Verlauf', () {
    final o = _order(hinweise: ['kein_kundenauftrag']);
    expect(productionOrderTitle(o), 'FA-2026-0001 · Pos. 3 Fenster (links)');
    expect(productionOrderSubtitle(o), 'P-1 Testprojekt · 4 Stück · Liefertermin 01.12.2026 · EXC2 · Entwurf');
    expect(productionHints(o), ['kein_kundenauftrag']);
    expect(productionHints({}), isEmpty);
    expect(productionOrderTitle({'nummer': 'FA-1', 'position_nummer': '', 'position_name': ''}), 'FA-1 · Position nicht mehr vorhanden');
    expect(productionOrderTitle({'nummer': 'FA-1', 'position_nummer': '7', 'position_name': ''}), 'FA-1 · Pos. 7');
    expect(productionOrderSubtitle({'stueckzahl': 1, 'status': 'in_arbeit'}), '1 Stück · In Arbeit');
    expect(productionOrderSubtitle({'stueckzahl': 1, 'status': 'neu'}), '1 Stück · neu');
    expect(productionStatusHistory(_order()), ['Angelegt 02.11.2026']);
    expect(productionStatusHistory(_order(status: 'freigegeben')), ['Angelegt 02.11.2026', 'Freigegeben 03.11.2026']);
    expect(productionStatusHistory(_order(status: 'storniert')),
        ['Angelegt 02.11.2026', 'Freigegeben 03.11.2026', 'Storniert 04.11.2026: doppelt']);
    expect(
        productionStatusHistory({..._order(status: 'fertig'), 'begonnen_at': '2026-11-04T00:00:00Z', 'fertig_at': '2026-11-06T00:00:00Z'}),
        ['Angelegt 02.11.2026', 'Freigegeben 03.11.2026', 'Begonnen 04.11.2026', 'Fertig 06.11.2026']);
  });

  test('Schritte und Storno je Status', () {
    expect(productionNextStep['entwurf']!.aktion, 'freigeben');
    expect(productionNextStep['freigegeben']!.aktion, 'beginnen');
    expect(productionNextStep['in_arbeit']!.aktion, 'fertigmelden');
    expect(productionNextStep['fertig'], isNull);
    expect(productionNextStep['storniert'], isNull);
    for (final s in ['entwurf', 'freigegeben', 'in_arbeit']) {
      expect(productionCanStorno(s), isTrue, reason: s);
    }
    for (final s in ['fertig', 'storniert', null, 'x']) {
      expect(productionCanStorno(s), isFalse, reason: '$s');
    }
  });

  test('productionItemLine', () {
    expect(productionItemLine(_items[2]), 'P-100 Blendrahmen · 1230,4 mm · 2 je Stück · gesamt 8 Stk');
    expect(productionItemLine(_items[1]), '4/16/4 Isolierglas · 1100 × 1350 mm · 1,485 m² · 1 je Stück · gesamt 4');
    expect(productionItemLine(_items[0]), 'A-7 · 1,5 je Stück · gesamt 6');
    expect(productionItemLine({'menge_je_stueck': 1, 'menge_gesamt': 1}), '(ohne Bezeichnung) · 1 je Stück · gesamt 1');
    // nur eine Glaskante: keine Maßangabe
    expect(productionItemLine({'art': 'glas', 'width_mm': 100, 'menge_je_stueck': 1, 'menge_gesamt': 1}),
        '(ohne Bezeichnung) · 1 je Stück · gesamt 1');
  });

  test('buildProductionDraftBody', () {
    final ok = buildProductionDraftBody(stueckzahl: ' 4 ', liefertermin: '1.12.2026', ausfuehrungsklasse: 'exc2', notiz: ' n ');
    expect(ok.error, isNull);
    expect(ok.body, {'stueckzahl': 4, 'liefertermin': '2026-12-01', 'ausfuehrungsklasse': 'EXC2', 'notiz': 'n'});
    expect(buildProductionDraftBody(stueckzahl: '10000').body,
        {'stueckzahl': 10000, 'liefertermin': '', 'ausfuehrungsklasse': '', 'notiz': ''});
    expect(buildProductionDraftBody(stueckzahl: '1', liefertermin: '2026-12-01').body!['liefertermin'], '2026-12-01');
    for (final bad in ['', '0', '-1', '1,5', 'zwei', '10001']) {
      expect(buildProductionDraftBody(stueckzahl: bad).error, 'Stückzahl als ganze Zahl von 1 bis 10000 angeben.', reason: bad);
    }
    expect(buildProductionDraftBody(stueckzahl: '1', liefertermin: 'morgen').error, 'Bitte den Liefertermin als TT.MM.JJJJ angeben.');
    expect(buildProductionDraftBody(stueckzahl: '1', liefertermin: '31.02.2026').error, 'Bitte den Liefertermin als TT.MM.JJJJ angeben.');
    expect(buildProductionDraftBody(stueckzahl: '1', ausfuehrungsklasse: 'EXC5').error, 'Ausführungsklasse EXC1 bis EXC4 oder keine.');
  });

  testWidgets('Liste: Hinweise, Filter, Öffnen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
      permissions: const {'production.read'},
      order: _order(),
      list: [
        _order(hinweise: ['menge_ueberschritten']),
        {..._order(status: 'freigegeben'), 'id': 'o2', 'nummer': 'FA-2026-0002'}
      ],
    );
    await tester.pumpWidget(_wrap(ProductionOrdersPage(api: api)));
    await tester.pumpAndSettle();
    expect(api.calls, ['list ']);
    expect(find.text('FA-2026-0001 · Pos. 3 Fenster (links)'), findsOneWidget);
    expect(find.textContaining('Zusammen mehr Stück als die Positionsmenge'), findsOneWidget);
    // Warnsymbol am Auftrag mit Hinweis, das normale am anderen.
    Finder iconIn(String nummer, IconData icon) => find.descendant(
          of: find.ancestor(of: find.text('$nummer · Pos. 3 Fenster (links)'), matching: find.byType(ListTile)),
          matching: find.byIcon(icon),
        );
    expect(iconIn('FA-2026-0001', Icons.warning_amber_rounded), findsOneWidget);
    expect(iconIn('FA-2026-0002', Icons.precision_manufacturing_rounded), findsOneWidget);
    expect(iconIn('FA-2026-0002', Icons.warning_amber_rounded), findsNothing);

    await tester.tap(find.byKey(const ValueKey('productionStatusFilter')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Storniert').last);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list storniert');

    await tester.tap(find.text('FA-2026-0001 · Pos. 3 Fenster (links)'));
    await tester.pumpAndSettle();
    expect(find.byType(ProductionOrderDetailPage), findsOneWidget);
    expect(api.calls, contains('get o1'));
    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(api.calls.last, 'list storniert'); // nach der Rückkehr neu geladen
  });

  testWidgets('Liste leer', (tester) async {
    await _desktop(tester);
    await tester.pumpWidget(_wrap(ProductionOrdersPage(api: _FakeApi(permissions: const {'production.read'}, order: _order()))));
    await tester.pumpAndSettle();
    expect(find.textContaining('Keine Fertigungsaufträge'), findsOneWidget);
  });

  testWidgets('Detail im Entwurf: Freigabe mit Rückfrage', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.read', 'production.write'}, order: _order(hinweise: ['kein_kundenauftrag']));
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(find.text('Kein Kundenauftrag zum Projekt'), findsOneWidget);
    expect(find.text('Los: Haus A'), findsOneWidget);
    expect(find.text('Maße: 1230 × 1480 mm'), findsOneWidget);
    expect(find.textContaining('wird bei der Freigabe aus der Position kopiert'), findsOneWidget);
    expect(find.text('Entwurf ändern'), findsOneWidget);
    expect(find.text('Stornieren'), findsOneWidget);

    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben'));
    await tester.pumpAndSettle();
    expect(find.text('Auftrag freigeben'), findsOneWidget);
    expect(find.text('Ohne Leserecht für Lager wird ohne Reservierung freigegeben.'), findsOneWidget);
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('step') || c.startsWith('release')), isEmpty);
    // Ohne warehouses.read: kein Lager laden, Hinweis im Dialog.
    expect(api.calls, isNot(contains('warehouses')));

    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben').last);
    await tester.pumpAndSettle();
    expect(api.calls, contains('release o1 -'));
    expect(find.widgetWithText(FilledButton, 'Beginnen'), findsOneWidget);
    expect(find.text('Entwurf ändern'), findsNothing);
  });

  testWidgets('Detail: Schritte ohne Rückfrage, Fehler als Meldung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.write'}, order: _order(status: 'freigegeben', items: _items));
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(find.text('Profile'), findsOneWidget);
    expect(find.text('Glas'), findsOneWidget);
    expect(find.text('Artikel'), findsOneWidget);
    expect(find.text('P-100 Blendrahmen · 1230,4 mm · 2 je Stück · gesamt 8 Stk'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Beginnen'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'step o1 beginnen');
    await tester.tap(find.widgetWithText(FilledButton, 'Fertig melden'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'step o1 fertigmelden');
    // fertig: kein Schritt, kein Storno
    expect(find.byType(FilledButton), findsNothing);
    expect(find.text('Stornieren'), findsNothing);

    final failing =
        _FakeApi(permissions: const {'production.write'}, order: _order(), failStep: 'Position oder Variante ist nicht mehr im Projekt');
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: failing, orderId: 'o1'), key: const ValueKey('f')));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben').last);
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Position oder Variante ist nicht mehr im Projekt'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Freigeben'), findsOneWidget); // Status unverändert
  });

  test('Reservierung je Zeile und Lagername', () {
    expect(productionReservationLine({'bedarf': 4.5, 'reserviert': 4, 'fehlmenge': 0.5, 'material_einheit': 'Stk'}),
        'Bedarf 4,5 Stk · reserviert 4 Stk · Fehlmenge 0,5 Stk');
    expect(productionReservationLine({'bedarf': 7.3824, 'reserviert': 7.3824, 'fehlmenge': 0, 'material_einheit': 'm'}),
        'Bedarf 7,382 m · reserviert 7,382 m');
    expect(productionReservationLine({'bedarf': 2, 'reserviert': null, 'fehlmenge': 2, 'material_einheit': ''}),
        'Bedarf 2 · reserviert 0 · Fehlmenge 2');
    // Negativfall: ohne Bedarf (nicht reserviert) keine Zeile.
    expect(productionReservationLine({'bedarf': null, 'material_id': 'm1'}), isNull);
    expect(productionReservationLine(<String, dynamic>{}), isNull);
    expect(productionItemHasShortfall({'fehlmenge': 0.5}), isTrue);
    expect(productionItemHasShortfall({'fehlmenge': 0}), isFalse);
    expect(productionItemHasShortfall({'fehlmenge': null}), isFalse);
    final lager = [
      {'id': 'wh1', 'code': 'HL', 'name': 'Halle'},
      {'id': 'wh2', 'code': '', 'name': 'Außenlager'},
    ];
    expect(productionWarehouseLabel('wh1', lager), 'HL – Halle');
    expect(productionWarehouseLabel('wh2', lager), 'Außenlager');
    expect(productionWarehouseLabel('wh9', lager), 'wh9');
  });

  testWidgets('Freigabe mit Lager', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
      permissions: const {'production.write', 'warehouses.read'},
      order: _order(),
      warehouses: const [
        {'id': 'wh1', 'code': 'HL', 'name': 'Halle'},
        {'id': 'wh2', 'code': 'AL', 'name': 'Außenlager'},
      ],
    );
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('warehouses'));
    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben'));
    await tester.pumpAndSettle();
    expect(find.text('Material reservieren aus'), findsOneWidget);
    expect(find.text('Ohne Reservierung'), findsOneWidget);
    await tester.tap(find.text('Ohne Reservierung'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('AL – Außenlager').last);
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben').last);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'release o1 wh2');
    expect(find.text('Reserviert aus Lager: AL – Außenlager'), findsOneWidget);
  });

  testWidgets('Freigabe: vorbelegt ohne Reservierung, kein Lager, Fehler', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.write', 'warehouses.read'}, order: _order(), warehouses: const [
      {'id': 'wh1', 'code': 'HL', 'name': 'Halle'},
    ]);
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Freigeben').last);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'release o1 -');
    expect(find.textContaining('Reserviert aus Lager'), findsNothing);

    for (final (fake, text) in [
      (_FakeApi(permissions: const {'production.write', 'warehouses.read'}, order: _order()), 'Kein Lager angelegt – Freigabe ohne Reservierung.'),
      (
        _FakeApi(permissions: const {'production.write', 'warehouses.read'}, order: _order(), failWarehouses: true),
        'Lager konnten nicht geladen werden: kaputt'
      ),
    ]) {
      await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: fake, orderId: 'o1'), key: ValueKey(text)));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Freigeben'));
      await tester.pumpAndSettle();
      expect(find.text(text), findsOneWidget);
      expect(find.text('Material reservieren aus'), findsNothing);
      await tester.tap(find.widgetWithText(FilledButton, 'Freigeben').last);
      await tester.pumpAndSettle();
      expect(fake.calls.last, 'release o1 -');
    }
  });

  testWidgets('Detail: reserviert und Fehlmenge je Zeile', (tester) async {
    await _desktop(tester);
    final items = [
      {..._items[0], 'material_id': 'm1', 'material_einheit': 'Stk', 'bedarf': 6, 'reserviert': 4, 'fehlmenge': 2},
      {..._items[1], 'material_id': 'm2', 'material_einheit': 'm²', 'bedarf': null},
      {..._items[2], 'material_id': 'm3', 'material_einheit': 'm', 'bedarf': 9.8432, 'reserviert': 9.8432, 'fehlmenge': 0},
    ];
    final api = _FakeApi(
        permissions: const {'production.read'},
        order: {..._order(status: 'freigegeben', hinweise: ['fehlmenge'], items: items), 'reservierung_lager_id': 'wh1'});
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(find.text('Nicht alles Material im Lager verfügbar (Fehlmenge)'), findsOneWidget);
    // Ohne warehouses.read: die ID statt des Namens.
    expect(find.text('Reserviert aus Lager: wh1'), findsOneWidget);
    final fehl = tester.widget<Text>(find.text('Bedarf 6 Stk · reserviert 4 Stk · Fehlmenge 2 Stk'));
    expect(fehl.style?.color, Colors.orange.shade900);
    final voll = tester.widget<Text>(find.text('Bedarf 9,843 m · reserviert 9,843 m'));
    expect(voll.style?.color, Colors.black54);
    expect(find.textContaining('Bedarf'), findsNWidgets(2));
  });

  test('Entnahme je Zeile und Status', () {
    expect(productionWithdrawalLine({'entnommen': 4, 'offen': 2, 'material_einheit': 'Stk'}), 'entnommen 4 Stk · offen 2 Stk');
    expect(productionWithdrawalLine({'entnommen': 9.8432, 'offen': 0, 'material_einheit': 'm'}), 'entnommen 9,843 m · alles entnommen');
    // Ohne umrechenbaren Bedarf oder nach "fertig": nur entnommen.
    expect(productionWithdrawalLine({'entnommen': 0, 'offen': null, 'material_einheit': ''}), 'entnommen 0');
    // Ohne Lagermaterial keine Zeile.
    expect(productionWithdrawalLine({'entnommen': null, 'offen': null}), isNull);
    expect(productionWithdrawalLine(<String, dynamic>{}), isNull);
    for (final s in ['freigegeben', 'in_arbeit']) {
      expect(productionCanWithdraw(s), isTrue, reason: s);
    }
    for (final s in ['entwurf', 'fertig', 'storniert', null, 'x']) {
      expect(productionCanWithdraw(s), isFalse, reason: '$s');
    }
    expect(productionHintLabels['nicht_entnommen'], 'Material noch nicht vollständig entnommen');
  });

  testWidgets('Detail: entnommen, offen und Kommissionierliste nur lesend', (tester) async {
    await _desktop(tester);
    final items = [
      {..._items[0], 'material_id': 'm1', 'material_einheit': 'Stk', 'entnommen': 4, 'offen': 2},
      {..._items[1], 'entnommen': null, 'offen': null},
      {..._items[2], 'material_id': 'm3', 'material_einheit': 'm', 'entnommen': 9.8432, 'offen': 0},
    ];
    final api = _FakeApi(
        permissions: const {'production.read'}, order: _order(status: 'in_arbeit', hinweise: ['nicht_entnommen'], items: items));
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(find.text('Material noch nicht vollständig entnommen'), findsOneWidget);
    expect(find.text('entnommen 4 Stk · offen 2 Stk'), findsOneWidget);
    expect(find.text('entnommen 9,843 m · alles entnommen'), findsOneWidget);
    expect(find.textContaining('entnommen '), findsNWidgets(2));
    // Abschnitt "Entnahmen" ab der Freigabe (H.3.4b).
    expect(find.text('Noch keine Entnahmen.'), findsOneWidget);
    // Nur lesen: keine Statusschritte, aber die Kommissionierliste.
    expect(find.byType(FilledButton), findsNothing);
    expect(find.text('Stornieren'), findsNothing);
    await tester.tap(find.text('Kommissionierliste'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('picklist o1 Kommissionierliste_FA-2026-0001.pdf'));
  });

  testWidgets('Detail: nach einer Entnahme wird der Auftrag neu geladen', (tester) async {
    await _desktop(tester);
    final items = [
      {..._items[0], 'id': 'i1', 'material_id': 'm1', 'material_einheit': 'Stk', 'entnommen': 4, 'offen': 2},
    ];
    final api = _FakeApi(
        permissions: const {'production.read', 'production.write'},
        order: {..._order(status: 'in_arbeit', items: items), 'reservierung_lager_id': 'wh1'});
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c == 'get o1'), hasLength(1));
    await tester.tap(find.widgetWithText(OutlinedButton, 'Entnehmen'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Entnehmen'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('withdraw o1 {"item_id":"i1","warehouse_id":"wh1","menge":2.0,"notiz":""}'));
    expect(api.calls.where((c) => c == 'get o1'), hasLength(2));
  });

  testWidgets('Kommissionierliste: Fehler als Meldung, nicht im Entwurf und nach fertig', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
        permissions: const {'production.read', 'production.write'},
        order: _order(status: 'freigegeben', items: const []),
        failPickList: 'Kommissionierliste nur bei freigegebenen oder begonnenen Aufträgen');
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(find.text('Beginnen'), findsOneWidget);
    await tester.tap(find.text('Kommissionierliste'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Kommissionierliste nur bei freigegebenen oder begonnenen Aufträgen'), findsOneWidget);

    for (final s in ['entwurf', 'fertig', 'storniert']) {
      final other = _FakeApi(permissions: const {'production.read', 'production.write'}, order: _order(status: s, items: const []));
      await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: other, orderId: 'o1'), key: ValueKey(s)));
      await tester.pumpAndSettle();
      expect(find.text('Kommissionierliste'), findsNothing, reason: s);
    }
  });

  testWidgets('Detail: freigegeben ohne Material', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.read'}, order: _order(status: 'freigegeben', items: const []));
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(find.text('Die Position hatte bei der Freigabe kein Material.'), findsOneWidget);
  });

  testWidgets('Detail nur lesen: keine Schaltflächen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.read'}, order: _order());
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    expect(find.text('FA-2026-0001 · Pos. 3 Fenster (links)'), findsOneWidget);
    expect(find.text('Entnahmen'), findsNothing);
    expect(find.byType(FilledButton), findsNothing);
    expect(find.byType(OutlinedButton), findsNothing);
  });

  testWidgets('Entwurf ändern mit Prüfung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.write'}, order: _order());
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Entwurf ändern'));
    await tester.pumpAndSettle();
    final fields = find.descendant(of: find.byType(AlertDialog), matching: find.byType(TextField));
    expect(tester.widget<TextField>(fields.at(0)).controller!.text, '4');
    expect(tester.widget<TextField>(fields.at(1)).controller!.text, '01.12.2026');
    // Ausführungsklasse nur angezeigt, nicht wählbar (H.1.6).
    expect(find.text('Ausführungsklasse: EXC2 (bleibt erhalten)'), findsOneWidget);
    expect(find.descendant(of: find.byType(AlertDialog), matching: find.byType(DropdownButtonFormField<String>)), findsNothing);
    expect(fields, findsNWidgets(3));
    await tester.enterText(fields.at(0), '0');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(find.text('Stückzahl als ganze Zahl von 1 bis 10000 angeben.'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('update')), isEmpty);
    await tester.enterText(fields.at(0), '6');
    await tester.enterText(fields.at(1), '');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'update o1 {"stueckzahl":6,"liefertermin":"","ausfuehrungsklasse":"EXC2","notiz":""}');
    expect(find.text('Entwurf gespeichert'), findsOneWidget);
  });

  testWidgets('Storno nur mit Begründung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.write'}, order: _order(status: 'freigegeben'));
    await tester.pumpWidget(_wrap(ProductionOrderDetailPage(api: api, orderId: 'o1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Stornieren'));
    await tester.pumpAndSettle();
    final confirm = find.descendant(of: find.byType(AlertDialog), matching: find.widgetWithText(FilledButton, 'Stornieren'));
    expect(tester.widget<FilledButton>(confirm).onPressed, isNull);
    await tester.enterText(find.descendant(of: find.byType(AlertDialog), matching: find.byType(TextField)), ' doppelt ');
    await tester.pump();
    await tester.tap(confirm);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'storno o1 doppelt');
    expect(find.textContaining('Storniert 05.11.2026: doppelt'), findsOneWidget);
    expect(find.text('Stornieren'), findsNothing);
  });

  testWidgets('Dashboard: Karte nur mit production.read', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.read'}, order: _order());
    await tester.pumpWidget(_wrap(DashboardPage(api: api, onLogout: () async {}, permissions: const {'production.write'})));
    await tester.pumpAndSettle();
    expect(find.text('Fertigungsaufträge'), findsNothing);
    await tester.pumpWidget(
        _wrap(DashboardPage(key: const ValueKey('mit'), api: api, onLogout: () async {}, permissions: const {'production.read'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fertigungsaufträge'));
    await tester.pumpAndSettle();
    expect(find.byType(ProductionOrdersPage), findsOneWidget);
  });

  // An der Variante im Projekt (Backlog H.1.4b).
  test('Vorschlag Stückzahl und Aufträge der Variante', () {
    expect(defaultProductionStueckzahl(3), '3');
    expect(defaultProductionStueckzahl(2.0), '2');
    expect(defaultProductionStueckzahl('4'), '4');
    expect(defaultProductionStueckzahl('2,0'), '2');
    expect(defaultProductionStueckzahl(10000), '10000');
    expect(defaultProductionStueckzahl(1), '1');
    expect(defaultProductionStueckzahl(1.5), '');
    expect(defaultProductionStueckzahl(0), '');
    expect(defaultProductionStueckzahl(10001), '');
    expect(defaultProductionStueckzahl(null), '');
    expect(defaultProductionStueckzahl('x'), '');
    final orders = [
      {'id': 'a', 'single_elevation_id': 'v1'},
      {'id': 'b', 'single_elevation_id': 'v2'},
      {'id': 'c', 'single_elevation_id': 'v1'},
      'kein Auftrag',
    ];
    expect(productionOrdersOfVariant(orders, 'v1').map((o) => o['id']), ['a', 'c']);
    expect(productionOrdersOfVariant(orders, 'v3'), isEmpty);
  });

  Widget variantSection(_FakeApi api, {Key? key}) => _wrap(
        Scaffold(body: VariantProductionOrders(api: api, projectId: 'p1', elevationId: 'e1', variantId: 'v1', variantMenge: 2)),
        key: key,
      );

  testWidgets('Variante: Aufträge lesen, ohne Anlegen', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.read'}, order: _order(), list: [
      {
        ..._order(hinweise: ['menge_ueberschritten']),
        'single_elevation_id': 'v1'
      },
      {..._order(), 'id': 'o2', 'nummer': 'FA-2026-0002', 'single_elevation_id': 'v2'},
    ]);
    await tester.pumpWidget(variantSection(api));
    await tester.pumpAndSettle();
    expect(api.listScope, 'p1/e1');
    expect(find.text('FA-2026-0001 · Pos. 3 Fenster (links)'), findsOneWidget);
    expect(find.textContaining('FA-2026-0002'), findsNothing);
    expect(find.byIcon(Icons.warning_amber_rounded), findsOneWidget);
    expect(find.text('Fertigungsauftrag anlegen'), findsNothing);
    await tester.tap(find.text('FA-2026-0001 · Pos. 3 Fenster (links)'));
    await tester.pumpAndSettle();
    expect(find.byType(ProductionOrderDetailPage), findsOneWidget);
    expect(api.calls, contains('get o1'));
  });

  testWidgets('Variante: ohne Recht nichts, leer mit Hinweis', (tester) async {
    await _desktop(tester);
    final none = _FakeApi(permissions: const {}, order: _order());
    await tester.pumpWidget(variantSection(none));
    await tester.pumpAndSettle();
    expect(find.text('Fertigungsaufträge'), findsNothing);
    expect(none.calls, isEmpty);
    final empty = _FakeApi(permissions: const {'production.read'}, order: _order());
    await tester.pumpWidget(variantSection(empty, key: const ValueKey('leer')));
    await tester.pumpAndSettle();
    expect(find.text('Noch kein Fertigungsauftrag zu dieser Variante.'), findsOneWidget);
    // Nur Schreibrecht: Anlegen ja, keine Liste.
    final writer = _FakeApi(permissions: const {'production.write'}, order: _order());
    await tester.pumpWidget(variantSection(writer, key: const ValueKey('schreiben')));
    await tester.pumpAndSettle();
    expect(find.text('Fertigungsauftrag anlegen'), findsOneWidget);
    expect(find.text('Noch kein Fertigungsauftrag zu dieser Variante.'), findsNothing);
    expect(writer.calls, isEmpty);
  });

  testWidgets('Variante: anlegen mit Prüfung, danach Detail', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(permissions: const {'production.read', 'production.write'}, order: {..._order(), 'single_elevation_id': 'v1'});
    await tester.pumpWidget(variantSection(api));
    await tester.pumpAndSettle();
    // Abbrechen legt nichts an.
    await tester.tap(find.text('Fertigungsauftrag anlegen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.calls.where((c) => c.startsWith('create')), isEmpty);
    await tester.tap(find.text('Fertigungsauftrag anlegen'));
    await tester.pumpAndSettle();
    expect(find.descendant(of: find.byType(AlertDialog), matching: find.text('Fertigungsauftrag anlegen')), findsOneWidget);
    final fields = find.descendant(of: find.byType(AlertDialog), matching: find.byType(TextField));
    expect(tester.widget<TextField>(fields.at(0)).controller!.text, '2');
    // Negativfall: beim Anlegen keine Ausführungsklasse, weder Auswahl noch Anzeige.
    expect(find.descendant(of: find.byType(AlertDialog), matching: find.textContaining('Ausführungsklasse')), findsNothing);
    expect(find.descendant(of: find.byType(AlertDialog), matching: find.byType(DropdownButtonFormField<String>)), findsNothing);
    await tester.enterText(fields.at(1), '31.02.2026');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte den Liefertermin als TT.MM.JJJJ angeben.'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('create')), isEmpty);
    await tester.enterText(fields.at(0), '3');
    await tester.enterText(fields.at(1), '01.12.2026');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(
        api.calls.where((c) => c.startsWith('create')).single,
        'create {"stueckzahl":3,"liefertermin":"2026-12-01","ausfuehrungsklasse":"","notiz":"",'
        '"project_id":"p1","elevation_id":"e1","single_elevation_id":"v1"}');
    expect(find.text('Fertigungsauftrag FA-2026-0001 angelegt'), findsOneWidget);
    expect(find.byType(ProductionOrderDetailPage), findsOneWidget);
    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(find.text('FA-2026-0001 · Pos. 3 Fenster (links)'), findsOneWidget);
  });

  testWidgets('Variante: Fehler beim Anlegen als Meldung', (tester) async {
    await _desktop(tester);
    final api = _FakeApi(
        permissions: const {'production.read', 'production.write'}, order: _order(), failCreate: 'Variante ist nicht an der Position');
    await tester.pumpWidget(variantSection(api));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fertigungsauftrag anlegen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Fehler: Variante ist nicht an der Position'), findsOneWidget);
    expect(find.byType(ProductionOrderDetailPage), findsNothing);
    final button = find.ancestor(of: find.text('Fertigungsauftrag anlegen'), matching: find.byWidgetPredicate((w) => w is TextButton));
    expect(tester.widget<TextButton>(button).onPressed, isNotNull);
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

    test('Pfade, Parameter, Bodies', () async {
      await mock([], () => api.listProductionOrders());
      await mock([], () => api.listProductionOrders(projectId: 'p1', elevationId: 'e1', status: 'fertig', includeStorniert: true));
      await mock([], () => api.listProductionOrders(status: ''));
      await mock({'id': 'o1'}, () => api.getProductionOrder('o1'));
      await mock({'id': 'o1'}, () => api.createProductionOrder({'stueckzahl': 1}), status: 201);
      await mock({'id': 'o1'}, () => api.updateProductionOrderDraft('o1', {'stueckzahl': 2}));
      await mock({'id': 'o1'}, () => api.productionOrderStep('o1', 'beginnen'));
      await mock({'id': 'o1'}, () => api.stornoProductionOrder('o1', 'x'));
      await mock({'id': 'o1'}, () => api.releaseProductionOrder('o1', lagerId: 'wh1'));
      await mock({'id': 'o1'}, () => api.releaseProductionOrder('o1'));
      await mock({'id': 'o1'}, () => api.releaseProductionOrder('o1', lagerId: ''));
      expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
        'GET /api/v1/production/orders/?',
        'GET /api/v1/production/orders/?project_id=p1&elevation_id=e1&status=fertig&include_storniert=true',
        'GET /api/v1/production/orders/?',
        'GET /api/v1/production/orders/o1?',
        'POST /api/v1/production/orders/?',
        'PUT /api/v1/production/orders/o1?',
        'POST /api/v1/production/orders/o1/beginnen?',
        'POST /api/v1/production/orders/o1/storno?',
        'POST /api/v1/production/orders/o1/freigeben?',
        'POST /api/v1/production/orders/o1/freigeben?',
        'POST /api/v1/production/orders/o1/freigeben?',
      ]);
      expect(calls[8].body, '{"lager_id":"wh1"}');
      expect(calls[9].body, '{}');
      expect(calls[10].body, '{}');
      expect(calls[4].body, '{"stueckzahl":1}');
      expect(calls[5].body, '{"stueckzahl":2}');
      expect(calls[7].body, '{"reason":"x"}');
    });

    test('Entnahmen und Kommissionierliste: Pfade und Bodies', () async {
      final list = await mock([
        {'id': 'w1'}
      ], () => api.listProductionWithdrawals('o1'));
      final w = await mock({
        'id': 'w2',
        'hinweise': ['mehrentnahme']
      }, () => api.createProductionWithdrawal('o1', {'item_id': 'i1', 'warehouse_id': 'wh1', 'menge': 2.5}), status: 201);
      await mock({'id': 'w2'}, () => api.stornoProductionWithdrawal('o1', 'w2', 'falsch gebucht'));
      await mock({}, () => api.downloadProductionPickListPdf('o1', filename: 'Kommissionierliste_FA-1.pdf'));
      expect(calls.map((c) => '${c.method} ${c.url.path}'), [
        'GET /api/v1/production/orders/o1/withdrawals',
        'POST /api/v1/production/orders/o1/withdrawals',
        'POST /api/v1/production/orders/o1/withdrawals/w2/storno',
        'GET /api/v1/production/orders/o1/kommissionierliste.pdf',
      ]);
      expect(list, hasLength(1));
      expect(w['hinweise'], ['mehrentnahme']);
      expect(calls[1].body, '{"item_id":"i1","warehouse_id":"wh1","menge":2.5}');
      expect(calls[2].body, '{"reason":"falsch gebucht"}');
    });

    test('Fehler als ApiException', () async {
      // Anlegen erwartet 201; 200 ist ein Fehler.
      await expectLater(mock({'id': 'o1'}, () => api.createProductionOrder({})), throwsA(isA<ApiException>()));
      await expectLater(
        mock({
          'error': {'code': 'conflict', 'message': 'Aktion im aktuellen Status nicht erlaubt'}
        }, () => api.productionOrderStep('o1', 'beginnen'), status: 409),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Aktion im aktuellen Status nicht erlaubt')),
      );
      await expectLater(
        mock({
          'error': {'code': 'forbidden', 'message': 'Keine Berechtigung'}
        }, () => api.listProductionOrders(), status: 403),
        throwsA(isA<ApiException>().having((e) => e.statusCode, 'status', 403)),
      );
      // Entnehmen erwartet 201, Kommissionierliste lehnt der Server im
      // Entwurf mit 409 ab.
      await expectLater(mock({'id': 'w1'}, () => api.createProductionWithdrawal('o1', {})), throwsA(isA<ApiException>()));
      await expectLater(
        mock({
          'error': {'code': 'conflict', 'message': 'Kommissionierliste nur bei freigegebenen oder begonnenen Aufträgen'}
        }, () => api.downloadProductionPickListPdf('o1'), status: 409),
        throwsA(isA<ApiException>().having((e) => e.statusCode, 'status', 409)),
      );
      await expectLater(
          mock({
            'error': {'code': 'conflict', 'message': 'Entnahme ist bereits storniert'}
          }, () => api.stornoProductionWithdrawal('o1', 'w1', 'x'), status: 409),
          throwsA(isA<ApiException>()));
      for (final call in [
        () => api.getProductionOrder('o1'),
        () => api.updateProductionOrderDraft('o1', {}),
        () => api.stornoProductionOrder('o1', 'x'),
        () => api.releaseProductionOrder('o1', lagerId: 'wh9'),
      ]) {
        await expectLater(
            mock({
              'error': {'code': 'not_found', 'message': 'Fertigungsauftrag nicht gefunden'}
            }, call, status: 404),
            throwsA(isA<ApiException>()));
      }
    });
  });
}
