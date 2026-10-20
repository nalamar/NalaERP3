import 'package:flutter/material.dart';
import '../api.dart';
import 'leave_admin_page.dart' show employeeNames;

// Teams pflegen (Backlog F.8.6.4.2), Routen aus F.8.5: anlegen,
// umbenennen, Teamleiter setzen, loeschen (nur ohne Mitglieder). Lesen mit
// employees.read, pflegen mit employees.write. Teams haben vorerst keine
// Wirkung auf Rechte - Urlaub genehmigen Personalabteilung und
// Geschaeftsfuehrung (ADR 0029); das Team dient der Abwesenheitsuebersicht.

// teamLine: "Montage · Teamleiter: Berg, Anna · 3 Mitglieder".
String teamLine(Map<String, dynamic> t, Map<String, String> names) {
  final parts = <String>[t['name']?.toString() ?? ''];
  final lead = t['lead_employee_id']?.toString();
  if (lead != null && lead.isNotEmpty) parts.add('Teamleiter: ${names[lead] ?? 'unbekannt'}');
  final n = (t['mitglieder'] as num?)?.toInt() ?? 0;
  parts.add(n == 1 ? '1 Mitglied' : '$n Mitglieder');
  return parts.join(' · ');
}

class TeamsPage extends StatefulWidget {
  const TeamsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<TeamsPage> createState() => _TeamsPageState();
}

class _TeamsPageState extends State<TeamsPage> {
  List<dynamic> teams = [];
  List<Map<String, dynamic>> employees = [];
  Map<String, String> names = {};
  bool loading = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('employees.write');

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
      final t = await widget.api.listTeams();
      final emps = await widget.api.listEmployees(limit: 500);
      if (!mounted) return;
      final all = employeeNames(emps);
      setState(() {
        teams = t;
        names = all;
        employees = emps.map((e) => e as Map<String, dynamic>).where((e) => e['active'] != false).toList()
          ..sort((a, b) => all[a['id'].toString()]!.compareTo(all[b['id'].toString()]!));
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => error = 'Teams konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted) setState(() => error = 'Teams konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  // _editDialog legt an (team == null) oder aendert.
  Future<void> _editDialog([Map<String, dynamic>? team]) async {
    final nameCtrl = TextEditingController(text: team?['name']?.toString() ?? '');
    String? lead = team?['lead_employee_id']?.toString();
    if (lead != null && !employees.any((e) => e['id'].toString() == lead)) lead = null;
    String? dialogError;
    final saved = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(team == null ? 'Team anlegen' : 'Team bearbeiten'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(controller: nameCtrl, decoration: const InputDecoration(labelText: 'Name')),
                DropdownButtonFormField<String?>(
                  initialValue: lead,
                  isExpanded: true,
                  decoration: const InputDecoration(labelText: 'Teamleiter (optional)'),
                  items: [
                    const DropdownMenuItem<String?>(value: null, child: Text('kein Teamleiter')),
                    for (final e in employees) DropdownMenuItem<String?>(value: e['id'].toString(), child: Text(names[e['id'].toString()] ?? '')),
                  ],
                  onChanged: (v) => setStateDialog(() => lead = v),
                ),
                if (dialogError != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(dialogError!, style: const TextStyle(color: Color(0xFFB00020))),
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () async {
                final name = nameCtrl.text.trim();
                if (name.isEmpty) {
                  setStateDialog(() => dialogError = 'Bitte einen Namen angeben.');
                  return;
                }
                final body = {'name': name, 'lead_employee_id': lead};
                try {
                  if (team == null) {
                    await widget.api.createTeam(body);
                  } else {
                    await widget.api.updateTeam(team['id'].toString(), body);
                  }
                  if (ctx.mounted) Navigator.of(ctx).pop(true);
                } on ApiException catch (e) {
                  setStateDialog(() => dialogError = e.message);
                } catch (e) {
                  setStateDialog(() => dialogError = '$e');
                }
              },
              child: Text(team == null ? 'Anlegen' : 'Speichern'),
            ),
          ],
        ),
      ),
    );
    if (saved != true) return;
    _snack(team == null ? 'Team angelegt' : 'Team gespeichert');
    await _load();
  }

  Future<void> _delete(Map<String, dynamic> team) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Team löschen'),
        content: Text('${team['name']} löschen? Das geht nur, wenn dem Team niemand mehr zugeordnet ist.'),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: const Text('Löschen')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.api.deleteTeam(team['id'].toString());
      _snack('Team gelöscht');
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Teams'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      floatingActionButton: _canWrite ? FloatingActionButton(onPressed: () => _editDialog(), tooltip: 'Team anlegen', child: const Icon(Icons.add)) : null,
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const Text('Teams gliedern die Abwesenheitsübersicht. Urlaub genehmigen weiterhin Personalabteilung und Geschäftsführung.'),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && teams.isEmpty) const Text('Noch keine Teams angelegt.'),
            for (final t in teams.cast<Map<String, dynamic>>())
              ListTile(
                title: Text(teamLine(t, names)),
                trailing: _canWrite
                    ? Wrap(
                        children: [
                          IconButton(tooltip: 'Bearbeiten', icon: const Icon(Icons.edit_outlined), onPressed: () => _editDialog(t)),
                          IconButton(tooltip: 'Löschen', icon: const Icon(Icons.delete_outline), onPressed: () => _delete(t)),
                        ],
                      )
                    : null,
              ),
          ],
        ),
      ),
    );
  }
}
