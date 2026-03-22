import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/invoices_page.dart';
import 'package:nalaerp_client/pages/projects_page.dart';
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
    this.invoicePayments = const [],
    this.permissions = const <String>{},
  }) : super(baseUrl: 'http://localhost:8080');

  final List<dynamic> quoteList;
  final Map<String, dynamic>? quoteDetail;
  final List<dynamic> invoiceList;
  final Map<String, dynamic>? invoiceDetail;
  final List<dynamic> salesOrderList;
  final Map<String, dynamic>? salesOrderDetail;
  final List<dynamic> salesOrderInvoices;
  final List<dynamic> projectPhases;
  final List<dynamic> invoicePayments;
  final Set<String> permissions;

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
      quoteList;

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
      sourceSalesOrderId != null && sourceSalesOrderId.isNotEmpty
          ? salesOrderInvoices
          : invoiceList;

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
      salesOrderList;

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
          initialQuoteId: 'q-1',
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
          initialInvoiceId: 'inv-1',
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
          initialSalesOrderId: 'so-1',
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
          initialQuoteId: 'q-1',
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
          initialInvoiceId: 'inv-1',
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
          initialSalesOrderId: 'so-1',
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
}
