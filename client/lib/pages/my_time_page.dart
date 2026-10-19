import 'package:flutter/material.dart';
import '../api.dart';

// "Meine Arbeitszeit" (Backlog F.5.1): Kommen und Gehen fuer den eigenen
// Mitarbeiter, laufender Eintrag, aktuelle Woche mit Tagessummen und den
// Hinweisen der Pruefung nach Arbeitszeitgesetz (Server, F.1).
//
// Gestempelt wird mit der Uhr des SERVERS - der Client sendet beim Kommen
// und Gehen keine Zeit. Voraussetzung: der Login ist mit einem
// Mitarbeiter verknuepft (Personalabteilung, F.7.2).

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

const notLinkedMessage = 'Kein Mitarbeiter mit diesem Benutzer verknüpft';

class MyTimePage extends StatefulWidget {
  const MyTimePage({super.key, required this.api, this.now});
  final ApiClient api;
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

  DateTime _now() => (widget.now ?? DateTime.now)();

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
      final range = weekRange(_now());
      final sheet = await widget.api.listTimeEntries(from: range.from, to: range.to);
      if (!mounted) return;
      setState(() {
        entries = (sheet['entries'] as List?) ?? const [];
        days = (sheet['days'] as List?) ?? const [];
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => error = e.message == notLinkedMessage
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
        if ((m['work_date']?.toString() ?? '').startsWith(workDate) && m['status'] == 'active') {
          final ende = m['ended_at'] == null ? 'läuft' : formatClock(m['ended_at']?.toString());
          final note = (m['note']?.toString() ?? '').isEmpty ? '' : ' · ${m['note']}';
          out.add(Text('${formatClock(m['started_at']?.toString())} – $ende, Pause ${m['break_minutes'] ?? 0} min$note'));
        }
      }
      for (final h in (day['hinweise'] as List?) ?? const []) {
        out.add(Text('⚠ $h', style: const TextStyle(color: Color(0xFFB26A00))));
      }
    }
    return out;
  }

  @override
  Widget build(BuildContext context) {
    final range = weekRange(_now());
    return Scaffold(
      appBar: AppBar(
        title: const Text('Meine Arbeitszeit'),
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
              _statusCard(),
              const SizedBox(height: 16),
              Text('Woche ${formatWorkDay(range.from)} – ${formatWorkDay(range.to)}',
                  style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
              if (!loading && days.isEmpty) const Text('Diese Woche noch keine Zeiten erfasst.'),
              ..._week(),
            ],
          ],
        ),
      ),
    );
  }
}
