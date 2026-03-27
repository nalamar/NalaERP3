import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/commercial_destinations.dart';
import 'package:nalaerp_client/commercial_navigation.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/invoices_page.dart';
import 'package:nalaerp_client/pages/materialwirtschaft_screen.dart';
import 'package:nalaerp_client/pages/purchase_order_detail_page.dart';
import 'package:nalaerp_client/pages/purchase_orders_page.dart';
import 'package:nalaerp_client/pages/projects_page.dart';
import 'package:nalaerp_client/pages/stock_movements_page.dart';
import 'package:nalaerp_client/material_selection.dart';
import 'package:nalaerp_client/project_navigation.dart';
import 'package:nalaerp_client/purchase_order_item_payloads.dart';
import 'package:nalaerp_client/purchase_order_material_selection.dart';
import 'package:nalaerp_client/purchase_order_receipt_flow.dart';
import 'package:nalaerp_client/stock_movement_payloads.dart';
import 'package:nalaerp_client/pages/quotes_page.dart';
import 'package:nalaerp_client/pages/sales_orders_page.dart';

class _FakeApiClient extends ApiClient {
  _FakeApiClient({
    this.quoteList = const [],
    this.quoteDetail,
    this.invoiceList = const [],
    this.invoiceDetail,
    this.salesOrderList = const [],
    this.salesOrderDetail,
    this.salesOrderInvoices = const [],
    this.projectPhases = const [],
    this.phaseElevations = const {},
    this.elevationVariants = const {},
    this.variantMaterials = const {},
    this.invoicePayments = const [],
    this.permissions = const <String>{},
    this.convertedSalesOrder,
    this.convertedInvoice,
    this.convertedQuote,
    this.purchaseOrderList = const [],
    this.purchaseOrderDetail,
    this.purchaseOrderItems = const [],
    this.contactList = const [],
    this.materialList = const [],
    this.materialDetail,
    this.materialStock = const [],
    this.materialDocuments = const [],
    this.warehouseList = const [],
    this.locationMap = const {},
  }) : super(baseUrl: 'http://localhost:8080');

  final List<dynamic> quoteList;
  final Map<String, dynamic>? quoteDetail;
  final List<dynamic> invoiceList;
  final Map<String, dynamic>? invoiceDetail;
  final List<dynamic> salesOrderList;
  final Map<String, dynamic>? salesOrderDetail;
  final List<dynamic> salesOrderInvoices;
  final List<dynamic> projectPhases;
  final Map<String, List<dynamic>> phaseElevations;
  final Map<String, List<dynamic>> elevationVariants;
  final Map<String, Map<String, dynamic>> variantMaterials;
  final List<dynamic> invoicePayments;
  final Set<String> permissions;
  final Map<String, dynamic>? convertedSalesOrder;
  final Map<String, dynamic>? convertedInvoice;
  final Map<String, dynamic>? convertedQuote;
  final List<dynamic> purchaseOrderList;
  final Map<String, dynamic>? purchaseOrderDetail;
  final List<dynamic> purchaseOrderItems;
  final List<dynamic> contactList;
  final List<dynamic> materialList;
  final Map<String, dynamic>? materialDetail;
  final List<dynamic> materialStock;
  final List<dynamic> materialDocuments;
  final List<dynamic> warehouseList;
  final Map<String, List<dynamic>> locationMap;
  final List<Map<String, dynamic>> createdStockMovements = [];
  final List<Map<String, dynamic>> updatedPurchaseOrders = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listQuotes({
    String? q,
    String? status,
    String? contactId,
    String? projectId,
    int? limit,
    int? offset,
  }) async =>
      quoteList.where((entry) {
        final item = (entry as Map).cast<String, dynamic>();
        final matchesQuery = q == null ||
            q.trim().isEmpty ||
            [
              item['id'],
              item['number'],
              item['contact_name'],
            ].any((value) => value.toString().contains(q));
        final matchesStatus =
            status == null || status.isEmpty || item['status'] == status;
        final matchesProject = projectId == null ||
            projectId.isEmpty ||
            !item.containsKey('project_id') ||
            item['project_id']?.toString() == projectId;
        return matchesQuery && matchesStatus && matchesProject;
      }).toList();

  @override
  Future<Map<String, dynamic>> getQuote(String id) async =>
      quoteDetail ?? <String, dynamic>{'id': id};

  @override
  Future<List<dynamic>> listInvoicesOut({
    String? q,
    String? status,
    String? contactId,
    String? sourceSalesOrderId,
    int? limit,
    int? offset,
  }) async =>
      (sourceSalesOrderId != null && sourceSalesOrderId.isNotEmpty
              ? salesOrderInvoices
              : invoiceList)
          .where((entry) {
        final item = (entry as Map).cast<String, dynamic>();
        final matchesQuery = q == null ||
            q.trim().isEmpty ||
            [
              item['id'],
              item['number'],
              item['nummer'],
              item['contact_name'],
            ].any((value) => value.toString().contains(q));
        final matchesStatus =
            status == null || status.isEmpty || item['status'] == status;
        final matchesContact = contactId == null ||
            contactId.isEmpty ||
            !item.containsKey('contact_id') ||
            item['contact_id']?.toString() == contactId;
        final matchesSalesOrder = sourceSalesOrderId == null ||
            sourceSalesOrderId.isEmpty ||
            !item.containsKey('source_sales_order_id') ||
            item['source_sales_order_id']?.toString() == sourceSalesOrderId;
        return matchesQuery &&
            matchesStatus &&
            matchesContact &&
            matchesSalesOrder;
      }).toList();

  @override
  Future<Map<String, dynamic>> getInvoiceOut(String id) async =>
      invoiceDetail ?? <String, dynamic>{'id': id};

  @override
  Future<List<dynamic>> listSalesOrders({
    String? q,
    String? status,
    String? contactId,
    String? projectId,
    int? limit,
    int? offset,
  }) async =>
      salesOrderList.where((entry) {
        final item = (entry as Map).cast<String, dynamic>();
        final matchesQuery = q == null ||
            q.trim().isEmpty ||
            [
              item['id'],
              item['number'],
              item['contact_name'],
            ].any((value) => value.toString().contains(q));
        final matchesStatus =
            status == null || status.isEmpty || item['status'] == status;
        final matchesProject = projectId == null ||
            projectId.isEmpty ||
            !item.containsKey('project_id') ||
            item['project_id']?.toString() == projectId;
        return matchesQuery && matchesStatus && matchesProject;
      }).toList();

  @override
  Future<List<String>> listSalesOrderStatuses() async =>
      const ['open', 'released', 'invoiced'];

  @override
  Future<Map<String, dynamic>> getSalesOrder(String id) async =>
      salesOrderDetail ?? <String, dynamic>{'id': id};

  @override
  Future<List<dynamic>> listInvoicePayments(String id) async => invoicePayments;

  @override
  Future<List<dynamic>> listProjectPhases(String projectId) async =>
      projectPhases;

  @override
  Future<List<dynamic>> listPhaseElevations(
          String projectId, String phaseId) async =>
      phaseElevations[phaseId] ?? const [];

  @override
  Future<List<dynamic>> listElevationVariants(
          String projectId, String elevationId) async =>
      elevationVariants[elevationId] ?? const [];

  @override
  Future<Map<String, dynamic>> getVariantMaterials(
          String projectId, String variantId) async =>
      variantMaterials[variantId] ??
      const <String, dynamic>{
        'profiles': [],
        'articles': [],
        'glass': [],
      };

  @override
  Future<List<dynamic>> listPurchaseOrders({
    String? q,
    String? status,
    int? limit,
    int? offset,
  }) async =>
      purchaseOrderList.where((entry) {
        final item = (entry as Map).cast<String, dynamic>();
        final matchesQuery = q == null ||
            q.trim().isEmpty ||
            [
              item['id'],
              item['nummer'],
              item['number'],
            ].any((value) => value.toString().contains(q));
        final matchesStatus =
            status == null || status.isEmpty || item['status'] == status;
        return matchesQuery && matchesStatus;
      }).toList();

  @override
  Future<Map<String, dynamic>> getPurchaseOrder(String id) async =>
      <String, dynamic>{
        'bestellung': purchaseOrderDetail ?? <String, dynamic>{'id': id},
        'positionen': purchaseOrderItems,
      };

  @override
  Future<Map<String, dynamic>> updatePurchaseOrder(
    String id,
    Map<String, dynamic> patch,
  ) async {
    updatedPurchaseOrders.add(<String, dynamic>{'id': id, 'patch': patch});
    if (purchaseOrderDetail != null) {
      purchaseOrderDetail!.addAll(patch);
    }
    return purchaseOrderDetail ?? <String, dynamic>{'id': id, ...patch};
  }

  @override
  Future<List<String>> listPOStatuses() async =>
      const ['draft', 'ordered', 'received', 'canceled'];

  @override
  Future<List<dynamic>> listContacts({
    String? q,
    String? rolle,
    String? status,
    String? typ,
    int? limit,
    int? offset,
  }) async =>
      contactList;

  @override
  Future<List<dynamic>> listMaterials({
    String? q,
    String? typ,
    String? kategorie,
    int? limit,
    int? offset,
  }) async =>
      materialList.where((entry) {
        final item = (entry as Map).cast<String, dynamic>();
        final matchesQuery = q == null ||
            q.trim().isEmpty ||
            [
              item['id'],
              item['nummer'],
              item['bezeichnung'],
            ].any((value) => value.toString().contains(q));
        final matchesType =
            typ == null || typ.isEmpty || item['typ']?.toString() == typ;
        final matchesCategory = kategorie == null ||
            kategorie.isEmpty ||
            item['kategorie']?.toString() == kategorie;
        return matchesQuery && matchesType && matchesCategory;
      }).toList();

  @override
  Future<Map<String, dynamic>> getMaterial(String id) async =>
      materialDetail ?? <String, dynamic>{'id': id};

  @override
  Future<List<dynamic>> stockByMaterial(String id) async => materialStock;

  @override
  Future<List<dynamic>> listMaterialDocuments(String materialId) async =>
      materialDocuments;

  @override
  Future<List<String>> listMaterialTypes() async => const [];

  @override
  Future<List<String>> listMaterialCategories() async => const [];

  @override
  Future<List<Map<String, dynamic>>> listUnits() async => const [];

  @override
  Future<List<dynamic>> listWarehouses() async => warehouseList;

  @override
  Future<List<dynamic>> listLocations(String warehouseId) async =>
      locationMap[warehouseId] ?? const [];

  @override
  Future<Map<String, dynamic>> createStockMovement(
      Map<String, dynamic> body) async {
    createdStockMovements.add(Map<String, dynamic>.from(body));
    return <String, dynamic>{'id': 'sm-${createdStockMovements.length}'};
  }

  @override
  Future<Map<String, dynamic>> convertQuoteToSalesOrder(String id) async =>
      convertedSalesOrder ?? <String, dynamic>{'id': 'so-converted'};

  @override
  Future<Map<String, dynamic>> convertQuoteToInvoice(
    String id, {
    String? revenueAccount,
    DateTime? invoiceDate,
    DateTime? dueDate,
  }) async =>
      <String, dynamic>{
        'quote': convertedQuote ?? <String, dynamic>{'id': id},
        'invoice': convertedInvoice ?? <String, dynamic>{'id': 'inv-converted'},
      };

