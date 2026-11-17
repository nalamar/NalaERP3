import 'package:flutter/material.dart';
import '../api.dart';

// Pruefliste der Zuordnung am GAEB-Import (ADR 0045, Backlog I.5.5a):
// Vorschlaege lesen, je Position bestaetigen oder verwerfen, alle ueber der
// Schwelle auf einmal bestaetigen. Lauf starten und Aendern folgen in I.5.5b.

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

/// Anzahl offener Vorschlaege, die die Sammelbestaetigung erfasst (wie der
/// Server: Code, vorhandene Leistung, Score ab Schwelle).
int matchSammelAnzahl(List<Map<String, dynamic>> zeilen) => zeilen
    .where((z) =>
        z['entscheidung'] == 'offen' &&
        z['ueber_schwelle'] == true &&
        z['leistung_id'] != null)
    .length;

Future<void> showQuoteImportMatchDialog(
        BuildContext context, ApiClient api, String importId) =>
    showDialog<void>(
      context: context,
      builder: (_) => QuoteImportMatchDialog(api: api, importId: importId),
    );

class QuoteImportMatchDialog extends StatefulWidget {
  const QuoteImportMatchDialog(
      {super.key, required this.api, required this.importId});

  final ApiClient api;
  final String importId;

  @override
  State<QuoteImportMatchDialog> createState() => _QuoteImportMatchDialogState();
}

class _QuoteImportMatchDialogState extends State<QuoteImportMatchDialog> {
  List<Map<String, dynamic>> zeilen = const [];
  bool loading = true;
  bool busy = false;
  String? error;
  String? hinweis;

  bool get canWrite => widget.api.hasPermission('quotes.write');

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final list = await widget.api.listQuoteImportMatches(widget.importId);
      if (!mounted) return;
      setState(() {
        zeilen = sortiereMatchPruefliste(list);
        loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        loading = false;
        error = _text(e, 'Prüfliste konnte nicht geladen werden');
      });
    }
  }

  String _text(Object e, String fallback) =>
      e is ApiException ? e.message : '$fallback: $e';

  // Ersetzt die Zeile an ihrem Platz: die Liste springt beim Entscheiden
  // nicht, neu sortiert wird erst beim naechsten Oeffnen.
  Future<void> _entscheiden(Map<String, dynamic> z, String aktion) async {
    setState(() {
      busy = true;
      error = null;
      hinweis = null;
    });
    try {
      final neu = await widget.api.decideQuoteImportMatch(
          widget.importId, z['item_id'].toString(), aktion);
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

  Widget _zeile(Map<String, dynamic> z) {
    final id = z['item_id'].toString();
    final entscheidung = (z['entscheidung'] ?? '').toString();
    final code = (z['code'] ?? '').toString();
    final begruendung = (z['begruendung'] ?? '').toString().trim();
    final vorgaben = ((z['systemvorgaben'] as Map?) ?? const {})
        .entries
        .map((e) => '${e.key}: ${e.value}')
        .join(', ');
    final quelle = z['quelle'] == 'modell'
        ? 'Modell ${(z['profil_name'] ?? '').toString()}'
        : 'Wortabgleich';
    final vorschlag = entscheidung.isEmpty
        ? null
        : code.isEmpty
            ? 'kein Vorschlag'
            : '$code · ${matchScoreText(z['score'])} '
                '(Schwelle ${matchScoreText(z['schwelle'])}) · $quelle';
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
            if (canWrite && entscheidung.isNotEmpty)
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
                    Text('${zeilen.length} Positionen, $offen offen'),
                    if (error != null)
                      Text(error!,
                          style: TextStyle(
                              color: Theme.of(context).colorScheme.error)),
                    if (hinweis != null) Text(hinweis!),
                    if (canWrite && sammel > 0)
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
