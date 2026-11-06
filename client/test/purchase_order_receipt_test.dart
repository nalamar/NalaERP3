import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/purchase_order_detail_page.dart';
import 'package:nalaerp_client/purchase_order_receipt_flow.dart';

// Wareneingang mit Bezug zur Bestellposition (Backlog H.4.x), seit H.4.z2
// am Stueck ueber die Route des Servers.

class _FakeApi extends ApiClient {
  _FakeApi(
      {this.receiveError,
      this.status = 'ordered',
      this.permissions = const {
        'purchase_orders.write',
        'stock_movements.write'
      }})
      : super(baseUrl: 'http://localhost:8080');

  final Object? receiveError;
  final String status;
  final Set<String> permissions;
  final List<Map<String, dynamic>> receives = [];
  final List<Map<String, dynamic>> movements = [];
  final List<Map<String, dynamic>> patches = [];
  int loads = 0;

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<Map<String, dynamic>> getPurchaseOrder(String id) async {
    loads++;
    return {
      'bestellung': {
        'id': id,
        'nummer': 'B-9',
        'status': status,
        'waehrung': 'EUR'
      },
      'positionen': _items,
    };
  }

  @override
  Future<Map<String, dynamic>> receivePurchaseOrder(String orderId,
      {required String warehouseId,
      String? locationId,
      bool setReceived = true}) async {
    receives.add({
      'id': orderId,
      'warehouse_id': warehouseId,
      'location_id': locationId,
      'status_received': setReceived,
    });
    if (receiveError != null) throw receiveError!;
    return {'bewegungen': 2};
  }

  @override
  Future<Map<String, dynamic>> updatePurchaseOrder(
      String id, Map<String, dynamic> patch) async {
    patches.add(patch);
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

Future<void> _open(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: PurchaseOrderDetailPage(key: UniqueKey(), api: api, id: 'po1')));
  await tester.pumpAndSettle();
}

Future<void> _receive(WidgetTester tester, _FakeApi api,
    {bool keepStatus = false}) async {
  await _open(tester, api);
  await tester.tap(find.byTooltip('Empfangen'));
  await tester.pumpAndSettle();
  await tester.tap(find.byType(DropdownButtonFormField<String>).first);
  await tester.pumpAndSettle();
  await tester.tap(find.text('L1 – Halle').last);
  await tester.pumpAndSettle();
  if (keepStatus) {
    await tester.tap(find.byType(Checkbox));
    await tester.pumpAndSettle();
  }
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

  test('receivePurchaseOrder sendet Lager, Platz nur wenn gesetzt, Status',
      () async {
    final api = ApiClient(baseUrl: 'http://localhost:8080');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      return http.Response.bytes(utf8.encode('{"bewegungen":2}'), 200,
          headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      final out = await api.receivePurchaseOrder('po1', warehouseId: ' wh1 ');
      expect(out['bewegungen'], 2);
      await api.receivePurchaseOrder('po1',
          warehouseId: 'wh1', locationId: ' loc1 ', setReceived: false);
      await api.receivePurchaseOrder('po1',
          warehouseId: 'wh1', locationId: ' ');
    }, () => client);

    expect(calls.map((c) => '${c.method} ${c.url.path}').toSet(),
        {'POST /api/v1/purchase-orders/po1/receive'});
    expect(calls.map((c) => jsonDecode(c.body)).toList(), [
      {'warehouse_id': 'wh1', 'status_received': true},
      {'warehouse_id': 'wh1', 'location_id': 'loc1', 'status_received': false},
      {'warehouse_id': 'wh1', 'status_received': true},
    ]);

    final conflict = MockClient((req) async => http.Response.bytes(
        utf8.encode(
            '{"error":{"code":"conflict","message":"bereits erfasst"}}'),
        409,
        headers: {'content-type': 'application/json'}));
    await expectLater(
        http.runWithClient(
            () => api.receivePurchaseOrder('po1', warehouseId: 'wh1'),
            () => conflict),
        throwsA(isA<ApiException>()));
  });

  testWidgets('Wareneingang ruft nur die Route des Servers', (tester) async {
    final api = _FakeApi();
    await _receive(tester, api);

    expect(api.receives, [
      {
        'id': 'po1',
        'warehouse_id': 'wh1',
        'location_id': null,
        'status_received': true
      }
    ]);
    expect(api.movements, isEmpty);
    expect(api.patches, isEmpty);
    expect(api.loads, 2);
    expect(find.text('Wareneingang gebucht'), findsOneWidget);
    expect(find.text('Wareneingang buchen'), findsNothing);
  });

  testWidgets('Ohne Haken bleibt der Status', (tester) async {
    final api = _FakeApi();
    await _receive(tester, api, keepStatus: true);

    expect(api.receives.single['status_received'], isFalse);
  });

  testWidgets('Fehler des Servers sichtbar, Dialog bleibt offen',
      (tester) async {
    final api = _FakeApi(
        receiveError: const ApiException(
            statusCode: 409,
            message: 'Wareneingang zu dieser Bestellung wurde bereits erfasst',
            code: 'conflict'));
    await _receive(tester, api);

    expect(api.receives, hasLength(1));
    expect(api.movements, isEmpty);
    expect(find.text('Wareneingang zu dieser Bestellung wurde bereits erfasst'),
        findsOneWidget);
    expect(find.text('Wareneingang gebucht'), findsNothing);
    // Offen lassen ist gefahrlos: der Server hat nichts gebucht.
    expect(find.text('Wareneingang buchen'), findsOneWidget);
  });

  testWidgets('Symbol nur mit beiden Rechten und im Status ordered',
      (tester) async {
    await _open(tester, _FakeApi());
    expect(find.byTooltip('Empfangen'), findsOneWidget);

    for (final api in [
      _FakeApi(permissions: const {'purchase_orders.write'}),
      _FakeApi(permissions: const {'stock_movements.write'}),
      _FakeApi(status: 'draft'),
      _FakeApi(status: 'received'),
    ]) {
      await _open(tester, api);
      expect(find.byTooltip('Empfangen'), findsNothing,
          reason: '${api.permissions} ${api.status}');
    }
  });
}
