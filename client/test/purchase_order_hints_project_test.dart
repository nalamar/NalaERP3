import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/purchase_order_detail_page.dart';
import 'package:nalaerp_client/pages/purchase_orders_page.dart';
import 'package:nalaerp_client/purchase_order_hints.dart';

// Hinweise und Projekt an der Bestellung im Client (Backlog H.4.5a).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, required this.order, this.saveError})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  Map<String, dynamic> order;
  final Object? saveError;
  final List<Map<String, dynamic>> patches = [];
  final List<String> projectReads = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<Map<String, dynamic>> getPurchaseOrder(String id) async =>
      {'bestellung': order, 'positionen': const []};

  @override
  Future<List<dynamic>> listPurchaseOrders(
          {String? q, String? status, int? limit, int? offset}) async =>
      [order];

  @override
  Future<Map<String, dynamic>> updatePurchaseOrder(
      String id, Map<String, dynamic> patch) async {
    patches.add(patch);
    if (saveError != null) throw saveError!;
    final pid = patch['projekt_id'].toString();
    order = {...order, 'projekt_id': pid.isEmpty ? null : pid};
    return order;
  }

  @override
  Future<Map<String, dynamic>> getProject(String id) async {
    projectReads.add(id);
    return {'id': id, 'nummer': 'P-$id', 'name': 'Halle $id'};
  }

  @override
  Future<List<dynamic>> listProjects(
          {String? q, String? status, int? limit, int? offset}) async =>
      const [
        {'id': '1', 'nummer': 'P-1', 'name': 'Halle 1'},
        {'id': '2', 'nummer': 'P-2', 'name': 'Halle 2'},
      ];

  @override
  Future<Map<String, dynamic>> getContact(String id) async =>
      {'id': id, 'name': 'Stahlbau AG'};

  @override
  Future<List<String>> listPOStatuses() async =>
      const ['draft', 'ordered', 'received', 'canceled'];

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

  @override
  Future<List<dynamic>> listContacts(
          {String? q,
          String? rolle,
          String? status,
          String? typ,
          int? limit,
          int? offset}) async =>
      const [];
}

const _read = {'purchase_orders.read'};
const _write = {'purchase_orders.read', 'purchase_orders.write'};

Map<String, dynamic> _order({List<String> hints = const [], String? pid}) => {
      'id': 'po1',
      'nummer': 'B-1',
      'lieferant_id': 's1',
      'status': 'ordered',
      'datum': '2031-01-05',
      'waehrung': 'EUR',
      'projekt_id': pid,
      'hinweise': hints,
    };

Future<void> _openDetail(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: PurchaseOrderDetailPage(key: UniqueKey(), api: api, id: 'po1')));
  await tester.pumpAndSettle();
}

void main() {
  test('Hinweis-Codes werden übersetzt, Unbekanntes bleibt sichtbar', () {
    expect(
        purchaseOrderHints({
          'hinweise': [
            'zertifikat_fehlt',
            'zertifikat_ungueltig',
            'leistungserklaerung_fehlt',
            'neu_x',
            ' '
          ]
        }),
        [
          'EN-1090-Zertifikat des Lieferanten fehlt',
          'EN-1090-Zertifikat deckt den Zeitraum nicht ab',
          'Leistungserklärung fehlt',
          'neu_x',
        ]);
    expect(purchaseOrderHints({'hinweise': null}), isEmpty);
    expect(purchaseOrderHints({}), isEmpty);
  });

  testWidgets('Liste zeigt die Hinweise der Bestellung', (tester) async {
    final api = _FakeApi(
        permissions: _read, order: _order(hints: ['zertifikat_fehlt']));
    await tester.pumpWidget(MaterialApp(home: PurchaseOrdersPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('EN-1090-Zertifikat des Lieferanten fehlt'),
        findsOneWidget);

    api.order = _order();
    await tester.pumpWidget(
        MaterialApp(home: PurchaseOrdersPage(key: UniqueKey(), api: api)));
    await tester.pumpAndSettle();
    expect(find.textContaining('Zertifikat'), findsNothing);
  });

  testWidgets('Detail zeigt Hinweise und Projekt, ohne Schreibrecht kein Knopf',
      (tester) async {
    final api = _FakeApi(
        permissions: _read,
        order: _order(hints: ['leistungserklaerung_fehlt'], pid: '7'));
    await _openDetail(tester, api);

    expect(find.text('Leistungserklärung fehlt'), findsOneWidget);
    expect(find.text('Projekt: P-7 – Halle 7'), findsOneWidget);
    expect(api.projectReads, ['7']);
    expect(find.byTooltip('Projekt zuordnen'), findsNothing);
  });

  testWidgets('Ohne Projekt: keine Abfrage, Anzeige "keins"', (tester) async {
    final api = _FakeApi(permissions: _write, order: _order());
    await _openDetail(tester, api);

    expect(find.text('Projekt: keins'), findsOneWidget);
    expect(api.projectReads, isEmpty);
    expect(find.byIcon(Icons.warning_amber), findsNothing);
  });

  testWidgets('Projekt zuordnen sendet nur projekt_id und lädt neu',
      (tester) async {
    final api = _FakeApi(permissions: _write, order: _order());
    await _openDetail(tester, api);

    await tester.tap(find.byTooltip('Projekt zuordnen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Kein Projekt'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('P-2 – Halle 2').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.patches, [
      {'projekt_id': '2'}
    ]);
    expect(find.text('Projekt: P-2 – Halle 2'), findsOneWidget);
  });

  testWidgets('Projekt entfernen sendet leere projekt_id', (tester) async {
    final api = _FakeApi(permissions: _write, order: _order(pid: '1'));
    await _openDetail(tester, api);

    await tester.tap(find.byTooltip('Projekt zuordnen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('P-1 – Halle 1').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Kein Projekt').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.patches, [
      {'projekt_id': ''}
    ]);
    expect(find.text('Projekt: keins'), findsOneWidget);
  });

  testWidgets('Fehler beim Speichern bleibt im Dialog', (tester) async {
    final api = _FakeApi(
        permissions: _write,
        order: _order(),
        saveError: const ApiException(
            statusCode: 400,
            message: 'Projekt nicht gefunden',
            code: 'validation_error'));
    await _openDetail(tester, api);

    await tester.tap(find.byTooltip('Projekt zuordnen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(find.text('Projekt nicht gefunden'), findsOneWidget);
    expect(find.text('Projekt zuordnen'), findsOneWidget);
    expect(find.text('Projekt: keins'), findsOneWidget);
  });
}
