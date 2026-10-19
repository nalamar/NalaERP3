import 'package:flutter/material.dart';
import '../api.dart';
import 'employee_qualifications_section.dart' show qualificationStatusLabel;

// Faellige Qualifikationen (Backlog F.13.2): die betriebsweite
// Arbeitsliste der Personalabteilung zum Anmahnen (Vorgabe des Nutzers vom
// 2026-10-13: "Eine anstehende Erneuerung sollte vom System angemahnt
// werden."). Route aus F.2.6.2, nur mit qualifications.read; der Server
// sortiert (fehlt, abgelaufen, laeuft ab; darin das Dringendste zuerst).

const defaultQualificationWarnDays = 60;

// dueCounts zaehlt die Eintraege je Stand.
Map<String, int> dueCounts(List<dynamic> entries) {
  final out = <String, int>{'fehlt': 0, 'abgelaufen': 0, 'laeuft_ab': 0};
  for (final e in entries) {
    final s = (e as Map<String, dynamic>)['status']?.toString() ?? '';
    out[s] = (out[s] ?? 0) + 1;
  }
  return out;
}

// dueEmployeeLabel: "Berg, Anna (P-17)" bzw. ohne Personalnummer.
String dueEmployeeLabel(Map<String, dynamic> e) {
  final name = '${e['last_name'] ?? ''}, ${e['first_name'] ?? ''}';
  final pn = e['personalnummer']?.toString() ?? '';
  return pn.isEmpty ? name : '$name ($pn)';
}

class QualificationsDuePage extends StatefulWidget {
  const QualificationsDuePage({super.key, required this.api});
  final ApiClient api;

  @override
  State<QualificationsDuePage> createState() => _QualificationsDuePageState();
}

class _QualificationsDuePageState extends State<QualificationsDuePage> {
  List<dynamic> entries = [];
  bool loading = false;
  String? error;
  int warnDays = defaultQualificationWarnDays;
  final warnCtrl = TextEditingController(text: '$defaultQualificationWarnDays');

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    warnCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final list = await widget.api.listQualificationsDue(warnDays: warnDays);
      if (mounted) setState(() => entries = list);
    } catch (e) {
      if (mounted) setState(() => error = 'Liste konnte nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _applyWarnDays() {
    final v = int.tryParse(warnCtrl.text.trim());
    if (v == null || v < 0) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Bitte die Vorwarnzeit als ganze Zahl von Tagen angeben (0 oder mehr).')));
      return;
    }
    setState(() => warnDays = v);
    _load();
  }

  Color _color(String? status) {
    switch (status) {
      case 'fehlt':
      case 'abgelaufen':
        return const Color(0xFFB00020);
      default:
        return const Color(0xFFB26A00);
    }
  }

  @override
  Widget build(BuildContext context) {
    final counts = dueCounts(entries);
    return Scaffold(
      appBar: AppBar(
        title: const Text('Fällige Qualifikationen'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
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
                  width: 220,
                  child: TextField(
                    controller: warnCtrl,
                    decoration: const InputDecoration(labelText: 'Vorwarnzeit (Tage)'),
                    onSubmitted: (_) => _applyWarnDays(),
                  ),
                ),
                OutlinedButton(onPressed: _applyWarnDays, child: const Text('Anwenden')),
                if (!loading && error == null)
                  Text('fehlt: ${counts['fehlt']} · abgelaufen: ${counts['abgelaufen']} · läuft ab: ${counts['laeuft_ab']}'),
              ],
            ),
            const SizedBox(height: 12),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && entries.isEmpty)
              Text('Nichts fällig – alle erforderlichen Qualifikationen gelten länger als $warnDays Tage.'),
            for (final e in entries)
              Builder(builder: (context) {
                final m = e as Map<String, dynamic>;
                return ListTile(
                  dense: true,
                  title: Text('${dueEmployeeLabel(m)} – ${m['name'] ?? m['code'] ?? ''}'),
                  subtitle: Text(qualificationStatusLabel(m), style: TextStyle(color: _color(m['status']?.toString()))),
                );
              }),
          ],
        ),
      ),
    );
  }
}
