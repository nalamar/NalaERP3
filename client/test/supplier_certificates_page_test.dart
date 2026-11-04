import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/contact_detail_screen.dart';
import 'package:nalaerp_client/pages/supplier_certificates_page.dart';

// Zertifikate am Lieferanten im Client, lesend (Backlog H.4.4c1).

class _FakeApi extends ApiClient {
  _FakeApi(
      {this.permissions = const {},
      this.rolle = 'supplier',
      this.active = const [],
      this.all = const [],
      this.error})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final String rolle;
  final List<dynamic> active;
  final List<dynamic> all;
  final Object? error;
  final List<String> calls = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listSupplierCertificates(String supplierId,
      {bool includeStorniert = false}) async {
    calls.add('$supplierId:$includeStorniert');
    if (error != null) throw error!;
    return includeStorniert ? all : active;
  }

  @override
  Future<Map<String, dynamic>> getContact(String id) async =>
      {'id': id, 'name': 'Stahlbau AG', 'rolle': rolle, 'typ': 'org'};
  @override
  Future<List<dynamic>> listContactAddresses(String contactId) async => [];
  @override
  Future<List<dynamic>> listContactPersons(String contactId) async => [];
  @override
  Future<List<dynamic>> listContactNotes(String contactId) async => [];
  @override
  Future<List<dynamic>> listContactTasks(String contactId) async => [];
  @override
  Future<List<dynamic>> listContactDocuments(String contactId) async => [];
  @override
  Future<List<dynamic>> listContactActivity(String contactId) async => [];
  @override
  Future<Map<String, dynamic>> getContactCommercialContext(
          String contactId) async =>
      {};
}

const _en1090 = {
  'id': 'c1',
  'art': 'en1090',
  'nummer': 'Z-123',
  'stelle': 'Prüfstelle',
  'exc_bis': 'EXC2',
  'gueltig_von': '2031-01-01T00:00:00Z',
  'gueltig_bis': '2031-02-15T00:00:00Z',
  'hinweise': ['zertifikat_laeuft_ab'],
};
const _iso = {
  'id': 'c2',
  'art': 'iso3834',
  'nummer': '',
  'stelle': '',
  'exc_bis': null,
  'gueltig_von': null,
  'gueltig_bis': null,
  'hinweise': null,
};
const _storniert = {
  'id': 'c3',
  'art': 'sonstiges',
  'nummer': 'ALT',
  'storno_grund': 'ersetzt',
  'storniert_am': '2031-01-05T10:00:00Z',
};

Future<void> _pumpPage(WidgetTester tester, _FakeApi api) async {
  await tester.pumpWidget(MaterialApp(
      home: SupplierCertificatesPage(
          key: UniqueKey(),
          api: api,
          supplierId: 's1',
          supplierName: 'Stahlbau AG')));
  await tester.pumpAndSettle();
}

Future<void> _pumpContact(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(2200, 1600));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
      home: ContactDetailScreen(key: UniqueKey(), api: api, id: 's1')));
  await tester.pumpAndSettle();
}

void main() {
  test('API: Liste je Lieferant, Stornierte nur auf Wunsch', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <String>[];
    final client = MockClient((req) async {
      calls.add('${req.method} ${req.url.path}?${req.url.query}');
      if (req.url.queryParameters['lieferant_id'] == 'fremd') {
        return http.Response(
            '{"error":{"code":"not_found","message":"Lieferant nicht gefunden"}}',
            404,
            headers: {'content-type': 'application/json'});
      }
      return http.Response.bytes(utf8.encode('[{"id":"c1"}]'), 200,
          headers: {'content-type': 'application/json'});
    });
    late List<dynamic> list;
    await http.runWithClient(() async {
      list = await api.listSupplierCertificates('s1');
      await api.listSupplierCertificates('s1', includeStorniert: true);
      await expectLater(
          api.listSupplierCertificates('fremd'), throwsA(isA<ApiException>()));
    }, () => client);
    expect(calls, [
      'GET /api/v1/supplier-certificates/?lieferant_id=s1',
      'GET /api/v1/supplier-certificates/?lieferant_id=s1&include_storniert=true',
      'GET /api/v1/supplier-certificates/?lieferant_id=fremd',
    ]);
    expect(list, [
      {'id': 'c1'}
    ]);
  });

  testWidgets('Liste zeigt Angaben und Hinweis "läuft bald ab"',
      (tester) async {
    final api = _FakeApi(active: const [_en1090, _iso]);
    await _pumpPage(tester, api);

    expect(api.calls, ['s1:false']);
    expect(find.text('EN 1090  •  Z-123'), findsOneWidget);
    expect(
        find.text(
            'Stelle: Prüfstelle  •  Ausführungsklasse bis EXC2  •  gültig ab 01.01.2031 bis 15.02.2031'),
        findsOneWidget);
    expect(find.text('ISO 3834'), findsOneWidget);
    expect(find.text('unbefristet'), findsOneWidget);
    // Nur das EN-1090-Zertifikat traegt den Hinweis.
    expect(find.text('Läuft bald ab'), findsOneWidget);
    expect(find.textContaining('Storniert:'), findsNothing);
  });

  testWidgets('Stornierte nur auf Wunsch', (tester) async {
    final api = _FakeApi(active: const [_iso], all: const [_iso, _storniert]);
    await _pumpPage(tester, api);
    expect(find.text('Sonstiges  •  ALT'), findsNothing);

    await tester.tap(find.text('Stornierte zeigen'));
    await tester.pumpAndSettle();
    expect(api.calls, ['s1:false', 's1:true']);
    expect(find.text('Sonstiges  •  ALT'), findsOneWidget);
    expect(find.text('Storniert: ersetzt'), findsOneWidget);
  });

  testWidgets('Leere Liste und Fehler', (tester) async {
    await _pumpPage(tester, _FakeApi());
    expect(find.text('Keine Zertifikate'), findsOneWidget);

    await _pumpPage(
        tester,
        _FakeApi(
            error: const ApiException(
                statusCode: 404,
                message: 'Lieferant nicht gefunden',
                code: 'not_found')));
    expect(find.text('Lieferant nicht gefunden'), findsOneWidget);
    expect(find.text('Keine Zertifikate'), findsNothing);
  });

  testWidgets('Kontakt: Zugang fuer Lieferant mit purchase_orders.read',
      (tester) async {
    final api = _FakeApi(permissions: {'purchase_orders.read'}, rolle: 'both');
    await _pumpContact(tester, api);

    await tester.tap(find.byTooltip('Zertifikate'));
    await tester.pumpAndSettle();
    expect(find.text('Zertifikate – Stahlbau AG'), findsOneWidget);
    expect(api.calls, ['s1:false']);
  });

  testWidgets('Kontakt: kein Zugang fuer Kunden oder ohne Recht',
      (tester) async {
    await _pumpContact(tester,
        _FakeApi(permissions: {'purchase_orders.read'}, rolle: 'customer'));
    expect(find.byTooltip('Zertifikate'), findsNothing);

    await _pumpContact(
        tester, _FakeApi(permissions: {'contacts.read', 'contacts.write'}));
    expect(find.byTooltip('Zertifikate'), findsNothing);
  });
}