  @override
  Future<Map<String, dynamic>> convertSalesOrderToInvoice(
    String id, {
    String? revenueAccount,
    DateTime? invoiceDate,
    DateTime? dueDate,
    List<Map<String, dynamic>>? items,
  }) async =>
      <String, dynamic>{
        'sales_order': convertedSalesOrder ?? <String, dynamic>{'id': id},
        'invoice': convertedInvoice ?? <String, dynamic>{'id': 'inv-converted'},
      };
}

Future<void> _prepareLargeViewport(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(2200, 2800));
  addTearDown(() async {
    await tester.binding.setSurfaceSize(null);
  });
}

Future<void> _prepareSmallViewport(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1100, 1400));
  addTearDown(() async {
    await tester.binding.setSurfaceSize(null);
  });
}

void main() {
  test(
      'Stock-movement payload builder trims optional fields and normalizes currency',
      () {
    final payload = buildStockMovementPayload(
      materialId: ' mat-1 ',
      warehouseId: ' wh-1 ',
      locationId: ' ',
      batchCode: ' B-01 ',
      quantity: 4,
      unit: ' kg ',
      type: ' purchase ',
      reason: ' Wareneingang ',
      reference: ' BE-2026-0001 ',
      purchasePrice: 12.5,
      currency: ' eur ',
    );

    expect(
      payload,
      <String, dynamic>{
        'material_id': 'mat-1',
        'warehouse_id': 'wh-1',
        'batch_code': 'B-01',
        'menge': 4,
        'einheit': 'kg',
        'typ': 'purchase',
        'grund': 'Wareneingang',
        'referenz': 'BE-2026-0001',
        'ek_preis': 12.5,
        'waehrung': 'EUR',
      },
    );
  });

  test('Material selection resolves label, description and unit generically',
      () {
    final selection = resolveMaterialSelection(
      const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'einheit': 'kg',
        },
      ],
      'mat-1',
    );

    expect(selection, isNotNull);
    expect(selection!.label, 'MAT-100 – Profilstahl');
    expect(selection.description, 'Profilstahl');
    expect(selection.unit, 'kg');
  });

  test('Material label resolver prefers normalized label over raw id', () {
    expect(
      resolveMaterialLabel(
        const [
          {
            'id': 'mat-1',
            'nummer': 'MAT-100',
            'bezeichnung': 'Profilstahl',
            'einheit': 'kg',
          },
        ],
        'mat-1',
      ),
      'MAT-100 – Profilstahl',
    );
  });

  test('Material reference label uses bullet-separated reference format', () {
    expect(
      materialReferenceLabel(const {
        'id': 'mat-1',
        'nummer': 'MAT-100',
        'bezeichnung': 'Profilstahl',
        'einheit': 'kg',
      }),
      'MAT-100 • Profilstahl',
    );
  });

  test('Project material resolvers derive title and linked label consistently',
      () {
    const item = {
      'article_code': 'Beschlagset',
      'description': 'Beschlagset links',
      'material_nummer': 'MAT-001',
    };

    expect(resolveProjectMaterialDisplayTitle(item), 'Beschlagset');
    expect(resolveProjectLinkedMaterialLabel(item), 'MAT-001');
    expect(buildProjectLinkedMaterialSuffix(item), '  •  verknüpft: MAT-001');
    expect(
        buildProjectLinkedMaterialSuffix(const {'material_nummer': '  '}), '');
  });

  test('Project navigation helpers derive consistent note and reason labels',
      () {
    expect(buildProjectPurchaseOrderNote('PRJ-0101 • Tor 1'),
        'Projektbezug: PRJ-0101 • Tor 1');
    expect(buildProjectPurchaseOrderDescription('PRJ-0101 • Tor 1'),
        'Projektbedarf: PRJ-0101 • Tor 1');
    expect(buildLinkedProjectMaterialNote('Variante A'),
        'Projektmaterial aus Variante Variante A');
    expect(buildLinkedProjectMaterialNote(null), 'Projektmaterial');
    expect(projectFlowReasonLabel, 'Projektbedarf');
  });

  test(
      'Purchase-order material selection derives label, description and unit from a material',
      () {
    final selection = resolvePurchaseOrderMaterialSelection(
      const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'einheit': 'kg',
        },
      ],
      'mat-1',
    );

    expect(selection, isNotNull);
    expect(selection!.label, 'MAT-100 – Profilstahl');
    expect(selection.description, 'Profilstahl');
    expect(selection.unit, 'kg');
  });

  test(
      'Purchase-order item payload builder normalizes numbers, currency and delivery date',
      () {
    final payload = buildPurchaseOrderItemPayload(
      materialId: ' mat-1 ',
      description: ' Profilstahl ',
      quantityText: '4.5',
      unitText: ' kg ',
      priceText: '12.5',
      currencyText: ' eur ',
      deliveryDate: DateTime(2026, 3, 24, 16, 30),
    );

    expect(
      payload,
      <String, dynamic>{
        'bezeichnung': 'Profilstahl',
        'menge': 4.5,
        'einheit': 'kg',
        'preis': 12.5,
        'waehrung': 'EUR',
        'material_id': 'mat-1',
        'liefertermin': '2026-03-24T00:00:00.000',
      },
    );
  });

  test(
      'Purchase-order create payload builder embeds a normalized single item payload',
      () {
    final payload = buildPurchaseOrderCreatePayload(
      supplierId: 'sup-1',
      number: ' BE-2026-0001 ',
      currencyText: ' eur ',
      status: 'draft',
      note: ' Test ',
      itemPayload: buildPurchaseOrderItemPayload(
        materialId: 'mat-1',
        description: 'Profilstahl',
        quantityText: '4',
        unitText: 'kg',
        priceText: '12.5',
        currencyText: 'eur',
      ),
    );

    expect(payload['lieferant_id'], 'sup-1');
    expect(payload['nummer'], 'BE-2026-0001');
    expect(payload['waehrung'], 'EUR');
    expect(payload['status'], 'draft');
    expect(payload['notiz'], 'Test');
    expect((payload['positionen'] as List<dynamic>), hasLength(1));
    expect(
      (payload['positionen'] as List<dynamic>).single,
      containsPair('material_id', 'mat-1'),
    );
  });

  test(
      'Purchase-order receipt builder derives stock movements from material items only',
      () {
    final bodies = buildPurchaseOrderReceiptStockMovements(
      order: const {
        'nummer': 'BE-2026-0001',
      },
      items: const [
        {
          'material_id': 'mat-1',
          'menge': 4,
          'einheit': 'kg',
          'preis': 12.5,
          'waehrung': 'EUR',
        },
        {
          'bezeichnung': 'Freitext',
          'menge': 1,
          'einheit': 'Stk',
        },
      ],
      warehouseId: 'wh-1',
      locationId: 'loc-1',
      defaultCurrency: 'EUR',
    );

    expect(bodies, hasLength(1));
    expect(
      bodies.single,
      <String, dynamic>{
        'material_id': 'mat-1',
        'warehouse_id': 'wh-1',
        'location_id': 'loc-1',
        'menge': 4,
        'einheit': 'kg',
        'typ': 'purchase',
        'grund': 'Wareneingang',
        'referenz': 'BE-2026-0001',
        'ek_preis': 12.5,
        'waehrung': 'EUR',
      },
    );
  });

  testWidgets('StockMovementsPage autofills unit when a material is selected',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'stock_movements.write'},
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-200',
          'bezeichnung': 'Beschlagset',
          'einheit': 'Stk',
        },
      ],
      warehouseList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: StockMovementsPage(
          api: api,
          openCreateOnStart: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(DropdownButtonFormField<String>).first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('MAT-200 – Beschlagset').last);
    await tester.pumpAndSettle();

    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'Stk'),
      ),
      findsOneWidget,
    );
  });

  testWidgets(
      'PurchaseOrdersPage autofills description and unit when a material is selected in create dialog',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'purchase_orders.write'},
      purchaseOrderList: const [],
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'einheit': 'kg',
        },
      ],
      contactList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildPurchaseOrdersPage(
          api: api,
          openCreateOnStart: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(DropdownButtonFormField<String>).last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('MAT-100 – Profilstahl').last);
    await tester.pumpAndSettle();

    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'Profilstahl'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'kg'),
      ),
      findsOneWidget,
    );
  });

  testWidgets('PurchaseOrdersPage adopts CommercialListContext for list start',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      purchaseOrderList: const [
        {
          'id': 'po-1',
          'nummer': 'BE-2026-0001',
          'status': 'draft',
        },
        {
          'id': 'po-2',
          'nummer': 'BE-2026-0002',
          'status': 'ordered',
        },
      ],
      purchaseOrderDetail: const {
        'id': 'po-1',
        'nummer': 'BE-2026-0001',
        'status': 'draft',
        'waehrung': 'EUR',
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildPurchaseOrdersPage(
          api: api,
          initialContext: const CommercialListContext.search('BE-2026-0001'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('BE-2026-0001  •  draft'), findsOneWidget);
    expect(find.text('BE-2026-0002  •  ordered'), findsNothing);
  });

  testWidgets(
      'PurchaseOrderDetailPage books receipt movements via shared receipt builder',
      (tester) async {
    await _prepareLargeViewport(tester);
    final purchaseOrder = <String, dynamic>{
      'id': 'po-1',
      'nummer': 'BE-2026-0001',
      'status': 'ordered',
      'waehrung': 'EUR',
      'datum': '2026-03-24',
    };
    final api = _FakeApiClient(
      purchaseOrderDetail: purchaseOrder,
      purchaseOrderItems: const [
        {
          'id': 'item-1',
          'position': 1,
          'material_id': 'mat-1',
          'bezeichnung': 'Profilstahl',
          'menge': 4,
          'einheit': 'kg',
          'preis': 12.5,
          'waehrung': 'EUR',
        },
        {
          'id': 'item-2',
          'position': 2,
          'bezeichnung': 'Freitextposition',
          'menge': 1,
          'einheit': 'Stk',
          'preis': 20,
        },
      ],
      warehouseList: const [
        {
          'id': 'wh-1',
          'code': 'WH-A',
          'name': 'Hauptlager',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: PurchaseOrderDetailPage(api: api, id: 'po-1'),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Empfangen'));
    await tester.pumpAndSettle();

    await tester.tap(find.byType(DropdownButtonFormField<String>).first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('WH-A – Hauptlager').last);
    await tester.pumpAndSettle();

    await tester.tap(find.text('Buchen'));
    await tester.pumpAndSettle();

    expect(api.createdStockMovements, hasLength(1));
    expect(
      api.createdStockMovements.single,
      containsPair('material_id', 'mat-1'),
    );
    expect(
      api.createdStockMovements.single,
      containsPair('warehouse_id', 'wh-1'),
    );
    expect(
      api.createdStockMovements.single,
      containsPair('grund', 'Wareneingang'),
    );
    expect(
      api.createdStockMovements.single,
      containsPair('referenz', 'BE-2026-0001'),
    );
    expect(api.updatedPurchaseOrders, hasLength(1));
    expect(api.updatedPurchaseOrders.single['id'], 'po-1');
    expect(
      api.updatedPurchaseOrders.single['patch'],
      <String, dynamic>{'status': 'received'},
    );
  });

  testWidgets(
      'PurchaseOrderDetailPage shows resolved material label when item description is empty',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      purchaseOrderDetail: const {
        'id': 'po-1',
        'nummer': 'BE-2026-0001',
        'status': 'ordered',
        'waehrung': 'EUR',
        'datum': '2026-03-24',
      },
      purchaseOrderItems: const [
        {
          'id': 'item-1',
          'position': 1,
          'material_id': 'mat-1',
          'bezeichnung': '',
          'menge': 4,
          'einheit': 'kg',
          'preis': 12.5,
          'waehrung': 'EUR',
        },
      ],
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'einheit': 'kg',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: PurchaseOrderDetailPage(api: api, id: 'po-1'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('MAT-100 – Profilstahl'), findsOneWidget);
    expect(find.text('mat-1'), findsNothing);
  });

  testWidgets(
      'PurchaseOrdersPage opens purchase-order detail via shared destination context',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      purchaseOrderList: const [
        {
          'id': 'po-1',
          'nummer': 'BE-2026-0001',
          'status': 'draft',
          'datum': '2026-03-24',
          'waehrung': 'EUR',
        },
      ],
      purchaseOrderDetail: const {
        'id': 'po-1',
        'nummer': 'BE-2026-0001',
        'status': 'draft',
        'datum': '2026-03-24',
        'waehrung': 'EUR',
        'notiz': 'Testbestellung',
        'positionen': [],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildPurchaseOrdersPage(
          api: api,
          initialContext: const CommercialListContext.detail('po-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Bestellung BE-2026-0001'), findsOneWidget);
    expect(find.text('Positionen'), findsOneWidget);
  });

  testWidgets(
      'PurchaseOrdersPage preloads status filter via shared filter context',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      purchaseOrderList: const [
        {
          'id': 'po-1',
          'nummer': 'BE-2026-0001',
          'status': 'draft',
          'datum': '2026-03-24',
          'waehrung': 'EUR',
        },
        {
          'id': 'po-2',
          'nummer': 'BE-2026-0002',
          'status': 'ordered',
          'datum': '2026-03-24',
          'waehrung': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildPurchaseOrdersPage(
          api: api,
          initialFilters: const PurchaseOrderFilterContext(status: 'ordered'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('BE-2026-0002  •  ordered'), findsOneWidget);
    expect(find.text('BE-2026-0001  •  draft'), findsNothing);
    expect(find.text('ordered'), findsWidgets);
  });

  testWidgets(
      'MaterialwirtschaftScreen routes purchase-order context into the purchase section',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      purchaseOrderList: const [
        {
          'id': 'po-2',
          'nummer': 'BE-2026-0002',
          'status': 'ordered',
          'datum': '2026-03-24',
          'waehrung': 'EUR',
        },
        {
          'id': 'po-3',
          'nummer': 'BE-2026-0003',
          'status': 'draft',
          'datum': '2026-03-24',
          'waehrung': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: MaterialwirtschaftScreen(
          api: api,
          initialPurchaseOrdersContext:
              const CommercialListContext.search('BE-2026-0002'),
          initialPurchaseOrderFilters:
              const PurchaseOrderFilterContext(status: 'ordered'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Bereich:'), findsOneWidget);
    expect(find.text('BE-2026-0002  •  ordered'), findsOneWidget);
    expect(find.text('BE-2026-0003  •  draft'), findsNothing);
  });

  testWidgets(
      'MaterialwirtschaftScreen routes warehouse selection context into the warehouse section',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      warehouseList: const [
        {
          'id': 'wh-1',
          'code': 'WH-A',
          'name': 'Hauptlager',
        },
        {
          'id': 'wh-2',
          'code': 'WH-B',
          'name': 'AuBenlager',
        },
      ],
      locationMap: const {
        'wh-2': [
          {
            'id': 'loc-1',
            'code': 'B-01',
            'name': 'Zone B',
            'warehouse_id': 'wh-2',
          },
        ],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: MaterialwirtschaftScreen(
          api: api,
          initialWarehouseSelection:
              const WarehouseSelectionContext(warehouseId: 'wh-2'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Lagerplätze'), findsOneWidget);
    expect(find.text('WH-B – AuBenlager'), findsWidgets);
    expect(find.text('B-01 – Zone B'), findsOneWidget);
  });

  testWidgets(
      'Warehouse destination opens materialwirtschaft in warehouse section',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      warehouseList: const [
        {
          'id': 'wh-1',
          'code': 'WH-A',
          'name': 'Hauptlager',
        },
        {
          'id': 'wh-2',
          'code': 'WH-B',
          'name': 'AuBenlager',
        },
      ],
      locationMap: const {
        'wh-2': [
          {
            'id': 'loc-1',
            'code': 'B-01',
            'name': 'Zone B',
            'warehouse_id': 'wh-2',
          },
        ],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildWarehouseSelectionDestination(
          api: api,
          warehouseId: 'wh-2',
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Lagerplätze'), findsOneWidget);
    expect(find.text('WH-B – AuBenlager'), findsWidgets);
    expect(find.text('B-01 – Zone B'), findsOneWidget);
  });

  testWidgets(
      'MaterialwirtschaftScreen routes stock-movement prefill into the movement dialog',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'stock_movements.write'},
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
        },
      ],
      warehouseList: const [
        {
          'id': 'wh-2',
          'code': 'WH-B',
          'name': 'AuBenlager',
        },
      ],
      locationMap: const {
        'wh-2': [
          {
            'id': 'loc-1',
            'code': 'B-01',
            'name': 'Zone B',
            'warehouse_id': 'wh-2',
          },
        ],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: MaterialwirtschaftScreen(
          api: api,
          initialStockMovementPrefill: const StockMovementPrefillContext(
            materialId: 'mat-1',
            warehouseId: 'wh-2',
            locationId: 'loc-1',
            type: 'purchase',
            reason: 'Wareneingang',
            reference: 'BE-2026-0002',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Bestandsbewegung'), findsOneWidget);
    expect(find.text('MAT-100 – Profilstahl'), findsWidgets);
    expect(find.text('WH-B – AuBenlager'), findsWidgets);
    expect(find.text('B-01 – Zone B'), findsWidgets);
    expect(find.widgetWithText(TextField, 'Wareneingang'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'BE-2026-0002'), findsOneWidget);
  });

  testWidgets(
      'Stock-movement destination opens materialwirtschaft with shared prefill context',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'stock_movements.write'},
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'einheit': 'kg',
        },
      ],
      warehouseList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildStockMovementCreateDestination(
          api: api,
          initialPrefill: const StockMovementPrefillContext(
            materialId: 'mat-1',
            reason: 'Projektbedarf',
            reference: 'PRJ-0101',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Bestandsbewegung'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'Projektbedarf'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'PRJ-0101'), findsOneWidget);
  });

  test(
      'Material purchase-order destination forwards shared material prefill context',
      () {
    final api = _FakeApiClient();
    final screen = buildMaterialPurchaseOrderCreateDestination(
      api: api,
      materialContext: MaterialNavigationContext.fromMaterial(const {
        'id': 'mat-1',
        'nummer': 'MAT-100',
        'bezeichnung': 'Profilstahl',
        'einheit': 'kg',
      }),
    );

    expect(
      screen.initialPurchaseOrderCreatePrefill?.normalizedNote,
      'Materialbezug: MAT-100 • Profilstahl',
    );
    expect(
      screen.initialPurchaseOrderCreatePrefill?.normalizedItemDescription,
      'MAT-100 • Profilstahl',
    );
    expect(
      screen.initialPurchaseOrderCreatePrefill?.normalizedItemMaterialId,
      'mat-1',
    );
    expect(
      screen.initialPurchaseOrderCreatePrefill?.normalizedItemQuantity,
      1,
    );
    expect(
      screen.initialPurchaseOrderCreatePrefill?.normalizedItemUnit,
      'kg',
    );
  });

  test('Material purchase-order prefill constructor derives shared defaults',
      () {
    const prefill = PurchaseOrderCreatePrefillContext.material(
      materialLabel: 'MAT-100 • Profilstahl',
      materialId: 'mat-1',
      unit: 'kg',
    );

    expect(prefill.normalizedNote, 'Materialbezug: MAT-100 • Profilstahl');
    expect(prefill.normalizedItemDescription, 'MAT-100 • Profilstahl');
    expect(prefill.normalizedItemMaterialId, 'mat-1');
    expect(prefill.normalizedItemQuantity, 1);
    expect(prefill.normalizedItemUnit, 'kg');
  });

  test(
      'Purchase-order flow destination forwards shared project context and prefill',
      () {
    final api = _FakeApiClient();
    final screen = buildPurchaseOrderFlowDestination(
      api: api,
      destinationContext: ProjectCommercialNavigationContext.fromProject(const {
        'id': 'project-1',
        'nummer': 'PRJ-0101',
        'name': 'Tor 1',
      }),
    );

    expect(
      screen.initialPurchaseOrdersContext?.normalizedSearchQuery,
      'PRJ-0101',
    );
    expect(
      screen.initialPurchaseOrderCreatePrefill?.normalizedNote,
      'Projektbezug: PRJ-0101 • Tor 1',
    );
  });

  test('Material list destination forwards a shared material context', () {
    final api = _FakeApiClient();
    final screen = buildMaterialListDestination(
      api: api,
      initialContext: const MaterialListContext(
        detailId: 'mat-1',
        searchQuery: 'MAT-100',
        type: 'rohstoff',
      ),
    );

    expect(screen.initialMaterialsContext?.normalizedDetailId, 'mat-1');
    expect(screen.initialMaterialsContext?.normalizedSearchQuery, 'MAT-100');
    expect(screen.initialMaterialsContext?.normalizedType, 'rohstoff');
  });

  test('Material detail flow destination forwards shared detail context', () {
    final api = _FakeApiClient();
    final screen = buildMaterialDetailFlowDestination(
      api: api,
      destinationContext: MaterialNavigationContext.fromMaterial(const {
        'id': 'mat-1',
        'nummer': 'MAT-100',
        'bezeichnung': 'Profilstahl',
        'einheit': 'kg',
      }),
    );

    expect(screen.initialMaterialsContext?.normalizedDetailId, 'mat-1');
  });

  test(
      'Material detail destination still delegates through shared material context',
      () {
    final api = _FakeApiClient();
    final screen = buildMaterialDetailDestination(
      api: api,
      materialId: ' mat-1 ',
    );

    expect(screen.initialMaterialsContext?.normalizedDetailId, 'mat-1');
    expect(screen.initialMaterialsContext?.effectiveSearchQuery, 'mat-1');
  });

  test(
      'Material stock-movement destination forwards shared material prefill context',
      () {
    final api = _FakeApiClient();
    final screen = buildMaterialStockMovementCreateDestination(
      api: api,
      materialContext: MaterialNavigationContext.fromMaterial(const {
        'id': 'mat-1',
        'nummer': 'MAT-100',
        'bezeichnung': 'Profilstahl',
        'einheit': 'kg',
      }),
    );

    expect(screen.initialStockMovementPrefill?.normalizedMaterialId, 'mat-1');
    expect(
      screen.initialStockMovementPrefill?.normalizedReference,
      'MAT-100 • Profilstahl',
    );
  });

  test('Stock-movement flow destination forwards shared project reason context',
      () {
    final api = _FakeApiClient();
    final screen = buildStockMovementFlowDestination(
      api: api,
      destinationContext: ProjectCommercialNavigationContext.fromProject(const {
        'id': 'project-1',
        'nummer': 'PRJ-0101',
        'name': 'Tor 1',
      }),
    );

    expect(
      screen.initialStockMovementPrefill?.normalizedReference,
      'PRJ-0101',
    );
    expect(
      screen.initialStockMovementPrefill?.normalizedReason,
      'Projektbedarf',
    );
  });

  test('Stock-movement reference constructor normalizes shared reference data',
      () {
    const prefill = StockMovementPrefillContext.reference(
      materialId: ' mat-1 ',
      reason: ' Projektbedarf ',
      reference: ' PRJ-0101 ',
    );

    expect(prefill.normalizedMaterialId, 'mat-1');
    expect(prefill.normalizedReason, 'Projektbedarf');
    expect(prefill.normalizedReference, 'PRJ-0101');
  });

  test('Warehouse list destination forwards a shared warehouse selection', () {
    final api = _FakeApiClient();
    final screen = buildWarehouseListDestination(
      api: api,
      initialSelection: const WarehouseSelectionContext(warehouseId: 'wh-2'),
    );

    expect(screen.initialSection, 1);
    expect(screen.initialWarehouseSelection?.normalizedWarehouseId, 'wh-2');
  });

  test(
      'Legacy raw wrapper destinations still normalize through shared contexts',
      () {
    final api = _FakeApiClient();
    final stockMovementScreen = buildMaterialStockMovementDestination(
      api: api,
      materialId: ' mat-1 ',
      reference: ' MAT-100 ',
      reason: ' Projektbedarf ',
    );
    final warehouseScreen = buildWarehouseSelectionDestination(
      api: api,
      warehouseId: ' wh-2 ',
    );

    expect(
      stockMovementScreen.initialStockMovementPrefill?.normalizedMaterialId,
      'mat-1',
    );
    expect(
      stockMovementScreen.initialStockMovementPrefill?.normalizedReference,
      'MAT-100',
    );
    expect(
      stockMovementScreen.initialStockMovementPrefill?.normalizedReason,
      'Projektbedarf',
    );
    expect(
      warehouseScreen.initialWarehouseSelection?.normalizedWarehouseId,
      'wh-2',
    );
  });

  testWidgets(
      'MaterialwirtschaftScreen routes material context into the materials section',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'typ': 'rohstoff',
          'einheit': 'kg',
          'kategorie': 'Stahl',
        },
        {
          'id': 'mat-2',
          'nummer': 'MAT-200',
          'bezeichnung': 'Aluminiumblech',
          'typ': 'halbzeug',
          'einheit': 'm2',
          'kategorie': 'Alu',
        },
      ],
      materialDetail: const {
        'id': 'mat-1',
        'nummer': 'MAT-100',
        'bezeichnung': 'Profilstahl',
        'typ': 'rohstoff',
        'einheit': 'kg',
        'kategorie': 'Stahl',
      },
      materialStock: const [],
      materialDocuments: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: MaterialwirtschaftScreen(
          api: api,
          initialMaterialsContext: const MaterialListContext(
            detailId: 'mat-1',
            searchQuery: 'MAT-100',
            type: 'rohstoff',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Profilstahl'), findsWidgets);
    expect(find.text('Aluminiumblech'), findsNothing);
    expect(find.text('Bitte ein Material auswählen'), findsNothing);
  });

  testWidgets(
      'Purchase-order create destination opens purchase section with prefilled dialog',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'purchase_orders.write'},
      purchaseOrderList: const [],
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-001',
          'bezeichnung': 'Beschlagset links',
        },
      ],
      contactList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildPurchaseOrderCreateDestination(
          api: api,
          initialContext: const CommercialListContext.search('PRJ-0101'),
          initialCreatePrefill: const PurchaseOrderCreatePrefillContext(
            note: 'Projektbezug: PRJ-0101',
            itemDescription: 'Projektbedarf: PRJ-0101',
            itemMaterialId: 'mat-1',
            itemQuantity: 2,
            itemUnit: 'Stk',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Bestellung anlegen'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'PRJ-0101'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'Projektbezug: PRJ-0101'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'Projektbedarf: PRJ-0101'),
      ),
      findsOneWidget,
    );
  });

  testWidgets(
      'MaterialsPage reuses shared stock-movement follow-up action for selected material',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'stock_movements.write',
        'purchase_orders.write',
      },
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'typ': 'rohstoff',
          'einheit': 'kg',
          'kategorie': 'Stahl',
        },
      ],
      materialDetail: const {
        'id': 'mat-1',
        'nummer': 'MAT-100',
        'bezeichnung': 'Profilstahl',
        'typ': 'rohstoff',
        'einheit': 'kg',
        'kategorie': 'Stahl',
      },
      materialStock: const [],
      materialDocuments: const [],
      warehouseList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildMaterialsPage(
          api: api,
          initialContext: const MaterialListContext.detail('mat-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Material'), findsNothing);
    expect(find.text('Bewegung'), findsOneWidget);
    expect(find.text('Bestellen'), findsOneWidget);

    await tester.tap(find.text('Bewegung'));
    await tester.pumpAndSettle();

    expect(find.text('Bestandsbewegung'), findsOneWidget);
    expect(find.text('MAT-100 – Profilstahl'), findsWidgets);
    expect(find.widgetWithText(TextField, 'MAT-100 • Profilstahl'),
        findsOneWidget);
  });

  testWidgets(
      'MaterialsPage reuses shared purchase-order follow-up action for selected material',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'stock_movements.write',
        'purchase_orders.write',
      },
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'typ': 'rohstoff',
          'einheit': 'kg',
          'kategorie': 'Stahl',
        },
      ],
      materialDetail: const {
        'id': 'mat-1',
        'nummer': 'MAT-100',
        'bezeichnung': 'Profilstahl',
        'typ': 'rohstoff',
        'einheit': 'kg',
        'kategorie': 'Stahl',
      },
      materialStock: const [],
      materialDocuments: const [],
      warehouseList: const [],
      purchaseOrderList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildMaterialsPage(
          api: api,
          initialContext: const MaterialListContext.detail('mat-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Material'), findsNothing);
    expect(find.text('Bewegung'), findsOneWidget);
    expect(find.text('Bestellen'), findsOneWidget);

    await tester.tap(find.text('Bestellen'));
    await tester.pumpAndSettle();

    expect(find.text('Bestellung anlegen'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(
            TextFormField, 'Materialbezug: MAT-100 • Profilstahl'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'MAT-100 • Profilstahl'),
      ),
      findsWidgets,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, '1'),
      ),
      findsOneWidget,
    );
  });

  testWidgets(
      'MaterialsPage create dialog inherits active material context filters',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'materials.write'},
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-100',
          'bezeichnung': 'Profilstahl',
          'typ': 'rohstoff',
          'einheit': 'kg',
          'kategorie': 'Stahl',
        },
      ],
      materialDetail: const {
        'id': 'mat-1',
        'nummer': 'MAT-100',
        'bezeichnung': 'Profilstahl',
        'typ': 'rohstoff',
        'einheit': 'kg',
        'kategorie': 'Stahl',
      },
      materialStock: const [],
      materialDocuments: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildMaterialsPage(
          api: api,
          initialContext: const MaterialListContext(
            searchQuery: 'MAT-100',
            type: 'rohstoff',
            category: 'Stahl',
          ),
          openCreateOnStart: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Material anlegen'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'rohstoff'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'Stahl'),
      ),
      findsOneWidget,
    );
  });

  testWidgets(
      'WarehousesPage create dialog starts in location mode for the selected warehouse context',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'warehouses.write'},
      warehouseList: const [
        {
          'id': 'wh-1',
          'code': 'WH-A',
          'name': 'Hauptlager',
        },
        {
          'id': 'wh-2',
          'code': 'WH-B',
          'name': 'AuBenlager',
        },
      ],
      locationMap: const {
        'wh-2': [
          {
            'id': 'loc-1',
            'code': 'B-01',
            'name': 'Zone B',
            'warehouse_id': 'wh-2',
          },
        ],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildWarehousesPage(
          api: api,
          initialSelection:
              const WarehouseSelectionContext(warehouseId: 'wh-2'),
          openCreateOnStart: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Neu anlegen'), findsOneWidget);
    expect(find.text('Für Lager: AuBenlager'), findsOneWidget);
    expect(find.text('Lagerplatz'), findsWidgets);
  });

  testWidgets(
      'Quote conversion keeps project filter when opening the created sales order',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'quotes.write',
        'sales_orders.read',
        'sales_orders.write'
      },
      quoteList: const [
        {
          'id': 'q-1',
          'number': 'ANG-2026-0001',
          'contact_name': 'Projekt GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
          'project_id': 'project-1',
        },
      ],
      quoteDetail: const {
        'id': 'q-1',
        'number': 'ANG-2026-0001',
        'status': 'accepted',
        'contact_name': 'Projekt GmbH',
        'gross_amount': 1190.0,
        'currency': 'EUR',
        'project_id': 'project-1',
        'items': [],
      },
      convertedSalesOrder: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
      },
      salesOrderList: const [
        {
          'id': 'so-1',
          'number': 'AUF-2026-0001',
          'contact_name': 'Projekt GmbH',
          'status': 'released',
          'gross_amount': 1428.0,
          'currency': 'EUR',
          'project_id': 'project-1',
        },
        {
          'id': 'so-9',
          'number': 'AUF-2026-0009',
          'contact_name': 'Andere GmbH',
          'status': 'open',
          'gross_amount': 500.0,
          'currency': 'EUR',
          'project_id': 'project-2',
        },
      ],
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'contact_name': 'Projekt GmbH',
        'status': 'released',
        'order_date': '2026-03-21',
        'gross_amount': 1428.0,
        'net_amount': 1200.0,
        'tax_amount': 228.0,
        'currency': 'EUR',
        'items': [],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildQuotesPage(
          api: api,
          initialFilters: const CommercialFilterContext(projectId: 'project-1'),
          initialContext: const CommercialListContext.detail('q-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('In Auftrag'));
    await tester.pumpAndSettle();

    expect(find.text('Aufträge'), findsOneWidget);
    expect(find.text('AUF-2026-0001'), findsWidgets);
    expect(find.text('AUF-2026-0009'), findsNothing);
    expect(find.widgetWithText(TextField, 'project-1'), findsOneWidget);
  });

  testWidgets(
      'Sales-order invoice conversion keeps sales-order filter when opening the created invoice',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'sales_orders.read',
        'sales_orders.write',
        'invoices_out.read',
        'invoices_out.write'
      },
      salesOrderList: const [
        {
          'id': 'so-1',
          'number': 'AUF-2026-0001',
          'contact_name': 'Projekt GmbH',
          'status': 'released',
          'gross_amount': 1428.0,
          'currency': 'EUR',
          'project_id': 'project-1',
        },
      ],
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'contact_name': 'Projekt GmbH',
        'status': 'released',
        'order_date': '2026-03-21',
        'gross_amount': 1428.0,
        'net_amount': 1200.0,
        'tax_amount': 228.0,
        'currency': 'EUR',
        'project_id': 'project-1',
        'source_quote_id': 'q-1',
        'items': [
          {
            'id': 'item-1',
            'description': 'Montage',
            'qty': 4,
            'unit': 'Std',
            'unit_price': 300.0,
            'tax_code': 'DE19',
            'invoiced_qty': 2,
            'remaining_qty': 2,
          },
        ],
      },
      convertedSalesOrder: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'contact_name': 'Projekt GmbH',
        'status': 'invoiced',
        'order_date': '2026-03-21',
        'gross_amount': 1428.0,
        'net_amount': 1200.0,
        'tax_amount': 228.0,
        'currency': 'EUR',
        'project_id': 'project-1',
        'source_quote_id': 'q-1',
        'linked_invoice_out_id': 'inv-2',
        'items': [
          {
            'id': 'item-1',
            'description': 'Montage',
            'qty': 4,
            'unit': 'Std',
            'unit_price': 300.0,
            'tax_code': 'DE19',
            'invoiced_qty': 4,
            'remaining_qty': 0,
          },
        ],
      },
      convertedInvoice: const {
        'id': 'inv-2',
        'number': 'RE-2026-0002',
        'contact_name': 'Projekt GmbH',
        'contact_id': 'contact-1',
        'status': 'draft',
        'invoice_date': '2026-03-21',
        'due_date': '2026-04-04',
        'gross_amount': 714.0,
        'paid_amount': 214.0,
        'currency': 'EUR',
        'source_sales_order_id': 'so-1',
        'items': [],
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-2',
          'number': 'RE-2026-0002',
          'contact_name': 'Projekt GmbH',
          'status': 'draft',
          'gross_amount': 714.0,
          'paid_amount': 214.0,
          'currency': 'EUR',
          'source_sales_order_id': 'so-1',
        },
      ],
      invoiceList: const [
        {
          'id': 'inv-9',
          'number': 'RE-2026-0009',
          'contact_name': 'Andere GmbH',
          'status': 'draft',
          'gross_amount': 500.0,
          'paid_amount': 0.0,
          'currency': 'EUR',
        },
      ],
      invoiceDetail: const {
        'id': 'inv-2',
        'number': 'RE-2026-0002',
        'contact_name': 'Projekt GmbH',
        'contact_id': 'contact-1',
        'status': 'draft',
        'invoice_date': '2026-03-21',
        'due_date': '2026-04-04',
        'gross_amount': 714.0,
        'paid_amount': 214.0,
        'currency': 'EUR',
        'source_sales_order_id': 'so-1',
        'items': [],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildSalesOrdersPage(
          api: api,
          initialFilters: const CommercialFilterContext(projectId: 'project-1'),
          initialContext: const CommercialListContext.detail('so-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('In Rechnung weiterführen'));
    await tester.pumpAndSettle();
    await tester.tap(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.text('Rechnung erzeugen'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Ausgangsrechnungen'), findsOneWidget);
    expect(find.text('RE-2026-0002'), findsWidgets);
    expect(find.text('RE-2026-0009'), findsNothing);
    expect(find.widgetWithText(TextField, 'so-1'), findsOneWidget);
  });

  testWidgets(
      'CommercialFilterContext preloads project and sales-order filters consistently',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      quoteList: const [
        {
          'id': 'q-project',
          'number': 'ANG-2026-0101',
          'contact_name': 'Projekt GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
          'project_id': 'project-1',
        },
        {
          'id': 'q-other',
          'number': 'ANG-2026-0102',
          'contact_name': 'Andere GmbH',
          'status': 'sent',
          'gross_amount': 500.0,
          'currency': 'EUR',
          'project_id': 'project-2',
        },
      ],
      salesOrderList: const [
        {
          'id': 'so-project',
          'number': 'AUF-2026-0101',
          'contact_name': 'Projekt GmbH',
          'status': 'released',
          'gross_amount': 1428.0,
          'currency': 'EUR',
          'project_id': 'project-1',
        },
        {
          'id': 'so-other',
          'number': 'AUF-2026-0102',
          'contact_name': 'Andere GmbH',
          'status': 'open',
          'gross_amount': 500.0,
          'currency': 'EUR',
          'project_id': 'project-2',
        },
      ],
      salesOrderInvoices: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'contact_name': 'Projekt GmbH',
          'status': 'draft',
          'gross_amount': 714.0,
          'paid_amount': 214.0,
          'currency': 'EUR',
          'source_sales_order_id': 'so-1',
        },
      ],
      invoiceList: const [
        {
          'id': 'inv-other',
          'number': 'RE-2026-0099',
          'contact_name': 'Andere GmbH',
          'status': 'draft',
          'gross_amount': 500.0,
          'paid_amount': 0.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildQuotesPage(
          api: api,
          initialFilters: const CommercialFilterContext(projectId: 'project-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('ANG-2026-0101'), findsOneWidget);
    expect(find.text('ANG-2026-0102'), findsNothing);
    expect(find.widgetWithText(TextField, 'project-1'), findsOneWidget);

    await tester.pumpWidget(
      MaterialApp(
        home: buildSalesOrdersPage(
          api: api,
          initialFilters: const CommercialFilterContext(projectId: 'project-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('AUF-2026-0101'), findsOneWidget);
    expect(find.text('AUF-2026-0102'), findsNothing);
    expect(find.widgetWithText(TextField, 'project-1'), findsOneWidget);

    await tester.pumpWidget(
      MaterialApp(
        home: buildInvoicesPage(
          api: api,
          initialFilters:
              const CommercialFilterContext(sourceSalesOrderId: 'so-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('RE-2026-0001'), findsOneWidget);
    expect(find.text('RE-2026-0099'), findsNothing);
    expect(find.widgetWithText(TextField, 'so-1'), findsOneWidget);
  });

  testWidgets(
      'CommercialFilterContext also preloads project-bound quote creation flow',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient();

    await tester.pumpWidget(
      MaterialApp(
        home: buildQuotesPage(
          api: api,
          initialFilters: const CommercialFilterContext(projectId: 'project-1'),
          openCreateOnStart: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Angebot anlegen'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextField, 'project-1'),
      ),
      findsOneWidget,
    );
  });

  testWidgets(
      'QuotesPage create action inherits the active project filter into the dialog',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'quotes.write'},
      quoteList: const [
        {
          'id': 'q-project',
          'number': 'ANG-2026-0101',
          'contact_name': 'Projekt GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
          'project_id': 'project-1',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: buildQuotesPage(
          api: api,
          initialFilters: const CommercialFilterContext(projectId: 'project-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Angebot'));
    await tester.pumpAndSettle();

    expect(find.text('Angebot anlegen'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextField, 'project-1'),
      ),
      findsOneWidget,
    );
  });

  testWidgets(
      'CommercialListContext opens quote, sales-order and invoice details consistently',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      quoteList: const [
        {
          'id': 'q-1',
          'number': 'ANG-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
        },
      ],
      quoteDetail: const {
        'id': 'q-1',
        'number': 'ANG-2026-0001',
        'status': 'accepted',
        'contact_name': 'Muster GmbH',
        'gross_amount': 1190.0,
        'net_amount': 1000.0,
        'tax_amount': 190.0,
        'currency': 'EUR',
        'items': [],
      },
      salesOrderList: const [
        {
          'id': 'so-1',
          'number': 'AUF-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'released',
          'gross_amount': 1428.0,
          'currency': 'EUR',
        },
      ],
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'contact_name': 'Muster GmbH',
        'status': 'released',
        'order_date': '2026-03-21',
        'gross_amount': 1428.0,
        'net_amount': 1200.0,
        'tax_amount': 228.0,
        'currency': 'EUR',
        'items': [],
      },
      invoiceList: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
        },
      ],
      invoiceDetail: const {
        'id': 'inv-1',
        'number': 'RE-2026-0001',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'status': 'draft',
        'invoice_date': '2026-03-21',
        'due_date': '2026-04-04',
        'gross_amount': 1428.0,
        'paid_amount': 428.0,
        'currency': 'EUR',
        'items': [],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: QuotesPage(
          api: api,
          initialContext: const CommercialListContext.detail('q-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('ANG-2026-0001'), findsWidgets);

    await tester.pumpWidget(
      MaterialApp(
        home: SalesOrdersPage(
          api: api,
          initialContext: const CommercialListContext.detail('so-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('AUF-2026-0001'), findsWidgets);

    await tester.pumpWidget(
      MaterialApp(
        home: InvoicesPage(
          api: api,
          initialContext: const CommercialListContext.detail('inv-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('RE-2026-0001'), findsWidgets);
  });

  testWidgets('Legacy filter parameters still resolve through page fallbacks',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      quoteList: const [
        {
          'id': 'q-project',
          'number': 'ANG-2026-0101',
          'contact_name': 'Projekt GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
          'project_id': 'project-1',
        },
        {
          'id': 'q-other',
          'number': 'ANG-2026-0102',
          'contact_name': 'Andere GmbH',
          'status': 'sent',
          'gross_amount': 500.0,
          'currency': 'EUR',
          'project_id': 'project-2',
        },
      ],
      salesOrderList: const [
        {
          'id': 'so-project',
          'number': 'AUF-2026-0101',
          'contact_name': 'Projekt GmbH',
          'status': 'released',
          'gross_amount': 1428.0,
          'currency': 'EUR',
          'project_id': 'project-1',
        },
        {
          'id': 'so-other',
          'number': 'AUF-2026-0102',
          'contact_name': 'Andere GmbH',
          'status': 'open',
          'gross_amount': 500.0,
          'currency': 'EUR',
          'project_id': 'project-2',
        },
      ],
      salesOrderInvoices: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'contact_name': 'Projekt GmbH',
          'status': 'draft',
          'gross_amount': 714.0,
          'paid_amount': 214.0,
          'currency': 'EUR',
          'source_sales_order_id': 'so-1',
        },
      ],
      invoiceList: const [
        {
          'id': 'inv-other',
          'number': 'RE-2026-0099',
          'contact_name': 'Andere GmbH',
          'status': 'draft',
          'gross_amount': 500.0,
          'paid_amount': 0.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: QuotesPage(
          api: api,
          initialProjectId: 'project-1',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('ANG-2026-0101'), findsOneWidget);
    expect(find.text('ANG-2026-0102'), findsNothing);

    await tester.pumpWidget(
      MaterialApp(
        home: SalesOrdersPage(
          api: api,
          initialProjectId: 'project-1',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('AUF-2026-0101'), findsOneWidget);
    expect(find.text('AUF-2026-0102'), findsNothing);

    await tester.pumpWidget(
      MaterialApp(
        home: InvoicesPage(
          api: api,
          initialSourceSalesOrderId: 'so-1',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('RE-2026-0001'), findsOneWidget);
    expect(find.text('RE-2026-0099'), findsNothing);
  });

  testWidgets(
      'Legacy initial detail parameters still resolve through page fallbacks',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      quoteList: const [
        {
          'id': 'q-1',
          'number': 'ANG-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
        },
      ],
      quoteDetail: const {
        'id': 'q-1',
        'number': 'ANG-2026-0001',
        'status': 'accepted',
        'contact_name': 'Muster GmbH',
        'gross_amount': 1190.0,
        'net_amount': 1000.0,
        'tax_amount': 190.0,
        'currency': 'EUR',
        'items': [],
      },
      salesOrderList: const [
        {
          'id': 'so-1',
          'number': 'AUF-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'released',
          'gross_amount': 1428.0,
          'currency': 'EUR',
        },
      ],
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'contact_name': 'Muster GmbH',
        'status': 'released',
        'order_date': '2026-03-21',
        'gross_amount': 1428.0,
        'net_amount': 1200.0,
        'tax_amount': 228.0,
        'currency': 'EUR',
        'items': [],
      },
      invoiceList: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
        },
      ],
      invoiceDetail: const {
        'id': 'inv-1',
        'number': 'RE-2026-0001',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'status': 'draft',
        'invoice_date': '2026-03-21',
        'due_date': '2026-04-04',
        'gross_amount': 1428.0,
        'paid_amount': 428.0,
        'currency': 'EUR',
        'items': [],
      },
    );

    await tester.pumpWidget(
      MaterialApp(
        home: QuotesPage(
          api: api,
          initialQuoteId: 'q-1',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('ANG-2026-0001'), findsWidgets);

    await tester.pumpWidget(
      MaterialApp(
        home: SalesOrdersPage(
          api: api,
          initialSalesOrderId: 'so-1',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('AUF-2026-0001'), findsWidgets);

    await tester.pumpWidget(
      MaterialApp(
        home: InvoicesPage(
          api: api,
          initialInvoiceId: 'inv-1',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('RE-2026-0001'), findsWidgets);
  });

  testWidgets(
      'QuotesPage shows linked sales order context with formatted value',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      quoteList: const [
        {
          'id': 'q-1',
          'number': 'ANG-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
        },
      ],
      quoteDetail: const {
        'id': 'q-1',
        'number': 'ANG-2026-0001',
        'status': 'accepted',
        'contact_name': 'Muster GmbH',
        'project_name': 'Tor 1',
        'note': 'Test',
        'quote_date': '2026-03-21',
        'valid_until': '2026-04-21',
        'gross_amount': 1190.0,
        'net_amount': 1000.0,
        'tax_amount': 190.0,
        'currency': 'EUR',
        'linked_sales_order_id': 'so-1',
        'items': [
          {
            'description': 'Montage',
            'qty': 2,
            'unit': 'Std',
            'unit_price': 500.0,
            'tax_code': 'DE19',
          },
        ],
      },
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'status': 'released',
        'gross_amount': 1428.0,
        'currency': 'EUR',
        'items': [
          {'id': 'item-1'},
          {'id': 'item-2'},
        ],
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-2',
          'number': 'RE-2026-0002',
          'status': 'draft',
          'gross_amount': 714.0,
          'paid_amount': 214.0,
          'currency': 'EUR',
        },
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'status': 'booked',
          'gross_amount': 714.0,
          'paid_amount': 714.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: QuotesPage(
          api: api,
          initialContext: const CommercialListContext.detail('q-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Verknüpfter Auftrag AUF-2026-0001'), findsOneWidget);
    expect(find.text('Auftragswert: 1428.00 EUR'), findsOneWidget);
    expect(find.text('Positionen: 2'), findsOneWidget);
    expect(find.text('Rechnungen aus Auftrag (2)'), findsOneWidget);
    expect(find.textContaining('Letzte Rechnung'), findsWidgets);
    expect(find.text('Brutto: 1190.00 EUR'), findsOneWidget);
    expect(find.text('Netto: 1000.00 EUR'), findsOneWidget);
    expect(find.text('Steuer: 190.00 EUR'), findsOneWidget);
  });

  testWidgets('QuotesPage filters list to offers with follow-up documents',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      quoteList: const [
        {
          'id': 'q-follow-up',
          'number': 'ANG-2026-0101',
          'contact_name': 'Muster GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
          'linked_sales_order_id': 'so-1',
        },
        {
          'id': 'q-open',
          'number': 'ANG-2026-0102',
          'contact_name': 'Offen GmbH',
          'status': 'sent',
          'gross_amount': 750.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: QuotesPage(api: api),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('ANG-2026-0101'), findsOneWidget);
    expect(find.text('ANG-2026-0102'), findsOneWidget);
    expect(find.textContaining('In Auftrag so-1 überführt'), findsOneWidget);

    await tester.tap(find.text('Mit Folgebeleg'));
    await tester.pumpAndSettle();

    expect(find.text('ANG-2026-0101'), findsOneWidget);
    expect(find.text('ANG-2026-0102'), findsNothing);
  });

  testWidgets(
      'InvoicesPage shows source sales order context with formatted values',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      invoiceList: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
        },
      ],
      invoiceDetail: const {
        'id': 'inv-1',
        'number': 'RE-2026-0001',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'status': 'draft',
        'invoice_date': '2026-03-21',
        'due_date': '2026-04-04',
        'gross_amount': 1428.0,
        'paid_amount': 428.0,
        'currency': 'EUR',
        'source_sales_order_id': 'so-1',
        'items': [
          {
            'description': 'Montage',
            'qty': 2,
            'unit_price': 500.0,
            'tax_code': 'DE19',
            'account_code': '8000',
          },
        ],
      },
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'status': 'invoiced',
        'gross_amount': 1428.0,
        'currency': 'EUR',
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
        },
        {
          'id': 'inv-2',
          'number': 'RE-2026-0002',
          'status': 'booked',
          'gross_amount': 714.0,
          'paid_amount': 714.0,
          'currency': 'EUR',
        },
      ],
      invoicePayments: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: InvoicesPage(
          api: api,
          initialContext: const CommercialListContext.detail('inv-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Auftrag AUF-2026-0001'), findsOneWidget);
    expect(find.text('Auftragsstatus: invoiced  ·  Auftragswert: 1428.00 EUR'),
        findsOneWidget);
    expect(find.text('Weitere Folgebelege aus Auftrag (2)'), findsOneWidget);
    expect(find.textContaining('Aktuelle Rechnung'), findsWidgets);
    expect(
        find.text('Brutto: 1428.00 EUR  Offen: 1000.00 EUR'), findsOneWidget);
    expect(find.text('Menge 2 x 500.00 EUR  ·  Steuer DE19  ·  Konto 8000'),
        findsOneWidget);
    expect(find.text('1000.00 EUR'), findsOneWidget);
  });

  testWidgets(
      'InvoicesPage opens source quote and sales-order lists with prefilled search',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      invoiceList: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
        },
      ],
      invoiceDetail: const {
        'id': 'inv-1',
        'number': 'RE-2026-0001',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'status': 'draft',
        'invoice_date': '2026-03-21',
        'due_date': '2026-04-04',
        'gross_amount': 1428.0,
        'paid_amount': 428.0,
        'currency': 'EUR',
        'source_quote_id': 'q-1',
        'source_sales_order_id': 'so-1',
        'items': [
          {
            'description': 'Montage',
            'qty': 2,
            'unit_price': 500.0,
            'tax_code': 'DE19',
            'account_code': '8000',
          },
        ],
      },
      quoteList: const [
        {
          'id': 'q-1',
          'number': 'ANG-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
        },
        {
          'id': 'q-9',
          'number': 'ANG-2026-0009',
          'contact_name': 'Andere GmbH',
          'status': 'sent',
          'gross_amount': 500.0,
          'currency': 'EUR',
        },
      ],
      salesOrderList: const [
        {
          'id': 'so-1',
          'number': 'AUF-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'released',
          'gross_amount': 1428.0,
          'currency': 'EUR',
        },
        {
          'id': 'so-9',
          'number': 'AUF-2026-0009',
          'contact_name': 'Andere GmbH',
          'status': 'open',
          'gross_amount': 500.0,
          'currency': 'EUR',
        },
      ],
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'status': 'released',
        'gross_amount': 1428.0,
        'currency': 'EUR',
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: InvoicesPage(
          api: api,
          initialContext: const CommercialListContext.detail('inv-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Angebotsliste öffnen'), findsOneWidget);
    expect(find.text('Auftragsliste öffnen'), findsOneWidget);

    await tester.tap(find.text('Angebotsliste öffnen'));
    await tester.pumpAndSettle();

    expect(find.text('ANG-2026-0001'), findsOneWidget);
    expect(find.text('ANG-2026-0009'), findsNothing);
    expect(find.widgetWithText(TextField, 'q-1'), findsOneWidget);

    await tester.pageBack();
    await tester.pumpAndSettle();

    await tester.tap(find.text('Auftragsliste öffnen'));
    await tester.pumpAndSettle();

    expect(find.text('AUF-2026-0001'), findsOneWidget);
    expect(find.text('AUF-2026-0009'), findsNothing);
    expect(find.widgetWithText(TextField, 'so-1'), findsOneWidget);
  });

  testWidgets('InvoicesPage filters list to sales-order follow-up invoices',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      invoiceList: const [
        {
          'id': 'inv-sales-order',
          'number': 'RE-2026-0101',
          'contact_name': 'Muster GmbH',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
          'source_sales_order_id': 'so-1',
        },
        {
          'id': 'inv-free',
          'number': 'RE-2026-0102',
          'contact_name': 'Freier Kunde',
          'status': 'draft',
          'gross_amount': 500.0,
          'paid_amount': 0.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: InvoicesPage(api: api),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('RE-2026-0101'), findsOneWidget);
    expect(find.text('RE-2026-0102'), findsOneWidget);
    expect(find.textContaining('Folgebeleg aus Auftrag so-1'), findsOneWidget);

    await tester.tap(find.text('Auftragsbezug'));
    await tester.pumpAndSettle();

    expect(find.text('RE-2026-0101'), findsOneWidget);
    expect(find.text('RE-2026-0102'), findsNothing);
  });

  testWidgets(
      'SalesOrdersPage shows latest and earlier follow-up invoices separately',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'sales_orders.write',
        'invoices_out.write',
        'invoices_out.read'
      },
      salesOrderList: const [
        {
          'id': 'so-1',
          'number': 'AUF-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'invoiced',
          'gross_amount': 1428.0,
          'currency': 'EUR',
        },
      ],
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'project_name': 'Tor 1',
        'project_id': 'project-1',
        'status': 'invoiced',
        'order_date': '2026-03-21',
        'currency': 'EUR',
        'gross_amount': 1428.0,
        'net_amount': 1200.0,
        'tax_amount': 228.0,
        'linked_invoice_out_id': 'inv-2',
        'items': [
          {
            'id': 'item-1',
            'position': 1,
            'description': 'Montage',
            'qty': 4,
            'unit': 'Std',
            'unit_price': 300.0,
            'tax_code': 'DE19',
            'invoiced_qty': 2,
            'remaining_qty': 2,
          },
        ],
      },
      invoiceDetail: const {
        'id': 'inv-2',
        'number': 'RE-2026-0002',
        'status': 'draft',
        'gross_amount': 714.0,
        'paid_amount': 214.0,
        'currency': 'EUR',
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-2',
          'number': 'RE-2026-0002',
          'status': 'draft',
          'gross_amount': 714.0,
          'paid_amount': 214.0,
          'currency': 'EUR',
        },
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'status': 'booked',
          'gross_amount': 714.0,
          'paid_amount': 714.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: SalesOrdersPage(
          api: api,
          initialContext: const CommercialListContext.detail('so-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Folgerechnungen (2)'), findsOneWidget);
    expect(find.text('Teilfakturiert'), findsWidgets);
    expect(find.textContaining('Letzte Rechnung'), findsWidgets);
    expect(find.textContaining('Weitere Rechnung'), findsWidgets);
    expect(find.text('RE-2026-0002'), findsOneWidget);
    expect(find.text('RE-2026-0001'), findsOneWidget);
  });

  testWidgets(
      'SalesOrdersPage opens invoice list filtered to the selected sales order',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'sales_orders.write',
        'invoices_out.write',
        'invoices_out.read'
      },
      salesOrderList: const [
        {
          'id': 'so-1',
          'number': 'AUF-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'invoiced',
          'gross_amount': 1428.0,
          'currency': 'EUR',
          'related_invoice_count': 2,
          'remaining_gross_amount': 300.0,
        },
      ],
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'project_name': 'Tor 1',
        'project_id': 'project-1',
        'status': 'invoiced',
        'order_date': '2026-03-21',
        'currency': 'EUR',
        'gross_amount': 1428.0,
        'net_amount': 1200.0,
        'tax_amount': 228.0,
        'linked_invoice_out_id': 'inv-2',
        'items': [
          {
            'id': 'item-1',
            'position': 1,
            'description': 'Montage',
            'qty': 4,
            'unit': 'Std',
            'unit_price': 300.0,
            'tax_code': 'DE19',
            'invoiced_qty': 2,
            'remaining_qty': 2,
          },
        ],
      },
      invoiceList: const [
        {
          'id': 'inv-outside',
          'number': 'RE-2026-0099',
          'contact_name': 'Andere GmbH',
          'status': 'draft',
          'gross_amount': 500.0,
          'paid_amount': 0.0,
          'currency': 'EUR',
        },
      ],
      invoiceDetail: const {
        'id': 'inv-2',
        'number': 'RE-2026-0002',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'status': 'draft',
        'invoice_date': '2026-03-21',
        'due_date': '2026-04-04',
        'gross_amount': 714.0,
        'paid_amount': 214.0,
        'currency': 'EUR',
        'source_sales_order_id': 'so-1',
        'items': [
          {
            'description': 'Montage',
            'qty': 2,
            'unit_price': 300.0,
            'tax_code': 'DE19',
            'account_code': '8000',
          },
        ],
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-2',
          'number': 'RE-2026-0002',
          'contact_name': 'Muster GmbH',
          'status': 'draft',
          'gross_amount': 714.0,
          'paid_amount': 214.0,
          'currency': 'EUR',
          'source_sales_order_id': 'so-1',
        },
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'booked',
          'gross_amount': 714.0,
          'paid_amount': 714.0,
          'currency': 'EUR',
          'source_sales_order_id': 'so-1',
        },
      ],
      invoicePayments: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: SalesOrdersPage(
          api: api,
          initialContext: const CommercialListContext.detail('so-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Auftragsrechnungen öffnen'), findsOneWidget);

    await tester.tap(find.text('Auftragsrechnungen öffnen'));
    await tester.pumpAndSettle();

    expect(find.text('Ausgangsrechnungen'), findsOneWidget);
    expect(find.text('RE-2026-0002'), findsOneWidget);
    expect(find.text('RE-2026-0001'), findsOneWidget);
    expect(find.text('RE-2026-0099'), findsNothing);
    expect(find.widgetWithText(TextField, 'so-1'), findsOneWidget);

    await tester.tap(find.text('RE-2026-0002'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Auftrag AUF-2026-0001'));
    await tester.pumpAndSettle();

    expect(find.text('Aufträge'), findsOneWidget);
    expect(find.text('Folgerechnungen (2)'), findsOneWidget);
  });

  testWidgets('SalesOrdersPage filters list to partial invoices only',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      salesOrderList: const [
        {
          'id': 'so-partial',
          'number': 'AUF-2026-0101',
          'contact_name': 'Teil GmbH',
          'status': 'invoiced',
          'gross_amount': 1500.0,
          'currency': 'EUR',
          'related_invoice_count': 2,
          'remaining_gross_amount': 300.0,
        },
        {
          'id': 'so-done',
          'number': 'AUF-2026-0102',
          'contact_name': 'Fertig GmbH',
          'status': 'invoiced',
          'gross_amount': 900.0,
          'currency': 'EUR',
          'related_invoice_count': 2,
          'remaining_gross_amount': 0.0,
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: SalesOrdersPage(api: api),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('AUF-2026-0101'), findsOneWidget);
    expect(find.text('AUF-2026-0102'), findsOneWidget);

    await tester.tap(find.text('Teilfaktura'));
    await tester.pumpAndSettle();

    expect(find.text('AUF-2026-0101'), findsOneWidget);
    expect(find.text('AUF-2026-0102'), findsNothing);
  });

  testWidgets('QuotesPage stays stable on smaller viewport', (tester) async {
    await _prepareSmallViewport(tester);
    final api = _FakeApiClient(
      quoteList: const [
        {
          'id': 'q-1',
          'number': 'ANG-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'accepted',
          'gross_amount': 1190.0,
          'currency': 'EUR',
        },
      ],
      quoteDetail: const {
        'id': 'q-1',
        'number': 'ANG-2026-0001',
        'status': 'accepted',
        'contact_name': 'Muster GmbH',
        'project_name': 'Tor 1',
        'note': 'Test',
        'quote_date': '2026-03-21',
        'valid_until': '2026-04-21',
        'gross_amount': 1190.0,
        'net_amount': 1000.0,
        'tax_amount': 190.0,
        'currency': 'EUR',
        'linked_sales_order_id': 'so-1',
        'items': [
          {
            'description': 'Montage',
            'qty': 2,
            'unit': 'Std',
            'unit_price': 500.0,
            'tax_code': 'DE19',
          },
        ],
      },
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'status': 'released',
        'gross_amount': 1428.0,
        'currency': 'EUR',
        'items': [
          {'id': 'item-1'},
          {'id': 'item-2'},
        ],
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-2',
          'number': 'RE-2026-0002',
          'status': 'draft',
          'gross_amount': 714.0,
          'paid_amount': 214.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: QuotesPage(
          api: api,
          initialContext: const CommercialListContext.detail('q-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('ANG-2026-0001'), findsWidgets);
    expect(find.text('Rechnungen aus Auftrag (1)'), findsOneWidget);
  });

  testWidgets('InvoicesPage stays stable on smaller viewport', (tester) async {
    await _prepareSmallViewport(tester);
    final api = _FakeApiClient(
      invoiceList: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
        },
      ],
      invoiceDetail: const {
        'id': 'inv-1',
        'number': 'RE-2026-0001',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'status': 'draft',
        'invoice_date': '2026-03-21',
        'due_date': '2026-04-04',
        'gross_amount': 1428.0,
        'paid_amount': 428.0,
        'currency': 'EUR',
        'source_sales_order_id': 'so-1',
        'items': [
          {
            'description': 'Montage',
            'qty': 2,
            'unit_price': 500.0,
            'tax_code': 'DE19',
            'account_code': '8000',
          },
        ],
      },
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'status': 'invoiced',
        'gross_amount': 1428.0,
        'currency': 'EUR',
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'status': 'draft',
          'gross_amount': 1428.0,
          'paid_amount': 428.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: InvoicesPage(
          api: api,
          initialContext: const CommercialListContext.detail('inv-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('RE-2026-0001'), findsWidgets);
    expect(find.text('Auftrag AUF-2026-0001'), findsOneWidget);
  });

  testWidgets('SalesOrdersPage stays stable on smaller viewport',
      (tester) async {
    await _prepareSmallViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'sales_orders.write',
        'invoices_out.write',
        'invoices_out.read'
      },
      salesOrderList: const [
        {
          'id': 'so-1',
          'number': 'AUF-2026-0001',
          'contact_name': 'Muster GmbH',
          'status': 'invoiced',
          'gross_amount': 1428.0,
          'currency': 'EUR',
          'related_invoice_count': 2,
          'remaining_gross_amount': 300.0,
        },
      ],
      salesOrderDetail: const {
        'id': 'so-1',
        'number': 'AUF-2026-0001',
        'contact_name': 'Muster GmbH',
        'contact_id': 'contact-1',
        'project_name': 'Tor 1',
        'project_id': 'project-1',
        'status': 'invoiced',
        'order_date': '2026-03-21',
        'currency': 'EUR',
        'gross_amount': 1428.0,
        'net_amount': 1200.0,
        'tax_amount': 228.0,
        'linked_invoice_out_id': 'inv-2',
        'items': [
          {
            'id': 'item-1',
            'position': 1,
            'description': 'Montage',
            'qty': 4,
            'unit': 'Std',
            'unit_price': 300.0,
            'tax_code': 'DE19',
            'invoiced_qty': 2,
            'remaining_qty': 2,
          },
        ],
      },
      invoiceDetail: const {
        'id': 'inv-2',
        'number': 'RE-2026-0002',
        'status': 'draft',
        'gross_amount': 714.0,
        'paid_amount': 214.0,
        'currency': 'EUR',
      },
      salesOrderInvoices: const [
        {
          'id': 'inv-2',
          'number': 'RE-2026-0002',
          'status': 'draft',
          'gross_amount': 714.0,
          'paid_amount': 214.0,
          'currency': 'EUR',
        },
        {
          'id': 'inv-1',
          'number': 'RE-2026-0001',
          'status': 'booked',
          'gross_amount': 714.0,
          'paid_amount': 714.0,
          'currency': 'EUR',
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: SalesOrdersPage(
          api: api,
          initialContext: const CommercialListContext.detail('so-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('AUF-2026-0001'), findsWidgets);
    expect(find.text('Folgerechnungen (2)'), findsOneWidget);
  });

  testWidgets('Dashboard shows partial invoicing insight for sales orders',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      salesOrderList: const [
        {
          'id': 'so-partial',
          'number': 'AUF-2026-0101',
          'contact_name': 'Teil GmbH',
          'status': 'invoiced',
          'gross_amount': 1500.0,
          'currency': 'EUR',
          'related_invoice_count': 2,
          'remaining_gross_amount': 300.0,
        },
        {
          'id': 'so-done',
          'number': 'AUF-2026-0102',
          'contact_name': 'Fertig GmbH',
          'status': 'invoiced',
          'gross_amount': 900.0,
          'currency': 'EUR',
          'related_invoice_count': 1,
          'remaining_gross_amount': 0.0,
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: DashboardPage(
          api: api,
          onLogout: () async {},
          permissions: const {'sales_orders.read'},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Aufträge'), findsOneWidget);
    expect(find.textContaining('1 Teilfaktura offen'), findsOneWidget);
    expect(find.textContaining('Rest 300.00 EUR'), findsOneWidget);
  });

  testWidgets('Dashboard opens commercial list pages via shared destinations',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      quoteList: const [],
      invoiceList: const [],
      salesOrderList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: DashboardPage(
          api: api,
          onLogout: () async {},
          permissions: const {
            'quotes.read',
            'invoices_out.read',
            'sales_orders.read',
          },
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Angebote'));
    await tester.pumpAndSettle();
    expect(find.text('Noch keine Angebote gefunden.'), findsOneWidget);

    await tester.pageBack();
    await tester.pumpAndSettle();

    await tester.tap(find.text('Rechnungen'));
    await tester.pumpAndSettle();
    expect(find.text('Ausgangsrechnungen'), findsOneWidget);

    await tester.pageBack();
    await tester.pumpAndSettle();

    await tester.tap(find.text('Aufträge'));
    await tester.pumpAndSettle();
    expect(find.text('Noch keine Aufträge gefunden.'), findsOneWidget);
  });

  testWidgets('Dashboard opens materialwirtschaft via shared destination',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      warehouseList: const [],
      materialList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: DashboardPage(
          api: api,
          onLogout: () async {},
          permissions: const {'materials.read'},
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Materialwirtschaft'));
    await tester.pumpAndSettle();

    expect(find.text('Bereich:'), findsOneWidget);
    expect(find.text('Materialien'), findsWidgets);
  });

  testWidgets(
      'ProjectDetailPage shows commercial follow-up and partial invoicing stats',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'sales_orders.read', 'quotes.read'},
      projectPhases: const [],
      quoteList: const [
        {
          'id': 'q-follow-up',
          'number': 'ANG-2026-0101',
          'linked_sales_order_id': 'so-1',
          'gross_amount': 1190.0,
          'currency': 'EUR',
        },
        {
          'id': 'q-open',
          'number': 'ANG-2026-0102',
          'gross_amount': 750.0,
          'currency': 'EUR',
        },
      ],
      salesOrderList: const [
        {
          'id': 'so-partial',
          'number': 'AUF-2026-0101',
          'related_invoice_count': 2,
          'remaining_gross_amount': 300.0,
        },
        {
          'id': 'so-done',
          'number': 'AUF-2026-0102',
          'related_invoice_count': 1,
          'remaining_gross_amount': 0.0,
        },
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: false,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Kaufmännischer Status'), findsOneWidget);
    expect(find.text('2 Angebote  •  2 Aufträge'), findsOneWidget);
    expect(find.textContaining('1 Angebote mit Folgebeleg'), findsOneWidget);
    expect(find.textContaining('1 Aufträge in Teilfaktura'), findsOneWidget);
    expect(
        find.textContaining('Offener Restbetrag: 300.00 EUR'), findsOneWidget);
    expect(find.text('Projekt-Angebote öffnen'), findsOneWidget);
    expect(find.text('Projekt-Aufträge öffnen'), findsOneWidget);

    await tester.tap(find.text('Projekt-Angebote öffnen'));
    await tester.pumpAndSettle();

    expect(find.text('Angebote'), findsOneWidget);
    expect(find.text('ANG-2026-0101'), findsOneWidget);

    await tester.pageBack();
    await tester.pumpAndSettle();

    await tester.tap(find.text('Projekt-Aufträge öffnen'));
    await tester.pumpAndSettle();

    expect(find.text('Aufträge'), findsOneWidget);
  });

  testWidgets(
      'ProjectDetailPage opens purchase-order flow with project note prefill',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'projects.write', 'purchase_orders.write'},
      projectPhases: const [],
      quoteList: const [],
      salesOrderList: const [],
      purchaseOrderList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Bestellung'));
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Bestellung anlegen'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(
            TextFormField, 'Projektbezug: PRJ-0101 • Tor 1'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(
            TextFormField, 'Projektbedarf: PRJ-0101 • Tor 1'),
      ),
      findsOneWidget,
    );
    expect(find.widgetWithText(TextField, 'PRJ-0101'), findsOneWidget);
  });

  testWidgets(
      'ProjectDetailPage hides purchase-order action without purchase-order permission',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'projects.write'},
      projectPhases: const [],
      quoteList: const [],
      salesOrderList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Bestellung'), findsNothing);
    expect(find.text('Materialbewegung'), findsNothing);
  });

  testWidgets(
      'ProjectDetailPage still shows stock movement without purchase-order permission',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'projects.write', 'stock_movements.write'},
      projectPhases: const [],
      quoteList: const [],
      salesOrderList: const [],
      materialList: const [],
      warehouseList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Bestellung'), findsNothing);
    expect(find.text('Materialbewegung'), findsOneWidget);
  });

  testWidgets(
      'ProjectDetailPage opens stock-movement flow with project reference prefill',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'stock_movements.write'},
      projectPhases: const [],
      quoteList: const [],
      salesOrderList: const [],
      materialList: const [],
      warehouseList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Materialbewegung'));
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Bestandsbewegung'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'Projektbedarf'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'PRJ-0101'), findsOneWidget);
  });

  testWidgets(
      'ProjectDetailPage opens purchase-order flow for linked project material with material prefill',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'projects.write', 'purchase_orders.write'},
      projectPhases: const [
        {'id': 'phase-1', 'name': 'Los 1', 'nummer': '1'},
      ],
      phaseElevations: const {
        'phase-1': [
          {'id': 'elev-1', 'nummer': '1', 'name': 'Position 1', 'menge': 1},
        ],
      },
      elevationVariants: const {
        'elev-1': [
          {'id': 'variant-1', 'name': 'Variante A', 'menge': 1},
        ],
      },
      variantMaterials: const {
        'variant-1': {
          'profiles': [],
          'articles': [
            {
              'id': 'article-1',
              'article_code': 'Beschlagset',
              'description': 'Beschlagset links',
              'qty': 2,
              'unit': 'Stk',
              'material_id': 'mat-1',
              'material_nummer': 'MAT-001',
            },
          ],
          'glass': [],
        },
      },
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-001',
          'bezeichnung': 'Beschlagset links'
        },
      ],
      purchaseOrderList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Los 1'));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('1: Position 1'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Variante A'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Bestellen'));
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Bestellung anlegen'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(
            TextFormField, 'Projektmaterial aus Variante Variante A'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.text('MAT-001 – Beschlagset links'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, '2.0'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextFormField, 'Stk'),
      ),
      findsOneWidget,
    );
  });

  testWidgets(
      'ProjectDetailPage opens material detail and stock movement for linked project material via shared actions',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'projects.write',
        'materials.read',
        'stock_movements.write'
      },
      projectPhases: const [
        {'id': 'phase-1', 'name': 'Los 1', 'nummer': '1'},
      ],
      phaseElevations: const {
        'phase-1': [
          {'id': 'elev-1', 'nummer': '1', 'name': 'Position 1', 'menge': 1},
        ],
      },
      elevationVariants: const {
        'elev-1': [
          {'id': 'variant-1', 'name': 'Variante A', 'menge': 1},
        ],
      },
      variantMaterials: const {
        'variant-1': {
          'profiles': [],
          'articles': [
            {
              'id': 'article-1',
              'article_code': 'Beschlagset',
              'description': 'Beschlagset links',
              'qty': 2,
              'unit': 'Stk',
              'material_id': 'mat-1',
              'material_nummer': 'MAT-001',
            },
          ],
          'glass': [],
        },
      },
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-001',
          'bezeichnung': 'Beschlagset links'
        },
      ],
      materialDetail: const {
        'id': 'mat-1',
        'nummer': 'MAT-001',
        'bezeichnung': 'Beschlagset links',
        'typ': 'artikel',
        'kategorie': 'Beschlag',
      },
      materialStock: const [],
      materialDocuments: const [],
      warehouseList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Los 1'));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('1: Position 1'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Variante A'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Material'));
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Beschlagset links'), findsWidgets);
    expect(find.text('Bitte ein Material auswählen'), findsNothing);

    await tester.pageBack();
    await tester.pumpAndSettle();

    await tester.tap(find.text('Bewegung'));
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Bestandsbewegung'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'Projektbedarf'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'Variante A'), findsOneWidget);
  });

  testWidgets(
      'ProjectDetailPage opens linked material detail directly from the project material row',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {'projects.write', 'materials.read'},
      projectPhases: const [
        {'id': 'phase-1', 'name': 'Los 1', 'nummer': '1'},
      ],
      phaseElevations: const {
        'phase-1': [
          {'id': 'elev-1', 'nummer': '1', 'name': 'Position 1', 'menge': 1},
        ],
      },
      elevationVariants: const {
        'elev-1': [
          {'id': 'variant-1', 'name': 'Variante A', 'menge': 1},
        ],
      },
      variantMaterials: const {
        'variant-1': {
          'profiles': [],
          'articles': [
            {
              'id': 'article-1',
              'article_code': 'Beschlagset',
              'description': 'Beschlagset links',
              'qty': 2,
              'unit': 'Stk',
              'material_id': 'mat-1',
              'material_nummer': 'MAT-001',
            },
          ],
          'glass': [],
        },
      },
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-001',
          'bezeichnung': 'Beschlagset links'
        },
      ],
      materialDetail: const {
        'id': 'mat-1',
        'nummer': 'MAT-001',
        'bezeichnung': 'Beschlagset links',
        'typ': 'artikel',
        'kategorie': 'Beschlag',
      },
      materialStock: const [],
      materialDocuments: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Los 1'));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('1: Position 1'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Variante A'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Beschlagset'));
    await tester.pumpAndSettle();

    expect(find.text('Materialwirtschaft'), findsOneWidget);
    expect(find.text('Beschlagset links'), findsWidgets);
    expect(find.text('Bitte ein Material auswählen'), findsNothing);
  });

  testWidgets(
      'ProjectDetailPage shows linked-material follow-up actions without project write permission',
      (tester) async {
    await _prepareLargeViewport(tester);
    final api = _FakeApiClient(
      permissions: const {
        'materials.read',
        'stock_movements.write',
        'purchase_orders.write',
      },
      projectPhases: const [
        {'id': 'phase-1', 'name': 'Los 1', 'nummer': '1'},
      ],
      phaseElevations: const {
        'phase-1': [
          {'id': 'elev-1', 'nummer': '1', 'name': 'Position 1', 'menge': 1},
        ],
      },
      elevationVariants: const {
        'elev-1': [
          {'id': 'variant-1', 'name': 'Variante A', 'menge': 1},
        ],
      },
      variantMaterials: const {
        'variant-1': {
          'profiles': [],
          'articles': [
            {
              'id': 'article-1',
              'article_code': 'Beschlagset',
              'description': 'Beschlagset links',
              'qty': 2,
              'unit': 'Stk',
              'material_id': 'mat-1',
              'material_nummer': 'MAT-001',
            },
          ],
          'glass': [],
        },
      },
      materialList: const [
        {
          'id': 'mat-1',
          'nummer': 'MAT-001',
          'bezeichnung': 'Beschlagset links'
        },
      ],
      purchaseOrderList: const [],
      materialDetail: const {
        'id': 'mat-1',
        'nummer': 'MAT-001',
        'bezeichnung': 'Beschlagset links',
      },
      materialStock: const [],
      materialDocuments: const [],
      warehouseList: const [],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ProjectDetailPage(
          api: api,
          project: const {
            'id': 'project-1',
            'name': 'Tor 1',
            'nummer': 'PRJ-0101',
          },
          canWrite: false,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Los 1'));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('1: Position 1'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Variante A'));
    await tester.pumpAndSettle();

    expect(find.text('Material'), findsOneWidget);
    expect(find.text('Bewegung'), findsOneWidget);
    expect(find.text('Bestellen'), findsOneWidget);
    expect(find.text('Ändern'), findsNothing);
    expect(find.text('Lösen'), findsNothing);
  });
}
