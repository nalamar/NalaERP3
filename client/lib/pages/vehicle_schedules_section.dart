import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Termine eines Fahrzeugs (Backlog G.2.4.1) auf den Routen aus G.2.3: HU,
// UVV, Wartung, Versicherung, Leasingende, Sonstiges - mit Stand. Ein
// Termin ist Planung (Starttermin, Intervall, Notiz; aenderbar,
// stilllegbar); die Erledigungen sind Belege (G.2.4.2). Die HU ist im
// Monat der Plakette faellig und wird als MM.JJJJ eingegeben.
//
// Lesen mit fleet.read, schreiben mit fleet.write. Am abgemeldeten
// Fahrzeug wird nichts mehr geplant.

const terminArten = {
  'hu': 'Hauptuntersuchung',
  'uvv': 'UVV-Prüfung',
  'wartung': 'Wartung',
  'versicherung': 'Versicherung',
  'leasing': 'Leasingende',
  'sonstige': 'Sonstiges',
};

// scheduleTitle: Bezeichnung der Art, bei "sonstige" die eigene.
String scheduleTitle(Map<String, dynamic> s) {
  if (s['art'] == 'sonstige') return s['bezeichnung']?.toString() ?? '';
  return terminArten[s['art']] ?? (s['art']?.toString() ?? '');
}

// dueDateLabel: bei der HU der Monat ("05/2027"), sonst der Tag.
String dueDateLabel(String? art, String? iso) {
  final d = isoToGerman(iso);
  if (d.isEmpty || art != 'hu') return d;
  return d.substring(3).replaceAll('.', '/');
}

// terminstandLabel beschreibt den Stand eines Termins in Worten.
String terminstandLabel(String? art, Map<String, dynamic> st) {
  final due = dueDateLabel(art, st['faellig_am']?.toString());
  final tage = st['tage_bis_faellig'];
  final km = st['faellig_km'];
  final kmText = km is num ? ' oder bei ${km.toInt()} km' : '';
  switch (st['status']) {
    case 'gesperrt':
      final grund = st['letztes_ergebnis'] == 'maengel' ? 'mit Mängeln' : 'nicht bestanden';
      return due.isEmpty ? 'gesperrt – letzte Prüfung $grund' : 'gesperrt – letzte Prüfung $grund, Nachprüfung bis $due';
    case 'ueberfaellig':
      return 'überfällig seit $due';
    case 'faellig_bald':
      return tage == 0 ? 'heute fällig' : 'fällig ${art == 'hu' ? 'im' : 'am'} $due (in $tage Tagen)';
    case 'ok':
      return 'fällig ${art == 'hu' ? 'im' : 'am'} $due$kmText';
    case 'ohne_termin':
      return km is num ? 'fällig bei ${km.toInt()} km' : 'kein Termin';
    default:
      return st['status']?.toString() ?? '';
  }
}

bool terminstandIsBad(String? status) => status == 'gesperrt' || status == 'ueberfaellig';

// parseMonthInput liest einen Monat als MM.JJJJ, M.JJJJ oder MM/JJJJ und
// liefert den Ersten des Monats als JJJJ-MM-TT (der Server legt die HU auf
// das Monatsende). Ein volles Datum wird ebenfalls angenommen. null bei
// ungueltiger Eingabe.
String? parseMonthInput(String input) {
  final t = input.trim();
  final m = RegExp(r'^(\d{1,2})[./](\d{4})$').firstMatch(t);
  if (m != null) {
    final month = int.parse(m.group(1)!);
    if (month < 1 || month > 12) return null;
    return '${m.group(2)}-${month.toString().padLeft(2, '0')}-01';
  }
  return parseDateInput(t);
}

// parsePositiveInt: leer = keine Angabe; sonst ganze Zahl > 0
// (Tausenderpunkte erlaubt, z. B. "30.000").
({bool ok, int? value}) parsePositiveInt(String input) {
  final t = input.trim().replaceAll('.', '').replaceAll(' ', '');
  if (t.isEmpty) return (ok: true, value: null);
  final v = int.tryParse(t);
  if (v == null || v <= 0) return (ok: false, value: null);
  return (ok: true, value: v);
}

