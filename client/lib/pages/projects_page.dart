import 'package:flutter/material.dart';
import '../api.dart';
import '../web/browser.dart' as browser;
import 'package:http/http.dart' as http;
import 'dart:convert';

class ProjectsPage extends StatefulWidget {
  const ProjectsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<ProjectsPage> createState() => _ProjectsPageState();
}

class _ProjectsPageState extends State<ProjectsPage> {
  bool _loading = true;
  String? _error;
  List<dynamic> _projects = const [];
  bool _importing = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final list = await widget.api.listProjects();
      setState(() { _projects = list; });
    } catch (e) {
      setState(() { _error = e.toString(); });
    } finally {
      if (mounted) setState(() { _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    return Scaffold(
      appBar: AppBar(title: const Text('Projekte'), backgroundColor: color, foregroundColor: Colors.white, actions: [
        TextButton.icon(onPressed: _analyzeLogikal, icon: const Icon(Icons.search_rounded, color: Colors.white), label: const Text('Analyse Logikal', style: TextStyle(color: Colors.white))),
        TextButton.icon(onPressed: _importing ? null : _importLogikal, icon: const Icon(Icons.upload_file_rounded, color: Colors.white), label: const Text('Import Logikal', style: TextStyle(color: Colors.white))),
        const SizedBox(width: 8),
      ]),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(child: Text('Fehler: $_error'))
                : _projects.isEmpty
                    ? Center(
                        child: Column(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            const Icon(Icons.workspaces_rounded, size: 48, color: Colors.black26),
                            const SizedBox(height: 12),
                            const Text('Noch keine Projekte.'),
                            const SizedBox(height: 8),
                            FilledButton.icon(onPressed: _addProject, icon: const Icon(Icons.add), label: const Text('Projekt anlegen')),
                          ],
                        ),
                      )
                    : ListView.separated(
                        itemCount: _projects.length,
                        separatorBuilder: (_, __) => const Divider(height: 1),
                        itemBuilder: (context, index) {
                          final p = _projects[index] as Map<String, dynamic>;
                          return ListTile(
                            leading: CircleAvatar(backgroundColor: color.withValues(alpha: 0.12), child: Icon(Icons.work_outline_rounded, color: color)),
                            title: Text(p['name']?.toString() ?? 'Projekt'),
                            subtitle: Text(p['nummer']?.toString() ?? ''),
                            onTap: () {
                              Navigator.of(context).push(MaterialPageRoute(builder: (_) => ProjectDetailPage(api: widget.api, project: p))).then((_) => _load());
                            },
                          );
                        },
                      ),
      ),
      floatingActionButton: FloatingActionButton.extended(onPressed: _addProject, icon: const Icon(Icons.add), label: const Text('Projekt')),
    );
  }

  Future<void> _importLogikal() async {
    try {
      final picked = await browser.pickFile(accept: '.db,.sqlite,.sqlite3');
      if (picked == null) return;
      // Debug-Ausgabe
      // ignore: avoid_print
      print('Import: picked ${picked.filename} bytes=${picked.bytes.length} ct=${picked.contentType}');
      setState(() { _importing = true; });
      if (mounted) {
        showDialog(context: context, barrierDismissible: false, builder: (_) => const _ProgressDialog(text: 'Import läuft...'));
      }
      final res = await widget.api.importLogikalProject(picked.filename, picked.bytes, contentType: picked.contentType);
      if (mounted) Navigator.of(context, rootNavigator: true).pop();
      setState(() { _importing = false; });
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Import erfolgreich: ${res['projekt']?['name'] ?? ''}')));
      // Optional: nach Assets-ZIP fragen
      final wantAssets = await showDialog<bool>(
        context: context,
        builder: (_) => AlertDialog(
          content: const Text('Möchten Sie zusätzlich einen Assets-Ordner (ZIP mit Emfs/Rtfs) hochladen?'),
          actions: [
            TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Nein')),
            FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Ja')),
          ],
        ),
      );
      if (wantAssets == true) {
        final zip = await browser.pickFile(accept: '.zip');
        if (zip != null) {
          try {
            setState(() { _importing = true; });
            if (mounted) {
              showDialog(context: context, barrierDismissible: false, builder: (_) => const _ProgressDialog(text: 'Assets werden hochgeladen...'));
            }
            await widget.api.uploadProjectAssets((res['projekt']?['id'] ?? widget.api.toString()).toString(), zip.filename, zip.bytes);
            if (mounted) Navigator.of(context, rootNavigator: true).pop();
            setState(() { _importing = false; });
            if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Assets hochgeladen.')));
          } catch (e) {
            if (mounted) {
              Navigator.of(context, rootNavigator: true).pop();
              setState(() { _importing = false; });
              ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Assets-Upload fehlgeschlagen: $e')));
            }
          }
        }
      }
      _load();
    } catch (e) {
      // Hilfreiche Ausgabe in der Konsole
      // ignore: avoid_print
      print('Import-Fehler: $e');
      if (!mounted) return;
      try { Navigator.of(context, rootNavigator: true).pop(); } catch (_) {}
      setState(() { _importing = false; });
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Import fehlgeschlagen: $e')));
    }
  }

  Future<void> _analyzeLogikal() async {
    try {
      final picked = await browser.pickFile(accept: '.db,.sqlite,.sqlite3');
      if (picked == null) return;
      final res = await widget.api.analyzeLogikalProject(picked.filename, picked.bytes, contentType: picked.contentType);
      if (!mounted) return;
      await showDialog(context: context, builder: (_) => _AnalysisDialog(summary: res));
    } catch (e) {
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Analyse fehlgeschlagen: $e')));
    }
  }

  Future<void> _addProject() async {
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _ProjectCreateDialog(api: widget.api),
    );
    if (res == null) return;
    if ((res['name'] as String?) == null || (res['name'] as String).trim().isEmpty) {
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Name ist erforderlich'))); return;
    }
    try {
      final body = {
        if ((res['nummer'] ?? '').toString().trim().isNotEmpty) 'nummer': res['nummer'],
        'name': res['name'],
        if ((res['kunde_id'] ?? '').toString().isNotEmpty) 'kunde_id': res['kunde_id'],
      };
      final created = await widget.api.createProject(body);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Projekt erstellt: ${created['name']}')));
      await _load();
      // direkt öffnen
      Navigator.of(context).push(MaterialPageRoute(builder: (_) => ProjectDetailPage(api: widget.api, project: created))).then((_) => _load());
    } catch (e) {
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Erstellen fehlgeschlagen: $e')));
    }
  }
}

class ProjectDetailPage extends StatefulWidget {
  const ProjectDetailPage({super.key, required this.api, required this.project});
  final ApiClient api;
  final Map<String, dynamic> project;

  @override
  State<ProjectDetailPage> createState() => _ProjectDetailPageState();
}

