import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/supplier_certificates_page.dart';

// Zertifikate am Lieferanten anlegen und stornieren (Backlog H.4.4c2).

class _FakeApi extends ApiClient {
  _FakeApi({this.permissions = const {}, this.items = const [], this.error})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<dynamic> items;
  final Object? error;
  int listCalls = 0;
  final List<Map<String, dynamic>> created = [];
  final List<String> stornos = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listSupplierCertificates(String supplierId,
      {bool includeStorniert = false}) async {
    listCalls++;
    return items;
  }

  @override
  Future<Map<String, dynamic>> createSupplierCertificate(
      Map<String, dynamic> body) async {
    created.add(body);
    if (error != null) throw error!;
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> stornoSupplierCertificate(
      String id, String reason) async {
    stornos.add('$id:$reason');
    if (error != null) throw error!;
    return {'id': id};
  }
}

const _write = {'purchase_orders.read', 'purchase_orders.write'};
const _active = {'id': 'c1', 'art': 'en1090', 'nummer': 'Z-1'};
const _cancelled = {
  'id': 'c2',
  'art': 'iso3834',
  'nummer': 'Z-2',
  'storno_grund': 'ersetzt',
  'storniert_am': '2031-01-05T10:00:00Z',
};

Future<void> _pump(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1200, 1600));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: SupplierCertificatesPage(
          key: UniqueKey(),
          api: api,
          supplierId: 's1',
          supplierName: 'Stahlbau AG')));
  await tester.pumpAndSettle();
}

Finder _field(String label) => find.widgetWithText(TextFormField, label);

Future<void> _pickExc(WidgetTester tester, String k) async {
  await tester.tap(find.widgetWithText(
      DropdownButtonFormField<String>, 'Ausführungsklasse bis'));
  await tester.pumpAndSettle();
  await tester.tap(find.text(k).last);
  await tester.pumpAndSettle();
}

