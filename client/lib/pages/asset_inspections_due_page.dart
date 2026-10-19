import 'package:flutter/material.dart';
import '../api.dart';
import 'asset_inspections_section.dart' show pruefstandLabel;

// Faellige Pruefungen (Backlog F.13.4.2), Route aus F.3.6.3: alle nicht
// ausgemusterten Betriebsmittel, die gesperrt, ueberfaellig, nie geprueft
// oder bald faellig sind - mit dem Mitarbeiter, der das Stueck gerade hat
// (dort muss es zur Pruefung abgeholt werden). Nur assets.read.

const defaultInspectionWarnDays = 30;

// dueAssetLabel: "E-17 – Bohrhammer · Halle 2 · bei Anna Berg".
String dueAssetLabel(Map<String, dynamic> e) {
  final parts = <String>['${e['inventarnummer'] ?? ''} – ${e['bezeichnung'] ?? ''}'];
  final standort = e['standort']?.toString() ?? '';
  if (standort.isNotEmpty) parts.add(standort);
  final halter = e['mitarbeiter_name']?.toString() ?? '';
  if (halter.isNotEmpty) parts.add('bei $halter');
  return parts.join(' · ');
}

class AssetInspectionsDuePage extends StatefulWidget {
  const AssetInspectionsDuePage({super.key, required this.api});
  final ApiClient api;

  @override
  State<AssetInspectionsDuePage> createState() => _AssetInspectionsDuePageState();
}

class _AssetInspectionsDuePageState extends State<AssetInspectionsDuePage> {
  List<dynamic> entries = [];
  bool loading = false;
  String? error;
  int warnDays = defaultInspectionWarnDays;
  final warnCtrl = TextEditingController(text: '$defaultInspectionWarnDays');

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
      final list = await widget.api.listAssetInspectionsDue(warnDays: warnDays);
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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Fällige Prüfungen'),
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
              ],
            ),
            const SizedBox(height: 12),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && entries.isEmpty) Text('Nichts fällig – alle Prüfungen gelten länger als $warnDays Tage.'),
            for (final e in entries)
              ListTile(
                dense: true,
                title: Text(dueAssetLabel(e as Map<String, dynamic>)),
                subtitle: Text(pruefstandLabel(e),
                    style: TextStyle(color: e['status'] == 'faellig_bald' ? const Color(0xFFB26A00) : const Color(0xFFB00020))),
              ),
          ],
        ),
      ),
    );
  }
}
