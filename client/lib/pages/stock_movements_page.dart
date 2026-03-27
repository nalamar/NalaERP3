import 'package:flutter/material.dart';
import '../api.dart';
import '../commercial_navigation.dart';
import '../material_selection.dart';
import '../stock_movement_payloads.dart';

class StockMovementsPage extends StatefulWidget {
  const StockMovementsPage({
    super.key,
    required this.api,
    this.initialPrefill,
    this.openCreateOnStart = false,
  });
  final ApiClient api;
  final StockMovementPrefillContext? initialPrefill;
  final bool openCreateOnStart;

  @override
  State<StockMovementsPage> createState() => _StockMovementsPageState();
}

class _StockMovementsPageState extends State<StockMovementsPage> {
  List<dynamic> materials = [];
  List<dynamic> warehouses = [];
  List<dynamic> locations = [];
  String? materialId;
  String? warehouseId;
  String? locationId;
  final batchCtrl = TextEditingController();
  final qtyCtrl = TextEditingController(text: '0');
  final uomCtrl = TextEditingController(text: 'kg');
  final typeCtrl = TextEditingController(text: 'in');
  final reasonCtrl = TextEditingController();
  final refCtrl = TextEditingController();
  final priceCtrl = TextEditingController();
  final currCtrl = TextEditingController(text: 'EUR');
  final _formKey = GlobalKey<FormState>();
  final List<String> _types = const [
    'purchase',
    'in',
    'out',
    'transfer',
    'adjust'
  ];
  bool _initialDialogHandled = false;

  @override
  void initState() {
    super.initState();
    final prefill = widget.initialPrefill;
    materialId = prefill?.normalizedMaterialId;
    warehouseId = prefill?.normalizedWarehouseId;
    locationId = prefill?.normalizedLocationId;
    final initialType = prefill?.normalizedType;
    if (initialType != null) {
      typeCtrl.text = initialType;
    }
    final initialReason = prefill?.normalizedReason;
    if (initialReason != null) {
      reasonCtrl.text = initialReason;
    }
    final initialReference = prefill?.normalizedReference;
    if (initialReference != null) {
      refCtrl.text = initialReference;
    }
    _loadData();
  }

