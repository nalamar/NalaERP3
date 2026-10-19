import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;
import 'my_time_page.dart' show notLinkedMessage;

// "Mein Urlaub" (Backlog F.8.6.1), Routen aus F.8.3/F.8.4: das eigene
// Urlaubskonto eines Jahres, die eigenen Antraege (stellen, offene
// zurueckziehen) und die eigenen Krankmeldungen (selbst melden). Recht
// leave_requests.self; der Server prueft, dass nur der eigene Mitarbeiter
// betroffen ist.
//
// Krankheit wird gemeldet, nicht beantragt. Erfasst werden nur Zeitraum -
// keine Diagnose, kein Grund (ADR 0029). Ob ein Attest vorliegt, haelt die
// Personalabteilung fest.

const leaveTypeLabels = {
  'vacation': 'Urlaub',
  'unpaid': 'unbezahlter Urlaub',
  'other': 'Sonstiges',
  'sick': 'Krankheit',
};

// Beantragbar sind alle Typen ausser Krankheit.
const requestableLeaveTypes = ['vacation', 'unpaid', 'other'];

const leaveStatusLabels = {
  'pending': 'offen',
  'approved': 'genehmigt',
  'rejected': 'abgelehnt',
  'withdrawn': 'zurückgezogen',
  'storniert': 'storniert',
};

// formatLeaveDays: "1 Tag", "5 Tage", "0,5 Tage", "4,5 Tage".
String formatLeaveDays(num? v) {
  final d = (v ?? 0).toDouble();
  final text = d == d.roundToDouble() ? d.toInt().toString() : d.toString().replaceAll('.', ',');
  return d == 1 ? '1 Tag' : '$text Tage';
}

String _range(Map<String, dynamic> m) {
  final from = isoToGerman(m['start_date']?.toString());
  final to = isoToGerman(m['end_date']?.toString());
  return from == to ? from : '$from – $to';
}

// leaveRequestLine: "08.06.2026 – 12.06.2026 · 5 Tage · Urlaub · genehmigt".
String leaveRequestLine(Map<String, dynamic> lr) {
  final parts = <String>[
    _range(lr),
    formatLeaveDays(lr['days'] as num?),
    leaveTypeLabels[lr['typ']] ?? (lr['typ']?.toString() ?? ''),
    leaveStatusLabels[lr['status']] ?? (lr['status']?.toString() ?? ''),
  ];
  final reason = lr['reason']?.toString() ?? '';
  if (reason.isNotEmpty) parts.add(reason);
  final storno = lr['storno_reason']?.toString() ?? '';
  if (storno.isNotEmpty) parts.add('Storno: $storno');
  return parts.join(' · ');
}

// sickLine: "10.06.2026 – 11.06.2026 · Attest liegt vor · noch nicht bestätigt".
String sickLine(Map<String, dynamic> a) {
  final parts = <String>[_range(a), a['attest'] == true ? 'Attest liegt vor' : 'kein Attest erfasst'];
  if (a['status'] == 'storniert') {
    parts.add('storniert');
  } else if (a['gemeldet_von'] == 'selbst' && a['bestaetigt_at'] == null) {
    parts.add('noch nicht bestätigt');
  }
  return parts.join(' · ');
}

// accountLines beschreibt den Kontostand in Worten.
List<String> accountLines(Map<String, dynamic> acc) {
  final lines = <String>[
    'Anspruch: ${formatLeaveDays(acc['anspruch_tage'] as num?)}',
  ];
  final uebertrag = (acc['uebertrag_tage'] as num?) ?? 0;
  if (uebertrag != 0) lines.add('Übertrag aus dem Vorjahr: ${formatLeaveDays(uebertrag)}');
  lines.add('Genommen: ${formatLeaveDays(acc['genommen_tage'] as num?)}');
  final krank = (acc['krank_im_urlaub_tage'] as num?) ?? 0;
  if (krank != 0) lines.add('Krank im Urlaub (gutgeschrieben): ${formatLeaveDays(krank)}');
  lines.add('Beantragt, noch offen: ${formatLeaveDays(acc['verplant_tage'] as num?)}');
  lines.add('Resturlaub: ${formatLeaveDays(acc['rest_tage'] as num?)}');
  lines.add('Noch verfügbar: ${formatLeaveDays(acc['verfuegbar_tage'] as num?)}');
  return lines;
}

