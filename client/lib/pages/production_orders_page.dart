import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Fertigungsaufträge im Client (ADR 0034, Backlog H.1.4a, Routen H.1.3):
// Liste nach Liefertermin und Detail mit Hinweisen, Fertigungsstückliste
// und Statusschritten. Lesen mit production.read; anlegen, ändern,
// freigeben, Status und Storno mit production.write. Angelegt wird an der
// Variante im Projekt (H.1.4b).

const productionStatusLabels = {
  'entwurf': 'Entwurf',
  'freigegeben': 'Freigegeben',
  'in_arbeit': 'In Arbeit',
  'fertig': 'Fertig',
  'storniert': 'Storniert',
};

const productionHintLabels = {
  'kein_kundenauftrag': 'Kein Kundenauftrag zum Projekt',
  'menge_ueberschritten': 'Zusammen mehr Stück als die Positionsmenge',
  'position_entfernt': 'Position oder Variante nicht mehr im Projekt',
  'position_geaendert': 'Position nach der Freigabe geändert',
};

const productionAusfuehrungsklassen = ['EXC1', 'EXC2', 'EXC3', 'EXC4'];

// Nächster Statusschritt je Status: (aktion für die Route, Beschriftung).
const productionNextStep = {
  'entwurf': (aktion: 'freigeben', label: 'Freigeben'),
  'freigegeben': (aktion: 'beginnen', label: 'Beginnen'),
  'in_arbeit': (aktion: 'fertigmelden', label: 'Fertig melden'),
};

// Storno nur vor "fertig".
bool productionCanStorno(String? status) => status == 'entwurf' || status == 'freigegeben' || status == 'in_arbeit';

List<String> productionHints(Map<String, dynamic> o) => ((o['hinweise'] as List?) ?? const []).map((e) => e.toString()).toList();

// formatQty: 1.5 -> "1,5", 2 -> "2", 1.4851 -> "1,485" (höchstens drei
// Nachkommastellen, ohne Nullen am Ende).
String formatQty(Object? v) {
  if (v is! num) return '';
  var s = v.toStringAsFixed(3);
  if (s.contains('.')) {
    s = s.replaceFirst(RegExp(r'0+$'), '');
    s = s.replaceFirst(RegExp(r'\.$'), '');
  }
  return s.replaceAll('.', ',');
}

// productionOrderTitle: "FA-2026-0001 · Pos. 3 Fenster (links)".
String productionOrderTitle(Map<String, dynamic> o) {
  final pos = o['position_nummer']?.toString() ?? '';
  final name = o['position_name']?.toString() ?? '';
  final variante = o['variante_name']?.toString() ?? '';
  final position = pos.isEmpty && name.isEmpty
      ? 'Position nicht mehr vorhanden'
      : 'Pos. ${[pos, name].where((x) => x.isNotEmpty).join(' ')}${variante.isEmpty ? '' : ' ($variante)'}';
  return '${o['nummer'] ?? ''} · $position';
}

// productionOrderSubtitle: "P-1 Testprojekt · 4 Stück · Liefertermin
// 01.12.2026 · EXC2 · Entwurf".
String productionOrderSubtitle(Map<String, dynamic> o) {
  final project = [o['project_nummer'], o['project_name']].map((x) => x?.toString() ?? '').where((x) => x.isNotEmpty).join(' ');
  final termin = isoToGerman(o['liefertermin']?.toString());
  final exc = o['ausfuehrungsklasse']?.toString() ?? '';
  return [
    project,
    '${o['stueckzahl'] ?? ''} Stück',
    if (termin.isNotEmpty) 'Liefertermin $termin',
    if (exc.isNotEmpty) exc,
    productionStatusLabels[o['status']] ?? o['status']?.toString() ?? '',
  ].where((x) => x.isNotEmpty).join(' · ');
}

