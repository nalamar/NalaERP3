import 'package:flutter/material.dart';
import '../api.dart';
import 'asset_inspections_section.dart';
import 'assets_page.dart' show assetKategorieLabel, intervallLabel, isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Detailseite eines Betriebsmittels (Backlog F.13.4.1): wer hat es, wer
// hatte es; ausgeben, zuruecknehmen, stornieren. Routen aus F.3.6.2.
// Lesen mit assets.read, schreiben mit assets.write. Ausgaben werden nie
// geaendert oder geloescht - nur zurueckgenommen oder storniert (F.3.4).
//
// Ein gesperrtes Stueck (letzte Pruefung nicht bestanden oder Maengel)
// lehnt der Server ab; die Meldung erscheint hier. Hinweise (Pruefung
// ueberfaellig, nie geprueft) zeigt die Seite nach der Ausgabe an.

String _two(int v) => v.toString().padLeft(2, '0');

// assignmentLine beschreibt eine Ausgabe: "Anna Berg · 05.01.2026 –
// 10.01.2026 (Holm verbogen)" bzw. "… · seit 05.01.2026".
String assignmentLine(Map<String, dynamic> a) {
  final name = a['mitarbeiter_name']?.toString() ?? '';
  final von = isoToGerman(a['ausgegeben_am']?.toString());
  final bis = isoToGerman(a['zurueckgegeben_am']?.toString());
  if (bis.isEmpty) return '$name · seit $von';
  final zustand = a['rueckgabe_zustand']?.toString() ?? '';
  return '$name · $von – $bis${zustand.isEmpty ? '' : ' ($zustand)'}';
}

// openAssignment: die offene (nicht zurueckgegebene, nicht stornierte)
// Ausgabe - hoechstens eine, das garantiert die Datenbank.
Map<String, dynamic>? openAssignment(List<dynamic> assignments) {
  for (final a in assignments) {
    final m = a as Map<String, dynamic>;
    if (m['status'] == 'active' && m['zurueckgegeben_am'] == null) return m;
  }
  return null;
}

// activeEmployees: nur aktive Mitarbeiter kommen fuer eine Ausgabe in Frage
// (der Server lehnt inaktive ab), nach Nachname sortiert.
List<Map<String, dynamic>> activeEmployees(List<dynamic> employees) {
  final out = [
    for (final e in employees)
      if ((e as Map<String, dynamic>)['active'] != false) e,
  ];
  out.sort((a, b) => '${a['last_name']} ${a['first_name']}'.toLowerCase().compareTo('${b['last_name']} ${b['first_name']}'.toLowerCase()));
  return out;
}

class AssetDetailPage extends StatefulWidget {
  const AssetDetailPage({super.key, required this.api, required this.asset, this.now});
  final ApiClient api;
  final Map<String, dynamic> asset;
  final DateTime Function()? now;

  @override
  State<AssetDetailPage> createState() => _AssetDetailPageState();
}

