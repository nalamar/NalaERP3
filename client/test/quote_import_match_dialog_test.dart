import 'dart:async';

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
  _FakeApi(this.zeilen,
      {this.write = true,
      this.settings = false,
      this.error,
      this.startError,
      List<Map<String, dynamic>>? status})
      : status = status ?? [_fertig],
        super(baseUrl: 'http://localhost:8080');

  List<Map<String, dynamic>> zeilen;
  final bool write;
  final bool settings;
  final ApiException? error;
  final ApiException? startError;
  // Antworten der Statusabfrage nacheinander; die letzte bleibt.
  final List<Map<String, dynamic>> status;
  final List<(String, String)> decided = [];
  final List<String> leistungIds = [];
  final List<(String, String)> gruende = [];
  final List<(String, bool)> started = [];
  final List<String> suchen = [];
  int confirmed = 0;
  int listCalls = 0;

  @override
  bool hasPermission(String permission) =>
      permission == 'quotes.read' ||
      (write && permission == 'quotes.write') ||
      (settings && permission == 'settings.manage');

  // Ist halt gesetzt, wartet die Statusabfrage (langsamer Server).
  Completer<void>? halt;
  int statusCalls = 0;

  @override
  Future<Map<String, dynamic>> getQuoteImportMatchStatus(
      String importId) async {
    statusCalls++;
    if (halt != null) await halt!.future;
    return status.length > 1 ? status.removeAt(0) : status.first;
  }

  @override
  Future<Map<String, dynamic>> startQuoteImportMatch(String importId,
      {String profilId = '', bool wortabgleich = false}) async {
    started.add((profilId, wortabgleich));
    if (startError != null) throw startError!;
    return _laeuft;
  }

  @override
  Future<List<dynamic>> listLlmProfiles() async => [
        {'id': 'p-1', 'name': 'Lokal', 'aktiv': true},
        {'id': 'p-2', 'name': 'Aus', 'aktiv': false},
      ];

  @override
  Future<List<dynamic>> listLeistungen(
      {String q = '', bool nurAktive = false}) async {
    suchen.add('$q|$nurAktive');
    return [
      {'id': 'l-2', 'code': 'L-2', 'kurztext': 'Tür', 'einheit': 'St'},
      if (q.isEmpty)
        {'id': 'l-3', 'code': 'L-3', 'kurztext': 'Tor', 'einheit': 'St'},
    ];
  }

  @override
  Future<List<dynamic>> listQuoteImportMatches(String importId) async {
    listCalls++;
    return [for (final z in zeilen) Map<String, dynamic>.from(z)];
  }

  @override
  Future<Map<String, dynamic>> decideQuoteImportMatch(
      String importId, String itemId, String aktion,
      {String leistungId = '',
      String grund = '',
      String grundText = ''}) async {
    decided.add((itemId, aktion));
    leistungIds.add(leistungId);
    gruende.add((grund, grundText));
    if (error != null) throw error!;
    final z = zeilen.firstWhere((z) => z['item_id'] == itemId);
    z['entscheidung'] = switch (aktion) {
      'bestaetigen' => 'bestaetigt',
      'aendern' => 'geaendert',
      _ => 'verworfen',
    };
    z['entschieden_code'] = aktion == 'aendern' ? 'L-2' : '';
    z['entschieden_leistung_id'] = aktion == 'aendern' ? leistungId : null;
    z['grund'] = grund;
    z['grund_text'] = grundText;
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

const _fertig = {
  'status': 'fertig',
  'quelle': 'wortabgleich',
  'fortschritt': 2,
  'gesamt': 2
};
const _laeuft = {
  'status': 'laeuft',
  'quelle': 'modell',
  'profil_name': 'Lokal',
  'fortschritt': 1,
  'gesamt': 2
};

Future<void> _pump(WidgetTester tester, _FakeApi api,
    {bool bearbeitbar = true}) async {
  tester.view.physicalSize = const Size(1200, 3000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(MaterialApp(
    home: Builder(
      builder: (context) => TextButton(
        onPressed: () => showQuoteImportMatchDialog(context, api, 'imp-1',
            bearbeitbar: bearbeitbar),
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

  test('Text zum Stand des Laufs', () {
    expect(matchLaufText(const {}), 'Noch keine Zuordnung gelaufen');
    expect(
        matchLaufText(const {'status': ''}), 'Noch keine Zuordnung gelaufen');
    expect(matchLaufText(_laeuft),
        'Zuordnung läuft (Modell Lokal): 1 von 2 Positionen');
    expect(matchLaufText(_fertig), 'Zuordnung fertig (Wortabgleich)');
    expect(matchLaufText(const {'status': 'fehler', 'fehler': 'Zeitlimit'}),
        'Zuordnung mit Fehler beendet: Zeitlimit');
    expect(matchLaufText(const {'status': 'abgebrochen'}),
        'Zuordnung abgebrochen (Serverneustart) - bitte neu starten');
  });

  testWidgets('startet den Wortabgleich und fragt den Status bis zum Ende ab',
      (tester) async {
    final api = _FakeApi([
      _zeile('a')
    ], status: [
      {'status': ''},
      _laeuft,
      _fertig,
    ]);
    await _pump(tester, api);
    expect(find.text('Noch keine Zuordnung gelaufen'), findsOneWidget);
    expect(find.text('Zuordnung starten'), findsOneWidget);
    // ohne settings.manage keine Profile in der Auswahl
    await tester.tap(find.byKey(const ValueKey('lauf-auswahl')));
    await tester.pumpAndSettle();
    expect(find.text('Profil Lokal'), findsNothing);
    await tester.tap(find.text('Wortabgleich (ohne Sprachmodell)').last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('lauf-starten')));
    await tester.pumpAndSettle();
    expect(api.started, [('', true)]);
    expect(find.text('Zuordnung läuft (Modell Lokal): 1 von 2 Positionen'),
        findsOneWidget);
    // waehrend des Laufs weder Start noch Entscheidungen
    expect(find.byKey(const ValueKey('lauf-starten')), findsNothing);
    expect(find.byKey(const ValueKey('verwerfen-a')), findsNothing);
    expect(find.byKey(const ValueKey('sammel-bestaetigen')), findsNothing);
    final vorher = api.listCalls;
    await tester.pump(matchPollIntervall);
    await tester.pumpAndSettle();
    expect(find.text('Zuordnung läuft (Modell Lokal): 1 von 2 Positionen'),
        findsOneWidget);
    expect(api.listCalls, vorher);
    await tester.pump(matchPollIntervall);
    await tester.pumpAndSettle();
    expect(find.text('Zuordnung fertig (Wortabgleich)'), findsOneWidget);
    expect(api.listCalls, vorher + 1);
    expect(find.text('Zuordnung neu starten'), findsOneWidget);
    expect(find.byKey(const ValueKey('verwerfen-a')), findsOneWidget);
    // nach dem Ende keine weitere Abfrage
    await tester.pump(matchPollIntervall * 3);
    expect(api.listCalls, vorher + 1);
  });

  testWidgets('startet mit Profil nur mit settings.manage, nur aktive Profile',
      (tester) async {
    final api = _FakeApi([_zeile('a')], settings: true);
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('lauf-auswahl')));
    await tester.pumpAndSettle();
    expect(find.text('Profil Aus'), findsNothing);
    await tester.tap(find.text('Profil Lokal').last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('lauf-starten')));
    await tester.pumpAndSettle();
    expect(api.started, [('p-1', false)]);
    // Dialog schliessen beendet die Abfrage (kein Timer bleibt haengen)
    await tester.tap(find.text('Schließen'));
    await tester.pumpAndSettle();
    await tester.pump(matchPollIntervall * 2);
  });

  testWidgets('Standardprofil ohne Auswahl, Fehler beim Start sichtbar',
      (tester) async {
    final api = _FakeApi([_zeile('a')],
        startError: const ApiException(
            statusCode: 409, message: 'Sprachmodell ist nicht eingeschaltet'));
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('lauf-starten')));
    await tester.pumpAndSettle();
    expect(api.started, [('', false)]);
    expect(find.text('Sprachmodell ist nicht eingeschaltet'), findsOneWidget);
    expect(find.text('Zuordnung fertig (Wortabgleich)'), findsOneWidget);
  });

  testWidgets('beim Öffnen laufender Lauf wird weiter abgefragt',
      (tester) async {
    final api = _FakeApi([_zeile('a')], status: [_laeuft, _fertig]);
    await _pump(tester, api);
    expect(find.text('Zuordnung läuft (Modell Lokal): 1 von 2 Positionen'),
        findsOneWidget);
    await tester.pump(matchPollIntervall);
    await tester.pumpAndSettle();
    expect(find.text('Zuordnung fertig (Wortabgleich)'), findsOneWidget);
  });

  testWidgets('langsamer Server: keine überlappenden Statusabfragen',
      (tester) async {
    final api = _FakeApi([_zeile('a')], status: [_laeuft, _laeuft, _fertig]);
    await _pump(tester, api);
    expect(api.statusCalls, 1);
    api.halt = Completer<void>();
    await tester.pump(matchPollIntervall);
    await tester.pump(matchPollIntervall);
    await tester.pump(matchPollIntervall);
    expect(api.statusCalls, 2);
    api.halt!.complete();
    api.halt = null;
    await tester.pumpAndSettle();
    await tester.pump(matchPollIntervall);
    await tester.pumpAndSettle();
    expect(api.statusCalls, 3);
    expect(find.text('Zuordnung fertig (Wortabgleich)'), findsOneWidget);
  });

  testWidgets('ändert über die Leistungsauswahl mit Suche', (tester) async {
    final api = _FakeApi([_zeile('a')]);
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('aendern-a')));
    await tester.pumpAndSettle();
    expect(find.text('Leistung wählen'), findsOneWidget);
    expect(find.text('L-3 Tor'), findsOneWidget);
    await tester.enterText(find.byKey(const ValueKey('leistung-suche')), 'Tür');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
    expect(api.suchen, ['|true', 'Tür|true']);
    expect(find.text('L-3 Tor'), findsNothing);
    await tester.tap(find.byKey(const ValueKey('leistung-l-2')));
    await tester.pumpAndSettle();
    expect(api.decided, [('a', 'aendern')]);
    expect(api.leistungIds, ['l-2']);
    expect(find.text('Entscheidung: geändert (L-2)'), findsOneWidget);
  });

  testWidgets('Abbrechen der Leistungsauswahl entscheidet nichts',
      (tester) async {
    final api = _FakeApi([_zeile('a')]);
    await _pump(tester, api);
    await tester.tap(find.byKey(const ValueKey('aendern-a')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(api.decided, isEmpty);
    expect(find.text('Entscheidung: offen'), findsOneWidget);
  });

  testWidgets('nach der Freigabe nur lesen', (tester) async {
    await _pump(tester, _FakeApi([_zeile('a')]), bearbeitbar: false);
    expect(find.text('Zuordnung fertig (Wortabgleich)'), findsOneWidget);
    expect(find.byKey(const ValueKey('lauf-starten')), findsNothing);
    expect(find.byKey(const ValueKey('aendern-a')), findsNothing);
    expect(find.byKey(const ValueKey('sammel-bestaetigen')), findsNothing);
    expect(find.byKey(const ValueKey('grund-a')), findsNothing);
  });

  // Gruende (Backlog I.8.3c1)
  group('Gründe', () {
    test('Auswahl wie der Server, Text der Zeile', () {
      expect(matchGruende.keys, [
        'falsche_leistung',
        'systemvorgabe_falsch',
        'einheit_menge',
        'keine_passende_leistung',
        'sonstiges',
      ]);
      expect(matchGrundText({'grund': ''}), '');
      expect(matchGrundText({'grund_text': 'ohne Grund'}), '');
      expect(matchGrundText({'grund': 'einheit_menge', 'grund_text': ' '}),
          'Einheit oder Menge');
      expect(matchGrundText({'grund': 'sonstiges', 'grund_text': ' alt '}),
          'Sonstiges: alt');
      expect(matchGrundText({'grund': 'neu_x'}), 'neu_x');
    });

    testWidgets('Verwerfen bleibt ein Klick, Grund danach freiwillig',
        (tester) async {
      final api = _FakeApi([_zeile('a')]);
      await _pump(tester, api);
      expect(find.byKey(const ValueKey('grund-a')), findsNothing);
      await tester.tap(find.byKey(const ValueKey('verwerfen-a')));
      await tester.pumpAndSettle();
      expect(api.decided, [('a', 'verwerfen')]);
      expect(api.gruende, [('', '')]);
      expect(find.text('Grund angeben'), findsOneWidget);
      expect(find.byKey(const ValueKey('grund-text-a')), findsNothing);

      await tester.tap(find.byKey(const ValueKey('grund-a')));
      await tester.pumpAndSettle();
      // Freitext erst mit Grund
      expect(
          tester
              .widget<TextField>(find.byKey(const ValueKey('grund-freitext')))
              .enabled,
          isFalse);
      await tester.tap(find.byKey(const ValueKey('grund-auswahl')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Keine passende Leistung im Katalog').last);
      await tester.pumpAndSettle();
      await tester.enterText(
          find.byKey(const ValueKey('grund-freitext')), '  nicht im Katalog ');
      await tester.tap(find.byKey(const ValueKey('grund-speichern')));
      await tester.pumpAndSettle();
      expect(api.decided, [('a', 'verwerfen'), ('a', 'verwerfen')]);
      expect(api.leistungIds, ['', '']);
      expect(api.gruende.last, ('keine_passende_leistung', 'nicht im Katalog'));
      expect(
          find.text('Grund: Keine passende Leistung im Katalog: '
              'nicht im Katalog'),
          findsOneWidget);
      expect(find.text('Grund ändern'), findsOneWidget);
    });

    testWidgets('Grund zur Änderung behält die gewählte Leistung',
        (tester) async {
      final api = _FakeApi([_zeile('a')]);
      await _pump(tester, api);
      await tester.tap(find.byKey(const ValueKey('aendern-a')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('leistung-l-3')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('grund-a')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('grund-auswahl')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Systemvorgabe falsch').last);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('grund-speichern')));
      await tester.pumpAndSettle();
      expect(api.decided, [('a', 'aendern'), ('a', 'aendern')]);
      expect(api.leistungIds, ['l-3', 'l-3']);
      expect(api.gruende.last, ('systemvorgabe_falsch', ''));
      expect(find.text('Grund: Systemvorgabe falsch'), findsOneWidget);
    });

    testWidgets('ohne Grund verwirft den Freitext, Abbrechen speichert nichts',
        (tester) async {
      final z = _zeile('a', entscheidung: 'verworfen')
        ..['grund'] = 'sonstiges'
        ..['grund_text'] = 'alt';
      final api = _FakeApi([z, _zeile('b', entscheidung: 'bestaetigt')]);
      await _pump(tester, api);
      expect(find.text('Grund: Sonstiges: alt'), findsOneWidget);
      // nur bei geaendert und verworfen
      expect(find.byKey(const ValueKey('grund-b')), findsNothing);

      await tester.tap(find.byKey(const ValueKey('grund-a')));
      await tester.pumpAndSettle();
      expect(find.text('alt'), findsOneWidget);
      await tester.tap(find.text('Abbrechen'));
      await tester.pumpAndSettle();
      expect(api.decided, isEmpty);

      // Speichern ohne Änderung behält Grund und Freitext
      await tester.tap(find.byKey(const ValueKey('grund-a')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('grund-speichern')));
      await tester.pumpAndSettle();
      expect(api.gruende, [('sonstiges', 'alt')]);

      await tester.tap(find.byKey(const ValueKey('grund-a')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('grund-auswahl')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('ohne Grund').last);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('grund-speichern')));
      await tester.pumpAndSettle();
      expect(api.gruende.last, ('', ''));
      expect(find.byKey(const ValueKey('grund-text-a')), findsNothing);
      expect(find.text('Grund angeben'), findsOneWidget);
    });
  });
}
