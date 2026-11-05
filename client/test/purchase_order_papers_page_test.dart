import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/purchase_order_detail_page.dart';
import 'package:nalaerp_client/pages/purchase_order_papers_page.dart';

// Lieferpapiere an der Bestellung im Client, lesend (Backlog H.4.5b1).

class _FakeApi extends ApiClient {
  _FakeApi({required this.papers, this.listError})
      : super(baseUrl: 'http://localhost:8080');

  final List<Map<String, dynamic>> papers;
  final Object? listError;
  final List<bool> listCalls = [];
  final List<String> downloads = [];

  @override
  bool hasPermission(String permission) => permission == 'purchase_orders.read';

  @override
  Future<List<dynamic>> listPurchaseOrderPapers(String orderId,
      {bool includeStorniert = false}) async {
    listCalls.add(includeStorniert);
    if (listError != null) throw listError!;
    return [
      for (final p in papers)
        if (includeStorniert || p['storniert_am'] == null) p
    ];
  }

  @override
  Future<void> downloadPurchaseOrderPaper(String orderId, String paperId,
      {String? filename}) async {
    downloads.add('$orderId:$paperId:$filename');
  }

  // Fuer den Einstieg ueber die Detailseite.
  @override
  Future<Map<String, dynamic>> getPurchaseOrder(String id) async => {
        'bestellung': {'id': id, 'nummer': 'B-7', 'status': 'ordered'},
        'positionen': _items,
      };

  @override
  Future<List<String>> listPOStatuses() async => const ['draft'];

  @override
  Future<List<dynamic>> listMaterials(
          {String? q,
          String? typ,
          String? kategorie,
          int? limit,
          int? offset}) async =>
      const [];

  @override
  Future<List<dynamic>> listWarehouses() async => const [];
}

const _items = [
  {'id': 'i1', 'position': 1, 'bezeichnung': 'HEB 200'},
  {'id': 'i2', 'position': 2, 'bezeichnung': ''},
];

const _dop = {
  'id': 'p1',
  'art': 'leistungserklaerung',
  'position_id': null,
  'charge': '',
  'notiz': 'DoP 2031',
  'filename': 'dop.pdf',
  'length': 2048,
};
const _cert = {
  'id': 'p2',
  'art': 'zeugnis_3_1',
  'position_id': 'i1',
  'charge': 'C-4711',
  'notiz': '',
  'filename': 'zeugnis.pdf',
  'length': 100,
};
const _old = {
  'id': 'p3',
  'art': 'lieferschein',
  'position_id': 'i2',
  'charge': '',
  'notiz': '',
  'filename': 'ls.pdf',
  'length': 100,
  'storno_grund': 'falsche Bestellung',
  'storniert_am': '2031-01-05T10:00:00Z',
};

Future<void> _open(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1200, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: PurchaseOrderPapersPage(
          key: UniqueKey(),
          api: api,
          orderId: 'po1',
          orderNumber: 'B-7',
          items: _items)));
  await tester.pumpAndSettle();
}

