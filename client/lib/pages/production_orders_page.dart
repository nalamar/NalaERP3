import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Fertigungsaufträge im Client (ADR 0034, Backlog H.1.4a, Routen H.1.3):
// Liste nach Liefertermin und Detail mit Hinweisen, Fertigungsstückliste
// und Statusschritten. Lesen mit production.read; anlegen, ändern,
// freigeben, Status und Storno mit production.write. Angelegt wird an der
// Variante im Projekt (H.1.4b). Bei der Freigabe optional ein Lager für die
// Reservierung (mit warehouses.read), je Zeile Bedarf, reserviert und
// Fehlmenge (H.1.5c).

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
  'fehlmenge': 'Nicht alles Material im Lager verfügbar (Fehlmenge)',
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

// productionReservationLine: "Bedarf 4,5 Stk · reserviert 4 Stk · Fehlmenge
// 0,5 Stk" in der Einheit des Lagermaterials (H.1.5); null, wenn die Zeile
// nicht reserviert wird (ohne Lager, ohne Material, Einheit passt nicht).
String? productionReservationLine(Map<String, dynamic> it) {
  if (it['bedarf'] is! num) return null;
  final unit = it['material_einheit']?.toString() ?? '';
  String q(Object? v) => '${formatQty(v is num ? v : 0)}${unit.isEmpty ? '' : ' $unit'}';
  final fehl = it['fehlmenge'];
  return [
    'Bedarf ${q(it['bedarf'])}',
    'reserviert ${q(it['reserviert'])}',
    if (fehl is num && fehl > 0) 'Fehlmenge ${q(fehl)}',
  ].join(' · ');
}

bool productionItemHasShortfall(Map<String, dynamic> it) => it['fehlmenge'] is num && (it['fehlmenge'] as num) > 0;