class _ProjectDetailPageState extends State<ProjectDetailPage> {
  bool _loading = true;
  String? _error;
  List<dynamic> _phases = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final list = await widget.api.listProjectPhases(widget.project['id'] as String);
      setState(() { _phases = list; });
    } catch (e) { setState(() { _error = e.toString(); }); }
    finally { if (mounted) setState(() { _loading = false; }); }
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    return DefaultTabController(
      length: 2,
      child: Scaffold(
        appBar: AppBar(
          title: Text(widget.project['name']?.toString() ?? 'Projekt'),
          backgroundColor: color,
          foregroundColor: Colors.white,
          actions: [
            IconButton(onPressed: _addPhase, icon: const Icon(Icons.playlist_add_rounded), tooltip: 'Los hinzufügen'),
            const SizedBox(width: 6),
          ],
          bottom: TabBar(
            labelColor: Colors.white,
            unselectedLabelColor: Colors.white,
            labelStyle: const TextStyle(fontWeight: FontWeight.bold),
            unselectedLabelStyle: const TextStyle(fontWeight: FontWeight.bold),
            tabs: const [
              Tab(text: 'Struktur'),
              Tab(text: 'Protokoll'),
            ],
          ),
        ),
        body: TabBarView(children: [
          _buildStructureTab(),
          ImportLogTab(api: widget.api, projectId: widget.project['id'] as String),
        ]),
      ),
    );
  }

  Widget _buildStructureTab() {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) return Center(child: Text('Fehler: $_error'));
    final left = ListView.builder(
      itemCount: _phases.length,
      itemBuilder: (context, index) {
        final ph = _phases[index] as Map<String, dynamic>;
        return _PhaseTile(api: widget.api, projectId: widget.project['id'] as String, phase: ph, onChanged: _load);
      },
    );
    final right = Padding(
      padding: const EdgeInsets.all(16),
      child: SingleChildScrollView(
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          const Text('Aktionen', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
          const SizedBox(height: 12),
          FilledButton.icon(onPressed: () {/* TODO: Bestellung */}, icon: const Icon(Icons.shopping_cart_checkout_rounded), label: const Text('Bestellung')),
          const SizedBox(height: 8),
          FilledButton.icon(onPressed: null, icon: const Icon(Icons.request_quote_rounded), label: const Text('Rechnung schreiben (bald)')),
          const SizedBox(height: 24),
          const Text('Weitere Links', style: TextStyle(fontWeight: FontWeight.bold)),
          const SizedBox(height: 8),
          OutlinedButton.icon(onPressed: () {/* TODO */}, icon: const Icon(Icons.picture_as_pdf_rounded), label: const Text('Dokumente')),
          const SizedBox(height: 8),
          OutlinedButton.icon(onPressed: () {/* TODO */}, icon: const Icon(Icons.settings_rounded), label: const Text('Einstellungen')),
        ]),
      ),
    );
    return Row(children: [
      Expanded(flex: 2, child: left),
      const VerticalDivider(width: 1),
      Expanded(flex: 1, child: right),
    ]);
  }

  Future<void> _addPhase() async {
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _EditDialog(title: 'Los anlegen', fields: const ['nummer','name','beschreibung'], initial: const {'nummer':'1','name':''}),
    );
    if (res == null) return;
    try { await widget.api.createPhase(widget.project['id'] as String, res); await _load(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Anlegen fehlgeschlagen: $e'))); }
  }
}

class _PhaseTile extends StatefulWidget {
  const _PhaseTile({required this.api, required this.projectId, required this.phase, required this.onChanged});
  final ApiClient api;
  final String projectId;
  final Map<String, dynamic> phase;
  final Future<void> Function() onChanged;
  @override
  State<_PhaseTile> createState() => _PhaseTileState();
}

class _PhaseTileState extends State<_PhaseTile> {
  List<dynamic>? _elevations;
  String? _error;

  Future<void> _load() async {
    try {
      final list = await widget.api.listPhaseElevations(widget.projectId, widget.phase['id'] as String);
      setState(() { _elevations = list; _error = null; });
    } catch (e) { setState(() { _error = e.toString(); }); }
  }

  @override
  Widget build(BuildContext context) {
    final phName = (widget.phase['name']?.toString() ?? '').trim();
    final phNum = (widget.phase['nummer']?.toString() ?? '').trim();
    final title = phName.isNotEmpty ? phName : (phNum.isNotEmpty ? 'Los $phNum' : 'Los');
    return ExpansionTile(
      title: Row(children: [
        Expanded(child: Text(title, overflow: TextOverflow.ellipsis)),
        IconButton(onPressed: _editPhase, icon: const Icon(Icons.edit_rounded), tooltip: 'Bearbeiten'),
        IconButton(onPressed: _deletePhase, icon: const Icon(Icons.delete_outline_rounded), tooltip: 'Löschen'),
        IconButton(onPressed: _addElevation, icon: const Icon(Icons.add_box_rounded), tooltip: 'Position hinzufügen'),
      ]),
      subtitle: phName.isNotEmpty && phNum.isNotEmpty ? Text('Los: $phNum') : null,
      initiallyExpanded: false,
      onExpansionChanged: (open) { if (open && _elevations == null) { _load(); } },
      children: [
        if (_error != null) Padding(padding: const EdgeInsets.all(12), child: Text('Fehler: $_error')),
        if (_elevations == null) const Padding(padding: EdgeInsets.all(12), child: CircularProgressIndicator()),
        if (_elevations != null) for (final e in _elevations!) _ElevationTile(api: widget.api, projectId: widget.projectId, phaseId: widget.phase['id'] as String, elevation: e as Map<String, dynamic>, onChanged: _load),
      ],
    );
  }

