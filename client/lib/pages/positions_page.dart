import 'package:flutter/material.dart';
import '../api.dart';

// Stellen (Backlog F.11.5) auf den Routen aus F.11.2/F.11.4: eine Stelle
// ("Monteur", "Schweißer") verlangt Qualifikationsarten, die fuer alle
// ihre Inhaber gelten - dauerhaft: eine Aenderung hier wirkt sofort auf
// alle Mitarbeiter mit dieser Stelle (ADR 0031). Wunsch des Nutzers vom
// 2026-10-13: "Vorlagen für Mitarbeiter-Stellen generieren, die bestimmte
// Qualifikationen erfordern".
//
// Lesen mit qualifications.read, pflegen mit qualifications.write. Stellen
// werden nicht geloescht, sondern stillgelegt.

// buildPositionBody: fuer POST mit code, fuer PUT mit aktiv - der Server
// verlangt beim PUT name, aktiv, hinweis und type_ids und lehnt "code" ab.
Map<String, dynamic> buildPositionBody({
  String? code,
  required String name,
  String hinweis = '',
  required List<String> typeIds,
  bool? aktiv,
}) {
  final ids = typeIds.toSet().toList()..sort();
  final body = <String, dynamic>{
    'name': name.trim(),
    'hinweis': hinweis.trim(),
    'type_ids': ids,
  };
  if (code != null) body['code'] = code.trim().toUpperCase();
  if (aktiv != null) body['aktiv'] = aktiv;
  return body;
}

// positionTypeChoices baut die Auswahl im Dialog: alle aktiven Arten, dazu
// bereits zugeordnete, inzwischen stillgelegte. Ohne diese wuerde das
// Speichern sie stillschweigend entfernen - PUT ersetzt die Menge.
// allTypes: Katalog inklusive stillgelegter (fuer die Namen).
List<Map<String, dynamic>> positionTypeChoices(List<dynamic> allTypes, List<String> assigned) {
  final byId = {for (final t in allTypes) (t as Map<String, dynamic>)['id'].toString(): t};
  final out = <Map<String, dynamic>>[
    for (final t in allTypes)
      if ((t as Map<String, dynamic>)['aktiv'] != false)
        {'type_id': t['id'].toString(), 'name': t['name']?.toString() ?? t['id'].toString(), 'stillgelegt': false},
  ];
  final known = out.map((c) => c['type_id']).toSet();
  for (final id in assigned) {
    if (known.contains(id)) continue;
    final t = byId[id];
    out.add({'type_id': id, 'name': t?['name']?.toString() ?? id, 'stillgelegt': true});
  }
  out.sort((a, b) => (a['name'] as String).toLowerCase().compareTo((b['name'] as String).toLowerCase()));
  return out;
}

// positionTypesLabel: die Arten einer Stelle als Namen, sortiert;
// stillgelegte gekennzeichnet (sie zaehlen im Stand nicht).
String positionTypesLabel(Map<String, dynamic> position, List<dynamic> allTypes) {
  final byId = {for (final t in allTypes) (t as Map<String, dynamic>)['id'].toString(): t};
  final ids = ((position['type_ids'] as List?) ?? const []).map((e) => e.toString());
  final names = [
    for (final id in ids)
      if (byId[id] == null)
        id
      else if (byId[id]!['aktiv'] == false)
        '${byId[id]!['name']} (stillgelegt)'
      else
        byId[id]!['name'].toString(),
  ]..sort((a, b) => a.toLowerCase().compareTo(b.toLowerCase()));
  return names.isEmpty ? 'keine Qualifikationen' : names.join(', ');
}

// adoptedPositionLines: je neu uebernommener Stelle eine Zeile, mit den
// fehlenden Arten.
List<String> adoptedPositionLines(List<dynamic> adopted) => [
      for (final a in adopted)
        () {
          final m = a as Map<String, dynamic>;
          final missing = ((m['fehlende_arten'] as List?) ?? const []).map((e) => e.toString()).toList();
          final name = m['name']?.toString() ?? '';
          return missing.isEmpty ? name : '$name – fehlende Arten: ${missing.join(', ')}';
        }(),
    ];

class PositionsPage extends StatefulWidget {
  const PositionsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<PositionsPage> createState() => _PositionsPageState();
}

