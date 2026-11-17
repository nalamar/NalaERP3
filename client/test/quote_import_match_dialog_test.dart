import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nalaerp_client/api.dart';
import 'package:nalaerp_client/pages/quote_import_match_dialog.dart';

// Pruefliste der Zuordnung (Backlog I.5.5a). Erfundene Positionen.

Map<String, dynamic> _zeile(String id,
        {String entscheidung = 'offen',
        String code = 'L-1',
        bool ueber = true,
        String? leistungId = 'l-1',
        double score = 0.9}) =>
    {
      'item_id': id,
      'position_no': '01.$id',
      'kurztext': 'Fenster $id',
      'menge': 2,
      'einheit': 'St',
      'leistung_id': code.isEmpty ? null : leistungId,
      'code': code,
      'score': code.isEmpty ? 0 : score,
      'begruendung': code.isEmpty ? '' : 'passt zu $id',
      'systemvorgaben': code.isEmpty ? {} : {'profilserie': 'S-70'},
      'quelle': entscheidung.isEmpty ? '' : 'wortabgleich',
      'profil_name': '',
      'schwelle': 0.7,
      'ueber_schwelle': code.isNotEmpty && ueber,
      'entscheidung': entscheidung,
      'entschieden_leistung_id': null,
      'entschieden_code': '',
    };

class _FakeApi extends ApiClient {
  _FakeApi(this.zeilen, {this.write = true, this.error})
      : super(baseUrl: 'http://localhost:8080');

  List<Map<String, dynamic>> zeilen;
  final bool write;
  final ApiException? error;
  final List<(String, String)> decided = [];
  int confirmed = 0;

  @override
  bool hasPermission(String permission) =>
      permission == 'quotes.read' || (write && permission == 'quotes.write');

  @override
  Future<List<dynamic>> listQuoteImportMatches(String importId) async =>
      [for (final z in zeilen) Map<String, dynamic>.from(z)];

  @override
  Future<Map<String, dynamic>> decideQuoteImportMatch(
      String importId, String itemId, String aktion,
      {String leistungId = ''}) async {
    decided.add((itemId, aktion));
    if (error != null) throw error!;
    final z = zeilen.firstWhere((z) => z['item_id'] == itemId);
    z['entscheidung'] = aktion == 'bestaetigen' ? 'bestaetigt' : 'verworfen';
    return Map<String, dynamic>.from(z);
  }

  @override
  Future<int> confirmQuoteImportMatches(String importId) async {
    confirmed++;
    var n = 0;
    for (final z in zeilen) {
      if (z['entscheidung'] == 'offen' && z['ueber_schwelle'] == true) {
        z['entscheidung'] = 'bestaetigt';
        n++;
      }
    }
    return n;
  }
}

