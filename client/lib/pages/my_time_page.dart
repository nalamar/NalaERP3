import 'package:flutter/material.dart';
import '../api.dart';
import 'employee_qualifications_section.dart' show parseDateInput;

// "Meine Arbeitszeit" (Backlog F.5.1, F.5.2): Kommen und Gehen fuer den
// eigenen Mitarbeiter, laufender Eintrag, Woche mit Tagessummen und den
// Hinweisen der Pruefung nach Arbeitszeitgesetz (Server, F.1); Zeiten
// nachtragen, korrigieren und stornieren, Wochen blaettern.
//
// Mit employeeId (F.5.3) dieselbe Ansicht fuer die Personalabteilung: die
// Zeiten eines anderen Mitarbeiters, lesen mit time_entries.read,
// nachtragen/korrigieren/stornieren mit time_entries.write. Kommen und
// Gehen gibt es dort nicht - stempeln kann nur der Mitarbeiter selbst.
//
// Beim Kommen und Gehen gilt die Uhr des SERVERS - der Client sendet dann
// keine Zeit. Beim Nachtragen und Korrigieren gibt der Mitarbeiter die
// Zeiten in der Ortszeit des Geraets an; sie werden nach UTC umgerechnet.
// Eintraege werden nie geaendert oder geloescht: Korrigieren storniert den
// alten Eintrag und legt einen neuen an, jeweils mit Begruendung (F.1).

String _two(int v) => v.toString().padLeft(2, '0');
String _iso(DateTime d) => '${d.year.toString().padLeft(4, '0')}-${_two(d.month)}-${_two(d.day)}';

// weekRange liefert Montag und Sonntag der Woche, in der now liegt, als
// JJJJ-MM-TT.
({String from, String to}) weekRange(DateTime now) {
  final day = DateTime(now.year, now.month, now.day);
  final monday = day.subtract(Duration(days: day.weekday - DateTime.monday));
  final sunday = monday.add(const Duration(days: 6));
  return (from: _iso(monday), to: _iso(sunday));
}

// formatMinutes: 465 -> "7:45 h".
String formatMinutes(int minutes) => '${minutes ~/ 60}:${_two(minutes % 60)} h';

// formatClock: Zeitpunkt des Servers (RFC 3339) als HH:MM in der Ortszeit
// des Geraets.
String formatClock(String? iso) {
  if (iso == null || iso.isEmpty) return '';
  final t = DateTime.tryParse(iso);
  if (t == null) return '';
  final l = t.toLocal();
  return '${_two(l.hour)}:${_two(l.minute)}';
}

const _weekdays = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'];

// formatWorkDay: "2026-10-13" -> "Di 13.10.".
String formatWorkDay(String workDate) {
  final d = DateTime.tryParse(workDate.length >= 10 ? workDate.substring(0, 10) : workDate);
  if (d == null) return workDate;
  return '${_weekdays[d.weekday - 1]} ${_two(d.day)}.${_two(d.month)}.';
}

// findRunningEntry: der aktive Eintrag ohne Ende - hoechstens einer, das
// garantiert der Server.
Map<String, dynamic>? findRunningEntry(List<dynamic> entries) {
  for (final e in entries) {
    final m = e as Map<String, dynamic>;
    if (m['status'] == 'active' && m['ended_at'] == null) return m;
  }
  return null;
}

// parseClockInput liest eine Uhrzeit H:MM oder HH:MM und liefert die
// Minuten seit Mitternacht; null bei ungueltiger Eingabe.
int? parseClockInput(String input) {
  final m = RegExp(r'^(\d{1,2}):(\d{2})$').firstMatch(input.trim());
  if (m == null) return null;
  final h = int.parse(m.group(1)!);
  final min = int.parse(m.group(2)!);
  if (h > 23 || min > 59) return null;
  return h * 60 + min;
}

class EntryTimes {
  const EntryTimes({required this.startedAt, required this.endedAt, required this.breakMinutes, required this.overnight});
  final String startedAt;
  final String endedAt;
  final int breakMinutes;
  // Ende am Folgetag (Nachtschicht).
  final bool overnight;
}

