import 'package:flutter/material.dart';
import '../api.dart';
import 'employee_qualifications_section.dart' show parseDateInput;
import 'hr_calendar_page.dart';
import 'leave_accounts_page.dart';
import 'my_leave_page.dart' show leaveRequestLine, sickLine;

// Urlaubsantraege und Krankmeldungen fuer die Personalabteilung (Backlog
// F.8.6.2.1), Routen aus F.8.4: offene Antraege genehmigen oder ablehnen,
// genehmigte stornieren, neue (selbst gemeldete) Krankmeldungen bestaetigen,
// Attest festhalten, stornieren.
//
// Lesen mit leave_requests.read; entscheiden, stornieren und Krankmeldungen
// pflegen nur mit leave_requests.decide (heute Personalabteilung und
// Geschaeftsfuehrung, ADR 0029). Ob ein Login einen BESTIMMTEN Antrag
// entscheiden darf, prueft der Server an einer Stelle (vorbereitet fuer
// Teamleiter) - die Seite zeigt die Buttons nach dem Recht und meldet eine
// Ablehnung des Servers.

// employeeNames: Mitarbeiter-ID -> "Nachname, Vorname".
Map<String, String> employeeNames(List<dynamic> employees) {
  final out = <String, String>{};
  for (final e in employees) {
    final m = e as Map<String, dynamic>;
    final last = m['last_name']?.toString() ?? '';
    final first = m['first_name']?.toString() ?? '';
    out[m['id'].toString()] = first.isEmpty ? last : '$last, $first';
  }
  return out;
}

// withName: "Berg, Anna: <Zeile>"; unbekannte IDs als "Mitarbeiter".
String withName(Map<String, String> names, Map<String, dynamic> m, String line) =>
    '${names[m['employee_id']?.toString()] ?? 'Mitarbeiter'}: $line';

// approvedFromYear: genehmigte Antraege, die im Jahr oder spaeter enden,
// nach Beginn sortiert - die Kandidaten fuer ein Storno.
List<Map<String, dynamic>> approvedFromYear(List<dynamic> approved, int year) {
  final out = approved
      .map((e) => e as Map<String, dynamic>)
      .where((m) => (m['end_date']?.toString() ?? '').compareTo('$year-01-01') >= 0)
      .toList()
    ..sort((a, b) => (a['start_date']?.toString() ?? '').compareTo(b['start_date']?.toString() ?? ''));
  return out;
}

class LeaveAdminPage extends StatefulWidget {
  const LeaveAdminPage({super.key, required this.api, this.now});
  final ApiClient api;
  final DateTime Function()? now;

  @override
  State<LeaveAdminPage> createState() => _LeaveAdminPageState();
}

class _LeaveAdminPageState extends State<LeaveAdminPage> {
  List<Map<String, dynamic>> pending = [];
  List<Map<String, dynamic>> approved = [];
  List<dynamic> newSick = [];
  Map<String, String> names = {};
  List<Map<String, dynamic>> activeEmployees = [];
  bool loading = false;
  String? error;

