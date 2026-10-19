import 'package:flutter/material.dart';
import '../api.dart';
import 'employee_qualifications_section.dart' show parseDateInput;

// Betriebsmittel-Bestand (Backlog F.13.3) auf den Routen aus F.3.6.1:
// Einzelstuecke mit Inventarnummer anlegen, bearbeiten, ausmustern. Lesen
// mit assets.read, pflegen mit assets.write (vorerst nur die
// Personalabteilung, Entscheidung des Nutzers vom 2026-10-15).
//
// Nicht verwechseln: /projects/{id}/assets sind Projektdateien.

const assetKategorien = {
  'werkzeug': 'Werkzeug',
  'elektrowerkzeug': 'Elektrowerkzeug',
  'messmittel': 'Messmittel',
  'leiter': 'Leiter',
  'psa': 'PSA',
  'it': 'IT',
  'maschine': 'Maschine',
  'sonstiges': 'Sonstiges',
};

String assetKategorieLabel(String? k) => assetKategorien[k] ?? (k ?? '');

String _two(int v) => v.toString().padLeft(2, '0');

// isoToGerman: "2025-03-01T00:00:00Z" -> "01.03.2025"; leer bei null.
String isoToGerman(String? iso) {
  if (iso == null || iso.length < 10) return '';
  final p = iso.substring(0, 10).split('-');
  return p.length == 3 ? '${p[2]}.${p[1]}.${p[0]}' : '';
}

// parsePrice liest einen Betrag in deutscher Schreibweise ("1.234,56",
// "499,9") oder mit Punkt als Dezimaltrenner ("499.90"). Leer = kein
// Preis. ok=false bei ungueltiger oder negativer Eingabe.
({bool ok, double? value}) parsePrice(String input) {
  var t = input.trim().replaceAll('€', '').replaceAll(' ', '');
  if (t.isEmpty) return (ok: true, value: null);
  if (t.contains(',')) t = t.replaceAll('.', '').replaceAll(',', '.');
  if (!RegExp(r'^\d+(\.\d{1,2})?$').hasMatch(t)) return (ok: false, value: null);
  return (ok: true, value: double.parse(t));
}

// formatPrice: 1234.5 -> "1234,50".
String formatPrice(Object? v) => v is num ? v.toStringAsFixed(2).replaceAll('.', ',') : '';

// intervallLabel: 6 -> "Prüfung alle 6 Monate", null -> "ohne Prüfintervall".
String intervallLabel(Object? months) {
  final m = months is num ? months.toInt() : null;
  if (m == null) return 'ohne Prüfintervall';
  return m == 1 ? 'Prüfung jeden Monat' : 'Prüfung alle $m Monate';
}

// buildAssetBody: immer alle zwoelf Felder - der Server verlangt sie beim
// PUT, und beim POST schadet die Vollstaendigkeit nicht.
Map<String, dynamic> buildAssetBody({
  required String inventarnummer,
  required String bezeichnung,
  required String kategorie,
  String seriennummer = '',
  String hersteller = '',
  String modell = '',
  String standort = '',
  String? anschaffungsdatum,
  double? anschaffungspreis,
  String? kostenstelleId,
  int? pruefintervallMonate,
  String notiz = '',
}) =>
    {
      'inventarnummer': inventarnummer.trim(),
      'bezeichnung': bezeichnung.trim(),
      'kategorie': kategorie,
      'seriennummer': seriennummer.trim(),
      'hersteller': hersteller.trim(),
      'modell': modell.trim(),
      'standort': standort.trim(),
      'anschaffungsdatum': anschaffungsdatum,
      'anschaffungspreis': anschaffungspreis,
      'kostenstelle_id': (kostenstelleId == null || kostenstelleId.isEmpty) ? null : kostenstelleId,
      'pruefintervall_monate': pruefintervallMonate,
      'notiz': notiz.trim(),
    };

class AssetsPage extends StatefulWidget {
  const AssetsPage({super.key, required this.api, this.now});
  final ApiClient api;
  final DateTime Function()? now;

  @override
  State<AssetsPage> createState() => _AssetsPageState();
}

class _AssetsPageState extends State<AssetsPage> {
  List<dynamic> assets = [];
  // Kostenstellen zur Auswahl; null = nicht ladbar (z. B. ohne
  // cost_centers.read) - dann bleibt eine bestehende Zuordnung erhalten.
  List<dynamic>? costCenters;
  bool loading = false;
  String? error;
  String kategorieFilter = '';
  bool showRetired = false;

