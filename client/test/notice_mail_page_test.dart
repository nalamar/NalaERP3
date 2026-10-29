import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';
import 'package:nalaerp_client/pages/mail_settings_page.dart';
import 'package:nalaerp_client/pages/notice_mail_page.dart';

// Hinweis-Mails im Client (Backlog F.14.4c2). Erfundene Namen.

class _FakeApi extends ApiClient {
  _FakeApi({this.abbestellt = false, this.log = const [], this.saveError})
      : super(baseUrl: 'http://localhost:8080');

  bool abbestellt;
  final List<Map<String, dynamic>> log;
  final ApiException? saveError;
  final List<bool> saved = [];
  int? logLimit;

  @override
  Future<bool> getNoticeMailOptOut() async => abbestellt;

  @override
  Future<bool> setNoticeMailOptOut(bool v) async {
    saved.add(v);
    if (saveError != null) throw saveError!;
    abbestellt = v;
    return v;
  }

  @override
  Future<List<dynamic>> listNoticeMailLog({int? limit}) async {
    logLimit = limit;
    return log;
  }

  @override
  Future<Map<String, dynamic>> getMailSettings() async => {
        'eingerichtet': true,
        'aktiv': true,
        'smtp_host': 'mail.example.test',
        'smtp_port': 465,
        'sicherheit': 'ssl',
        'benutzer': 'erp@example.test',
        'absender_adresse': 'erp@example.test',
        'absender_name': '',
        'passwort_gesetzt': true,
        'passwort_lesbar': true,
        'schluessel_eingerichtet': true,
      };
}

Future<void> _pump(WidgetTester tester, Widget w) async {
  await tester.binding.setSurfaceSize(const Size(1200, 1600));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(home: w));
  await tester.pumpAndSettle();
}

bool _switchValue(WidgetTester tester) =>
    tester.widget<SwitchListTile>(find.byType(SwitchListTile)).value;