// buildScheduleCreateBody: fuer POST. intervall_monate null = Vorschlag der
// Art (HU 24, UVV und Versicherung 12 Monate).
Map<String, dynamic> buildScheduleCreateBody({
  required String vehicleId,
  required String art,
  String bezeichnung = '',
  String? starttermin,
  int? startKm,
  int? intervallMonate,
  int? intervallKm,
  String notiz = '',
}) =>
    {
      'vehicle_id': vehicleId,
      'art': art,
      'bezeichnung': art == 'sonstige' ? bezeichnung.trim() : '',
      'starttermin': starttermin,
      'start_km': startKm,
      'intervall_monate': intervallMonate,
      'intervall_km': intervallKm,
      'notiz': notiz.trim(),
    };

// buildScheduleUpdateBody: fuer PUT - immer alle sechs Felder (der Server
// verlangt sie; ein fehlendes "aktiv" legte den Termin sonst still).
Map<String, dynamic> buildScheduleUpdateBody({
  String? starttermin,
  int? startKm,
  int? intervallMonate,
  int? intervallKm,
  String notiz = '',
  required bool aktiv,
}) =>
    {
      'starttermin': starttermin,
      'start_km': startKm,
      'intervall_monate': intervallMonate,
      'intervall_km': intervallKm,
      'notiz': notiz.trim(),
      'aktiv': aktiv,
    };

class VehicleSchedulesSection extends StatefulWidget {
  const VehicleSchedulesSection({super.key, required this.api, required this.vehicleId, this.deregistered = false});
  final ApiClient api;
  final String vehicleId;
  final bool deregistered;

  @override
  State<VehicleSchedulesSection> createState() => _VehicleSchedulesSectionState();
}