  bool get _canDecide => widget.api.hasPermission('leave_requests.decide');

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
      final year = (widget.now ?? DateTime.now)().year;
      final open = await widget.api.listLeaveRequests(status: 'pending', limit: 500);
      final ok = await widget.api.listLeaveRequests(status: 'approved', limit: 500);
      final sick = await widget.api.listNewSickReports();
      final emps = widget.api.hasPermission('employees.read') ? await widget.api.listEmployees(limit: 500) : const <dynamic>[];
      if (!mounted) return;
      setState(() {
        pending = open.map((e) => e as Map<String, dynamic>).toList()
          ..sort((a, b) => (a['start_date']?.toString() ?? '').compareTo(b['start_date']?.toString() ?? ''));
        approved = approvedFromYear(ok, year);
        newSick = sick;
        names = employeeNames(emps);
        activeEmployees = emps.map((e) => e as Map<String, dynamic>).where((e) => e['active'] != false).toList()
          ..sort((a, b) => names[a['id'].toString()]!.compareTo(names[b['id'].toString()]!));
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => error = 'Daten konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted) setState(() => error = 'Daten konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _run(Future<void> Function() action, String success) async {
    try {
      await action();
      _snack(success);
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<bool> _confirm(String title, String text, String action) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(title),
        content: Text(text),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: Text(action)),
        ],
      ),
    );
    return ok == true;
  }

  // _reason fragt eine Pflicht-Begruendung ab; null = abgebrochen.
  Future<String?> _reason(String title, String text) async {
    final ctrl = TextEditingController();
    return showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(title),
          content: SizedBox(
            width: 440,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(text),
                TextField(
                  controller: ctrl,
                  decoration: const InputDecoration(labelText: 'Begründung'),
                  onChanged: (_) => setStateDialog(() {}),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: ctrl.text.trim().isEmpty ? null : () => Navigator.of(ctx).pop(ctrl.text.trim()),
              child: const Text('Stornieren'),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _decide(Map<String, dynamic> lr, bool approve) async {
    final line = withName(names, lr, leaveRequestLine(lr));
    if (!await _confirm(approve ? 'Antrag genehmigen' : 'Antrag ablehnen', line, approve ? 'Genehmigen' : 'Ablehnen')) return;
    await _run(() => widget.api.decideLeaveRequest(lr['id'].toString(), approve), approve ? 'Antrag genehmigt' : 'Antrag abgelehnt');
  }

  Future<void> _stornoRequest(Map<String, dynamic> lr) async {
    final reason = await _reason('Urlaub stornieren',
        '${withName(names, lr, leaveRequestLine(lr))}\n\nDie Tage gehen auf das Urlaubskonto zurück.');
    if (reason == null) return;
    await _run(() => widget.api.stornoLeaveRequest(lr['id'].toString(), reason), 'Urlaub storniert');
  }

  Future<void> _stornoSick(Map<String, dynamic> a) async {
    final reason = await _reason('Krankmeldung stornieren', withName(names, a, sickLine(a)));
    if (reason == null) return;
    await _run(() => widget.api.stornoAbsence(a['id'].toString(), reason), 'Krankmeldung storniert');
  }

  // Krankmeldung fuer einen Mitarbeiter: gilt als bestaetigt, Attest wie
  // angegeben. Nur Zeitraum und Attest - keine Diagnose (ADR 0029).
  Future<void> _reportSickDialog() async {
    final t = (widget.now ?? DateTime.now)();
    String two(int v) => v.toString().padLeft(2, '0');
    final today = '${two(t.day)}.${two(t.month)}.${t.year}';
    final fromCtrl = TextEditingController(text: today);
    final toCtrl = TextEditingController(text: today);
    String? employeeId;
    bool attest = false;
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Krankmeldung erfassen'),
          content: SizedBox(
            width: 440,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                DropdownButtonFormField<String>(
                  initialValue: employeeId,
                  isExpanded: true,
                  decoration: const InputDecoration(labelText: 'Mitarbeiter'),
                  items: [
                    for (final e in activeEmployees)
                      DropdownMenuItem(value: e['id'].toString(), child: Text(names[e['id'].toString()] ?? '')),
                  ],
                  onChanged: (v) => setStateDialog(() => employeeId = v),
                ),
                TextField(controller: fromCtrl, decoration: const InputDecoration(labelText: 'Erster Krankheitstag (TT.MM.JJJJ)')),
                TextField(controller: toCtrl, decoration: const InputDecoration(labelText: 'Letzter Tag (TT.MM.JJJJ)')),
                CheckboxListTile(
                  contentPadding: EdgeInsets.zero,
                  value: attest,
                  onChanged: (v) => setStateDialog(() => attest = v ?? false),
                  title: const Text('Attest liegt vor'),
                ),
                const Text('Erfasst wird nur der Zeitraum - keine Diagnose.'),
                if (dialogError != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(dialogError!, style: const TextStyle(color: Color(0xFFB00020))),
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final from = parseDateInput(fromCtrl.text);
                final to = parseDateInput(toCtrl.text);
                String? problem;
                if (employeeId == null) {
                  problem = 'Bitte den Mitarbeiter wählen.';
                } else if (from == null || to == null) {
                  problem = 'Bitte beide Tage als TT.MM.JJJJ angeben.';
                } else if (to.compareTo(from) < 0) {
                  problem = 'Der letzte Tag liegt vor dem ersten.';
                }
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop({'employee_id': employeeId, 'start_date': from, 'end_date': to, 'attest': attest});
              },
              child: const Text('Erfassen'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    await _run(() => widget.api.reportSick(body), 'Krankmeldung erfasst');
  }

  Widget _header(String text) => Padding(
        padding: const EdgeInsets.only(top: 16, bottom: 4),
        child: Text(text, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
      );

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Urlaubsanträge und Krankmeldungen'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Wrap(
              spacing: 12,
              runSpacing: 8,
              children: [
                if (_canDecide && activeEmployees.isNotEmpty)
                  FilledButton(onPressed: _reportSickDialog, child: const Text('Krankmeldung erfassen')),
                if (widget.api.hasPermission('employees.read'))
                  OutlinedButton(
                    onPressed: () => Navigator.of(context)
                        .push(MaterialPageRoute(builder: (_) => LeaveAccountsPage(api: widget.api, now: widget.now)))
                        .then((_) => _load()),
                    child: const Text('Urlaubskonten und Resturlaub'),
                  ),
                if (_canDecide)
                  OutlinedButton(
                    onPressed: () =>
                        Navigator.of(context).push(MaterialPageRoute(builder: (_) => HrCalendarPage(api: widget.api, now: widget.now))),
                    child: const Text('Feiertage und Betriebsruhe'),
                  ),
              ],
            ),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (error == null) ...[
              _header('Offene Anträge'),
              if (!loading && pending.isEmpty) const Text('Keine offenen Anträge.'),
              for (final lr in pending)
                Wrap(
                  crossAxisAlignment: WrapCrossAlignment.center,
                  spacing: 8,
                  children: [
                    Text(withName(names, lr, leaveRequestLine(lr))),
                    if (_canDecide) ...[
                      FilledButton.tonal(onPressed: () => _decide(lr, true), child: const Text('Genehmigen')),
                      TextButton(onPressed: () => _decide(lr, false), child: const Text('Ablehnen')),
                    ],
                  ],
                ),
              _header('Neue Krankmeldungen'),
              if (!loading && newSick.isEmpty) const Text('Keine neuen Krankmeldungen.'),
              for (final a in newSick.cast<Map<String, dynamic>>())
                Wrap(
                  crossAxisAlignment: WrapCrossAlignment.center,
                  spacing: 8,
                  children: [
                    Text(withName(names, a, sickLine(a))),
                    if (_canDecide) ...[
                      FilledButton.tonal(
                        onPressed: () => _run(() => widget.api.confirmAbsence(a['id'].toString()), 'Krankmeldung bestätigt'),
                        child: const Text('Bestätigen'),
                      ),
                      TextButton(
                        onPressed: () => _run(() => widget.api.setAbsenceAttest(a['id'].toString(), a['attest'] != true),
                            a['attest'] == true ? 'Attest entfernt' : 'Attest erfasst'),
                        child: Text(a['attest'] == true ? 'Attest entfernen' : 'Attest liegt vor'),
                      ),
                      TextButton(onPressed: () => _stornoSick(a), child: const Text('Stornieren')),
                    ],
                  ],
                ),
              _header('Genehmigter Urlaub (dieses Jahr und später)'),
              if (!loading && approved.isEmpty) const Text('Kein genehmigter Urlaub.'),
              for (final lr in approved)
                Wrap(
                  crossAxisAlignment: WrapCrossAlignment.center,
                  spacing: 8,
                  children: [
                    Text(withName(names, lr, leaveRequestLine(lr))),
                    if (_canDecide) TextButton(onPressed: () => _stornoRequest(lr), child: const Text('Stornieren')),
                  ],
                ),
            ],
          ],
        ),
      ),
    );
  }
}
