import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/purchase_order_papers_page.dart';
import 'package:nalaerp_client/web/browser.dart' as browser;

// Lieferpapiere hochladen und stornieren im Client (Backlog H.4.5b2).

class _FakeApi extends ApiClient {
  _FakeApi(
      {required this.permissions,
      this.uploadError,
      this.stornoError,
      List<Map<String, dynamic>> extra = const []})
      : super(baseUrl: 'http://localhost:8080') {
    papers.addAll(extra);
  }

  final Set<String> permissions;
  final Object? uploadError;
  final Object? stornoError;
  final List<Map<String, dynamic>> papers = [
    {
      'id': 'p1',
      'art': 'lieferschein',
      'position_id': null,
      'filename': 'ls.pdf',
      'length': 100,
    }
  ];
  int listCalls = 0;
  final List<String> uploads = [];
  final List<String> stornos = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listPurchaseOrderPapers(String orderId,
      {bool includeStorniert = false}) async {
    listCalls++;
    return List.of(papers);
  }

  @override
  Future<Map<String, dynamic>> uploadPurchaseOrderPaper(
      String orderId, String filename, Uint8List bytes,
      {required String art,
      String? positionId,
      String charge = '',
      String notiz = '',
      String? contentType}) async {
    uploads.add(
        '$orderId:$filename:${bytes.length}:$art:$positionId:$charge:$notiz:$contentType');
    if (uploadError != null) throw uploadError!;
    final p = {
      'id': 'p2',
      'art': art,
      'position_id': positionId,
      'charge': charge,
      'filename': filename,
      'length': bytes.length,
    };
    papers.add(p);
    return p;
  }

  @override
  Future<Map<String, dynamic>> stornoPurchaseOrderPaper(
      String orderId, String paperId, String reason) async {
    stornos.add('$orderId:$paperId:$reason');
    if (stornoError != null) throw stornoError!;
    papers.removeWhere((p) => p['id'] == paperId);
    return {'id': paperId};
  }
}

const _read = {'purchase_orders.read'};
const _write = {'purchase_orders.read', 'purchase_orders.write'};
const _items = [
  {'id': 'i1', 'position': 1, 'bezeichnung': 'HEB 200'},
];

browser.PickedFile _file(int size) =>
    browser.PickedFile(Uint8List(size), 'zeugnis.pdf', 'application/pdf');

Future<void> _open(WidgetTester tester, _FakeApi api,
    {String status = 'ordered',
    List<browser.PickedFile?> picks = const []}) async {
  final queue = List.of(picks);
  await tester.binding.setSurfaceSize(const Size(1200, 1400));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: PurchaseOrderPapersPage(
          key: UniqueKey(),
          api: api,
          orderId: 'po1',
          orderNumber: 'B-7',
          orderStatus: status,
          items: _items,
          filePicker: ({String? accept}) async =>
              queue.isEmpty ? null : queue.removeAt(0))));
  await tester.pumpAndSettle();
}

Future<void> _choose(WidgetTester tester, String current, String next) async {
  await tester.tap(find.text(current).last);
  await tester.pumpAndSettle();
  await tester.tap(find.text(next).last);
  await tester.pumpAndSettle();
}

FilledButton _button(WidgetTester tester, String label) => tester.widget(
    find.ancestor(of: find.text(label), matching: find.byType(FilledButton)));

