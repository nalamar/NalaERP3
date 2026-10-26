import 'package:flutter/material.dart';
import '../api.dart';
import 'vehicle_detail_page.dart';
import 'vehicle_schedules_section.dart' show scheduleTitle, terminstandLabel;

// Faellige Fahrzeugtermine (Backlog G.2.4.3), Route aus G.2.3: alle
// aktiven Termine nicht abgemeldeter Fahrzeuge, die gesperrt, ueberfaellig
// oder innerhalb der Vorwarnzeit faellig sind - die Arbeitsliste des
// Fuhrparks. Eine Zeile oeffnet das Fahrzeug. Nur fleet.read.

const defaultScheduleWarnDays = 30;

// dueVehicleLabel: "M-AB 1234 – Montagebus · Hauptuntersuchung".
String dueVehicleLabel(Map<String, dynamic> e) {
  final bez = e['vehicle_bezeichnung']?.toString() ?? '';
  return '${e['kennzeichen'] ?? ''}${bez.isEmpty ? '' : ' – $bez'} · ${scheduleTitle(e)}';
}

// dueStatusCounts zaehlt die Eintraege je Stand.
Map<String, int> dueStatusCounts(List<dynamic> entries) {
  final out = {'gesperrt': 0, 'ueberfaellig': 0, 'faellig_bald': 0};
  for (final e in entries) {
    final st = ((e as Map<String, dynamic>)['standing'] as Map<String, dynamic>?)?['status']?.toString();
    if (st != null && out.containsKey(st)) out[st] = out[st]! + 1;
  }
  return out;
}

class FleetDuePage extends StatefulWidget {
  const FleetDuePage({super.key, required this.api});
  final ApiClient api;

  @override
  State<FleetDuePage> createState() => _FleetDuePageState();
}

class _FleetDuePageState extends State<FleetDuePage> {
  List<dynamic> entries = [];
  bool loading = false;
  String? error;
  int warnDays = defaultScheduleWarnDays;
  final warnCtrl = TextEditingController(text: '$defaultScheduleWarnDays');

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
      final list = await widget.api.listScheduleDue(warnDays: warnDays);
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

  Future<void> _openVehicle(String vehicleId) async {
    Map<String, dynamic> vehicle;
    try {
      vehicle = await widget.api.getVehicle(vehicleId);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      return;
    }
    if (!mounted) return;
    await Navigator.of(context).push(MaterialPageRoute(builder: (_) => VehicleDetailPage(api: widget.api, vehicle: vehicle)));
    if (mounted) _load();
  }

  @override
  Widget build(BuildContext context) {
    final counts = dueStatusCounts(entries);
    return Scaffold(
      appBar: AppBar(
        title: const Text('Fällige Fahrzeugtermine'),
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
                if (entries.isNotEmpty)
                  Text('gesperrt: ${counts['gesperrt']} · überfällig: ${counts['ueberfaellig']} · bald fällig: ${counts['faellig_bald']}'),
              ],
            ),
            const SizedBox(height: 12),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && entries.isEmpty) Text('Nichts fällig – alle Termine liegen weiter als $warnDays Tage entfernt.'),
            for (final e in entries)
              Builder(builder: (context) {
                final m = e as Map<String, dynamic>;
                final st = (m['standing'] as Map<String, dynamic>?) ?? const {};
                return ListTile(
                  dense: true,
                  onTap: () => _openVehicle(m['vehicle_id'].toString()),
                  title: Text(dueVehicleLabel(m)),
                  subtitle: Text(terminstandLabel(m['art']?.toString(), st),
                      style: TextStyle(color: st['status'] == 'faellig_bald' ? const Color(0xFFB26A00) : const Color(0xFFB00020))),
                );
              }),
          ],
        ),
      ),
    );
  }
}
