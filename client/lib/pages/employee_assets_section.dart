import 'package:flutter/material.dart';
import '../api.dart';

// Ausgegebene Betriebsmittel am Mitarbeiter (Backlog F.7.4): was hat er
// gerade? Nur lesend, mit assets.read; ausgeben und zuruecknehmen gehoert
// in die Betriebsmittel-Oberflaeche.

String _date(String? iso) {
  if (iso == null || iso.length < 10) return '';
  final p = iso.substring(0, 10).split('-');
  return p.length == 3 ? '${p[2]}.${p[1]}.${p[0]}' : iso;
}

// assetAssignmentLabel beschreibt eine offene Ausgabe in einer Zeile.
String assetAssignmentLabel(Map<String, dynamic> a) {
  final nr = a['inventarnummer']?.toString() ?? '';
  final name = a['bezeichnung']?.toString() ?? '';
  return '$nr – $name, seit ${_date(a['ausgegeben_am']?.toString())}';
}

class EmployeeAssetsSection extends StatefulWidget {
  const EmployeeAssetsSection({super.key, required this.api, required this.employeeId});
  final ApiClient api;
  final String employeeId;

  @override
  State<EmployeeAssetsSection> createState() => _EmployeeAssetsSectionState();
}

class _EmployeeAssetsSectionState extends State<EmployeeAssetsSection> {
  List<dynamic> open = [];
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
      final list = await widget.api.listAssetAssignments(employeeId: widget.employeeId, nurOffen: true);
      if (mounted) setState(() => open = list);
    } catch (e) {
      if (mounted) setState(() => error = 'Betriebsmittel konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Ausgegebene Betriebsmittel', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
        if (loading) const LinearProgressIndicator(minHeight: 2),
        if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (!loading && error == null && open.isEmpty) const Text('Keine Betriebsmittel ausgegeben.'),
        for (final a in open) Text(assetAssignmentLabel(a as Map<String, dynamic>), overflow: TextOverflow.ellipsis),
      ],
    );
  }
}
