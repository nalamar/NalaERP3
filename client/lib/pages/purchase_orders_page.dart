import 'package:flutter/material.dart';
import '../api.dart';
import 'purchase_order_detail_page.dart';

class PurchaseOrdersPage extends StatefulWidget {
  const PurchaseOrdersPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<PurchaseOrdersPage> createState() => _PurchaseOrdersPageState();
}

class _PurchaseOrdersPageState extends State<PurchaseOrdersPage> {
  List<dynamic> items = [];
  bool loading = false;
  final searchCtrl = TextEditingController();
  String? filterStatus;
  List<String> statuses = [];
  int limit = 20;
  int offset = 0;

  // Create dialog state
  String? supplierId;
  final numberCtrl = TextEditingController();
  final currencyCtrl = TextEditingController(text: 'EUR');
  String status = 'draft';
  final noteCtrl = TextEditingController();
  // One simple item row for MVP (can be extended to multiple later)
  String? itemMaterialId;
  final itemDescCtrl = TextEditingController();
  final itemQtyCtrl = TextEditingController(text: '1');
  final itemUomCtrl = TextEditingController(text: 'Stk');
  final itemPriceCtrl = TextEditingController(text: '0');
  List<dynamic> suppliers = [];
  List<dynamic> materials = [];

  @override
  void initState() {
    super.initState();
    _loadFacets();
    _preloadRefs();
    _reload();
  }

  Future<void> _loadFacets() async {
    try { statuses = await widget.api.listPOStatuses(); setState((){}); } catch (_) {}
  }

  Future<void> _preloadRefs() async {
    try {
      suppliers = await widget.api.listContacts(rolle: 'supplier', limit: 100);
      materials = await widget.api.listMaterials(limit: 100);
      setState(() {});
    } catch (e) { debugPrint('refs error: $e'); }
  }

  Future<void> _reload() async {
    setState(()=> loading = true);
    try {
      offset = 0;
      items = await widget.api.listPurchaseOrders(q: searchCtrl.text.trim(), status: filterStatus, limit: limit, offset: offset);
    } catch (e) {
      if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Bestellungen konnten nicht geladen werden: $e'))); }
    } finally { setState(()=> loading = false); }
  }

  Future<void> _loadMore() async {
    final next = await widget.api.listPurchaseOrders(q: searchCtrl.text.trim(), status: filterStatus, limit: limit, offset: offset+limit);
    if (next.isNotEmpty) {
      offset += limit; setState(()=> items.addAll(next));
    }
  }

