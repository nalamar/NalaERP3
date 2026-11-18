import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';

// API der Zuordnung am GAEB-Import (Backlog I.5.5a, Routen I.5.3).

void main() {
  test('API: Lauf, Status, Prüfliste, Sammelbestätigung, Entscheidung',
      () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      final path = req.url.path;
      final (body, code) = switch (req.method) {
        'POST' when path.endsWith('/match') => ('{"status":"laeuft"}', 202),
        'POST' => ('{"bestaetigt":3}', 200),
        'PUT' => ('{"item_id":"i1","entscheidung":"geaendert"}', 200),
        _ when path.endsWith('/matches') => ('[{"item_id":"i1"}]', 200),
        _ => ('{"status":"fertig"}', 200),
      };
      return http.Response.bytes(utf8.encode(body), code,
          headers: {'content-type': 'application/json'});
    });
    late Map<String, dynamic> started, status, decided;
    late List<dynamic> list;
    late int n;
    await http.runWithClient(() async {
      started = await api.startQuoteImportMatch('imp-1');
      await api.startQuoteImportMatch('imp-1', profilId: 'p-1');
      await api.startQuoteImportMatch('imp-1', wortabgleich: true);
      status = await api.getQuoteImportMatchStatus('imp-1');
      list = await api.listQuoteImportMatches('imp-1');
      n = await api.confirmQuoteImportMatches('imp-1');
      decided = await api.decideQuoteImportMatch('imp-1', 'i1', 'aendern',
          leistungId: 'l-2');
      await api.decideQuoteImportMatch('imp-1', 'i2', 'verwerfen');
      await api.decideQuoteImportMatch('imp-1', 'i3', 'verwerfen',
          grund: 'sonstiges', grundText: 'zu teuer');
    }, () => client);
    expect(calls.map((c) => '${c.method} ${c.url.path}'), [
      'POST /api/v1/quotes/imports/imp-1/match',
      'POST /api/v1/quotes/imports/imp-1/match',
      'POST /api/v1/quotes/imports/imp-1/match',
      'GET /api/v1/quotes/imports/imp-1/match',
      'GET /api/v1/quotes/imports/imp-1/matches',
      'POST /api/v1/quotes/imports/imp-1/matches/bestaetigen',
      'PUT /api/v1/quotes/imports/imp-1/matches/i1',
      'PUT /api/v1/quotes/imports/imp-1/matches/i2',
      'PUT /api/v1/quotes/imports/imp-1/matches/i3',
    ]);
    expect(started['status'], 'laeuft');
    expect(status['status'], 'fertig');
    expect(list.single['item_id'], 'i1');
    expect(n, 3);
    expect(decided['entscheidung'], 'geaendert');
    // leer = Standardprofil
    expect(jsonDecode(calls[0].body), <String, dynamic>{});
    expect(jsonDecode(calls[1].body), {'profil_id': 'p-1'});
    expect(jsonDecode(calls[2].body), {'wortabgleich': true});
    expect(
        jsonDecode(calls[6].body), {'aktion': 'aendern', 'leistung_id': 'l-2'});
    expect(jsonDecode(calls[7].body), {'aktion': 'verwerfen'});
    expect(jsonDecode(calls[8].body), {
      'aktion': 'verwerfen',
      'grund': 'sonstiges',
      'grund_text': 'zu teuer'
    });
  });

  test('API: Konflikt beim Start und beim Entscheiden wirft ApiException',
      () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final conflict = MockClient((req) async => http.Response.bytes(
        utf8.encode(
            '{"error":{"code":"conflict","message":"Zuordnung läuft bereits"}}'),
        409,
        headers: {'content-type': 'application/json'}));
    await expectLater(
        http.runWithClient(
            () => api.startQuoteImportMatch('imp-1'), () => conflict),
        throwsA(isA<ApiException>()
            .having((e) => e.message, 'message', 'Zuordnung läuft bereits')));
    await expectLater(
        http.runWithClient(
            () => api.decideQuoteImportMatch('imp-1', 'i1', 'verwerfen'),
            () => conflict),
        throwsA(
            isA<ApiException>().having((e) => e.statusCode, 'status', 409)));
  });
}