// productionWarehouseLabel: "HL – Halle" aus der Lagerliste, sonst die ID.
String productionWarehouseLabel(String id, List<dynamic> warehouses) {
  for (final w in warehouses) {
    if (w is Map && w['id']?.toString() == id) {
      return [w['code'], w['name']].map((x) => x?.toString() ?? '').where((x) => x.isNotEmpty).join(' – ');
    }
  }
  return id;
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

// defaultProductionStueckzahl schlägt die Menge der Variante als Stückzahl
// vor, wenn sie eine ganze Zahl von 1 bis 10000 ist; sonst bleibt das Feld
// leer.
String defaultProductionStueckzahl(Object? menge) {
  final n = menge is num ? menge : num.tryParse(menge?.toString().trim().replaceAll(',', '.') ?? '');
  if (n == null || n != n.roundToDouble() || n < 1 || n > 10000) return '';
  return n.round().toString();
}

// productionOrdersOfVariant: die Aufträge einer Variante aus der Liste der
// Position.
List<Map<String, dynamic>> productionOrdersOfVariant(List<dynamic> orders, String variantId) => orders
    .whereType<Map<String, dynamic>>()
    .where((o) => o['single_elevation_id']?.toString() == variantId)
    .toList();

// showProductionDraftDialog fragt Stückzahl, Liefertermin und Notiz ab
// (anlegen und Entwurf ändern) und liefert den geprüften Body oder null.
// Die Ausführungsklasse wird nicht abgefragt, eine vorhandene geht
// unverändert in den Body (H.1.6).
Future<Map<String, dynamic>?> showProductionDraftDialog(
  BuildContext context, {
  required String title,
  required String saveLabel,
  String stueckzahl = '',
  String liefertermin = '',
  String ausfuehrungsklasse = '',
  String notiz = '',
}) {
  final stueckCtrl = TextEditingController(text: stueckzahl);
  final terminCtrl = TextEditingController(text: liefertermin);
  final notizCtrl = TextEditingController(text: notiz);
  final exc = ausfuehrungsklasse.trim();
  String? dialogError;
  return showDialog<Map<String, dynamic>>(
    context: context,
    builder: (ctx) => StatefulBuilder(
      builder: (ctx, setStateDialog) => AlertDialog(
        title: Text(title),
        content: SizedBox(
          width: 420,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                  controller: stueckCtrl, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: 'Stückzahl')),
              TextField(controller: terminCtrl, decoration: const InputDecoration(labelText: 'Liefertermin (TT.MM.JJJJ, optional)')),
              // Ausführungsklasse nicht mehr zur Eingabe (ADR 0034,
              // Präzisierung EN 1090, Frage 8): ein gesetzter Wert bleibt
              // beim Speichern erhalten und wird nur angezeigt.
              if (exc.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: Text('Ausführungsklasse: $exc (bleibt erhalten)', style: const TextStyle(color: Colors.black54)),
                  ),
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
            child: Text(saveLabel),
          ),
        ],
      ),
    ),
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
  // Lager für die Reservierung bei der Freigabe; nur mit warehouses.read.
  List<dynamic> warehouses = const [];
  String? warehousesError;

  bool get _canWrite => widget.api.hasPermission('production.write');
  bool get _canReadWarehouses => widget.api.hasPermission('warehouses.read');

  @override
  void initState() {
    super.initState();
    _load();
    if (_canReadWarehouses) _loadWarehouses();
  }

  Future<void> _loadWarehouses() async {
    try {
      final list = await widget.api.listWarehouses();
      if (mounted) setState(() => warehouses = list);
    } catch (e) {
      if (mounted) setState(() => warehousesError = 'Lager konnten nicht geladen werden: $e');
    }
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
      // null: abgebrochen; '': ohne Reservierung.
      var lager = '';
      final lagerId = await showDialog<String>(
        context: context,
        builder: (ctx) => StatefulBuilder(
          builder: (ctx, setStateDialog) => AlertDialog(
            title: const Text('Auftrag freigeben'),
            content: SizedBox(
              width: 420,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('Position, Maße und Materialliste werden jetzt in den Auftrag kopiert. '
                      'Danach lässt sich der Auftrag nicht mehr ändern – nur stornieren und neu anlegen.'),
                  const SizedBox(height: 12),
                  if (_canReadWarehouses && warehouses.isNotEmpty)
                    DropdownButtonFormField<String>(
                      initialValue: lager,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Material reservieren aus'),
                      items: [
                        const DropdownMenuItem(value: '', child: Text('Ohne Reservierung')),
                        for (final w in warehouses)
                          if (w is Map && w['id'] != null)
                            DropdownMenuItem(
                              value: w['id'].toString(),
                              child: Text(productionWarehouseLabel(w['id'].toString(), warehouses), overflow: TextOverflow.ellipsis),
                            ),
                      ],
                      onChanged: (v) => setStateDialog(() => lager = v ?? ''),
                    )
                  else
                    Text(
                      warehousesError ??
                          (_canReadWarehouses
                              ? 'Kein Lager angelegt – Freigabe ohne Reservierung.'
                              : 'Ohne Leserecht für Lager wird ohne Reservierung freigegeben.'),
                      style: const TextStyle(color: Colors.black54),
                    ),
                ],
              ),
            ),
            actions: [
              TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
              FilledButton(onPressed: () => Navigator.of(ctx).pop(lager), child: const Text('Freigeben')),
            ],
          ),
        ),
      );
      if (lagerId == null) return;
      await _run(() => widget.api.releaseProductionOrder(widget.orderId, lagerId: lagerId.isEmpty ? null : lagerId), '$label: erledigt');
      return;
    }
    await _run(() => widget.api.productionOrderStep(widget.orderId, aktion), '$label: erledigt');
  }

  Future<void> _editDialog() async {
    final o = order!;
    final body = await showProductionDraftDialog(
      context,
      title: 'Entwurf ändern',
      saveLabel: 'Speichern',
      stueckzahl: '${o['stueckzahl'] ?? ''}',
      liefertermin: isoToGerman(o['liefertermin']?.toString()),
      ausfuehrungsklasse: o['ausfuehrungsklasse']?.toString() ?? '',
      notiz: o['notiz']?.toString() ?? '',
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
            for (final it in items.where((it) => it['art'] == art)) ...[
              Text(productionItemLine(it)),
              if (productionReservationLine(it) case final line?)
                Padding(
                  padding: const EdgeInsets.only(left: 12, bottom: 2),
                  child: Text(
                    line,
                    style: TextStyle(fontSize: 12, color: productionItemHasShortfall(it) ? Colors.orange.shade900 : Colors.black54),
                  ),
                ),
            ],
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
            if ((o['reservierung_lager_id']?.toString() ?? '').isNotEmpty)
              Text('Reserviert aus Lager: ${productionWarehouseLabel(o['reservierung_lager_id'].toString(), warehouses)}'),
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

// VariantProductionOrders zeigt an der Variante im Projekt ihre
// Fertigungsaufträge (production.read) und legt einen neuen Entwurf an
// (production.write); danach öffnet sich das Detail.
class VariantProductionOrders extends StatefulWidget {
  const VariantProductionOrders({
    super.key,
    required this.api,
    required this.projectId,
    required this.elevationId,
    required this.variantId,
    this.variantMenge,
  });
  final ApiClient api;
  final String projectId;
  final String elevationId;
  final String variantId;
  final Object? variantMenge;

  @override
  State<VariantProductionOrders> createState() => _VariantProductionOrdersState();
}

class _VariantProductionOrdersState extends State<VariantProductionOrders> {
  List<Map<String, dynamic>>? orders;
  String? error;
  bool busy = false;

  bool get _canRead => widget.api.hasPermission('production.read');
  bool get _canWrite => widget.api.hasPermission('production.write');

  @override
  void initState() {
    super.initState();
    if (_canRead) _load();
  }

  Future<void> _load() async {
    try {
      final list = await widget.api.listProductionOrders(projectId: widget.projectId, elevationId: widget.elevationId);
      if (mounted) {
        setState(() {
          orders = productionOrdersOfVariant(list, widget.variantId);
          error = null;
        });
      }
    } catch (e) {
      if (mounted) setState(() => error = 'Fertigungsaufträge konnten nicht geladen werden: $e');
    }
  }

  Future<void> _open(String id) async {
    await Navigator.of(context).push(MaterialPageRoute(builder: (_) => ProductionOrderDetailPage(api: widget.api, orderId: id)));
    if (mounted && _canRead) await _load();
  }

  Future<void> _create() async {
    final body = await showProductionDraftDialog(
      context,
      title: 'Fertigungsauftrag anlegen',
      saveLabel: 'Anlegen',
      stueckzahl: defaultProductionStueckzahl(widget.variantMenge),
    );
    if (body == null || !mounted) return;
    setState(() => busy = true);
    try {
      final o = await widget.api.createProductionOrder({
        ...body,
        'project_id': widget.projectId,
        'elevation_id': widget.elevationId,
        'single_elevation_id': widget.variantId,
      });
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fertigungsauftrag ${o['nummer'] ?? ''} angelegt')));
      await _open(o['id'].toString());
    } on ApiException catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: ${e.message}')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (!_canRead && !_canWrite) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 12),
        Row(children: [
          const Expanded(child: Text('Fertigungsaufträge', style: TextStyle(fontWeight: FontWeight.bold))),
          if (_canWrite)
            TextButton.icon(
              onPressed: busy ? null : _create,
              icon: const Icon(Icons.precision_manufacturing_rounded),
              label: const Text('Fertigungsauftrag anlegen'),
            ),
        ]),
        if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (_canRead && orders != null && orders!.isEmpty) const Text('Noch kein Fertigungsauftrag zu dieser Variante.'),
        for (final o in orders ?? const <Map<String, dynamic>>[])
          ListTile(
            dense: true,
            contentPadding: EdgeInsets.zero,
            leading: productionHints(o).isEmpty
                ? const Icon(Icons.assignment_rounded)
                : Icon(Icons.warning_amber_rounded, color: Colors.orange.shade800),
            title: Text(productionOrderTitle(o)),
            subtitle: Text(productionOrderSubtitle(o)),
            onTap: () => _open(o['id'].toString()),
          ),
      ],
    );
  }
}
