import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';

// Kontoauszuege im Client (Backlog 0.48): bank_statements_page.dart rief
// listBankStatements/matchBankStatement auf, die im ApiClient fehlten.

void main() {
  test('API: Kontoauszuege listen und zuordnen', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final body = req.method == 'GET' ? '[{"id":"b1"}]' : '{"payment_id":"p1"}';
      return http.Response.bytes(utf8.encode(body), 200, headers: {'content-type': 'application/json'});
    });
    late List<dynamic> list;
    late Map<String, dynamic> matched;
    await http.runWithClient(() async {
      list = await api.listBankStatements();
      await api.listBankStatements(limit: 50, offset: 100);
      matched = await api.matchBankStatement('b1', invoiceId: 'inv-1');
      await api.matchBankStatement('b2');
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}?${c.url.query}'), [
      'GET /api/v1/bank-statements?',
      'GET /api/v1/bank-statements?limit=50&offset=100',
      'POST /api/v1/bank-statements/b1/match?',
      'POST /api/v1/bank-statements/b2/match?',
    ]);
    expect(list, [
      {'id': 'b1'}
    ]);
    expect(matched['payment_id'], 'p1');
    expect(jsonDecode(calls[2].body), {'invoice_id': 'inv-1'});
    // Ohne Rechnung kein invoice_id: der Server sucht selbst (Verwendungszweck, Betrag).
    expect(jsonDecode(calls[3].body), <String, dynamic>{});
  });

  test('API: fehlgeschlagene Zuordnung wirft ApiException', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final notFound = MockClient((req) async => http.Response(
        '{"error":{"code":"not_found","message":"Keine passende Rechnung"}}', 404,
        headers: {'content-type': 'application/json'}));
    await expectLater(
        http.runWithClient(() => api.matchBankStatement('b1'), () => notFound), throwsA(isA<ApiException>()));
  });
}
