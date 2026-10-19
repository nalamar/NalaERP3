import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'my_leave_page.dart' show leaveTypeLabels;

// Abwesenheitsuebersicht "wer fehlt wann" (Backlog F.8.6.3), Route aus
// F.8.4.3: ein Monat als Raster (Mitarbeiter mit Abwesenheit x Tage), dazu
// die Abwesenheiten als Liste. Wochenenden und die freien Tage des
// Mandanten (gesetzlich und betrieblich, F.8.2.2) sind hinterlegt.
//
// Fuer die Personalabteilung (leave_requests.read). Der Kalender zeigt die
// Feiertage des Mandanten - ein abweichendes Bundesland einzelner
// Mitarbeiter (F.8.2.3) wird im Raster nicht beruecksichtigt.

const monthNames = [
  'Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', //
  'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember',
];

const _weekdayLetters = ['M', 'D', 'M', 'D', 'F', 'S', 'S'];

// absenceCodes: Kuerzel im Raster.
const absenceCodes = {'vacation': 'U', 'sick': 'K', 'unpaid': 'B', 'other': 'S'};

String _two(int v) => v.toString().padLeft(2, '0');

int daysInMonth(int year, int month) => DateTime.utc(year, month + 1, 0).day;

// monthRange: erster und letzter Tag als JJJJ-MM-TT.
({String from, String to}) monthRange(int year, int month) =>
    (from: '$year-${_two(month)}-01', to: '$year-${_two(month)}-${_two(daysInMonth(year, month))}');

// AbsenceRow ist eine Zeile des Rasters: Name und Kuerzel je Tag.
class AbsenceRow {
  AbsenceRow(this.name);
  final String name;
  final Map<int, String> days = {};
}

String _nameOf(Map<String, dynamic> a) {
  final first = a['first_name']?.toString() ?? '';
  final last = a['last_name']?.toString() ?? '';
  return first.isEmpty ? last : '$last, $first';
}

// buildAbsenceRows: je Mitarbeiter eine Zeile mit den Tagen des Monats, an
// denen er fehlt; nach Namen sortiert. Ueberlappen sich zwei Abwesenheiten
// an einem Tag (Krankheit im Urlaub), gewinnt die Krankheit.
List<AbsenceRow> buildAbsenceRows(List<dynamic> absences, int year, int month) {
  final rows = <String, AbsenceRow>{};
  final first = DateTime.utc(year, month, 1);
  final last = DateTime.utc(year, month, daysInMonth(year, month));
  for (final e in absences) {
    final a = e as Map<String, dynamic>;
    final id = a['employee_id']?.toString() ?? '';
    final row = rows.putIfAbsent(id, () => AbsenceRow(_nameOf(a)));
    final code = absenceCodes[a['typ']] ?? '?';
    var d = DateTime.parse(a['start_date'].toString().substring(0, 10));
    final end = DateTime.parse(a['end_date'].toString().substring(0, 10));
    final start = DateTime.utc(d.year, d.month, d.day);
    d = start.isBefore(first) ? first : start;
    final stop = DateTime.utc(end.year, end.month, end.day).isAfter(last) ? last : DateTime.utc(end.year, end.month, end.day);
    for (; !d.isAfter(stop); d = d.add(const Duration(days: 1))) {
      if (row.days[d.day] == 'K') continue;
      row.days[d.day] = code;
    }
  }
  return rows.values.toList()..sort((a, b) => a.name.compareTo(b.name));
}

// freeDaysOfMonth: Tag -> Name der freien Tage des Mandanten im Monat.
Map<int, String> freeDaysOfMonth(Map<String, dynamic>? calendar, int year, int month) {
  final out = <int, String>{};
  final prefix = '$year-${_two(month)}-';
  for (final e in (calendar?['days'] as List?) ?? const []) {
    final d = (e as Map<String, dynamic>)['date']?.toString() ?? '';
    if (d.startsWith(prefix)) out[int.parse(d.substring(8, 10))] = e['name']?.toString() ?? '';
  }
  return out;
}

// absenceLine: "Berg, Anna: 08.06.2026 – 12.06.2026 · Urlaub".
String absenceLine(Map<String, dynamic> a) {
  final from = isoToGerman(a['start_date']?.toString());
  final to = isoToGerman(a['end_date']?.toString());
  final typ = leaveTypeLabels[a['typ']] ?? (a['typ']?.toString() ?? '');
  return '${_nameOf(a)}: ${from == to ? from : '$from – $to'} · $typ';
}

class AbsenceOverviewPage extends StatefulWidget {
  const AbsenceOverviewPage({super.key, required this.api, this.now});
  final ApiClient api;
  final DateTime Function()? now;

  @override
  State<AbsenceOverviewPage> createState() => _AbsenceOverviewPageState();
}

class _AbsenceOverviewPageState extends State<AbsenceOverviewPage> {
  late int year;
  late int month;
  String? teamId;
  List<dynamic> teams = [];
  List<dynamic> absences = [];
  Map<String, dynamic>? calendar;
  bool loading = false;
  String? error;

  @override
  void initState() {
    super.initState();
    final t = (widget.now ?? DateTime.now)();
    year = t.year;
    month = t.month;
    _loadTeams();
    _load();
  }