Future<void> _pump(WidgetTester tester, _FakeApi api) async {
  tester.view.physicalSize = const Size(1200, 3000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(MaterialApp(
    home: Builder(
      builder: (context) => TextButton(
        onPressed: () => showQuoteImportMatchDialog(context, api, 'imp-1'),
        child: const Text('öffnen'),
      ),
    ),
  ));
  await tester.tap(find.text('öffnen'));
  await tester.pumpAndSettle();
}

void main() {
  group('sortiereMatchPruefliste', () {
    test('ohne Vorschlag, unter, über der Schwelle, entschieden, ohne Lauf',
        () {
      final sorted = sortiereMatchPruefliste([
        _zeile('a', entscheidung: ''),
        _zeile('b', entscheidung: 'bestaetigt'),
        _zeile('c'),
        _zeile('d', ueber: false),
        _zeile('e', code: ''),
        _zeile('f', ueber: false),
        'kein Eintrag',
      ]);
      expect(sorted.map((z) => z['item_id']), ['e', 'd', 'f', 'c', 'b', 'a']);
    });

    // List.sort ist erst ab 32 Eintraegen nicht mehr stabil.
    test('innerhalb des Rangs LV-Reihenfolge, auch bei langen Listen', () {
      final ids = [for (var i = 0; i < 40; i++) i.toString().padLeft(2, '0')];
      final sorted = sortiereMatchPruefliste(
          [for (final id in ids) _zeile(id, ueber: int.parse(id).isEven)]);
      expect(sorted.map((z) => z['item_id']), [
        ...ids.where((id) => int.parse(id).isOdd),
        ...ids.where((id) => int.parse(id).isEven),
      ]);
    });

    test('Rang je Zustand', () {
      expect(matchPruefRang(_zeile('x', code: '')), 0);
      expect(matchPruefRang(_zeile('x', ueber: false)), 1);
      expect(matchPruefRang(_zeile('x')), 2);
      expect(matchPruefRang(_zeile('x', entscheidung: 'verworfen')), 3);
      expect(matchPruefRang(_zeile('x', entscheidung: 'geaendert')), 3);
      expect(matchPruefRang(_zeile('x', entscheidung: '')), 4);
    });
  });

  test('Texte und Anzahl der Sammelbestätigung', () {
    expect(matchEntscheidungLabel('geaendert'), 'geändert');
    expect(matchEntscheidungLabel('bestaetigt'), 'bestätigt');
    expect(matchEntscheidungLabel(''), 'noch nicht zugeordnet');
    expect(matchScoreText(0.725), '73 %');
    expect(matchScoreText(null), '0 %');
    expect(
        matchSammelAnzahl([
          _zeile('a'),
          _zeile('b', ueber: false),
          _zeile('c', leistungId: null),
          _zeile('d', entscheidung: 'verworfen'),
          _zeile('e'),
        ]),
        2);
  });

  testWidgets('zeigt die Prüfliste sortiert mit Vorschlag und Begründung',
      (tester) async {
    await _pump(tester, _FakeApi([_zeile('a'), _zeile('b', code: '')]));
    expect(find.text('2 Positionen, 2 offen'), findsOneWidget);
    expect(find.text('Vorschlag: kein Vorschlag'), findsOneWidget);
    expect(find.text('Vorschlag: L-1 · 90 % (Schwelle 70 %) · Wortabgleich'),
        findsOneWidget);
    expect(find.text('Systemvorgaben: profilserie: S-70'), findsOneWidget);
    expect(find.text('Begründung: passt zu a'), findsOneWidget);
    // ohne Vorschlag zuerst
    expect(tester.getTopLeft(find.byKey(const ValueKey('match-b'))).dy,
        lessThan(tester.getTopLeft(find.byKey(const ValueKey('match-a'))).dy));
    // ohne Leistung kein Bestätigen, Verwerfen geht
    expect(find.byKey(const ValueKey('bestaetigen-b')), findsNothing);
    expect(find.byKey(const ValueKey('verwerfen-b')), findsOneWidget);
  });

  testWidgets('bestätigt und verwirft je Position', (tester) async {
    final api = _FakeApi([_zeile('a'), _zeile('b', ueber: false)]);
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('bestaetigen-a')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('verwerfen-b')));
    await tester.pumpAndSettle();
    expect(api.decided, [('a', 'bestaetigen'), ('b', 'verwerfen')]);
    expect(find.text('Entscheidung: bestätigt'), findsOneWidget);
    expect(find.text('Entscheidung: verworfen'), findsOneWidget);
    expect(find.byKey(const ValueKey('bestaetigen-a')), findsNothing);
    expect(find.byKey(const ValueKey('verwerfen-b')), findsNothing);
  });

  testWidgets('Sammelbestätigung erfasst nur die über der Schwelle',
      (tester) async {
    final api = _FakeApi([_zeile('a'), _zeile('b', ueber: false), _zeile('c')]);
    await _pump(tester, api);
    await tester.tap(find.text('Alle über der Schwelle bestätigen (2)'));
    await tester.pumpAndSettle();
    expect(api.confirmed, 1);
    expect(find.text('2 Vorschläge bestätigt'), findsOneWidget);
    expect(find.text('3 Positionen, 1 offen'), findsOneWidget);
    expect(find.byKey(const ValueKey('sammel-bestaetigen')), findsNothing);
  });

  testWidgets('Fehler des Servers bleibt sichtbar, Zeile unverändert',
      (tester) async {
    final api = _FakeApi([_zeile('a')],
        error: const ApiException(
            statusCode: 409,
            message: 'Entscheiden im aktuellen Status nicht erlaubt'));
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('verwerfen-a')));
    await tester.pumpAndSettle();
    expect(find.text('Entscheiden im aktuellen Status nicht erlaubt'),
        findsOneWidget);
    expect(find.text('Entscheidung: offen'), findsOneWidget);
  });

  testWidgets('ohne quotes.write keine Knöpfe, ohne Lauf keine Knöpfe',
      (tester) async {
    await _pump(tester, _FakeApi([_zeile('a')], write: false));
    expect(find.byKey(const ValueKey('verwerfen-a')), findsNothing);
    expect(find.byKey(const ValueKey('sammel-bestaetigen')), findsNothing);
    await tester.tap(find.text('Schließen'));
    await tester.pumpAndSettle();
    await _pump(tester, _FakeApi([_zeile('b', entscheidung: '')]));
    expect(find.text('Entscheidung: noch nicht zugeordnet'), findsOneWidget);
    expect(find.byKey(const ValueKey('verwerfen-b')), findsNothing);
  });
}
