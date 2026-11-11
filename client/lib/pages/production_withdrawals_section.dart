import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'production_orders_page.dart' show formatQty, productionCanWithdraw, productionWarehouseLabel;
import 'production_planning_page.dart' show parseDecimalDe;

// Entnahmen im Detail des Fertigungsauftrags (Backlog H.3.4b, ADR 0038) auf
// den Routen aus H.3.2c: Liste neueste zuerst mit Stornos (production.read);
// mit production.write entnehmen (nur freigegeben und in Arbeit) und
// stornieren (in jedem Status, nur mit Begründung). Lager und Platz zur
// Auswahl nur mit warehouses.read, sonst das Reservierungslager.

const withdrawalHintLabels = {
  'mehrentnahme': 'Mehr entnommen als der Bedarf der Zeile',
};

// withdrawalItemChoices: Zeilen der Stückliste mit Lagermaterial als
// (id, Beschriftung), z. B. "Pos. 3 P-100 Blendrahmen · offen 4,922 m".
List<({String id, String label})> withdrawalItemChoices(Map<String, dynamic> order) {
  final items = ((order['items'] as List?) ?? const []).whereType<Map<String, dynamic>>();
  return [
    for (final it in items)
      if ((it['material_id']?.toString() ?? '').isNotEmpty && it['id'] != null)
        (
          id: it['id'].toString(),
          label: [
            ['Pos. ${it['pos']}', it['article_code'], it['description']]
                .map((x) => x?.toString() ?? '')
                .where((x) => x.isNotEmpty)
                .join(' '),
            if (it['offen'] is num)
              'offen ${formatQty(it['offen'])}${(it['material_einheit']?.toString() ?? '').isEmpty ? '' : ' ${it['material_einheit']}'}',
          ].join(' · '),
        ),
  ];
}

// withdrawalDefaultQty: offene Menge der Zeile als Vorschlag, sonst leer.
String withdrawalDefaultQty(Map<String, dynamic> order, String? itemId) {
  for (final it in ((order['items'] as List?) ?? const []).whereType<Map<String, dynamic>>()) {
    if (it['id']?.toString() == itemId && it['offen'] is num && (it['offen'] as num) > 0) return formatQty(it['offen']);
  }
  return '';
}

// buildWithdrawalBody prüft die Eingaben oder liefert eine Meldung. Grenzen
// wie auf dem Server: Menge über 0 bis 1.000.000, Notiz bis 500 Zeichen.
({Map<String, dynamic>? body, String? error}) buildWithdrawalBody({
  required String? itemId,
  required String? warehouseId,
  String? locationId,
  required String menge,
  String notiz = '',
}) {
  if (itemId == null || itemId.isEmpty) return (body: null, error: 'Bitte eine Zeile der Stückliste wählen.');
  if (warehouseId == null || warehouseId.isEmpty) return (body: null, error: 'Bitte ein Lager wählen.');
  final q = parseDecimalDe(menge);
  if (q == null || q <= 0 || q > 1000000) return (body: null, error: 'Menge größer 0 bis 1.000.000 angeben.');
  if (notiz.trim().length > 500) return (body: null, error: 'Notiz höchstens 500 Zeichen.');
  return (
    body: {
      'item_id': itemId,
      'warehouse_id': warehouseId,
      if (locationId != null && locationId.isNotEmpty) 'location_id': locationId,
      'menge': q,
      'notiz': notiz.trim(),
    },
    error: null,
  );
}

// withdrawalLine: "10.11.2026 · Pos. 3 · mat-1 Profil · 4,922 m · HL – Halle".
String withdrawalLine(Map<String, dynamic> w, List<dynamic> warehouses) {
  final einheit = w['einheit']?.toString() ?? '';
  return [
    isoToGerman(w['created_at']?.toString()),
    if (w['item_pos'] != null) 'Pos. ${w['item_pos']}',
    [w['material_nummer'], w['material_bezeichnung']].map((x) => x?.toString() ?? '').where((x) => x.isNotEmpty).join(' '),
    '${formatQty(w['menge'])}${einheit.isEmpty ? '' : ' $einheit'}',
    productionWarehouseLabel(w['warehouse_id']?.toString() ?? '', warehouses),
  ].where((x) => x.isNotEmpty).join(' · ');
}