// parseLeaveDays liest eine Tageszahl ("4,5", "4.5", "5"); leer = null
// (der Server zaehlt die Arbeitstage). ok=false bei ungueltiger Eingabe,
// 0 oder keinem halben Schritt.
({bool ok, double? value}) parseLeaveDays(String input) {
  final t = input.trim().replaceAll(',', '.');
  if (t.isEmpty) return (ok: true, value: null);
  final v = double.tryParse(t);
  if (v == null || v <= 0 || (v * 2) != (v * 2).roundToDouble()) return (ok: false, value: null);
  return (ok: true, value: v);
}

// buildLeaveRequestBody prueft die Eingaben und baut den Body fuer
// POST /leave-requests. Fehlertext statt Body bei ungueltiger Eingabe.
({Map<String, dynamic>? body, String? error}) buildLeaveRequestBody({
  required String typ,
  required String from,
  required String to,
  String daysText = '',
  String reason = '',
}) {
  final start = parseDateInput(from);
  final end = parseDateInput(to);
  if (start == null) return (body: null, error: 'Bitte den ersten Tag als TT.MM.JJJJ angeben.');
  if (end == null) return (body: null, error: 'Bitte den letzten Tag als TT.MM.JJJJ angeben.');
  if (end.compareTo(start) < 0) return (body: null, error: 'Der letzte Tag liegt vor dem ersten.');
  final days = parseLeaveDays(daysText);
  if (!days.ok) return (body: null, error: 'Bitte die Tage als ganze oder halbe Zahl angeben oder leer lassen.');
  final body = <String, dynamic>{'typ': typ, 'start_date': start, 'end_date': end};
  if (days.value != null) body['days'] = days.value;
  if (reason.trim().isNotEmpty) body['reason'] = reason.trim();
  return (body: body, error: null);
}

// buildSickBody: nur Zeitraum - keine weiteren Angaben (ADR 0029).
({Map<String, dynamic>? body, String? error}) buildSickBody({required String from, required String to}) {
  final start = parseDateInput(from);
  final end = parseDateInput(to);
  if (start == null) return (body: null, error: 'Bitte den ersten Krankheitstag als TT.MM.JJJJ angeben.');
  if (end == null) return (body: null, error: 'Bitte den letzten Krankheitstag als TT.MM.JJJJ angeben.');
  if (end.compareTo(start) < 0) return (body: null, error: 'Der letzte Tag liegt vor dem ersten.');
  return (body: {'start_date': start, 'end_date': end}, error: null);
}

// overlapsYear: beruehrt der Zeitraum (start_date/end_date) das Jahr?
bool overlapsYear(Map<String, dynamic> m, int year) {
  final from = m['start_date']?.toString() ?? '';
  final to = m['end_date']?.toString() ?? '';
  if (from.length < 4 || to.length < 4) return false;
  return int.parse(from.substring(0, 4)) <= year && int.parse(to.substring(0, 4)) >= year;
}

String _two(int v) => v.toString().padLeft(2, '0');

class MyLeavePage extends StatefulWidget {
  const MyLeavePage({super.key, required this.api, this.now});
  final ApiClient api;
  // Uhr fuer das voreingestellte Jahr und die Datumsvorgabe; in Tests fest.
  final DateTime Function()? now;

  @override
  State<MyLeavePage> createState() => _MyLeavePageState();
}

class _MyLeavePageState extends State<MyLeavePage> {
  Map<String, dynamic>? account;
  List<dynamic> requests = [];
  List<dynamic> sick = [];
  bool loading = false;
  String? error;
  bool notLinked = false;
  late int year;

  DateTime _now() => (widget.now ?? DateTime.now)();

