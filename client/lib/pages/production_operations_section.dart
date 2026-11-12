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

// operationDoneLine: "Erledigt am 12.11.2026 · Ist 1 h 35 min · Rest morgen"
// aus der gueltigen Rueckmeldung (H.3.5e1), sonst null.
String? operationDoneLine(Map<String, dynamic> op) {
  if (op['erledigt'] != true) return null;
  final ist = op['ist_minuten'];
  final notiz = op['rueckmeldung_notiz']?.toString() ?? '';
  return [
    'Erledigt am ${isoToGerman(op['erledigt_at']?.toString())}',
    if (ist is num) 'Ist ${formatMinutes(ist)}',
    if (notiz.isNotEmpty) notiz,
  ].join(' · ');
}

// sollIstLine: Fortschritt und Vergleich Soll/Ist des Auftrags (H.3.5c),
// null solange nichts erledigt ist. Ist wird nur mit dem Soll der
// Arbeitsgaenge mit Ist-Zeit verglichen.
String? sollIstLine(Map<String, dynamic>? s) {
  if (s == null) return null;
  final erledigt = (s['erledigt'] as num?)?.toInt() ?? 0;
  if (erledigt == 0) return null;
  final mitIst = (s['mit_ist'] as num?)?.toInt() ?? 0;
  num m(String k) => (s[k] as num?) ?? 0;
  return [
    'Erledigt $erledigt von ${s['arbeitsgaenge']}',
    'Soll erledigt ${formatMinutes(m('soll_erledigt_min'))} von ${formatMinutes(m('soll_min'))}',
    if (mitIst > 0)
      'Ist ${formatMinutes(m('ist_min'))} zu Soll ${formatMinutes(m('soll_mit_ist_min'))} '
          '(${mitIst == 1 ? '1 Arbeitsgang' : '$mitIst Arbeitsgänge'} mit Ist-Zeit)',
  ].join(' · ');
}

// feedbackAllowed: melden und zuruecknehmen nur bei begonnenen Auftraegen
// (wie im Server, H.3.5a).
bool feedbackAllowed(String? status) => status == 'in_arbeit';

// buildFeedbackBody prueft die Eingaben der Rueckmeldung wie der Server
// oder liefert eine Meldung. Leere Ist-Zeit = ohne Ist-Zeit.
({Map<String, dynamic>? body, String? error}) buildFeedbackBody(
    {String ist = '', String notiz = ''}) {
  final body = <String, dynamic>{'notiz': notiz.trim()};
  if (ist.trim().isNotEmpty) {
    final v = parseDecimalDe(ist);
    if (v == null || !v.isFinite || v <= 0 || v > 100000)
      return (
        body: null,
        error: 'Ist-Zeit: Minuten größer als 0, höchstens 100.000.'
      );
    body['ist_minuten'] = v;
  }
  if (notiz.trim().runes.length > 500)
    return (body: null, error: 'Notiz: höchstens 500 Zeichen.');
  return (body: body, error: null);
}

