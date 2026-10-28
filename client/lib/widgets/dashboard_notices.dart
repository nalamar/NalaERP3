import 'package:flutter/material.dart';
import '../api.dart';
import '../pages/asset_inspections_due_page.dart';
import '../pages/fleet_due_page.dart';
import '../pages/qualifications_due_page.dart';

// Hinweise auf dem Dashboard (Backlog F.10.4.2): was faellig ist, je Art
// eine Zeile mit Anzahl; die Farbe folgt der Dringlichkeit. Der Server
// entscheidet nach den Rechten, welche Arten kommen (GET /notices); ein
// Klick oeffnet die passende Faelligkeitsliste. Ohne Hinweise erscheint
// nichts.

// Reihenfolge und Worte der Staende in einer Zeile.
const _statusWords = <String, String>{
  'gesperrt': 'gesperrt',
  'fehlt': 'fehlt',
  'abgelaufen': 'abgelaufen',
  'ueberfaellig': 'überfällig',
  'nie_geprueft': 'nie geprüft',
  'laeuft_ab': 'läuft ab',
  'faellig_bald': 'bald fällig',
};

// noticeLine beschreibt einen Hinweis, z. B.
// "Fällige Qualifikationen: 3 (1 fehlt, 2 läuft ab)".
String noticeLine(Map<String, dynamic> n) {
  final counts = (n['counts'] as Map?)?.cast<String, dynamic>() ?? const {};
  final parts = <String>[
    for (final e in _statusWords.entries)
      if ((counts[e.key] as num? ?? 0) > 0) '${counts[e.key]} ${e.value}',
    for (final k in counts.keys)
      if (!_statusWords.containsKey(k) && (counts[k] as num? ?? 0) > 0) '${counts[k]} $k',
  ];
  final head = '${n['title'] ?? n['kind'] ?? ''}: ${n['total'] ?? 0}';
  return parts.isEmpty ? head : '$head (${parts.join(', ')})';
}

// noticeDestination: die Liste zu einer Art; null, wenn es keine eigene
// Seite gibt (eigene Qualifikationen stehen beim Mitarbeiter).
Widget? noticeDestination(String kind, ApiClient api) {
  switch (kind) {
    case 'qualifications_due':
      return QualificationsDuePage(api: api);
    case 'asset_inspections_due':
      return AssetInspectionsDuePage(api: api);
    case 'fleet_schedules_due':
      return FleetDuePage(api: api);
    default:
      return null;
  }
}

class DashboardNotices extends StatefulWidget {
  const DashboardNotices({super.key, required this.api});
  final ApiClient api;

  @override
  State<DashboardNotices> createState() => _DashboardNoticesState();
}

class _DashboardNoticesState extends State<DashboardNotices> {
  late Future<List<dynamic>> _future;

  @override
  void initState() {
    super.initState();
    _future = widget.api.listNotices();
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<dynamic>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) return const SizedBox.shrink();
        if (snapshot.hasError) {
          return Padding(
            padding: const EdgeInsets.only(bottom: 16),
            child: Text('Hinweise aktuell nicht verfügbar', style: TextStyle(fontSize: 12, color: Colors.grey.shade700)),
          );
        }
        final notices = (snapshot.data ?? const []).whereType<Map>().map((e) => e.cast<String, dynamic>()).toList();
        if (notices.isEmpty) return const SizedBox.shrink();
        return Padding(
          padding: const EdgeInsets.only(bottom: 24),
          child: Card(
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Padding(
                    padding: EdgeInsets.symmetric(horizontal: 16, vertical: 4),
                    child: Text('Hinweise', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                  ),
                  for (final n in notices) _tile(context, n),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _tile(BuildContext context, Map<String, dynamic> n) {
    final kritisch = n['severity'] == 'kritisch';
    final color = kritisch ? const Color(0xFFB00020) : const Color(0xFFB26A00);
    final dest = noticeDestination(n['kind']?.toString() ?? '', widget.api);
    return ListTile(
      dense: true,
      leading: Icon(kritisch ? Icons.error_outline : Icons.schedule, color: color),
      title: Text(noticeLine(n), style: TextStyle(color: color)),
      trailing: dest == null ? null : const Icon(Icons.chevron_right),
      onTap: dest == null ? null : () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => dest)),
    );
  }
}