void main() {
  test('Art und Bezug werden beschriftet', () {
    expect(purchaseOrderPaperArts.map(purchaseOrderPaperArtLabel).toList(), [
      'Leistungserklärung',
      'Werkszeugnis 2.2',
      'Abnahmeprüfzeugnis 3.1',
      'Abnahmeprüfzeugnis 3.2',
      'Lieferschein',
      'Sonstiges',
    ]);
    expect(purchaseOrderPaperArtLabel('neu'), 'neu');
    expect(purchaseOrderPaperScope(_dop, _items), 'ganze Bestellung');
    expect(purchaseOrderPaperScope(_cert, _items), 'Position 1 – HEB 200');
    expect(purchaseOrderPaperScope(_old, _items), 'Position 2');
    expect(purchaseOrderPaperScope({'position_id': 'weg'}, _items), 'Position');
  });

  test('API: Pfade, Stornierte auf Wunsch, Fehler 404', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <String>[];
    final client = MockClient((req) async {
      calls.add('${req.method} ${req.url.path}?${req.url.query}');
      if (req.url.path.contains('/keine/')) {
        return http.Response(
            '{"error":{"code":"not_found","message":"Bestellung nicht gefunden"}}',
            404,
            headers: {'content-type': 'application/json'});
      }
      if (req.url.path.endsWith('/papers/')) {
        return http.Response('[{"id":"p1"}]', 200,
            headers: {'content-type': 'application/json'});
      }
      return http.Response.bytes([1, 2, 3], 200,
          headers: {'content-type': 'application/pdf'});
    });
    await http.runWithClient(() async {
      expect(await api.listPurchaseOrderPapers('po1'), [
        {'id': 'p1'}
      ]);
      await api.listPurchaseOrderPapers('po1', includeStorniert: true);
      await api.downloadPurchaseOrderPaper('po1', 'p1', filename: 'x.pdf');
      await expectLater(
          api.listPurchaseOrderPapers('keine'), throwsA(isA<ApiException>()));
    }, () => client);
    expect(calls, [
      'GET /api/v1/purchase-orders/po1/papers/?',
      'GET /api/v1/purchase-orders/po1/papers/?include_storniert=true',
      'GET /api/v1/purchase-orders/po1/papers/p1?',
      'GET /api/v1/purchase-orders/keine/papers/?',
    ]);
  });

  testWidgets('Liste zeigt Art, Bezug, Charge, Notiz und lädt herunter',
      (tester) async {
    final api = _FakeApi(papers: [_dop, _cert, _old]);
    await _open(tester, api);

    expect(find.text('Lieferpapiere – Bestellung B-7'), findsOneWidget);
    expect(find.text('Leistungserklärung'), findsOneWidget);
    expect(find.text('ganze Bestellung  •  dop.pdf · 2 KB'), findsOneWidget);
    expect(find.text('DoP 2031'), findsOneWidget);
    expect(
        find.text(
            'Position 1 – HEB 200  •  Charge: C-4711  •  zeugnis.pdf · 100 B'),
        findsOneWidget);
    expect(find.text('Lieferschein'), findsNothing);

    await tester.tap(find.byTooltip('Herunterladen').at(1));
    await tester.pumpAndSettle();
    expect(api.downloads, ['po1:p2:zeugnis.pdf']);
  });

  testWidgets('Stornierte nur auf Wunsch, mit Grund', (tester) async {
    final api = _FakeApi(papers: [_dop, _old]);
    await _open(tester, api);
    expect(find.textContaining('Storniert:'), findsNothing);

    await tester.tap(find.text('Stornierte zeigen'));
    await tester.pumpAndSettle();
    expect(api.listCalls, [false, true]);
    expect(find.text('Lieferschein'), findsOneWidget);
    expect(find.text('Storniert: falsche Bestellung'), findsOneWidget);
  });

  testWidgets('Leer und Fehler des Servers', (tester) async {
    await _open(tester, _FakeApi(papers: const []));
    expect(find.text('Keine Lieferpapiere'), findsOneWidget);

    await _open(
        tester,
        _FakeApi(
            papers: const [],
            listError: const ApiException(
                statusCode: 404,
                message: 'Bestellung nicht gefunden',
                code: 'not_found')));
    expect(find.text('Bestellung nicht gefunden'), findsOneWidget);
    expect(find.text('Keine Lieferpapiere'), findsNothing);
  });

  testWidgets('Einstieg über die Bestellung reicht die Positionen durch',
      (tester) async {
    final api = _FakeApi(papers: [_cert]);
    await tester.binding.setSurfaceSize(const Size(1400, 1200));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(
        MaterialApp(home: PurchaseOrderDetailPage(api: api, id: 'po1')));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Lieferpapiere'));
    await tester.pumpAndSettle();
    expect(find.text('Lieferpapiere – Bestellung B-7'), findsOneWidget);
    expect(find.textContaining('Position 1 – HEB 200'), findsOneWidget);
  });
}
