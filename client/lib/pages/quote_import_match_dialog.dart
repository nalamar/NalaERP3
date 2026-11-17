import 'dart:async';

import 'package:flutter/material.dart';
import '../api.dart';

// Pruefliste der Zuordnung am GAEB-Import (ADR 0045, Backlog I.5.5a/b):
// Lauf starten und verfolgen, Vorschlaege lesen, je Position bestaetigen,
// aendern oder verwerfen, alle ueber der Schwelle auf einmal bestaetigen.

/// Abstand der Statusabfrage, solange ein Lauf im Hintergrund laeuft.
const matchPollIntervall = Duration(seconds: 2);

/// Auswahl beim Start fuer den Wortabgleich; sonst Profil-ID, leer = Standard.
const matchAuswahlWortabgleich = '#wortabgleich';

/// Reihenfolge der Pruefung: offene ohne Vorschlag, offene unter der
/// Schwelle, offene darueber, entschiedene, zuletzt Positionen ohne Lauf.
int matchPruefRang(Map<String, dynamic> z) {
  switch ((z['entscheidung'] ?? '').toString()) {
    case 'offen':
      if ((z['code'] ?? '').toString().isEmpty) return 0;
      return z['ueber_schwelle'] == true ? 2 : 1;
    case '':
      return 4;
    default:
      return 3;
  }
}

/// Sortiert stabil nach [matchPruefRang]; innerhalb gleich LV-Reihenfolge.
List<Map<String, dynamic>> sortiereMatchPruefliste(List<dynamic> zeilen) {
  final list =
      zeilen.whereType<Map>().map((z) => z.cast<String, dynamic>()).toList();
  final index = {for (var i = 0; i < list.length; i++) list[i]: i};
  list.sort((a, b) {
    final r = matchPruefRang(a).compareTo(matchPruefRang(b));
    return r != 0 ? r : index[a]!.compareTo(index[b]!);
  });
  return list;
}

String matchEntscheidungLabel(String entscheidung) {
  switch (entscheidung) {
    case 'offen':
      return 'offen';
    case 'bestaetigt':
      return 'bestätigt';
    case 'geaendert':
      return 'geändert';
    case 'verworfen':
      return 'verworfen';
    default:
      return 'noch nicht zugeordnet';
  }
}

String matchScoreText(Object? score) =>
    '${(((score as num?) ?? 0) * 100).round()} %';

String _quelleText(Map<String, dynamic> m) => m['quelle'] == 'modell'
    ? 'Modell ${(m['profil_name'] ?? '').toString()}'
    : 'Wortabgleich';

/// Status des Laufs fuer den Kopf der Pruefliste.
String matchLaufText(Map<String, dynamic> lauf) {
  switch ((lauf['status'] ?? '').toString()) {
    case '':
      return 'Noch keine Zuordnung gelaufen';
    case 'laeuft':
      return 'Zuordnung läuft (${_quelleText(lauf)}): '
          '${lauf['fortschritt'] ?? 0} von ${lauf['gesamt'] ?? 0} Positionen';
    case 'fertig':
      return 'Zuordnung fertig (${_quelleText(lauf)})';
    case 'fehler':
      return 'Zuordnung mit Fehler beendet: ${(lauf['fehler'] ?? '').toString()}';
    case 'abgebrochen':
      return 'Zuordnung abgebrochen (Serverneustart) - bitte neu starten';
    default:
      return 'Zuordnung: ${lauf['status']}';
  }
}

/// Anzahl offener Vorschlaege, die die Sammelbestaetigung erfasst (wie der
/// Server: Code, vorhandene Leistung, Score ab Schwelle).
int matchSammelAnzahl(List<Map<String, dynamic>> zeilen) => zeilen
    .where((z) =>
        z['entscheidung'] == 'offen' &&
        z['ueber_schwelle'] == true &&
        z['leistung_id'] != null)
    .length;

/// [bearbeitbar]: nur ein geparster, noch nicht freigegebener Importlauf
/// laesst Lauf und Entscheidungen zu; sonst nur lesen.
Future<void> showQuoteImportMatchDialog(
        BuildContext context, ApiClient api, String importId,
        {bool bearbeitbar = true}) =>
    showDialog<void>(
      context: context,
      builder: (_) => QuoteImportMatchDialog(
          api: api, importId: importId, bearbeitbar: bearbeitbar),
    );