// productionItemLine beschreibt eine Zeile der Fertigungsstückliste:
// "P-100 Blendrahmen · 1230,4 mm · 2 je Stück · gesamt 6 Stk".
String productionItemLine(Map<String, dynamic> it) {
  final unit = it['unit']?.toString() ?? '';
  final ident =
      [it['article_code'], it['configuration'], it['description']].map((x) => x?.toString() ?? '').where((x) => x.isNotEmpty).join(' ');
  final dims = <String>[
    if (it['length_mm'] is num) '${formatQty(it['length_mm'])} mm',
    if (it['width_mm'] is num && it['height_mm'] is num) '${formatQty(it['width_mm'])} × ${formatQty(it['height_mm'])} mm',
    if (it['area_m2'] is num) '${formatQty(it['area_m2'])} m²',
  ];
  return [
    ident.isEmpty ? '(ohne Bezeichnung)' : ident,
    ...dims,
    '${formatQty(it['menge_je_stueck'])} je Stück',
    'gesamt ${formatQty(it['menge_gesamt'])}${unit.isEmpty ? '' : ' $unit'}',
  ].join(' · ');
}

// productionStatusHistory: "Freigegeben 02.11.2026", ... in der Reihenfolge
// des Ablaufs, nur erreichte Schritte.
List<String> productionStatusHistory(Map<String, dynamic> o) {
  final out = <String>['Angelegt ${isoToGerman(o['created_at']?.toString())}'];
  for (final (key, label) in [
    ('freigegeben_at', 'Freigegeben'),
    ('begonnen_at', 'Begonnen'),
    ('fertig_at', 'Fertig'),
    ('storniert_at', 'Storniert'),
  ]) {
    final d = isoToGerman(o[key]?.toString());
    if (d.isNotEmpty) out.add('$label $d');
  }
  final reason = o['storno_reason']?.toString() ?? '';
  if (reason.isNotEmpty) out[out.length - 1] = '${out.last}: $reason';
  return out;
}

// buildProductionDraftBody prüft die änderbaren Angaben (für Anlegen und
// Ändern) oder liefert eine Meldung. Leerer Liefertermin und leere Klasse
// heißen "keine Angabe".
({Map<String, dynamic>? body, String? error}) buildProductionDraftBody({
  required String stueckzahl,
  String liefertermin = '',
  String ausfuehrungsklasse = '',
  String notiz = '',
}) {
  final n = int.tryParse(stueckzahl.trim());
  if (n == null || n <= 0 || n > 10000) {
    return (body: null, error: 'Stückzahl als ganze Zahl von 1 bis 10000 angeben.');
  }
  String termin = '';
  if (liefertermin.trim().isNotEmpty) {
    final d = parseDateInput(liefertermin);
    if (d == null) return (body: null, error: 'Bitte den Liefertermin als TT.MM.JJJJ angeben.');
    termin = d;
  }
  final exc = ausfuehrungsklasse.trim().toUpperCase();
  if (exc.isNotEmpty && !productionAusfuehrungsklassen.contains(exc)) {
    return (body: null, error: 'Ausführungsklasse EXC1 bis EXC4 oder keine.');
  }
  return (
    body: {'stueckzahl': n, 'liefertermin': termin, 'ausfuehrungsklasse': exc, 'notiz': notiz.trim()},
    error: null,
  );
}

class ProductionOrdersPage extends StatefulWidget {
  const ProductionOrdersPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<ProductionOrdersPage> createState() => _ProductionOrdersPageState();
}

