import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/commercial_navigation.dart';
import 'package:nalaerp_client/pages/materials_page.dart';

// Kennzeichen "Nachweis EN 1090" am Material im Client (Backlog H.4.4b):
// setzen nur mit materials.write, anzeigen fuer alle mit materials.read.

const _label = 'Nachweis EN 1090 erforderlich';

class _FakeApi extends ApiClient {
  _FakeApi({required this.permissions, required this.materials})
      : super(baseUrl: 'http://localhost:8080');

  final Set<String> permissions;
  final List<Map<String, dynamic>> materials;
  final List<Map<String, dynamic>> created = [];
  final List<Map<String, dynamic>> patches = [];

  @override
  bool hasPermission(String permission) => permissions.contains(permission);

  @override
  Future<List<dynamic>> listMaterials(
          {String? q,
          String? typ,
          String? kategorie,
          int? limit,
          int? offset}) async =>
      (offset ?? 0) == 0 ? materials : [];

  @override
  Future<Map<String, dynamic>> getMaterial(String id) async =>
      materials.firstWhere((m) => m['id'] == id);

  @override
  Future<Map<String, dynamic>> createMaterial(Map<String, dynamic> body) async {
    created.add(body);
    return {'id': 'neu', ...body};
  }

  @override
  Future<Map<String, dynamic>> updateMaterial(
      String id, Map<String, dynamic> patch) async {
    patches.add(patch);
    return getMaterial(id);
  }

  @override
  Future<List<String>> listMaterialTypes() async => const [];

  @override
  Future<List<String>> listMaterialCategories() async => const [];

  @override
  Future<List<Map<String, dynamic>>> listUnits() async => [
        {'code': 'Stk'}
      ];

  @override
  Future<List<dynamic>> stockByMaterial(String id) async => const [];

  @override
  Future<List<dynamic>> listWarehouses() async => const [];

  @override
  Future<List<dynamic>> listMaterialDocuments(String materialId) async =>
      const [];
}

Map<String, dynamic> _material(String id, bool nachweis) => {
      'id': id,
      'nummer': 'MAT-$id',
      'bezeichnung': 'Material $id',
      'typ': 'profil',
      'einheit': 'Stk',
      'kategorie': '',
      'nachweis_en1090': nachweis,
    };

Future<void> _pump(WidgetTester tester, _FakeApi api, {String? detail}) async {
  await tester.binding.setSurfaceSize(const Size(2200, 2800));
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(MaterialApp(
    home: MaterialsPage(
      api: api,
      initialContext:
          detail == null ? null : MaterialListContext.detail(detail),
    ),
  ));
  await tester.pumpAndSettle();
}

Finder _inDialog(Finder f) =>
    find.descendant(of: find.byType(AlertDialog), matching: f);

Future<void> _fillCreateDialog(WidgetTester tester) async {
  await tester.enterText(
      _inDialog(find.widgetWithText(TextFormField, 'Nummer')), 'MAT-NEU');
  await tester.enterText(
      _inDialog(find.widgetWithText(TextFormField, 'Bezeichnung')), 'Träger');
  await tester.tap(_inDialog(
      find.widgetWithText(DropdownButtonFormField<String>, 'Einheit')));
  await tester.pumpAndSettle();
  await tester.tap(find.text('Stk').last);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('Anlegen ohne Haken sendet kein Kennzeichen', (tester) async {
    final api = _FakeApi(
        permissions: {'materials.read', 'materials.write'}, materials: []);
    await _pump(tester, api);
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();

    final box = _inDialog(find.widgetWithText(CheckboxListTile, _label));
    expect(box, findsOneWidget);
    expect(tester.widget<CheckboxListTile>(box).value, isFalse);
    await _fillCreateDialog(tester);
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();

    expect(api.created, hasLength(1));
    expect(api.created.single['nachweis_en1090'], isFalse);
  });

  testWidgets('Anlegen mit Haken sendet das Kennzeichen', (tester) async {
    final api = _FakeApi(
        permissions: {'materials.read', 'materials.write'}, materials: []);
    await _pump(tester, api);
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();

    await _fillCreateDialog(tester);
    await tester.tap(_inDialog(find.text(_label)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anlegen'));
    await tester.pumpAndSettle();

    expect(api.created.single['nachweis_en1090'], isTrue);

    // Naechstes Anlegen beginnt wieder ohne Haken.
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();
    final box = _inDialog(find.widgetWithText(CheckboxListTile, _label));
    expect(tester.widget<CheckboxListTile>(box).value, isFalse);
  });

  testWidgets('Bearbeiten zeigt und aendert das Kennzeichen', (tester) async {
    final api = _FakeApi(
        permissions: {'materials.read', 'materials.write'},
        materials: [_material('a', true)]);
    await _pump(tester, api, detail: 'a');
    expect(find.text(_label), findsOneWidget);

    await tester.tap(find.byIcon(Icons.edit));
    await tester.pumpAndSettle();
    final box = _inDialog(find.widgetWithText(CheckboxListTile, _label));
    expect(tester.widget<CheckboxListTile>(box).value, isTrue);
    await tester.tap(box);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.patches.single['nachweis_en1090'], isFalse);
  });

  testWidgets('Bearbeiten ohne Aenderung behaelt das Kennzeichen',
      (tester) async {
    final api = _FakeApi(
        permissions: {'materials.read', 'materials.write'},
        materials: [_material('a', true)]);
    await _pump(tester, api, detail: 'a');
    await tester.tap(find.byIcon(Icons.edit));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(api.patches.single['nachweis_en1090'], isTrue);
  });

  testWidgets('Ohne materials.write: Anzeige ja, Setzen nein', (tester) async {
    final api = _FakeApi(
        permissions: {'materials.read'},
        materials: [_material('a', true), _material('b', false)]);
    await _pump(tester, api, detail: 'a');

    expect(find.text(_label), findsOneWidget);
    expect(find.text('MAT-a  •  profil  •  Stk  •  EN 1090'), findsOneWidget);
    expect(find.text('MAT-b  •  profil  •  Stk'), findsOneWidget);
    expect(find.byType(FloatingActionButton), findsNothing);
    expect(find.byIcon(Icons.edit), findsNothing);
    expect(find.byType(CheckboxListTile), findsNothing);

    // Material ohne Kennzeichen: kein Hinweis im Detail.
    await tester.tap(find.text('Material b'));
    await tester.pumpAndSettle();
    expect(find.text(_label), findsNothing);
  });
}