void main() {
  test('API: Upload mit Formularfeldern, Storno, Fehler', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <String>[];
    final bodies = <String>[];
    final client = MockClient((req) async {
      calls.add('${req.method} ${req.url.path}');
      final body = utf8.decode(req.bodyBytes, allowMalformed: true);
      bodies.add(body);
      if (body.contains('filename="kaputt.exe"') || body.contains('"x"')) {
        return http.Response(
            '{"error":{"code":"validation_error","message":"Ungültig"}}', 400,
            headers: {'content-type': 'application/json'});
      }
      return http.Response(
          '{"id":"p9"}', req.url.path.endsWith('/storno') ? 200 : 201,
          headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      expect(
          await api.uploadPurchaseOrderPaper(
              'po1', 'z.pdf', Uint8List.fromList([1]),
              art: 'zeugnis_3_1',
              positionId: 'i1',
              charge: ' C-1 ',
              notiz: 'n'),
          {'id': 'p9'});
      await api.uploadPurchaseOrderPaper(
          'po1', 'g.pdf', Uint8List.fromList([1]),
          art: 'leistungserklaerung');
      expect(await api.stornoPurchaseOrderPaper('po1', 'p1', 'falsch'),
          {'id': 'p9'});
      await expectLater(
          api.uploadPurchaseOrderPaper(
              'po1', 'kaputt.exe', Uint8List.fromList([1]),
              art: 'sonstiges'),
          throwsA(isA<ApiException>()));
      await expectLater(api.stornoPurchaseOrderPaper('po1', 'p1', 'x'),
          throwsA(isA<ApiException>()));
    }, () => client);
    expect(calls, [
      'POST /api/v1/purchase-orders/po1/papers/',
      'POST /api/v1/purchase-orders/po1/papers/',
      'POST /api/v1/purchase-orders/po1/papers/p1/storno',
      'POST /api/v1/purchase-orders/po1/papers/',
      'POST /api/v1/purchase-orders/po1/papers/p1/storno',
    ]);
    expect(bodies[0], contains('name="art"\r\n\r\nzeugnis_3_1'));
    expect(bodies[0], contains('name="position_id"\r\n\r\ni1'));
    expect(bodies[0], contains('name="charge"\r\n\r\nC-1\r\n'));
    expect(bodies[0], contains('name="file"; filename="z.pdf"'));
    expect(bodies[1], isNot(contains('position_id')));
    expect(jsonDecode(bodies[2]), {'reason': 'falsch'});
  });

  testWidgets('Ohne Schreibrecht: kein Hochladen, kein Stornieren',
      (tester) async {
    await _open(tester, _FakeApi(permissions: _read));
    expect(find.text('Lieferschein'), findsOneWidget);
    expect(find.byTooltip('Lieferpapier hochladen'), findsNothing);
    expect(find.byTooltip('Stornieren'), findsNothing);
  });

  testWidgets('Stornierte Bestellung: kein Hochladen, Storno bleibt',
      (tester) async {
    await _open(
        tester,
        _FakeApi(permissions: _write, extra: [
          {
            'id': 'p0',
            'art': 'zeugnis_2_2',
            'filename': 'alt.pdf',
            'length': 10,
            'storno_grund': 'ersetzt',
            'storniert_am': '2031-01-05T10:00:00Z',
          }
        ]),
        status: 'canceled');
    expect(find.text('Storniert: ersetzt'), findsOneWidget);
    expect(find.byTooltip('Lieferpapier hochladen'), findsNothing);
    expect(find.byTooltip('Stornieren'), findsOneWidget);
  });

  testWidgets('Hochladen mit Art, Position, Charge und Notiz', (tester) async {
    final api = _FakeApi(permissions: _write);
    await _open(tester, api, picks: [_file(100)]);
    await tester.tap(find.byTooltip('Lieferpapier hochladen'));
    await tester.pumpAndSettle();

    expect(_button(tester, 'Hochladen').onPressed, isNull);
    await tester.tap(find.text('Art'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abnahmeprüfzeugnis 3.1').last);
    await tester.pumpAndSettle();
    expect(_button(tester, 'Hochladen').onPressed, isNull);
    await _choose(tester, 'ganze Bestellung', 'Position 1 – HEB 200');
    await tester.enterText(find.widgetWithText(TextField, 'Charge'), 'C-4711');
    await tester.enterText(find.widgetWithText(TextField, 'Notiz'), 'Träger');
    await tester.tap(find.text('Datei wählen'));
    await tester.pumpAndSettle();
    expect(find.text('zeugnis.pdf · 100 B'), findsOneWidget);
    await tester.tap(find.text('Hochladen'));
    await tester.pumpAndSettle();

    expect(api.uploads,
        ['po1:zeugnis.pdf:100:zeugnis_3_1:i1:C-4711:Träger:application/pdf']);
    expect(api.listCalls, 2);
    expect(find.text('Abnahmeprüfzeugnis 3.1'), findsOneWidget);
  });

  testWidgets('Zu große oder leere Datei wird nicht übernommen',
      (tester) async {
    final api = _FakeApi(permissions: _write);
    await _open(tester, api, picks: [_file(10 * 1024 * 1024 + 1), _file(0)]);
    await tester.tap(find.byTooltip('Lieferpapier hochladen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Art'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Lieferschein').last);
    await tester.pumpAndSettle();

    await tester.tap(find.text('Datei wählen'));
    await tester.pumpAndSettle();
    expect(find.text('Die Datei ist zu groß (10,0 MB, höchstens 10 MB).'),
        findsOneWidget);
    expect(find.text('Keine Datei gewählt'), findsOneWidget);
    expect(_button(tester, 'Hochladen').onPressed, isNull);

    await tester.tap(find.text('Datei wählen'));
    await tester.pumpAndSettle();
    expect(find.text('Die Datei ist leer.'), findsOneWidget);
    await tester.tap(find.text('Datei wählen'));
    await tester.pumpAndSettle();
    expect(find.text('Keine Datei gewählt'), findsOneWidget);
    expect(api.uploads, isEmpty);
  });

  testWidgets('Serverfehler beim Hochladen bleibt im Dialog', (tester) async {
    final api = _FakeApi(
        permissions: _write,
        uploadError: const ApiException(
            statusCode: 400,
            message: 'Bestellung ist storniert',
            code: 'validation_error'));
    await _open(tester, api, picks: [_file(100)]);
    await tester.tap(find.byTooltip('Lieferpapier hochladen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Datei wählen'));
    await tester.pumpAndSettle();
    // Datei ohne Art reicht nicht.
    expect(_button(tester, 'Hochladen').onPressed, isNull);
    await tester.tap(find.text('Art'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Sonstiges').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Hochladen'));
    await tester.pumpAndSettle();

    expect(api.uploads.single, contains(':sonstiges:null::'));
    expect(find.text('Bestellung ist storniert'), findsOneWidget);
    expect(find.text('Lieferpapier hochladen'), findsOneWidget);
    expect(api.listCalls, 1);
  });

  testWidgets('Stornieren nur mit Begründung, Fehler bleibt im Dialog',
      (tester) async {
    final failing = _FakeApi(
        permissions: _write,
        stornoError: const ApiException(
            statusCode: 409,
            message: 'Lieferpapier ist bereits storniert',
            code: 'conflict'));
    await _open(tester, failing);
    await tester.tap(find.byTooltip('Stornieren'));
    await tester.pumpAndSettle();
    expect(_button(tester, 'Stornieren').onPressed, isNull);
    await tester.enterText(find.byType(TextField), 'falsch');
    await tester.pump();
    await tester.tap(find.widgetWithText(FilledButton, 'Stornieren'));
    await tester.pumpAndSettle();
    expect(find.text('Lieferpapier ist bereits storniert'), findsOneWidget);
    expect(failing.listCalls, 1);

    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    final api = _FakeApi(permissions: _write);
    await _open(tester, api);
    await tester.tap(find.byTooltip('Stornieren'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), ' ersetzt ');
    await tester.pump();
    await tester.tap(find.widgetWithText(FilledButton, 'Stornieren'));
    await tester.pumpAndSettle();
    expect(api.stornos, ['po1:p1:ersetzt']);
    expect(api.listCalls, 2);
    expect(find.text('Keine Lieferpapiere'), findsOneWidget);
  });
}
