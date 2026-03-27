import 'package:flutter/material.dart';
import '../api.dart';
import '../material_selection.dart';
import '../purchase_order_item_payloads.dart';
import '../purchase_order_material_selection.dart';
import '../purchase_order_receipt_flow.dart';

String _purchaseOrderErrorMessage(Object error,
    {String fallback = 'Vorgang fehlgeschlagen'}) {
  if (error is ApiException) {
    switch (error.code) {
      case 'validation_error':
        return error.message;
      case 'not_found':
        return 'Bestellung oder Position nicht gefunden oder nicht mehr verfügbar.';
      case 'internal_error':
        return 'Serverfehler. Bitte erneut versuchen.';
    }
    return error.message;
  }
  return '$fallback: $error';
}

String _contactCommercialSummary(Map<String, dynamic> contact,
    {bool supplier = false}) {
  final parts = <String>[];
  final paymentTerms = (contact['zahlungsbedingungen'] ?? '').toString().trim();
  final reference = supplier
      ? (contact['kreditor_nr'] ?? '').toString().trim()
      : (contact['debitor_nr'] ?? '').toString().trim();
  final taxCountry = (contact['steuer_land'] ?? '').toString().trim();
  final taxExempt = contact['steuerbefreit'] == true;
  if (reference.isNotEmpty) {
    parts.add('${supplier ? 'Kreditor-Nr.' : 'Debitor-Nr.'}: $reference');
  }
  if (paymentTerms.isNotEmpty) parts.add('Zahlungsbedingungen: $paymentTerms');
  if (taxCountry.isNotEmpty) parts.add('Steuerland: $taxCountry');
  if (taxExempt) parts.add('Steuerbefreit');
  return parts.join(' • ');
}

class PurchaseOrderDetailPage extends StatefulWidget {
  const PurchaseOrderDetailPage(
      {super.key, required this.api, required this.id});
  final ApiClient api;
  final String id;

  @override
  State<PurchaseOrderDetailPage> createState() =>
      _PurchaseOrderDetailPageState();
}

