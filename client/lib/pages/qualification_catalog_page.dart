import 'package:flutter/material.dart';
import '../api.dart';

// Qualifikationskatalog (Backlog F.13.1) auf den Routen aus F.2.5.1:
// Arten anlegen, aendern, stilllegen und den Vorschlagskatalog
// uebernehmen. Vorgabe des Nutzers vom 2026-10-13: "Die Liste sollte in
// jedem Fall auch erweiterbar sein."
//
// Lesen mit qualifications.read, pflegen mit qualifications.write. Arten
// werden nicht geloescht, sondern stillgelegt - Nachweise verweisen auf sie.

const qualificationKategorien = {
  'qualifikation': 'Qualifikation',
  'unterweisung': 'Unterweisung',
  'beauftragung': 'Beauftragung',
};

String kategorieLabel(String? k) => qualificationKategorien[k] ?? (k ?? '');

// fristLabel: 12 -> "alle 12 Monate", 1 -> "jeden Monat", null ->
// "unbefristet".
String fristLabel(Object? months) {
  final m = months is num ? months.toInt() : null;
  if (m == null) return 'unbefristet';
  if (m == 1) return 'jeden Monat';
  return 'alle $m Monate';
}

// bestaetigungLabel: Zwischenbestaetigung (Backlog F.10.3), z. B. bei der
// Schweisserpruefung "Bestätigung alle 6 Monate"; null = keine.
String? bestaetigungLabel(Object? months) {
  final m = months is num ? months.toInt() : null;
  if (m == null) return null;
  return m == 1 ? 'Bestätigung jeden Monat' : 'Bestätigung alle $m Monate';
}

// parseFrist liest die Frist in Monaten: leer = unbefristet (null), sonst
// eine ganze Zahl > 0. ok=false bei ungueltiger Eingabe.
({bool ok, int? months}) parseFrist(String input) {
  final t = input.trim();
  if (t.isEmpty) return (ok: true, months: null);
  final v = int.tryParse(t);
  if (v == null || v <= 0) return (ok: false, months: null);
  return (ok: true, months: v);
}

// buildQualificationTypeBody: fuer POST mit code, fuer PUT mit aktiv - der
// Server verlangt beim PUT alle sieben Felder und lehnt "code" ab.
Map<String, dynamic> buildQualificationTypeBody({
  String? code,
  required String name,
  required String kategorie,
  int? gueltigkeitMonate,
  int? bestaetigungMonate,
  required bool pflichtFuerAlle,
  bool? aktiv,
  String hinweis = '',
}) {
  final body = <String, dynamic>{
    'name': name.trim(),
    'kategorie': kategorie,
    'gueltigkeit_monate': gueltigkeitMonate,
    'bestaetigung_monate': bestaetigungMonate,
    'pflicht_fuer_alle': pflichtFuerAlle,
    'hinweis': hinweis.trim(),
  };
  if (code != null) body['code'] = code.trim().toUpperCase();
  if (aktiv != null) body['aktiv'] = aktiv;
  return body;
}

class QualificationCatalogPage extends StatefulWidget {
  const QualificationCatalogPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<QualificationCatalogPage> createState() => _QualificationCatalogPageState();
}