// parseEntryForm prueft die Eingaben eines Nachtrags bzw. einer Korrektur
// und rechnet sie in UTC um. Liegt das Ende vor dem Beginn, gilt es als
// Folgetag (Nachtschicht). Liefert entweder die Zeiten oder einen Fehler.
({EntryTimes? times, String? error}) parseEntryForm({
  required String date,
  required String start,
  required String end,
  required String pause,
}) {
  final day = parseDateInput(date);
  if (day == null) return (times: null, error: 'Bitte das Datum als TT.MM.JJJJ angeben.');
  final s = parseClockInput(start);
  if (s == null) return (times: null, error: 'Bitte den Beginn als HH:MM angeben.');
  final e = parseClockInput(end);
  if (e == null) return (times: null, error: 'Bitte das Ende als HH:MM angeben.');
  if (e == s) return (times: null, error: 'Beginn und Ende dürfen nicht gleich sein.');
  final p = int.tryParse(pause.trim());
  if (p == null || p < 0) {
    return (times: null, error: 'Bitte die Pause als ganze Zahl von Minuten angeben (0 für keine).');
  }
  final d = DateTime.parse(day);
  final startLocal = DateTime(d.year, d.month, d.day, s ~/ 60, s % 60);
  final overnight = e < s;
  final endDay = overnight ? DateTime(d.year, d.month, d.day + 1) : d;
  final endLocal = DateTime(endDay.year, endDay.month, endDay.day, e ~/ 60, e % 60);
  return (
    times: EntryTimes(
      startedAt: startLocal.toUtc().toIso8601String(),
      endedAt: endLocal.toUtc().toIso8601String(),
      breakMinutes: p,
      overnight: overnight,
    ),
    error: null,
  );
}

const notLinkedMessage = 'Kein Mitarbeiter mit diesem Benutzer verknüpft';

class MyTimePage extends StatefulWidget {
  const MyTimePage({super.key, required this.api, this.now, this.employeeId, this.employeeName});
  final ApiClient api;
  // Fremder Mitarbeiter (Personalabteilung); null = die eigenen Zeiten.
  final String? employeeId;
  final String? employeeName;
  // Uhr fuer die Wochenauswahl; in Tests fest vorgegeben.
  final DateTime Function()? now;

  @override
  State<MyTimePage> createState() => _MyTimePageState();
}

class _MyTimePageState extends State<MyTimePage> {
  List<dynamic> entries = [];
  List<dynamic> days = [];
  bool loading = false;
  bool busy = false;
  String? error;
  // 0 = aktuelle Woche, -1 = Vorwoche, ...
  int weekOffset = 0;
  bool showStorniert = false;