class _AssetDetailPageState extends State<AssetDetailPage> {
  List<dynamic> assignments = [];
  bool loading = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('assets.write');
  bool get _retired => widget.asset['ausgemustert_am'] != null;
  String get _id => widget.asset['id'].toString();
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
      final list = await widget.api.listAssetAssignments(assetId: _id);
      if (mounted) setState(() => assignments = list);
    } catch (e) {
      if (mounted) setState(() => error = 'Ausgaben konnten nicht geladen werden: $e');
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

  Future<void> _issueDialog() async {
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
    final datumCtrl = TextEditingController(text: _today());
    final notizCtrl = TextEditingController();
    String? dialogError;
    final result = await showDialog<({String employeeId, String datum, String notiz})>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Ausgeben'),
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
                TextField(controller: datumCtrl, decoration: const InputDecoration(labelText: 'Ausgegeben am (TT.MM.JJJJ)')),
                TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
                _errorText(dialogError),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final datum = parseDateInput(datumCtrl.text);
                if (employeeId == null) {
                  setStateDialog(() => dialogError = 'Bitte einen Mitarbeiter auswählen.');
                  return;
                }
                if (datum == null) {
                  setStateDialog(() => dialogError = 'Bitte das Datum als TT.MM.JJJJ angeben.');
                  return;
                }
                Navigator.of(ctx).pop((employeeId: employeeId!, datum: datum, notiz: notizCtrl.text.trim()));
              },
              child: const Text('Ausgeben'),
            ),
          ],
        ),
      ),
    );
    if (result == null) return;
    try {
      final out = await widget.api.issueAsset(assetId: _id, employeeId: result.employeeId, ausgegebenAm: result.datum, notiz: result.notiz);
      final hints = ((out['hinweise'] as List?) ?? const []).map((h) => h.toString()).toList();
      await _load();
      if (!mounted) return;
      if (hints.isEmpty) {
        _snack('Ausgegeben');
        return;
      }
      // Hinweise nicht in einer fluechtigen Meldung verstecken.
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Ausgegeben – bitte beachten'),
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

  Future<void> _returnDialog(Map<String, dynamic> open) async {
    final datumCtrl = TextEditingController(text: _today());
    final zustandCtrl = TextEditingController();
    String? dialogError;
    final result = await showDialog<({String datum, String zustand})>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Zurücknehmen'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(assignmentLine(open)),
                TextField(controller: datumCtrl, decoration: const InputDecoration(labelText: 'Zurückgegeben am (TT.MM.JJJJ)')),
                TextField(controller: zustandCtrl, decoration: const InputDecoration(labelText: 'Zustand (optional)')),
                _errorText(dialogError),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final datum = parseDateInput(datumCtrl.text);
                if (datum == null) {
                  setStateDialog(() => dialogError = 'Bitte das Datum als TT.MM.JJJJ angeben.');
                  return;
                }
                Navigator.of(ctx).pop((datum: datum, zustand: zustandCtrl.text.trim()));
              },
              child: const Text('Zurücknehmen'),
            ),
          ],
        ),
      ),
    );
    if (result == null) return;
    try {
      await widget.api.returnAsset(open['id'].toString(), datum: result.datum, zustand: result.zustand);
      _snack('Zurückgenommen');
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
          title: const Text('Ausgabe stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(assignmentLine(a)),
                const SizedBox(height: 8),
                const Text('Nur für falsch erfasste Ausgaben. Sie wird nicht gelöscht, sondern mit Begründung storniert.'),
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
      await widget.api.stornoAssetAssignment(a['id'].toString(), reason);
      _snack('Ausgabe storniert');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final a = widget.asset;
    final open = openAssignment(assignments);
    final details = [
      assetKategorieLabel(a['kategorie']?.toString()),
      if ((a['standort']?.toString() ?? '').isNotEmpty) 'Standort ${a['standort']}',
      intervallLabel(a['pruefintervall_monate']),
      if ((a['seriennummer']?.toString() ?? '').isNotEmpty) 'SN ${a['seriennummer']}',
    ].join(' · ');
    return Scaffold(
      appBar: AppBar(
        title: Text('${a['inventarnummer'] ?? ''} – ${a['bezeichnung'] ?? ''}'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Text(details),
            if (_retired)
              Text('Ausgemustert am ${isoToGerman(a['ausgemustert_am']?.toString())} (${a['ausgemustert_grund'] ?? ''})',
                  style: const TextStyle(color: Color(0xFFB00020))),
            const Divider(height: 32),
            const Text('Ausgaben', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null) ...[
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 8),
                child: Row(
                  children: [
                    Expanded(child: Text(open != null ? 'Ausgegeben an ${assignmentLine(open)}' : 'Nicht ausgegeben')),
                    if (_canWrite && open != null)
                      FilledButton.tonal(onPressed: () => _returnDialog(open), child: const Text('Zurücknehmen')),
                    if (_canWrite && open == null && !_retired)
                      FilledButton(onPressed: _issueDialog, child: const Text('Ausgeben')),
                  ],
                ),
              ),
              if (assignments.isEmpty) const Text('Noch nie ausgegeben.'),
              for (final x in assignments)
                Row(
                  children: [
                    Expanded(child: Text(assignmentLine(x as Map<String, dynamic>))),
                    if (_canWrite)
                      IconButton(tooltip: 'Stornieren', icon: const Icon(Icons.block, size: 18), onPressed: () => _stornoDialog(x)),
                  ],
                ),
            ],
            const Divider(height: 32),
            AssetInspectionsSection(api: widget.api, assetId: _id, retired: _retired, now: widget.now),
          ],
        ),
      ),
    );
  }
}
