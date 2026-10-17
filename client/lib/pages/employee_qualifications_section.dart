import 'package:flutter/material.dart';
import '../api.dart';

// Qualifikationen am Mitarbeiter (Backlog F.7.3.1): Stand je Art und die
// Anforderungen der Stelle. Vorgabe des Nutzers vom 2026-10-13: die
// Personalabteilung legt fest, welche Qualifikationen fuer die Stelle
// noetig sind; anstehende Erneuerungen mahnt das System an.
//
// Lesen mit qualifications.read, Anforderungen setzen mit
// qualifications.write.

String _date(String? iso) {
  if (iso == null || iso.length < 10) return '';
  final p = iso.substring(0, 10).split('-');
  return p.length == 3 ? '${p[2]}.${p[1]}.${p[0]}' : iso;
}

// qualificationStatusLabel beschreibt den Stand einer Art in Worten.
String qualificationStatusLabel(Map<String, dynamic> st) {
  final bis = _date(st['gueltig_bis']?.toString());
  final tage = st['tage_bis_ablauf'];
  switch (st['status']) {
    case 'fehlt':
      return 'fehlt';
    case 'abgelaufen':
      return 'abgelaufen seit $bis';
    case 'laeuft_ab':
      return tage == 0 ? 'läuft heute ab' : 'läuft ab am $bis (in $tage Tagen)';
    case 'gueltig':
      return bis.isEmpty ? 'gültig, unbefristet' : 'gültig bis $bis';
    default:
      return st['status']?.toString() ?? '';
  }
}

Color _statusColor(String? status) {
  switch (status) {
    case 'fehlt':
    case 'abgelaufen':
      return const Color(0xFFB00020);
    case 'laeuft_ab':
      return const Color(0xFFB26A00);
    default:
      return const Color(0xFF2E7D32);
  }
}

// requirementChoices baut die Auswahl fuer den Dialog: alle aktiven Arten
// des Katalogs, dazu Anforderungen auf inzwischen stillgelegte Arten. Ohne
// die stillgelegten wuerde das Speichern sie stillschweigend entfernen -
// PUT ersetzt die Menge vollstaendig.
List<Map<String, dynamic>> requirementChoices(List<dynamic> activeTypes, List<dynamic> requirements) {
  final out = <Map<String, dynamic>>[
    for (final t in activeTypes)
      {
        'type_id': (t as Map<String, dynamic>)['id'].toString(),
        'name': t['name']?.toString() ?? '',
        'pflicht_fuer_alle': t['pflicht_fuer_alle'] == true,
        'stillgelegt': false,
      },
  ];
  final known = out.map((c) => c['type_id']).toSet();
  for (final r in requirements) {
    final m = r as Map<String, dynamic>;
    final id = m['type_id'].toString();
    if (!known.contains(id)) {
      out.add({'type_id': id, 'name': m['name']?.toString() ?? id, 'pflicht_fuer_alle': false, 'stillgelegt': true});
    }
  }
  out.sort((a, b) => (a['name'] as String).toLowerCase().compareTo((b['name'] as String).toLowerCase()));
  return out;
}

class EmployeeQualificationsSection extends StatefulWidget {
  const EmployeeQualificationsSection({super.key, required this.api, required this.employeeId});
  final ApiClient api;
  final String employeeId;

  @override
  State<EmployeeQualificationsSection> createState() => _EmployeeQualificationsSectionState();
}

class _EmployeeQualificationsSectionState extends State<EmployeeQualificationsSection> {
  List<dynamic> standing = [];
  bool loading = false;
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
      final st = await widget.api.getQualificationStanding(widget.employeeId);
      if (mounted) setState(() => standing = st);
    } catch (e) {
      if (mounted) setState(() => error = 'Qualifikationen konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> _editRequirements() async {
    List<dynamic> types;
    List<dynamic> requirements;
    try {
      types = await widget.api.listQualificationTypes();
      requirements = await widget.api.listQualificationRequirements(widget.employeeId);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      return;
    }
    if (!mounted) return;
    final choices = requirementChoices(types, requirements);
    final selected = <String>{for (final r in requirements) (r as Map<String, dynamic>)['type_id'].toString()};
    final result = await showDialog<Set<String>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Anforderungen der Stelle'),
          content: SizedBox(
            width: 460,
            child: choices.isEmpty
                ? const Text('Der Katalog ist leer. Arten legt die Personalabteilung im Qualifikationskatalog an.')
                : SingleChildScrollView(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        for (final c in choices)
                          CheckboxListTile(
                            dense: true,
                            value: selected.contains(c['type_id']),
                            title: Text(c['name'] as String),
                            subtitle: c['stillgelegt'] == true
                                ? const Text('stillgelegt – zählt im Stand nicht mehr')
                                : (c['pflicht_fuer_alle'] == true ? const Text('ohnehin Pflicht für alle') : null),
                            onChanged: (v) => setStateDialog(() {
                              if (v == true) {
                                selected.add(c['type_id'] as String);
                              } else {
                                selected.remove(c['type_id']);
                              }
                            }),
                          ),
                      ],
                    ),
                  ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(onPressed: () => Navigator.of(ctx).pop(Set<String>.from(selected)), child: const Text('Speichern')),
          ],
        ),
      ),
    );
    if (result == null) return;
    try {
      final ids = result.toList()..sort();
      await widget.api.setQualificationRequirements(widget.employeeId, ids);
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Expanded(
              child: Text('Qualifikationen', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
            ),
            if (_canWrite)
              TextButton.icon(
                onPressed: _editRequirements,
                icon: const Icon(Icons.checklist),
                label: const Text('Anforderungen festlegen'),
              ),
          ],
        ),
        if (loading) const LinearProgressIndicator(minHeight: 2),
        if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (!loading && error == null && standing.isEmpty)
          const Text('Keine Anforderungen und keine Nachweise.'),
        for (final s in standing)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 2),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    '${(s as Map<String, dynamic>)['name'] ?? ''}${s['erforderlich'] == true ? ' (erforderlich)' : ''}',
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                const SizedBox(width: 8),
                Text(qualificationStatusLabel(s), style: TextStyle(color: _statusColor(s['status']?.toString()))),
              ],
            ),
          ),
      ],
    );
  }
}
