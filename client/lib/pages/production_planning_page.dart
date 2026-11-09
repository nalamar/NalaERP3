import 'package:flutter/material.dart';
import '../api.dart';
import 'production_orders_page.dart' show formatQty;

// Stammdaten der Grobplanung (Backlog H.2.5a, ADR 0037) auf den Routen aus
// H.2.2: Arbeitsplaetze mit Stunden je Arbeitstag und Arbeitsgangvorlagen
// mit Arbeitsplatz, Reihenfolge und Planzeit. Lesen mit production.read,
// pflegen mit production.write. Nichts wird geloescht, sondern
// deaktiviert.

// parseDecimalDe liest "7,5" und "7.5"; leer oder unlesbar = null.
double? parseDecimalDe(String input) {
  final t = input.trim().replaceAll(',', '.');
  if (t.isEmpty) return null;
  return double.tryParse(t);
}

// formatMinutes: "45 min", "1 h", "1 h 10 min" (gerundet auf Minuten).
String formatMinutes(num minutes) {
  final m = minutes.round();
  if (m < 60) return '$m min';
  final h = m ~/ 60, rest = m % 60;
  return rest == 0 ? '$h h' : '$h h $rest min';
}

// workCenterInputError: Pruefung im Dialog wie im Server (Name, Stunden
// groesser 0 bis 24).
String? workCenterInputError(String name, String hours) {
  if (name.trim().isEmpty) return 'Bitte einen Namen angeben.';
  final h = parseDecimalDe(hours);
  if (h == null || h <= 0 || h > 24)
    return 'Stunden je Tag: mehr als 0 und höchstens 24.';
  return null;
}

// buildWorkCenterBody: aktiv nur beim Aendern.
Map<String, dynamic> buildWorkCenterBody(
        {required String name, required String hours, bool? aktiv}) =>
    {
      'name': name.trim(),
      'stunden_je_tag': parseDecimalDe(hours),
      if (aktiv != null) 'aktiv': aktiv,
    };

// templateInputError: Name, Arbeitsplatz, Reihenfolge ab 1, Zeiten nicht
// negativ (leer = 0).
String? templateInputError(String name, String? workCenterId,
    String reihenfolge, String ruest, String jeStueck) {
  if (name.trim().isEmpty) return 'Bitte einen Namen angeben.';
  if (workCenterId == null || workCenterId.isEmpty)
    return 'Bitte einen Arbeitsplatz wählen.';
  final r = int.tryParse(reihenfolge.trim());
  if (r == null || r < 1) return 'Reihenfolge: ganze Zahl ab 1.';
  for (final t in [ruest, jeStueck]) {
    if (t.trim().isEmpty) continue;
    final v = parseDecimalDe(t);
    if (v == null || v < 0) return 'Zeiten: Minuten, nicht negativ.';
  }
  return null;
}

Map<String, dynamic> buildTemplateBody({
  required String name,
  required String workCenterId,
  required String reihenfolge,
  required String ruest,
  required String jeStueck,
  bool? aktiv,
}) =>
    {
      'name': name.trim(),
      'work_center_id': workCenterId,
      'reihenfolge': int.parse(reihenfolge.trim()),
      'ruestzeit_min': parseDecimalDe(ruest) ?? 0,
      'minuten_je_stueck': parseDecimalDe(jeStueck) ?? 0,
      if (aktiv != null) 'aktiv': aktiv,
    };

// templateWorkCenterChoices: aktive Arbeitsplaetze, dazu der schon
// zugeordnete, inzwischen deaktivierte (der Server erlaubt nur diesen).
List<Map<String, dynamic>> templateWorkCenterChoices(
        List<dynamic> allCenters, String? currentId) =>
    [
      for (final c in allCenters)
        if ((c as Map<String, dynamic>)['aktiv'] != false ||
            c['id'].toString() == currentId)
          {
            'id': c['id'].toString(),
            'label': c['aktiv'] == false
                ? '${c['name']} (deaktiviert)'
                : c['name'].toString()
          },
    ];

// workCenterLine: "8 h je Arbeitstag · deaktiviert".
String workCenterLine(Map<String, dynamic> c) =>
    '${formatQty(c['stunden_je_tag'])} h je Arbeitstag${c['aktiv'] == false ? ' · deaktiviert' : ''}';

