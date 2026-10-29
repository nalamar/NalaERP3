import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show formatPrice, isoToGerman, parsePrice;
import 'employee_qualifications_section.dart' show parseDateInput;

// Kosten auf der Detailseite eines Fahrzeugs (Backlog G.3d.3, Routen
// G.3d.2): Summen je Kostenart, Belege; erfassen und stornieren mit
// fleet.write. Geschlossene Form - ein Beleg wird nie geändert, nur mit
// Begründung storniert. Beträge netto.

const costKostenarten = {
  'kraftstoff': 'Kraftstoff',
  'wartung': 'Wartung',
  'reparatur': 'Reparatur',
  'versicherung': 'Versicherung',
  'steuer': 'Kfz-Steuer',
  'leasing': 'Leasing',
  'sonstiges': 'Sonstiges',
};

String _two(int v) => v.toString().padLeft(2, '0');

// Baut den Body für POST /fleet/costs oder liefert eine Meldung.
// kostenstelleId null = die des Fahrzeugs.
({Map<String, dynamic>? body, String? error}) buildCostBody({
  required String vehicleId,
  required String datum,
  required String kostenart,
  required String betrag,
  String bezeichnung = '',
  String lieferant = '',
  String belegnummer = '',
  String kmStand = '',
  String liter = '',
  String? kostenstelleId,
  String notiz = '',
}) {
  final d = parseDateInput(datum);
  if (d == null)
    return (body: null, error: 'Bitte das Datum als TT.MM.JJJJ angeben.');
  if (!costKostenarten.containsKey(kostenart))
    return (body: null, error: 'Bitte eine Kostenart wählen.');
  if (kostenart == 'sonstiges' && bezeichnung.trim().isEmpty) {
    return (
      body: null,
      error: 'Bezeichnung ist bei sonstigen Kosten erforderlich.'
    );
  }
  final b = parsePrice(betrag);
  if (!b.ok || b.value == null || b.value! <= 0) {
    return (
      body: null,
      error:
          'Betrag als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'
    );
  }
  int? km;
  if (kmStand.trim().isNotEmpty) {
    km = int.tryParse(kmStand.trim());
    if (km == null || km < 0)
      return (body: null, error: 'km-Stand muss eine ganze Zahl ab 0 sein.');
  }
  double? l;
  if (liter.trim().isNotEmpty) {
    if (kostenart != 'kraftstoff')
      return (body: null, error: 'Liter nur bei Kraftstoff.');
    final p = parsePrice(liter);
    if (!p.ok || p.value == null || p.value! <= 0) {
      return (
        body: null,
        error:
            'Liter als Zahl über 0 mit höchstens zwei Nachkommastellen angeben.'
      );
    }
    l = p.value;
  }
  return (
    body: {
      'vehicle_id': vehicleId,
      'datum': d,
      'kostenart': kostenart,
      'bezeichnung': bezeichnung.trim(),
      'betrag_netto': b.value,
      'lieferant': lieferant.trim(),
      'belegnummer': belegnummer.trim(),
      if (km != null) 'km_stand': km,
      if (l != null) 'liter': l,
      if (kostenstelleId != null) 'kostenstelle_id': kostenstelleId,
      'notiz': notiz.trim(),
    },
    error: null,
  );
}

// costLine beschreibt einen Beleg: "02.03.2026 · Kraftstoff · 80,12 € ·
// 45,50 l · Tanke · Beleg B-1".
String costLine(Map<String, dynamic> c) {
  final art =
      costKostenarten[c['kostenart']] ?? c['kostenart']?.toString() ?? '';
  final bez = c['bezeichnung']?.toString() ?? '';
  final parts = [
    isoToGerman(c['datum']?.toString()),
    bez.isEmpty ? art : '$art: $bez',
    '${formatPrice(c['betrag_netto'])} €',
    if (c['liter'] is num) '${formatPrice(c['liter'])} l',
    if (c['km_stand'] is num) 'km ${c['km_stand']}',
    c['lieferant']?.toString() ?? '',
    if ((c['belegnummer']?.toString() ?? '').isNotEmpty)
      'Beleg ${c['belegnummer']}',
  ];
  var line = parts.where((p) => p.isNotEmpty).join(' · ');
  if (c['status'] == 'storniert')
    line = 'STORNIERT: $line (${c['storno_reason'] ?? ''})';
  return line;
}

class VehicleCostsSection extends StatefulWidget {
  const VehicleCostsSection(
      {super.key, required this.api, required this.vehicleId, this.now});
  final ApiClient api;
  final String vehicleId;
  final DateTime Function()? now;

  @override
  State<VehicleCostsSection> createState() => _VehicleCostsSectionState();
}

class _VehicleCostsSectionState extends State<VehicleCostsSection> {
  List<dynamic> costs = [];
  Map<String, dynamic>? summary;
  bool loading = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('fleet.write');

