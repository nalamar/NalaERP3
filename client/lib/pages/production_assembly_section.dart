import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;

// Montagetermine im Detail des Fertigungsauftrags (Backlog H.3.6e, ADR 0038
// Frage 6 (a)) auf den Routen aus H.3.6d: Liste nach Beginn mit Stornos und
// Hinweisen (production.read). Hinweise sind keine Sperren; die Abwesenheit
// im Team nennt keine Person.

const assemblyHintLabels = {
  'team_doppelt': 'Team ist im Zeitraum doppelt eingeplant',
  'team_abwesend': 'Abwesenheit im Team',
  'fahrzeug_gesperrt': 'Fahrzeug ist gesperrt (HU/UVV)',
  'vor_fertigstellung': 'Montage vor der voraussichtlichen Fertigstellung',
  'nach_liefertermin': 'Montage endet nach dem Liefertermin',
};

bool assemblyCancelled(Map<String, dynamic> a) => a['storniert_at'] != null;

// assemblyDays: Kalendertage von bis bis einschließlich, 0 ohne Datum.
int assemblyDays(Map<String, dynamic> a) {
  final von = DateTime.tryParse(a['von']?.toString() ?? '');
  final bis = DateTime.tryParse(a['bis']?.toString() ?? '');
  if (von == null || bis == null) return 0;
  final v = DateTime.utc(von.year, von.month, von.day);
  final b = DateTime.utc(bis.year, bis.month, bis.day);
  return b.difference(v).inDays + 1;
}

// assemblyRange: "04.03.2030 – 06.03.2030 (3 Tage)" oder "04.03.2030 (1 Tag)".
String assemblyRange(Map<String, dynamic> a) {
  final von = isoToGerman(a['von']?.toString());
  final bis = isoToGerman(a['bis']?.toString());
  final n = assemblyDays(a);
  return '${von == bis ? von : '$von – $bis'} ($n ${n == 1 ? 'Tag' : 'Tage'})';
}

// assemblyLine: "04.03.2030 – 06.03.2030 (3 Tage) · Montage Nord · MA-1 · Hafenstr. 2".
String assemblyLine(Map<String, dynamic> a) {
  final team = a['team_name']?.toString() ?? '';
  return [
    assemblyRange(a),
    team.isEmpty ? 'ohne Team' : team,
    a['kennzeichen']?.toString() ?? '',
    a['baustelle']?.toString() ?? '',
  ].where((x) => x.isNotEmpty).join(' · ');
}

// assemblyHints: Klartext der Hinweise; unbekannte Codes unverändert.
List<String> assemblyHints(Map<String, dynamic> a) =>
    [for (final h in (a['hinweise'] as List?) ?? const []) assemblyHintLabels[h] ?? h.toString()];

// assemblyDetails: Hinweise, Notiz und Storno mit Datum und Begründung,
// sonst null.
String? assemblyDetails(Map<String, dynamic> a) {
  final notiz = a['notiz']?.toString() ?? '';
  final rows = [
    for (final h in assemblyHints(a)) 'Hinweis: $h',
    if (notiz.isNotEmpty) 'Notiz: $notiz',
    if (assemblyCancelled(a)) 'Storniert ${isoToGerman(a['storniert_at']?.toString())}: ${a['storno_reason'] ?? ''}',
  ];
  return rows.isEmpty ? null : rows.join('\n');
}

class ProductionAssemblySection extends StatefulWidget {
  const ProductionAssemblySection({super.key, required this.api, required this.orderId, required this.status});
  final ApiClient api;
  final String orderId;
  final String? status;

  @override
  State<ProductionAssemblySection> createState() => _ProductionAssemblySectionState();
}

class _ProductionAssemblySectionState extends State<ProductionAssemblySection> {
  List<dynamic> list = const [];
  bool loading = false;
  String? error;

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
      final l = await widget.api.listProductionAssembly(widget.orderId);
      if (mounted) setState(() => list = l);
    } catch (e) {
      if (mounted) setState(() => error = 'Montagetermine konnten nicht geladen werden: ${e is ApiException ? e.message : e}');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Montage', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
        if (loading) const LinearProgressIndicator(minHeight: 2),
        if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (!loading && error == null && list.isEmpty) const Text('Noch keine Montagetermine.'),
        for (final x in list.whereType<Map<String, dynamic>>())
          ListTile(
            dense: true,
            contentPadding: EdgeInsets.zero,
            leading: assemblyCancelled(x)
                ? const Icon(Icons.event_busy_outlined, color: Colors.black38)
                : assemblyHints(x).isEmpty
                    ? const Icon(Icons.event_available_outlined)
                    : const Icon(Icons.warning_amber_rounded, color: Color(0xFFB26A00)),
            title: Text(
              assemblyLine(x),
              style: assemblyCancelled(x) ? const TextStyle(decoration: TextDecoration.lineThrough, color: Colors.black54) : null,
            ),
            subtitle: assemblyDetails(x) == null ? null : Text(assemblyDetails(x)!),
          ),
      ],
    );
  }
}
