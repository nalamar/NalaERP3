import 'dart:ui';
import 'package:flutter/material.dart';
import '../api.dart';
import 'employee_work_settings.dart';
import 'teams_page.dart';
import 'employee_qualifications_section.dart';
import 'employee_assets_section.dart';
import 'initial_qualifications_dialog.dart';
import 'my_time_page.dart';

// Mitarbeiterverwaltung (Backlog F.7.1) auf den Routen aus F.4.4:
// /api/v1/employees. Lesen mit employees.read, anlegen und aendern mit
// employees.write. Die Liste enthaelt auch inaktive (ausgeschiedene)
// Mitarbeiter; sie sind gekennzeichnet und koennen reaktiviert werden.
class EmployeesPage extends StatefulWidget {
  const EmployeesPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<EmployeesPage> createState() => _EmployeesPageState();
}

String employeeDisplayName(Map<String, dynamic> e) {
  final name = '${e['first_name'] ?? ''} ${e['last_name'] ?? ''}'.trim();
  return name.isEmpty ? (e['id']?.toString() ?? '') : name;
}

// buildEmployeeCreateBody baut den Body fuer POST /employees. Leere
// optionale Angaben werden weggelassen statt als "" gesendet; "active"
// fehlt bewusst - neue Mitarbeiter sind auf dem Server standardmaessig
// aktiv (ADR 0025).
Map<String, dynamic> buildEmployeeCreateBody({
  required String firstName,
  required String lastName,
  String personalNr = '',
  String email = '',
  String phone = '',
  String role = '',
  String? teamId,
}) {
  final body = <String, dynamic>{
    'first_name': firstName.trim(),
    'last_name': lastName.trim(),
  };
  void put(String key, String value) {
    if (value.trim().isNotEmpty) body[key] = value.trim();
  }

  put('personalnummer', personalNr);
  put('email', email);
  put('phone', phone);
  put('role', role);
  if (teamId != null && teamId.isNotEmpty) body['team_id'] = teamId;
  return body;
}

// freeLoginUsers sind die Logins, die noch mit keinem Mitarbeiter
// verknuepft sind - nur sie kommen fuer eine neue Verknuepfung in Frage.
List<Map<String, dynamic>> freeLoginUsers(List<dynamic> loginUsers) => [
      for (final u in loginUsers)
        if ((u as Map<String, dynamic>)['employee_id'] == null) u,
    ];

String loginUserLabel(Map<String, dynamic> u) {
  final name = u['name']?.toString().trim() ?? '';
  final email = u['email']?.toString() ?? '';
  return name.isEmpty ? email : '$name <$email>';
}

class _EmployeesPageState extends State<EmployeesPage> {
  List<dynamic> items = [];
  Map<String, String> teamNames = {};
  List<dynamic> teams = [];
  // Logins zur Verknuepfung - nur mit employees.write geladen.
  List<dynamic> loginUsers = [];
  Map<String, dynamic>? selected;
  bool loading = false;
  String? loadError;
  int limit = 50;
  int offset = 0;