// templateLine: "1. · Zuschnitt · Rüsten 30 min · 4,5 min je Stück".
String templateLine(Map<String, dynamic> t) {
  final parts = <String>[
    '${t['reihenfolge']}.',
    t['work_center_name']?.toString() ?? ''
  ];
  final ruest = t['ruestzeit_min'];
  if (ruest is num && ruest > 0) parts.add('Rüsten ${formatMinutes(ruest)}');
  final je = t['minuten_je_stueck'];
  if (je is num && je > 0) parts.add('${formatQty(je)} min je Stück');
  if (t['aktiv'] == false) parts.add('deaktiviert');
  return parts.join(' · ');
}

class ProductionPlanningPage extends StatefulWidget {
  const ProductionPlanningPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<ProductionPlanningPage> createState() => _ProductionPlanningPageState();
}

class _ProductionPlanningPageState extends State<ProductionPlanningPage> {
  // inklusive deaktivierter: Namen und Auswahl im Vorlagen-Dialog
  List<dynamic> centers = [];
  List<dynamic> templates = [];
  bool loading = false;
  bool showInactive = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('production.write');

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
      final c = await widget.api.listWorkCenters(includeInactive: true);
      final t = await widget.api
          .listOperationTemplates(includeInactive: showInactive);
      if (mounted) {
        setState(() {
          centers = c;
          templates = t;
        });
      }
    } catch (e) {
      if (mounted)
        setState(() => error =
            'Arbeitsplätze und Vorlagen konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted)
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Widget _errorText(String? e) => e == null
      ? const SizedBox.shrink()
      : Padding(
          padding: const EdgeInsets.only(top: 8),
          child: Text(e, style: const TextStyle(color: Color(0xFFB00020))));

  Future<void> _workCenterDialog({Map<String, dynamic>? center}) async {
    final editing = center != null;
    final nameCtrl =
        TextEditingController(text: center?['name']?.toString() ?? '');
    final hoursCtrl = TextEditingController(
        text: editing ? formatQty(center['stunden_je_tag']) : '8');
    bool aktiv = center == null || center['aktiv'] != false;
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title:
              Text(editing ? 'Arbeitsplatz bearbeiten' : 'Neuer Arbeitsplatz'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                TextField(
                    controller: nameCtrl,
                    decoration: const InputDecoration(labelText: 'Name')),
                TextField(
                  controller: hoursCtrl,
                  decoration: const InputDecoration(
                      labelText: 'Stunden je Arbeitstag',
                      helperText:
                          'Mo–Fr; Feiertage und Betriebsruhe gehen ab.'),
                ),
                if (editing)
                  CheckboxListTile(
                    contentPadding: EdgeInsets.zero,
                    value: aktiv,
                    title: const Text('Aktiv'),
                    subtitle: const Text(
                        'Ein deaktivierter Arbeitsplatz behält seine Arbeitsgänge, bekommt aber keine neuen.'),
                    onChanged: (v) => setStateDialog(() => aktiv = v == true),
                  ),
                _errorText(dialogError),
              ],
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.of(ctx).pop(),
                child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final problem =
                    workCenterInputError(nameCtrl.text, hoursCtrl.text);
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(buildWorkCenterBody(
                    name: nameCtrl.text,
                    hours: hoursCtrl.text,
                    aktiv: editing ? aktiv : null));
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
        await widget.api.updateWorkCenter(center['id'].toString(), body);
        _snack('Arbeitsplatz gespeichert');
      } else {
        await widget.api.createWorkCenter(body);
        _snack('Arbeitsplatz angelegt');
      }
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _templateDialog({Map<String, dynamic>? template}) async {
    final editing = template != null;
    final nameCtrl =
        TextEditingController(text: template?['name']?.toString() ?? '');
    final nextOrder = templates.isEmpty
        ? 1
        : templates
                .map((t) =>
                    ((t as Map<String, dynamic>)['reihenfolge'] as num?)
                        ?.toInt() ??
                    0)
                .reduce((a, b) => a > b ? a : b) +
            1;
    final orderCtrl = TextEditingController(
        text: (template?['reihenfolge'] ?? nextOrder).toString());
    final ruestCtrl = TextEditingController(
        text: editing ? formatQty(template['ruestzeit_min']) : '');
    final stueckCtrl = TextEditingController(
        text: editing ? formatQty(template['minuten_je_stueck']) : '');
    final currentWc = template?['work_center_id']?.toString();
    final choices = templateWorkCenterChoices(centers, currentWc);
    String? wcId = currentWc;
    bool aktiv = template == null || template['aktiv'] != false;
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(editing ? 'Vorlage bearbeiten' : 'Neue Vorlage'),
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
                          'Noch kein aktiver Arbeitsplatz – bitte zuerst einen anlegen.'),
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
                    controller: orderCtrl,
                    decoration: const InputDecoration(
                        labelText: 'Reihenfolge',
                        helperText:
                            'In dieser Reihenfolge wird vorgeschlagen.'),
                  ),
                  TextField(
                      controller: ruestCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Rüstzeit (Minuten je Auftrag)')),
                  TextField(
                      controller: stueckCtrl,
                      decoration:
                          const InputDecoration(labelText: 'Minuten je Stück')),
                  if (editing)
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      value: aktiv,
                      title: const Text('Aktiv'),
                      subtitle: const Text(
                          'Nur aktive Vorlagen werden am Auftrag vorgeschlagen.'),
                      onChanged: (v) => setStateDialog(() => aktiv = v == true),
                    ),
                  _errorText(dialogError),
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
                final problem = templateInputError(nameCtrl.text, wcId,
                    orderCtrl.text, ruestCtrl.text, stueckCtrl.text);
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(buildTemplateBody(
                  name: nameCtrl.text,
                  workCenterId: wcId!,
                  reihenfolge: orderCtrl.text,
                  ruest: ruestCtrl.text,
                  jeStueck: stueckCtrl.text,
                  aktiv: editing ? aktiv : null,
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
        await widget.api
            .updateOperationTemplate(template['id'].toString(), body);
        _snack('Vorlage gespeichert');
      } else {
        await widget.api.createOperationTemplate(body);
        _snack('Vorlage angelegt');
      }
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final shownCenters = [
      for (final c in centers)
        if (showInactive || (c as Map<String, dynamic>)['aktiv'] != false)
          c as Map<String, dynamic>,
    ];
    return Scaffold(
      appBar: AppBar(
        title: const Text('Arbeitsplätze und Vorlagen'),
        actions: [
          IconButton(
              onPressed: _load,
              icon: const Icon(Icons.refresh),
              tooltip: 'Neu laden')
        ],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Align(
              alignment: Alignment.centerLeft,
              child: FilterChip(
                label: const Text('Deaktivierte anzeigen'),
                selected: showInactive,
                onSelected: (v) {
                  setState(() => showInactive = v);
                  _load();
                },
              ),
            ),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null)
              Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text('Arbeitsplätze',
                  style: Theme.of(context).textTheme.titleMedium),
              trailing: _canWrite
                  ? TextButton.icon(
                      onPressed: () => _workCenterDialog(),
                      icon: const Icon(Icons.add),
                      label: const Text('Arbeitsplatz'))
                  : null,
            ),
            if (!loading && error == null && shownCenters.isEmpty)
              const Text('Noch keine Arbeitsplätze.'),
            for (final c in shownCenters)
              ListTile(
                title: Text(c['name']?.toString() ?? ''),
                subtitle: Text(workCenterLine(c)),
                trailing: _canWrite
                    ? IconButton(
                        tooltip: 'Arbeitsplatz bearbeiten',
                        icon: const Icon(Icons.edit),
                        onPressed: () => _workCenterDialog(center: c))
                    : null,
              ),
            const Divider(),
            ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text('Arbeitsgangvorlagen',
                  style: Theme.of(context).textTheme.titleMedium),
              trailing: _canWrite
                  ? TextButton.icon(
                      onPressed: () => _templateDialog(),
                      icon: const Icon(Icons.add),
                      label: const Text('Vorlage'))
                  : null,
            ),
            if (!loading && error == null && templates.isEmpty)
              const Text('Noch keine Vorlagen.'),
            for (final t in templates)
              Builder(builder: (context) {
                final m = t as Map<String, dynamic>;
                return ListTile(
                  title: Text(m['name']?.toString() ?? ''),
                  subtitle: Text(templateLine(m)),
                  trailing: _canWrite
                      ? IconButton(
                          tooltip: 'Vorlage bearbeiten',
                          icon: const Icon(Icons.edit),
                          onPressed: () => _templateDialog(template: m))
                      : null,
                );
              }),
          ],
        ),
      ),
    );
  }
}