  Future<void> _editPhase() async {
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _EditDialog(title: 'Los bearbeiten', fields: const ['nummer','name','beschreibung','sort_order'], initial: widget.phase),
    );
    if (res == null) return;
    try { await widget.api.updatePhase(widget.projectId, widget.phase['id'] as String, res); await widget.onChanged(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Speichern fehlgeschlagen: $e'))); }
  }
  Future<void> _deletePhase() async {
    final ok = await showDialog<bool>(context: context, builder: (_) => _ConfirmDialog(text: 'Los wirklich löschen?'));
    if (ok != true) return;
    try { await widget.api.deletePhase(widget.projectId, widget.phase['id'] as String); await widget.onChanged(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Löschen fehlgeschlagen: $e'))); }
  }
  Future<void> _addElevation() async {
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _EditDialog(title: 'Position anlegen', fields: const ['nummer','name','beschreibung','menge','width_mm','height_mm'], initial: const {'nummer':'1','name':'', 'menge':'1'}),
    );
    if (res == null) return;
    try { await widget.api.createElevation(widget.projectId, widget.phase['id'] as String, res); await _load(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Anlegen fehlgeschlagen: $e'))); }
  }
}

class _ElevationTile extends StatefulWidget {
  const _ElevationTile({required this.api, required this.projectId, required this.phaseId, required this.elevation, required this.onChanged});
  final ApiClient api;
  final String projectId;
  final String phaseId;
  final Map<String, dynamic> elevation;
  final Future<void> Function() onChanged;
  @override
  State<_ElevationTile> createState() => _ElevationTileState();
}

class _ElevationTileState extends State<_ElevationTile> {
  List<dynamic>? _variants;
  String? _error;
  String _normKey(String k) => k.toLowerCase().replaceAll(RegExp(r'[^a-z0-9]'), '');
  dynamic _looseGet(Map<String, dynamic> m, List<String> desired) {
    // direct
    for (final entry in m.entries) {
      final nk = _normKey(entry.key);
      for (final d in desired) { if (nk == _normKey(d)) return entry.value; }
    }
    // common nested containers
    for (final c in ['properties','props','attributes','meta']) {
      final v = m[c];
      if (v is Map<String, dynamic>) {
        for (final entry in v.entries) {
          final nk = _normKey(entry.key);
          for (final d in desired) { if (nk == _normKey(d)) return entry.value; }
        }
      }
    }
    return null;
  }
  String _fmt4Local(dynamic v) {
    if (v == null) return '-';
    num? n;
    if (v is num) n = v; else n = num.tryParse(v.toString());
    if (n == null) return '-';
    var s = n.toDouble().toStringAsFixed(4);
    while (s.contains('.') && s.endsWith('0')) { s = s.substring(0, s.length - 1); }
    if (s.endsWith('.')) s = s.substring(0, s.length - 1);
    return s;
  }
  Future<void> _load() async {
    try {
      final list = await widget.api.listElevationVariants(widget.projectId, widget.elevation['id'] as String);
      setState(() { _variants = list; _error = null; });
    } catch (e) { setState(() { _error = e.toString(); }); }
  }
  @override
  Widget build(BuildContext context) {
    final title = '${widget.elevation['nummer']}: ${widget.elevation['name']} (Menge ${widget.elevation['menge']})';
    final serie = (widget.elevation['serie'] as String?)?.trim() ?? '';
    final surf = (widget.elevation['oberflaeche'] as String?)?.trim() ?? '';
    // Kurzbeschreibung oberhalb von Serie/Oberfläche
    final autoDesc = (_looseGet(widget.elevation, ['AutoDescription']) ?? '').toString().trim();
    final wOut = _looseGet(widget.elevation, ['Width_Output','Width Output','width_output','Width']);
    final wUnit = (_looseGet(widget.elevation, ['Width_unit','Width_Unit','width_unit']) ?? 'mm').toString();
    final lOut = _looseGet(widget.elevation, ['Lenght_Output','Length_Output','length_output','Lenght','Length','height_mm','length_mm']);
    final lUnit = (_looseGet(widget.elevation, ['Lenght_Unit','Length_Unit','length_unit']) ?? 'mm').toString();
    final shortDescParts = <String>[];
    if (autoDesc.isNotEmpty) shortDescParts.add(autoDesc);
    if (wOut != null && lOut != null) {
      shortDescParts.add('${_fmt4Local(wOut)} $wUnit x ${_fmt4Local(lOut)} $lUnit');
    }
    final shortDesc = shortDescParts.join(' ');
    final subs = <String>[];
    if (serie.isNotEmpty) subs.add('Serie: $serie');
    if (surf.isNotEmpty) subs.add('Oberfläche: $surf');
    // GUID absichtlich nicht anzeigen
    return ExpansionTile(
      title: Row(crossAxisAlignment: CrossAxisAlignment.center, children: [
        if ((widget.elevation['picture1_relpath'] as String?)?.isNotEmpty == true) ...[
          Padding(
            padding: const EdgeInsets.only(right: 12),
            child: _ElevationImage(
              projectId: widget.projectId,
              api: widget.api,
              relPath: widget.elevation['picture1_relpath'] as String,
            ),
          ),
        ],
        Expanded(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(title, overflow: TextOverflow.ellipsis),
            if (shortDesc.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(shortDesc, overflow: TextOverflow.ellipsis, style: const TextStyle(fontSize: 12, color: Colors.black87)),
            ],
            if (serie.isNotEmpty || surf.isNotEmpty) ...[
              const SizedBox(height: 4),
              if (serie.isNotEmpty) Text('Serie: $serie', overflow: TextOverflow.ellipsis, style: const TextStyle(fontSize: 12, color: Colors.black54)),
              if (surf.isNotEmpty) Text('Oberfläche: $surf', overflow: TextOverflow.ellipsis, style: const TextStyle(fontSize: 12, color: Colors.black54)),
            ],
          ]),
        ),
        IconButton(onPressed: _editElevation, icon: const Icon(Icons.edit_rounded), tooltip: 'Bearbeiten'),
        IconButton(onPressed: _deleteElevation, icon: const Icon(Icons.delete_outline_rounded), tooltip: 'Löschen'),
        IconButton(onPressed: _addVariant, icon: const Icon(Icons.add_circle_outline_rounded), tooltip: 'Variante hinzufügen'),
      ]),
      subtitle: null,
      onExpansionChanged: (open) { if (open && _variants == null) { _load(); } },
      children: [
        if (_error != null) Padding(padding: const EdgeInsets.all(12), child: Text('Fehler: $_error')),
        if (_variants == null) const Padding(padding: EdgeInsets.all(12), child: CircularProgressIndicator()),
        if (_variants != null) for (final v in _variants!) _VariantTile(api: widget.api, projectId: widget.projectId, elevationId: widget.elevation['id'] as String, variant: v as Map<String, dynamic>, onChanged: _load),
      ],
    );
  }

  Future<void> _editElevation() async {
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _EditDialog(title: 'Position bearbeiten', fields: const ['nummer','name','beschreibung','menge','width_mm','height_mm','external_guid'], initial: widget.elevation),
    );
    if (res == null) return;
    try { await widget.api.updateElevation(widget.projectId, widget.phaseId, widget.elevation['id'] as String, res); await widget.onChanged(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Speichern fehlgeschlagen: $e'))); }
  }
  Future<void> _deleteElevation() async {
    final ok = await showDialog<bool>(context: context, builder: (_) => _ConfirmDialog(text: 'Position wirklich löschen?'));
    if (ok != true) return;
    try { await widget.api.deleteElevation(widget.projectId, widget.phaseId, widget.elevation['id'] as String); await widget.onChanged(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Löschen fehlgeschlagen: $e'))); }
  }
  Future<void> _addVariant() async {
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _EditDialog(title: 'Variante anlegen', fields: const ['name','beschreibung','menge','external_guid'], initial: const {'name':'','menge':'1'}),
    );
    if (res == null) return;
    try { await widget.api.createVariant(widget.projectId, widget.elevation['id'] as String, res); await _load(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Anlegen fehlgeschlagen: $e'))); }
  }
}

class _ElevationImage extends StatelessWidget {
  const _ElevationImage({required this.projectId, required this.api, required this.relPath});
  final String projectId;
  final ApiClient api;
  final String relPath;
  @override
  Widget build(BuildContext context) {
    final url = api.projectAssetUrl(projectId, relPath);
    final isEmf = relPath.toLowerCase().endsWith('.emf');
    return ClipRRect(
      borderRadius: BorderRadius.circular(6),
      child: Image.network(
        url,
        width: 200,
        height: 200,
        fit: BoxFit.contain,
        errorBuilder: (c, e, s) => _NoImagePlaceholder(
          text: isEmf ? 'EMF vorhanden, PNG-Konvertierung fehlgeschlagen' : 'Bild nicht verfügbar',
        ),
      ),
    );
  }
}

class _NoImagePlaceholder extends StatelessWidget {
  const _NoImagePlaceholder({required this.text});
  final String text;
  @override
  Widget build(BuildContext context) {
    final bg = Theme.of(context).colorScheme.surfaceContainerHighest.withValues(alpha: 0.5);
    final fg = Theme.of(context).colorScheme.onSurfaceVariant;
    return Container(
      width: 200,
      height: 200,
      decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(6)),
      alignment: Alignment.center,
      child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
        Icon(Icons.image_not_supported_outlined, color: fg),
        const SizedBox(width: 8),
        Text(text, style: TextStyle(color: fg)),
      ]),
    );
  }
}

class _ProgressDialog extends StatelessWidget {
  const _ProgressDialog({required this.text});
  final String text;
  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      content: Row(children: [
        const SizedBox(width: 8),
        const CircularProgressIndicator(),
        const SizedBox(width: 16),
        Flexible(child: Text(text)),
      ]),
    );
  }
}

class _VariantTile extends StatefulWidget {
  const _VariantTile({required this.api, required this.projectId, required this.elevationId, required this.variant, required this.onChanged});
  final ApiClient api;
  final String projectId;
  final String elevationId;
  final Map<String, dynamic> variant;
  final Future<void> Function() onChanged;
  @override
  State<_VariantTile> createState() => _VariantTileState();
}

class _VariantTileState extends State<_VariantTile> {
  Map<String, dynamic>? _materials;
  String? _error;
  bool _saving = false;
  // Helpers to robustly read imported keys with varying spelling/case/spacing
  String _normKey(String k) => k.toLowerCase().replaceAll(RegExp(r'[^a-z0-9]'), '');
  dynamic _looseGet(Map<String, dynamic> m, List<String> desired) {
    for (final entry in m.entries) {
      final nk = _normKey(entry.key);
      for (final d in desired) { if (nk == _normKey(d)) return entry.value; }
    }
    for (final c in ['properties','props','attributes','meta']) {
      final v = m[c];
      if (v is Map<String, dynamic>) {
        for (final entry in v.entries) {
          final nk = _normKey(entry.key);
          for (final d in desired) { if (nk == _normKey(d)) return entry.value; }
        }
      }
    }
    return null;
  }
  num? _numVal(dynamic v) {
    if (v == null) return null;
    if (v is num) return v;
    return num.tryParse(v.toString().trim());
  }
  Future<void> _load() async {
    try {
      final m = await widget.api.getVariantMaterials(widget.projectId, widget.variant['id'] as String);
      setState(() { _materials = m; _error = null; });
    } catch (e) { setState(() { _error = e.toString(); }); }
  }
  @override
  Widget build(BuildContext context) {
    final name = widget.variant['name']?.toString() ?? 'Variante';
    final menge = widget.variant['menge']?.toString() ?? '1';
    return ExpansionTile(
      leading: const Icon(Icons.category_rounded),
      title: Row(children: [
        Expanded(child: Text(name, overflow: TextOverflow.ellipsis)),
        IconButton(onPressed: _editVariant, icon: const Icon(Icons.edit_rounded), tooltip: 'Bearbeiten'),
        IconButton(onPressed: _deleteVariant, icon: const Icon(Icons.delete_outline_rounded), tooltip: 'Löschen'),
      ]),
      subtitle: Text('Menge $menge'),
      onExpansionChanged: (open) { if (open && _materials == null) { _load(); } },
      children: [
        if (_error != null) Padding(padding: const EdgeInsets.all(12), child: Text('Fehler: $_error')),
        if (_materials == null) const Padding(padding: EdgeInsets.all(12), child: CircularProgressIndicator()),
        if (_materials != null) Padding(
          padding: const EdgeInsets.only(left: 12, right: 12, bottom: 12),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            const Text('Profile', style: TextStyle(fontWeight: FontWeight.bold)),
            const SizedBox(height: 6),
            ...List<Widget>.from(((_materials!['profiles'] ?? []) as List).map((p0) {
              final p = p0 as Map<String, dynamic>;
              _ensureComputedQtyForProfile(p);
              final lenVal = _looseGet(p, ['Lenght_Output','Length_Output','length_output','Lenght','Length','length_mm']);
              final lenUnit = (_looseGet(p, ['Lenght_Unit','Length_Unit','length_unit']) ?? 'mm').toString();
              final lenStr = _fmt4(lenVal);
              final qtyStr = _qtyDisplayForProfile(p);
              final linkedStr = ((p)['material_nummer'] ?? '').toString().isNotEmpty ? '  •  verknüpft: '+(p)['material_nummer'] : '';
              return ListTile(
                dense: true,
                leading: const Icon(Icons.straighten_rounded),
                title: Text(((p)['article_code'] ?? (p)['description'] ?? '').toString()),
                subtitle: Text('Länge $lenStr $lenUnit, Menge $qtyStr$linkedStr'),
                trailing: _buildMaterialActions(p, 'profiles'),
              );
            })),
            const SizedBox(height: 12),
            const Text('Artikel', style: TextStyle(fontWeight: FontWeight.bold)),
            const SizedBox(height: 6),
            ...List<Widget>.from(((_materials!['articles'] ?? []) as List).map((a0) {
              final a = a0 as Map<String, dynamic>;
              final materialLink = ((a)['material_nummer'] ?? '').toString().isNotEmpty ? '  •  verknüpft: '+(a)['material_nummer'] : '';
              return ListTile(
                dense: true,
                leading: const Icon(Icons.extension_rounded),
                title: Text(((a)['article_code'] ?? (a)['description'] ?? '').toString()),
                subtitle: Text('Menge ${(a)['qty'] ?? 1} ${(a)['unit'] ?? ''}$materialLink'),
                trailing: _buildMaterialActions(a, 'articles'),
              );
            })),
            const SizedBox(height: 12),
            const Text('Glas', style: TextStyle(fontWeight: FontWeight.bold)),
            const SizedBox(height: 6),
            ...List<Widget>.from(((_materials!['glass'] ?? []) as List).map((g0) {
              final g = g0 as Map<String, dynamic>;
              final materialLink = ((g)['material_nummer'] ?? '').toString().isNotEmpty ? '  •  verknüpft: '+(g)['material_nummer'] : '';
              return ListTile(
                dense: true,
                leading: const Icon(Icons.window_rounded),
                title: Text(((g)['configuration'] ?? (g)['description'] ?? '').toString()),
                subtitle: Text('Menge ${(g)['qty'] ?? 1}$materialLink'),
                trailing: _buildMaterialActions(g, 'glass'),
              );
            })),
          ]),
        ),
      ],
    );
  }

  Future<void> _editVariant() async {
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _EditDialog(title: 'Variante bearbeiten', fields: const ['name','beschreibung','menge','external_guid'], initial: widget.variant),
    );
    if (res == null) return;
    try { await widget.api.updateVariant(widget.projectId, widget.elevationId, widget.variant['id'] as String, res); await widget.onChanged(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Speichern fehlgeschlagen: $e'))); }
  }
  Future<void> _deleteVariant() async {
    final ok = await showDialog<bool>(context: context, builder: (_) => _ConfirmDialog(text: 'Variante wirklich löschen?'));
    if (ok != true) return;
    try { await widget.api.deleteVariant(widget.projectId, widget.elevationId, widget.variant['id'] as String); await widget.onChanged(); }
    catch (e) { if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Löschen fehlgeschlagen: $e'))); }
  }

  String _fmt4(dynamic v) {
    if (v == null) return '-';
    num? n;
    if (v is num) n = v;
    else { n = num.tryParse(v.toString()); }
    if (n == null) return '-';
    final d = n.toDouble();
    String s = d.toStringAsFixed(4);
    // remove trailing zeros and dot
    while (s.contains('.') && s.endsWith('0')) { s = s.substring(0, s.length - 1); }
    if (s.endsWith('.')) s = s.substring(0, s.length - 1);
    return s;
  }

  void _ensureComputedQtyForProfile(Map<String, dynamic> it) async {
    // Compute only if linked and not yet computed
    final mid = (it['material_id'] ?? '').toString();
    if (mid.isEmpty) return;
    if (it['__qty_computed'] == true || it['__qty_loading'] == true) return;
    it['__qty_loading'] = true;
    try {
      final mat = await widget.api.getMaterial(mid);
      num? baseLen = _numVal(mat['length_mm']);
      baseLen ??= 6000; // Default 6000 mm
      final dynLen = _looseGet(it, ['Lenght_Output','Length_Output','length_output','Lenght','Length','length_mm']);
      final len = (_numVal(dynLen) ?? 0).toDouble();
      final qty = baseLen > 0 ? (len / baseLen) : 0.0;
      it['__computed_qty'] = qty;
      it['__qty_computed'] = true;
      if (mounted) setState(() {});
    } catch (_) {
      // ignore errors; keep defaults
    } finally {
      it['__qty_loading'] = false;
    }
  }

  String _qtyDisplayForProfile(Map<String, dynamic> it) {
    final mid = (it['material_id'] ?? '').toString();
    if (mid.isNotEmpty) {
      final q = it['__computed_qty'];
      if (q is num) return '${_fmt4(q)} Stk.';
      if (it['__qty_loading'] == true) return '…';
      // linked but failed to compute; fall back
    }
    final q = it['qty'];
    final u = (it['unit'] ?? '').toString();
    if (q is num) return '${_fmt4(q)} ${u.isNotEmpty ? u : ''}'.trim();
    return '${q ?? 1} ${u.isNotEmpty ? u : ''}'.trim();
  }

  Widget _buildMaterialActions(Map<String, dynamic> it, String kind) {
    final linked = (it['material_id'] as String?)?.isNotEmpty == true;
    if (!linked) {
      return OutlinedButton.icon(onPressed: _saving ? null : () => _adoptMaterial(it, kind), icon: const Icon(Icons.download_done_rounded), label: const Text('Übernehmen'));
    }
    return Wrap(spacing: 6, children: [
      OutlinedButton(onPressed: _saving ? null : () => _changeLink(it, kind), child: const Text('Ändern')),
      OutlinedButton(onPressed: _saving ? null : () => _unlink(it, kind), child: const Text('Lösen')),
    ]);
  }

  Future<void> _unlink(Map<String, dynamic> it, String kind) async {
    setState(() { _saving = true; });
    try {
      await widget.api.linkVariantMaterial(widget.projectId, widget.variant['id'] as String, kind, it['id'] as String, '');
      if (!mounted) return; await _load();
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Verknüpfung gelöst')));
    } catch (e) {
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Lösen fehlgeschlagen: $e')));
    } finally {
      if (mounted) setState(() { _saving = false; });
    }
  }

  Future<void> _changeLink(Map<String, dynamic> it, String kind) async {
    // Suche nach Kandidaten anhand aktueller Nummer/Bezeichnung
    final prop = _defaultsForMaterial(it, kind);
    final existing = await widget.api.listMaterials(q: prop['nummer'] as String, limit: 50);
    final choice = await showDialog<_MaterialChoice>(
      context: context,
      builder: (ctx) => _MaterialSelectDialog(candidates: existing.cast<Map<String, dynamic>>(), proposed: prop),
    );
    if (choice == null) return;
    setState(() { _saving = true; });
    try {
      if (choice.useExisting && choice.materialId != null) {
        await widget.api.linkVariantMaterial(widget.projectId, widget.variant['id'] as String, kind, it['id'] as String, choice.materialId!);
      } else if (choice.proposed != null) {
        final body = { ...prop, ...choice.proposed! };
        final created = await widget.api.createMaterial(body);
        final mid = (created['id'] ?? '').toString();
        if (mid.isNotEmpty) {
          await widget.api.linkVariantMaterial(widget.projectId, widget.variant['id'] as String, kind, it['id'] as String, mid);
        }
      }
      if (!mounted) return; await _load();
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Verknüpfung aktualisiert')));
    } catch (e) {
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Aktualisierung fehlgeschlagen: $e')));
    } finally {
      if (mounted) setState(() { _saving = false; });
    }
  }

  Future<void> _adoptMaterial(Map<String, dynamic> it, String kind) async {
    final def = _defaultsForMaterial(it, kind);
    final res = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _EditDialog(
        title: 'Material übernehmen',
        fields: const ['nummer','bezeichnung','typ','einheit','kategorie','norm','werkstoffnummer','dichte'],
        initial: def,
      ),
    );
    if (res == null) return;
    // Sanitize types
    final body = <String, dynamic>{
      'nummer': (res['nummer'] ?? def['nummer']).toString().trim(),
      'bezeichnung': (res['bezeichnung'] ?? def['bezeichnung']).toString().trim(),
      'typ': (res['typ'] ?? def['typ']).toString().trim(),
      'einheit': (res['einheit'] ?? def['einheit']).toString().trim(),
      if ((res['kategorie'] ?? '').toString().trim().isNotEmpty) 'kategorie': res['kategorie'].toString().trim(),
      if ((res['norm'] ?? '').toString().trim().isNotEmpty) 'norm': res['norm'].toString().trim(),
      if ((res['werkstoffnummer'] ?? '').toString().trim().isNotEmpty) 'werkstoffnummer': res['werkstoffnummer'].toString().trim(),
      'dichte': double.tryParse('${res['dichte'] ?? 0}') ?? 0,
      'attribute': <String, dynamic>{
        'source': 'logikal-import',
        'variant_id': widget.variant['id'],
        'kind': kind,
      },
    };
    if (body['nummer'].toString().isEmpty || body['bezeichnung'].toString().isEmpty) {
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Nummer und Bezeichnung sind erforderlich'))); return;
    }
    setState(() { _saving = true; });
    try {
      // Duplikat-Check und Auswahl vorhandener Materialien
      final existing = await widget.api.listMaterials(q: body['nummer'] as String, limit: 50);
      final matches = existing.where((e) {
        final m = (e as Map<String, dynamic>);
        final numA = (m['nummer']?.toString() ?? '').trim().toLowerCase();
        final numB = (body['nummer'] as String).trim().toLowerCase();
        return numA.contains(numB) || numB.contains(numA);
      }).toList();

      if (matches.isNotEmpty) {
        // Dialog: vorhandenes Material auswählen oder trotzdem neu anlegen
        final choice = await showDialog<_MaterialChoice>(
          context: context,
          builder: (ctx) => _MaterialSelectDialog(candidates: matches.cast<Map<String, dynamic>>(), proposed: body),
        );
        if (choice == null) { return; }
        if (choice.useExisting && choice.materialId != null) {
          // Link setzen
          try {
            await widget.api.linkVariantMaterial(widget.projectId, widget.variant['id'] as String, kind, it['id'] as String, choice.materialId!);
            if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Vorhandenes Material verknüpft')));
            await _load();
            return;
          } catch (e) {
            if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Verknüpfung fehlgeschlagen: $e')));
            return;
          }
        }
        // Ansonsten: Neu anlegen mit ggf. angepasster Nummer
        if (choice.proposed != null) {
          for (final k in choice.proposed!.keys) { body[k] = choice.proposed![k]; }
        }
      }

      final created = await widget.api.createMaterial(body);
      final mid = (created['id'] ?? '').toString();
      if (mid.isNotEmpty) {
        await widget.api.linkVariantMaterial(widget.projectId, widget.variant['id'] as String, kind, it['id'] as String, mid);
        if (!mounted) return; await _load();
      }
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Material übernommen und verknüpft')));
    } catch (e) {
      if (!mounted) return; ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Übernahme fehlgeschlagen: $e')));
    } finally {
      if (mounted) setState(() { _saving = false; });
    }
  }

  Map<String, dynamic> _defaultsForMaterial(Map<String, dynamic> it, String kind) {
    String nummer = '';
    String bezeichnung = '';
    String typ = 'artikel';
    String einheit = 'stk';
    String kategorie = 'import';
    switch (kind) {
      case 'profiles':
        final supplier = (it['supplier_code'] ?? '').toString().trim();
        final code = (it['article_code'] ?? '').toString().trim();
        final desc = (it['description'] ?? '').toString().trim();
        nummer = [supplier, code].where((s) => s.isNotEmpty).join('-');
        if (nummer.isEmpty) nummer = code.isNotEmpty ? code : (desc.isNotEmpty ? desc : 'PROFIL');
        bezeichnung = desc.isNotEmpty ? desc : (code.isNotEmpty ? code : 'Profil');
        typ = 'profil';
        einheit = (it['unit'] ?? '').toString().trim().isNotEmpty ? it['unit'] : 'stk';
        break;
      case 'articles':
        final supplier = (it['supplier_code'] ?? '').toString().trim();
        final code = (it['article_code'] ?? '').toString().trim();
        final desc = (it['description'] ?? '').toString().trim();
        nummer = [supplier, code].where((s) => s.isNotEmpty).join('-');
        if (nummer.isEmpty) nummer = code.isNotEmpty ? code : (desc.isNotEmpty ? desc : 'ARTIKEL');
        bezeichnung = desc.isNotEmpty ? desc : (code.isNotEmpty ? code : 'Artikel');
        typ = 'artikel';
        einheit = (it['unit'] ?? '').toString().trim().isNotEmpty ? it['unit'] : 'stk';
        break;
      case 'glass':
        final conf = (it['configuration'] ?? '').toString().trim();
        final desc = (it['description'] ?? '').toString().trim();
        nummer = conf.isNotEmpty ? 'GLAS-$conf' : (desc.isNotEmpty ? 'GLAS-$desc' : 'GLAS');
        bezeichnung = desc.isNotEmpty ? desc : (conf.isNotEmpty ? conf : 'Glas');
        typ = 'glas';
        einheit = (it['unit'] ?? '').toString().trim().isNotEmpty ? it['unit'] : 'stk';
        break;
    }
    return {
      'nummer': nummer,
      'bezeichnung': bezeichnung,
      'typ': typ,
      'einheit': einheit,
      'kategorie': kategorie,
    };
  }
}