bool withdrawalCancelled(Map<String, dynamic> w) => w['storniert_at'] != null;

// withdrawalDetails: Notiz und Storno mit Datum und Begründung, sonst null.
String? withdrawalDetails(Map<String, dynamic> w) {
  final notiz = w['notiz']?.toString() ?? '';
  final rows = [
    if (notiz.isNotEmpty) 'Notiz: $notiz',
    if (withdrawalCancelled(w)) 'Storniert ${isoToGerman(w['storniert_at']?.toString())}: ${w['storno_reason'] ?? ''}',
  ];
  return rows.isEmpty ? null : rows.join('\n');
}

class ProductionWithdrawalsSection extends StatefulWidget {
  const ProductionWithdrawalsSection(
      {super.key, required this.api, required this.order, this.warehouses = const [], required this.onChanged});
  final ApiClient api;
  final Map<String, dynamic> order;
  final List<dynamic> warehouses;
  // Nach Entnahme oder Storno: Auftrag neu laden (entnommen, offen, Hinweise).
  final Future<void> Function() onChanged;

  @override
  State<ProductionWithdrawalsSection> createState() => _ProductionWithdrawalsSectionState();
}

class _ProductionWithdrawalsSectionState extends State<ProductionWithdrawalsSection> {
  List<dynamic> list = const [];
  bool loading = false;
  bool busy = false;
  String? error;

