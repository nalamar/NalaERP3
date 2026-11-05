import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/supplier_certificates_page.dart';
import 'package:nalaerp_client/web/browser.dart' as browser;

// Dateien am Zertifikat des Lieferanten (Backlog H.4.4c3).

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, required this.items, this.uploadError})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<dynamic> items;
  final Object? uploadError;
  final List<Map<String, dynamic>> docs = [
    {'id': 'd1', 'filename': 'zertifikat.pdf', 'length': 2048}
  ];
  int docListCalls = 0;
  final List<String> uploads = [];
  final List<String> downloads = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listSupplierCertificates(String supplierId,
          {bool includeStorniert = false}) async =>
      items;

  @override
  Future<List<dynamic>> listSupplierCertificateDocuments(
      String certificateId) async {
    docListCalls++;
    return List.of(docs);
  }

  @override
  Future<Map<String, dynamic>> uploadSupplierCertificateDocument(
      String certificateId, String filename, Uint8List bytes,
      {String? contentType}) async {
    uploads.add('$certificateId:$filename:${bytes.length}:$contentType');
    if (uploadError != null) throw uploadError!;
    final d = {'id': 'd2', 'filename': filename, 'length': bytes.length};
    docs.add(d);
    return d;
  }

  @override
  Future<void> downloadSupplierCertificateDocument(
      String certificateId, String documentId,
      {String? filename}) async {
    downloads.add('$certificateId:$documentId:$filename');
  }
}

const _read = {'purchase_orders.read'};
const _write = {'purchase_orders.read', 'purchase_orders.write'};
const _active = {'id': 'c1', 'art': 'en1090', 'nummer': 'Z-1'};
const _cancelled = {
  'id': 'c2',
  'art': 'iso3834',
  'nummer': 'Z-2',
  'storno_grund': 'ersetzt',
  'storniert_am': '2031-01-05T10:00:00Z',
};

Future<void> _openDocs(WidgetTester tester, _FakeApi api,
    {browser.PickedFile? pick, int index = 0}) async {
  await tester.binding.setSurfaceSize(const Size(1200, 1600));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: SupplierCertificatesPage(
          key: UniqueKey(),
          api: api,
          supplierId: 's1',
          supplierName: 'Stahlbau AG',
          filePicker: ({String? accept}) async => pick)));
  await tester.pumpAndSettle();
  await tester.tap(find.byTooltip('Dateien').at(index));
  await tester.pumpAndSettle();
}

browser.PickedFile _file(int size) =>
    browser.PickedFile(Uint8List(size), 'neu.pdf', 'application/pdf');

