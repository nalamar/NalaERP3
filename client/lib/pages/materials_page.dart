import 'package:flutter/material.dart';
import '../api.dart';
import '../web/browser.dart' as browser;

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
  Map<String, dynamic>? selected;
  List<dynamic> stock = [];
  List<dynamic> docs = [];
  List<dynamic> warehouses = [];
  // Suche & Pagination
  final searchCtrl = TextEditingController();
  int limit = 20;
  int offset = 0;
  String? filterTyp;
  String? filterKat;
  List<String> types = [];
  List<String> categories = [];
  List<Map<String, dynamic>> units = [];

  final formKey = GlobalKey<FormState>();
  final nummerCtrl = TextEditingController();
  final bezCtrl = TextEditingController();
  final typCtrl = TextEditingController(text: 'rohstoff');
  final normCtrl = TextEditingController();
  final wnrCtrl = TextEditingController();
  String? _unitSel = null;
  final dichteCtrl = TextEditingController(text: '0');
  final laengeCtrl = TextEditingController();
  final breiteCtrl = TextEditingController();
  final hoeheCtrl = TextEditingController();
  final katCtrl = TextEditingController(text: '');

  @override
  void initState() {
    super.initState();
    _reload();
    _loadFacets();
  }

  Future<void> _loadFacets() async {
    try {
      final t = await widget.api.listMaterialTypes();
      final c = await widget.api.listMaterialCategories();
      final u = await widget.api.listUnits();
      setState(() {
        types = t;
        categories = c;
        units = u;
      });
    } catch (e) {
      debugPrint('Facets error: $e');
    }
  }

  Future<void> _reload() async {
    setState(() => loading = true);
    try {
      offset = 0;
      items = await widget.api.listMaterials(
        q: searchCtrl.text.trim(),
        typ: filterTyp == null || filterTyp!.isEmpty ? null : filterTyp,
        kategorie: filterKat == null || filterKat!.isEmpty ? null : filterKat,
        limit: limit,
        offset: offset,
      );
    } catch (e) {
      if (mounted) {
        debugPrint('Fehler beim Laden Materialien: $e');
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            content: Text('Materialien konnten nicht geladen werden: $e')));
      }
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> _loadMore() async {
    final q = searchCtrl.text.trim();
    final next = await widget.api.listMaterials(
      q: q.isNotEmpty ? q : null,
      typ: filterTyp == null || filterTyp!.isEmpty ? null : filterTyp,
      kategorie: filterKat == null || filterKat!.isEmpty ? null : filterKat,
      limit: limit,
      offset: offset + limit,
    );
    if (next.isNotEmpty) {
      offset += limit;
      setState(() {
        items.addAll(next);
      });
    }
  }

  Future<void> _select(String id) async {
    setState(() => selectedId = id);
    selected = await widget.api.getMaterial(id);
    stock = await widget.api.stockByMaterial(id);
    // Lade Lagerliste, um Namen/Codes statt IDs anzeigen zu können
    try {
      warehouses = await widget.api.listWarehouses();
    } catch (_) {}
    docs = await widget.api.listMaterialDocuments(id);
    setState(() {});
  }

  Future<void> _editSelected() async {
    if (selectedId == null) return;
    final m = selected ?? await widget.api.getMaterial(selectedId!);
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) =>
          _EditMaterialDialog(initial: m, api: widget.api, units: units),
    );
    if (res == null) return;
    try {
      await widget.api.updateMaterial(selectedId!, res);
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Material aktualisiert')));
      await _select(selectedId!);
      await _reload();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Aktualisieren fehlgeschlagen: $e')));
    }
  }

  Future<void> _deleteSelected() async {
    if (selectedId == null) return;
    final ok = await showDialog<bool>(
        context: context,
        builder: (_) => AlertDialog(
                title: const Text('Material löschen'),
                content: const Text('Wirklich löschen? (wird deaktiviert)'),
                actions: [
                  TextButton(
                      onPressed: () => Navigator.pop(context, false),
                      child: const Text('Abbrechen')),
                  FilledButton(
                      onPressed: () => Navigator.pop(context, true),
                      child: const Text('Löschen'))
                ]));
    if (ok != true) return;
    try {
      await widget.api.deleteMaterial(selectedId!);
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Material gelöscht')));
      selectedId = null;
      selected = null;
      stock = [];
      docs = [];
      await _reload();
    } catch (e) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Löschen fehlgeschlagen: $e')));
    }
  }

  String _warehouseLabel(String id) {
    for (final w in warehouses) {
      final m = w as Map<String, dynamic>;
      if (m['id'] == id) {
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
      'einheit': (_unitSel ?? '').trim(),
      'dichte': double.tryParse(dichteCtrl.text.trim()) ?? 0,
      if (laengeCtrl.text.trim().isNotEmpty)
        'length_mm': double.tryParse(laengeCtrl.text.trim()),
      if (breiteCtrl.text.trim().isNotEmpty)
        'width_mm': double.tryParse(breiteCtrl.text.trim()),
      if (hoeheCtrl.text.trim().isNotEmpty)
        'height_mm': double.tryParse(hoeheCtrl.text.trim()),
      'kategorie': katCtrl.text.trim(),
      'attribute': {},
    };
    try {
      await widget.api.createMaterial(body);
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Material angelegt')));
      }
      nummerCtrl.clear();
      bezCtrl.clear();
      await _reload();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _uploadFile() async {
    if (selectedId == null) return;
    final picked = await browser.pickFile(accept: '*/*');
    if (picked == null) return;
    try {
      await widget.api.uploadMaterialDocument(
        selectedId!,
        picked.filename,
        picked.bytes,
        contentType: picked.contentType,
      );
      docs = await widget.api.listMaterialDocuments(selectedId!);
      if (mounted) setState(() {});
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Upload erfolgreich')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Upload fehlgeschlagen: ')));
      }
    }
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
                    child: TextFormField(
                        controller: nummerCtrl,
                        decoration: const InputDecoration(labelText: 'Nummer'),
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? 'Pflichtfeld'
                            : null),
                  ),
                  SizedBox(
                    width: 220,
                    child: TextFormField(
                        controller: bezCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Bezeichnung'),
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? 'Pflichtfeld'
                            : null),
                  ),
                  SizedBox(
                      width: 140,
                      child: TextFormField(
                          controller: typCtrl,
                          decoration: const InputDecoration(labelText: 'Typ'),
                          validator: (v) => (v == null || v.trim().isEmpty)
                              ? 'Pflichtfeld'
                              : null)),
                  SizedBox(
                      width: 160,
                      child: DropdownButtonFormField<String>(
                        isDense: true,
                        decoration: const InputDecoration(labelText: 'Einheit'),
                        initialValue: _unitSel,
                        items: [
                          for (final u in units)
                            DropdownMenuItem<String>(
                                value: (u['code'] ?? '').toString(),
                                child: Text((u['code'] ?? '').toString()))
                        ],
                        onChanged: (v) {
                          _unitSel = v;
                        },
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? 'Pflichtfeld'
                            : null,
                      )),
                  SizedBox(
                      width: 120,
                      child: TextFormField(
                        controller: dichteCtrl,
                        decoration: const InputDecoration(labelText: 'Dichte'),
                        validator: (v) {
                          if (v == null || v.trim().isEmpty) return null;
                          final d = double.tryParse(v.trim());
                          if (d == null) return 'Zahl erforderlich';
                          if (d < 0) return '≥ 0 erwartet';
                          return null;
                        },
                      )),
                  SizedBox(
                      width: 140,
                      child: TextFormField(
                          controller: normCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Norm'))),
                  SizedBox(
                      width: 180,
                      child: TextFormField(
                          controller: wnrCtrl,
                          decoration: const InputDecoration(
                              labelText: 'Werkstoffnummer'))),
                  SizedBox(
                      width: 160,
                      child: TextFormField(
                          controller: katCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Kategorie'))),
                  SizedBox(
                      width: 120,
                      child: TextFormField(
                          controller: laengeCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Länge (mm)'),
                          keyboardType: TextInputType.number)),
                  SizedBox(
                      width: 120,
                      child: TextFormField(
                          controller: breiteCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Breite (mm)'),
                          keyboardType: TextInputType.number)),
                  SizedBox(
                      width: 120,
                      child: TextFormField(
                          controller: hoeheCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Höhe (mm)'),
                          keyboardType: TextInputType.number)),
                ],
              ),
            ),
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Abbrechen')),
          FilledButton.icon(
              onPressed: () async {
                await _create();
                if (mounted) Navigator.of(ctx).pop();
              },
              icon: const Icon(Icons.check),
              label: const Text('Anlegen')),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        surfaceTintColor: Colors.transparent,
        title: const Text('Materialien'),
      ),
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
                  child: Container(
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        colors: [
                          Colors.white.withValues(alpha: 0.12),
                          Colors.white.withValues(alpha: 0.05)
                        ],
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                      ),
                      borderRadius: BorderRadius.circular(14),
                      border: Border.all(
                          color: Colors.white.withValues(alpha: 0.14)),
                      boxShadow: [
                        BoxShadow(
                            color: Colors.black.withValues(alpha: 0.22),
                            blurRadius: 18,
                            offset: const Offset(0, 12))
                      ],
                    ),
                    padding: const EdgeInsets.all(12),
                    child: Row(
                      children: [
                        const Text('Materialien',
                            style: TextStyle(
                                fontSize: 18, fontWeight: FontWeight.bold)),
                        const Spacer(),
                        SizedBox(
                            width: 260,
                            height: 40,
                            child: TextField(
                              controller: searchCtrl,
                              textAlignVertical: TextAlignVertical.center,
                              decoration: InputDecoration(
                                  isDense: true,
                                  contentPadding: const EdgeInsets.symmetric(
                                      horizontal: 12, vertical: 10),
                                  prefixIcon: const Icon(Icons.search),
                                  hintText: 'Suchen (Nummer/Bezeichnung)',
                                  suffixIcon: IconButton(
                                      icon: const Icon(Icons.clear),
                                      onPressed: () {
                                        searchCtrl.clear();
                                        _reload();
                                      })),
                              onSubmitted: (_) => _reload(),
                            )),
                        const SizedBox(width: 8),
                        SizedBox(
                            width: 180,
                            height: 40,
                            child: DropdownButtonFormField<String?>(
                              isDense: true,
                              decoration: const InputDecoration(
                                  isDense: true,
                                  labelText: 'Typ',
                                  contentPadding: EdgeInsets.symmetric(
                                      horizontal: 12, vertical: 10)),
                              initialValue: filterTyp,
                              items: [
                                const DropdownMenuItem<String?>(
                                    value: null, child: Text('Alle')),
                                for (final t in types)
                                  DropdownMenuItem<String?>(
                                      value: t, child: Text(t)),
                              ],
                              onChanged: (v) {
                                setState(() {
                                  filterTyp = v;
                                });
                                _reload();
                              },
                            )),
                        const SizedBox(width: 8),
                        SizedBox(
                            width: 180,
                            height: 40,
                            child: DropdownButtonFormField<String?>(
                              isDense: true,
                              decoration: const InputDecoration(
                                  isDense: true,
                                  labelText: 'Kategorie',
                                  contentPadding: EdgeInsets.symmetric(
                                      horizontal: 12, vertical: 10)),
                              initialValue: filterKat,
                              items: [
                                const DropdownMenuItem<String?>(
                                    value: null, child: Text('Alle')),
                                for (final k in categories)
                                  DropdownMenuItem<String?>(
                                      value: k, child: Text(k)),
                              ],
                              onChanged: (v) {
                                setState(() {
                                  filterKat = v;
                                });
                                _reload();
                              },
                            )),
                        const SizedBox(width: 8),
                        SizedBox(
                            height: 40,
                            child: FilledButton.tonal(
                                onPressed: _reload,
                                style: FilledButton.styleFrom(
                                    minimumSize: const Size(100, 40)),
                                child: const Text('Suchen'))),
                        const SizedBox(width: 4),
                        SizedBox(
                            height: 40,
                            width: 40,
                            child: IconButton(
                                onPressed: _reload,
                                padding: EdgeInsets.zero,
                                constraints: const BoxConstraints.tightFor(
                                    width: 40, height: 40),
                                icon: const Icon(Icons.refresh))),
                      ],
                    ),
                  ),
                ),
                if (loading) const LinearProgressIndicator(minHeight: 2),
                Expanded(
                  child: ListView.builder(
                    itemCount: items.length + 1,
                    itemBuilder: (ctx, i) {
                      if (i < items.length) {
                        final m = items[i] as Map<String, dynamic>;
                        final sel = m['id'] == selectedId;
                        String dims() {
                          String fmt(num? v) {
                            if (v == null) return '';
                            final d = v.toDouble();
                            if ((d - d.round()).abs() < 0.001)
                              return d.round().toString();
                            return d
                                .toStringAsFixed(2)
                                .replaceAll(RegExp(r'0+$'), '')
                                .replaceAll(RegExp(r'\.$'), '');
                          }

                          final l = m['length_mm'] as num?;
                          final w = m['width_mm'] as num?;
                          final h = m['height_mm'] as num?;
                          final parts = <String>[];
                          if (l != null) parts.add(fmt(l));
                          if (w != null) parts.add(fmt(w));
                          if (h != null) parts.add(fmt(h));
                          if (parts.isEmpty) return '';
                          return parts.join('×') + ' mm';
                        }

                        final dim = dims();
                        return ListTile(
                          selected: sel,
                          title: Text('${m['bezeichnung']}'),
                          subtitle: Text([
                            '${m['nummer']}',
                            '${m['typ']}',
                            if (dim.isNotEmpty) dim else '${m['einheit']}',
                          ]
                              .where((e) => e.toString().trim().isNotEmpty)
                              .join('  •  ')),
                          onTap: () => _select(m['id'] as String),
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
                              label: const Text('Mehr laden')),
                        ),
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
                        // Abstand nach unten, damit die Kopfzeile nicht mit der linken Suchzeile kollidiert
                        const SizedBox(height: 56),
                        if (selected != null) ...[
                          Row(children: [
                            Expanded(
                                child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                  Text('${selected!['bezeichnung']}',
                                      style: const TextStyle(
                                          fontSize: 18,
                                          fontWeight: FontWeight.bold)),
                                  const SizedBox(height: 4),
                                  Text(
                                      'Nr.: ${selected!['nummer']}  •  Typ: ${selected!['typ']}  •  Einheit: ${selected!['einheit']}'),
                                  if ((selected!['kategorie'] ?? '')
                                      .toString()
                                      .isNotEmpty)
                                    Text(
                                        'Kategorie: ${selected!['kategorie']}'),
                                ])),
                            IconButton(
                                onPressed: _editSelected,
                                icon: const Icon(Icons.edit)),
                            IconButton(
                                onPressed: _deleteSelected,
                                icon: const Icon(Icons.delete_outline)),
                          ]),
                          const SizedBox(height: 12),
                        ],
                        Row(children: [
                          const Text('Bestand',
                              style: TextStyle(
                                  fontSize: 18, fontWeight: FontWeight.bold)),
                          const Spacer(),
                          IconButton(
                              onPressed: () async {
                                if (selectedId != null) {
                                  stock = await widget.api
                                      .stockByMaterial(selectedId!);
                                  setState(() {});
                                }
                              },
                              icon: const Icon(Icons.refresh)),
                        ]),
                        const SizedBox(height: 8),
                        Expanded(
                          child: Container(
                            decoration: BoxDecoration(
                                border: Border.all(color: Colors.black12)),
                            child: ListView.builder(
                              itemCount: stock.length,
                              itemBuilder: (ctx, i) {
                                final r = stock[i] as Map<String, dynamic>;
                                final loc = r['location_id'] ?? '-';
                                final batch = r['batch_code'] ?? '';
                                return ListTile(
                                  dense: true,
                                  title: Text(
                                      'Lager ${_warehouseLabel((r['warehouse_id'] ?? '').toString())}  •  Platz ${loc ?? '-'}'),
                                  subtitle: Text(
                                      'Menge: ${r['menge']} ${r['einheit']}  ${batch.isNotEmpty ? '• Batch: $batch' : ''}'),
                                );
                              },
                            ),
                          ),
                        ),
                        const SizedBox(height: 12),
                        Row(children: [
                          const Text('Dokumente',
                              style: TextStyle(
                                  fontSize: 18, fontWeight: FontWeight.bold)),
                          const Spacer(),
                          FilledButton.icon(
                              onPressed: _uploadFile,
                              icon: const Icon(Icons.upload_file),
                              label: const Text('Upload')),
                        ]),
                        const SizedBox(height: 8),
                        Expanded(
                          child: Container(
                            decoration: BoxDecoration(
                                border: Border.all(color: Colors.black12)),
                            child: ListView.builder(
                              itemCount: docs.length,
                              itemBuilder: (ctx, i) {
                                final d = docs[i] as Map<String, dynamic>;
                                return ListTile(
                                  dense: true,
                                  leading:
                                      const Icon(Icons.description_outlined),
                                  title:
                                      Text(d['filename'] ?? d['document_id']),
                                  subtitle: Text(
                                      '${d['content_type'] ?? ''}  •  ${d['length'] ?? 0} B'),
                                  trailing: IconButton(
                                    icon: const Icon(Icons.download),
                                    onPressed: () => widget.api
                                        .downloadDocument(d['document_id'],
                                            filename: d['filename']),
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

class _EditMaterialDialog extends StatefulWidget {
  const _EditMaterialDialog(
      {required this.initial, required this.api, required this.units});
  final Map<String, dynamic> initial;
  final ApiClient api;
  final List<Map<String, dynamic>> units;
  @override
  State<_EditMaterialDialog> createState() => _EditMaterialDialogState();
}

class _EditMaterialDialogState extends State<_EditMaterialDialog> {
  final _formKey = GlobalKey<FormState>();
  late final nummer =
      TextEditingController(text: widget.initial['nummer']?.toString() ?? '');
  late final bez = TextEditingController(
      text: widget.initial['bezeichnung']?.toString() ?? '');
  late final typ =
      TextEditingController(text: widget.initial['typ']?.toString() ?? '');
  late final norm =
      TextEditingController(text: widget.initial['norm']?.toString() ?? '');
  late final wnr = TextEditingController(
      text: widget.initial['werkstoffnummer']?.toString() ?? '');
  String? einheit;
  List<Map<String, dynamic>> _units = const [];
  late final dichte =
      TextEditingController(text: (widget.initial['dichte'] ?? 0).toString());
  late final laenge = TextEditingController(
      text: (widget.initial['length_mm'] ?? '').toString());
  late final breite = TextEditingController(
      text: (widget.initial['width_mm'] ?? '').toString());
  late final hoehe = TextEditingController(
      text: (widget.initial['height_mm'] ?? '').toString());
  late final kat = TextEditingController(
      text: widget.initial['kategorie']?.toString() ?? '');

  @override
  void initState() {
    super.initState();
    // Übernehme Einheiten aus Parametern, lade ansonsten nach
    _units = widget.units;
    if (_units.isEmpty) {
      _loadUnitsFallback();
    }
  }

  Future<void> _loadUnitsFallback() async {
    try {
      final list = await widget.api.listUnits();
      if (mounted) setState(() => _units = list);
    } catch (_) {
      // ignore – wenn es fehlschlägt, bleibt die Liste leer
    } finally {}
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Material bearbeiten'),
      content: SizedBox(
        width: 520,
        child: Form(
          key: _formKey,
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            Row(children: [
              Expanded(
                  child: TextFormField(
                      controller: nummer,
                      decoration: const InputDecoration(labelText: 'Nummer'),
                      validator: (v) => (v == null || v.trim().isEmpty)
                          ? 'Pflichtfeld'
                          : null)),
              const SizedBox(width: 8),
              Expanded(
                  child: TextFormField(
                      controller: bez,
                      decoration:
                          const InputDecoration(labelText: 'Bezeichnung'),
                      validator: (v) => (v == null || v.trim().isEmpty)
                          ? 'Pflichtfeld'
                          : null)),
            ]),
            Row(children: [
              Expanded(
                  child: TextFormField(
                      controller: typ,
                      decoration: const InputDecoration(labelText: 'Typ'))),
              const SizedBox(width: 8),
              Expanded(
                  child: TextFormField(
                      controller: norm,
                      decoration: const InputDecoration(labelText: 'Norm'))),
            ]),
            Row(children: [
              Expanded(
                  child: TextFormField(
                      controller: wnr,
                      decoration:
                          const InputDecoration(labelText: 'Werkstoffnummer'))),
              const SizedBox(width: 8),
              SizedBox(
                  width: 160,
                  child: DropdownButtonFormField<String>(
                    isDense: true,
                    decoration: const InputDecoration(labelText: 'Einheit'),
                    initialValue:
                        einheit ?? widget.initial['einheit']?.toString(),
                    items: [
                      for (final u in _units)
                        DropdownMenuItem<String>(
                            value: (u['code'] ?? '').toString(),
                            child: Text((u['code'] ?? '').toString()))
                    ],
                    onChanged: (v) {
                      einheit = v;
                    },
                    validator: (v) =>
                        (v == null || v.trim().isEmpty) ? 'Pflichtfeld' : null,
                  )),
              const SizedBox(width: 8),
              SizedBox(
                  width: 120,
                  child: TextFormField(
                      controller: dichte,
                      decoration: const InputDecoration(labelText: 'Dichte'),
                      keyboardType: TextInputType.number))
            ]),
            Row(children: [
              Expanded(
                  child: TextFormField(
                      controller: kat,
                      decoration:
                          const InputDecoration(labelText: 'Kategorie'))),
            ]),
            Row(children: [
              Expanded(
                  child: TextFormField(
                      controller: laenge,
                      decoration:
                          const InputDecoration(labelText: 'Länge (mm)'),
                      keyboardType: TextInputType.number)),
              const SizedBox(width: 8),
              Expanded(
                  child: TextFormField(
                      controller: breite,
                      decoration:
                          const InputDecoration(labelText: 'Breite (mm)'),
                      keyboardType: TextInputType.number)),
              const SizedBox(width: 8),
              Expanded(
                  child: TextFormField(
                      controller: hoehe,
                      decoration: const InputDecoration(labelText: 'Höhe (mm)'),
                      keyboardType: TextInputType.number)),
            ]),
          ]),
        ),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Abbrechen')),
        FilledButton(
            onPressed: () {
              if (!_formKey.currentState!.validate()) return;
              Navigator.pop(context, {
                'nummer': nummer.text.trim(),
                'bezeichnung': bez.text.trim(),
                'typ': typ.text.trim(),
                'norm': norm.text.trim(),
                'werkstoffnummer': wnr.text.trim(),
                'einheit':
                    (einheit ?? widget.initial['einheit']?.toString() ?? '')
                        .trim(),
                'dichte': double.tryParse(dichte.text.trim()),
                'kategorie': kat.text.trim(),
                'length_mm': double.tryParse(laenge.text.trim()),
                'width_mm': double.tryParse(breite.text.trim()),
                'height_mm': double.tryParse(hoehe.text.trim()),
              });
            },
            child: const Text('Speichern')),
      ],
    );
  }
}
