import 'dart:html' as html;
import 'dart:typed_data';

import 'package:flutter/material.dart';
import '../api.dart';

class MaterialsPage extends StatefulWidget {
  const MaterialsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<MaterialsPage> createState() => _MaterialsPageState();
}

class _MaterialsPageState extends State<MaterialsPage> {
  List<dynamic> items = [];
  bool loading = false;
  String? selectedId;
  List<dynamic> stock = [];
  List<dynamic> docs = [];
  List<dynamic> warehouses = [];

  final formKey = GlobalKey<FormState>();
  final nummerCtrl = TextEditingController();
  final bezCtrl = TextEditingController();
  final typCtrl = TextEditingController(text: 'rohstoff');
  final normCtrl = TextEditingController();
  final wnrCtrl = TextEditingController();
  final einheitCtrl = TextEditingController(text: 'kg');
  final dichteCtrl = TextEditingController(text: '0');
  final katCtrl = TextEditingController(text: '');

  @override
  void initState() {
    super.initState();
    _reload();
  }

  Future<void> _reload() async {
    setState(() => loading = true);
    try {
      items = await widget.api.listMaterials();
    } catch (e) {
      if (mounted) {
        debugPrint('Fehler beim Laden Materialien: $e');
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Materialien konnten nicht geladen werden: $e')));
      }
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> _select(String id) async {
    setState(() => selectedId = id);
    stock = await widget.api.stockByMaterial(id);
    // Lade Lagerliste, um Namen/Codes statt IDs anzeigen zu können
    try { warehouses = await widget.api.listWarehouses(); } catch (_) {}
    docs = await widget.api.listMaterialDocuments(id);
    setState(() {});
  }

  String _warehouseLabel(String id){
    for (final w in warehouses){
      final m = w as Map<String, dynamic>;
      if (m['id'] == id){
        final code = (m['code'] ?? '').toString();
        final name = (m['name'] ?? '').toString();
        if (name.isNotEmpty && code.isNotEmpty) return '$code – $name';
        if (name.isNotEmpty) return name;
        if (code.isNotEmpty) return code;
        break;
      }
    }
    return id;
  }

  Future<void> _create() async {
    if (!formKey.currentState!.validate()) return;
    final body = {
      'nummer': nummerCtrl.text.trim(),
      'bezeichnung': bezCtrl.text.trim(),
      'typ': typCtrl.text.trim(),
      'norm': normCtrl.text.trim(),
      'werkstoffnummer': wnrCtrl.text.trim(),
      'einheit': einheitCtrl.text.trim(),
      'dichte': double.tryParse(dichteCtrl.text.trim()) ?? 0,
      'kategorie': katCtrl.text.trim(),
      'attribute': {},
    };
    try {
      await widget.api.createMaterial(body);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Material angelegt')));
      }
      nummerCtrl.clear();
      bezCtrl.clear();
      await _reload();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _uploadFile() async {
    if (selectedId == null) return;
    final input = html.FileUploadInputElement();
    input.accept = '*/*';
    input.click();
    await input.onChange.first;
    if (input.files == null || input.files!.isEmpty) return;
    final f = input.files!.first;
    final reader = html.FileReader();
    reader.readAsArrayBuffer(f);
    await reader.onLoad.first;
    final data = reader.result as Uint8List;
    await widget.api.uploadMaterialDocument(selectedId!, f.name, data, contentType: f.type);
    docs = await widget.api.listMaterialDocuments(selectedId!);
    setState(() {});
  }

  Future<void> _openCreateDialog() async {
    await showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Material anlegen'),
        content: Form(
          key: formKey,
          child: SizedBox(
            width: 520,
            child: SingleChildScrollView(
              child: Wrap(
                runSpacing: 8,
                spacing: 8,
                children: [
                  SizedBox(
                    width: 160,
                    child: TextFormField(controller: nummerCtrl, decoration: const InputDecoration(labelText: 'Nummer'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null),
                  ),
                  SizedBox(
                    width: 220,
                    child: TextFormField(controller: bezCtrl, decoration: const InputDecoration(labelText: 'Bezeichnung'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null),
                  ),
                  SizedBox(width: 140, child: TextFormField(controller: typCtrl, decoration: const InputDecoration(labelText: 'Typ'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null)),
                  SizedBox(width: 120, child: TextFormField(controller: einheitCtrl, decoration: const InputDecoration(labelText: 'Einheit'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null)),
                  SizedBox(width: 120, child: TextFormField(controller: dichteCtrl, decoration: const InputDecoration(labelText: 'Dichte'),
                    validator: (v){
                      if (v==null||v.trim().isEmpty) return null;
                      final d = double.tryParse(v.trim());
                      if (d==null) return 'Zahl erforderlich';
                      if (d<0) return '≥ 0 erwartet';
                      return null;
                    },
                  )),
                  SizedBox(width: 140, child: TextFormField(controller: normCtrl, decoration: const InputDecoration(labelText: 'Norm'))),
                  SizedBox(width: 180, child: TextFormField(controller: wnrCtrl, decoration: const InputDecoration(labelText: 'Werkstoffnummer'))),
                  SizedBox(width: 160, child: TextFormField(controller: katCtrl, decoration: const InputDecoration(labelText: 'Kategorie'))),
                ],
              ),
            ),
          ),
        ),
        actions: [
          TextButton(onPressed: ()=> Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
          FilledButton.icon(onPressed: () async { await _create(); if (mounted) Navigator.of(ctx).pop(); }, icon: const Icon(Icons.check), label: const Text('Anlegen')),
        ],
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
            flex: 2,
            child: Column(
              children: [
              Padding(
                padding: const EdgeInsets.all(12),
                child: Row(
                  children: [
                    const Text('Materialien', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                    const Spacer(),
                    IconButton(onPressed: _reload, icon: const Icon(Icons.refresh)),
                  ],
                ),
              ),
              if (loading) const LinearProgressIndicator(minHeight: 2),
              Expanded(
                child: ListView.builder(
                  itemCount: items.length,
                  itemBuilder: (ctx, i) {
                    final m = items[i] as Map<String, dynamic>;
                    final sel = m['id'] == selectedId;
                    return ListTile(
                      selected: sel,
                      title: Text('${m['bezeichnung']}'),
                      subtitle: Text('${m['nummer']}  •  ${m['typ']}  •  ${m['einheit']}'),
                      onTap: () => _select(m['id'] as String),
                    );
                  },
                ),
              ),
            ],
          ),
          ),
          const VerticalDivider(width: 1),
          Expanded(
            flex: 3,
            child: selectedId == null
                ? const Center(child: Text('Bitte ein Material auswählen'))
                : Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(children: [
                        const Text('Bestand', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                        const Spacer(),
                        IconButton(onPressed: () async { if (selectedId!=null){ stock = await widget.api.stockByMaterial(selectedId!); setState((){});} }, icon: const Icon(Icons.refresh)),
                      ]),
                      const SizedBox(height: 8),
                      Expanded(
                        child: Container(
                          decoration: BoxDecoration(border: Border.all(color: Colors.black12)),
                          child: ListView.builder(
                            itemCount: stock.length,
                            itemBuilder: (ctx, i){
                              final r = stock[i] as Map<String, dynamic>;
                              final loc = r['location_id'] ?? '-';
                              final batch = r['batch_code'] ?? '';
                              return ListTile(
                                dense: true,
                                title: Text('Lager ${_warehouseLabel((r['warehouse_id']??'').toString())}  •  Platz ${loc ?? '-'}'),
                                subtitle: Text('Menge: ${r['menge']} ${r['einheit']}  ${batch.isNotEmpty ? '• Batch: $batch' : ''}'),
                              );
                            },
                          ),
                        ),
                      ),
                      const SizedBox(height: 12),
                      Row(children: [
                        const Text('Dokumente', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                        const Spacer(),
                        FilledButton.icon(onPressed: _uploadFile, icon: const Icon(Icons.upload_file), label: const Text('Upload')),
                      ]),
                      const SizedBox(height: 8),
                      Expanded(
                        child: Container(
                          decoration: BoxDecoration(border: Border.all(color: Colors.black12)),
                          child: ListView.builder(
                            itemCount: docs.length,
                            itemBuilder: (ctx, i){
                              final d = docs[i] as Map<String, dynamic>;
                              return ListTile(
                                dense: true,
                                leading: const Icon(Icons.description_outlined),
                                title: Text(d['filename'] ?? d['document_id']),
                                subtitle: Text('${d['content_type'] ?? ''}  •  ${d['length'] ?? 0} B'),
                                trailing: IconButton(
                                  icon: const Icon(Icons.download),
                                  onPressed: () => widget.api.downloadDocument(d['document_id'], filename: d['filename']),
                                ),
                              );
                            },
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
          ),
        ],
      ),
    );
  }
}