  Future<void> _loadTeams() async {
    if (!widget.api.hasPermission('employees.read')) return;
    try {
      final list = await widget.api.listTeams();
      if (mounted) setState(() => teams = list);
    } catch (_) {
      // Ohne Teamliste bleibt der Filter leer - die Uebersicht geht trotzdem.
    }
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final range = monthRange(year, month);
      final list = await widget.api.listAbsences(from: range.from, to: range.to, teamId: teamId);
      final cal = await widget.api.getHolidays(year: year);
      if (!mounted) return;
      setState(() {
        absences = list;
        calendar = cal;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => error = 'Abwesenheiten konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted) setState(() => error = 'Abwesenheiten konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _shiftMonth(int delta) {
    var m = month + delta;
    var y = year;
    if (m < 1) {
      m = 12;
      y--;
    } else if (m > 12) {
      m = 1;
      y++;
    }
    setState(() {
      year = y;
      month = m;
    });
    _load();
  }

  Color? _cellColor(String? code, bool free) {
    switch (code) {
      case 'U':
        return const Color(0xFF81C784);
      case 'K':
        return const Color(0xFFE57373);
      case 'B':
      case 'S':
        return const Color(0xFF90CAF9);
    }
    return free ? const Color(0xFFE0E0E0) : null;
  }

  Widget _grid(List<AbsenceRow> rows, Map<int, String> free) {
    final n = daysInMonth(year, month);
    bool isFree(int day) => free.containsKey(day) || DateTime.utc(year, month, day).weekday >= 6;
    Widget cell(String text, {Color? color, String? tooltip, bool bold = false}) {
      final box = Container(
        width: 26,
        height: 24,
        alignment: Alignment.center,
        decoration: BoxDecoration(color: color, border: Border.all(color: const Color(0xFFBDBDBD), width: 0.5)),
        child: Text(text, style: TextStyle(fontSize: 11, fontWeight: bold ? FontWeight.bold : FontWeight.normal)),
      );
      return tooltip == null || tooltip.isEmpty ? box : Tooltip(message: tooltip, child: box);
    }

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [
            const SizedBox(width: 180),
            for (var d = 1; d <= n; d++)
              cell(_weekdayLetters[DateTime.utc(year, month, d).weekday - 1], color: isFree(d) ? const Color(0xFFE0E0E0) : null, tooltip: free[d]),
          ]),
          Row(children: [
            const SizedBox(width: 180),
            for (var d = 1; d <= n; d++) cell('$d', color: isFree(d) ? const Color(0xFFE0E0E0) : null, tooltip: free[d], bold: true),
          ]),
          for (final r in rows)
            Row(children: [
              SizedBox(width: 180, child: Text(r.name, overflow: TextOverflow.ellipsis)),
              for (var d = 1; d <= n; d++) cell(r.days[d] ?? '', color: _cellColor(r.days[d], isFree(d))),
            ]),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final rows = buildAbsenceRows(absences, year, month);
    final free = freeDaysOfMonth(calendar, year, month);
    final sorted = absences.cast<Map<String, dynamic>>().toList()
      ..sort((a, b) => (a['start_date']?.toString() ?? '').compareTo(b['start_date']?.toString() ?? ''));
    return Scaffold(
      appBar: AppBar(
        title: const Text('Abwesenheiten'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 8,
              runSpacing: 8,
              children: [
                IconButton(onPressed: () => _shiftMonth(-1), icon: const Icon(Icons.chevron_left), tooltip: 'Vormonat'),
                Text('${monthNames[month - 1]} $year', style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                IconButton(onPressed: () => _shiftMonth(1), icon: const Icon(Icons.chevron_right), tooltip: 'Folgemonat'),
                if (teams.isNotEmpty)
                  SizedBox(
                    width: 240,
                    child: DropdownButtonFormField<String>(
                      initialValue: teamId,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Team'),
                      items: [
                        const DropdownMenuItem<String>(value: null, child: Text('Alle')),
                        for (final t in teams.cast<Map<String, dynamic>>())
                          DropdownMenuItem(value: t['id'].toString(), child: Text(t['name']?.toString() ?? '')),
                      ],
                      onChanged: (v) {
                        setState(() => teamId = v);
                        _load();
                      },
                    ),
                  ),
              ],
            ),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (error == null) ...[
              const Text('U = Urlaub · K = krank · B = unbezahlt · S = Sonstiges · grau = Wochenende oder freier Tag'),
              if ((calendar?['bundesland']?.toString() ?? '').isEmpty)
                const Text('Hinweis: Kein Bundesland eingestellt - es sind nur die bundesweiten Feiertage hinterlegt.',
                    style: TextStyle(color: Color(0xFFB26A00))),
              const SizedBox(height: 8),
              if (!loading && rows.isEmpty) const Text('In diesem Monat ist niemand abwesend.'),
              if (rows.isNotEmpty) _grid(rows, free),
              const SizedBox(height: 16),
              for (final a in sorted) Text(absenceLine(a)),
              if (free.isNotEmpty) ...[
                const SizedBox(height: 16),
                const Text('Freie Tage', style: TextStyle(fontWeight: FontWeight.bold)),
                for (final e in (free.entries.toList()..sort((a, b) => a.key.compareTo(b.key))))
                  Text('${_two(e.key)}.${_two(month)}.$year ${e.value}'),
              ],
            ],
          ],
        ),
      ),
    );
  }
}
