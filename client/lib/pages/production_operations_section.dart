import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;
import 'production_orders_page.dart' show formatQty;
import 'production_planning_page.dart'
    show formatMinutes, parseDecimalDe, templateWorkCenterChoices;

// Arbeitsgaenge im Detail des Fertigungsauftrags (Backlog H.2.5b, ADR 0037)
// auf den Routen aus H.2.3b: Liste mit Arbeitsplatz, geplanter Woche,
// Planzeit und Hinweis; mit production.write Vorschlag aus den Vorlagen,
// hinzufuegen, aendern, entfernen - solange der Auftrag weder fertig noch
// storniert ist.

const operationHintLabels = {
  'nach_liefertermin': 'Geplant nach dem Liefertermin'
};

bool operationsEditable(String? status) =>
    status != null && status != 'fertig' && status != 'storniert';

// isoWeekNumber: Kalenderwoche nach ISO 8601.
int isoWeekNumber(DateTime d) {
  final day = DateTime.utc(d.year, d.month, d.day);
  final thursday = day.add(Duration(days: 4 - day.weekday));
  final firstThursday = DateTime.utc(thursday.year, 1, 4);
  final weekOfFirst =
      firstThursday.add(Duration(days: 4 - firstThursday.weekday));
  return 1 + thursday.difference(weekOfFirst).inDays ~/ 7;
}

// operationWeekLabel: "KW 48 (ab 23.11.2026)" oder "ungeplant".
String operationWeekLabel(String? iso) {
  if (iso == null || iso.length < 10) return 'ungeplant';
  final d = DateTime.tryParse(iso.substring(0, 10));
  if (d == null) return 'ungeplant';
  return 'KW ${isoWeekNumber(d)} (ab ${isoToGerman(iso)})';
}

// operationLine: "Zuschnitt · KW 48 (ab 23.11.2026) · 1 h 10 min".
String operationLine(Map<String, dynamic> op) {
  final plan = op['plan_minuten'];
  return [
    op['work_center_name']?.toString() ?? '',
    operationWeekLabel(op['geplante_woche']?.toString()),
    if (plan is num) formatMinutes(plan),
  ].join(' · ');
}

// operationsTotal: Summe der Planzeit.
num operationsTotal(List<dynamic> ops) => ops.fold<num>(
    0,
    (sum, op) =>
        sum + (((op as Map<String, dynamic>)['plan_minuten'] as num?) ?? 0));

// buildOperationBody prueft die Eingaben oder liefert eine Meldung. Leere
// Zeiten = 0, leere Woche = ungeplant.
({Map<String, dynamic>? body, String? error}) buildOperationBody({
  required String name,
  required String? workCenterId,
  String ruest = '',
  String jeStueck = '',
  String woche = '',
}) {
  if (name.trim().isEmpty)
    return (body: null, error: 'Bitte einen Namen angeben.');
  if (workCenterId == null || workCenterId.isEmpty)
    return (body: null, error: 'Bitte einen Arbeitsplatz wählen.');
  for (final t in [ruest, jeStueck]) {
    if (t.trim().isEmpty) continue;
    final v = parseDecimalDe(t);
    if (v == null || v < 0)
      return (body: null, error: 'Zeiten: Minuten, nicht negativ.');
  }
  String iso = '';
  if (woche.trim().isNotEmpty) {
    final d = parseDateInput(woche);
    if (d == null)
      return (
        body: null,
        error: 'Bitte die Woche als Datum TT.MM.JJJJ angeben.'
      );
    iso = d;
  }
  return (
    body: {
      'name': name.trim(),
      'work_center_id': workCenterId,
      'ruestzeit_min': parseDecimalDe(ruest) ?? 0,
      'minuten_je_stueck': parseDecimalDe(jeStueck) ?? 0,
      'geplante_woche': iso,
    },
    error: null,
  );
}

class ProductionOperationsSection extends StatefulWidget {
  const ProductionOperationsSection(
      {super.key,
      required this.api,
      required this.orderId,
      required this.status});
  final ApiClient api;
  final String orderId;
  final String? status;

  @override
  State<ProductionOperationsSection> createState() =>
      _ProductionOperationsSectionState();
}