class _VehicleSchedulesSectionState extends State<VehicleSchedulesSection> {
  List<dynamic> schedules = [];
  bool loading = false;
  bool showInactive = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('fleet.write') && !widget.deregistered;

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
      final list = await widget.api.listVehicleSchedules(widget.vehicleId, includeInactive: showInactive);
      if (mounted) setState(() => schedules = list);
    } catch (e) {
      if (mounted) setState(() => error = 'Termine konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  // _scheduleDialog: neuer Termin (schedule == null) oder Planung aendern.
  Future<void> _scheduleDialog({Map<String, dynamic>? schedule}) async {
    final editing = schedule != null;
    String art = schedule?['art']?.toString() ?? 'hu';
    final bezCtrl = TextEditingController(text: schedule?['bezeichnung']?.toString() ?? '');
    final startIso = schedule?['starttermin']?.toString();
    final startCtrl = TextEditingController(text: art == 'hu' ? dueDateLabel('hu', startIso).replaceAll('/', '.') : isoToGerman(startIso));
    final startKmCtrl = TextEditingController(text: schedule?['start_km']?.toString() ?? '');
    final ivMCtrl = TextEditingController(text: schedule?['intervall_monate']?.toString() ?? '');
    final ivKmCtrl = TextEditingController(text: schedule?['intervall_km']?.toString() ?? '');
    final notizCtrl = TextEditingController(text: schedule?['notiz']?.toString() ?? '');
    bool aktiv = schedule == null || schedule['aktiv'] != false;
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(editing ? '${scheduleTitle(schedule)} bearbeiten' : 'Neuer Termin'),
          content: SizedBox(
            width: 460,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (!editing)
                    DropdownButtonFormField<String>(
                      initialValue: art,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Art'),
                      items: [for (final e in terminArten.entries) DropdownMenuItem(value: e.key, child: Text(e.value))],
                      onChanged: (v) => setStateDialog(() => art = v ?? art),
                    ),
                  if (!editing && art == 'sonstige')
                    TextField(controller: bezCtrl, decoration: const InputDecoration(labelText: 'Bezeichnung (z. B. Tachograph)')),
                  TextField(
                    controller: startCtrl,
                    decoration: InputDecoration(
                      labelText: art == 'hu' ? 'Nächste HU (MM.JJJJ, laut Plakette)' : 'Nächster Termin (TT.MM.JJJJ, optional)',
                      helperText: editing ? 'Gilt bis zur ersten Erledigung.' : null,
                    ),
                  ),
                  TextField(
                    controller: ivMCtrl,
                    decoration: InputDecoration(
                      labelText: 'Intervall in Monaten',
                      helperText: editing ? 'Leer = kein Vorschlag für den Folgetermin.' : 'Leer = Vorschlag (HU 24, UVV und Versicherung 12).',
                    ),
                  ),
                  if (art == 'wartung') ...[
                    TextField(controller: startKmCtrl, decoration: const InputDecoration(labelText: 'Nächste Wartung bei km (optional)')),
                    TextField(controller: ivKmCtrl, decoration: const InputDecoration(labelText: 'Intervall in km (optional)')),
                  ],
                  TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
                  if (editing)
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      value: aktiv,
                      title: const Text('Aktiv'),
                      subtitle: const Text('Stillgelegte Termine nehmen keine Erledigungen an; die bisherigen bleiben.'),
                      onChanged: (v) => setStateDialog(() => aktiv = v == true),
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
                final startText = startCtrl.text.trim();
                final start = startText.isEmpty ? null : (art == 'hu' ? parseMonthInput(startText) : parseDateInput(startText));
                final ivM = parsePositiveInt(ivMCtrl.text);
                final startKm = parsePositiveInt(art == 'wartung' ? startKmCtrl.text : '');
                final ivKm = parsePositiveInt(art == 'wartung' ? ivKmCtrl.text : '');
                String? problem;
                if (!editing && art == 'sonstige' && bezCtrl.text.trim().isEmpty) {
                  problem = 'Bitte eine Bezeichnung angeben.';
                } else if (startText.isNotEmpty && start == null) {
                  problem = art == 'hu' ? 'Bitte die HU als MM.JJJJ angeben oder leer lassen.' : 'Bitte den Termin als TT.MM.JJJJ angeben oder leer lassen.';
                } else if (!ivM.ok) {
                  problem = 'Bitte das Intervall als ganze Zahl von Monaten angeben oder leer lassen.';
                } else if (!startKm.ok || !ivKm.ok) {
                  problem = 'Bitte Kilometer als ganze Zahl angeben oder leer lassen.';
                }
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(editing
                    ? buildScheduleUpdateBody(
                        starttermin: start,
                        startKm: startKm.value,
                        intervallMonate: ivM.value,
                        intervallKm: ivKm.value,
                        notiz: notizCtrl.text,
                        aktiv: aktiv,
                      )
                    : buildScheduleCreateBody(
                        vehicleId: widget.vehicleId,
                        art: art,
                        bezeichnung: bezCtrl.text,
                        starttermin: start,
                        startKm: startKm.value,
                        intervallMonate: ivM.value,
                        intervallKm: ivKm.value,
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
        await widget.api.updateSchedule(schedule['id'].toString(), body);
        _snack('Termin gespeichert');
      } else {
        await widget.api.createSchedule(body);
        _snack('Termin angelegt');
      }
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Expanded(child: Text('Termine', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold))),
            if (_canWrite)
              TextButton.icon(onPressed: () => _scheduleDialog(), icon: const Icon(Icons.event), label: const Text('Termin anlegen')),
          ],
        ),
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
        if (!loading && error == null && schedules.isEmpty) const Text('Keine Termine.'),
        for (final s in schedules)
          Builder(builder: (context) {
            final m = s as Map<String, dynamic>;
            final st = (m['standing'] as Map<String, dynamic>?) ?? const {};
            final art = m['art']?.toString();
            return ListTile(
              dense: true,
              contentPadding: EdgeInsets.zero,
              title: Text('${scheduleTitle(m)}${m['aktiv'] == false ? ' · stillgelegt' : ''}'),
              subtitle: Text(
                terminstandLabel(art, st),
                style: TextStyle(color: terminstandIsBad(st['status']?.toString()) ? const Color(0xFFB00020) : null),
              ),
              trailing: _canWrite
                  ? IconButton(tooltip: 'Termin bearbeiten', icon: const Icon(Icons.edit), onPressed: () => _scheduleDialog(schedule: m))
                  : null,
            );
          }),
      ],
    );
  }
}