class _PositionsPageState extends State<PositionsPage> {
  List<dynamic> positions = [];
  // Katalog inklusive stillgelegter: Namen und Auswahl.
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
      final list = await widget.api.listPositions(includeInactive: showInactive);
      final cat = await widget.api.listQualificationTypes(includeInactive: true);
      if (mounted) {
        setState(() {
          positions = list;
          types = cat;
        });
      }
    } catch (e) {
      if (mounted) setState(() => error = 'Stellen konnten nicht geladen werden: $e');
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
        title: const Text('Vorschläge übernehmen'),
        content: const Text('Übernimmt typische Stellen (Monteur, Schweißer, Lagerist, Staplerfahrer, Kranführer). '
            'Die Zuordnung der Qualifikationen ist ein Vorschlag, nicht recherchiert – bitte mit der Fachkraft für '
            'Arbeitssicherheit abgleichen.\n\n'
            'Qualifikationen werden nur zugeordnet, wenn sie im Katalog stehen: übernehmen Sie zuerst den '
            'Qualifikationskatalog. Vorhandene Stellen (gleicher Code oder Name) bleiben unverändert.'),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: const Text('Übernehmen')),
        ],
      ),
    );
    if (ok != true) return;
    List<dynamic> created;
    try {
      created = await widget.api.adoptDefaultPositions();
    } catch (e) {
      _snack('Fehler: $e');
      return;
    }
    await _load();
    if (!mounted) return;
    if (created.isEmpty) {
      _snack('Alle Vorschläge sind bereits vorhanden.');
      return;
    }
    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('${created.length} Stellen übernommen'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [for (final l in adoptedPositionLines(created)) Text(l)],
        ),
        actions: [FilledButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('OK'))],
      ),
    );
  }

  // _positionDialog: neue Stelle (position == null) oder Stelle aendern.
  Future<void> _positionDialog({Map<String, dynamic>? position}) async {
    final editing = position != null;
    final codeCtrl = TextEditingController(text: position?['code']?.toString() ?? '');
    final nameCtrl = TextEditingController(text: position?['name']?.toString() ?? '');
    final hinweisCtrl = TextEditingController(text: position?['hinweis']?.toString() ?? '');
    bool aktiv = position == null || position['aktiv'] != false;
    final assigned = ((position?['type_ids'] as List?) ?? const []).map((e) => e.toString()).toList();
    final selected = assigned.toSet();
    final choices = positionTypeChoices(types, assigned);
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(editing ? 'Stelle bearbeiten' : 'Neue Stelle'),
          content: SizedBox(
            width: 480,
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
                  TextField(controller: hinweisCtrl, decoration: const InputDecoration(labelText: 'Hinweis')),
                  if (editing)
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      value: aktiv,
                      title: const Text('Aktiv'),
                      subtitle: const Text('Eine stillgelegte Stelle zählt bei ihren Inhabern nicht mehr.'),
                      onChanged: (v) => setStateDialog(() => aktiv = v == true),
                    ),
                  const SizedBox(height: 8),
                  const Text('Erforderliche Qualifikationen', style: TextStyle(fontWeight: FontWeight.bold)),
                  if (editing)
                    const Text('Änderungen gelten sofort für alle Mitarbeiter mit dieser Stelle.'),
                  if (choices.isEmpty)
                    const Text('Der Qualifikationskatalog ist leer.'),
                  for (final c in choices)
                    CheckboxListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      value: selected.contains(c['type_id']),
                      title: Text(c['name'] as String),
                      subtitle: c['stillgelegt'] == true ? const Text('stillgelegt – zählt im Stand nicht mehr') : null,
                      onChanged: (v) => setStateDialog(() {
                        if (v == true) {
                          selected.add(c['type_id'] as String);
                        } else {
                          selected.remove(c['type_id']);
                        }
                      }),
                    ),
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
                String? problem;
                if (!editing && codeCtrl.text.trim().isEmpty) {
                  problem = 'Bitte einen Code angeben.';
                } else if (nameCtrl.text.trim().isEmpty) {
                  problem = 'Bitte einen Namen angeben.';
                }
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(buildPositionBody(
                  code: editing ? null : codeCtrl.text,
                  name: nameCtrl.text,
                  hinweis: hinweisCtrl.text,
                  typeIds: selected.toList(),
                  aktiv: editing ? aktiv : null,
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
        await widget.api.updatePosition(position['id'].toString(), body);
        _snack('Stelle gespeichert');
      } else {
        await widget.api.createPosition(body);
        _snack('Stelle angelegt');
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
        title: const Text('Stellen'),
        actions: [
          if (_canWrite)
            TextButton.icon(
              onPressed: _adoptDefaults,
              icon: const Icon(Icons.playlist_add),
              label: const Text('Vorschläge übernehmen'),
            ),
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden'),
        ],
      ),
      floatingActionButton: _canWrite
          ? FloatingActionButton(onPressed: () => _positionDialog(), tooltip: 'Neue Stelle', child: const Icon(Icons.add))
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
            if (!loading && error == null && positions.isEmpty)
              Text(_canWrite ? 'Noch keine Stellen. Legen Sie Stellen an oder übernehmen Sie die Vorschläge.' : 'Noch keine Stellen.'),
            for (final p in positions)
              Builder(builder: (context) {
                final m = p as Map<String, dynamic>;
                return ListTile(
                  title: Text('${m['name'] ?? ''} (${m['code'] ?? ''})${m['aktiv'] == false ? ' · stillgelegt' : ''}'),
                  subtitle: Text(positionTypesLabel(m, types)),
                  trailing: _canWrite
                      ? IconButton(tooltip: 'Bearbeiten', icon: const Icon(Icons.edit), onPressed: () => _positionDialog(position: m))
                      : null,
                );
              }),
          ],
        ),
      ),
    );
  }
}