class _ProductionOperationsSectionState
    extends State<ProductionOperationsSection> {
  List<dynamic> ops = const [];
  bool loading = false;
  bool busy = false;
  String? error;

  bool get _canEdit =>
      widget.api.hasPermission('production.write') &&
      operationsEditable(widget.status);

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
      final list = await widget.api.listProductionOperations(widget.orderId);
      if (mounted) setState(() => ops = list);
    } catch (e) {
      if (mounted)
        setState(() => error = 'Arbeitsgänge konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted)
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  String _message(Object e) => e is ApiException ? e.message : e.toString();

  // _write fuehrt einen Schreibaufruf aus und laedt danach die Liste.
  Future<void> _write(Future<void> Function() call, String done) async {
    setState(() => busy = true);
    try {
      await call();
      _snack(done);
      await _load();
    } catch (e) {
      _snack('Fehler: ${_message(e)}');
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> _suggest() => _write(() async {
        final list =
            await widget.api.suggestProductionOperations(widget.orderId);
        if (mounted) setState(() => ops = list);
      }, 'Arbeitsgänge aus den Vorlagen übernommen');

  Future<void> _dialog({Map<String, dynamic>? op}) async {
    List<dynamic> centers;
    try {
      centers = await widget.api.listWorkCenters(includeInactive: true);
    } catch (e) {
      _snack('Arbeitsplätze konnten nicht geladen werden: ${_message(e)}');
      return;
    }
    if (!mounted) return;
    final editing = op != null;
    final currentWc = op?['work_center_id']?.toString();
    final choices = templateWorkCenterChoices(centers, currentWc);
    String? wcId = currentWc;
    final nameCtrl = TextEditingController(text: op?['name']?.toString() ?? '');
    final ruestCtrl = TextEditingController(
        text: editing ? formatQty(op['ruestzeit_min']) : '');
    final stueckCtrl = TextEditingController(
        text: editing ? formatQty(op['minuten_je_stueck']) : '');
    final wocheCtrl = TextEditingController(
        text: isoToGerman(op?['geplante_woche']?.toString()));
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(editing ? 'Arbeitsgang bearbeiten' : 'Neuer Arbeitsgang'),
          content: SizedBox(
            width: 460,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(
                      controller: nameCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Name des Arbeitsgangs')),
                  if (choices.isEmpty)
                    const Padding(
                      padding: EdgeInsets.only(top: 8),
                      child: Text(
                          'Noch kein aktiver Arbeitsplatz – bitte unter „Arbeitsplätze und Vorlagen“ anlegen.'),
                    )
                  else
                    DropdownButtonFormField<String>(
                      initialValue: wcId,
                      isExpanded: true,
                      decoration:
                          const InputDecoration(labelText: 'Arbeitsplatz'),
                      items: [
                        for (final c in choices)
                          DropdownMenuItem(
                              value: c['id'] as String,
                              child: Text(c['label'] as String,
                                  overflow: TextOverflow.ellipsis)),
                      ],
                      onChanged: (v) => setStateDialog(() => wcId = v),
                    ),
                  TextField(
                      controller: ruestCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Rüstzeit (Minuten je Auftrag)')),
                  TextField(
                      controller: stueckCtrl,
                      decoration:
                          const InputDecoration(labelText: 'Minuten je Stück')),
                  TextField(
                    controller: wocheCtrl,
                    decoration: const InputDecoration(
                        labelText: 'Geplante Woche (ein Tag als TT.MM.JJJJ)',
                        helperText: 'Leer lassen = ungeplant.'),
                  ),
                  if (dialogError != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(dialogError!,
                          style: const TextStyle(color: Color(0xFFB00020))),
                    ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.of(ctx).pop(),
                child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final r = buildOperationBody(
                    name: nameCtrl.text,
                    workCenterId: wcId,
                    ruest: ruestCtrl.text,
                    jeStueck: stueckCtrl.text,
                    woche: wocheCtrl.text);
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop(r.body);
              },
              child: Text(editing ? 'Speichern' : 'Anlegen'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    if (editing) {
      await _write(
          () => widget.api.updateProductionOperation(
              widget.orderId, op['id'].toString(), body),
          'Arbeitsgang gespeichert');
    } else {
      await _write(
          () => widget.api.addProductionOperation(widget.orderId, body),
          'Arbeitsgang angelegt');
    }
  }

  Future<void> _delete(Map<String, dynamic> op) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Arbeitsgang entfernen'),
        content: Text('„${op['name']}“ aus dem Auftrag entfernen?'),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: const Text('Abbrechen')),
          FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: const Text('Entfernen')),
        ],
      ),
    );
    if (ok != true) return;
    await _write(
        () => widget.api
            .deleteProductionOperation(widget.orderId, op['id'].toString()),
        'Arbeitsgang entfernt');
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(children: [
          const Text('Arbeitsgänge',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
          const SizedBox(width: 8),
          if (ops.isNotEmpty)
            Text('Planzeit gesamt ${formatMinutes(operationsTotal(ops))}'),
        ]),
        if (loading || busy) const LinearProgressIndicator(minHeight: 2),
        if (error != null)
          Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (_canEdit)
          Wrap(spacing: 8, children: [
            if (ops.isEmpty && !loading)
              OutlinedButton(
                  onPressed: busy ? null : _suggest,
                  child: const Text('Aus Vorlagen vorschlagen')),
            OutlinedButton(
                onPressed: busy ? null : () => _dialog(),
                child: const Text('Arbeitsgang hinzufügen')),
          ]),
        if (!loading && error == null && ops.isEmpty)
          const Text('Noch keine Arbeitsgänge.'),
        for (final o in ops)
          Builder(builder: (context) {
            final m = o as Map<String, dynamic>;
            final hints = ((m['hinweise'] as List?) ?? const [])
                .map((e) => e.toString())
                .toList();
            return ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text('${m['pos']}. ${m['name'] ?? ''}'),
              subtitle: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(operationLine(m)),
                  for (final h in hints)
                    Row(children: [
                      Icon(Icons.warning_amber_rounded,
                          size: 16, color: Colors.orange.shade800),
                      const SizedBox(width: 4),
                      Expanded(child: Text(operationHintLabels[h] ?? h)),
                    ]),
                ],
              ),
              trailing: _canEdit
                  ? Row(mainAxisSize: MainAxisSize.min, children: [
                      IconButton(
                          tooltip: 'Arbeitsgang bearbeiten',
                          icon: const Icon(Icons.edit),
                          onPressed: busy ? null : () => _dialog(op: m)),
                      IconButton(
                          tooltip: 'Arbeitsgang entfernen',
                          icon: const Icon(Icons.delete_outline),
                          onPressed: busy ? null : () => _delete(m)),
                    ])
                  : null,
            );
          }),
      ],
    );
  }
}