  bool get _canWrite => widget.api.hasPermission('employees.write');

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      loadError = null;
    });
    try {
      final list = await widget.api.listEmployees(limit: limit, offset: offset);
      List<dynamic> loadedTeams = const [];
      try {
        loadedTeams = await widget.api.listTeams();
      } catch (_) {
        // Ohne Teams bleibt die Seite benutzbar; angezeigt wird dann die ID.
      }
      List<dynamic> loadedLogins = const [];
      if (_canWrite) {
        try {
          loadedLogins = await widget.api.listLoginUsers();
        } catch (_) {
          // Ohne die Liste bleibt die Seite benutzbar; Verknuepfen geht dann nicht.
        }
      }
      if (!mounted) return;
      setState(() {
        items = list;
        teams = loadedTeams;
        loginUsers = loadedLogins;
        teamNames = {
          for (final t in loadedTeams)
            (t as Map<String, dynamic>)['id'].toString():
                t['name']?.toString() ?? t['id'].toString(),
        };
      });
    } catch (e) {
      if (mounted) setState(() => loadError = 'Mitarbeiter konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _showError(Object e) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
  }

  List<DropdownMenuItem<String?>> _teamItems() => [
        const DropdownMenuItem<String?>(value: null, child: Text('Kein Team')),
        ...teams.map((t) {
          final m = t as Map<String, dynamic>;
          final id = m['id'].toString();
          return DropdownMenuItem<String?>(value: id, child: Text(m['name']?.toString() ?? id));
        }),
      ];

  Future<void> _createDialog() async {
    final personalNrCtrl = TextEditingController();
    final firstCtrl = TextEditingController();
    final lastCtrl = TextEditingController();
    final emailCtrl = TextEditingController();
    final phoneCtrl = TextEditingController();
    final roleCtrl = TextEditingController();
    String? teamId;
    // Anforderungen und vorhandene Nachweise gleich mit (F.7.3.3, F.10.5):
    // nur mit qualifications.write und nicht leerem Katalog.
    List<InitialQualificationEntry>? qualEntries;
    String? qualHint;
    List<dynamic>? positionChoices;
    String? positionHint;
    if (widget.api.hasPermission('qualifications.write')) {
      try {
        final types = await widget.api.listQualificationTypes();
        if (types.isNotEmpty) qualEntries = initialQualificationEntries(types);
      } catch (e) {
        qualHint = 'Qualifikationen konnten nicht geladen werden ($e). Der Mitarbeiter wird ohne sie angelegt; '
            'nachtragen im Abschnitt "Qualifikationen".';
      }
      // Stellen (F.11.6): nur die aktiven, nur wenn es welche gibt.
      try {
        final pos = await widget.api.listPositions();
        if (pos.isNotEmpty) positionChoices = pos;
      } catch (e) {
        positionHint = 'Stellen konnten nicht geladen werden ($e). Nachtragen im Abschnitt "Qualifikationen".';
      }
      if (!mounted) return;
    }
    final chosenPositions = <String>{};
    String? dialogError;
    String? summary;
    final created = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Mitarbeiter anlegen'),
          content: SizedBox(
            width: qualEntries == null && positionChoices == null ? 400 : 620,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(controller: personalNrCtrl, decoration: const InputDecoration(labelText: 'Personalnummer (optional)')),
                  TextField(controller: firstCtrl, decoration: const InputDecoration(labelText: 'Vorname')),
                  TextField(controller: lastCtrl, decoration: const InputDecoration(labelText: 'Nachname')),
                  TextField(controller: emailCtrl, decoration: const InputDecoration(labelText: 'E-Mail')),
                  TextField(controller: phoneCtrl, decoration: const InputDecoration(labelText: 'Telefon')),
                  TextField(controller: roleCtrl, decoration: const InputDecoration(labelText: 'Stelle / Rolle')),
                  DropdownButtonFormField<String?>(
                    initialValue: teamId,
                    decoration: const InputDecoration(labelText: 'Team (optional)'),
                    items: _teamItems(),
                    onChanged: (v) => setStateDialog(() => teamId = v),
                  ),
                  if (positionChoices != null)
                    InitialPositionFields(positions: positionChoices, selected: chosenPositions, onChanged: setStateDialog),
                  if (positionHint != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(positionHint),
                    ),
                  if (qualEntries != null) InitialQualificationFields(entries: qualEntries, onChanged: setStateDialog),
                  if (qualHint != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(qualHint),
                    ),
                  if (dialogError != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(dialogError!, style: const TextStyle(color: Color(0xFFB00020))),
                    ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () async {
                final body = buildEmployeeCreateBody(
                  firstName: firstCtrl.text,
                  lastName: lastCtrl.text,
                  personalNr: personalNrCtrl.text,
                  email: emailCtrl.text,
                  phone: phoneCtrl.text,
                  role: roleCtrl.text,
                  teamId: teamId,
                );
                final entries = qualEntries;
                // Der eine Aufruf, sobald Qualifikationen oder Stellen
                // gewaehlt werden koennen.
                InitialQualificationPlan? plan = positionChoices != null ? const InitialQualificationPlan() : null;
                if (entries != null) {
                  plan = buildInitialQualificationPlan(entries);
                  if (plan.error != null) {
                    setStateDialog(() => dialogError = plan!.error);
                    return;
                  }
                }
                setStateDialog(() => dialogError = null);
                try {
                  Map<String, dynamic> emp;
                  if (plan != null) {
                    final res = await widget.api
                        .createEmployeeWithQualifications(buildEmployeeWithQualificationsBody(body, plan, positionIds: chosenPositions));
                    emp = res['employee'] as Map<String, dynamic>;
                    summary = initialQualificationSummary(res);
                  } else {
                    emp = await widget.api.createEmployee(body);
                  }
                  if (ctx.mounted) Navigator.of(ctx).pop(emp);
                } catch (e) {
                  _showError(e);
                }
              },
              child: const Text('Anlegen'),
            ),
          ],
        ),
      ),
    );
    if (created == null || !mounted) return;
    if (summary != null) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(summary!)));
    setState(() => selected = created);
    await _load();
  }

  Future<void> _loadDetail(String id) async {
    try {
      final emp = await widget.api.getEmployee(id);
      if (mounted) setState(() => selected = emp);
    } catch (e) {
      _showError(e);
    }
  }

  Future<void> _setActive(bool active) async {
    final id = selected?['id']?.toString();
    if (id == null) return;
    try {
      final emp = await widget.api.updateEmployee(id, {'active': active});
      if (!mounted) return;
      setState(() => selected = emp);
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(active ? 'Mitarbeiter reaktiviert' : 'Mitarbeiter deaktiviert')));
      }
    } catch (e) {
      _showError(e);
    }
  }

  Future<void> _editSelected() async {
    final emp = selected;
    if (emp == null) return;
    final firstCtrl = TextEditingController(text: emp['first_name']?.toString() ?? '');
    final lastCtrl = TextEditingController(text: emp['last_name']?.toString() ?? '');
    final emailCtrl = TextEditingController(text: emp['email']?.toString() ?? '');
    final phoneCtrl = TextEditingController(text: emp['phone']?.toString() ?? '');
    final roleCtrl = TextEditingController(text: emp['role']?.toString() ?? '');
    final locationCtrl = TextEditingController(text: emp['location']?.toString() ?? '');
    final costCenterCtrl = TextEditingController(text: emp['cost_center']?.toString() ?? '');
    String? teamId = emp['team_id']?.toString();
    if (teamId != null && !teamNames.containsKey(teamId)) teamId = null;
    final updated = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Mitarbeiter bearbeiten'),
          content: SizedBox(
            width: 400,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  TextField(controller: firstCtrl, decoration: const InputDecoration(labelText: 'Vorname')),
                  TextField(controller: lastCtrl, decoration: const InputDecoration(labelText: 'Nachname')),
                  TextField(controller: emailCtrl, decoration: const InputDecoration(labelText: 'E-Mail')),
                  TextField(controller: phoneCtrl, decoration: const InputDecoration(labelText: 'Telefon')),
                  TextField(controller: roleCtrl, decoration: const InputDecoration(labelText: 'Stelle / Rolle')),
                  TextField(controller: locationCtrl, decoration: const InputDecoration(labelText: 'Standort')),
                  TextField(controller: costCenterCtrl, decoration: const InputDecoration(labelText: 'Kostenstelle')),
                  DropdownButtonFormField<String?>(
                    initialValue: teamId,
                    decoration: const InputDecoration(labelText: 'Team (optional)'),
                    items: _teamItems(),
                    onChanged: (v) => setStateDialog(() => teamId = v),
                  ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () async {
                try {
                  final result = await widget.api.updateEmployee(emp['id'].toString(), {
                    'first_name': firstCtrl.text.trim(),
                    'last_name': lastCtrl.text.trim(),
                    'email': emailCtrl.text.trim(),
                    'phone': phoneCtrl.text.trim(),
                    'role': roleCtrl.text.trim(),
                    'location': locationCtrl.text.trim(),
                    'cost_center': costCenterCtrl.text.trim(),
                    'team_id': teamId,
                  });
                  if (ctx.mounted) Navigator.of(ctx).pop(result);
                } catch (e) {
                  _showError(e);
                }
              },
              child: const Text('Speichern'),
            ),
          ],
        ),
      ),
    );
    if (updated == null || !mounted) return;
    setState(() => selected = updated);
    await _load();
  }

  Widget _glassPanel(Widget child, {EdgeInsets padding = const EdgeInsets.all(12)}) {
    final scheme = Theme.of(context).colorScheme;
    return ClipRRect(
      borderRadius: BorderRadius.circular(18),
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 14, sigmaY: 14),
        child: Container(
          padding: padding,
          decoration: BoxDecoration(
            gradient: LinearGradient(
              colors: [Colors.white.withValues(alpha: 0.12), Colors.white.withValues(alpha: 0.04)],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            borderRadius: BorderRadius.circular(18),
            border: Border.all(color: Colors.white.withValues(alpha: 0.14)),
            boxShadow: [
              BoxShadow(color: Colors.black.withValues(alpha: 0.25), blurRadius: 24, offset: const Offset(0, 16)),
              BoxShadow(color: scheme.primary.withValues(alpha: 0.16), blurRadius: 28, offset: const Offset(-10, -8)),
            ],
          ),
          child: child,
        ),
      ),
    );
  }

  String _loginLabel(Map<String, dynamic> e) {
    final userId = e['user_id']?.toString();
    if (userId == null || userId.isEmpty) return 'kein Login';
    for (final u in loginUsers) {
      final m = u as Map<String, dynamic>;
      if (m['user_id'] == userId) return loginUserLabel(m);
    }
    return 'verknüpft';
  }

  Future<void> _setLogin(String? userId) async {
    final id = selected?['id']?.toString();
    if (id == null) return;
    try {
      final emp = await widget.api.setEmployeeUser(id, userId);
      if (!mounted) return;
      setState(() => selected = emp);
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(userId == null ? 'Verknüpfung gelöst' : 'Login verknüpft')));
      }
    } catch (e) {
      _showError(e);
    }
  }

  Future<void> _linkDialog() async {
    final free = freeLoginUsers(loginUsers);
    if (free.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content: Text('Kein freier Login vorhanden. Logins legt die Benutzerverwaltung an.')));
      return;
    }
    String? chosen;
    final userId = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Login verknüpfen'),
          content: SizedBox(
            width: 400,
            child: DropdownButtonFormField<String>(
              initialValue: chosen,
              decoration: const InputDecoration(labelText: 'Login'),
              items: [
                for (final u in free)
                  DropdownMenuItem<String>(value: u['user_id'].toString(), child: Text(loginUserLabel(u))),
              ],
              onChanged: (v) => setStateDialog(() => chosen = v),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: chosen == null ? null : () => Navigator.of(ctx).pop(chosen),
              child: const Text('Verknüpfen'),
            ),
          ],
        ),
      ),
    );
    if (userId != null) await _setLogin(userId);
  }

  String _teamLabel(Map<String, dynamic> e) {
    final id = e['team_id']?.toString();
    if (id == null || id.isEmpty) return '';
    return teamNames[id] ?? id;
  }

  Widget _detail(Map<String, dynamic> sel) {
    final active = sel['active'] != false;
    final id = sel['id']?.toString() ?? '';
    return SingleChildScrollView(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(employeeDisplayName(sel), style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold)),
          const SizedBox(height: 6),
          Text('Status: ${active ? 'aktiv' : 'inaktiv'}'),
          Text('Personalnummer: ${sel['personalnummer'] ?? ''}'),
          Text('E-Mail: ${sel['email'] ?? ''}'),
          Text('Telefon: ${sel['phone'] ?? ''}'),
          Text('Stelle / Rolle: ${sel['role'] ?? ''}'),
          Text('Team: ${_teamLabel(sel)}'),
          Text('Standort: ${sel['location'] ?? ''}'),
          Text('Kostenstelle: ${sel['cost_center'] ?? ''}'),
          Text('Login: ${_loginLabel(sel)}'),
          // Arbeitstage und Urlaub (F.8.6.4.2).
          Text('Bundesland: ${employeeBundeslandLabel(sel)}'),
          Text('Arbeitstage: ${workdaysLabel(((sel['arbeitstage'] as List?) ?? const [1, 2, 3, 4, 5]).map((e) => (e as num).toInt()).toList())}'),
          Text('Jahresurlaub: ${leaveEntitlementLabel(sel)}'),
          const SizedBox(height: 12),
          if (widget.api.hasPermission('time_entries.read') && id.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: OutlinedButton.icon(
                onPressed: () => Navigator.of(context).push(MaterialPageRoute(
                  builder: (_) => MyTimePage(api: widget.api, employeeId: id, employeeName: employeeDisplayName(sel)),
                )),
                icon: const Icon(Icons.schedule),
                label: const Text('Arbeitszeiten'),
              ),
            ),
          if (_canWrite)
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                FilledButton.tonal(onPressed: _editSelected, child: const Text('Bearbeiten')),
                OutlinedButton(
                  onPressed: () async {
                    final updated = await showWorkSettingsDialog(context, widget.api, sel);
                    if (updated == null || !mounted) return;
                    setState(() => selected = updated);
                    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Arbeitstage und Urlaub gespeichert')));
                  },
                  child: const Text('Arbeitstage und Urlaub'),
                ),
                if (active)
                  FilledButton.icon(
                    onPressed: () => _setActive(false),
                    icon: const Icon(Icons.remove_circle_outline),
                    style: FilledButton.styleFrom(backgroundColor: Colors.redAccent),
                    label: const Text('Deaktivieren'),
                  )
                else
                  FilledButton.icon(
                    onPressed: () => _setActive(true),
                    icon: const Icon(Icons.restart_alt),
                    label: const Text('Reaktivieren'),
                  ),
                if (sel['user_id'] == null)
                  OutlinedButton.icon(
                    onPressed: _linkDialog,
                    icon: const Icon(Icons.link),
                    label: const Text('Login verknüpfen'),
                  )
                else
                  OutlinedButton.icon(
                    onPressed: () => _setLogin(null),
                    icon: const Icon(Icons.link_off),
                    label: const Text('Verknüpfung lösen'),
                  ),
              ],
            ),
          if (widget.api.hasPermission('qualifications.read') && id.isNotEmpty) ...[
            const Divider(height: 32),
            // Key je Mitarbeiter: beim Wechsel wird der Abschnitt neu
            // aufgebaut und laedt den Stand des neuen Mitarbeiters.
            EmployeeQualificationsSection(key: ValueKey('quals-$id'), api: widget.api, employeeId: id),
          ],
          if (widget.api.hasPermission('assets.read') && id.isNotEmpty) ...[
            const Divider(height: 32),
            EmployeeAssetsSection(key: ValueKey('assets-$id'), api: widget.api, employeeId: id),
          ],
        ],
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
          TextButton.icon(
            onPressed: () => Navigator.of(context).push(MaterialPageRoute(builder: (_) => TeamsPage(api: widget.api))).then((_) => _load()),
            icon: const Icon(Icons.groups_outlined),
            label: const Text('Teams'),
          ),
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden'),
        ],
        backgroundColor: Colors.transparent,
      ),
      floatingActionButton: _canWrite
          ? FloatingActionButton(onPressed: _createDialog, tooltip: 'Mitarbeiter anlegen', child: const Icon(Icons.add))
          : null,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
          child: Row(
            children: [
              Expanded(
                flex: 4,
                child: _glassPanel(
                  Column(
                    children: [
                      Row(
                        children: [
                          const Expanded(
                            child: Text('Mitarbeiter',
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                          ),
                          Text('${items.length} Einträge'),
                        ],
                      ),
                      if (loading) const LinearProgressIndicator(minHeight: 2),
                      if (loadError != null)
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 8),
                          child: Text(loadError!, style: const TextStyle(color: Color(0xFFB00020))),
                        ),
                      const SizedBox(height: 8),
                      Expanded(
                        child: ListView.separated(
                          itemCount: items.length,
                          separatorBuilder: (_, __) => const Divider(height: 1, color: Colors.white24),
                          itemBuilder: (ctx, i) {
                            final it = items[i] as Map<String, dynamic>;
                            final id = it['id']?.toString() ?? '';
                            final active = it['active'] != false;
                            final subtitle = [
                              if ((it['role']?.toString() ?? '').isNotEmpty) it['role'].toString(),
                              if (!active) 'inaktiv',
                            ].join(' · ');
                            return ListTile(
                              selected: sel != null && sel['id'] == id,
                              title: Text(employeeDisplayName(it)),
                              subtitle: subtitle.isEmpty ? null : Text(subtitle),
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
                  padding: const EdgeInsets.all(14),
                ),
              ),
              const SizedBox(width: 16),
              Expanded(
                flex: 7,
                child: _glassPanel(
                  sel == null ? const Center(child: Text('Bitte einen Mitarbeiter auswählen')) : _detail(sel),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