// ----- Auswahl-Dialog für vorhandenes Material oder Neuanlage -----
class _MaterialChoice {
  _MaterialChoice.existing(this.materialId) : useExisting = true, proposed = null;
  _MaterialChoice.create(this.proposed) : useExisting = false, materialId = null;
  final bool useExisting;
  final String? materialId;
  final Map<String, dynamic>? proposed; // optional geänderte Felder für Neuanlage
}

class _MaterialSelectDialog extends StatefulWidget {
  const _MaterialSelectDialog({required this.candidates, required this.proposed});
  final List<Map<String, dynamic>> candidates;
  final Map<String, dynamic> proposed;
  @override
  State<_MaterialSelectDialog> createState() => _MaterialSelectDialogState();
}

class _MaterialSelectDialogState extends State<_MaterialSelectDialog> {
  final nummerCtrl = TextEditingController();
  @override
  void initState() {
    super.initState();
    nummerCtrl.text = widget.proposed['nummer']?.toString() ?? '';
  }
  @override
  void dispose() { nummerCtrl.dispose(); super.dispose(); }
  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Material vorhanden?'),
      content: SizedBox(
        width: 520,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Es wurden ähnliche Materialien gefunden:'),
            const SizedBox(height: 8),
            SizedBox(
              height: 200,
              child: ListView.builder(
                itemCount: widget.candidates.length,
                itemBuilder: (context, index) {
                  final m = widget.candidates[index];
                  return ListTile(
                    dense: true,
                    leading: const Icon(Icons.inventory_2_outlined),
                    title: Text('${m['nummer']} — ${m['bezeichnung']}'),
                    subtitle: Text([m['typ'], m['einheit'], m['kategorie']].whereType<String>().where((s)=>s.isNotEmpty).join(' • ')),
                    onTap: () => Navigator.pop(context, _MaterialChoice.existing(m['id'] as String)),
                  );
                },
              ),
            ),
            const SizedBox(height: 12),
            const Text('Oder neues Material anlegen mit Nummer:'),
            TextField(controller: nummerCtrl, decoration: const InputDecoration(labelText: 'Nummer')),
          ],
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: const Text('Abbrechen')),
        FilledButton.icon(onPressed: () => Navigator.pop(context, _MaterialChoice.create({'nummer': nummerCtrl.text.trim()})), icon: const Icon(Icons.add), label: const Text('Neu anlegen')),
      ],
    );
  }
}

