import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'production_operations_section.dart' show isoWeekNumber;
import 'production_orders_page.dart' show formatQty;
import 'production_planning_page.dart' show formatMinutes;

// Auslastung je Arbeitsplatz und Woche (Backlog H.2.5c, ADR 0037) auf der
// Route aus H.2.4: Tabelle Arbeitsplatz x Woche mit geplanter und
// verfuegbarer Zeit, Ueberlast hervorgehoben (Hinweis, keine Sperre),
// ungeplante Zeit je Arbeitsplatz; acht Wochen, vor- und zurueckblaettern.

const capacityWeeks = 8;

// addDays rechnet in Kalendertagen; Duration verschiebt ueber die
// Zeitumstellung um eine Stunde und damit ggf. um einen Tag.
DateTime addDays(DateTime d, int days) =>
    DateTime(d.year, d.month, d.day + days);

// mondayOf: Montag der Woche, ohne Uhrzeit.
DateTime mondayOf(DateTime d) => addDays(d, 1 - d.weekday);

String isoDay(DateTime d) =>
    '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

// capacityQuery: von/bis fuer acht Wochen ab dem Montag von start.
Map<String, String> capacityQuery(DateTime start) {
  final von = mondayOf(start);
  return {
    'von': isoDay(von),
    'bis': isoDay(addDays(von, 7 * (capacityWeeks - 1))),
  };
}

// capacityWeekHeader: "KW 46" und darunter der Montag.
String capacityWeekHeader(String iso) {
  final d = DateTime.tryParse(iso.length >= 10 ? iso.substring(0, 10) : iso);
  if (d == null) return iso;
  return 'KW ${isoWeekNumber(d)}\n${isoToGerman(iso)}';
}

bool capacityOverloaded(Map<String, dynamic> week) =>
    (week['hinweise'] as List?)?.contains('ueberlast') ?? false;

// capacityCell: "12 h von 40 h" oder "frei" ohne Arbeitstage.
String capacityCell(Map<String, dynamic> week) {
  final geplant = (week['geplant_min'] as num?) ?? 0;
  final verfuegbar = (week['verfuegbar_min'] as num?) ?? 0;
  if (verfuegbar == 0 && geplant == 0) return 'frei';
  return '${formatMinutes(geplant)} von ${formatMinutes(verfuegbar)}';
}

// capacityRowLine: "8 h je Arbeitstag · ungeplant 2 h · deaktiviert".
String capacityRowLine(Map<String, dynamic> row) {
  final parts = <String>[];
  final h = row['stunden_je_tag'];
  if (h is num) parts.add('${formatQty(h)} h je Arbeitstag');
  final ungeplant = row['ungeplant_min'];
  if (ungeplant is num && ungeplant > 0)
    parts.add('ungeplant ${formatMinutes(ungeplant)}');
  if (row['aktiv'] == false) parts.add('deaktiviert');
  return parts.join(' · ');
}

class ProductionCapacityPage extends StatefulWidget {
  const ProductionCapacityPage({super.key, required this.api, this.today});
  final ApiClient api;
  // fuer Tests; sonst heute
  final DateTime? today;

  @override
  State<ProductionCapacityPage> createState() => _ProductionCapacityPageState();
}

class _ProductionCapacityPageState extends State<ProductionCapacityPage> {
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
      final d =
          await widget.api.getProductionCapacity(von: q['von'], bis: q['bis']);
      if (mounted) setState(() => data = d);
    } catch (e) {
      if (mounted)
        setState(() => error = 'Auslastung konnte nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _shift(int weeks) {
    start = addDays(start, 7 * weeks);
    _load();
  }

  Widget _cell(Map<String, dynamic> week) {
    final over = capacityOverloaded(week);
    final text = Text(capacityCell(week),
        style: over
            ? TextStyle(
                color: Theme.of(context).colorScheme.error,
                fontWeight: FontWeight.bold)
            : null);
    if (!over) return text;
    return Tooltip(
      message: 'Überlast: mehr geplant als verfügbar',
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        Icon(Icons.warning_amber_rounded,
            size: 16, color: Theme.of(context).colorScheme.error),
        const SizedBox(width: 4),
        text,
      ]),
    );
  }

  Widget _table(List<dynamic> rows) {
    final weeks = rows.isEmpty
        ? const <dynamic>[]
        : ((rows.first as Map<String, dynamic>)['wochen'] as List? ?? const []);
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: DataTable(
        dataRowMinHeight: 56,
        dataRowMaxHeight: 72,
        columns: [
          const DataColumn(label: Text('Arbeitsplatz')),
          for (final w in weeks)
            DataColumn(
                label: Text(capacityWeekHeader(
                    (w as Map<String, dynamic>)['woche']?.toString() ?? ''))),
        ],
        rows: [
          for (final r in rows)
            DataRow(cells: [
              DataCell(Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text((r as Map<String, dynamic>)['name']?.toString() ?? ''),
                  Text(capacityRowLine(r),
                      style: Theme.of(context).textTheme.bodySmall),
                ],
              )),
              for (final w in (r['wochen'] as List? ?? const []))
                DataCell(_cell(w as Map<String, dynamic>)),
            ]),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final q = capacityQuery(start);
    final rows = (data?['arbeitsplaetze'] as List?) ?? const [];
    return Scaffold(
      appBar: AppBar(
        title: const Text('Auslastung'),
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
            Wrap(
              spacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                IconButton(
                    onPressed: loading ? null : () => _shift(-capacityWeeks),
                    icon: const Icon(Icons.chevron_left),
                    tooltip: 'Frühere Wochen'),
                Text(
                    '${isoToGerman(q['von'])} bis ${isoToGerman(q['bis'])} (Wochen ab Montag)'),
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
            Text(
                'Geplant: Arbeitsgänge offener Aufträge. Verfügbar: Stunden je Arbeitstag ohne Feiertage und Betriebsruhe. Überlast ist ein Hinweis, keine Sperre.',
                style: Theme.of(context).textTheme.bodySmall),
            const SizedBox(height: 12),
            if (loading) const LinearProgressIndicator(),
            if (error != null)
              Text(error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error))
            else if (!loading && data != null && rows.isEmpty)
              const Text(
                  'Noch kein aktiver Arbeitsplatz – bitte unter „Arbeitsplätze und Vorlagen“ anlegen.')
            else if (rows.isNotEmpty)
              _table(rows),
          ],
        ),
      ),
    );
  }
}