  DateTime _now() => (widget.now ?? DateTime.now)();
  bool get _forOther => widget.employeeId != null;
  // Eigene Zeiten bearbeitet der Mitarbeiter mit time_entries.self (der
  // Server prueft); fremde nur mit time_entries.write.
  bool get _canEdit => !_forOther || widget.api.hasPermission('time_entries.write');
  DateTime _weekAnchor() => _now().add(Duration(days: 7 * weekOffset));

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
      final range = weekRange(_weekAnchor());
      final sheet = await widget.api
          .listTimeEntries(from: range.from, to: range.to, employeeId: widget.employeeId, includeStorniert: showStorniert);
      if (!mounted) return;
      setState(() {
        entries = (sheet['entries'] as List?) ?? const [];
        days = (sheet['days'] as List?) ?? const [];
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => error = e.message == notLinkedMessage && !_forOther
          ? 'Ihr Login ist mit keinem Mitarbeiter verknüpft. Bitte wenden Sie sich an die Personalabteilung.'
          : 'Arbeitszeiten konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted) setState(() => error = 'Arbeitszeiten konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  void _goToWeek(int offset) {
    setState(() => weekOffset = offset);
    _load();
  }

  Future<void> _clockIn() async {
    setState(() => busy = true);
    try {
      final e = await widget.api.clockIn();
      _snack('Eingestempelt um ${formatClock(e['started_at']?.toString())}');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> _clockOut(Map<String, dynamic> running) async {
    final pauseCtrl = TextEditingController(text: '0');
    String? dialogError;
    final minutes = await showDialog<int>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Gehen'),
          content: SizedBox(
            width: 360,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Eingestempelt seit ${formatClock(running['started_at']?.toString())}.'),
                TextField(
                  controller: pauseCtrl,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(labelText: 'Pause (Minuten)'),
                ),
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
                final v = int.tryParse(pauseCtrl.text.trim());
                if (v == null || v < 0) {
                  setStateDialog(() => dialogError = 'Bitte die Pause als ganze Zahl von Minuten angeben (0 für keine).');
                  return;
                }
                Navigator.of(ctx).pop(v);
              },
              child: const Text('Ausstempeln'),
            ),
          ],
        ),
      ),
    );
    if (minutes == null) return;
    setState(() => busy = true);
    try {
      final e = await widget.api.clockOut(running['id'].toString(), breakMinutes: minutes);
      _snack('Ausgestempelt um ${formatClock(e['ended_at']?.toString())}');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  String _localDate(String? iso) {
    final t = iso == null ? null : DateTime.tryParse(iso);
    if (t == null) return '';
    final l = t.toLocal();
    return '${_two(l.day)}.${_two(l.month)}.${l.year}';
  }

  // _entryDialog: Nachtragen (entry == null) oder Korrigieren (mit
  // Begruendung). Liefert die Zeiten, Notiz und ggf. Begruendung.
  Future<({EntryTimes times, String note, String reason})?> _entryDialog({Map<String, dynamic>? entry}) {
    final correcting = entry != null;
    final today = _now();
    final dateCtrl = TextEditingController(
        text: correcting ? _localDate(entry['started_at']?.toString()) : '${_two(today.day)}.${_two(today.month)}.${today.year}');
    final startCtrl = TextEditingController(text: correcting ? formatClock(entry['started_at']?.toString()) : '');
    final endCtrl = TextEditingController(text: correcting ? formatClock(entry['ended_at']?.toString()) : '');
    final pauseCtrl = TextEditingController(text: correcting ? '${entry['break_minutes'] ?? 0}' : '0');
    final noteCtrl = TextEditingController(text: correcting ? (entry['note']?.toString() ?? '') : '');
    final reasonCtrl = TextEditingController();
    String? dialogError;
    return showDialog<({EntryTimes times, String note, String reason})>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(correcting ? 'Zeit korrigieren' : 'Zeit nachtragen'),
          content: SizedBox(
            width: 380,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(controller: dateCtrl, decoration: const InputDecoration(labelText: 'Datum (TT.MM.JJJJ)')),
                  TextField(controller: startCtrl, decoration: const InputDecoration(labelText: 'Beginn (HH:MM)')),
                  TextField(controller: endCtrl, decoration: const InputDecoration(labelText: 'Ende (HH:MM, vor Beginn = Folgetag)')),
                  TextField(controller: pauseCtrl, decoration: const InputDecoration(labelText: 'Pause (Minuten)')),
                  TextField(controller: noteCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
                  if (correcting)
                    TextField(controller: reasonCtrl, decoration: const InputDecoration(labelText: 'Begründung der Korrektur')),
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
                final parsed = parseEntryForm(date: dateCtrl.text, start: startCtrl.text, end: endCtrl.text, pause: pauseCtrl.text);
                if (parsed.error != null) {
                  setStateDialog(() => dialogError = parsed.error);
                  return;
                }
                if (correcting && reasonCtrl.text.trim().isEmpty) {
                  setStateDialog(() => dialogError = 'Bitte die Korrektur begründen.');
                  return;
                }
                Navigator.of(ctx).pop((times: parsed.times!, note: noteCtrl.text.trim(), reason: reasonCtrl.text.trim()));
              },
              child: Text(correcting ? 'Korrigieren' : 'Nachtragen'),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _addEntry() async {
    final r = await _entryDialog();
    if (r == null) return;
    try {
      await widget.api.createTimeEntry(
          startedAt: r.times.startedAt,
          endedAt: r.times.endedAt,
          breakMinutes: r.times.breakMinutes,
          note: r.note,
          employeeId: widget.employeeId);
      _snack('Zeit nachgetragen');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _correctEntry(Map<String, dynamic> entry) async {
    final r = await _entryDialog(entry: entry);
    if (r == null) return;
    try {
      await widget.api.correctTimeEntry(entry['id'].toString(),
          startedAt: r.times.startedAt,
          endedAt: r.times.endedAt,
          breakMinutes: r.times.breakMinutes,
          note: r.note,
          reason: r.reason);
      _snack('Zeit korrigiert');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _stornoEntry(Map<String, dynamic> entry) async {
    final reasonCtrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Eintrag stornieren'),
          content: SizedBox(
            width: 380,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${formatClock(entry['started_at']?.toString())} – '
                    '${entry['ended_at'] == null ? 'läuft' : formatClock(entry['ended_at']?.toString())}'),
                const SizedBox(height: 8),
                const Text('Ein Eintrag wird nicht gelöscht, sondern mit Begründung storniert.'),
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
      await widget.api.stornoTimeEntry(entry['id'].toString(), reason);
      _snack('Eintrag storniert');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Widget _statusCard() {
    final running = findRunningEntry(entries);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Icon(running != null ? Icons.timer : Icons.timer_off, size: 32),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                running != null ? 'Eingestempelt seit ${formatClock(running['started_at']?.toString())}' : 'Nicht eingestempelt',
                style: const TextStyle(fontSize: 18),
              ),
            ),
            if (running != null)
              FilledButton.icon(
                onPressed: busy ? null : () => _clockOut(running),
                icon: const Icon(Icons.logout),
                label: const Text('Gehen'),
              )
            else
              FilledButton.icon(
                onPressed: busy ? null : _clockIn,
                icon: const Icon(Icons.login),
                label: const Text('Kommen'),
              ),
          ],
        ),
      ),
    );
  }

  Widget _entryRow(Map<String, dynamic> m) {
    final storniert = m['status'] == 'storniert';
    final ende = m['ended_at'] == null ? 'läuft' : formatClock(m['ended_at']?.toString());
    final note = (m['note']?.toString() ?? '').isEmpty ? '' : ' · ${m['note']}';
    final text = '${formatClock(m['started_at']?.toString())} – $ende, Pause ${m['break_minutes'] ?? 0} min$note';
    if (storniert) {
      return Text('$text – storniert: ${m['storno_reason'] ?? ''}',
          style: const TextStyle(color: Colors.grey, decoration: TextDecoration.lineThrough));
    }
    if (!_canEdit) return Text(text);
    return Row(
      children: [
        Expanded(child: Text(text)),
        if (m['ended_at'] != null)
          IconButton(tooltip: 'Korrigieren', icon: const Icon(Icons.edit, size: 18), onPressed: () => _correctEntry(m)),
        IconButton(tooltip: 'Stornieren', icon: const Icon(Icons.block, size: 18), onPressed: () => _stornoEntry(m)),
      ],
    );
  }

  List<Widget> _week() {
    final out = <Widget>[];
    for (final d in days) {
      final day = d as Map<String, dynamic>;
      final workDate = day['work_date']?.toString() ?? '';
      final open = day['open'] == true;
      out.add(Padding(
        padding: const EdgeInsets.only(top: 12),
        child: Text(
          '${formatWorkDay(workDate)}: ${formatMinutes((day['net_minutes'] as num?)?.toInt() ?? 0)}'
          ' (Pause ${(day['break_minutes'] as num?)?.toInt() ?? 0} min)${open ? ' – läuft noch' : ''}',
          style: const TextStyle(fontWeight: FontWeight.bold),
        ),
      ));
      for (final e in entries) {
        final m = e as Map<String, dynamic>;
        if ((m['work_date']?.toString() ?? '').startsWith(workDate) && (m['status'] == 'active' || showStorniert)) {
          out.add(_entryRow(m));
        }
      }
      for (final h in (day['hinweise'] as List?) ?? const []) {
        out.add(Text('⚠ $h', style: const TextStyle(color: Color(0xFFB26A00))));
      }
    }
    // Tage nur mit stornierten Eintraegen haben keine Auswertung - die
    // Eintraege sollen trotzdem sichtbar sein, wenn danach gefragt ist.
    if (showStorniert) {
      final known = days.map((d) => (d as Map<String, dynamic>)['work_date']?.toString() ?? '').toSet();
      for (final e in entries) {
        final m = e as Map<String, dynamic>;
        final wd = (m['work_date']?.toString() ?? '');
        if (m['status'] == 'storniert' && !known.any((k) => k.isNotEmpty && wd.startsWith(k))) {
          out.add(Padding(padding: const EdgeInsets.only(top: 12), child: Text(formatWorkDay(wd), style: const TextStyle(fontWeight: FontWeight.bold))));
          out.add(_entryRow(m));
        }
      }
    }
    return out;
  }

  @override
  Widget build(BuildContext context) {
    final range = weekRange(_weekAnchor());
    return Scaffold(
      appBar: AppBar(
        title: Text(_forOther ? 'Arbeitszeiten: ${widget.employeeName ?? ''}' : 'Meine Arbeitszeit'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null)
              Text(error!, style: const TextStyle(color: Color(0xFFB00020)))
            else ...[
              // Kommen/Gehen gilt fuer jetzt - nur in der aktuellen Woche.
              if (weekOffset == 0 && !_forOther) _statusCard(),
              const SizedBox(height: 16),
              Wrap(
                crossAxisAlignment: WrapCrossAlignment.center,
                spacing: 8,
                runSpacing: 8,
                children: [
                  IconButton(tooltip: 'Vorwoche', icon: const Icon(Icons.chevron_left), onPressed: () => _goToWeek(weekOffset - 1)),
                  Text('Woche ${formatWorkDay(range.from)} – ${formatWorkDay(range.to)}',
                      style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                  IconButton(tooltip: 'Nächste Woche', icon: const Icon(Icons.chevron_right), onPressed: () => _goToWeek(weekOffset + 1)),
                  if (weekOffset != 0) TextButton(onPressed: () => _goToWeek(0), child: const Text('Diese Woche')),
                  if (_canEdit)
                    OutlinedButton.icon(onPressed: _addEntry, icon: const Icon(Icons.add), label: const Text('Zeit nachtragen')),
                  FilterChip(
                    label: const Text('Stornierte anzeigen'),
                    selected: showStorniert,
                    onSelected: (v) {
                      setState(() => showStorniert = v);
                      _load();
                    },
                  ),
                ],
              ),
              if (!loading && days.isEmpty && !(showStorniert && entries.isNotEmpty))
                const Text('In dieser Woche keine Zeiten erfasst.'),
              ..._week(),
            ],
          ],
        ),
      ),
    );
  }
}
