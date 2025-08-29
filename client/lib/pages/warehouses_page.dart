import 'package:flutter/material.dart';
import '../api.dart';

class WarehousesPage extends StatefulWidget {
  const WarehousesPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<WarehousesPage> createState() => _WarehousesPageState();
}

class _WarehousesPageState extends State<WarehousesPage> {
  List<dynamic> warehouses = [];
  List<dynamic> locations = [];
  String? selectedWarehouseId;
  final whCodeCtrl = TextEditingController();
  final whNameCtrl = TextEditingController();
  final locCodeCtrl = TextEditingController();
  final locNameCtrl = TextEditingController();
  final _whFormKey = GlobalKey<FormState>();
  final _locFormKey = GlobalKey<FormState>();

  @override
  void initState() {
    super.initState();
    _reloadWarehouses();
  }

  Future<void> _reloadWarehouses() async {
    try {
      warehouses = await widget.api.listWarehouses();
      setState(() {});
    } catch (e) {
      if (mounted) {
        debugPrint('Fehler beim Laden Lager: $e');
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Lager konnten nicht geladen werden: $e')));
      }
    }
  }

  Future<void> _selectWarehouse(String id) async {
    selectedWarehouseId = id;
    locations = await widget.api.listLocations(id);
    setState(() {});
  }

  Future<void> _createWarehouse() async {
    if (!_whFormKey.currentState!.validate()) return;
    try {
      await widget.api.createWarehouse({'code': whCodeCtrl.text.trim(), 'name': whNameCtrl.text.trim()});
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Lager angelegt')));
      }
      whCodeCtrl.clear(); whNameCtrl.clear();
      await _reloadWarehouses();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _createLocation() async {
    if (selectedWarehouseId == null) return;
    if (!_locFormKey.currentState!.validate()) return;
    try {
      await widget.api.createLocation(selectedWarehouseId!, {'code': locCodeCtrl.text.trim(), 'name': locNameCtrl.text.trim()});
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Lagerplatz angelegt')));
      }
      locCodeCtrl.clear(); locNameCtrl.clear();
      locations = await widget.api.listLocations(selectedWarehouseId!);
      setState(() {});
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  String _warehouseNameById(String? id) {
    if (id == null) return '-';
    for (final w in warehouses) {
      final m = w as Map<String, dynamic>;
      if (m['id'] == id) {
        final name = (m['name'] ?? '').toString();
        if (name.isNotEmpty) return name;
        final code = (m['code'] ?? '').toString();
        if (code.isNotEmpty) return code;
        return id;
      }
    }
    return id;
  }

  Future<void> _openCreateDialog() async {
    bool createLocation = selectedWarehouseId != null;
    await showDialog(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDlg) => AlertDialog(
          title: const Text('Neu anlegen'),
          content: SizedBox(
            width: 520,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Wrap(spacing: 8, runSpacing: 8, children: [
                  ChoiceChip(
                    label: const Text('Lager'),
                    selected: !createLocation,
                    onSelected: (_) => setStateDlg(() => createLocation = false),
                  ),
                  ChoiceChip(
                    label: const Text('Lagerplatz'),
                    selected: createLocation,
                    onSelected: (_) => setStateDlg(() => createLocation = true),
                  ),
                ]),
                const SizedBox(height: 12),
                if (!createLocation)
                  Form(
                    key: _whFormKey,
                    child: Wrap(spacing: 8, runSpacing: 8, children: [
                      SizedBox(width: 160, child: TextFormField(controller: whCodeCtrl, decoration: const InputDecoration(labelText: 'Code'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null)),
                      SizedBox(width: 240, child: TextFormField(controller: whNameCtrl, decoration: const InputDecoration(labelText: 'Name'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null)),
                    ]),
                  ),
                if (createLocation)
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Für Lager: ${_warehouseNameById(selectedWarehouseId)}'),
                      const SizedBox(height: 8),
                      Form(
                        key: _locFormKey,
                        child: Wrap(spacing: 8, runSpacing: 8, children: [
                          SizedBox(width: 160, child: TextFormField(controller: locCodeCtrl, decoration: const InputDecoration(labelText: 'Code'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null)),
                          SizedBox(width: 240, child: TextFormField(controller: locNameCtrl, decoration: const InputDecoration(labelText: 'Name'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null)),
                        ]),
                      ),
                    ],
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: ()=> Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton.icon(
              onPressed: () async {
                if (!createLocation) {
                  await _createWarehouse();
                } else {
                  await _createLocation();
                }
                if (mounted) Navigator.of(ctx).pop();
              },
              icon: const Icon(Icons.check), label: const Text('Speichern'),
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      floatingActionButtonLocation: FloatingActionButtonLocation.startFloat,
      floatingActionButton: FloatingActionButton(
        onPressed: _openCreateDialog,
        child: const Icon(Icons.add),
      ),
      body: Row(
        children: [
          Expanded(
            child: Column(
              children: [
                Padding(
                  padding: const EdgeInsets.all(12),
                  child: Row(children: [
                    const Text('Lager', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                    const Spacer(),
                    IconButton(onPressed: _reloadWarehouses, icon: const Icon(Icons.refresh)),
                  ]),
                ),
                Expanded(
                  child: ListView.builder(
                    itemCount: warehouses.length,
                    itemBuilder: (ctx, i){
                      final w = warehouses[i] as Map<String, dynamic>;
                      final sel = w['id'] == selectedWarehouseId;
                      return ListTile(
                        selected: sel,
                        title: Text('${w['code']} – ${w['name']}'),
                        onTap: () => _selectWarehouse(w['id'] as String),
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
          const VerticalDivider(width: 1),
          Expanded(
            child: Column(
              children: [
                const SizedBox(height: 12),
                const Text('Lagerplätze', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                const SizedBox(height: 8),
                Expanded(
                  child: ListView.builder(
                    itemCount: locations.length,
                    itemBuilder: (ctx, i){
                      final l = locations[i] as Map<String, dynamic>;
                      return ListTile(
                        dense: true,
                        title: Text('${l['code']} – ${l['name']}'),
                        subtitle: Text('Lager: ${_warehouseNameById(l['warehouse_id'] as String?)}'),
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
