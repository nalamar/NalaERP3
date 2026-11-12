import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'production_assembly_section.dart' show assemblyHints, assemblyRange;
import 'production_capacity_page.dart' show addDays, capacityQuery, capacityWeekHeader, capacityWeeks, isoDay, mondayOf;
import 'production_orders_page.dart' show ProductionOrderDetailPage;

// Wochenübersicht der Montagetermine je Team (Backlog H.3.6f, ADR 0038
// Frage 6 (a)) auf GET /production/montage: Tabelle Team x Woche, alle
// Teams nach Name, "ohne Team" zuletzt; nur nicht stornierte Termine. Ein
// Termin über mehrere Wochen steht in jeder berührten Woche. Hinweise sind
// keine Sperren; ein Tipp öffnet den Fertigungsauftrag. Acht Wochen,
// vor- und zurückblättern wie die Auslastung.

DateTime? _day(Object? iso) {
  final d = DateTime.tryParse(iso?.toString() ?? '');
  return d == null ? null : DateTime(d.year, d.month, d.day);
}

// assemblyWeekStarts: Montage der Wochen von bis bis einschließlich.
List<DateTime> assemblyWeekStarts(Object? von, Object? bis) {
  final v = _day(von), b = _day(bis);
  if (v == null || b == null) return const [];
  return [for (var w = mondayOf(v); !w.isAfter(b); w = addDays(w, 7)) w];
}

// assemblyInWeek: der Termin berührt die Woche ab monday (bis Sonntag).
bool assemblyInWeek(Map<String, dynamic> t, DateTime monday) {
  final v = _day(t['von']), b = _day(t['bis']);
  if (v == null || b == null) return false;
  return !v.isAfter(addDays(monday, 6)) && !b.isBefore(monday);
}

// assemblyWeekTitle: "FA-2026-0001 · P-7 Hafen"; fehlende Teile entfallen.
String assemblyWeekTitle(Map<String, dynamic> t) {
  final project = [t['project_nummer'], t['project_name']].map((x) => x?.toString() ?? '').where((x) => x.isNotEmpty).join(' ');
  return [t['order_nummer']?.toString() ?? '', project].where((x) => x.isNotEmpty).join(' · ');
}

// assemblyWeekTooltip: Zeitraum, Fahrzeug, Baustelle und Hinweise.
String assemblyWeekTooltip(Map<String, dynamic> t) {
  final kennzeichen = t['kennzeichen']?.toString() ?? '';
  final baustelle = t['baustelle']?.toString() ?? '';
  return [
    assemblyRange(t),
    if (kennzeichen.isNotEmpty) 'Fahrzeug: $kennzeichen',
    if (baustelle.isNotEmpty) 'Baustelle: $baustelle',
    for (final h in assemblyHints(t)) 'Hinweis: $h',
  ].join('\n');
}

// assemblyTeamLine: "2 Termine", "1 Termin" oder "keine Termine".
String assemblyTeamLine(Map<String, dynamic> row) {
  final n = ((row['termine'] as List?) ?? const []).length;
  return n == 0 ? 'keine Termine' : '$n ${n == 1 ? 'Termin' : 'Termine'}';
}

class ProductionAssemblyWeeksPage extends StatefulWidget {
  const ProductionAssemblyWeeksPage({super.key, required this.api, this.today});
  final ApiClient api;
  // fuer Tests; sonst heute
  final DateTime? today;

  @override
  State<ProductionAssemblyWeeksPage> createState() => _ProductionAssemblyWeeksPageState();
}

class _ProductionAssemblyWeeksPageState extends State<ProductionAssemblyWeeksPage> {
  late DateTime start;
  Map<String, dynamic>? data;
  bool loading = false;
  String? error;

  DateTime get _today => widget.today ?? DateTime.now();