  bool get _canWrite => widget.api.hasPermission('assets.write');
  DateTime _now() => (widget.now ?? DateTime.now)();

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final list = await widget.api.listAssets(kategorie: kategorieFilter, includeAusgemustert: showRetired);
      List<dynamic>? cc = costCenters;
      if (_canWrite && cc == null) {
        try {
          cc = await widget.api.listCostCenters();
        } catch (_) {
          cc = null;
        }
      }
      if (!mounted) return;
      setState(() {
        assets = list;
        costCenters = cc;
      });
    } catch (e) {
      if (mounted) setState(() => error = 'Betriebsmittel konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _assetDialog({Map<String, dynamic>? asset}) async {
    final editing = asset != null;
    TextEditingController c(String key) => TextEditingController(text: asset?[key]?.toString() ?? '');
    final invCtrl = c('inventarnummer');
    final bezCtrl = c('bezeichnung');
    final snCtrl = c('seriennummer');
    final herstellerCtrl = c('hersteller');
    final modellCtrl = c('modell');
    final standortCtrl = c('standort');
    final notizCtrl = c('notiz');
    final datumCtrl = TextEditingController(text: isoToGerman(asset?['anschaffungsdatum']?.toString()));
    final preisCtrl = TextEditingController(text: formatPrice(asset?['anschaffungspreis']));
    final intervallCtrl = c('pruefintervall_monate');
    String kategorie = asset?['kategorie']?.toString() ?? 'werkzeug';
    String? kostenstelle = asset?['kostenstelle_id']?.toString();
    final ccList = costCenters;
    final ccKnown = ccList != null && (kostenstelle == null || ccList.any((x) => (x as Map<String, dynamic>)['id'] == kostenstelle));
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(editing ? 'Betriebsmittel bearbeiten' : 'Neues Betriebsmittel'),
          content: SizedBox(
            width: 480,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(controller: invCtrl, decoration: const InputDecoration(labelText: 'Inventarnummer')),
                  TextField(controller: bezCtrl, decoration: const InputDecoration(labelText: 'Bezeichnung')),
                  DropdownButtonFormField<String>(
                    initialValue: kategorie,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Kategorie'),
                    items: [for (final e in assetKategorien.entries) DropdownMenuItem(value: e.key, child: Text(e.value))],
                    onChanged: (v) => setStateDialog(() => kategorie = v ?? kategorie),
                  ),
                  TextField(controller: snCtrl, decoration: const InputDecoration(labelText: 'Seriennummer')),
                  TextField(controller: herstellerCtrl, decoration: const InputDecoration(labelText: 'Hersteller')),
                  TextField(controller: modellCtrl, decoration: const InputDecoration(labelText: 'Modell')),
                  TextField(controller: standortCtrl, decoration: const InputDecoration(labelText: 'Standort')),
                  TextField(controller: datumCtrl, decoration: const InputDecoration(labelText: 'Anschaffungsdatum (TT.MM.JJJJ, optional)')),
                  TextField(controller: preisCtrl, decoration: const InputDecoration(labelText: 'Anschaffungspreis in € (optional)')),
                  TextField(
                    controller: intervallCtrl,
                    decoration: const InputDecoration(labelText: 'Prüfintervall in Monaten (leer = keine wiederkehrende Prüfung)'),
                  ),
                  if (ccKnown)
                    DropdownButtonFormField<String?>(
                      initialValue: kostenstelle,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Kostenstelle (optional)'),
                      items: [
                        const DropdownMenuItem<String?>(value: null, child: Text('Keine')),
                        for (final x in ccList)
                          DropdownMenuItem<String?>(
                            value: (x as Map<String, dynamic>)['id'].toString(),
                            child: Text('${x['code'] ?? ''} – ${x['name'] ?? ''}'),
                          ),
                      ],
                      onChanged: (v) => setStateDialog(() => kostenstelle = v),
                    )
                  else if (kostenstelle != null)
                    const Padding(
                      padding: EdgeInsets.only(top: 8),
                      child: Text('Kostenstelle zugeordnet – sie bleibt beim Speichern erhalten.'),
                    ),
                  TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
                  if (dialogError != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(dialogError!, style: const TextStyle(color: Color(0xFFB00020))),
                    ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final datumText = datumCtrl.text.trim();
                final datum = datumText.isEmpty ? null : parseDateInput(datumText);
                final preis = parsePrice(preisCtrl.text);
                final intervallText = intervallCtrl.text.trim();
                final intervall = intervallText.isEmpty ? null : int.tryParse(intervallText);
                String? problem;
                if (invCtrl.text.trim().isEmpty) {
                  problem = 'Bitte eine Inventarnummer angeben.';
                } else if (bezCtrl.text.trim().isEmpty) {
                  problem = 'Bitte eine Bezeichnung angeben.';
                } else if (datumText.isNotEmpty && datum == null) {
                  problem = 'Bitte das Anschaffungsdatum als TT.MM.JJJJ angeben oder leer lassen.';
                } else if (!preis.ok) {
                  problem = 'Bitte den Preis als Betrag angeben (z. B. 499,90) oder leer lassen.';
                } else if (intervallText.isNotEmpty && (intervall == null || intervall <= 0)) {
                  problem = 'Bitte das Prüfintervall als ganze Zahl von Monaten angeben oder leer lassen.';
                }
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(buildAssetBody(
                  inventarnummer: invCtrl.text,
                  bezeichnung: bezCtrl.text,
                  kategorie: kategorie,
                  seriennummer: snCtrl.text,
                  hersteller: herstellerCtrl.text,
                  modell: modellCtrl.text,
                  standort: standortCtrl.text,
                  anschaffungsdatum: datum,
                  anschaffungspreis: preis.value,
                  kostenstelleId: kostenstelle,
                  pruefintervallMonate: intervall,
                  notiz: notizCtrl.text,
                ));
              },
              child: Text(editing ? 'Speichern' : 'Anlegen'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    try {
      if (editing) {
        await widget.api.updateAsset(asset['id'].toString(), body);
        _snack('Betriebsmittel gespeichert');
      } else {
        await widget.api.createAsset(body);
        _snack('Betriebsmittel angelegt');
      }
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _retireDialog(Map<String, dynamic> asset) async {
    final today = _now();
    final datumCtrl = TextEditingController(text: '${_two(today.day)}.${_two(today.month)}.${today.year}');
    final grundCtrl = TextEditingController();
    String? dialogError;
    final result = await showDialog<({String datum, String grund})>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Ausmustern'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${asset['inventarnummer']} – ${asset['bezeichnung']}'),
                const SizedBox(height: 8),
                const Text('Ausmustern ist endgültig. Ein noch ausgegebenes Stück muss vorher zurückgenommen werden.'),
                TextField(controller: datumCtrl, decoration: const InputDecoration(labelText: 'Datum (TT.MM.JJJJ)')),
                TextField(controller: grundCtrl, decoration: const InputDecoration(labelText: 'Grund')),
                if (dialogError != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(dialogError!, style: const TextStyle(color: Color(0xFFB00020))),
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final datum = parseDateInput(datumCtrl.text);
                if (datum == null) {
                  setStateDialog(() => dialogError = 'Bitte das Datum als TT.MM.JJJJ angeben.');
                  return;
                }
                if (grundCtrl.text.trim().isEmpty) {
                  setStateDialog(() => dialogError = 'Bitte einen Grund angeben.');
                  return;
                }
                Navigator.of(ctx).pop((datum: datum, grund: grundCtrl.text.trim()));
              },
              child: const Text('Ausmustern'),
            ),
          ],
        ),
      ),
    );
    if (result == null) return;
    try {
      await widget.api.retireAsset(asset['id'].toString(), datum: result.datum, grund: result.grund);
      _snack('Betriebsmittel ausgemustert');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Betriebsmittel'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      floatingActionButton: _canWrite
          ? FloatingActionButton(onPressed: () => _assetDialog(), tooltip: 'Neues Betriebsmittel', child: const Icon(Icons.add))
          : null,
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Wrap(
              spacing: 12,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                SizedBox(
                  width: 240,
                  child: DropdownButtonFormField<String>(
                    initialValue: kategorieFilter,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Kategorie'),
                    items: [
                      const DropdownMenuItem(value: '', child: Text('Alle')),
                      for (final e in assetKategorien.entries) DropdownMenuItem(value: e.key, child: Text(e.value)),
                    ],
                    onChanged: (v) {
                      setState(() => kategorieFilter = v ?? '');
                      _load();
                    },
                  ),
                ),
                FilterChip(
                  label: const Text('Ausgemusterte anzeigen'),
                  selected: showRetired,
                  onSelected: (v) {
                    setState(() => showRetired = v);
                    _load();
                  },
                ),
              ],
            ),
            const SizedBox(height: 8),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && assets.isEmpty) const Text('Keine Betriebsmittel gefunden.'),
            for (final a in assets)
              Builder(builder: (context) {
                final m = a as Map<String, dynamic>;
                final retired = m['ausgemustert_am'] != null;
                final details = [
                  assetKategorieLabel(m['kategorie']?.toString()),
                  if ((m['standort']?.toString() ?? '').isNotEmpty) m['standort'].toString(),
                  intervallLabel(m['pruefintervall_monate']),
                  if (retired) 'ausgemustert am ${isoToGerman(m['ausgemustert_am']?.toString())} (${m['ausgemustert_grund'] ?? ''})',
                ].join(' · ');
                return ListTile(
                  title: Text('${m['inventarnummer'] ?? ''} – ${m['bezeichnung'] ?? ''}'),
                  subtitle: Text(details),
                  trailing: _canWrite
                      ? Wrap(
                          children: [
                            IconButton(tooltip: 'Bearbeiten', icon: const Icon(Icons.edit), onPressed: () => _assetDialog(asset: m)),
                            if (!retired)
                              IconButton(tooltip: 'Ausmustern', icon: const Icon(Icons.delete_sweep), onPressed: () => _retireDialog(m)),
                          ],
                        )
                      : null,
                );
              }),
          ],
        ),
      ),
    );
  }
}