// feedbackRevertReasonError: Begruendung Pflicht, hoechstens 500 Zeichen.
String? feedbackRevertReasonError(String reason) {
  final t = reason.trim();
  if (t.isEmpty) return 'Bitte eine Begründung angeben.';
  if (t.runes.length > 500) return 'Begründung: höchstens 500 Zeichen.';
  return null;
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
  Map<String, dynamic>? sollIst;
  bool loading = false;
  bool busy = false;
  String? error;

  bool get _canEdit =>
      widget.api.hasPermission('production.write') &&
      operationsEditable(widget.status);

  bool get _canReport =>
      widget.api.hasPermission('production.write') &&
      feedbackAllowed(widget.status);

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
    // Soll/Ist ist eine Zusatzangabe: ohne sie bleibt die Liste nutzbar.
    try {
      final s = await widget.api.getProductionSollIst(widget.orderId);
      if (mounted) setState(() => sollIst = s);
    } catch (_) {
      if (mounted) setState(() => sollIst = null);
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

  // _askFeedback zeigt einen Dialog mit Eingabefeldern; check liefert die
  // Meldung oder null. Ergebnis sind die Texte, bei Abbruch null.
  Future<List<String>?> _askFeedback(String title, String action,
      List<InputDecoration> fields, String? Function(List<String>) check) {
    final ctrls = [for (final _ in fields) TextEditingController()];
    String? dialogError;
    return showDialog<List<String>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(title),
          content: SizedBox(
            width: 460,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (var i = 0; i < fields.length; i++)
                  TextField(controller: ctrls[i], decoration: fields[i]),
                if (dialogError != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(dialogError!,
                        style: const TextStyle(color: Color(0xFFB00020))),
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.of(ctx).pop(),
                child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final values = [for (final c in ctrls) c.text];
                final e = check(values);
                if (e != null) {
                  setStateDialog(() => dialogError = e);
                  return;
                }
                Navigator.of(ctx).pop(values);
              },
              child: Text(action),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _report(Map<String, dynamic> op) async {
    final v = await _askFeedback(
        '„${op['name']}“ erledigt melden',
        'Erledigt melden',
        const [
          InputDecoration(
              labelText: 'Ist-Zeit (Minuten)',
              helperText: 'Leer lassen = ohne Ist-Zeit.'),
          InputDecoration(labelText: 'Notiz'),
        ],
        (v) => buildFeedbackBody(ist: v[0], notiz: v[1]).error);
    if (v == null) return;
    final body = buildFeedbackBody(ist: v[0], notiz: v[1]).body!;
    await _write(
        () => widget.api.reportProductionOperationDone(
            widget.orderId, op['id'].toString(), body),
        'Arbeitsgang erledigt gemeldet');
  }

  Future<void> _revert(Map<String, dynamic> op) async {
    final v = await _askFeedback(
        'Rückmeldung zu „${op['name']}“ zurücknehmen',
        'Zurücknehmen',
        const [InputDecoration(labelText: 'Begründung')],
        (v) => feedbackRevertReasonError(v[0]));
    if (v == null) return;
    await _write(
        () => widget.api.revertProductionOperationDone(
            widget.orderId, op['id'].toString(), v[0].trim()),
        'Rückmeldung zurückgenommen');
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
        if (sollIstLine(sollIst) case final line?) Text(line),
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
            final done = operationDoneLine(m);
            return ListTile(
              contentPadding: EdgeInsets.zero,
              title: Row(children: [
                if (done != null) ...[
                  Icon(Icons.check_circle,
                      size: 18,
                      color: Colors.green.shade700,
                      semanticLabel: 'erledigt'),
                  const SizedBox(width: 4),
                ],
                Expanded(child: Text('${m['pos']}. ${m['name'] ?? ''}')),
              ]),
              subtitle: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(operationLine(m)),
                  if (done != null) Text(done),
                  for (final h in hints)
                    Row(children: [
                      Icon(Icons.warning_amber_rounded,
                          size: 16, color: Colors.orange.shade800),
                      const SizedBox(width: 4),
                      Expanded(child: Text(operationHintLabels[h] ?? h)),
                    ]),
                ],
              ),
              // Melden setzt Aenderbarkeit voraus (in Arbeit, write).
              trailing: _canEdit
                  ? Row(mainAxisSize: MainAxisSize.min, children: [
                      if (_canReport && done == null)
                        IconButton(
                            tooltip: 'Erledigt melden',
                            icon: const Icon(Icons.task_alt),
                            onPressed: busy ? null : () => _report(m)),
                      if (_canReport && done != null)
                        IconButton(
                            tooltip: 'Rückmeldung zurücknehmen',
                            icon: const Icon(Icons.undo),
                            onPressed: busy ? null : () => _revert(m)),
                      if (_canEdit) ...[
                        IconButton(
                            tooltip: 'Arbeitsgang bearbeiten',
                            icon: const Icon(Icons.edit),
                            onPressed: busy ? null : () => _dialog(op: m)),
                        // Mit Rueckmeldung sperrt der Server das Entfernen.
                        if (done == null)
                          IconButton(
                              tooltip: 'Arbeitsgang entfernen',
                              icon: const Icon(Icons.delete_outline),
                              onPressed: busy ? null : () => _delete(m)),
                      ],
                    ])
                  : null,
            );
          }),
      ],
    );
  }
}
