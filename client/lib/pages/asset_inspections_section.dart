import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Pruefungen eines Betriebsmittels (Backlog F.13.4.2), Routen aus F.3.6.3:
// Pruefstand, Pruefungen erfassen und stornieren. Lesen mit assets.read,
// schreiben mit assets.write. Pruefungen werden nie geaendert oder
// geloescht (F.3.5.1). "Maengel" und "nicht bestanden" sperren das Stueck
// fuer die Ausgabe (Entscheidungen des Nutzers vom 2026-10-15/19).

const inspectionErgebnisse = {
  'bestanden': 'bestanden',
  'maengel': 'Mängel',
  'nicht_bestanden': 'nicht bestanden',
};

String _two(int v) => v.toString().padLeft(2, '0');

// pruefstandLabel beschreibt den Pruefstand in Worten.
String pruefstandLabel(Map<String, dynamic> st) {
  final next = isoToGerman(st['naechste_pruefung_am']?.toString());
  final tage = st['tage_bis_faellig'];
  switch (st['status']) {
    case 'gesperrt':
      return st['maengel'] == true ? 'gesperrt – letzte Prüfung mit Mängeln' : 'gesperrt – letzte Prüfung nicht bestanden';
    case 'ueberfaellig':
      return 'Prüfung überfällig seit $next';
    case 'faellig_bald':
      return tage == 0 ? 'Prüfung heute fällig' : 'Prüfung fällig am $next (in $tage Tagen)';
    case 'nie_geprueft':
      return 'noch nie geprüft';
    case 'nicht_pruefpflichtig':
      return 'nicht prüfpflichtig';
    case 'geprueft':
      return next.isEmpty ? 'geprüft' : 'geprüft, nächste Prüfung am $next';
    default:
      return st['status']?.toString() ?? '';
  }
}

bool pruefstandIsBad(String? status) => status == 'gesperrt' || status == 'ueberfaellig' || status == 'nie_geprueft';

// inspectionLine: "15.03.2026 · bestanden · Elektro Meier · nächste 15.09.2026".
String inspectionLine(Map<String, dynamic> i) {
  final parts = <String>[
    isoToGerman(i['geprueft_am']?.toString()),
    inspectionErgebnisse[i['ergebnis']] ?? (i['ergebnis']?.toString() ?? ''),
    i['pruefer_name']?.toString() ?? '',
  ];
  final next = isoToGerman(i['naechste_pruefung_am']?.toString());
  if (next.isNotEmpty) parts.add('nächste $next');
  final notiz = i['notiz']?.toString() ?? '';
  if (notiz.isNotEmpty) parts.add(notiz);
  return parts.join(' · ');
}

// buildInspectionBody: leeres naechste_pruefung_am wird weggelassen - dann
// rechnet der Server aus dem Pruefintervall (bei nicht_bestanden: keine).
Map<String, dynamic> buildInspectionBody({
  required String assetId,
  required String geprueftAm,
  required String ergebnis,
  String? naechstePruefungAm,
  required String prueferName,
  String notiz = '',
}) {
  final body = <String, dynamic>{
    'asset_id': assetId,
    'geprueft_am': geprueftAm,
    'ergebnis': ergebnis,
    'pruefer_name': prueferName.trim(),
    'notiz': notiz.trim(),
  };
  if (naechstePruefungAm != null && naechstePruefungAm.isNotEmpty) body['naechste_pruefung_am'] = naechstePruefungAm;
  return body;
}

class AssetInspectionsSection extends StatefulWidget {
  const AssetInspectionsSection({super.key, required this.api, required this.assetId, this.retired = false, this.now, this.onChanged});
  final ApiClient api;
  final String assetId;
  final bool retired;
  final DateTime Function()? now;
  // Nach Erfassen/Storno - die Seite kann z. B. die Ausgabe neu pruefen.
  final VoidCallback? onChanged;

  @override
  State<AssetInspectionsSection> createState() => _AssetInspectionsSectionState();
}