class ImportLogTab extends StatefulWidget {
  const ImportLogTab({super.key, required this.api, required this.projectId});
  final ApiClient api;
  final String projectId;
  @override
  State<ImportLogTab> createState() => _ImportLogTabState();
}

class _ImportLogTabState extends State<ImportLogTab> {
  bool _loading = true;
  String? _error;
  List<dynamic> _imports = const [];
  String _filterKind = '';
  String _filterAction = '';
  // Summary über Löschungen im letzten Import
  bool _hasDeletions = false;
  int _delPhases = 0;
  int _delElevs = 0;
  int _delVars = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  String _buildDeletionSummary() {
    final parts = <String>[];
    if (_delPhases > 0) parts.add('Lose: $_delPhases');
    if (_delElevs > 0) parts.add('Positionen: $_delElevs');
    if (_delVars > 0) parts.add('Varianten: $_delVars');
    if (parts.isEmpty) return 'Keine Details verfügbar.';
    return parts.join(' • ');
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final list = await widget.api.listProjectImports(widget.projectId);
      // Standard: zurücksetzen
      bool hasDel = false; int dP = 0, dE = 0, dV = 0;
      if (list.isNotEmpty) {
        // Änderungen des letzten Imports laden und nach "deleted" zählen
        final last = (list.first as Map<String, dynamic>);
        try {
          final changes = await widget.api.listImportChanges(widget.projectId, last['id'] as String);
          for (final c in changes) {
            final m = (c as Map).cast<String, dynamic>();
            if ((m['action'] ?? '') == 'deleted') {
              hasDel = true;
              final k = (m['kind'] ?? '') as String;
              if (k == 'phase') dP++;
              else if (k == 'elevation') dE++;
              else if (k == 'variant') dV++;
            }
          }
        } catch (_) {
          // ignorieren – Anzeige nur best-effort
        }
      }
      setState(() {
        _imports = list;
        _hasDeletions = hasDel; _delPhases = dP; _delElevs = dE; _delVars = dV;
      });
    } catch (e) { setState(() { _error = e.toString(); }); }
    finally { if (mounted) setState(() { _loading = false; }); }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) return Center(child: Text('Fehler: $_error'));
    if (_imports.isEmpty) return const Center(child: Text('Noch keine Importe.'));
    return Column(children: [
      if (_hasDeletions)
        Padding(
          padding: const EdgeInsets.fromLTRB(8, 8, 8, 0),
          child: Material(
            color: Colors.amber.shade50,
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8), side: BorderSide(color: Colors.amber.shade200)),
            child: ListTile(
              leading: Icon(Icons.info_outline_rounded, color: Colors.amber.shade800),
              title: const Text('Beim letzten Import wurden Einträge gelöscht'),
              subtitle: Text(_buildDeletionSummary()),
              dense: true,
            ),
          ),
        ),
      Padding(
        padding: const EdgeInsets.all(8.0),
        child: Row(children: [
          const Text('Filter:  '),
          DropdownButton<String>(value: _filterKind.isEmpty ? null : _filterKind, hint: const Text('Kind'), items: const [
            DropdownMenuItem(value: 'phase', child: Text('phase')),
            DropdownMenuItem(value: 'elevation', child: Text('elevation')),
            DropdownMenuItem(value: 'variant', child: Text('variant')),
            DropdownMenuItem(value: 'materials', child: Text('materials')),
          ], onChanged: (v) { setState(()=> _filterKind = v??''); }),
          const SizedBox(width: 12),
          DropdownButton<String>(value: _filterAction.isEmpty ? null : _filterAction, hint: const Text('Action'), items: const [
            DropdownMenuItem(value: 'created', child: Text('created')),
            DropdownMenuItem(value: 'updated', child: Text('updated')),
            DropdownMenuItem(value: 'deleted', child: Text('deleted')),
            DropdownMenuItem(value: 'replaced', child: Text('replaced')),
          ], onChanged: (v) { setState(()=> _filterAction = v??''); }),
        ]),
      ),
      const Divider(height: 1),
      Expanded(
        child: ListView.separated(
          itemCount: _imports.length,
          separatorBuilder: (_, __) => const Divider(height: 1),
          itemBuilder: (context, index) {
            final imp = _imports[index] as Map<String, dynamic>;
            return _ImportTile(api: widget.api, projectId: widget.projectId, importRun: imp, filterKind: _filterKind, filterAction: _filterAction, onUndo: _load);
          },
        ),
      ),
    ]);
  }
}