class QuoteImportMatchDialog extends StatefulWidget {
  const QuoteImportMatchDialog(
      {super.key,
      required this.api,
      required this.importId,
      this.bearbeitbar = true});

  final ApiClient api;
  final String importId;
  final bool bearbeitbar;

  @override
  State<QuoteImportMatchDialog> createState() => _QuoteImportMatchDialogState();
}

class _QuoteImportMatchDialogState extends State<QuoteImportMatchDialog> {
  List<Map<String, dynamic>> zeilen = const [];
  bool loading = true;
  bool busy = false;
  String? error;
  String? hinweis;
  Map<String, dynamic> lauf = const {};
  List<Map<String, dynamic>> profile = const [];
  String auswahl = '';
  Timer? timer;
  bool polling = false;

  bool get canWrite =>
      widget.bearbeitbar && widget.api.hasPermission('quotes.write');
  bool get laeuft => lauf['status'] == 'laeuft';

  @override
  void initState() {
    super.initState();
    _load();
    _loadProfile();
  }

  @override
  void dispose() {
    timer?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final st = await widget.api.getQuoteImportMatchStatus(widget.importId);
      final list = await widget.api.listQuoteImportMatches(widget.importId);
      if (!mounted) return;
      setState(() {
        lauf = st;
        zeilen = sortiereMatchPruefliste(list);
        loading = false;
      });
      if (laeuft) _poll();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        loading = false;
        error = _text(e, 'Prüfliste konnte nicht geladen werden');
      });
    }
  }

  // Profile nur mit settings.manage; ohne sie bleiben Standard und
  // Wortabgleich.
  Future<void> _loadProfile() async {
    if (!canWrite || !widget.api.hasPermission('settings.manage')) return;
    try {
      final list = await widget.api.listLlmProfiles();
      if (!mounted) return;
      setState(() => profile = list
          .whereType<Map>()
          .map((p) => p.cast<String, dynamic>())
          .where((p) => p['aktiv'] == true)
          .toList());
    } catch (_) {
      // ohne Profilliste geht der Start trotzdem
    }
  }

  // Fragt den Status ab, bis der Lauf endet, und laedt dann die Liste neu.
  void _poll() {
    timer?.cancel();
    timer = Timer.periodic(matchPollIntervall, (_) async {
      if (polling) return;
      polling = true;
      try {
        final st = await widget.api.getQuoteImportMatchStatus(widget.importId);
        if (!mounted) return;
        setState(() => lauf = st);
        if (!laeuft) {
          timer?.cancel();
          final list = await widget.api.listQuoteImportMatches(widget.importId);
          if (!mounted) return;
          setState(() => zeilen = sortiereMatchPruefliste(list));
        }
      } catch (e) {
        timer?.cancel();
        if (mounted) setState(() => error = _text(e, 'Status nicht abrufbar'));
      } finally {
        polling = false;
      }
    });
  }

  Future<void> _starten() async {
    setState(() {
      busy = true;
      error = null;
      hinweis = null;
    });
    try {
      final st = await widget.api.startQuoteImportMatch(widget.importId,
          profilId: auswahl == matchAuswahlWortabgleich ? '' : auswahl,
          wortabgleich: auswahl == matchAuswahlWortabgleich);
      if (!mounted) return;
      setState(() => lauf = st);
      if (laeuft) _poll();
    } catch (e) {
      if (!mounted) return;
      setState(() => error = _text(e, 'Zuordnung nicht gestartet'));
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> _aendern(Map<String, dynamic> z) async {
    final leistungId = await showDialog<String>(
      context: context,
      builder: (_) => _LeistungAuswahlDialog(api: widget.api),
    );
    if (leistungId == null || !mounted) return;
    await _entscheiden(z, 'aendern', leistungId: leistungId);
  }

  String _text(Object e, String fallback) =>
      e is ApiException ? e.message : '$fallback: $e';

  // Ersetzt die Zeile an ihrem Platz: die Liste springt beim Entscheiden
  // nicht, neu sortiert wird erst beim naechsten Oeffnen.
  Future<void> _entscheiden(Map<String, dynamic> z, String aktion,
      {String leistungId = ''}) async {
    setState(() {
      busy = true;
      error = null;
      hinweis = null;
    });
    try {
      final neu = await widget.api.decideQuoteImportMatch(
          widget.importId, z['item_id'].toString(), aktion,
          leistungId: leistungId);
      if (!mounted) return;
      setState(() {
        zeilen = [
          for (final alt in zeilen) alt['item_id'] == neu['item_id'] ? neu : alt
        ];
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => error = _text(e, 'Entscheidung nicht gespeichert'));
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> _sammelBestaetigen() async {
    setState(() {
      busy = true;
      error = null;
      hinweis = null;
    });
    try {
      final n = await widget.api.confirmQuoteImportMatches(widget.importId);
      final list = await widget.api.listQuoteImportMatches(widget.importId);
      if (!mounted) return;
      setState(() {
        zeilen = sortiereMatchPruefliste(list);
        hinweis = '$n Vorschläge bestätigt';
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => error = _text(e, 'Sammelbestätigung fehlgeschlagen'));
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Widget _startZeile() => Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: Wrap(
          spacing: 8,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            DropdownButton<String>(
              key: const ValueKey('lauf-auswahl'),
              value: auswahl,
              items: [
                const DropdownMenuItem(
                    value: '', child: Text('Standardprofil')),
                const DropdownMenuItem(
                    value: matchAuswahlWortabgleich,
                    child: Text('Wortabgleich (ohne Sprachmodell)')),
                for (final p in profile)
                  DropdownMenuItem(
                      value: p['id'].toString(),
                      child: Text('Profil ${(p['name'] ?? '').toString()}')),
              ],
              onChanged: busy ? null : (v) => setState(() => auswahl = v ?? ''),
            ),
            FilledButton(
              key: const ValueKey('lauf-starten'),
              onPressed: busy ? null : _starten,
              child: Text((lauf['status'] ?? '') == ''
                  ? 'Zuordnung starten'
                  : 'Zuordnung neu starten'),
            ),
          ],
        ),
      );

  Widget _zeile(Map<String, dynamic> z) {
    final id = z['item_id'].toString();
    final entscheidung = (z['entscheidung'] ?? '').toString();
    final code = (z['code'] ?? '').toString();
    final begruendung = (z['begruendung'] ?? '').toString().trim();
    final vorgaben = ((z['systemvorgaben'] as Map?) ?? const {})
        .entries
        .map((e) => '${e.key}: ${e.value}')
        .join(', ');
    final vorschlag = entscheidung.isEmpty
        ? null
        : code.isEmpty
            ? 'kein Vorschlag'
            : '$code · ${matchScoreText(z['score'])} '
                '(Schwelle ${matchScoreText(z['schwelle'])}) · ${_quelleText(z)}';
    final entschiedenCode = (z['entschieden_code'] ?? '').toString();
    return Card(
      key: ValueKey('match-$id'),
      child: Padding(
        padding: const EdgeInsets.all(8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '${(z['position_no'] ?? '').toString()} '
              '${(z['kurztext'] ?? '').toString()}',
              style: const TextStyle(fontWeight: FontWeight.bold),
            ),
            Text('${z['menge'] ?? ''} ${(z['einheit'] ?? '').toString()}'),
            if (vorschlag != null) Text('Vorschlag: $vorschlag'),
            if (vorgaben.isNotEmpty) Text('Systemvorgaben: $vorgaben'),
            if (begruendung.isNotEmpty) Text('Begründung: $begruendung'),
            Text(
              'Entscheidung: ${matchEntscheidungLabel(entscheidung)}'
              '${entschiedenCode.isEmpty ? '' : ' ($entschiedenCode)'}',
            ),
            if (canWrite && !laeuft && entscheidung.isNotEmpty)
              Wrap(
                spacing: 8,
                children: [
                  if (z['leistung_id'] != null && entscheidung != 'bestaetigt')
                    TextButton(
                      key: ValueKey('bestaetigen-$id'),
                      onPressed:
                          busy ? null : () => _entscheiden(z, 'bestaetigen'),
                      child: const Text('Bestätigen'),
                    ),
                  TextButton(
                    key: ValueKey('aendern-$id'),
                    onPressed: busy ? null : () => _aendern(z),
                    child: const Text('Ändern'),
                  ),
                  if (entscheidung != 'verworfen')
                    TextButton(
                      key: ValueKey('verwerfen-$id'),
                      onPressed:
                          busy ? null : () => _entscheiden(z, 'verwerfen'),
                      child: const Text('Verwerfen'),
                    ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final offen = zeilen.where((z) => z['entscheidung'] == 'offen').length;
    final sammel = matchSammelAnzahl(zeilen);
    return AlertDialog(
      title: const Text('Zuordnung prüfen'),
      content: SizedBox(
        width: 720,
        child: loading
            ? const Center(child: CircularProgressIndicator())
            : SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(matchLaufText(lauf),
                        key: const ValueKey('lauf-status')),
                    if (canWrite && !laeuft) _startZeile(),
                    Text('${zeilen.length} Positionen, $offen offen'),
                    if (error != null)
                      Text(error!,
                          style: TextStyle(
                              color: Theme.of(context).colorScheme.error)),
                    if (hinweis != null) Text(hinweis!),
                    if (canWrite && !laeuft && sammel > 0)
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 8),
                        child: FilledButton.tonal(
                          key: const ValueKey('sammel-bestaetigen'),
                          onPressed: busy ? null : _sammelBestaetigen,
                          child: Text(
                              'Alle über der Schwelle bestätigen ($sammel)'),
                        ),
                      ),
                    ...zeilen.map(_zeile),
                  ],
                ),
              ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Schließen'),
        ),
      ],
    );
  }
}

/// Auswahl einer aktiven Leistung zum Aendern; liefert ihre ID.
class _LeistungAuswahlDialog extends StatefulWidget {
  const _LeistungAuswahlDialog({required this.api});

  final ApiClient api;

  @override
  State<_LeistungAuswahlDialog> createState() => _LeistungAuswahlDialogState();
}

class _LeistungAuswahlDialogState extends State<_LeistungAuswahlDialog> {
  final suche = TextEditingController();
  List<Map<String, dynamic>> treffer = const [];
  bool loading = true;
  String? error;

  @override
  void initState() {
    super.initState();
    _suchen();
  }

  @override
  void dispose() {
    suche.dispose();
    super.dispose();
  }

  Future<void> _suchen() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final list =
          await widget.api.listLeistungen(q: suche.text, nurAktive: true);
      if (!mounted) return;
      setState(() => treffer =
          list.whereType<Map>().map((l) => l.cast<String, dynamic>()).toList());
    } catch (e) {
      if (!mounted) return;
      setState(() => error = e is ApiException
          ? e.message
          : 'Leistungen konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: const Text('Leistung wählen'),
        content: SizedBox(
          width: 560,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              TextField(
                key: const ValueKey('leistung-suche'),
                controller: suche,
                decoration: const InputDecoration(
                    labelText: 'Suche (Code oder Text)',
                    suffixIcon: Icon(Icons.search)),
                onSubmitted: (_) => _suchen(),
              ),
              if (error != null)
                Text(error!,
                    style:
                        TextStyle(color: Theme.of(context).colorScheme.error)),
              if (loading)
                const Padding(
                  padding: EdgeInsets.all(16),
                  child: Center(child: CircularProgressIndicator()),
                )
              else if (treffer.isEmpty && error == null)
                const Text('Keine aktive Leistung gefunden')
              else
                Flexible(
                  child: ListView(
                    shrinkWrap: true,
                    children: [
                      for (final l in treffer)
                        ListTile(
                          key: ValueKey('leistung-${l['id']}'),
                          title: Text('${(l['code'] ?? '').toString()} '
                              '${(l['kurztext'] ?? '').toString()}'),
                          subtitle: Text([
                            (l['einheit'] ?? '').toString(),
                            (l['profilserie'] ?? '').toString(),
                          ].where((t) => t.isNotEmpty).join(' · ')),
                          onTap: () =>
                              Navigator.of(context).pop(l['id'].toString()),
                        ),
                    ],
                  ),
                ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Abbrechen'),
          ),
        ],
      );
}