  String get _orderId => widget.order['id'].toString();
  bool get _canWrite => widget.api.hasPermission('production.write');
  bool get _canWithdraw => _canWrite && productionCanWithdraw(widget.order['status']?.toString());
  bool get _canReadWarehouses => widget.api.hasPermission('warehouses.read');

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
      final l = await widget.api.listProductionWithdrawals(_orderId);
      if (mounted) setState(() => list = l);
    } catch (e) {
      if (mounted) setState(() => error = 'Entnahmen konnten nicht geladen werden: ${_message(e)}');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  String _message(Object e) => e is ApiException ? e.message : e.toString();

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  // _write bucht, meldet Hinweise und lädt Liste und Auftrag neu.
  Future<void> _write(Future<Map<String, dynamic>> Function() call, String done) async {
    setState(() => busy = true);
    try {
      final w = await call();
      final hints = ((w['hinweise'] as List?) ?? const []).map((h) => withdrawalHintLabels[h] ?? h.toString());
      _snack([done, ...hints].join(' – '));
      await _load();
      await widget.onChanged();
    } catch (e) {
      _snack('Fehler: ${_message(e)}');
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> _dialog() async {
    final items = withdrawalItemChoices(widget.order);
    final reserved = widget.order['reservierung_lager_id']?.toString() ?? '';
    String? itemId = items.isEmpty ? null : items.first.id;
    String? warehouseId = reserved.isEmpty ? null : reserved;
    String? locationId;
    List<dynamic> locations = const [];
    final mengeCtrl = TextEditingController(text: withdrawalDefaultQty(widget.order, itemId));
    final notizCtrl = TextEditingController();
    String? dialogError;

    Future<void> loadLocations(StateSetter setStateDialog) async {
      locations = const [];
      locationId = null;
      if (!_canReadWarehouses || warehouseId == null) return;
      try {
        final l = await widget.api.listLocations(warehouseId!);
        setStateDialog(() => locations = l);
      } catch (e) {
        setStateDialog(() => dialogError = 'Plätze konnten nicht geladen werden: ${_message(e)}');
      }
    }

    var started = false;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(builder: (ctx, setStateDialog) {
        if (!started) {
          started = true;
          loadLocations(setStateDialog);
        }
        return AlertDialog(
          title: const Text('Material entnehmen'),
          content: SizedBox(
            width: 480,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (items.isEmpty)
                    const Text('Keine Zeile der Stückliste hat Lagermaterial.')
                  else
                    DropdownButtonFormField<String>(
                      initialValue: itemId,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Zeile der Stückliste'),
                      items: [
                        for (final c in items) DropdownMenuItem(value: c.id, child: Text(c.label, overflow: TextOverflow.ellipsis)),
                      ],
                      onChanged: (v) => setStateDialog(() {
                        itemId = v;
                        mengeCtrl.text = withdrawalDefaultQty(widget.order, v);
                      }),
                    ),
                  if (_canReadWarehouses && widget.warehouses.isNotEmpty)
                    DropdownButtonFormField<String>(
                      initialValue: warehouseId,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Lager'),
                      items: [
                        for (final w in widget.warehouses)
                          if (w is Map && w['id'] != null)
                            DropdownMenuItem(
                                value: w['id'].toString(),
                                child:
                                    Text(productionWarehouseLabel(w['id'].toString(), widget.warehouses), overflow: TextOverflow.ellipsis)),
                      ],
                      onChanged: (v) {
                        setStateDialog(() => warehouseId = v);
                        loadLocations(setStateDialog);
                      },
                    )
                  else
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(
                        warehouseId == null
                            ? 'Ohne Leserecht für Lager und ohne Reservierungslager ist keine Entnahme möglich.'
                            : 'Lager: Reservierungslager ${productionWarehouseLabel(warehouseId!, widget.warehouses)}',
                        style: const TextStyle(color: Colors.black54),
                      ),
                    ),
                  if (locations.isNotEmpty)
                    DropdownButtonFormField<String>(
                      key: ValueKey('loc-$warehouseId'),
                      initialValue: locationId ?? '',
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Platz'),
                      items: [
                        const DropdownMenuItem(value: '', child: Text('Ohne Platz')),
                        for (final l in locations)
                          if (l is Map && l['id'] != null)
                            DropdownMenuItem(
                                value: l['id'].toString(),
                                child: Text([l['code'], l['name']].map((x) => x?.toString() ?? '').where((x) => x.isNotEmpty).join(' – '),
                                    overflow: TextOverflow.ellipsis)),
                      ],
                      onChanged: (v) => setStateDialog(() => locationId = v),
                    ),
                  TextField(controller: mengeCtrl, decoration: const InputDecoration(labelText: 'Menge (Einheit des Lagermaterials)')),
                  TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz (optional)')),
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
                final r = buildWithdrawalBody(
                    itemId: itemId, warehouseId: warehouseId, locationId: locationId, menge: mengeCtrl.text, notiz: notizCtrl.text);
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop(r.body);
              },
              child: const Text('Entnehmen'),
            ),
          ],
        );
      }),
    );
    if (body == null) return;
    await _write(() => widget.api.createProductionWithdrawal(_orderId, body), 'Entnahme gebucht');
  }

  Future<void> _storno(Map<String, dynamic> w) async {
    final reasonCtrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Entnahme stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(withdrawalLine(w, widget.warehouses)),
              const SizedBox(height: 8),
              const Text('Das Material wird ins selbe Lager und auf denselben Platz zurückgebucht.'),
              TextField(
                controller: reasonCtrl,
                decoration: const InputDecoration(labelText: 'Begründung'),
                onChanged: (_) => setStateDialog(() {}),
              ),
            ]),
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
    await _write(() => widget.api.stornoProductionWithdrawal(_orderId, w['id'].toString(), reason), 'Entnahme storniert');
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(children: [
          const Expanded(child: Text('Entnahmen', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold))),
          if (_canWithdraw)
            OutlinedButton.icon(
              onPressed: busy ? null : _dialog,
              icon: const Icon(Icons.outbox_outlined),
              label: const Text('Entnehmen'),
            ),
        ]),
        if (loading || busy) const LinearProgressIndicator(minHeight: 2),
        if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (!loading && error == null && list.isEmpty) const Text('Noch keine Entnahmen.'),
        for (final x in list.whereType<Map<String, dynamic>>())
          ListTile(
            dense: true,
            contentPadding: EdgeInsets.zero,
            title: Text(
              withdrawalLine(x, widget.warehouses),
              style: withdrawalCancelled(x) ? const TextStyle(decoration: TextDecoration.lineThrough, color: Colors.black54) : null,
            ),
            subtitle: withdrawalDetails(x) == null ? null : Text(withdrawalDetails(x)!),
            trailing: _canWrite && !withdrawalCancelled(x)
                ? IconButton(
                    tooltip: 'Entnahme stornieren', icon: const Icon(Icons.undo_rounded), onPressed: busy ? null : () => _storno(x))
                : null,
          ),
      ],
    );
  }
}
