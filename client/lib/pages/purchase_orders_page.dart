import 'package:flutter/material.dart';
import '../api.dart';
import '../commercial_navigation.dart';
import 'purchase_order_detail_page.dart';

class PurchaseOrdersPage extends StatefulWidget {
  const PurchaseOrdersPage({
    super.key,
    required this.api,
    this.initialContext,
    this.initialFilters,
    this.initialCreatePrefill,
    this.openCreateOnStart = false,
  });
  final ApiClient api;
  final CommercialListContext? initialContext;
  final PurchaseOrderFilterContext? initialFilters;
  final PurchaseOrderCreatePrefillContext? initialCreatePrefill;
  final bool openCreateOnStart;

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
  bool _initialSelectionHandled = false;
  bool _initialCreateHandled = false;

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

  Map<String, dynamic>? get _selectedSupplier {
    final id = supplierId;
    if (id == null || id.isEmpty) return null;
    for (final supplier in suppliers) {
      final mapped = supplier as Map<String, dynamic>;
      if ((mapped['id'] ?? '').toString() == id) {
        return mapped;
      }
    }
    return null;
  }

  String _errorMessage(Object error,
      {String fallback = 'Vorgang fehlgeschlagen'}) {
    if (error is ApiException) {
      switch (error.code) {
        case 'validation_error':
          return error.message;
        case 'not_found':
          return 'Bestellung nicht gefunden oder nicht mehr verfügbar.';
        case 'internal_error':
          return 'Serverfehler. Bitte erneut versuchen.';
      }
      return error.message;
    }
    return '$fallback: $error';
  }

  String _supplierCommercialSummary(Map<String, dynamic> supplier) {
    final parts = <String>[];
    final creditorNo = (supplier['kreditor_nr'] ?? '').toString().trim();
    final paymentTerms =
        (supplier['zahlungsbedingungen'] ?? '').toString().trim();
    final taxCountry = (supplier['steuer_land'] ?? '').toString().trim();
    final taxExempt = supplier['steuerbefreit'] == true;
    if (creditorNo.isNotEmpty) parts.add('Kreditor-Nr.: $creditorNo');
    if (paymentTerms.isNotEmpty)
      parts.add('Zahlungsbedingungen: $paymentTerms');
    if (taxCountry.isNotEmpty) parts.add('Steuerland: $taxCountry');
    if (taxExempt) parts.add('Steuerbefreit');
    return parts.join(' • ');
  }

