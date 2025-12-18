import 'package:flutter/material.dart';
import '../api.dart';

class EmployeesPage extends StatefulWidget {
  const EmployeesPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<EmployeesPage> createState() => _EmployeesPageState();
}

class _EmployeesPageState extends State<EmployeesPage> {
  List<dynamic> items = [];
  Map<String, dynamic>? selected;
  bool loading = false;
  int limit = 50;
  int offset = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => loading = true);
    try {
      final list = await widget.api.listEmployees(limit: limit, offset: offset);
      setState(() => items = list);
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> _createDialog() async {
    final firstCtrl = TextEditingController();
    final lastCtrl = TextEditingController();
    final emailCtrl = TextEditingController();
    final roleCtrl = TextEditingController();
    String? teamId;
    List<dynamic> teams = [];
    try {
      teams = await widget.api.listTeams();
    } catch (_) {}
    await showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Mitarbeiter anlegen'),
        content: SizedBox(
          width: 400,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                  controller: firstCtrl,
                  decoration: const InputDecoration(labelText: 'Vorname')),
              TextField(
                  controller: lastCtrl,
                  decoration: const InputDecoration(labelText: 'Nachname')),
              TextField(
                  controller: emailCtrl,
                  decoration: const InputDecoration(labelText: 'E-Mail')),
              TextField(
                  controller: roleCtrl,
                  decoration: const InputDecoration(labelText: 'Rolle')),
              DropdownButtonFormField<String>(
                initialValue: teamId,
                decoration: const InputDecoration(labelText: 'Team (optional)'),
                items: [
                  const DropdownMenuItem(value: null, child: Text('Kein Team')),
                  ...teams.map((t) {
                    final m = t as Map<String, dynamic>;
                    return DropdownMenuItem(
                        value: m['id'].toString(),
                        child:
                            Text(m['name']?.toString() ?? m['id'].toString()));
                  })
                ],
                onChanged: (v) => teamId = v,
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Abbrechen')),
          FilledButton(
              onPressed: () async {
                try {
                  final body = {
                    'first_name': firstCtrl.text.trim(),
                    'last_name': lastCtrl.text.trim(),
                    'email': emailCtrl.text.trim(),
                    'role': roleCtrl.text.trim(),
                    'team_id': teamId,
                    'active': true,
                  };
                  final emp = await widget.api.createEmployee(body);
                  if (mounted) {
                    Navigator.of(ctx).pop();
                    setState(() {
                      items.insert(0, emp);
                      selected = emp;
                    });
                  }
                } catch (e) {
                  if (mounted)
                    ScaffoldMessenger.of(context)
                        .showSnackBar(SnackBar(content: Text('Fehler: $e')));
                }
              },
              child: const Text('Anlegen')),
        ],
      ),
    );
  }

  Future<void> _loadDetail(String id) async {
    try {
      final emp = await widget.api.getEmployee(id);
      if (mounted) setState(() => selected = emp);
    } catch (_) {}
  }

  Future<void> _deactivateSelected() async {
    final id = selected?['id']?.toString();
    if (id == null) return;
    try {
      await widget.api.updateEmployee(id, {'active': false});
      await _loadDetail(id);
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Mitarbeiter deaktiviert')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _editSelected() async {
    final emp = selected;
    if (emp == null) return;
    final emailCtrl =
        TextEditingController(text: emp['email']?.toString() ?? '');
    final roleCtrl = TextEditingController(text: emp['role']?.toString() ?? '');
    String? teamId = emp['team_id']?.toString();
    List<dynamic> teams = [];
    try {
      teams = await widget.api.listTeams();
    } catch (_) {}
    await showDialog(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Mitarbeiter bearbeiten'),
          content: SizedBox(
            width: 400,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                    controller: emailCtrl,
                    decoration: const InputDecoration(labelText: 'E-Mail')),
                TextField(
                    controller: roleCtrl,
                    decoration: const InputDecoration(labelText: 'Rolle')),
                DropdownButtonFormField<String>(
                  initialValue: teamId,
                  decoration:
                      const InputDecoration(labelText: 'Team (optional)'),
                  items: [
                    const DropdownMenuItem(
                        value: null, child: Text('Kein Team')),
                    ...teams.map((t) {
                      final m = t as Map<String, dynamic>;
                      final id = m['id']?.toString();
                      return DropdownMenuItem(
                          value: id,
                          child: Text(m['name']?.toString() ?? id ?? ''));
                    })
                  ],
                  onChanged: (v) => setStateDialog(() => teamId = v),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.of(ctx).pop(),
                child: const Text('Abbrechen')),
            FilledButton(
                onPressed: () async {
                  try {
                    final patch = {
                      'email': emailCtrl.text.trim(),
                      'role': roleCtrl.text.trim(),
                      'team_id': teamId,
                    };
                    await widget.api
                        .updateEmployee(emp['id'].toString(), patch);
                    if (mounted) Navigator.of(ctx).pop();
                    await _loadDetail(emp['id'].toString());
                    await _load();
                  } catch (e) {
                    if (mounted)
                      ScaffoldMessenger.of(context)
                          .showSnackBar(SnackBar(content: Text('Fehler: $e')));
                  }
                },
                child: const Text('Speichern')),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final sel = selected;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Personal'),
        actions: [
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh))
        ],
      ),
      floatingActionButton: FloatingActionButton(
          onPressed: _createDialog, child: const Icon(Icons.add)),
      body: Row(
        children: [
          SizedBox(
            width: 360,
            child: Column(
              children: [
                if (loading) const LinearProgressIndicator(minHeight: 2),
                Expanded(
                  child: ListView.separated(
                    itemCount: items.length,
                    separatorBuilder: (_, __) => const Divider(height: 1),
                    itemBuilder: (ctx, i) {
                      final it = items[i] as Map<String, dynamic>;
                      final id = it['id']?.toString() ?? '';
                      final name =
                          '${it['first_name'] ?? ''} ${it['last_name'] ?? ''}'
                              .trim();
                      return ListTile(
                        selected: sel != null && sel['id'] == id,
                        title: Text(name.isEmpty ? id : name),
                        subtitle: Text(it['role']?.toString() ?? ''),
                        onTap: () {
                          setState(() => selected = it);
                          _loadDetail(id);
                        },
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
          const VerticalDivider(width: 1),
          Expanded(
            child: sel == null
                ? const Center(child: Text('Bitte Mitarbeiter auswaehlen'))
                : Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '${sel['first_name'] ?? ''} ${sel['last_name'] ?? ''}'
                              .trim(),
                          style: const TextStyle(
                              fontSize: 20, fontWeight: FontWeight.bold),
                        ),
                        const SizedBox(height: 6),
                        Text('E-Mail: ${sel['email'] ?? ''}'),
                        Text('Rolle: ${sel['role'] ?? ''}'),
                        Text('Team: ${sel['team_id'] ?? ''}'),
                        Text('Standort: ${sel['location'] ?? ''}'),
                        Text('Kostenstelle: ${sel['cost_center'] ?? ''}'),
                        const SizedBox(height: 12),
                        Row(
                          children: [
                            FilledButton.tonal(
                                onPressed: _editSelected,
                                child: const Text('Bearbeiten')),
                            const SizedBox(width: 8),
                            FilledButton.icon(
                                onPressed: (sel['active'] == false)
                                    ? null
                                    : _deactivateSelected,
                                icon: const Icon(Icons.remove_circle_outline),
                                style: FilledButton.styleFrom(
                                    backgroundColor: Colors.redAccent),
                                label: const Text('Deaktivieren')),
                          ],
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
