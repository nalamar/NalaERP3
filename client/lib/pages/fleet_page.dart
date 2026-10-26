import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;
import 'vehicle_detail_page.dart';

// Fuhrpark: Fahrzeugstamm (Backlog G.1.4) auf den Routen aus G.1.3 -
// zulassungspflichtige Fahrzeuge und Anhaenger anlegen, bearbeiten,
// abmelden. Maschinen ohne Zulassung (Stapler, Buehnen) sind
// Betriebsmittel. Lesen mit fleet.read, pflegen mit fleet.write - nur
// Fuhrpark und Administrator (Entscheidung des Nutzers vom 2026-10-25).

const vehicleArten = {
  'pkw': 'Pkw',
  'transporter': 'Transporter',
  'lkw': 'Lkw',
  'anhaenger': 'Anhänger',
};

String vehicleArtLabel(String? a) => vehicleArten[a] ?? (a ?? '');

String _two(int v) => v.toString().padLeft(2, '0');

// parseMasse liest die zulaessige Gesamtmasse in kg: leer = keine Angabe,
// sonst eine ganze Zahl > 0 (Tausenderpunkte erlaubt: "3.500").
({bool ok, int? kg}) parseMasse(String input) {
  final t = input.trim().replaceAll('.', '').replaceAll(' ', '');
  if (t.isEmpty) return (ok: true, kg: null);
  final v = int.tryParse(t);
  if (v == null || v <= 0) return (ok: false, kg: null);
  return (ok: true, kg: v);
}

// buildVehicleBody: immer alle zwoelf Felder - der Server verlangt sie
// beim PUT, und beim POST schadet die Vollstaendigkeit nicht. Leere FIN
// und Kostenstelle als null.
Map<String, dynamic> buildVehicleBody({
  required String kennzeichen,
  required String art,
  String bezeichnung = '',
  String hersteller = '',
  String modell = '',
  String fin = '',
  String? erstzulassung,
  int? zulaessigeGesamtmasseKg,
  String standort = '',
  String? kostenstelleId,
  bool fahrtenbuch = false,
  String notiz = '',
}) =>
    {
      'kennzeichen': kennzeichen.trim(),
      'art': art,
      'bezeichnung': bezeichnung.trim(),
      'hersteller': hersteller.trim(),
      'modell': modell.trim(),
      'fin': fin.trim().isEmpty ? null : fin.trim().toUpperCase(),
      'erstzulassung': erstzulassung,
      'zulaessige_gesamtmasse_kg': zulaessigeGesamtmasseKg,
      'standort': standort.trim(),
      'kostenstelle_id': (kostenstelleId == null || kostenstelleId.isEmpty) ? null : kostenstelleId,
      'fahrtenbuch': fahrtenbuch,
      'notiz': notiz.trim(),
    };

// vehicleDetails: die Zeile unter dem Kennzeichen.
String vehicleDetails(Map<String, dynamic> v) {
  final hm = [v['hersteller'], v['modell']].map((e) => e?.toString() ?? '').where((e) => e.isNotEmpty).join(' ');
  final ez = isoToGerman(v['erstzulassung']?.toString());
  return [
    vehicleArtLabel(v['art']?.toString()),
    if (hm.isNotEmpty) hm,
    if (ez.isNotEmpty) 'Erstzulassung $ez',
    if (v['fahrtenbuch'] == true) 'Fahrtenbuch',
    if (v['abgemeldet_am'] != null) 'abgemeldet am ${isoToGerman(v['abgemeldet_am']?.toString())} (${v['abgemeldet_grund'] ?? ''})',
  ].join(' · ');
}

class FleetPage extends StatefulWidget {
  const FleetPage({super.key, required this.api, this.now});
  final ApiClient api;
  final DateTime Function()? now;

  @override
  State<FleetPage> createState() => _FleetPageState();
}

class _FleetPageState extends State<FleetPage> {
  List<dynamic> vehicles = [];
  // Kostenstellen zur Auswahl; null = nicht ladbar (die Rolle Fuhrpark hat
  // ggf. kein cost_centers.read) - dann bleibt eine bestehende Zuordnung
  // erhalten (wie bei den Betriebsmitteln).
  List<dynamic>? costCenters;
  bool loading = false;
  String? error;
  String artFilter = '';
  bool showDeregistered = false;

