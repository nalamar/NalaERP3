import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/main.dart';
import 'package:nalaerp_client/pages/change_password_page.dart';
import 'package:nalaerp_client/pages/dashboard_page.dart';

// Passwort aendern, erzwungen nach dem ersten Login (Backlog 0.49.2).

class _FakeApi extends ApiClient {
  _FakeApi({this.mustChange = true, this.changeError}) : super(baseUrl: 'http://localhost:8080');

  bool mustChange;
  final Object? changeError;
  final List<String> calls = [];

  @override
  Future<bool> restoreSession() async {
    calls.add('restore');
    return true;
  }

  @override
  Future<Map<String, dynamic>> getCurrentUser() async {
    calls.add('me');
    return {
      'user': {'id': 'u1', 'username': 'Admin', 'must_change_password': mustChange},
      'roles': ['admin'],
      'permissions': ['employees.read'],
    };
  }

  @override
  Future<void> changePassword(String currentPassword, String newPassword) async {
    calls.add('change $currentPassword $newPassword');
    if (changeError != null) throw changeError!;
    mustChange = false;
  }

  @override
  Future<void> logout() async {
    calls.add('logout');
  }
}

Future<void> _pumpGate(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(theme: ThemeData(splashFactory: InkRipple.splashFactory), home: AuthGate(api: api)));
  await tester.pumpAndSettle();
}

Future<void> _fill(WidgetTester tester, String current, String next, String repeat) async {
  final fields = find.byType(TextField);
  await tester.enterText(fields.at(0), current);
  await tester.enterText(fields.at(1), next);
  await tester.enterText(fields.at(2), repeat);
  await tester.tap(find.widgetWithText(FilledButton, 'Passwort ändern'));
  await tester.pumpAndSettle();
}

void main() {
  test('Kennzeichen und Pruefung', () {
    expect(mustChangePassword({'user': {'must_change_password': true}}), isTrue);
    expect(mustChangePassword({'user': {'must_change_password': false}}), isFalse);
    expect(mustChangePassword({'user': {}}), isFalse);
    expect(mustChangePassword(null), isFalse);
    expect(validatePasswordChange(current: '', next: 'Neuespasswort', repeat: 'Neuespasswort'), 'Bitte das aktuelle Passwort angeben.');
    expect(validatePasswordChange(current: 'Admin', next: 'kurz', repeat: 'kurz'), 'Das neue Passwort muss mindestens 10 Zeichen lang sein.');
    expect(validatePasswordChange(current: 'Admin', next: '         x', repeat: '         x'), 'Das neue Passwort muss mindestens 10 Zeichen lang sein.');
    expect(validatePasswordChange(current: 'Admin', next: 'Neuespasswort', repeat: 'Neuespasswor'), 'Die Wiederholung stimmt nicht mit dem neuen Passwort überein.');
    expect(validatePasswordChange(current: 'Altespasswort', next: 'Altespasswort', repeat: 'Altespasswort'),
        'Das neue Passwort muss sich vom bisherigen unterscheiden.');
    expect(validatePasswordChange(current: 'Admin', next: 'Neuespasswort', repeat: 'Neuespasswort'), isNull);
  });

  testWidgets('Erzwungen: statt Dashboard die Wechsel-Seite, danach Dashboard', (tester) async {
    final api = _FakeApi();
    await _pumpGate(tester, api);
    expect(find.byType(ChangePasswordPage), findsOneWidget);
    expect(find.byType(DashboardPage), findsNothing);
    expect(find.text('Bevor Sie weiterarbeiten können, müssen Sie Ihr Passwort ändern.'), findsOneWidget);
    expect(find.byType(BackButton), findsNothing);
    // Pruefung im Client: nichts geht an den Server.
    await _fill(tester, 'Admin', 'kurz', 'kurz');
    expect(find.text('Das neue Passwort muss mindestens 10 Zeichen lang sein.'), findsOneWidget);
    await _fill(tester, 'Admin', 'Neuespasswort', 'Anderespasswort');
    expect(find.text('Die Wiederholung stimmt nicht mit dem neuen Passwort überein.'), findsOneWidget);
    expect(api.calls.where((c) => c.startsWith('change')), isEmpty);
    await _fill(tester, 'Admin', 'Neuespasswort', 'Neuespasswort');
    expect(api.calls, contains('change Admin Neuespasswort'));
    expect(api.calls.where((c) => c == 'me').length, 2);
    expect(find.byType(DashboardPage), findsOneWidget);
    expect(find.byType(ChangePasswordPage), findsNothing);
  });

  testWidgets('Erzwungen: Fehler des Servers bleibt sichtbar; Abmelden', (tester) async {
    final api = _FakeApi(changeError: const ApiException(statusCode: 400, code: 'validation_error', message: 'Ungültiges aktuelles Passwort'));
    await _pumpGate(tester, api);
    await _fill(tester, 'falsch', 'Neuespasswort', 'Neuespasswort');
    expect(find.text('Ungültiges aktuelles Passwort'), findsOneWidget);
    expect(find.byType(ChangePasswordPage), findsOneWidget);
    await tester.tap(find.text('Abmelden'));
    await tester.pumpAndSettle();
    expect(api.calls, contains('logout'));
    expect(find.byType(ChangePasswordPage), findsNothing);
    expect(find.byType(DashboardPage), findsNothing);
  });

  testWidgets('Ohne Kennzeichen direkt das Dashboard; freiwilliger Wechsel schliesst die Seite', (tester) async {
    final api = _FakeApi(mustChange: false);
    await _pumpGate(tester, api);
    expect(find.byType(DashboardPage), findsOneWidget);
    await tester.tap(find.byTooltip('Passwort ändern'));
    await tester.pumpAndSettle();
    expect(find.byType(ChangePasswordPage), findsOneWidget);
    expect(find.text('Bevor Sie weiterarbeiten können, müssen Sie Ihr Passwort ändern.'), findsNothing);
    expect(find.text('Abmelden'), findsNothing);
    await _fill(tester, 'Altespasswort', 'Neuespasswort', 'Neuespasswort');
    expect(api.calls, contains('change Altespasswort Neuespasswort'));
    expect(find.text('Passwort geändert'), findsOneWidget);
    expect(find.byType(ChangePasswordPage), findsNothing);
    expect(find.byType(DashboardPage), findsOneWidget);
  });

  test('API: Passwort aendern', () async {
    final api = ApiClient(baseUrl: 'http://api.test');
    final calls = <http.Request>[];
    final client = MockClient((req) async {
      calls.add(req);
      return http.Response('', 204);
    });
    await http.runWithClient(() => api.changePassword('Admin', 'Neuespasswort'), () => client);
    expect(calls.single.method, 'POST');
    expect(calls.single.url.path, '/api/v1/auth/change-password');
    expect(jsonDecode(calls.single.body), {'current_password': 'Admin', 'new_password': 'Neuespasswort'});
    final ok200 = MockClient((req) async => http.Response('{}', 200, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.changePassword('a', 'b'), () => ok200), throwsA(isA<ApiException>()));
    final bad = MockClient((req) async =>
        http.Response('{"error":{"code":"validation_error","message":"Ungültiges aktuelles Passwort"}}', 400, headers: {'content-type': 'application/json'}));
    await expectLater(http.runWithClient(() => api.changePassword('a', 'b'), () => bad), throwsA(isA<ApiException>()));
  });
}