void main() {
  test('Body: EN 1090 mit Klasse und Daten, sonst ohne Klasse', () {
    expect(
        buildSupplierCertificateBody(
            supplierId: 's1',
            art: 'en1090',
            nummer: ' Z-1 ',
            stelle: 'Stelle',
            excBis: 'EXC2',
            von: '1.2.2031',
            bis: '2031-12-31',
            notiz: ' n '),
        {
          'lieferant_id': 's1',
          'art': 'en1090',
          'nummer': 'Z-1',
          'stelle': 'Stelle',
          'exc_bis': 'EXC2',
          'gueltig_von': '2031-02-01',
          'gueltig_bis': '2031-12-31',
          'notiz': 'n',
        });
    expect(
        buildSupplierCertificateBody(
            supplierId: 's1', art: 'iso3834', excBis: 'EXC2', von: '  '),
        {
          'lieferant_id': 's1',
          'art': 'iso3834',
          'nummer': '',
          'stelle': '',
          'notiz': '',
        });
  });

  test('API: anlegen und stornieren, Fehler wirft ApiException', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      if (req.url.path.contains('/c9/')) {
        return http.Response(
            '{"error":{"code":"conflict","message":"Zertifikat ist bereits storniert"}}',
            409,
            headers: {'content-type': 'application/json'});
      }
      return http.Response.bytes(utf8.encode('{"id":"c1"}'), 201,
          headers: {'content-type': 'application/json'});
    });
    await http.runWithClient(() async {
      expect(
          await api.createSupplierCertificate({'art': 'en1090'}), {'id': 'c1'});
      await api.stornoSupplierCertificate('c1', 'ersetzt');
      await expectLater(api.stornoSupplierCertificate('c9', 'x'),
          throwsA(isA<ApiException>()));
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}'), [
      'POST /api/v1/supplier-certificates/',
      'POST /api/v1/supplier-certificates/c1/storno',
      'POST /api/v1/supplier-certificates/c9/storno',
    ]);
    expect(jsonDecode(calls[0].body), {'art': 'en1090'});
    expect(jsonDecode(calls[1].body), {'reason': 'ersetzt'});
  });

  testWidgets('Ohne purchase_orders.write: weder anlegen noch stornieren',
      (tester) async {
    await _pump(
        tester,
        _FakeApi(
            permissions: {'purchase_orders.read'}, items: const [_active]));
    expect(find.text('EN 1090  •  Z-1'), findsOneWidget);
    expect(find.byTooltip('Zertifikat anlegen'), findsNothing);
    expect(find.byTooltip('Stornieren'), findsNothing);
  });

  testWidgets('Anlegen prüft Klasse und Datum, sendet und lädt neu',
      (tester) async {
    final api = _FakeApi(permissions: _write);
    await _pump(tester, api);
    await tester.tap(find.byTooltip('Zertifikat anlegen'));
    await tester.pumpAndSettle();

    // Ohne Ausfuehrungsklasse und mit falschem Datum: nichts gesendet.
    await tester.enterText(_field('Gültig bis (TT.MM.JJJJ)'), '31.02.2031');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(find.text('Pflichtfeld bei EN 1090'), findsOneWidget);
    expect(find.text('Datum als TT.MM.JJJJ'), findsOneWidget);
    expect(api.created, isEmpty);

    await _pickExc(tester, 'EXC3');
    await tester.enterText(_field('Gültig bis (TT.MM.JJJJ)'), '28.02.2031');
    await tester.enterText(_field('Nummer'), 'Z-9');
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();

    expect(api.created.single, {
      'lieferant_id': 's1',
      'art': 'en1090',
      'nummer': 'Z-9',
      'stelle': '',
      'exc_bis': 'EXC3',
      'gueltig_bis': '2031-02-28',
      'notiz': '',
    });
    expect(find.text('Zertifikat anlegen'), findsNothing);
    expect(api.listCalls, 2);
  });

  testWidgets('Anlegen: andere Art ohne Klasse, Serverfehler bleibt im Dialog',
      (tester) async {
    final api = _FakeApi(
        permissions: _write,
        error: const ApiException(
            statusCode: 400,
            message: 'Ungültiger Zeitraum: gültig bis liegt vor gültig von',
            code: 'validation_error'));
    await _pump(tester, api);
    await tester.tap(find.byTooltip('Zertifikat anlegen'));
    await tester.pumpAndSettle();
    await tester
        .tap(find.widgetWithText(DropdownButtonFormField<String>, 'Art'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('ISO 3834').last);
    await tester.pumpAndSettle();
    expect(find.text('Ausführungsklasse bis'), findsNothing);

    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();
    expect(api.created.single['art'], 'iso3834');
    expect(api.created.single.containsKey('exc_bis'), isFalse);
    expect(find.text('Ungültiger Zeitraum: gültig bis liegt vor gültig von'),
        findsOneWidget);
    expect(find.text('Zertifikat anlegen'), findsOneWidget);
    expect(api.listCalls, 1);
  });

  testWidgets('Stornieren braucht Begründung, Stornierte ohne Knopf',
      (tester) async {
    final api =
        _FakeApi(permissions: _write, items: const [_active, _cancelled]);
    await _pump(tester, api);
    expect(find.byTooltip('Stornieren'), findsOneWidget);

    await tester.tap(find.byTooltip('Stornieren'));
    await tester.pumpAndSettle();
    final button = find.widgetWithText(FilledButton, 'Stornieren');
    expect(tester.widget<FilledButton>(button).onPressed, isNull);
    await tester.enterText(
        find.widgetWithText(TextField, 'Begründung'), ' ersetzt ');
    await tester.pumpAndSettle();
    await tester.tap(button);
    await tester.pumpAndSettle();

    expect(api.stornos, ['c1:ersetzt']);
    expect(find.text('Zertifikat stornieren'), findsNothing);
    expect(api.listCalls, 2);
  });

  testWidgets('Stornieren: Serverfehler bleibt im Dialog', (tester) async {
    final api = _FakeApi(
        permissions: _write,
        items: const [_active],
        error: const ApiException(
            statusCode: 409,
            message: 'Zertifikat ist bereits storniert',
            code: 'conflict'));
    await _pump(tester, api);
    await tester.tap(find.byTooltip('Stornieren'));
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, 'Begründung'), 'x');
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Stornieren'));
    await tester.pumpAndSettle();

    expect(find.text('Zertifikat ist bereits storniert'), findsOneWidget);
    expect(find.text('Zertifikat stornieren'), findsOneWidget);
    expect(api.listCalls, 1);
  });
}