class _ImportTile extends StatefulWidget {
  const _ImportTile({required this.api, required this.projectId, required this.importRun, this.filterKind = '', this.filterAction = '', this.onUndo});
  final ApiClient api;
  final String projectId;
  final Map<String, dynamic> importRun;
  final String filterKind;
  final String filterAction;
  final Future<void> Function()? onUndo;
  @override
  State<_ImportTile> createState() => _ImportTileState();
}

class _ImportTileState extends State<_ImportTile> {
  List<dynamic>? _changes;
  String? _error;

  Future<void> _load() async {
    try {
      final changes = await widget.api.listImportChanges(widget.projectId, widget.importRun['id'] as String);
      // client-side filter fallback (server filter also available via query params; for simplicity using client filter)
      List<dynamic> list = changes;
      if (widget.filterKind.isNotEmpty) { list = list.where((e) => (e as Map<String, dynamic>)['kind'] == widget.filterKind).toList(); }
      if (widget.filterAction.isNotEmpty) { list = list.where((e) => (e as Map<String, dynamic>)['action'] == widget.filterAction).toList(); }
      setState(() { _changes = list; _error = null; });
    } catch (e) { setState(() { _error = e.toString(); }); }
  }

  @override
  Widget build(BuildContext context) {
    final r = widget.importRun;
    final title = r['source']?.toString().isNotEmpty == true ? r['source'].toString() : 'Import';
    final subtitle = 'am ${r['imported_at']} | Phases +${r['created_phases']}/${r['updated_phases']}, Elevations +${r['created_elevations']}/${r['updated_elevations']}, Variants +${r['created_variants']}/${r['updated_variants']}, -${r['deleted_variants']}, Mat ${r['materials_replaced_variants']}x';
    return ExpansionTile(
      title: Text(title),
      subtitle: Text(subtitle),
      onExpansionChanged: (open) { if (open && _changes == null) _load(); },
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8.0),
          child: Row(children: [
            TextButton.icon(onPressed: _downloadCSV, icon: const Icon(Icons.download_rounded), label: const Text('CSV')),
            const SizedBox(width: 8),
            TextButton.icon(onPressed: _downloadJSON, icon: const Icon(Icons.code_rounded), label: const Text('JSON')),
            const Spacer(),
            TextButton.icon(onPressed: _undo, icon: const Icon(Icons.undo_rounded), label: const Text('Rückgängig')),
          ]),
        ),
        if (_error != null) Padding(padding: const EdgeInsets.all(12), child: Text('Fehler: $_error')),
        if (_changes == null) const Padding(padding: EdgeInsets.all(12), child: CircularProgressIndicator()),
        if (_changes != null) ..._changes!.map((c) {
          final m = c as Map<String, dynamic>;
          final line = '[${m['kind']}/${m['action']}] ${m['message'] ?? ''}';
          return ExpansionTile(
            tilePadding: const EdgeInsets.symmetric(horizontal: 12),
            childrenPadding: const EdgeInsets.only(left: 16, right: 16, bottom: 12),
            title: Text(line),
            subtitle: (m['external_ref'] as String?)?.isNotEmpty == true ? Text('ext: ${m['external_ref']}') : null,
            children: [ _ChangeDiffView(change: m) ],
          );
        }),
      ],
    );
  }

  void _downloadCSV() {
    final url = widget.api.baseUrl + '/api/v1/projects/${widget.projectId}/imports/${widget.importRun['id']}/changes?format=csv';
    browser.downloadUrl(url, filename: 'import-${widget.importRun['id']}.csv');
  }
  void _downloadJSON() {
    final url = widget.api.baseUrl + '/api/v1/projects/${widget.projectId}/imports/${widget.importRun['id']}/changes';
    browser.downloadUrl(url, filename: 'import-${widget.importRun['id']}.json');
  }
  Future<void> _undo() async {
    final ok = await showDialog<bool>(context: context, builder: (_) => const _ConfirmDialog(text: 'Diesen Import wirklich rückgängig machen?'));
    if (ok != true) return;
    try {
      final uri = Uri.parse(widget.api.baseUrl + '/api/v1/projects/${widget.projectId}/imports/${widget.importRun['id']}/undo');
      final resp = await http.post(uri);
      if (resp.statusCode != 200) throw Exception('Fehler: ${resp.statusCode} ${resp.body}');
      if (widget.onUndo != null) await widget.onUndo!();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Import rückgängig gemacht.')));
      setState(() { _changes = null; });
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Rückgängig fehlgeschlagen: $e')));
    }
  }
}