class _PurchaseOrderDetailPageState extends State<PurchaseOrderDetailPage> {
  Map<String, dynamic>? po;
  List<dynamic> items = [];
  Map<String, dynamic>? supplier;
  bool loading = false;
  // Edit header
  final noteCtrl = TextEditingController();
  String? status;
  String currency = 'EUR';
  List<String> statuses = [];
  // Item edit state
  List<dynamic> materials = [];
  // Receiving
  List<dynamic> warehouses = [];
  List<dynamic> locations = [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => loading = true);
    try {
      final resp = await widget.api.getPurchaseOrder(widget.id);
      final order = (resp['bestellung'] ?? {}) as Map<String, dynamic>;
      final pos = (resp['positionen'] ?? []) as List<dynamic>;
      Map<String, dynamic>? supp;
      final sid = (order['lieferant_id'] ?? '').toString();
      if (sid.isNotEmpty) {
        try {
          supp = await widget.api.getContact(sid);
        } catch (_) {}
      }
      try {
        statuses = await widget.api.listPOStatuses();
      } catch (_) {
        statuses = ['draft', 'ordered', 'received', 'canceled'];
      }
      noteCtrl.text = (order['notiz'] ?? '').toString();
      status = (order['status'] ?? 'draft').toString();
      currency = (order['waehrung'] ?? 'EUR').toString();
      // preload materials for quick item edit (limit 200)
      try {
        materials = await widget.api.listMaterials(limit: 200);
      } catch (_) {
        materials = [];
      }
      try {
        warehouses = await widget.api.listWarehouses();
      } catch (_) {
        warehouses = [];
      }
      setState(() {
        po = order;
        items = pos;
        supplier = supp;
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            content: Text(_purchaseOrderErrorMessage(e,
                fallback: 'Laden fehlgeschlagen'))));
      }
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> _editHeader() async {
    final curCtrl = TextEditingController(text: currency);
    String? st = status;
    final note = TextEditingController(text: noteCtrl.text);
    await showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
              title: const Text('Bestellung bearbeiten'),
              content: SizedBox(
                width: 500,
                child: SingleChildScrollView(
                  child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        DropdownButtonFormField<String>(
                            initialValue: st,
                            items: [
                              for (final s in (statuses.isEmpty
                                  ? ['draft', 'ordered', 'received', 'canceled']
                                  : statuses))
                                DropdownMenuItem(value: s, child: Text(s))
                            ],
                            onChanged: (v) => st = v,
                            decoration:
                                const InputDecoration(labelText: 'Status')),
                        TextFormField(
                            controller: curCtrl,
                            decoration:
                                const InputDecoration(labelText: 'Währung')),
                        TextFormField(
                            controller: note,
                            decoration:
                                const InputDecoration(labelText: 'Notiz')),
                      ]),
                ),
              ),
              actions: [
                TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: const Text('Abbrechen')),
                FilledButton(
                    onPressed: () async {
                      try {
                        await widget.api.updatePurchaseOrder(widget.id, {
                          'status': st,
                          'waehrung': curCtrl.text.trim().isEmpty
                              ? 'EUR'
                              : curCtrl.text.trim().toUpperCase(),
                          'notiz': note.text.trim()
                        });
                        await _load();
                        if (mounted) Navigator.of(ctx).pop();
                      } catch (e) {
                        if (mounted) {
                          ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                              content: Text(_purchaseOrderErrorMessage(e))));
                        }
                      }
                    },
                    child: const Text('Speichern')),
              ],
            ));
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    final order = po;
    return Scaffold(
      appBar: AppBar(
        backgroundColor: color,
        foregroundColor: Colors.white,
        title: Text(order == null
            ? 'Bestellung'
            : 'Bestellung ${order['nummer'] ?? ''}'),
        actions: [
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh)),
          IconButton(
              onPressed: _receiveDialog,
              icon: const Icon(Icons.inventory_2_outlined),
              tooltip: 'Empfangen'),
          IconButton(
              onPressed: () {
                widget.api.downloadPurchaseOrderPdf(widget.id);
              },
              icon: const Icon(Icons.picture_as_pdf),
              tooltip: 'PDF'),
          IconButton(onPressed: _editHeader, icon: const Icon(Icons.edit_note)),
        ],
      ),
      body: loading && order == null
          ? const Center(child: CircularProgressIndicator())
          : Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (order != null) ...[
                    Wrap(
                        spacing: 12,
                        runSpacing: 8,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          Chip(
                              label: Text(
                                  (order['status'] ?? 'draft').toString())),
                          Text('Datum: ${(order['datum'] ?? '').toString()}'),
                          Text(
                              'Währung: ${(order['waehrung'] ?? 'EUR').toString()}'),
                          if (supplier != null)
                            Text(
                                'Lieferant: ${(supplier!['name'] ?? '').toString()}'),
                        ]),
                    if (supplier != null &&
                        _contactCommercialSummary(supplier!, supplier: true)
                            .isNotEmpty) ...[
                      const SizedBox(height: 6),
                      Text(
                        _contactCommercialSummary(supplier!, supplier: true),
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                    const Divider(),
                  ],
                  const Text('Positionen',
                      style:
                          TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                  const SizedBox(height: 6),
                  Align(
                    alignment: Alignment.centerLeft,
                    child: FilledButton.icon(
                        onPressed: _addItem,
                        icon: const Icon(Icons.add),
                        label: const Text('Position hinzufügen')),
                  ),
                  const SizedBox(height: 6),
                  Expanded(
                    child: ListView.separated(
                      itemCount: items.length,
                      separatorBuilder: (_, __) => const Divider(height: 1),
                      itemBuilder: (ctx, i) {
                        final it = items[i] as Map<String, dynamic>;
                        final title = (it['bezeichnung'] ?? '').toString();
                        final qty = (it['menge'] ?? 0).toString();
                        final uom = (it['einheit'] ?? '').toString();
                        final price = (it['preis'] ?? 0).toString();
                        final cur = (it['waehrung'] ?? '').toString();
                        return ListTile(
                          dense: true,
                          leading: CircleAvatar(
                              child: Text('${it['position'] ?? ''}')),
                          title: Text(title.isEmpty
                              ? resolveMaterialLabel(
                                  materials,
                                  it['material_id']?.toString(),
                                  fallback:
                                      (it['material_id'] ?? '').toString(),
                                )
                              : title),
                          subtitle:
                              Text('Menge: $qty $uom  •  Preis: $price $cur'),
                          trailing:
                              Row(mainAxisSize: MainAxisSize.min, children: [
                            IconButton(
                                icon: const Icon(Icons.edit),
                                onPressed: () => _editItem(it)),
                            IconButton(
                                icon: const Icon(Icons.delete_outline),
                                onPressed: () => _deleteItem(it)),
                          ]),
                        );
                      },
                    ),
                  ),
                ],
              ),
            ),
    );
  }

  Future<void> _receiveDialog() async {
    String? whId;
    String? locId;
    bool setReceived = true;
    await showDialog(
        context: context,
        builder: (ctx) => StatefulBuilder(builder: (ctx, setStateDlg) {
              return AlertDialog(
                title: const Text('Wareneingang buchen'),
                content: SizedBox(
                  width: 520,
                  child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text(
                            'Ziel-Lager und -Platz wählen. Es werden alle Positionen gebucht.'),
                        const SizedBox(height: 8),
                        DropdownButtonFormField<String>(
                          initialValue: whId,
                          items: [
                            for (final w in warehouses)
                              DropdownMenuItem(
                                  value: w['id'] as String,
                                  child: Text('${w['code']} – ${w['name']}'))
                          ],
                          onChanged: (v) async {
                            setStateDlg(() => whId = v);
                            if (v != null) {
                              try {
                                locations = await widget.api.listLocations(v);
                                setState(() => {});
                                setStateDlg(() => {});
                              } catch (_) {
                                locations = [];
                              }
                            }
                          },
                          decoration: const InputDecoration(labelText: 'Lager'),
                        ),
                        const SizedBox(height: 8),
                        DropdownButtonFormField<String>(
                          initialValue: locId,
                          items: [
                            for (final l in locations)
                              DropdownMenuItem(
                                  value: l['id'] as String,
                                  child: Text('${l['code']} – ${l['name']}'))
                          ],
                          onChanged: (v) {
                            setStateDlg(() => locId = v);
                          },
                          decoration: const InputDecoration(
                              labelText: 'Lagerplatz (optional)'),
                        ),
                        Row(children: [
                          Checkbox(
                              value: setReceived,
                              onChanged: (v) =>
                                  setStateDlg(() => setReceived = v ?? true)),
                          const Expanded(
                            child: Text('Bestellstatus auf "received" setzen'),
                          ),
                        ])
                      ]),
                ),
                actions: [
                  TextButton(
                      onPressed: () => Navigator.of(ctx).pop(),
                      child: const Text('Abbrechen')),
                  FilledButton.icon(
                      onPressed: () async {
                        try {
                          await _receiveNow(whId, locId, setReceived);
                          if (mounted) Navigator.of(ctx).pop();
                        } catch (e) {
                          if (mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                                content: Text(_purchaseOrderErrorMessage(e))));
                          }
                        }
                      },
                      icon: const Icon(Icons.check),
                      label: const Text('Buchen')),
                ],
              );
            }));
  }

  Future<void> _receiveNow(
      String? whId, String? locId, bool setReceived) async {
    if (po == null) return;
    if (whId == null || whId.isEmpty) {
      throw Exception('Lager wählen');
    }
    final bodies = buildPurchaseOrderReceiptStockMovements(
      order: po!,
      items: items,
      warehouseId: whId,
      locationId: locId,
      defaultCurrency: currency,
    );
    if (bodies.isEmpty) {
      throw Exception(
          'Keine materialgebundenen Positionen zum Buchen vorhanden');
    }
    for (final body in bodies) {
      await widget.api.createStockMovement(body);
    }
    if (setReceived) {
      try {
        await widget.api.updatePurchaseOrder(widget.id, {'status': 'received'});
      } catch (_) {}
    }
    await _load();
    if (mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Wareneingang gebucht')));
    }
  }

  Future<void> _addItem() async {
    String? materialId;
    final desc = TextEditingController();
    final qty = TextEditingController(text: '1');
    final uom = TextEditingController(text: 'Stk');
    final price = TextEditingController(text: '0');
    final cur = TextEditingController(text: currency);
    DateTime? deliv;
    await showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
              title: const Text('Position hinzufügen'),
              content: SizedBox(
                width: 600,
                child: SingleChildScrollView(
                  child: Wrap(spacing: 12, runSpacing: 12, children: [
                    SizedBox(
                      width: 320,
                      child: DropdownButtonFormField<String>(
                        initialValue: materialId,
                        items: [
                          for (final m in materials)
                            DropdownMenuItem(
                                value: m['id'] as String,
                                child: Text(materialSelectionLabel(
                                    m as Map<String, dynamic>)))
                        ],
                        onChanged: (v) {
                          materialId = v;
                          applyPurchaseOrderMaterialSelection(
                            materials: materials,
                            materialId: materialId,
                            descriptionController: desc,
                            unitController: uom,
                          );
                        },
                        decoration:
                            const InputDecoration(labelText: 'Material'),
                      ),
                    ),
                    SizedBox(
                        width: 120,
                        child: TextFormField(
                            controller: qty,
                            decoration:
                                const InputDecoration(labelText: 'Menge'))),
                    SizedBox(
                        width: 120,
                        child: TextFormField(
                            controller: uom,
                            decoration:
                                const InputDecoration(labelText: 'Einheit'))),
                    SizedBox(
                        width: 160,
                        child: TextFormField(
                            controller: price,
                            decoration:
                                const InputDecoration(labelText: 'Preis'))),
                    SizedBox(
                        width: 320,
                        child: TextFormField(
                            controller: desc,
                            decoration: const InputDecoration(
                                labelText: 'Bezeichnung/Notiz'))),
                    Row(children: [
                      const Text('Liefertermin:'),
                      const SizedBox(width: 8),
                      FilledButton.tonal(
                          onPressed: () async {
                            final now = DateTime.now();
                            final picked = await showDatePicker(
                                context: context,
                                firstDate: DateTime(now.year - 1),
                                lastDate: DateTime(now.year + 5),
                                initialDate: deliv ?? now);
                            if (picked != null) {
                              setState(() => deliv = picked);
                            }
                          },
                          child: Text(deliv == null
                              ? 'Datum wählen'
                              : deliv!.toIso8601String().substring(0, 10))),
                    ])
                  ]),
                ),
              ),
              actions: [
                TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: const Text('Abbrechen')),
                FilledButton(
                    onPressed: () async {
                      try {
                        await widget.api.createPurchaseOrderItem(
                          widget.id,
                          buildPurchaseOrderItemPayload(
                            materialId: materialId,
                            description: desc.text,
                            quantityText: qty.text,
                            unitText: uom.text,
                            priceText: price.text,
                            currencyText: cur.text,
                            deliveryDate: deliv,
                          ),
                        );
                        if (mounted) Navigator.of(ctx).pop();
                        await _load();
                      } catch (e) {
                        if (mounted) {
                          ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                              content: Text(_purchaseOrderErrorMessage(e))));
                        }
                      }
                    },
                    child: const Text('Hinzufügen')),
              ],
            ));
  }

  Future<void> _editItem(Map<String, dynamic> it) async {
    final desc =
        TextEditingController(text: (it['bezeichnung'] ?? '').toString());
    final qty = TextEditingController(text: (it['menge'] ?? 0).toString());
    final uom = TextEditingController(text: (it['einheit'] ?? '').toString());
    final price = TextEditingController(text: (it['preis'] ?? 0).toString());
    final cur =
        TextEditingController(text: (it['waehrung'] ?? currency).toString());
    DateTime? deliv;
    final existing = (it['liefertermin'] ?? '').toString();
    if (existing.isNotEmpty) {
      try {
        deliv = DateTime.parse(existing);
      } catch (_) {}
    }
    await showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
              title: const Text('Position bearbeiten'),
              content: SizedBox(
                width: 600,
                child: SingleChildScrollView(
                  child: Wrap(spacing: 12, runSpacing: 12, children: [
                    SizedBox(
                        width: 320,
                        child: TextFormField(
                            controller: desc,
                            decoration: const InputDecoration(
                                labelText: 'Bezeichnung/Notiz'))),
                    SizedBox(
                        width: 120,
                        child: TextFormField(
                            controller: qty,
                            decoration:
                                const InputDecoration(labelText: 'Menge'))),
                    SizedBox(
                        width: 120,
                        child: TextFormField(
                            controller: uom,
                            decoration:
                                const InputDecoration(labelText: 'Einheit'))),
                    SizedBox(
                        width: 160,
                        child: TextFormField(
                            controller: price,
                            decoration:
                                const InputDecoration(labelText: 'Preis'))),
                    SizedBox(
                        width: 120,
                        child: TextFormField(
                            controller: cur,
                            decoration:
                                const InputDecoration(labelText: 'Währung'))),
                    Row(children: [
                      const Text('Liefertermin:'),
                      const SizedBox(width: 8),
                      FilledButton.tonal(
                          onPressed: () async {
                            final now = DateTime.now();
                            final picked = await showDatePicker(
                                context: context,
                                firstDate: DateTime(now.year - 1),
                                lastDate: DateTime(now.year + 5),
                                initialDate: deliv ?? now);
                            if (picked != null) {
                              setState(() => deliv = picked);
                            }
                          },
                          child: Text(deliv == null
                              ? 'Datum wählen'
                              : deliv!.toIso8601String().substring(0, 10))),
                    ])
                  ]),
                ),
              ),
              actions: [
                TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: const Text('Abbrechen')),
                FilledButton(
                    onPressed: () async {
                      try {
                        await widget.api.updatePurchaseOrderItem(
                          widget.id,
                          (it['id'] as String),
                          buildPurchaseOrderItemPayload(
                            description: desc.text,
                            quantityText: qty.text,
                            unitText: uom.text,
                            priceText: price.text,
                            currencyText: cur.text,
                            deliveryDate: deliv,
                            includeMaterialId: false,
                          ),
                        );
                        if (mounted) Navigator.of(ctx).pop();
                        await _load();
                      } catch (e) {
                        if (mounted) {
                          ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                              content: Text(_purchaseOrderErrorMessage(e))));
                        }
                      }
                    },
                    child: const Text('Speichern')),
              ],
            ));
  }

  Future<void> _deleteItem(Map<String, dynamic> it) async {
    final ok = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
                title: const Text('Position löschen'),
                content: const Text('Position wirklich löschen?'),
                actions: [
                  TextButton(
                      onPressed: () => Navigator.of(ctx).pop(false),
                      child: const Text('Abbrechen')),
                  FilledButton(
                      onPressed: () => Navigator.of(ctx).pop(true),
                      child: const Text('Löschen'))
                ]));
    if (ok == true) {
      try {
        await widget.api
            .deletePurchaseOrderItem(widget.id, (it['id'] as String));
        await _load();
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text(_purchaseOrderErrorMessage(e))));
        }
      }
    }
  }
}