  bool get _canWrite => widget.api.hasPermission('fleet.write');
  DateTime _now() => (widget.now ?? DateTime.now)();

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
      final list = await widget.api.listVehicles(art: artFilter, includeAbgemeldete: showDeregistered);
      List<dynamic>? cc = costCenters;
      if (_canWrite && cc == null) {
        try {
          cc = await widget.api.listCostCenters();
        } catch (_) {
          cc = null;
        }
      }
      if (!mounted) return;
      setState(() {
        vehicles = list;
        costCenters = cc;
      });
    } catch (e) {
      if (mounted) setState(() => error = 'Fahrzeuge konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _vehicleDialog({Map<String, dynamic>? vehicle}) async {
    final editing = vehicle != null;
    TextEditingController c(String key) => TextEditingController(text: vehicle?[key]?.toString() ?? '');
    final kennzeichenCtrl = c('kennzeichen');
    final bezCtrl = c('bezeichnung');
    final herstellerCtrl = c('hersteller');
    final modellCtrl = c('modell');
    final finCtrl = c('fin');
    final standortCtrl = c('standort');
    final notizCtrl = c('notiz');
    final ezCtrl = TextEditingController(text: isoToGerman(vehicle?['erstzulassung']?.toString()));
    final masseCtrl = c('zulaessige_gesamtmasse_kg');
    String art = vehicle?['art']?.toString() ?? 'pkw';
    bool fahrtenbuch = vehicle?['fahrtenbuch'] == true;
    String? kostenstelle = vehicle?['kostenstelle_id']?.toString();
    final ccList = costCenters;
    final ccKnown = ccList != null && (kostenstelle == null || ccList.any((x) => (x as Map<String, dynamic>)['id'] == kostenstelle));
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(editing ? 'Fahrzeug bearbeiten' : 'Neues Fahrzeug'),
          content: SizedBox(
            width: 480,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(controller: kennzeichenCtrl, decoration: const InputDecoration(labelText: 'Kennzeichen (z. B. M-AB 1234)')),
                  DropdownButtonFormField<String>(
                    initialValue: art,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Art'),
                    items: [for (final e in vehicleArten.entries) DropdownMenuItem(value: e.key, child: Text(e.value))],
                    onChanged: (v) => setStateDialog(() => art = v ?? art),
                  ),
                  TextField(controller: bezCtrl, decoration: const InputDecoration(labelText: 'Bezeichnung (z. B. Montagebus)')),
                  TextField(controller: herstellerCtrl, decoration: const InputDecoration(labelText: 'Hersteller')),
                  TextField(controller: modellCtrl, decoration: const InputDecoration(labelText: 'Modell')),
                  TextField(controller: finCtrl, decoration: const InputDecoration(labelText: 'FIN (17 Zeichen, optional)')),
                  TextField(controller: ezCtrl, decoration: const InputDecoration(labelText: 'Erstzulassung (TT.MM.JJJJ, optional)')),
                  TextField(controller: masseCtrl, decoration: const InputDecoration(labelText: 'Zulässige Gesamtmasse in kg (optional)')),
                  TextField(controller: standortCtrl, decoration: const InputDecoration(labelText: 'Standort')),
                  if (ccKnown)
                    DropdownButtonFormField<String?>(
                      initialValue: kostenstelle,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Kostenstelle (optional)'),
                      items: [
                        const DropdownMenuItem<String?>(value: null, child: Text('Keine')),
                        for (final x in ccList)
                          DropdownMenuItem<String?>(
                            value: (x as Map<String, dynamic>)['id'].toString(),
                            child: Text('${x['code'] ?? ''} – ${x['name'] ?? ''}'),
                          ),
                      ],
                      onChanged: (v) => setStateDialog(() => kostenstelle = v),
                    )
                  else if (kostenstelle != null)
                    const Padding(
                      padding: EdgeInsets.only(top: 8),
                      child: Text('Kostenstelle zugeordnet – sie bleibt beim Speichern erhalten.'),
                    ),
                  CheckboxListTile(
                    contentPadding: EdgeInsets.zero,
                    value: fahrtenbuch,
                    title: const Text('Fahrtenbuch führen'),
                    onChanged: (v) => setStateDialog(() => fahrtenbuch = v == true),
                  ),
                  TextField(controller: notizCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
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
              onPressed: () {
                final ezText = ezCtrl.text.trim();
                final ez = ezText.isEmpty ? null : parseDateInput(ezText);
                final masse = parseMasse(masseCtrl.text);
                String? problem;
                if (kennzeichenCtrl.text.trim().isEmpty) {
                  problem = 'Bitte ein Kennzeichen angeben.';
                } else if (ezText.isNotEmpty && ez == null) {
                  problem = 'Bitte die Erstzulassung als TT.MM.JJJJ angeben oder leer lassen.';
                } else if (!masse.ok) {
                  problem = 'Bitte die Gesamtmasse als ganze Zahl in kg angeben oder leer lassen.';
                }
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(buildVehicleBody(
                  kennzeichen: kennzeichenCtrl.text,
                  art: art,
                  bezeichnung: bezCtrl.text,
                  hersteller: herstellerCtrl.text,
                  modell: modellCtrl.text,
                  fin: finCtrl.text,
                  erstzulassung: ez,
                  zulaessigeGesamtmasseKg: masse.kg,
                  standort: standortCtrl.text,
                  kostenstelleId: kostenstelle,
                  fahrtenbuch: fahrtenbuch,
                  notiz: notizCtrl.text,
                ));
              },
              child: Text(editing ? 'Speichern' : 'Anlegen'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    try {
      if (editing) {
        await widget.api.updateVehicle(vehicle['id'].toString(), body);
        _snack('Fahrzeug gespeichert');
      } else {
        await widget.api.createVehicle(body);
        _snack('Fahrzeug angelegt');
      }
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _deregisterDialog(Map<String, dynamic> vehicle) async {
    final today = _now();
    final datumCtrl = TextEditingController(text: '${_two(today.day)}.${_two(today.month)}.${today.year}');
    final grundCtrl = TextEditingController();
    String? dialogError;
    final result = await showDialog<({String datum, String grund})>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Fahrzeug abmelden'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${vehicle['kennzeichen']}${(vehicle['bezeichnung']?.toString() ?? '').isEmpty ? '' : ' – ${vehicle['bezeichnung']}'}'),
                const SizedBox(height: 8),
                const Text('Abmelden ist endgültig. Das Kennzeichen ist danach wieder frei.'),
                TextField(controller: datumCtrl, decoration: const InputDecoration(labelText: 'Datum (TT.MM.JJJJ)')),
                TextField(controller: grundCtrl, decoration: const InputDecoration(labelText: 'Grund (z. B. verkauft)')),
                if (dialogError != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(dialogError!, style: const TextStyle(color: Color(0xFFB00020))),
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final datum = parseDateInput(datumCtrl.text);
                if (datum == null) {
                  setStateDialog(() => dialogError = 'Bitte das Datum als TT.MM.JJJJ angeben.');
                  return;
                }
                if (grundCtrl.text.trim().isEmpty) {
                  setStateDialog(() => dialogError = 'Bitte einen Grund angeben.');
                  return;
                }
                Navigator.of(ctx).pop((datum: datum, grund: grundCtrl.text.trim()));
              },
              child: const Text('Abmelden'),
            ),
          ],
        ),
      ),
    );
    if (result == null) return;
    try {
      await widget.api.deregisterVehicle(vehicle['id'].toString(), datum: result.datum, grund: result.grund);
      _snack('Fahrzeug abgemeldet');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Fuhrpark'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      floatingActionButton: _canWrite
          ? FloatingActionButton(onPressed: () => _vehicleDialog(), tooltip: 'Neues Fahrzeug', child: const Icon(Icons.add))
          : null,
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Wrap(
              spacing: 12,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                SizedBox(
                  width: 240,
                  child: DropdownButtonFormField<String>(
                    initialValue: artFilter,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Art'),
                    items: [
                      const DropdownMenuItem(value: '', child: Text('Alle')),
                      for (final e in vehicleArten.entries) DropdownMenuItem(value: e.key, child: Text(e.value)),
                    ],
                    onChanged: (v) {
                      setState(() => artFilter = v ?? '');
                      _load();
                    },
                  ),
                ),
                FilterChip(
                  label: const Text('Abgemeldete anzeigen'),
                  selected: showDeregistered,
                  onSelected: (v) {
                    setState(() => showDeregistered = v);
                    _load();
                  },
                ),
              ],
            ),
            const SizedBox(height: 8),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && vehicles.isEmpty) const Text('Keine Fahrzeuge gefunden.'),
            for (final v in vehicles)
              Builder(builder: (context) {
                final m = v as Map<String, dynamic>;
                final deregistered = m['abgemeldet_am'] != null;
                final bez = m['bezeichnung']?.toString() ?? '';
                return ListTile(
                  onTap: () async {
                    await Navigator.of(context).push(MaterialPageRoute(builder: (_) => VehicleDetailPage(api: widget.api, vehicle: m)));
                    if (mounted) _load();
                  },
                  title: Text('${m['kennzeichen'] ?? ''}${bez.isEmpty ? '' : ' – $bez'}'),
                  subtitle: Text(vehicleDetails(m)),
                  trailing: _canWrite
                      ? Wrap(
                          children: [
                            IconButton(tooltip: 'Bearbeiten', icon: const Icon(Icons.edit), onPressed: () => _vehicleDialog(vehicle: m)),
                            if (!deregistered)
                              IconButton(tooltip: 'Abmelden', icon: const Icon(Icons.no_crash_outlined), onPressed: () => _deregisterDialog(m)),
                          ],
                        )
                      : null,
                );
              }),
          ],
        ),
      ),
    );
  }
}