class _ChangeDiffView extends StatelessWidget {
  const _ChangeDiffView({required this.change});
  final Map<String, dynamic> change;

  @override
  Widget build(BuildContext context) {
    final kind = (change['kind'] ?? '').toString();
    final before = (change['before_data'] as Map?)?.cast<String, dynamic>() ?? const {};
    final after = (change['after_data'] as Map?)?.cast<String, dynamic>() ?? const {};
    if (kind == 'materials') {
      return _buildMaterialsDiff(before, after);
    }
    return _buildMapDiff(before, after);
  }

  Widget _buildMapDiff(Map<String, dynamic> before, Map<String, dynamic> after) {
    final keys = <String>{...before.keys, ...after.keys}.toList()..sort();
    if (keys.isEmpty) return const Text('Keine Detaildaten.');
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      for (final k in keys)
        _diffRow(k, before[k], after[k]),
    ]);
  }

  Widget _diffRow(String key, dynamic bv, dynamic av) {
    final changed = _toStr(bv) != _toStr(av);
    final styleKey = TextStyle(fontWeight: changed ? FontWeight.bold : FontWeight.normal);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        SizedBox(width: 160, child: Text(key, style: styleKey)),
        Expanded(child: Text(_toStr(bv))),
        const Padding(padding: EdgeInsets.symmetric(horizontal: 6), child: Text('→')),
        Expanded(child: Text(_toStr(av))),
      ]),
    );
  }

  Widget _buildMaterialsDiff(Map<String, dynamic> before, Map<String, dynamic> after) {
    final bProfs = ((before['profiles'] ?? const []) as List).cast<dynamic>();
    final bArts = ((before['articles'] ?? const []) as List).cast<dynamic>();
    final bGlass = ((before['glass'] ?? const []) as List).cast<dynamic>();
    final aProfs = ((after['profiles'] ?? const []) as List).cast<dynamic>();
    final aArts = ((after['articles'] ?? const []) as List).cast<dynamic>();
    final aGlass = ((after['glass'] ?? const []) as List).cast<dynamic>();
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text('Materialliste', style: const TextStyle(fontWeight: FontWeight.bold)),
      const SizedBox(height: 6),
      _diffRow('Profile (Anzahl)', bProfs.length, aProfs.length),
      _diffRow('Artikel (Anzahl)', bArts.length, aArts.length),
      _diffRow('Glas (Anzahl)', bGlass.length, aGlass.length),
    ]);
  }

  String _toStr(dynamic v) {
    if (v == null) return '';
    if (v is String) return v;
    if (v is num || v is bool) return v.toString();
    try { return const JsonEncoder().convert(v); } catch (_) { return v.toString(); }
  }
}

