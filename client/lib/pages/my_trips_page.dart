import 'package:flutter/material.dart';
import '../api.dart';
import 'vehicle_trips_section.dart';

// "Meine Fahrten" (Backlog G.3c.4b2, Server G.3c.4b1): für Fahrer mit
// fleet.self. Erfassen nur für ein Fahrzeug mit Fahrtenbuch, das dem
// Fahrer gerade zugeordnet ist (ANNAHME: Poolfahrzeuge ohne Zuordnung
// trägt der Fuhrpark ein). Darunter alle eigenen Fahrten, auch an
// früheren Fahrzeugen. Immer nur die eigenen - auch für Benutzer mit
// fleet.read (nur_eigene).

// logbookAssignments: offene eigene Zuordnungen an Fahrzeugen mit
// Fahrtenbuch.
List<Map<String, dynamic>> logbookAssignments(List<dynamic> assignments) => [
      for (final a in assignments.cast<Map<String, dynamic>>())
        if (a['fahrtenbuch'] == true &&
            a['status'] == 'active' &&
            a['ende'] == null)
          a,
    ];

class MyTripsPage extends StatefulWidget {
  const MyTripsPage({super.key, required this.api, this.now});
  final ApiClient api;
  final DateTime Function()? now;

  @override
  State<MyTripsPage> createState() => _MyTripsPageState();
}

class _MyTripsPageState extends State<MyTripsPage> {
  List<Map<String, dynamic>> vehicles = [];
  List<dynamic> trips = [];
  bool loading = false;
  String? error;

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
      final results = await Future.wait<List<dynamic>>([
        widget.api.listOwnFleetAssignments(nurOffen: true),
        widget.api.listOwnFleetTrips(includeStorniert: true),
      ]);
      if (!mounted) return;
      setState(() {
        vehicles = logbookAssignments(results[0]);
        trips = results[1];
      });
    } on ApiException catch (e) {
      if (mounted)
        setState(
            () => error = 'Fahrten konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted)
        setState(() => error = 'Fahrten konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  // Nach dem Erfassen nur die Gesamtliste neu laden; die Abschnitte
  // aktualisieren sich selbst.
  Future<void> _reloadTrips() async {
    try {
      final list = await widget.api.listOwnFleetTrips(includeStorniert: true);
      if (mounted) setState(() => trips = list);
    } catch (_) {
      // Die Abschnitte zeigen den Stand; die Gesamtliste folgt beim nächsten Öffnen.
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Meine Fahrten')),
      body: loading && vehicles.isEmpty && trips.isEmpty
          ? const Center(child: CircularProgressIndicator())
          : error != null
              ? Center(child: Text(error!))
              : ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    if (vehicles.isEmpty)
                      const Text(
                          'Ihnen ist derzeit kein Fahrzeug mit Fahrtenbuch zugeordnet. '
                          'Fahrten mit anderen Fahrzeugen trägt der Fuhrpark ein.'),
                    for (final a in vehicles) ...[
                      VehicleTripsSection(
                        key: ValueKey('trips-${a['vehicle_id']}'),
                        api: widget.api,
                        vehicleId: a['vehicle_id'].toString(),
                        now: widget.now,
                        ownOnly: true,
                        title: 'Fahrtenbuch ${a['kennzeichen'] ?? ''}',
                        onChanged: _reloadTrips,
                      ),
                      const Divider(height: 32),
                    ],
                    Text('Alle meine Fahrten',
                        style: Theme.of(context).textTheme.titleMedium),
                    if (trips.isEmpty) const Text('Noch keine Fahrt erfasst.'),
                    for (final t in trips)
                      Text(
                          '${(t as Map<String, dynamic>)['kennzeichen'] ?? ''}: ${tripLine(t)}'),
                  ],
                ),
    );
  }
}
