import 'package:flutter/material.dart';
import '../api.dart';
import 'fleet_page.dart' show vehicleDetails;
import 'vehicle_schedules_section.dart';

// Detailseite eines Fahrzeugs (Backlog G.2.4.1): Stammdaten und Termine.
// Bearbeitet werden die Stammdaten weiter in der Fuhrpark-Liste;
// Fahrerzuordnung und Fahrtenbuch kommen mit G.3.

class VehicleDetailPage extends StatelessWidget {
  const VehicleDetailPage({super.key, required this.api, required this.vehicle});
  final ApiClient api;
  final Map<String, dynamic> vehicle;

  @override
  Widget build(BuildContext context) {
    final bez = vehicle['bezeichnung']?.toString() ?? '';
    final standort = vehicle['standort']?.toString() ?? '';
    final notiz = vehicle['notiz']?.toString() ?? '';
    final fin = vehicle['fin']?.toString() ?? '';
    return Scaffold(
      appBar: AppBar(title: Text('${vehicle['kennzeichen'] ?? ''}${bez.isEmpty ? '' : ' – $bez'}')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Text(vehicleDetails(vehicle)),
            if (fin.isNotEmpty) Text('FIN: $fin'),
            if (standort.isNotEmpty) Text('Standort: $standort'),
            if (notiz.isNotEmpty) Text('Notiz: $notiz'),
            const Divider(height: 32),
            VehicleSchedulesSection(
              api: api,
              vehicleId: vehicle['id'].toString(),
              deregistered: vehicle['abgemeldet_am'] != null,
            ),
          ],
        ),
      ),
    );
  }
}