// ---------- kleine generische Dialoge ----------
class _EditDialog extends StatefulWidget {
  const _EditDialog({required this.title, required this.fields, this.initial});
  final String title;
  final List<String> fields;
  final Map<String, dynamic>? initial;
  @override
  State<_EditDialog> createState() => _EditDialogState();
}
class _EditDialogState extends State<_EditDialog> {
  final Map<String, TextEditingController> ctrls = {};
  @override
  void initState() {
    super.initState();
    for (final f in widget.fields) {
      final v = widget.initial?[f]?.toString() ?? '';
      ctrls[f] = TextEditingController(text: v);
    }
  }
  @override
  void dispose() {
    for (final c in ctrls.values) { c.dispose(); }
    super.dispose();
  }
  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.title),
      content: SingleChildScrollView(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          for (final f in widget.fields)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: TextField(controller: ctrls[f], decoration: InputDecoration(labelText: f)),
            ),
        ]),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: const Text('Abbrechen')),
        FilledButton(onPressed: _save, child: const Text('Speichern')),
      ],
    );
  }
  void _save() {
    final map = <String, dynamic>{};
    for (final e in ctrls.entries) {
      final k = e.key; final t = e.value.text.trim();
      if (t.isEmpty) continue;
      // Zahlenfelder heuristisch konvertieren
      if (k == 'menge' || k.endsWith('_mm') || k == 'sort_order') {
        final numv = int.tryParse(t) ?? double.tryParse(t) ?? t;
        map[k] = numv;
      } else {
        map[k] = t;
      }
    }
    Navigator.pop(context, map);
  }
}

class _ConfirmDialog extends StatelessWidget {
  const _ConfirmDialog({required this.text});
  final String text;
  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      content: Text(text),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Abbrechen')),
        FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Löschen')),
      ],
    );
  }
}

class _ProjectCreateDialog extends StatefulWidget {
  const _ProjectCreateDialog({required this.api});
  final ApiClient api;
  @override
  State<_ProjectCreateDialog> createState() => _ProjectCreateDialogState();
}

class _ProjectCreateDialogState extends State<_ProjectCreateDialog> {
  final nameCtrl = TextEditingController();
  final nummerCtrl = TextEditingController();
  Map<String, dynamic>? _customer;

  @override
  void dispose() {
    nameCtrl.dispose(); nummerCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Projekt anlegen'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            TextField(controller: nameCtrl, decoration: const InputDecoration(labelText: 'Name*')),
            const SizedBox(height: 8),
            TextField(controller: nummerCtrl, decoration: const InputDecoration(labelText: 'Nummer (optional)')),
            const SizedBox(height: 12),
            Row(children: [
              const Text('Kunde:'),
              const SizedBox(width: 8),
              Expanded(child: Text(_customer != null ? (_customer!['name']?.toString() ?? '') : '— kein Kunde —', overflow: TextOverflow.ellipsis)),
              const SizedBox(width: 8),
              OutlinedButton.icon(onPressed: _pickCustomer, icon: const Icon(Icons.search_rounded), label: const Text('Auswählen')),
            ]),
          ],
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: const Text('Abbrechen')),
        FilledButton(onPressed: _save, child: const Text('Anlegen')),
      ],
    );
  }

  Future<void> _pickCustomer() async {
    final picked = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => _ContactPickerDialog(api: widget.api),
    );
    if (picked != null) setState(() { _customer = picked; });
  }

  void _save() {
    final name = nameCtrl.text.trim();
    final nummer = nummerCtrl.text.trim();
    if (name.isEmpty) { ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Name ist erforderlich'))); return; }
    Navigator.pop(context, {
      'name': name,
      if (nummer.isNotEmpty) 'nummer': nummer,
      if (_customer != null) 'kunde_id': _customer!['id'],
    });
  }
}

class _AnalysisDialog extends StatelessWidget {
  const _AnalysisDialog({required this.summary});
  final Map<String, dynamic> summary;
  @override
  Widget build(BuildContext context) {
    final project = (summary['project'] as Map?)?.cast<String, dynamic>() ?? const {};
    final phases = ((summary['phases'] ?? const []) as List).cast<dynamic>();
    return AlertDialog(
      title: const Text('Analyse Logikal'),
      content: SizedBox(
        width: 560,
        child: SingleChildScrollView(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text('Datei: ${summary['source'] ?? ''}'),
            const SizedBox(height: 6),
            Text('Projekt: ${project['name'] ?? ''}  |  OfferNo: ${project['offer_no'] ?? ''}  |  OrderNo: ${project['order_no'] ?? ''}'),
            const SizedBox(height: 12),
            Text('Phasen (Lose): ${summary['phase_count'] ?? phases.length}'),
            const SizedBox(height: 8),
            for (final p in phases)
              _PhaseAnalysisTile(phase: (p as Map).cast<String, dynamic>()),
            const SizedBox(height: 12),
            Text('${summary['notes'] ?? ''}', style: const TextStyle(color: Colors.black54, fontSize: 12)),
          ]),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: const Text('Schließen')),
      ],
    );
  }
}

class _PhaseAnalysisTile extends StatelessWidget {
  const _PhaseAnalysisTile({required this.phase});
  final Map<String, dynamic> phase;
  @override
  Widget build(BuildContext context) {
    final pid = phase['phase_id'];
    final name = (phase['name'] ?? '').toString();
    final elevs = phase['elevations'] ?? 0;
    final groups = ((phase['elevation_groups'] ?? const []) as List).cast<dynamic>();
    return ExpansionTile(
      title: Text('PhaseId $pid${name.isNotEmpty ? ' — '+name : ''}  –  Elevations: $elevs, Groups: ${groups.length}') ,
      children: [
        for (final g in groups)
          ListTile(
            dense: true,
            title: Text('Group ${ (g as Map)['group_id'] }'),
            subtitle: Text('Elevations: ${g['elevations']}, SingleElevations: ${g['single_elevations'] ?? 0}'),
          )
      ],
    );
  }
}

class _ContactPickerDialog extends StatefulWidget {
  const _ContactPickerDialog({required this.api});
  final ApiClient api;
  @override
  State<_ContactPickerDialog> createState() => _ContactPickerDialogState();
}

class _ContactPickerDialogState extends State<_ContactPickerDialog> {
  final qCtrl = TextEditingController();
  bool _loading = true; String? _error; List<dynamic> _items = const [];

  @override
  void initState() { super.initState(); _load(); }
  @override
  void dispose() { qCtrl.dispose(); super.dispose(); }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final list = await widget.api.listContacts(q: qCtrl.text.trim(), rolle: 'customer', limit: 50);
      setState(() { _items = list; });
    } catch (e) { setState(() { _error = e.toString(); }); }
    finally { if (mounted) setState(() { _loading = false; }); }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Kunde auswählen'),
      content: SizedBox(
        width: 500,
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          Row(children: [
            Expanded(child: TextField(controller: qCtrl, decoration: const InputDecoration(hintText: 'Suchen...'))),
            const SizedBox(width: 8),
            OutlinedButton.icon(onPressed: _load, icon: const Icon(Icons.search), label: const Text('Suchen')),
          ]),
          const SizedBox(height: 8),
          if (_loading) const Padding(padding: EdgeInsets.all(12), child: CircularProgressIndicator()),
          if (_error != null) Padding(padding: const EdgeInsets.all(12), child: Text('Fehler: $_error')),
          Flexible(
            child: ListView.builder(
              shrinkWrap: true,
              itemCount: _items.length,
              itemBuilder: (context, index) {
                final c = _items[index] as Map<String, dynamic>;
                final subtitle = [c['email'], c['telefon']].whereType<String>().where((s) => s.isNotEmpty).join(' • ');
                return ListTile(
                  leading: const Icon(Icons.person_outline_rounded),
                  title: Text(c['name']?.toString() ?? ''),
                  subtitle: subtitle.isNotEmpty ? Text(subtitle) : null,
                  onTap: () => Navigator.pop(context, c),
                );
              },
            ),
          ),
        ]),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: const Text('Schließen')),
      ],
    );
  }
}