class _AssetInspectionsSectionState extends State<AssetInspectionsSection> {
  Map<String, dynamic>? standing;
  List<dynamic> inspections = [];
  bool loading = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('assets.write');

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
      final st = await widget.api.getAssetInspectionStanding(widget.assetId);
      final list = await widget.api.listAssetInspections(widget.assetId);
      if (!mounted) return;
      setState(() {
        standing = st;
        inspections = list;
      });
    } catch (e) {
      if (mounted) setState(() => error = 'Prüfungen konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _recordDialog() async {
    final t = (widget.now ?? DateTime.now)();
    final datumCtrl = TextEditingController(text: '${_two(t.day)}.${_two(t.month)}.${t.year}');
    final nextCtrl = TextEditingController();
    final prueferCtrl = TextEditingController();
    final notizCtrl = TextEditingController();
    String ergebnis = 'bestanden';
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Prüfung erfassen'),
          content: SizedBox(
            width: 440,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(controller: datumCtrl, decoration: const InputDecoration(labelText: 'Geprüft am (TT.MM.JJJJ)')),
                  DropdownButtonFormField<String>(
                    initialValue: ergebnis,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Ergebnis'),
                    items: [for (final e in inspectionErgebnisse.entries) DropdownMenuItem(value: e.key, child: Text(e.value))],
                    onChanged: (v) => setStateDialog(() => ergebnis = v ?? ergebnis),
                  ),
                  if (ergebnis != 'bestanden')
                    const Padding(
                      padding: EdgeInsets.only(top: 4),
                      child: Text('Das Stück ist danach für die Ausgabe gesperrt, bis eine Prüfung bestanden ist.',
                          style: TextStyle(color: Color(0xFFB00020))),
                    ),
                  TextField(
                    controller: nextCtrl,
                    enabled: ergebnis != 'nicht_bestanden',
                    decoration: const InputDecoration(labelText: 'Nächste Prüfung (TT.MM.JJJJ, leer = aus Prüfintervall)'),
                  ),
                  TextField(controller: prueferCtrl, decoration: const InputDecoration(labelText: 'Prüfer')),
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
                final datum = parseDateInput(datumCtrl.text);
                final nextText = ergebnis == 'nicht_bestanden' ? '' : nextCtrl.text.trim();
                final next = nextText.isEmpty ? null : parseDateInput(nextText);
                String? problem;
                if (datum == null) {
                  problem = 'Bitte das Prüfdatum als TT.MM.JJJJ angeben.';
                } else if (nextText.isNotEmpty && next == null) {
                  problem = 'Bitte die nächste Prüfung als TT.MM.JJJJ angeben oder leer lassen.';
                } else if (prueferCtrl.text.trim().isEmpty) {
                  problem = 'Bitte den Prüfer angeben.';
                }
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(buildInspectionBody(
                  assetId: widget.assetId,
                  geprueftAm: datum!,
                  ergebnis: ergebnis,
                  naechstePruefungAm: next,
                  prueferName: prueferCtrl.text,
                  notiz: notizCtrl.text,
                ));
              },
              child: const Text('Erfassen'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    try {
      await widget.api.recordAssetInspection(body);
      _snack('Prüfung erfasst');
      await _load();
      widget.onChanged?.call();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _stornoDialog(Map<String, dynamic> i) async {
    final reasonCtrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Prüfung stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(inspectionLine(i)),
                const SizedBox(height: 8),
                const Text('Eine Prüfung wird nicht gelöscht, sondern mit Begründung storniert.'),
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
      await widget.api.stornoAssetInspection(i['id'].toString(), reason);
      _snack('Prüfung storniert');
      await _load();
      widget.onChanged?.call();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final st = standing;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Expanded(child: Text('Prüfungen', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold))),
            if (_canWrite && !widget.retired)
              FilledButton.tonal(onPressed: _recordDialog, child: const Text('Prüfung erfassen')),
          ],
        ),
        if (loading) const LinearProgressIndicator(minHeight: 2),
        if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (st != null && error == null)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 8),
            child: Text(
              'Prüfstand: ${pruefstandLabel(st)}',
              style: TextStyle(
                fontWeight: FontWeight.bold,
                color: pruefstandIsBad(st['status']?.toString()) ? const Color(0xFFB00020) : null,
              ),
            ),
          ),
        if (!loading && error == null && inspections.isEmpty) const Text('Keine Prüfungen erfasst.'),
        for (final i in inspections)
          Row(
            children: [
              Expanded(child: Text(inspectionLine(i as Map<String, dynamic>))),
              if (_canWrite)
                IconButton(tooltip: 'Prüfung stornieren', icon: const Icon(Icons.block, size: 18), onPressed: () => _stornoDialog(i)),
            ],
          ),
      ],
    );
  }
}
