import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;
import 'leave_admin_page.dart' show employeeNames;
import 'my_leave_page.dart' show accountLines, formatLeaveDays;

// Urlaubskonten und Resturlaub fuer die Personalabteilung (Backlog
// F.8.6.2.2), Routen aus F.8.3: Konto eines Mitarbeiters je Jahr ansehen
// und festlegen (Anspruch, Uebertrag, Hinweis auf Resturlaub und Verfall,
// Notiz; Teilurlaub-Vorschlag uebernehmbar) und die Liste "Resturlaub".
//
// Lesen mit leave_requests.read, festlegen mit leave_requests.decide. Das
// System laesst nichts verfallen - der Uebertrag wird hier ausdruecklich
// gesetzt (ADR 0029).

// parseAccountDays: ganze oder halbe Tage von 0 bis 366; leer = null.
({bool ok, double? value}) parseAccountDays(String input) {
  final t = input.trim().replaceAll(',', '.');
  if (t.isEmpty) return (ok: true, value: null);
  final v = double.tryParse(t);
  if (v == null || v < 0 || v > 366 || (v * 2) != (v * 2).roundToDouble()) return (ok: false, value: null);
  return (ok: true, value: v);
}

// buildAccountBody prueft die Eingaben fuer PUT /leave-accounts.
({Map<String, dynamic>? body, String? error}) buildAccountBody({
  required String anspruch,
  String uebertrag = '',
  String hinweis = '',
  String notiz = '',
}) {
  final a = parseAccountDays(anspruch);
  if (!a.ok || a.value == null) return (body: null, error: 'Bitte den Anspruch in ganzen oder halben Tagen angeben (0 bis 366).');
  final u = parseAccountDays(uebertrag);
  if (!u.ok) return (body: null, error: 'Bitte den Übertrag in ganzen oder halben Tagen angeben oder leer lassen.');
  final body = <String, dynamic>{'anspruch_tage': a.value, 'uebertrag_tage': u.value ?? 0, 'notiz': notiz.trim()};
  if (hinweis.trim().isNotEmpty) {
    final h = parseDateInput(hinweis);
    if (h == null) return (body: null, error: 'Bitte das Datum des Hinweises als TT.MM.JJJJ angeben oder leer lassen.');
    body['hinweis_erteilt_am'] = h;
  }
  return (body: body, error: null);
}

// remainingLine: "Berg, Anna · Rest 15 Tage · Hinweis erteilt am 01.10.2026"
// bzw. "· Hinweis auf Verfall fehlt".
String remainingLine(Map<String, dynamic> r) {
  final konto = (r['konto'] as Map<String, dynamic>?) ?? const {};
  final first = r['first_name']?.toString() ?? '';
  final name = first.isEmpty ? (r['last_name']?.toString() ?? '') : '${r['last_name']}, $first';
  final hint = konto['hinweis_erteilt_am']?.toString();
  return [
    name,
    'Rest ${formatLeaveDays(konto['rest_tage'] as num?)}',
    hint == null || hint.isEmpty ? 'Hinweis auf Verfall fehlt' : 'Hinweis erteilt am ${isoToGerman(hint)}',
  ].join(' · ');
}

String _days(num? v) {
  final d = (v ?? 0).toDouble();
  return d == d.roundToDouble() ? d.toInt().toString() : d.toString().replaceAll('.', ',');
}

class LeaveAccountsPage extends StatefulWidget {
  const LeaveAccountsPage({super.key, required this.api, this.now});
  final ApiClient api;
  final DateTime Function()? now;

  @override
  State<LeaveAccountsPage> createState() => _LeaveAccountsPageState();
}

class _LeaveAccountsPageState extends State<LeaveAccountsPage> {
  late int year;
  List<dynamic> employees = [];
  Map<String, String> names = {};
  List<dynamic> remaining = [];
  String? selected;
  Map<String, dynamic>? account;
  bool loading = false;
  String? error;

  bool get _canDecide => widget.api.hasPermission('leave_requests.decide');

