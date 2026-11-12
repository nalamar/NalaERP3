import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Montagetermine im Detail des Fertigungsauftrags (Backlog H.3.6e, ADR 0038
// Frage 6 (a)) auf den Routen aus H.3.6d: Liste nach Beginn mit Stornos und
// Hinweisen (production.read); mit production.write anlegen und ändern
// (nicht bei stornierten Aufträgen) und stornieren (nur mit Begründung).
// Hinweise sind keine Sperren; die Abwesenheit im Team nennt keine Person.

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

// assemblyAllowed: neue und geänderte Termine für jeden Auftrag außer
// stornierten (wie auf dem Server); stornieren geht immer.
bool assemblyAllowed(String? status) => status != 'storniert';

// buildAssemblyBody prüft die Eingaben wie der Server oder liefert eine
// Meldung: Beginn und Ende Pflicht, Ende nicht vor Beginn, höchstens 60
// Tage, Baustelle bis 300 und Notiz bis 500 Zeichen. Team und Fahrzeug
// leer = ohne.
({Map<String, dynamic>? body, String? error}) buildAssemblyBody({
  required String von,
  required String bis,
  String? teamId,
  String? vehicleId,
  String baustelle = '',
  String notiz = '',
}) {
  final v = parseDateInput(von), b = parseDateInput(bis);
  if (v == null || b == null) return (body: null, error: 'Bitte Beginn und Ende als TT.MM.JJJJ angeben.');
  final days = assemblyDays({'von': v, 'bis': b});
  if (days < 1) return (body: null, error: 'Das Ende liegt vor dem Beginn.');
  if (days > 60) return (body: null, error: 'Montage höchstens 60 Tage.');
  if (baustelle.trim().runes.length > 300) return (body: null, error: 'Baustelle: höchstens 300 Zeichen.');
  if (notiz.trim().runes.length > 500) return (body: null, error: 'Notiz: höchstens 500 Zeichen.');
  return (
    body: {
      'von': v,
      'bis': b,
      if (teamId != null && teamId.isNotEmpty) 'team_id': teamId,
      if (vehicleId != null && vehicleId.isNotEmpty) 'vehicle_id': vehicleId,
      'baustelle': baustelle.trim(),
      'notiz': notiz.trim(),
    },
    error: null,
  );
}

// assemblyCancelReasonError: Meldung zur Begründung des Stornos oder null.
String? assemblyCancelReasonError(String reason) {
  final r = reason.trim();
  if (r.isEmpty) return 'Bitte eine Begründung angeben.';
  if (r.runes.length > 500) return 'Begründung: höchstens 500 Zeichen.';
  return null;
}

// assemblyVehicleLabel: "MA-3 Sprinter (abgemeldet ab 31.01.2031)".
String assemblyVehicleLabel(Map<String, dynamic> v) {
  final ab = isoToGerman(v['abgemeldet_am']?.toString());
  return [
    [v['kennzeichen'], v['bezeichnung']].map((x) => x?.toString() ?? '').where((x) => x.isNotEmpty).join(' '),
    if (ab.isNotEmpty) '(abgemeldet ab $ab)',
  ].join(' ');
}

// assemblyChoiceOptions: Auswahl als (id, Beschriftung) mit "ohne" vorn; ein
// gesetzter Wert, der nicht mehr zur Auswahl steht (z. B. Fahrzeug heute
// abgemeldet), bleibt mit seiner Beschriftung erhalten.
List<({String id, String label})> assemblyChoiceOptions(
    List<dynamic> choices, String Function(Map<String, dynamic>) label, String none, String? currentId, String currentLabel) {
  final out = [(id: '', label: none)];
  for (final c in choices.whereType<Map<String, dynamic>>()) {
    if (c['id'] != null) out.add((id: c['id'].toString(), label: label(c)));
  }
  if (currentId != null && currentId.isNotEmpty && !out.any((o) => o.id == currentId)) {
    out.add((id: currentId, label: currentLabel.isEmpty ? currentId : currentLabel));
  }
  return out;
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
  bool busy = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('production.write');
  bool get _canPlan => _canWrite && assemblyAllowed(widget.status);

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

  String _message(Object e) => e is ApiException ? e.message : e.toString();

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  // _write speichert, meldet die Hinweise des Termins und lädt neu.
  Future<void> _write(Future<Map<String, dynamic>> Function() call, String done) async {
    setState(() => busy = true);
    try {
      final a = await call();
      _snack([done, ...assemblyHints(a)].join(' – '));
      await _load();
    } catch (e) {
      _snack('Fehler: ${_message(e)}');
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  // _edit legt einen Termin an (a == null) oder ändert ihn.
  Future<void> _edit([Map<String, dynamic>? a]) async {
    Map<String, dynamic> choices;
    try {
      choices = await widget.api.getProductionAssemblyChoices();
    } catch (e) {
      _snack('Teams und Fahrzeuge konnten nicht geladen werden: ${_message(e)}');
      return;
    }
    if (!mounted) return;
    final teams = assemblyChoiceOptions((choices['teams'] as List?) ?? const [], (t) => t['name']?.toString() ?? '', 'ohne Team',
        a?['team_id']?.toString(), a?['team_name']?.toString() ?? '');
    final vehicles = assemblyChoiceOptions((choices['fahrzeuge'] as List?) ?? const [], assemblyVehicleLabel, 'ohne Fahrzeug',
        a?['vehicle_id']?.toString(), a?['kennzeichen']?.toString() ?? '');
    final vonCtrl = TextEditingController(text: isoToGerman(a?['von']?.toString()));
    final bisCtrl = TextEditingController(text: isoToGerman(a?['bis']?.toString()));
    final baustelleCtrl = TextEditingController(text: a?['baustelle']?.toString() ?? '');
    final notizCtrl = TextEditingController(text: a?['notiz']?.toString() ?? '');
    var teamId = a?['team_id']?.toString() ?? '';
    var vehicleId = a?['vehicle_id']?.toString() ?? '';
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(a == null ? 'Montagetermin anlegen' : 'Montagetermin ändern'),
          content: SizedBox(
            width: 480,
            child: SingleChildScrollView(
              child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
                TextField(controller: vonCtrl, decoration: const InputDecoration(labelText: 'Beginn (TT.MM.JJJJ)')),
                TextField(controller: bisCtrl, decoration: const InputDecoration(labelText: 'Ende (TT.MM.JJJJ)')),
                DropdownButtonFormField<String>(
                  initialValue: teamId,
                  isExpanded: true,
                  decoration: const InputDecoration(labelText: 'Team'),
                  items: [for (final o in teams) DropdownMenuItem(value: o.id, child: Text(o.label, overflow: TextOverflow.ellipsis))],
                  onChanged: (v) => setStateDialog(() => teamId = v ?? ''),
                ),
                DropdownButtonFormField<String>(
                  initialValue: vehicleId,
                  isExpanded: true,
                  decoration: const InputDecoration(labelText: 'Fahrzeug'),
                  items: [for (final o in vehicles) DropdownMenuItem(value: o.id, child: Text(o.label, overflow: TextOverflow.ellipsis))],
                  onChanged: (v) => setStateDialog(() => vehicleId = v ?? ''),
                ),
                TextField(controller: baustelleCtrl, decoration: const InputDecoration(labelText: 'Baustelle (optional)')),
                TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz (optional)')),
                if (dialogError != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(dialogError!, style: const TextStyle(color: Color(0xFFB00020))),
                  ),
              ]),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final r = buildAssemblyBody(
                    von: vonCtrl.text,
                    bis: bisCtrl.text,
                    teamId: teamId,
                    vehicleId: vehicleId,
                    baustelle: baustelleCtrl.text,
                    notiz: notizCtrl.text);
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop(r.body);
              },
              child: const Text('Speichern'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    if (a == null) {
      await _write(() => widget.api.createProductionAssembly(widget.orderId, body), 'Montagetermin angelegt');
    } else {
      await _write(() => widget.api.updateProductionAssembly(widget.orderId, a['id'].toString(), body), 'Montagetermin geändert');
    }
  }

  Future<void> _cancel(Map<String, dynamic> a) async {
    final reasonCtrl = TextEditingController();
    String? dialogError;
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Montagetermin stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(assemblyLine(a)),
              TextField(controller: reasonCtrl, decoration: const InputDecoration(labelText: 'Begründung')),
              if (dialogError != null)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Text(dialogError!, style: const TextStyle(color: Color(0xFFB00020))),
                ),
            ]),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final problem = assemblyCancelReasonError(reasonCtrl.text);
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(reasonCtrl.text.trim());
              },
              child: const Text('Stornieren'),
            ),
          ],
        ),
      ),
    );
    if (reason == null) return;
    await _write(() => widget.api.cancelProductionAssembly(widget.orderId, a['id'].toString(), reason), 'Montagetermin storniert');
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(children: [
          const Expanded(child: Text('Montage', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold))),
          if (_canPlan)
            OutlinedButton.icon(
              onPressed: busy ? null : () => _edit(),
              icon: const Icon(Icons.event_outlined),
              label: const Text('Termin anlegen'),
            ),
        ]),
        if (loading || busy) const LinearProgressIndicator(minHeight: 2),
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
            trailing: _canWrite && !assemblyCancelled(x)
                ? Row(mainAxisSize: MainAxisSize.min, children: [
                    if (_canPlan)
                      IconButton(tooltip: 'Termin ändern', icon: const Icon(Icons.edit_outlined), onPressed: busy ? null : () => _edit(x)),
                    IconButton(tooltip: 'Termin stornieren', icon: const Icon(Icons.event_busy), onPressed: busy ? null : () => _cancel(x)),
                  ])
                : null,
          ),
      ],
    );
  }
}
