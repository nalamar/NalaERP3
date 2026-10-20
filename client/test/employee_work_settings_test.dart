import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/employee_work_settings.dart';
import 'package:nalaerp_client/pages/employees_page.dart';
import 'package:nalaerp_client/pages/teams_page.dart';

// Arbeitstage und Urlaub am Mitarbeiter (Backlog F.8.6.4.2).

class _FakeApi extends ApiClient {
  _FakeApi({this.perms = const {'employees.read', 'employees.write'}, this.patchError}) : super(baseUrl: 'http://localhost:8080');

  final Set<String> perms;
  final Object? patchError;
  final List<Map<String, dynamic>> patches = [];
  final Map<String, dynamic> emp = {
    'id': 'e1',
    'first_name': 'Anna',
    'last_name': 'Berg',
    'active': true,
    'arbeitstage': [1, 2, 3, 4, 5],
    'bundesland': null,
    'urlaubstage_jahr': null,
  };

  @override
  bool hasPermission(String code) => perms.contains(code);

  @override
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) async => [Map<String, dynamic>.from(emp)];

  @override
  Future<List<dynamic>> listTeams() async => [];

  @override
  Future<List<dynamic>> listLoginUsers() async => [];

  @override
  Future<Map<String, dynamic>> getEmployee(String id) async => Map<String, dynamic>.from(emp);

  @override
  Future<Map<String, dynamic>> updateEmployee(String id, Map<String, dynamic> patch) async {
    if (patchError != null) throw patchError!;
    patches.add(patch);
    emp.addAll(patch);
    return Map<String, dynamic>.from(emp);
  }
}

Future<void> _open(WidgetTester tester, _FakeApi api) async {
  await tester.binding.setSurfaceSize(const Size(1400, 1000));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(theme: ThemeData(splashFactory: InkRipple.splashFactory), home: EmployeesPage(api: api)));
  await tester.pumpAndSettle();
  await tester.tap(find.text('Anna Berg'));
  await tester.pumpAndSettle();
}

Finder _inDialog(Finder f) => find.descendant(of: find.byType(AlertDialog), matching: f);

void main() {
  test('Anzeigen und Patch', () {
    expect(workdaysLabel([5, 1, 3]), 'Mo, Mi, Fr');
    expect(defaultLeaveDays(3), 12);
    expect(leaveEntitlementLabel({'arbeitstage': [1, 2, 3]}), '12 Tage (Vorgabe)');
    expect(leaveEntitlementLabel({}), '20 Tage (Vorgabe)');
    expect(leaveEntitlementLabel({'urlaubstage_jahr': 28.5}), '28,5 Tage');
    expect(employeeBundeslandLabel({'bundesland': null}), 'wie Firma');
    expect(employeeBundeslandLabel({'bundesland': 'SN'}), 'Sachsen');
    expect(buildWorkSettingsPatch(bundesland: '', days: {3, 1, 2}, urlaub: '').body,
        {'bundesland': null, 'arbeitstage': [1, 2, 3], 'urlaubstage_jahr': null});
    expect(buildWorkSettingsPatch(bundesland: 'NW', days: {1, 2, 3, 4, 5}, urlaub: '28,5').body,
        {'bundesland': 'NW', 'arbeitstage': [1, 2, 3, 4, 5], 'urlaubstage_jahr': 28.5});
    expect(buildWorkSettingsPatch(bundesland: '', days: {}, urlaub: '').error, 'Bitte mindestens einen Arbeitstag wählen.');
    expect(buildWorkSettingsPatch(bundesland: '', days: {1, 2, 3}, urlaub: '11,5').error,
        'Mindestens 12 Tage bei 3 Arbeitstagen je Woche (gesetzlicher Mindesturlaub).');
    expect(buildWorkSettingsPatch(bundesland: '', days: {1, 2, 3}, urlaub: '12').body!['urlaubstage_jahr'], 12.0);
    for (final bad in ['x', '20,3', '367']) {
      expect(buildWorkSettingsPatch(bundesland: '', days: {1, 2, 3, 4, 5}, urlaub: bad).error,
          'Bitte den Jahresurlaub in ganzen oder halben Tagen angeben oder leer lassen.',
          reason: bad);
    }
  });

  testWidgets('Detail zeigt die Angaben; Dialog speichert Teilzeit und Bundesland', (tester) async {
    final api = _FakeApi();
    await _open(tester, api);
    expect(find.text('Bundesland: wie Firma'), findsOneWidget);
    expect(find.text('Arbeitstage: Mo, Di, Mi, Do, Fr'), findsOneWidget);
    expect(find.text('Jahresurlaub: 20 Tage (Vorgabe)'), findsOneWidget);
    await tester.tap(find.text('Arbeitstage und Urlaub'));
    await tester.pumpAndSettle();
    // Do und Fr abwaehlen -> Vorgabe 12; 11 ist zu wenig.
    await tester.tap(_inDialog(find.text('Do')));
    await tester.tap(_inDialog(find.text('Fr')));
    await tester.pumpAndSettle();
    expect(_inDialog(find.text('Jahresurlaub in Tagen (leer = Vorgabe 12)')), findsOneWidget);
    await tester.enterText(_inDialog(find.byType(TextField)), '11');
    await tester.tap(_inDialog(find.text('Speichern')));
    await tester.pumpAndSettle();
    expect(find.text('Mindestens 12 Tage bei 3 Arbeitstagen je Woche (gesetzlicher Mindesturlaub).'), findsOneWidget);
    expect(api.patches, isEmpty);
    await tester.enterText(_inDialog(find.byType(TextField)), '15');
    await tester.tap(_inDialog(find.byType(DropdownButtonFormField<String>)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Sachsen').last);
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Speichern')));
    await tester.pumpAndSettle();
    expect(api.patches, [
      {'bundesland': 'SN', 'arbeitstage': [1, 2, 3], 'urlaubstage_jahr': 15.0}
    ]);
    expect(find.text('Arbeitstage und Urlaub gespeichert'), findsOneWidget);
    expect(find.text('Bundesland: Sachsen'), findsOneWidget);
    expect(find.text('Arbeitstage: Mo, Di, Mi'), findsOneWidget);
    expect(find.text('Jahresurlaub: 15 Tage'), findsOneWidget);
  });

  testWidgets('Dialog: Fehler des Servers bleibt im Dialog; Abbrechen sendet nichts', (tester) async {
    final api = _FakeApi(patchError: const ApiException(statusCode: 400, code: 'validation_error', message: 'Ungültiges Bundesland'));
    await _open(tester, api);
    await tester.tap(find.text('Arbeitstage und Urlaub'));
    await tester.pumpAndSettle();
    await tester.tap(_inDialog(find.text('Speichern')));
    await tester.pumpAndSettle();
    expect(_inDialog(find.text('Ungültiges Bundesland')), findsOneWidget);
    await tester.tap(_inDialog(find.text('Abbrechen')));
    await tester.pumpAndSettle();
    expect(find.byType(AlertDialog), findsNothing);
    expect(find.text('Arbeitstage und Urlaub gespeichert'), findsNothing);
  });

  testWidgets('Ohne Schreibrecht kein Dialog-Button; Teams erreichbar', (tester) async {
    final api = _FakeApi(perms: const {'employees.read'});
    await _open(tester, api);
    expect(find.text('Jahresurlaub: 20 Tage (Vorgabe)'), findsOneWidget);
    expect(find.text('Arbeitstage und Urlaub'), findsNothing);
    await tester.tap(find.text('Teams'));
    await tester.pumpAndSettle();
    expect(find.byType(TeamsPage), findsOneWidget);
  });
}
