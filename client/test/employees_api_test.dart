import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';

// Mitarbeiter-Methoden des ApiClient gegen einen MockClient (Backlog
// F.7.1): Methode, Pfad, Query, Body und Auswertung der Antwort.
// http.runWithClient leitet die Aufrufe der Top-Level-Funktionen von
// package:http (http.get, http.post, ...) auf den MockClient um.

class _Recorded {
  _Recorded(this.method, this.url, this.body, this.contentType);
  final String method;
  final Uri url;
  final String body;
  final String? contentType;
}

Future<T> _withMock<T>(
  List<_Recorded> calls,
  http.Response Function(http.Request) respond,
  Future<T> Function() body,
) {
  final mock = MockClient((req) async {
    calls.add(_Recorded(req.method, req.url, req.body, req.headers['content-type']));
    return respond(req);
  });
  return http.runWithClient(body, () => mock);
}

http.Response _json(Object body, int status) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

void main() {
  final api = ApiClient(baseUrl: 'http://api.test');

  test('listEmployees: GET mit limit und offset, liefert die Liste', () async {
    final calls = <_Recorded>[];
    final out = await _withMock(calls, (_) => _json([
          {'id': 'e1', 'first_name': 'Anna', 'last_name': 'Bärg'}
        ], 200), () => api.listEmployees(limit: 50, offset: 100));
    expect(calls.single.method, 'GET');
    expect(calls.single.url.path, '/api/v1/employees/');
    expect(calls.single.url.queryParameters, {'limit': '50', 'offset': '100'});
    expect(out.single['last_name'], 'Bärg');
  });

  test('listEmployees ohne Angaben sendet keine Query', () async {
    final calls = <_Recorded>[];
    await _withMock(calls, (_) => _json([], 200), () => api.listEmployees());
    expect(calls.single.url.query, isEmpty);
  });

  test('getEmployee und listTeams: Pfade', () async {
    final calls = <_Recorded>[];
    final emp = await _withMock(calls, (_) => _json({'id': 'e1'}, 200), () => api.getEmployee('e1'));
    final teams = await _withMock(calls, (_) => _json([
          {'id': 't1', 'name': 'Montage'}
        ], 200), () => api.listTeams());
    expect(calls.map((c) => '${c.method} ${c.url.path}'), ['GET /api/v1/employees/e1', 'GET /api/v1/employees/teams']);
    expect(emp['id'], 'e1');
    expect(teams.single['name'], 'Montage');
  });

  test('createEmployee: POST mit JSON, erwartet 201', () async {
    final calls = <_Recorded>[];
    final out = await _withMock(calls, (_) => _json({'id': 'neu', 'active': true}, 201),
        () => api.createEmployee({'first_name': 'Anna', 'last_name': 'Berg'}));
    expect(calls.single.method, 'POST');
    expect(calls.single.url.path, '/api/v1/employees/');
    expect(calls.single.contentType, startsWith('application/json'));
    expect(jsonDecode(calls.single.body), {'first_name': 'Anna', 'last_name': 'Berg'});
    expect(out['id'], 'neu');

    // 200 statt 201 ist kein Erfolg (der Server antwortet beim Anlegen mit 201).
    expect(
      () => _withMock(<_Recorded>[], (_) => _json({'id': 'x'}, 200), () => api.createEmployee({'first_name': 'A'})),
      throwsA(isA<ApiException>()),
    );
  });

  test('updateEmployee: PATCH nur mit den genannten Feldern, liefert den Mitarbeiter', () async {
    final calls = <_Recorded>[];
    final out = await _withMock(calls, (_) => _json({'id': 'e1', 'active': false}, 200),
        () => api.updateEmployee('e1', {'active': false, 'team_id': null}));
    expect(calls.single.method, 'PATCH');
    expect(calls.single.url.path, '/api/v1/employees/e1');
    expect(calls.single.contentType, startsWith('application/json'));
    expect(jsonDecode(calls.single.body), {'active': false, 'team_id': null});
    expect(out['active'], false);
  });

  test('Fehlerantworten werden zur ApiException mit Meldung und Code', () async {
    Future<ApiException> caught(Future<Object?> Function() call) async {
      try {
        await _withMock(<_Recorded>[], (_) => _json({'error': {'code': 'validation_error', 'message': 'unbekanntes Feld: activ'}}, 400), call);
      } on ApiException catch (e) {
        return e;
      }
      fail('ApiException erwartet');
    }

    for (final call in <Future<Object?> Function()>[
      () => api.listEmployees(),
      () => api.getEmployee('e1'),
      () => api.listTeams(),
      () => api.createEmployee({'first_name': 'A'}),
      () => api.updateEmployee('e1', {'activ': true}),
    ]) {
      final e = await caught(call);
      expect(e.statusCode, 400);
      expect(e.code, 'validation_error');
      expect(e.message, 'unbekanntes Feld: activ');
    }
  });
}
