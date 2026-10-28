import 'package:flutter/material.dart';
import '../api.dart';
import 'asset_detail_page.dart' show activeEmployees;
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Fahrerzuordnung auf der Detailseite eines Fahrzeugs (Backlog G.3a.3,
// Routen G.3a.2): wer fährt es, wer fuhr es; zuordnen, beenden,
// stornieren. Muster der Ausgabe von Betriebsmitteln (asset_detail_page).
//
// Eine HU/UVV mit Mängeln oder nicht bestanden sperrt die Zuordnung - die
// Ablehnung des Servers erscheint als Meldung. Hinweise (Termin
// überfällig, keine gültige Führerscheinkontrolle) zeigt ein Dialog nach
// dem Zuordnen.

String _two(int v) => v.toString().padLeft(2, '0');

// vehicleAssignmentLine beschreibt eine Zuordnung: "Anna Berg · seit
// 01.03.2026" bzw. "Anna Berg · 01.03.2026 – 10.03.2026".
String vehicleAssignmentLine(Map<String, dynamic> a) {
  final name = a['mitarbeiter_name']?.toString() ?? '';
  final von = isoToGerman(a['beginn']?.toString());
  final bis = isoToGerman(a['ende']?.toString());
  final notiz = a['notiz']?.toString() ?? '';
  final base = bis.isEmpty ? '$name · seit $von' : '$name · $von – $bis';
  return notiz.isEmpty ? base : '$base ($notiz)';
}

// openVehicleAssignment: die offene (nicht beendete, nicht stornierte)
// Zuordnung - höchstens eine, das garantiert die Datenbank.
Map<String, dynamic>? openVehicleAssignment(List<dynamic> assignments) {
  for (final a in assignments) {
    final m = a as Map<String, dynamic>;
    if (m['status'] == 'active' && m['ende'] == null) return m;
  }
  return null;
}

class VehicleAssignmentsSection extends StatefulWidget {
  const VehicleAssignmentsSection({super.key, required this.api, required this.vehicleId, this.deregistered = false, this.now});
  final ApiClient api;
  final String vehicleId;
  final bool deregistered;
  final DateTime Function()? now;

  @override
  State<VehicleAssignmentsSection> createState() => _VehicleAssignmentsSectionState();
}

class _VehicleAssignmentsSectionState extends State<VehicleAssignmentsSection> {
  List<dynamic> assignments = [];
  bool loading = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('fleet.write');
  DateTime _now() => (widget.now ?? DateTime.now)();
  String _today() {
    final t = _now();
    return '${_two(t.day)}.${_two(t.month)}.${t.year}';
  }

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
      final list = await widget.api.listFleetAssignments(vehicleId: widget.vehicleId);
      if (mounted) setState(() => assignments = list);
    } catch (e) {
      if (mounted) setState(() => error = 'Fahrerzuordnungen konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Widget _errorText(String? e) => e == null
      ? const SizedBox.shrink()
      : Padding(padding: const EdgeInsets.only(top: 8), child: Text(e, style: const TextStyle(color: Color(0xFFB00020))));

  Future<void> _assignDialog() async {
    List<Map<String, dynamic>> employees;
    try {
      employees = activeEmployees(await widget.api.listEmployees(limit: 500));
    } catch (e) {
      _snack('Mitarbeiterliste konnte nicht geladen werden: $e');
      return;
    }
    if (!mounted) return;
    if (employees.isEmpty) {
      _snack('Es gibt keine aktiven Mitarbeiter.');
      return;
    }
    String? employeeId;
    final beginnCtrl = TextEditingController(text: _today());
    final notizCtrl = TextEditingController();
    String? dialogError;
    final result = await showDialog<({String employeeId, String beginn, String notiz})>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Fahrer zuordnen'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                DropdownButtonFormField<String>(
                  initialValue: employeeId,
                  isExpanded: true,
                  decoration: const InputDecoration(labelText: 'Mitarbeiter'),
                  items: [
                    for (final e in employees)
                      DropdownMenuItem(value: e['id'].toString(), child: Text('${e['last_name'] ?? ''}, ${e['first_name'] ?? ''}')),
                  ],
                  onChanged: (v) => setStateDialog(() => employeeId = v),
                ),
                TextField(controller: beginnCtrl, decoration: const InputDecoration(labelText: 'Beginn (TT.MM.JJJJ)')),
                TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
                _errorText(dialogError),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final beginn = parseDateInput(beginnCtrl.text);
                if (employeeId == null) {
                  setStateDialog(() => dialogError = 'Bitte einen Mitarbeiter auswählen.');
                  return;
                }
                if (beginn == null) {
                  setStateDialog(() => dialogError = 'Bitte das Datum als TT.MM.JJJJ angeben.');
                  return;
                }
                Navigator.of(ctx).pop((employeeId: employeeId!, beginn: beginn, notiz: notizCtrl.text.trim()));
              },
              child: const Text('Zuordnen'),
            ),
          ],
        ),
      ),
    );
    if (result == null) return;
    try {
      final out = await widget.api.assignVehicle(vehicleId: widget.vehicleId, employeeId: result.employeeId, beginn: result.beginn, notiz: result.notiz);
      final hints = ((out['hinweise'] as List?) ?? const []).map((h) => h.toString()).toList();
      await _load();
      if (!mounted) return;
      if (hints.isEmpty) {
        _snack('Zugeordnet');
        return;
      }
      // Hinweise nicht in einer flüchtigen Meldung verstecken.
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Zugeordnet – bitte beachten'),
          content: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            for (final h in hints) Text('⚠ $h'),
          ]),
          actions: [FilledButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('OK'))],
        ),
      );
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _endDialog(Map<String, dynamic> open) async {
    final endeCtrl = TextEditingController(text: _today());
    String? dialogError;
    final ende = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Zuordnung beenden'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(vehicleAssignmentLine(open)),
                TextField(controller: endeCtrl, decoration: const InputDecoration(labelText: 'Ende (TT.MM.JJJJ)')),
                _errorText(dialogError),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final d = parseDateInput(endeCtrl.text);
                if (d == null) {
                  setStateDialog(() => dialogError = 'Bitte das Datum als TT.MM.JJJJ angeben.');
                  return;
                }
                Navigator.of(ctx).pop(d);
              },
              child: const Text('Beenden'),
            ),
          ],
        ),
      ),
    );
    if (ende == null) return;
    try {
      await widget.api.endFleetAssignment(open['id'].toString(), ende);
      _snack('Zuordnung beendet');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _stornoDialog(Map<String, dynamic> a) async {
    final reasonCtrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Zuordnung stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(vehicleAssignmentLine(a)),
                const SizedBox(height: 8),
                const Text('Nur für falsch erfasste Zuordnungen. Sie wird nicht gelöscht, sondern mit Begründung storniert.'),
                TextField(
                  controller: reasonCtrl,
                  decoration: const InputDecoration(labelText: 'Begründung'),
                  onChanged: (_) => setStateDialog(() {}),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: reasonCtrl.text.trim().isEmpty ? null : () => Navigator.of(ctx).pop(reasonCtrl.text.trim()),
              child: const Text('Stornieren'),
            ),
          ],
        ),
      ),
    );
    if (reason == null) return;
    try {
      await widget.api.stornoFleetAssignment(a['id'].toString(), reason);
      _snack('Zuordnung storniert');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final open = openVehicleAssignment(assignments);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Fahrer', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
        if (loading) const LinearProgressIndicator(minHeight: 2),
        if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (!loading && error == null) ...[
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 8),
            child: Row(
              children: [
                Expanded(child: Text(open != null ? 'Zugeordnet: ${vehicleAssignmentLine(open)}' : 'Kein fester Fahrer')),
                if (_canWrite && open != null)
                  FilledButton.tonal(onPressed: () => _endDialog(open), child: const Text('Beenden')),
                // Ein abgemeldetes Fahrzeug bekommt keinen Fahrer mehr.
                if (_canWrite && open == null && !widget.deregistered)
                  FilledButton(onPressed: _assignDialog, child: const Text('Fahrer zuordnen')),
              ],
            ),
          ),
          if (assignments.isEmpty) const Text('Noch nie zugeordnet.'),
          for (final x in assignments)
            Row(
              children: [
                Expanded(child: Text(vehicleAssignmentLine(x as Map<String, dynamic>))),
                // Stornieren auch am abgemeldeten Fahrzeug (Berichtigung).
                if (_canWrite)
                  IconButton(tooltip: 'Zuordnung stornieren', icon: const Icon(Icons.block, size: 18), onPressed: () => _stornoDialog(x)),
              ],
            ),
        ],
      ],
    );
  }
}