  @override
  void initState() {
    super.initState();
    year = _now().year;
    _load();
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final acc = await widget.api.getLeaveAccount(year: year);
      final reqs = await widget.api.listLeaveRequests(limit: 200);
      final abs = await widget.api.listAbsences(from: '$year-01-01', to: '$year-12-31');
      if (!mounted) return;
      setState(() {
        account = acc;
        requests = reqs.where((r) => overlapsYear(r as Map<String, dynamic>, year)).toList()
          ..sort((a, b) => (a['start_date']?.toString() ?? '').compareTo(b['start_date']?.toString() ?? ''));
        sick = abs.where((a) => (a as Map<String, dynamic>)['typ'] == 'sick' && a['source_request_id'] == null).toList();
        notLinked = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        notLinked = e.message == notLinkedMessage;
        error = notLinked
            ? 'Ihr Login ist mit keinem Mitarbeiter verknüpft. Bitte wenden Sie sich an die Personalabteilung.'
            : 'Urlaubsdaten konnten nicht geladen werden: ${e.message}';
      });
    } catch (e) {
      if (mounted) setState(() => error = 'Urlaubsdaten konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  void _goToYear(int y) {
    setState(() => year = y);
    _load();
  }

  String _today() {
    final t = _now();
    return '${_two(t.day)}.${_two(t.month)}.${t.year}';
  }

  Future<void> _requestDialog() async {
    final fromCtrl = TextEditingController(text: _today());
    final toCtrl = TextEditingController(text: _today());
    final daysCtrl = TextEditingController();
    final reasonCtrl = TextEditingController();
    String typ = 'vacation';
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Urlaub beantragen'),
          content: SizedBox(
            width: 420,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  DropdownButtonFormField<String>(
                    initialValue: typ,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Art'),
                    items: [for (final t in requestableLeaveTypes) DropdownMenuItem(value: t, child: Text(leaveTypeLabels[t]!))],
                    onChanged: (v) => setStateDialog(() => typ = v ?? typ),
                  ),
                  TextField(controller: fromCtrl, decoration: const InputDecoration(labelText: 'Erster Tag (TT.MM.JJJJ)')),
                  TextField(controller: toCtrl, decoration: const InputDecoration(labelText: 'Letzter Tag (TT.MM.JJJJ)')),
                  TextField(
                    controller: daysCtrl,
                    decoration: const InputDecoration(labelText: 'Tage (leer = Arbeitstage im Zeitraum, z. B. 0,5 für einen halben Tag)'),
                  ),
                  TextField(controller: reasonCtrl, decoration: const InputDecoration(labelText: 'Bemerkung (optional)')),
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
                final r = buildLeaveRequestBody(
                    typ: typ, from: fromCtrl.text, to: toCtrl.text, daysText: daysCtrl.text, reason: reasonCtrl.text);
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop(r.body);
              },
              child: const Text('Beantragen'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    try {
      final lr = await widget.api.createLeaveRequest(body);
      _snack('Antrag gestellt: ${formatLeaveDays(lr['days'] as num?)}');
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _sickDialog() async {
    final fromCtrl = TextEditingController(text: _today());
    final toCtrl = TextEditingController(text: _today());
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Krank melden'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('Erfasst wird nur der Zeitraum - keine Diagnose. Ein Attest reichen Sie bitte bei der Personalabteilung ein.'),
                TextField(controller: fromCtrl, decoration: const InputDecoration(labelText: 'Erster Krankheitstag (TT.MM.JJJJ)')),
                TextField(controller: toCtrl, decoration: const InputDecoration(labelText: 'Voraussichtlich letzter Tag (TT.MM.JJJJ)')),
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
                final r = buildSickBody(from: fromCtrl.text, to: toCtrl.text);
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop(r.body);
              },
              child: const Text('Melden'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    try {
      await widget.api.reportSick(body);
      _snack('Krankmeldung übermittelt - gute Besserung');
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _withdraw(Map<String, dynamic> lr) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Antrag zurückziehen'),
        content: Text(leaveRequestLine(lr)),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: const Text('Zurückziehen')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.api.withdrawLeaveRequest(lr['id'].toString());
      _snack('Antrag zurückgezogen');
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
    final thisYear = _now().year;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Mein Urlaub'),
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
                if (year != thisYear) TextButton(onPressed: () => _goToYear(thisYear), child: const Text('Dieses Jahr')),
              ],
            ),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!notLinked) ...[
              const SizedBox(height: 8),
              Wrap(
                spacing: 12,
                runSpacing: 8,
                children: [
                  FilledButton(onPressed: _requestDialog, child: const Text('Urlaub beantragen')),
                  OutlinedButton(onPressed: _sickDialog, child: const Text('Krank melden')),
                ],
              ),
            ],
            if (acc != null && error == null) ...[
              const SizedBox(height: 16),
              const Text('Urlaubskonto', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
              for (final line in accountLines(acc)) Text(line),
            ],
            if (error == null) ...[
              const SizedBox(height: 16),
              const Text('Meine Anträge', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
              if (!loading && requests.isEmpty) const Text('Keine Anträge in diesem Jahr.'),
              for (final r in requests)
                Row(
                  children: [
                    Expanded(child: Text(leaveRequestLine(r as Map<String, dynamic>))),
                    if (r['status'] == 'pending') TextButton(onPressed: () => _withdraw(r), child: const Text('Zurückziehen')),
                  ],
                ),
              const SizedBox(height: 16),
              const Text('Meine Krankmeldungen', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
              if (!loading && sick.isEmpty) const Text('Keine Krankmeldungen in diesem Jahr.'),
              for (final a in sick) Text(sickLine(a as Map<String, dynamic>)),
            ],
          ],
        ),
      ),
    );
  }
}
