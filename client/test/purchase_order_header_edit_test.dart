import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/purchase_order_detail_page.dart';

// "Bestellung bearbeiten" sendet nur Geaendertes (Backlog H.4.y).

class _FakeApi extends ApiClient {
  _FakeApi({this.saveError}) : super(baseUrl: 'http://localhost:8080');

  final Object? saveError;
  Map<String, dynamic> order = {
    'id': 'po1',
    'nummer': 'B-3',
    'status': 'ordered',
    'waehrung': 'EUR',
    'notiz': 'Termin fix',
  };
  final List<Map<String, dynamic>> patches = [];

  @override
  bool hasPermission(String permission) => true;

  @override
  Future<Map<String, dynamic>> getPurchaseOrder(String id) async =>
      {'bestellung': order, 'positionen': const []};

  @override
  Future<Map<String, dynamic>> updatePurchaseOrder(
      String id, Map<String, dynamic> patch) async {
    patches.add(patch);
    if (saveError != null) throw saveError!;
    order = {...order, ...patch};
    return order;
  }

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
}

Future<void> _openEdit(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1200));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: PurchaseOrderDetailPage(key: UniqueKey(), api: api, id: 'po1')));
  await tester.pumpAndSettle();
  await tester.tap(find.byIcon(Icons.edit_note));
  await tester.pumpAndSettle();
}

Future<void> _setStatus(WidgetTester tester, String from, String to) async {
  await tester.tap(find.text(from).last);
  await tester.pumpAndSettle();
  await tester.tap(find.text(to).last);
  await tester.pumpAndSettle();
}

void main() {
  test('Patch enthält nur geänderte Felder', () {
    Map<String, dynamic> patch(
            {String? status = 'ordered',
            String currency = 'EUR',
            String note = 'Termin fix'}) =>
        buildPurchaseOrderHeaderPatch(
            currentStatus: 'ordered',
            currentCurrency: 'EUR',
            currentNote: 'Termin fix',
            status: status,
            currency: currency,
            note: note);

    expect(patch(), isEmpty);
    expect(patch(status: null), isEmpty);
    expect(patch(currency: ' eur ', note: ' Termin fix '), isEmpty);
    expect(patch(currency: ''), isEmpty); // leer heisst EUR
    expect(patch(status: 'received'), {'status': 'received'});
    expect(patch(currency: 'chf'), {'waehrung': 'CHF'});
    expect(patch(note: ''), {'notiz': ''});
    expect(patch(status: 'canceled', currency: 'USD', note: 'neu'),
        {'status': 'canceled', 'waehrung': 'USD', 'notiz': 'neu'});
  });

  testWidgets('Nur Status ändern sendet nur den Status', (tester) async {
    final api = _FakeApi();
    await _openEdit(tester, api);
    await _setStatus(tester, 'ordered', 'received');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.patches, [
      {'status': 'received'}
    ]);
    expect(find.text('Bestellung bearbeiten'), findsNothing);
    expect(find.widgetWithText(Chip, 'received'), findsOneWidget);
  });

  testWidgets('Nichts geändert: kein Aufruf, Dialog zu', (tester) async {
    final api = _FakeApi();
    await _openEdit(tester, api);
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.patches, isEmpty);
    expect(find.text('Bestellung bearbeiten'), findsNothing);
  });

  testWidgets('Notiz geändert: nur die Notiz, Fehler des Servers sichtbar',
      (tester) async {
    final api = _FakeApi(
        saveError: const ApiException(
            statusCode: 400,
            message:
                'Bestellung ist nicht im Status draft und kann nicht mehr inhaltlich geändert werden (aktuell: ordered)',
            code: 'validation_error'));
    await _openEdit(tester, api);
    await tester.enterText(
        find.widgetWithText(TextFormField, 'Notiz'), 'Termin verschoben');
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.patches, [
      {'notiz': 'Termin verschoben'}
    ]);
    expect(find.textContaining('kann nicht mehr inhaltlich geändert werden'),
        findsOneWidget);
    expect(find.text('Bestellung bearbeiten'), findsOneWidget);
  });
}