  @override
  void initState() {
    super.initState();
    year = (widget.now ?? DateTime.now)().year;
    _load();
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final emps = await widget.api.listEmployees(limit: 500);
      final rest = await widget.api.listRemainingLeave(year: year);
      Map<String, dynamic>? acc;
      if (selected != null) acc = await widget.api.getLeaveAccount(employeeId: selected, year: year);
      if (!mounted) return;
      final allNames = employeeNames(emps);
      setState(() {
        names = allNames;
        // Auswahl: aktive Mitarbeiter, nach Namen sortiert.
        employees = emps.where((e) => (e as Map<String, dynamic>)['active'] != false).toList()
          ..sort((a, b) => allNames[a['id'].toString()]!.compareTo(allNames[b['id'].toString()]!));
        remaining = rest;
        account = acc;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => error = 'Daten konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted) setState(() => error = 'Daten konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _select(String? id) {
    setState(() {
      selected = id;
      account = null;
    });
    _load();
  }

  void _goToYear(int y) {
    setState(() => year = y);
    _load();
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _setDialog() async {
    final acc = account;
    final id = selected;
    if (acc == null || id == null) return;
    final anspruchCtrl = TextEditingController(text: _days(acc['anspruch_tage'] as num?));
    final uebertragCtrl = TextEditingController(text: _days(acc['uebertrag_tage'] as num?));
    final hinweisCtrl = TextEditingController(text: isoToGerman(acc['hinweis_erteilt_am']?.toString()));
    final notizCtrl = TextEditingController(text: acc['notiz']?.toString() ?? '');
    final proposal = acc['teilurlaub'] as Map<String, dynamic>?;
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text('Urlaubskonto $year: ${names[id] ?? ''}'),
          content: SizedBox(
            width: 460,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (proposal != null)
                    Row(
                      children: [
                        Expanded(
                            child: Text('Vorschlag: ${formatLeaveDays(proposal['tage'] as num?)} – ${proposal['grund'] ?? ''}')),
                        TextButton(
                          onPressed: () => setStateDialog(() => anspruchCtrl.text = _days(proposal['tage'] as num?)),
                          child: const Text('Übernehmen'),
                        ),
                      ],
                    ),
                  TextField(controller: anspruchCtrl, decoration: const InputDecoration(labelText: 'Anspruch (Tage)')),
                  TextField(controller: uebertragCtrl, decoration: const InputDecoration(labelText: 'Übertrag aus dem Vorjahr (Tage)')),
                  TextField(
                    controller: hinweisCtrl,
                    decoration: const InputDecoration(labelText: 'Hinweis auf Resturlaub und Verfall erteilt am (TT.MM.JJJJ)'),
                  ),
                  TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
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
                final r = buildAccountBody(
                    anspruch: anspruchCtrl.text, uebertrag: uebertragCtrl.text, hinweis: hinweisCtrl.text, notiz: notizCtrl.text);
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
    try {
      await widget.api.setLeaveAccount(id, year, body);
      _snack('Urlaubskonto gespeichert');
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final acc = account;
    final proposal = acc?['teilurlaub'] as Map<String, dynamic>?;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Urlaubskonten'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 8,
              children: [
                IconButton(onPressed: () => _goToYear(year - 1), icon: const Icon(Icons.chevron_left), tooltip: 'Vorjahr'),
                Text('Urlaubsjahr $year', style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                IconButton(onPressed: () => _goToYear(year + 1), icon: const Icon(Icons.chevron_right), tooltip: 'Folgejahr'),
              ],
            ),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            SizedBox(
              width: 400,
              child: DropdownButtonFormField<String>(
                key: ValueKey('mitarbeiter-$selected-${employees.length}'),
                initialValue: selected,
                isExpanded: true,
                decoration: const InputDecoration(labelText: 'Mitarbeiter'),
                items: [
                  for (final e in employees.cast<Map<String, dynamic>>())
                    DropdownMenuItem(value: e['id'].toString(), child: Text(names[e['id'].toString()] ?? '')),
                ],
                onChanged: _select,
              ),
            ),
            if (acc != null) ...[
              const SizedBox(height: 8),
              Text(acc['festgelegt'] == true ? 'Konto festgelegt' : 'Konto nicht festgelegt - es gilt der Jahresanspruch des Mitarbeiters'),
              for (final line in accountLines(acc)) Text(line),
              if ((acc['hinweis_erteilt_am']?.toString() ?? '').isNotEmpty)
                Text('Hinweis auf Resturlaub und Verfall erteilt am ${isoToGerman(acc['hinweis_erteilt_am'].toString())}'),
              if ((acc['notiz']?.toString() ?? '').isNotEmpty) Text('Notiz: ${acc['notiz']}'),
              if (proposal != null)
                Text('Vorschlag Teilurlaub: ${formatLeaveDays(proposal['tage'] as num?)} – ${proposal['grund'] ?? ''}',
                    style: const TextStyle(color: Color(0xFFB26A00))),
              if (_canDecide)
                Align(
                  alignment: Alignment.centerLeft,
                  child: FilledButton.tonal(onPressed: _setDialog, child: const Text('Konto festlegen')),
                ),
            ],
            const SizedBox(height: 16),
            const Text('Resturlaub', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
            const Text('Mitarbeiter mit offenem Resturlaub - Grundlage für den Hinweis auf Verfall. Das System lässt nichts verfallen.'),
            if (!loading && error == null && remaining.isEmpty) const Text('Niemand hat Resturlaub.'),
            for (final r in remaining.cast<Map<String, dynamic>>())
              ListTile(
                dense: true,
                title: Text(remainingLine(r)),
                onTap: () => _select(r['employee_id'].toString()),
              ),
          ],
        ),
      ),
    );
  }
}