void main() {
  test('noticeMailLogLine', () {
    expect(
        noticeMailLogLine({
          'versand_tag': '2026-10-29T00:00:00Z',
          'benutzer': 'Anna Berg',
          'ergebnis': 'gesendet'
        }),
        '29.10.2026 · Anna Berg · gesendet');
    expect(
        noticeMailLogLine({
          'versand_tag': '2026-10-29T00:00:00Z',
          'benutzer': 'Anna Berg',
          'ergebnis': 'fehler',
          'fehler_stufe': 'anmeldung'
        }),
        '29.10.2026 · Anna Berg · Fehler: Anmeldung fehlgeschlagen');
    expect(
        noticeMailLogLine({
          'versand_tag': '2026-10-29T00:00:00Z',
          'benutzer': '',
          'ergebnis': 'fehler',
          'fehler_stufe': 'neu'
        }),
        '29.10.2026 · (Benutzer entfernt) · Fehler: neu');
    expect(
        noticeMailLogLine({
          'versand_tag': '2026-10-30T00:00:00Z',
          'benutzer': 'B',
          'ergebnis': 'versand'
        }),
        '30.10.2026 · B · Versand begonnen, nicht abgeschlossen');
    for (final s in [
      'verbindung',
      'tls',
      'anmeldung',
      'absender',
      'empfaenger',
      'nachricht',
      'eingabe'
    ]) {
      expect(noticeMailStufen[s], isNotNull, reason: s);
    }
  });

  testWidgets('abbestellen und wieder bestellen', (tester) async {
    final api = _FakeApi();
    await _pump(tester, NoticeMailPrefsPage(api: api));
    expect(_switchValue(tester), isTrue);
    await tester.tap(find.text('Hinweis-Mails erhalten'));
    await tester.pumpAndSettle();
    expect(api.saved, [true]);
    expect(find.text('Hinweis-Mails abbestellt'), findsOneWidget);
    expect(_switchValue(tester), isFalse);
    await tester.tap(find.text('Hinweis-Mails erhalten'));
    await tester.pumpAndSettle();
    expect(api.saved, [true, false]);
    expect(_switchValue(tester), isTrue);
  });

  testWidgets(
      'abbestellt geladen; Fehler beim Speichern erscheint, Stand bleibt',
      (tester) async {
    final api = _FakeApi(
        abbestellt: true,
        saveError:
            const ApiException(statusCode: 500, message: 'Serverfehler'));
    await _pump(tester, NoticeMailPrefsPage(api: api));
    expect(_switchValue(tester), isFalse);
    await tester.tap(find.text('Hinweis-Mails erhalten'));
    await tester.pumpAndSettle();
    expect(api.saved, [false]);
    expect(find.text('Speichern fehlgeschlagen: Serverfehler'), findsOneWidget);
    expect(_switchValue(tester), isFalse);
  });

  testWidgets('Protokoll auf der Seite E-Mail-Versand', (tester) async {
    final api = _FakeApi(log: [
      {
        'versand_tag': '2026-10-30T00:00:00Z',
        'benutzer': 'Anna Berg',
        'ergebnis': 'gesendet'
      },
      {
        'versand_tag': '2026-10-29T00:00:00Z',
        'benutzer': 'Bert Cord',
        'ergebnis': 'fehler',
        'fehler_stufe': 'tls'
      },
    ]);
    await _pump(tester, MailSettingsPage(api: api));
    await tester.scrollUntilVisible(
        find.text('30.10.2026 · Anna Berg · gesendet'), 200,
        scrollable: find.byType(Scrollable).first);
    expect(api.logLimit, 50);
    expect(
        find.text(
            '29.10.2026 · Bert Cord · Fehler: verschlüsselte Verbindung fehlgeschlagen'),
        findsOneWidget);
    expect(find.text('Noch keine Hinweis-Mail versandt.'), findsNothing);

    await _pump(
        tester, NoticeMailLogSection(key: UniqueKey(), api: _FakeApi()));
    expect(find.text('Noch keine Hinweis-Mail versandt.'), findsOneWidget);
  });

  testWidgets('Dashboard: Hinweis-Mails fuer jeden Benutzer', (tester) async {
    final api = _FakeApi();
    await _pump(tester,
        DashboardPage(api: api, onLogout: () async {}, permissions: const {}));
    await tester.tap(find.byTooltip('Hinweis-Mails'));
    await tester.pumpAndSettle();
    expect(find.byType(NoticeMailPrefsPage), findsOneWidget);
  });

  test('API: Pfade, Methoden, Bodies', () async {
    final calls = <http.Request>[];
    final api = ApiClient(baseUrl: 'http://api.test');
    Future<T> mock<T>(Object body, Future<T> Function() call,
        {int status = 200}) {
      final client = MockClient((req) async {
        calls.add(req);
        return http.Response.bytes(utf8.encode(jsonEncode(body)), status,
            headers: {'content-type': 'application/json'});
      });
      return http.runWithClient(call, () => client);
    }

    expect(await mock({'abbestellt': true}, () => api.getNoticeMailOptOut()),
        isTrue);
    expect(calls.last.method, 'GET');
    expect(calls.last.url.path, '/api/v1/notice-mails/me');
    expect(
        await mock({'abbestellt': false}, () => api.setNoticeMailOptOut(false)),
        isFalse);
    expect(calls.last.method, 'PUT');
    expect(calls.last.url.path, '/api/v1/notice-mails/me');
    expect(jsonDecode(calls.last.body), {'abbestellt': false});
    await expectLater(
        mock({
          'error': {'code': 'x', 'message': 'x'}
        }, () => api.setNoticeMailOptOut(true), status: 400),
        throwsA(isA<ApiException>()));
    await mock([], () => api.listNoticeMailLog(limit: 50));
    expect(calls.last.url.path, '/api/v1/notice-mails/log');
    expect(calls.last.url.queryParameters, {'limit': '50'});
    await mock([], () => api.listNoticeMailLog());
    expect(calls.last.url.queryParameters, isEmpty);
  });
}
