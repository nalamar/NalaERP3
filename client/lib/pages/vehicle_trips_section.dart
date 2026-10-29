import 'package:flutter/material.dart';
import '../api.dart';
import 'asset_detail_page.dart' show activeEmployees;
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Fahrtenbuch auf der Detailseite eines Fahrzeugs (Backlog G.3c.4a,
// Routen G.3c.3): km-Stand, Lücken, Fahrten; erfassen und stornieren.
// Geschlossene Form - eine Fahrt wird nie geändert, nur mit Begründung
// storniert. Erfassen mit fleet.write für einen gewählten Mitarbeiter,
// mit nur fleet.self für sich selbst; stornieren nur mit fleet.write.

const tripFahrtarten = {
  'dienstlich': 'Dienstfahrt',
  'privat': 'Privatfahrt',
  'arbeitsweg': 'Arbeitsweg',
};

String _two(int v) => v.toString().padLeft(2, '0');

// formatKm: 12345 -> "12.345".
String formatKm(int km) {
  final s = km.toString();
  final b = StringBuffer();
  for (var i = 0; i < s.length; i++) {
    if (i > 0 && (s.length - i) % 3 == 0) b.write('.');
    b.write(s[i]);
  }
  return b.toString();
}

// Baut den Body für POST /fleet/trips oder liefert eine Meldung.
// employeeId null = die Fahrt gilt dem eigenen Mitarbeiter.
({Map<String, dynamic>? body, String? error}) buildTripBody({
  required String vehicleId,
  String? employeeId,
  required String datum,
  required String kmBeginn,
  required String kmEnde,
  required String fahrtart,
  String ziel = '',
  String zweck = '',
  String geschaeftspartner = '',
  String notiz = '',
}) {
  final d = parseDateInput(datum);
  if (d == null)
    return (body: null, error: 'Bitte das Datum als TT.MM.JJJJ angeben.');
  final b = int.tryParse(kmBeginn.trim());
  if (b == null || b < 0)
    return (
      body: null,
      error: 'Kilometerstand Beginn muss eine ganze Zahl ab 0 sein.'
    );
  final e = int.tryParse(kmEnde.trim());
  if (e == null || e < 0)
    return (
      body: null,
      error: 'Kilometerstand Ende muss eine ganze Zahl ab 0 sein.'
    );
  if (e <= b)
    return (
      body: null,
      error: 'Kilometerstand Ende muss über dem Beginn liegen.'
    );
  if (!tripFahrtarten.containsKey(fahrtart))
    return (body: null, error: 'Bitte eine Fahrtart wählen.');
  if (fahrtart == 'dienstlich' && ziel.trim().isEmpty)
    return (body: null, error: 'Reiseziel ist bei Dienstfahrten erforderlich.');
  if (fahrtart == 'dienstlich' && zweck.trim().isEmpty)
    return (
      body: null,
      error: 'Reisezweck ist bei Dienstfahrten erforderlich.'
    );
  return (
    body: {
      'vehicle_id': vehicleId,
      if (employeeId != null) 'employee_id': employeeId,
      'datum': d,
      'km_beginn': b,
      'km_ende': e,
      'fahrtart': fahrtart,
      'ziel': ziel.trim(),
      'zweck': zweck.trim(),
      'geschaeftspartner': geschaeftspartner.trim(),
      'notiz': notiz.trim(),
    },
    error: null,
  );
}

// tripLine beschreibt eine Fahrt: "02.03.2026 · 1.000–1.050 km (50 km) ·
// Dienstfahrt · Anna Berg · Augsburg, Aufmaß".
String tripLine(Map<String, dynamic> t) {
  final b = (t['km_beginn'] as num?)?.toInt() ?? 0;
  final e = (t['km_ende'] as num?)?.toInt() ?? 0;
  final art = tripFahrtarten[t['fahrtart']] ?? t['fahrtart']?.toString() ?? '';
  final parts = [
    isoToGerman(t['datum']?.toString()),
    '${formatKm(b)}–${formatKm(e)} km (${formatKm(e - b)} km)',
    art,
    t['mitarbeiter_name']?.toString() ?? '',
  ];
  final reise = [t['ziel'], t['zweck'], t['geschaeftspartner']]
      .map((v) => v?.toString() ?? '')
      .where((v) => v.isNotEmpty)
      .join(', ');
  if (reise.isNotEmpty) parts.add(reise);
  var line = parts.where((p) => p.isNotEmpty).join(' · ');
  if (t['status'] == 'storniert')
    line = 'STORNIERT: $line (${t['storno_reason'] ?? ''})';
  return line;
}

class VehicleTripsSection extends StatefulWidget {
  const VehicleTripsSection(
      {super.key, required this.api, required this.vehicleId, this.now});
  final ApiClient api;
  final String vehicleId;
  final DateTime Function()? now;

  @override
  State<VehicleTripsSection> createState() => _VehicleTripsSectionState();
}

class _VehicleTripsSectionState extends State<VehicleTripsSection> {
  List<dynamic> trips = [];
  Map<String, dynamic>? chain;
  bool loading = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('fleet.write');
  bool get _canRecord => _canWrite || widget.api.hasPermission('fleet.self');
  int? get _kmStand => (chain?['km_stand'] as num?)?.toInt();

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
        widget.api.listFleetTrips(
            vehicleId: widget.vehicleId, includeStorniert: true),
        widget.api.getFleetTripChain(widget.vehicleId),
      ]);
      if (!mounted) return;
      setState(() {
        trips = results[0] as List<dynamic>;
        chain = results[1] as Map<String, dynamic>;
      });
    } catch (e) {
      if (mounted)
        setState(() => error = 'Fahrtenbuch konnte nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted)
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _recordDialog() async {
    List<Map<String, dynamic>> employees = const [];
    if (_canWrite) {
      try {
        employees = activeEmployees(await widget.api.listEmployees(limit: 500));
      } catch (e) {
        _snack('Mitarbeiterliste konnte nicht geladen werden: $e');
        return;
      }
      if (!mounted) return;
    }
    String? employeeId;
    String fahrtart = 'dienstlich';
    final datumCtrl = TextEditingController(text: _today());
    final beginnCtrl = TextEditingController(text: _kmStand?.toString() ?? '');
    final endeCtrl = TextEditingController();
    final zielCtrl = TextEditingController();
    final zweckCtrl = TextEditingController();
    final partnerCtrl = TextEditingController();
    final notizCtrl = TextEditingController();
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Fahrt erfassen'),
          content: SizedBox(
            width: 460,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (_canWrite)
                    DropdownButtonFormField<String>(
                      key: const ValueKey('tripEmployee'),
                      initialValue: employeeId,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Fahrer'),
                      items: [
                        for (final e in employees)
                          DropdownMenuItem(
                              value: e['id'].toString(),
                              child: Text(
                                  '${e['last_name'] ?? ''}, ${e['first_name'] ?? ''}')),
                      ],
                      onChanged: (v) => setStateDialog(() => employeeId = v),
                    ),
                  TextField(
                      controller: datumCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Datum (TT.MM.JJJJ)')),
                  TextField(
                      controller: beginnCtrl,
                      keyboardType: TextInputType.number,
                      decoration:
                          const InputDecoration(labelText: 'km Beginn')),
                  TextField(
                      controller: endeCtrl,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(labelText: 'km Ende')),
                  DropdownButtonFormField<String>(
                    key: const ValueKey('tripFahrtart'),
                    initialValue: fahrtart,
                    decoration: const InputDecoration(labelText: 'Fahrtart'),
                    items: [
                      for (final e in tripFahrtarten.entries)
                        DropdownMenuItem(value: e.key, child: Text(e.value))
                    ],
                    onChanged: (v) =>
                        setStateDialog(() => fahrtart = v ?? fahrtart),
                  ),
                  TextField(
                      controller: zielCtrl,
                      decoration: InputDecoration(
                          labelText: fahrtart == 'dienstlich'
                              ? 'Reiseziel'
                              : 'Reiseziel (optional)')),
                  TextField(
                      controller: zweckCtrl,
                      decoration: InputDecoration(
                          labelText: fahrtart == 'dienstlich'
                              ? 'Reisezweck'
                              : 'Reisezweck (optional)')),
                  TextField(
                      controller: partnerCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Geschäftspartner (optional)')),
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
                if (_canWrite && employeeId == null) {
                  setStateDialog(
                      () => dialogError = 'Bitte einen Fahrer auswählen.');
                  return;
                }
                final r = buildTripBody(
                  vehicleId: widget.vehicleId,
                  employeeId: employeeId,
                  datum: datumCtrl.text,
                  kmBeginn: beginnCtrl.text,
                  kmEnde: endeCtrl.text,
                  fahrtart: fahrtart,
                  ziel: zielCtrl.text,
                  zweck: zweckCtrl.text,
                  geschaeftspartner: partnerCtrl.text,
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
      await widget.api.recordFleetTrip(body);
      _snack('Fahrt erfasst');
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _stornoDialog(Map<String, dynamic> t) async {
    final reasonCtrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Fahrt stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(tripLine(t)),
                const SizedBox(height: 8),
                const Text(
                    'Nur für falsch erfasste Fahrten. Sie wird nicht gelöscht, sondern mit Begründung storniert; '
                    'ihr km-Bereich wird zur Lücke, die eine neue Fahrt füllen kann.'),
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
      await widget.api.stornoFleetTrip(t['id'].toString(), reason);
      _snack('Fahrt storniert');
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final km = _kmStand;
    final luecken = (chain?['luecken'] as List?) ?? const [];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Fahrtenbuch',
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
                    child: Text(km == null
                        ? 'Noch keine Fahrt erfasst'
                        : 'km-Stand: ${formatKm(km)} km')),
                if (_canRecord)
                  FilledButton(
                      onPressed: _recordDialog,
                      child: const Text('Fahrt erfassen')),
              ],
            ),
          ),
          for (final g in luecken)
            Text(
              '⚠ Lücke im km-Stand: ${formatKm((g['von_km'] as num).toInt())}–${formatKm((g['bis_km'] as num).toInt())} km',
              style: const TextStyle(color: Color(0xFFB00020)),
            ),
          for (final x in trips)
            Row(
              children: [
                Expanded(child: Text(tripLine(x as Map<String, dynamic>))),
                if (_canWrite && x['status'] == 'active')
                  IconButton(
                      tooltip: 'Fahrt stornieren',
                      icon: const Icon(Icons.block, size: 18),
                      onPressed: () => _stornoDialog(x)),
              ],
            ),
        ],
      ],
    );
  }
}