  String _today() {
    final t = (widget.now ?? DateTime.now)();
    return '${_two(t.day)}.${_two(t.month)}.${t.year}';
  }

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
      final results = await Future.wait<Object>([
        widget.api.listFleetCosts(
            vehicleId: widget.vehicleId, includeStorniert: true),
        widget.api.getFleetCostSummary(vehicleId: widget.vehicleId),
      ]);
      if (!mounted) return;
      setState(() {
        costs = results[0] as List<dynamic>;
        summary = results[1] as Map<String, dynamic>;
      });
    } catch (e) {
      if (mounted)
        setState(() => error = 'Kosten konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted)
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _recordDialog() async {
    // Kostenstellen sind optional; ohne Liste bleibt "wie Fahrzeug".
    List<dynamic> costCenters = const [];
    try {
      costCenters = await widget.api.listCostCenters();
    } catch (_) {}
    if (!mounted) return;
    String kostenart = 'kraftstoff';
    String? kostenstelleId;
    final datumCtrl = TextEditingController(text: _today());
    final betragCtrl = TextEditingController();
    final bezCtrl = TextEditingController();
    final lieferantCtrl = TextEditingController();
    final belegCtrl = TextEditingController();
    final kmCtrl = TextEditingController();
    final literCtrl = TextEditingController();
    final notizCtrl = TextEditingController();
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Kosten erfassen'),
          content: SizedBox(
            width: 460,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  TextField(
                      controller: datumCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Belegdatum (TT.MM.JJJJ)')),
                  DropdownButtonFormField<String>(
                    key: const ValueKey('costKostenart'),
                    initialValue: kostenart,
                    decoration: const InputDecoration(labelText: 'Kostenart'),
                    items: [
                      for (final e in costKostenarten.entries)
                        DropdownMenuItem(value: e.key, child: Text(e.value))
                    ],
                    onChanged: (v) =>
                        setStateDialog(() => kostenart = v ?? kostenart),
                  ),
                  TextField(
                      controller: bezCtrl,
                      decoration: InputDecoration(
                          labelText: kostenart == 'sonstiges'
                              ? 'Bezeichnung'
                              : 'Bezeichnung (optional)')),
                  TextField(
                      controller: betragCtrl,
                      decoration:
                          const InputDecoration(labelText: 'Betrag netto (€)')),
                  if (kostenart == 'kraftstoff')
                    TextField(
                        controller: literCtrl,
                        decoration: const InputDecoration(
                            labelText: 'Liter (optional)')),
                  TextField(
                      controller: kmCtrl,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                          labelText: 'km-Stand (optional)')),
                  TextField(
                      controller: lieferantCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Lieferant / Werkstatt (optional)')),
                  TextField(
                      controller: belegCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Belegnummer (optional)')),
                  DropdownButtonFormField<String?>(
                    key: const ValueKey('costKostenstelle'),
                    initialValue: kostenstelleId,
                    isExpanded: true,
                    decoration:
                        const InputDecoration(labelText: 'Kostenstelle'),
                    items: [
                      const DropdownMenuItem<String?>(
                          value: null, child: Text('wie Fahrzeug')),
                      for (final x in costCenters)
                        DropdownMenuItem<String?>(
                            value: x['id'].toString(),
                            child: Text(
                                '${x['code'] ?? ''} – ${x['name'] ?? ''}')),
                    ],
                    onChanged: (v) => setStateDialog(() => kostenstelleId = v),
                  ),
                  TextField(
                      controller: notizCtrl,
                      decoration: const InputDecoration(labelText: 'Notiz')),
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
                final r = buildCostBody(
                  vehicleId: widget.vehicleId,
                  datum: datumCtrl.text,
                  kostenart: kostenart,
                  betrag: betragCtrl.text,
                  bezeichnung: bezCtrl.text,
                  lieferant: lieferantCtrl.text,
                  belegnummer: belegCtrl.text,
                  kmStand: kmCtrl.text,
                  // Liter nur, solange das Feld sichtbar ist.
                  liter: kostenart == 'kraftstoff' ? literCtrl.text : '',
                  kostenstelleId: kostenstelleId,
                  notiz: notizCtrl.text,
                );
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop(r.body);
              },
              child: const Text('Erfassen'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    try {
      await widget.api.recordFleetCost(body);
      _snack('Kosten erfasst');
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _stornoDialog(Map<String, dynamic> c) async {
    final reasonCtrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Kosten stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(costLine(c)),
                const SizedBox(height: 8),
                const Text(
                    'Nur für falsch erfasste Belege. Er wird nicht gelöscht, sondern mit Begründung storniert.'),
                TextField(
                  controller: reasonCtrl,
                  decoration: const InputDecoration(labelText: 'Begründung'),
                  onChanged: (_) => setStateDialog(() {}),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.of(ctx).pop(),
                child: const Text('Abbrechen')),
            FilledButton(
              onPressed: reasonCtrl.text.trim().isEmpty
                  ? null
                  : () => Navigator.of(ctx).pop(reasonCtrl.text.trim()),
              child: const Text('Stornieren'),
            ),
          ],
        ),
      ),
    );
    if (reason == null) return;
    try {
      await widget.api.stornoFleetCost(c['id'].toString(), reason);
      _snack('Kosten storniert');
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final summen = (summary?['summen'] as List?) ?? const [];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Kosten',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
        if (loading) const LinearProgressIndicator(minHeight: 2),
        if (error != null)
          Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (!loading && error == null) ...[
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 8),
            child: Row(
              children: [
                Expanded(
                  child: Text(summen.isEmpty
                      ? 'Noch keine Kosten erfasst'
                      : 'Gesamt netto: ${formatPrice(summary?['gesamt'])} €'),
                ),
                if (_canWrite)
                  FilledButton(
                      onPressed: _recordDialog,
                      child: const Text('Kosten erfassen')),
              ],
            ),
          ),
          for (final s in summen)
            Text(
                '${costKostenarten[s['kostenart']] ?? s['kostenart']}: ${formatPrice(s['betrag_netto'])} € (${s['anzahl']} Belege)'),
          if (summen.isNotEmpty) const SizedBox(height: 8),
          for (final x in costs)
            Row(
              children: [
                Expanded(child: Text(costLine(x as Map<String, dynamic>))),
                if (_canWrite && x['status'] == 'active')
                  IconButton(
                      tooltip: 'Kosten stornieren',
                      icon: const Icon(Icons.block, size: 18),
                      onPressed: () => _stornoDialog(x)),
              ],
            ),
        ],
      ],
    );
  }
}