  @override
  void initState() {
    super.initState();
    start = mondayOf(_today);
    _load();
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final q = capacityQuery(start);
      final d = await widget.api.getProductionAssemblyWeeks(von: q['von'], bis: q['bis']);
      if (mounted) setState(() => data = d);
    } catch (e) {
      if (mounted) setState(() => error = 'Montagetermine konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _shift(int weeks) {
    start = addDays(start, 7 * weeks);
    _load();
  }

  Future<void> _open(Map<String, dynamic> t) async {
    final id = t['order_id']?.toString() ?? '';
    if (id.isEmpty) return;
    await Navigator.of(context).push(MaterialPageRoute(builder: (_) => ProductionOrderDetailPage(api: widget.api, orderId: id)));
    if (mounted) await _load();
  }

  Widget _entry(Map<String, dynamic> t) {
    final warn = assemblyHints(t).isNotEmpty;
    return Tooltip(
      message: assemblyWeekTooltip(t),
      child: InkWell(
        onTap: () => _open(t),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 4),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            if (warn) Icon(Icons.warning_amber_rounded, size: 16, color: Theme.of(context).colorScheme.error),
            if (warn) const SizedBox(width: 4),
            Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
              Text(assemblyWeekTitle(t)),
              Text(assemblyRange(t), style: Theme.of(context).textTheme.bodySmall),
            ]),
          ]),
        ),
      ),
    );
  }

  Widget _cell(List<Map<String, dynamic>> termine, DateTime monday) {
    final inWeek = [
      for (final t in termine)
        if (assemblyInWeek(t, monday)) t
    ];
    if (inWeek.isEmpty) return const Text('–');
    return Column(
        crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [for (final t in inWeek) _entry(t)]);
  }

  Widget _table(List<dynamic> rows, List<DateTime> weeks) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: DataTable(
        dataRowMinHeight: 56,
        dataRowMaxHeight: double.infinity,
        columns: [
          const DataColumn(label: Text('Team')),
          for (final w in weeks) DataColumn(label: Text(capacityWeekHeader(isoDay(w)))),
        ],
        rows: [
          for (final r in rows.cast<Map<String, dynamic>>())
            DataRow(cells: [
              DataCell(Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(r['team_id'] == null ? 'ohne Team' : r['team_name']?.toString() ?? ''),
                Text(assemblyTeamLine(r), style: Theme.of(context).textTheme.bodySmall),
              ])),
              for (final w in weeks) DataCell(_cell(((r['termine'] as List?) ?? const []).cast<Map<String, dynamic>>(), w)),
            ]),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final q = capacityQuery(start);
    final rows = (data?['teams'] as List?) ?? const [];
    return Scaffold(
      appBar: AppBar(
        title: const Text('Montageplanung'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Wrap(
              spacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                IconButton(
                    onPressed: loading ? null : () => _shift(-capacityWeeks),
                    icon: const Icon(Icons.chevron_left),
                    tooltip: 'Frühere Wochen'),
                Text('${isoToGerman(q['von'])} bis ${isoToGerman(q['bis'])} (Wochen ab Montag)'),
                IconButton(
                    onPressed: loading ? null : () => _shift(capacityWeeks),
                    icon: const Icon(Icons.chevron_right),
                    tooltip: 'Spätere Wochen'),
                TextButton(
                    onPressed: loading
                        ? null
                        : () {
                            start = mondayOf(_today);
                            _load();
                          },
                    child: const Text('Aktuelle Woche')),
              ],
            ),
            const SizedBox(height: 8),
            Text('Nicht stornierte Montagetermine je Team. Hinweise wie doppelt eingeplantes Team oder Abwesenheit sind keine Sperren.',
                style: Theme.of(context).textTheme.bodySmall),
            const SizedBox(height: 12),
            if (loading) const LinearProgressIndicator(),
            if (error != null)
              Text(error!, style: TextStyle(color: Theme.of(context).colorScheme.error))
            else if (!loading && data != null && rows.isEmpty)
              const Text('Noch kein Team und kein Montagetermin – Teams werden unter „Personal“ angelegt.')
            else if (rows.isNotEmpty)
              _table(rows, assemblyWeekStarts(data?['von'], data?['bis'])),
          ],
        ),
      ),
    );
  }
}