class _ProductionOrdersPageState extends State<ProductionOrdersPage> {
  List<dynamic> orders = [];
  bool loading = false;
  String? error;
  // '' = alle offenen (ohne stornierte), sonst ein Status.
  String status = '';

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
      final list = await widget.api.listProductionOrders(status: status);
      if (mounted) setState(() => orders = list);
    } catch (e) {
      if (mounted) setState(() => error = 'Fertigungsaufträge konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> _open(Map<String, dynamic> o) async {
    await Navigator.of(context)
        .push(MaterialPageRoute(builder: (_) => ProductionOrderDetailPage(api: widget.api, orderId: o['id'].toString())));
    if (mounted) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Fertigungsaufträge'),
        backgroundColor: color,
        foregroundColor: Colors.white,
        actions: [
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh_rounded), tooltip: 'Aktualisieren'),
        ],
      ),
      body: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: 280,
              child: DropdownButtonFormField<String>(
                key: const ValueKey('productionStatusFilter'),
                initialValue: status,
                isExpanded: true,
                decoration: const InputDecoration(labelText: 'Status'),
                items: [
                  const DropdownMenuItem(value: '', child: Text('Alle ohne Storno')),
                  for (final e in productionStatusLabels.entries) DropdownMenuItem(value: e.key, child: Text(e.value)),
                ],
                onChanged: (v) {
                  setState(() => status = v ?? '');
                  _load();
                },
              ),
            ),
            const SizedBox(height: 8),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && orders.isEmpty)
              const Padding(
                padding: EdgeInsets.only(top: 12),
                child: Text('Keine Fertigungsaufträge. Angelegt werden sie im Projekt an der Variante einer Position.'),
              ),
            Expanded(
              child: ListView(
                children: [
                  for (final x in orders)
                    Card(
                      child: ListTile(
                        leading: Icon(
                          productionHints(x as Map<String, dynamic>).isEmpty
                              ? Icons.precision_manufacturing_rounded
                              : Icons.warning_amber_rounded,
                          color: productionHints(x).isEmpty ? null : Colors.orange.shade800,
                        ),
                        title: Text(productionOrderTitle(x)),
                        subtitle: Text([
                          productionOrderSubtitle(x),
                          for (final h in productionHints(x)) productionHintLabels[h] ?? h,
                        ].join('\n')),
                        onTap: () => _open(x),
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class ProductionOrderDetailPage extends StatefulWidget {
  const ProductionOrderDetailPage({super.key, required this.api, required this.orderId});
  final ApiClient api;
  final String orderId;

  @override
  State<ProductionOrderDetailPage> createState() => _ProductionOrderDetailPageState();
}

class _ProductionOrderDetailPageState extends State<ProductionOrderDetailPage> {
  Map<String, dynamic>? order;
  bool loading = false;
  bool busy = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('production.write');

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
      final o = await widget.api.getProductionOrder(widget.orderId);
      if (mounted) setState(() => order = o);
    } catch (e) {
      if (mounted) setState(() => error = 'Fertigungsauftrag konnte nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  // _run führt einen Schreibaufruf aus und übernimmt den Auftrag aus der
  // Antwort.
  Future<void> _run(Future<Map<String, dynamic>> Function() call, String done) async {
    setState(() => busy = true);
    try {
      final o = await call();
      if (mounted) setState(() => order = o);
      _snack(done);
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> _step(String aktion, String label) async {
    if (aktion == 'freigeben') {
      final ok = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Auftrag freigeben'),
          content: const SizedBox(
            width: 420,
            child: Text('Position, Maße und Materialliste werden jetzt in den Auftrag kopiert. '
                'Danach lässt sich der Auftrag nicht mehr ändern – nur stornieren und neu anlegen.'),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
            FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: const Text('Freigeben')),
          ],
        ),
      );
      if (ok != true) return;
    }
    await _run(() => widget.api.productionOrderStep(widget.orderId, aktion), '$label: erledigt');
  }

  Future<void> _editDialog() async {
    final o = order!;
    final stueckCtrl = TextEditingController(text: '${o['stueckzahl'] ?? ''}');
    final terminCtrl = TextEditingController(text: isoToGerman(o['liefertermin']?.toString()));
    final notizCtrl = TextEditingController(text: o['notiz']?.toString() ?? '');
    String exc = o['ausfuehrungsklasse']?.toString() ?? '';
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Entwurf ändern'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                    controller: stueckCtrl, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: 'Stückzahl')),
                TextField(controller: terminCtrl, decoration: const InputDecoration(labelText: 'Liefertermin (TT.MM.JJJJ, optional)')),
                DropdownButtonFormField<String>(
                  key: const ValueKey('productionExc'),
                  initialValue: exc,
                  decoration: const InputDecoration(labelText: 'Ausführungsklasse (EN 1090)'),
                  items: [
                    const DropdownMenuItem(value: '', child: Text('keine')),
                    for (final k in productionAusfuehrungsklassen) DropdownMenuItem(value: k, child: Text(k)),
                  ],
                  onChanged: (v) => setStateDialog(() => exc = v ?? ''),
                ),
                TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
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
                final r = buildProductionDraftBody(
                    stueckzahl: stueckCtrl.text, liefertermin: terminCtrl.text, ausfuehrungsklasse: exc, notiz: notizCtrl.text);
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop(r.body);
              },
              child: const Text('Speichern'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    await _run(() => widget.api.updateProductionOrderDraft(widget.orderId, body), 'Entwurf gespeichert');
  }

  Future<void> _stornoDialog() async {
    final reasonCtrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Auftrag stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(productionOrderTitle(order!)),
                const SizedBox(height: 8),
                const Text('Der Auftrag wird nicht gelöscht, sondern mit Begründung storniert.'),
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
    await _run(() => widget.api.stornoProductionOrder(widget.orderId, reason), 'Auftrag storniert');
  }

  Widget _itemsSection(Map<String, dynamic> o) {
    final items = ((o['items'] as List?) ?? const []).cast<Map<String, dynamic>>();
    if (o['freigegeben_at'] == null) {
      return const Text('Die Fertigungsstückliste wird bei der Freigabe aus der Position kopiert.');
    }
    if (items.isEmpty) return const Text('Die Position hatte bei der Freigabe kein Material.');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final (art, label) in [('profil', 'Profile'), ('artikel', 'Artikel'), ('glas', 'Glas')])
          if (items.any((it) => it['art'] == art)) ...[
            Padding(
              padding: const EdgeInsets.only(top: 8, bottom: 4),
              child: Text(label, style: const TextStyle(fontWeight: FontWeight.bold)),
            ),
            for (final it in items.where((it) => it['art'] == art)) Text(productionItemLine(it)),
          ],
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    final o = order;
    final status = o?['status']?.toString();
    final next = productionNextStep[status];
    return Scaffold(
      appBar: AppBar(
        title: Text(o?['nummer']?.toString() ?? 'Fertigungsauftrag'),
        backgroundColor: color,
        foregroundColor: Colors.white,
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          if (loading || busy) const LinearProgressIndicator(minHeight: 2),
          if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
          if (o != null) ...[
            Text(productionOrderTitle(o), style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
            const SizedBox(height: 4),
            Text(productionOrderSubtitle(o)),
            if ((o['phase_name']?.toString() ?? '').isNotEmpty) Text('Los: ${o['phase_name']}'),
            if (o['width_mm'] is num && o['height_mm'] is num) Text('Maße: ${formatQty(o['width_mm'])} × ${formatQty(o['height_mm'])} mm'),
            if ((o['notiz']?.toString() ?? '').isNotEmpty) Text('Notiz: ${o['notiz']}'),
            for (final h in productionHints(o))
              Padding(
                padding: const EdgeInsets.only(top: 6),
                child: Row(children: [
                  Icon(Icons.warning_amber_rounded, size: 18, color: Colors.orange.shade800),
                  const SizedBox(width: 6),
                  Expanded(child: Text(productionHintLabels[h] ?? h)),
                ]),
              ),
            const SizedBox(height: 8),
            Text(productionStatusHistory(o).join(' · '), style: const TextStyle(color: Colors.black54)),
            if (_canWrite) ...[
              const SizedBox(height: 12),
              Wrap(spacing: 8, runSpacing: 8, children: [
                if (next != null) FilledButton(onPressed: busy ? null : () => _step(next.aktion, next.label), child: Text(next.label)),
                if (status == 'entwurf') OutlinedButton(onPressed: busy ? null : _editDialog, child: const Text('Entwurf ändern')),
                if (productionCanStorno(status)) OutlinedButton(onPressed: busy ? null : _stornoDialog, child: const Text('Stornieren')),
              ]),
            ],
            const SizedBox(height: 16),
            const Text('Fertigungsstückliste', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
            const SizedBox(height: 4),
            _itemsSection(o),
          ],
        ],
      ),
    );
  }
}