  @override
  void initState() {
    super.initState();
    final initialSearch = widget.initialContext?.effectiveSearchQuery;
    if (initialSearch != null) {
      searchCtrl.text = initialSearch;
    }
    filterStatus = widget.initialFilters?.normalizedStatus;
    final initialDetailId = widget.initialContext?.normalizedDetailId;
    if (initialDetailId != null && initialDetailId.isNotEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted || _initialSelectionHandled) return;
        _initialSelectionHandled = true;
        _openDetail(initialDetailId);
      });
    }
    _loadFacets();
    _preloadRefs();
    _reload();
  }

  Future<void> _loadFacets() async {
    try {
      statuses = await widget.api.listPOStatuses();
      setState(() {});
    } catch (_) {}
    if (widget.openCreateOnStart &&
        !_initialCreateHandled &&
        widget.api.hasPermission('purchase_orders.write')) {
      _initialCreateHandled = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        _openCreateDialog();
      });
    }
  }

  Future<void> _preloadRefs() async {
    try {
      suppliers = await widget.api.listContacts(rolle: 'supplier', limit: 100);
      materials = await widget.api.listMaterials(limit: 100);
      setState(() {});
    } catch (e) {
      debugPrint('refs error: $e');
    }
  }

  Future<void> _reload() async {
    setState(() => loading = true);
    try {
      offset = 0;
      items = await widget.api.listPurchaseOrders(
          q: searchCtrl.text.trim(),
          status: filterStatus,
          limit: limit,
          offset: offset);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
              content: Text(_errorMessage(e,
                  fallback: 'Bestellungen konnten nicht geladen werden'))),
        );
      }
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> _openDetail(String id) async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => PurchaseOrderDetailPage(api: widget.api, id: id),
      ),
    );
    if (!mounted) return;
    await _reload();
  }

  Future<void> _loadMore() async {
    final next = await widget.api.listPurchaseOrders(
        q: searchCtrl.text.trim(),
        status: filterStatus,
        limit: limit,
        offset: offset + limit);
    if (next.isNotEmpty) {
      offset += limit;
      setState(() => items.addAll(next));
    }
  }

  Future<void> _openCreateDialog() async {
    final prefilledNote = widget.initialCreatePrefill?.normalizedNote;
    if (prefilledNote != null) {
      noteCtrl.text = prefilledNote;
    }
    final prefilledItemDescription =
        widget.initialCreatePrefill?.normalizedItemDescription;
    if (prefilledItemDescription != null) {
      itemDescCtrl.text = prefilledItemDescription;
    }
    final prefilledItemMaterialId =
        widget.initialCreatePrefill?.normalizedItemMaterialId;
    if (prefilledItemMaterialId != null) {
      itemMaterialId = prefilledItemMaterialId;
    }
    final prefilledItemQuantity =
        widget.initialCreatePrefill?.normalizedItemQuantity;
    if (prefilledItemQuantity != null) {
      itemQtyCtrl.text = prefilledItemQuantity.toString();
    }
    final prefilledItemUnit = widget.initialCreatePrefill?.normalizedItemUnit;
    if (prefilledItemUnit != null) {
      itemUomCtrl.text = prefilledItemUnit;
    }
    await showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
              title: const Text('Bestellung anlegen'),
              content: SizedBox(
                width: 720,
                child: SingleChildScrollView(
                  child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Wrap(spacing: 12, runSpacing: 12, children: [
                          SizedBox(
                            width: 320,
                            child: DropdownButtonFormField<String>(
                              isExpanded: true,
                              initialValue: supplierId,
                              decoration:
                                  const InputDecoration(labelText: 'Lieferant'),
                              items: [
                                for (final s in suppliers)
                                  DropdownMenuItem(
                                      value: s['id'] as String,
                                      child: Text((s['name'] ?? '').toString()))
                              ],
                              onChanged: (v) => setState(() => supplierId = v),
                            ),
                          ),
                          if (_selectedSupplier != null)
                            SizedBox(
                              width: 520,
                              child: Container(
                                padding: const EdgeInsets.all(12),
                                decoration: BoxDecoration(
                                  color: Theme.of(context)
                                      .colorScheme
                                      .surfaceContainerHighest,
                                  borderRadius: BorderRadius.circular(12),
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      (_selectedSupplier!['name'] ?? '')
                                          .toString(),
                                      style: const TextStyle(
                                          fontWeight: FontWeight.w600),
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      _supplierCommercialSummary(
                                          _selectedSupplier!),
                                      style:
                                          Theme.of(context).textTheme.bodySmall,
                                    ),
                                  ],
                                ),
                              ),
                            ),
                          SizedBox(
                              width: 160,
                              child: TextFormField(
                                  controller: numberCtrl,
                                  decoration: const InputDecoration(
                                      labelText: 'Bestellnummer'))),
                          SizedBox(
                              width: 120,
                              child: TextFormField(
                                  controller: currencyCtrl,
                                  decoration: const InputDecoration(
                                      labelText: 'Währung'))),
                          SizedBox(
                            width: 180,
                            child: DropdownButtonFormField<String>(
                              initialValue: status,
                              decoration:
                                  const InputDecoration(labelText: 'Status'),
                              items: [
                                for (final st in (statuses.isEmpty
                                    ? [
                                        'draft',
                                        'ordered',
                                        'received',
                                        'canceled'
                                      ]
                                    : statuses))
                                  DropdownMenuItem(value: st, child: Text(st))
                              ],
                              onChanged: (v) =>
                                  setState(() => status = v ?? 'draft'),
                            ),
                          ),
                          SizedBox(
                              width: 520,
                              child: TextFormField(
                                  controller: noteCtrl,
                                  decoration: const InputDecoration(
                                      labelText: 'Notiz'))),
                        ]),
                        const SizedBox(height: 12),
                        const Text('Position (MVP, eine Zeile)'),
                        Wrap(spacing: 12, runSpacing: 12, children: [
                          SizedBox(
                            width: 320,
                            child: DropdownButtonFormField<String>(
                              isExpanded: true,
                              initialValue: itemMaterialId,
                              decoration:
                                  const InputDecoration(labelText: 'Material'),
                              items: [
                                for (final m in materials)
                                  DropdownMenuItem(
                                      value: m['id'] as String,
                                      child: Text(
                                          '${m['nummer']} – ${m['bezeichnung']}'))
                              ],
                              onChanged: (v) =>
                                  setState(() => itemMaterialId = v),
                            ),
                          ),
                          SizedBox(
                              width: 120,
                              child: TextFormField(
                                  controller: itemQtyCtrl,
                                  decoration: const InputDecoration(
                                      labelText: 'Menge'))),
                          SizedBox(
                              width: 120,
                              child: TextFormField(
                                  controller: itemUomCtrl,
                                  decoration: const InputDecoration(
                                      labelText: 'Einheit'))),
                          SizedBox(
                              width: 160,
                              child: TextFormField(
                                  controller: itemPriceCtrl,
                                  decoration: const InputDecoration(
                                      labelText: 'Preis'))),
                          SizedBox(
                              width: 520,
                              child: TextFormField(
                                  controller: itemDescCtrl,
                                  decoration: const InputDecoration(
                                      labelText: 'Bezeichnung/Notiz'))),
                        ])
                      ]),
                ),
              ),
              actions: [
                TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: const Text('Abbrechen')),
                FilledButton.icon(
                    onPressed: () async {
                      try {
                        final body = {
                          'lieferant_id': supplierId,
                          'nummer': numberCtrl.text.trim(),
                          'waehrung': currencyCtrl.text.trim().isEmpty
                              ? 'EUR'
                              : currencyCtrl.text.trim().toUpperCase(),
                          'status': status,
                          'notiz': noteCtrl.text.trim(),
                          'positionen': [
                            if (itemMaterialId != null)
                              {
                                'material_id': itemMaterialId,
                                'bezeichnung': itemDescCtrl.text.trim(),
                                'menge':
                                    double.tryParse(itemQtyCtrl.text.trim()) ??
                                        0,
                                'einheit': itemUomCtrl.text.trim(),
                                'preis': double.tryParse(
                                        itemPriceCtrl.text.trim()) ??
                                    0,
                                'waehrung': currencyCtrl.text.trim().isEmpty
                                    ? 'EUR'
                                    : currencyCtrl.text.trim().toUpperCase(),
                              }
                          ]
                        };
                        await widget.api.createPurchaseOrder(body);
                        if (mounted) Navigator.of(ctx).pop();
                        await _reload();
                      } catch (e) {
                        if (mounted) {
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(content: Text(_errorMessage(e))),
                          );
                        }
                      }
                    },
                    icon: const Icon(Icons.check),
                    label: const Text('Anlegen')),
              ],
            ));
  }

  @override
  Widget build(BuildContext context) {
    final canWrite = widget.api.hasPermission('purchase_orders.write');
    return Scaffold(
      floatingActionButtonLocation: FloatingActionButtonLocation.startFloat,
      floatingActionButton: canWrite
          ? FloatingActionButton(
              onPressed: _openCreateDialog, child: const Icon(Icons.add))
          : null,
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(12),
            child: Row(children: [
              const Text('Bestellungen',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              const Spacer(),
              SizedBox(
                width: 260,
                child: TextField(
                  controller: searchCtrl,
                  decoration: InputDecoration(
                      isDense: true,
                      prefixIcon: const Icon(Icons.search),
                      hintText: 'Suchen (Nummer)',
                      suffixIcon: IconButton(
                          icon: const Icon(Icons.clear),
                          onPressed: () {
                            searchCtrl.clear();
                            _reload();
                          })),
                  onSubmitted: (_) => _reload(),
                ),
              ),
              const SizedBox(width: 8),
              SizedBox(
                width: 180,
                child: InputDecorator(
                  decoration:
                      const InputDecoration(isDense: true, labelText: 'Status'),
                  child: DropdownButton<String?>(
                    isExpanded: true,
                    value: filterStatus,
                    hint: const Text('Alle'),
                    items: [
                      const DropdownMenuItem<String?>(
                          value: null, child: Text('Alle')),
                      for (final s in (statuses.isEmpty
                          ? ['draft', 'ordered', 'received', 'canceled']
                          : statuses))
                        DropdownMenuItem<String?>(value: s, child: Text(s))
                    ],
                    onChanged: (v) {
                      setState(() => filterStatus = v);
                      _reload();
                    },
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
                    title:
                        Text('${po['nummer'] ?? ''}  •  ${po['status'] ?? ''}'),
                    subtitle: Text(
                        'Datum: ${po['datum'] ?? ''}  •  Währung: ${po['waehrung'] ?? 'EUR'}'),
                    onTap: () {
                      final id = (po['id'] ?? '').toString();
                      if (id.isNotEmpty) {
                        _openDetail(id);
                      }
                    },
                  );
                }
                final canLoadMore =
                    items.isNotEmpty && items.length % limit == 0;
                if (!canLoadMore) return const SizedBox.shrink();
                return Padding(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  child: Center(
                      child: FilledButton.icon(
                          onPressed: _loadMore,
                          icon: const Icon(Icons.expand_more),
                          label: const Text('Mehr laden'))),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