  Future<void> _loadData() async {
    try {
      materials = await widget.api.listMaterials();
      warehouses = await widget.api.listWarehouses();
      final initialWarehouseId = widget.initialPrefill?.normalizedWarehouseId;
      if (initialWarehouseId != null) {
        locations = await widget.api.listLocations(initialWarehouseId);
      }
      applyMaterialSelection(
        materials: materials,
        materialId: materialId,
        unitController: uomCtrl,
      );
      setState(() {});
      if (widget.openCreateOnStart &&
          !_initialDialogHandled &&
          widget.api.hasPermission('stock_movements.write')) {
        _initialDialogHandled = true;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          _openDialog();
        });
      }
    } catch (e) {
      if (mounted) {
        debugPrint('Fehler beim Initial-Laden: $e');
        ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Daten konnten nicht geladen werden: $e')));
      }
    }
  }

  Future<void> _onWarehouseChanged(String? id) async {
    warehouseId = id;
    locationId = null;
    locations = id != null ? await widget.api.listLocations(id) : [];
    setState(() {});
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    final qty = double.tryParse(qtyCtrl.text.trim()) ?? 0;
    final body = buildStockMovementPayload(
      materialId: materialId ?? '',
      warehouseId: warehouseId ?? '',
      locationId: locationId,
      batchCode: batchCtrl.text,
      quantity: qty,
      unit: uomCtrl.text,
      type: typeCtrl.text,
      reason: reasonCtrl.text,
      reference: refCtrl.text,
      purchasePrice: priceCtrl.text.trim().isEmpty
          ? null
          : double.tryParse(priceCtrl.text.trim()),
      currency: currCtrl.text,
    );
    try {
      await widget.api.createStockMovement(body);
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Bewegung erfasst')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _openDialog() async {
    await showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Bestandsbewegung'),
        content: SizedBox(
          width: 680,
          child: Form(
            key: _formKey,
            child: SingleChildScrollView(
              child: Wrap(spacing: 12, runSpacing: 12, children: [
                SizedBox(
                  width: 280,
                  child: DropdownButtonFormField<String>(
                    initialValue: materialId,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Material'),
                    items: [
                      for (final m in materials)
                        DropdownMenuItem(
                            value: m['id'] as String,
                            child: Text(materialSelectionLabel(
                                m as Map<String, dynamic>)))
                    ],
                    validator: (v) => (v == null || v.isEmpty)
                        ? 'Bitte Material wählen'
                        : null,
                    onChanged: (v) => setState(() {
                      materialId = v;
                      applyMaterialSelection(
                        materials: materials,
                        materialId: materialId,
                        unitController: uomCtrl,
                      );
                    }),
                  ),
                ),
                SizedBox(
                  width: 240,
                  child: DropdownButtonFormField<String>(
                    initialValue: warehouseId,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Lager'),
                    items: [
                      for (final w in warehouses)
                        DropdownMenuItem(
                            value: w['id'] as String,
                            child: Text('${w['code']} – ${w['name']}'))
                    ],
                    validator: (v) =>
                        (v == null || v.isEmpty) ? 'Bitte Lager wählen' : null,
                    onChanged: (v) => _onWarehouseChanged(v),
                  ),
                ),
                SizedBox(
                  width: 220,
                  child: DropdownButtonFormField<String>(
                    initialValue: locationId,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Lagerplatz'),
                    hint: const Text('Optional'),
                    items: [
                      for (final l in locations)
                        DropdownMenuItem(
                            value: l['id'] as String,
                            child: Text('${l['code']} – ${l['name']}'))
                    ],
                    onChanged: (v) => setState(() => locationId = v),
                  ),
                ),
                SizedBox(
                    width: 160,
                    child: TextFormField(
                        controller: batchCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Chargencode'))),
                SizedBox(
                    width: 120,
                    child: TextFormField(
                        controller: qtyCtrl,
                        decoration: const InputDecoration(labelText: 'Menge'),
                        validator: (v) {
                          final d = double.tryParse((v ?? '').trim());
                          if (d == null) return 'Zahl erforderlich';
                          if (d == 0) return '≠ 0 erwartet';
                          return null;
                        })),
                SizedBox(
                    width: 120,
                    child: TextFormField(
                        controller: uomCtrl,
                        decoration: const InputDecoration(labelText: 'Einheit'),
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? 'Pflichtfeld'
                            : null)),
                SizedBox(
                  width: 200,
                  child: DropdownButtonFormField<String>(
                    initialValue: _types.contains(typeCtrl.text.trim())
                        ? typeCtrl.text.trim()
                        : null,
                    isExpanded: true,
                    items: [
                      for (final t in _types)
                        DropdownMenuItem(value: t, child: Text(t))
                    ],
                    decoration: const InputDecoration(labelText: 'Typ'),
                    validator: (v) =>
                        (v == null || v.isEmpty) ? 'Bitte Typ wählen' : null,
                    onChanged: (v) {
                      setState(() => typeCtrl.text = v ?? '');
                    },
                  ),
                ),
                SizedBox(
                    width: 220,
                    child: TextField(
                        controller: reasonCtrl,
                        decoration: const InputDecoration(labelText: 'Grund'))),
                SizedBox(
                    width: 220,
                    child: TextField(
                        controller: refCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Referenz'))),
                SizedBox(
                    width: 160,
                    child: TextFormField(
                        controller: priceCtrl,
                        decoration: const InputDecoration(
                            labelText: 'EK-Preis (nur purchase)'),
                        validator: (v) {
                          final typ = typeCtrl.text.trim();
                          if (typ == 'purchase') {
                            if (v == null || v.trim().isEmpty)
                              return 'Preis erforderlich';
                            final p = double.tryParse(v.trim());
                            if (p == null) return 'Zahl erforderlich';
                            if (p < 0) return '≥ 0 erwartet';
                          } else if (v != null && v.trim().isNotEmpty) {
                            final p = double.tryParse(v.trim());
                            if (p == null) return 'Zahl erforderlich';
                          }
                          return null;
                        })),
                SizedBox(
                    width: 120,
                    child: TextFormField(
                        controller: currCtrl,
                        decoration: const InputDecoration(labelText: 'Währung'),
                        validator: (v) {
                          final need = typeCtrl.text.trim() == 'purchase';
                          if (!need && (priceCtrl.text.trim().isEmpty))
                            return null;
                          if (v == null || v.trim().isEmpty)
                            return 'Pflichtfeld';
                          final t = v.trim().toUpperCase();
                          if (t.length != 3) return '3 Buchstaben';
                          currCtrl.text = t;
                          return null;
                        })),
              ]),
            ),
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Abbrechen')),
          FilledButton.icon(
              onPressed: () async {
                await _submit();
                if (mounted) Navigator.of(ctx).pop();
              },
              icon: const Icon(Icons.check),
              label: const Text('Speichern')),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final canWrite = widget.api.hasPermission('stock_movements.write');
    return Scaffold(
      floatingActionButtonLocation: FloatingActionButtonLocation.startFloat,
      floatingActionButton: canWrite
          ? FloatingActionButton(
              onPressed: _openDialog, child: const Icon(Icons.add))
          : null,
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Bestandsbewegungen',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
            const SizedBox(height: 8),
            Text(
              canWrite
                  ? 'Neue Bewegung über den + Button unten links erfassen.'
                  : 'Für diesen Benutzer ist nur die Ansicht freigeschaltet.',
            ),
          ],
        ),
      ),
    );
  }
}
