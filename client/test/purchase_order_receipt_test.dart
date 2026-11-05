import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/purchase_order_detail_page.dart';
import 'package:nalaerp_client/purchase_order_receipt_flow.dart';

// Wareneingang mit Bezug zur Bestellposition, Fehler beim Status sichtbar
// (Backlog H.4.x).

class _FakeApi extends ApiClient {
  _FakeApi({this.statusError}) : super(baseUrl: 'http://localhost:8080');

  final Object? statusError;
  final List<Map<String, dynamic>> movements = [];
  final List<Map<String, dynamic>> patches = [];

  @override
  bool hasPermission(String permission) => true;

  @override
  Future<Map<String, dynamic>> getPurchaseOrder(String id) async => {
        'bestellung': {
          'id': id,
          'nummer': 'B-9',
          'status': 'ordered',
          'waehrung': 'EUR'
        },
        'positionen': _items,
      };

  @override
  Future<Map<String, dynamic>> updatePurchaseOrder(
      String id, Map<String, dynamic> patch) async {
    patches.add(patch);
    if (statusError != null) throw statusError!;
    return {'id': id};
  }

  @override
  Future<Map<String, dynamic>> createStockMovement(
      Map<String, dynamic> body) async {
    movements.add(body);
    return {'id': 'sm${movements.length}'};
  }

  @override
  Future<List<String>> listPOStatuses() async => const ['ordered'];

  @override
  Future<List<dynamic>> listMaterials(
          {String? q,
          String? typ,
          String? kategorie,
          int? limit,
          int? offset}) async =>
      const [];

  @override
  Future<List<dynamic>> listWarehouses() async => const [
        {'id': 'wh1', 'code': 'L1', 'name': 'Halle'}
      ];

  @override
  Future<List<dynamic>> listLocations(String warehouseId) async => const [];
}

const _items = [
  {
    'id': 'i1',
    'position': 1,
    'material_id': 'm1',
    'menge': 4,
    'einheit': 'm',
    'preis': 10,
  },
  {
    'id': 'i2',
    'position': 2,
    'material_id': 'm2',
    'menge': 1,
    'einheit': 'Stk',
  },
  {'id': 'i3', 'position': 3, 'bezeichnung': 'Fracht', 'menge': 1},
];

Future<void> _receive(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: PurchaseOrderDetailPage(key: UniqueKey(), api: api, id: 'po1')));
  await tester.pumpAndSettle();
  await tester.tap(find.byTooltip('Empfangen'));
  await tester.pumpAndSettle();
  await tester.tap(find.byType(DropdownButtonFormField<String>).first);
  await tester.pumpAndSettle();
  await tester.tap(find.text('L1 – Halle').last);
  await tester.pumpAndSettle();
  await tester.tap(find.text('Buchen'));
  await tester.pumpAndSettle();
}

void main() {
  test('Bewegung trägt die Bestellposition, ohne ID kein Bezug', () {
    final bodies = buildPurchaseOrderReceiptStockMovements(order: const {
      'nummer': 'B-9'
    }, items: [
      ..._items,
      {'id': ' ', 'material_id': 'm3', 'menge': 2, 'einheit': 'kg'},
      {'material_id': 'm4', 'menge': 2, 'einheit': 'kg'},
    ], warehouseId: 'wh1', defaultCurrency: 'EUR');

    expect(bodies.map((b) => b['purchase_order_item_id']).toList(),
        ['i1', 'i2', null, null]);
    expect(bodies[2].containsKey('purchase_order_item_id'), isFalse);
    expect(bodies[3].containsKey('purchase_order_item_id'), isFalse);
  });

  testWidgets('Wareneingang bucht je Position mit Bezug und setzt Status',
      (tester) async {
    final api = _FakeApi();
    await _receive(tester, api);

    expect(api.movements.map((m) => m['purchase_order_item_id']).toList(),
        ['i1', 'i2']);
    expect(api.patches, [
      {'status': 'received'}
    ]);
    expect(find.text('Wareneingang gebucht'), findsOneWidget);
    expect(find.text('Wareneingang buchen'), findsNothing);
  });

  testWidgets('Fehler beim Status wird gemeldet, Dialog schließt trotzdem',
      (tester) async {
    final api = _FakeApi(
        statusError: const ApiException(
            statusCode: 400,
            message: 'Ungültiger Statuswechsel',
            code: 'validation_error'));
    await _receive(tester, api);

    expect(api.movements, hasLength(2));
    expect(
        find.text(
            'Wareneingang gebucht, Status nicht geändert: Ungültiger Statuswechsel'),
        findsOneWidget);
    expect(find.text('Wareneingang gebucht'), findsNothing);
    // Nicht offen lassen: erneutes Buchen verdoppelte die Menge.
    expect(find.text('Wareneingang buchen'), findsNothing);
  });
}