void main() {
  test('API: Dateien listen, hochladen, herunterladen', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <String>[];
    final client = MockClient((req) async {
      calls.add('${req.method} ${req.url.path}');
      if (req.method == 'POST') {
        final body = utf8.decode(req.bodyBytes, allowMalformed: true);
        if (body.contains('filename="kaputt.exe"')) {
          return http.Response(
              '{"error":{"code":"validation_error","message":"Ungültige Datei: erlaubt sind PDF, JPEG und PNG"}}',
              400,
              headers: {'content-type': 'application/json'});
        }
        expect(body, contains('name="file"; filename="z.pdf"'));
        return http.Response('{"id":"d9"}', 201,
            headers: {'content-type': 'application/json'});
      }
      if (req.url.path.endsWith('/documents')) {
        return http.Response('[{"id":"d1"}]', 200,
            headers: {'content-type': 'application/json'});
      }
      return http.Response.bytes([1, 2, 3], 200,
          headers: {'content-type': 'application/pdf'});
    });
    await http.runWithClient(() async {
      expect(await api.listSupplierCertificateDocuments('c1'), [
        {'id': 'd1'}
      ]);
      expect(
          await api.uploadSupplierCertificateDocument(
              'c1', 'z.pdf', Uint8List.fromList([1]),
              contentType: 'application/pdf'),
          {'id': 'd9'});
      await api.downloadSupplierCertificateDocument('c1', 'd1',
          filename: 'z.pdf');
      await expectLater(
          api.uploadSupplierCertificateDocument(
              'c1', 'kaputt.exe', Uint8List.fromList([1])),
          throwsA(isA<ApiException>()));
    }, () => client);
    expect(calls, [
      'GET /api/v1/supplier-certificates/c1/documents',
      'POST /api/v1/supplier-certificates/c1/documents',
      'GET /api/v1/supplier-certificates/c1/documents/d1',
      'POST /api/v1/supplier-certificates/c1/documents',
    ]);
  });

  testWidgets('Nur lesen: Dateien sehen und herunterladen, nicht hochladen',
      (tester) async {
    final api = _FakeApi(permissions: _read, items: const [_active]);
    await _openDocs(tester, api);

    expect(find.text('Dateien zum Zertifikat'), findsOneWidget);
    expect(find.text('zertifikat.pdf · 2 KB'), findsOneWidget);
    expect(find.text('Hochladen'), findsNothing);
    await tester.tap(find.byTooltip('Herunterladen'));
    await tester.pumpAndSettle();
    expect(api.downloads, ['c1:d1:zertifikat.pdf']);
  });

  testWidgets('Hochladen sendet die Datei und lädt die Liste neu',
      (tester) async {
    final api = _FakeApi(permissions: _write, items: const [_active]);
    await _openDocs(tester, api, pick: _file(100));
    await tester.tap(find.text('Hochladen'));
    await tester.pumpAndSettle();

    expect(api.uploads, ['c1:neu.pdf:100:application/pdf']);
    expect(api.docListCalls, 2);
    expect(find.text('neu.pdf · 100 B'), findsOneWidget);
  });

  testWidgets('Zu große, leere oder keine Datei: nichts gesendet',
      (tester) async {
    final api = _FakeApi(permissions: _write, items: const [_active]);
    await _openDocs(tester, api, pick: _file(10 * 1024 * 1024 + 1));
    await tester.tap(find.text('Hochladen'));
    await tester.pumpAndSettle();
    expect(find.text('Die Datei ist zu groß (10,0 MB, höchstens 10 MB).'),
        findsOneWidget);

    await tester.tap(find.text('Schließen'));
    await tester.pumpAndSettle();
    await _openDocs(tester, api, pick: _file(0));
    await tester.tap(find.text('Hochladen'));
    await tester.pumpAndSettle();
    expect(find.text('Die Datei ist leer.'), findsOneWidget);

    await tester.tap(find.text('Schließen'));
    await tester.pumpAndSettle();
    await _openDocs(tester, api);
    await tester.tap(find.text('Hochladen'));
    await tester.pumpAndSettle();
    expect(api.uploads, isEmpty);
  });

  testWidgets('Serverfehler beim Hochladen wird angezeigt', (tester) async {
    final api = _FakeApi(
        permissions: _write,
        items: const [_active],
        uploadError: const ApiException(
            statusCode: 400,
            message: 'Ungültige Datei: erlaubt sind PDF, JPEG und PNG',
            code: 'validation_error'));
    await _openDocs(tester, api, pick: _file(100));
    await tester.tap(find.text('Hochladen'));
    await tester.pumpAndSettle();

    expect(find.text('Ungültige Datei: erlaubt sind PDF, JPEG und PNG'),
        findsOneWidget);
    expect(api.docListCalls, 1);
    expect(find.text('Hochladen'), findsOneWidget);
  });

  testWidgets('Storniertes Zertifikat: Dateien lesbar, kein Hochladen',
      (tester) async {
    final api =
        _FakeApi(permissions: _write, items: const [_active, _cancelled]);
    await _openDocs(tester, api, index: 1);

    expect(find.text('ISO 3834  •  Z-2'), findsWidgets);
    expect(find.text('zertifikat.pdf · 2 KB'), findsOneWidget);
    expect(find.text('Hochladen'), findsNothing);
  });
}