class _QualificationCatalogPageState extends State<QualificationCatalogPage> {
  List<dynamic> types = [];
  bool loading = false;
  bool showInactive = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('qualifications.write');

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
      final list = await widget.api.listQualificationTypes(includeInactive: showInactive);
      if (mounted) setState(() => types = list);
    } catch (e) {
      if (mounted) setState(() => error = 'Katalog konnte nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _adoptDefaults() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Vorschlagskatalog übernehmen'),
        content: const Text(
            'Übernimmt branchenübliche Arten (u. a. Arbeitsschutz, Erste Hilfe, Stapler, Hubarbeitsbühne, Leitern). '
            'Bereits vorhandene Arten bleiben unverändert.'),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: const Text('Übernehmen')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      final created = await widget.api.adoptDefaultQualificationTypes();
      _snack(created.isEmpty ? 'Alle Vorschläge sind bereits im Katalog.' : '${created.length} Arten übernommen.');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  // _typeDialog: neue Art (type == null) oder Art aendern.
  Future<void> _typeDialog({Map<String, dynamic>? type}) async {
    final editing = type != null;
    final codeCtrl = TextEditingController(text: type?['code']?.toString() ?? '');
    final nameCtrl = TextEditingController(text: type?['name']?.toString() ?? '');
    final fristCtrl = TextEditingController(text: type?['gueltigkeit_monate']?.toString() ?? '');
    final bestaetigungCtrl = TextEditingController(text: type?['bestaetigung_monate']?.toString() ?? '');
    final hinweisCtrl = TextEditingController(text: type?['hinweis']?.toString() ?? '');
    String kategorie = type?['kategorie']?.toString() ?? 'unterweisung';
    bool pflicht = type?['pflicht_fuer_alle'] == true;
    bool aktiv = type == null || type['aktiv'] != false;
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(editing ? 'Art bearbeiten' : 'Neue Art'),
          content: SizedBox(
            width: 460,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(
                    controller: codeCtrl,
                    enabled: !editing,
                    decoration: InputDecoration(
                        labelText: 'Code (A-Z, 0-9, _)', helperText: editing ? 'Der Code ist nicht änderbar.' : null),
                  ),
                  TextField(controller: nameCtrl, decoration: const InputDecoration(labelText: 'Name')),
                  DropdownButtonFormField<String>(
                    initialValue: kategorie,
                    decoration: const InputDecoration(labelText: 'Kategorie'),
                    items: [
                      for (final e in qualificationKategorien.entries) DropdownMenuItem(value: e.key, child: Text(e.value)),
                    ],
                    onChanged: (v) => setStateDialog(() => kategorie = v ?? kategorie),
                  ),
                  TextField(
                    controller: fristCtrl,
                    decoration: const InputDecoration(labelText: 'Gültigkeit in Monaten (leer = unbefristet)'),
                  ),
                  TextField(
                    controller: bestaetigungCtrl,
                    decoration: const InputDecoration(
                        labelText: 'Bestätigung alle … Monate (leer = keine)',
                        helperText: 'z. B. Schweißerprüfung: 6. Eine überfällige Bestätigung wirkt wie ein Ablauf.'),
                  ),
                  CheckboxListTile(
                    contentPadding: EdgeInsets.zero,
                    value: pflicht,
                    title: const Text('Pflicht für alle Mitarbeiter'),
                    onChanged: (v) => setStateDialog(() => pflicht = v == true),
                  ),
                  if (editing)
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      value: aktiv,
                      title: const Text('Aktiv'),
                      subtitle: const Text('Stillgelegte Arten nehmen keine neuen Nachweise an; bestehende bleiben.'),
                      onChanged: (v) => setStateDialog(() => aktiv = v == true),
                    ),
                  TextField(controller: hinweisCtrl, decoration: const InputDecoration(labelText: 'Hinweis (z. B. Rechtsgrundlage)')),
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
                final frist = parseFrist(fristCtrl.text);
                final bestaetigung = parseFrist(bestaetigungCtrl.text);
                String? problem;
                if (!editing && codeCtrl.text.trim().isEmpty) {
                  problem = 'Bitte einen Code angeben.';
                } else if (nameCtrl.text.trim().isEmpty) {
                  problem = 'Bitte einen Namen angeben.';
                } else if (!frist.ok) {
                  problem = 'Bitte die Gültigkeit als ganze Zahl von Monaten angeben oder leer lassen.';
                } else if (!bestaetigung.ok) {
                  problem = 'Bitte die Bestätigung als ganze Zahl von Monaten angeben oder leer lassen.';
                } else if (bestaetigung.months != null && frist.months != null && bestaetigung.months! >= frist.months!) {
                  problem = 'Die Bestätigung muss kürzer als die Gültigkeit sein.';
                }
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(buildQualificationTypeBody(
                  code: editing ? null : codeCtrl.text,
                  name: nameCtrl.text,
                  kategorie: kategorie,
                  gueltigkeitMonate: frist.months,
                  bestaetigungMonate: bestaetigung.months,
                  pflichtFuerAlle: pflicht,
                  aktiv: editing ? aktiv : null,
                  hinweis: hinweisCtrl.text,
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
        await widget.api.updateQualificationType(type['id'].toString(), body);
        _snack('Art gespeichert');
      } else {
        await widget.api.createQualificationType(body);
        _snack('Art angelegt');
      }
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Qualifikationskatalog'),
        actions: [
          if (_canWrite)
            TextButton.icon(
              onPressed: _adoptDefaults,
              icon: const Icon(Icons.playlist_add),
              label: const Text('Vorschlagskatalog übernehmen'),
            ),
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden'),
        ],
      ),
      floatingActionButton: _canWrite
          ? FloatingActionButton(onPressed: () => _typeDialog(), tooltip: 'Neue Art', child: const Icon(Icons.add))
          : null,
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Align(
              alignment: Alignment.centerLeft,
              child: FilterChip(
                label: const Text('Stillgelegte anzeigen'),
                selected: showInactive,
                onSelected: (v) {
                  setState(() => showInactive = v);
                  _load();
                },
              ),
            ),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && types.isEmpty)
              Text(_canWrite
                  ? 'Der Katalog ist leer. Legen Sie Arten an oder übernehmen Sie den Vorschlagskatalog.'
                  : 'Der Katalog ist leer.'),
            for (final t in types)
              Builder(builder: (context) {
                final m = t as Map<String, dynamic>;
                final details = [
                  kategorieLabel(m['kategorie']?.toString()),
                  fristLabel(m['gueltigkeit_monate']),
                  if (bestaetigungLabel(m['bestaetigung_monate']) != null) bestaetigungLabel(m['bestaetigung_monate'])!,
                  if (m['pflicht_fuer_alle'] == true) 'Pflicht für alle',
                  if (m['aktiv'] == false) 'stillgelegt',
                ].join(' · ');
                return ListTile(
                  title: Text('${m['name'] ?? ''} (${m['code'] ?? ''})'),
                  subtitle: Text(details),
                  trailing: _canWrite
                      ? IconButton(tooltip: 'Bearbeiten', icon: const Icon(Icons.edit), onPressed: () => _typeDialog(type: m))
                      : null,
                );
              }),
          ],
        ),
      ),
    );
  }
}