  Future<void> _openCreateDialog() async {
    await showDialog(context: context, builder: (ctx) => AlertDialog(
      title: const Text('Bestellung anlegen'),
      content: SizedBox(
        width: 720,
        child: SingleChildScrollView(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Wrap(spacing: 12, runSpacing: 12, children: [
              SizedBox(
                width: 320,
                child: DropdownButtonFormField<String>(
                  value: supplierId,
                  decoration: const InputDecoration(labelText: 'Lieferant'),
                  items: [for (final s in suppliers) DropdownMenuItem(value: s['id'] as String, child: Text((s['name']??'').toString()))],
                  onChanged: (v)=> setState(()=> supplierId = v),
                ),
              ),
              SizedBox(width: 160, child: TextFormField(controller: numberCtrl, decoration: const InputDecoration(labelText: 'Bestellnummer'))),
              SizedBox(width: 120, child: TextFormField(controller: currencyCtrl, decoration: const InputDecoration(labelText: 'Währung'))),
              SizedBox(
                width: 180,
                child: DropdownButtonFormField<String>(
                  value: status,
                  decoration: const InputDecoration(labelText: 'Status'),
                  items: [for (final st in (statuses.isEmpty? ['draft','ordered','received','canceled'] : statuses)) DropdownMenuItem(value: st, child: Text(st))],
                  onChanged: (v)=> setState(()=> status = v ?? 'draft'),
                ),
              ),
              SizedBox(width: 520, child: TextFormField(controller: noteCtrl, decoration: const InputDecoration(labelText: 'Notiz'))),
            ]),
            const SizedBox(height: 12),
            const Text('Position (MVP, eine Zeile)'),
            Wrap(spacing: 12, runSpacing: 12, children: [
              SizedBox(
                width: 320,
                child: DropdownButtonFormField<String>(
                  value: itemMaterialId,
                  decoration: const InputDecoration(labelText: 'Material'),
                  items: [for (final m in materials) DropdownMenuItem(value: m['id'] as String, child: Text('${m['nummer']} – ${m['bezeichnung']}'))],
                  onChanged: (v)=> setState(()=> itemMaterialId = v),
                ),
              ),
              SizedBox(width: 120, child: TextFormField(controller: itemQtyCtrl, decoration: const InputDecoration(labelText: 'Menge'))),
              SizedBox(width: 120, child: TextFormField(controller: itemUomCtrl, decoration: const InputDecoration(labelText: 'Einheit'))),
              SizedBox(width: 160, child: TextFormField(controller: itemPriceCtrl, decoration: const InputDecoration(labelText: 'Preis'))),
              SizedBox(width: 520, child: TextFormField(controller: itemDescCtrl, decoration: const InputDecoration(labelText: 'Bezeichnung/Notiz'))),
            ])
          ]),
        ),
      ),
      actions: [
        TextButton(onPressed: ()=> Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
        FilledButton.icon(onPressed: () async {
          try {
            final body = {
              'lieferant_id': supplierId,
              'nummer': numberCtrl.text.trim(),
              'waehrung': currencyCtrl.text.trim().isEmpty? 'EUR' : currencyCtrl.text.trim().toUpperCase(),
              'status': status,
              'notiz': noteCtrl.text.trim(),
              'positionen': [
                if (itemMaterialId != null) {
                  'material_id': itemMaterialId,
                  'bezeichnung': itemDescCtrl.text.trim(),
                  'menge': double.tryParse(itemQtyCtrl.text.trim()) ?? 0,
                  'einheit': itemUomCtrl.text.trim(),
                  'preis': double.tryParse(itemPriceCtrl.text.trim()) ?? 0,
                  'waehrung': currencyCtrl.text.trim().isEmpty? 'EUR' : currencyCtrl.text.trim().toUpperCase(),
                }
              ]
            };
            final resp = await widget.api.createPurchaseOrder(body);
            if (mounted) Navigator.of(ctx).pop();
            await _reload();
          } catch (e) { if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); } }
        }, icon: const Icon(Icons.check), label: const Text('Anlegen')),
      ],
    ));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      floatingActionButtonLocation: FloatingActionButtonLocation.startFloat,
      floatingActionButton: FloatingActionButton(onPressed: _openCreateDialog, child: const Icon(Icons.add)),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(12),
            child: Row(children: [
              const Text('Bestellungen', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              const Spacer(),
              SizedBox(
                width: 260,
                child: TextField(
                  controller: searchCtrl,
                  decoration: InputDecoration(isDense: true, prefixIcon: const Icon(Icons.search), hintText: 'Suchen (Nummer)', suffixIcon: IconButton(icon: const Icon(Icons.clear), onPressed: () { searchCtrl.clear(); _reload(); })),
                  onSubmitted: (_) => _reload(),
                ),
              ),
              const SizedBox(width: 8),
              SizedBox(
                width: 180,
                child: InputDecorator(
                  decoration: const InputDecoration(isDense: true, labelText: 'Status'),
                  child: DropdownButton<String?>(
                    isExpanded: true,
                    value: filterStatus,
                    hint: const Text('Alle'),
                    items: [
                      const DropdownMenuItem<String?>(value: null, child: Text('Alle')),
                      for (final s in (statuses.isEmpty? ['draft','ordered','received','canceled'] : statuses)) DropdownMenuItem<String?>(value: s, child: Text(s))
                    ],
                    onChanged: (v){ setState(()=> filterStatus = v); _reload(); },
                    underline: const SizedBox.shrink(),
                  ),
                ),
              ),
            ]),
          ),
          if (loading) const LinearProgressIndicator(minHeight: 2),
          Expanded(
            child: ListView.builder(
              itemCount: items.length + 1,
              itemBuilder: (ctx, i) {
                if (i < items.length) {
                  final po = items[i] as Map<String, dynamic>;
                  return ListTile(
                    leading: const Icon(Icons.receipt_long),
                    title: Text('${po['nummer'] ?? ''}  •  ${po['status'] ?? ''}'),
                    subtitle: Text('Datum: ${po['datum'] ?? ''}  •  Währung: ${po['waehrung'] ?? 'EUR'}'),
                    onTap: (){
                      final id = (po['id'] ?? '').toString();
                      if (id.isNotEmpty){ Navigator.of(context).push(MaterialPageRoute(builder: (_)=> PurchaseOrderDetailPage(api: widget.api, id: id))).then((_)=> _reload()); }
                    },
                  );
                }
                final canLoadMore = items.isNotEmpty && items.length % limit == 0;
                if (!canLoadMore) return const SizedBox.shrink();
                return Padding(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  child: Center(child: FilledButton.icon(onPressed: _loadMore, icon: const Icon(Icons.expand_more), label: const Text('Mehr laden'))),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
