import 'package:flutter/material.dart';
import 'employee_qualifications_section.dart';

// Qualifikationen gleich beim Anlegen eines Mitarbeiters (Backlog F.7.3.3,
// seit F.10.5.2 in einem Aufruf). Vorgabe des Nutzers vom 2026-10-13:
// "Beim Anlegen eines neuen Mitarbeiters soll die Personalabteilung
// festlegen koennen, welche Qualifikationen fuer die Stelle noetig sind und
// ob sie schon vorhanden sind."
//
// Die Auswahl steht im Anlegedialog selbst; gespeichert wird mit
// POST /employees/with-qualifications - Mitarbeiter, Anforderungen und
// Nachweise ganz oder gar nicht (F.10.5.1). Einen Teilerfolg gibt es
// deshalb nicht mehr; ein Fehler bleibt im Dialog, die Eingaben auch.

class InitialQualificationEntry {
  InitialQualificationEntry({required this.typeId, required this.name});
  final String typeId;
  final String name;
  bool required = false;
  bool present = false;
  String erworbenAm = '';
}

// initialQualificationEntries: je aktiver Art eine Zeile, nach Name sortiert.
List<InitialQualificationEntry> initialQualificationEntries(List<dynamic> types) => [
      for (final t in types)
        InitialQualificationEntry(
          typeId: (t as Map<String, dynamic>)['id'].toString(),
          name: t['name']?.toString() ?? t['id'].toString(),
        ),
    ]..sort((a, b) => a.name.toLowerCase().compareTo(b.name.toLowerCase()));

class InitialQualificationPlan {
  const InitialQualificationPlan({this.requiredIds = const [], this.records = const [], this.error});
  final List<String> requiredIds;
  // Je vorhandener Qualifikation: type_id und erworben_am (JJJJ-MM-TT).
  final List<Map<String, dynamic>> records;
  final String? error;
}

// buildInitialQualificationPlan prueft die Eingaben VOR dem Aufruf: ein
// "vorhanden" ohne gueltiges Erwerbsdatum wird abgelehnt, statt still
// uebergangen.
InitialQualificationPlan buildInitialQualificationPlan(List<InitialQualificationEntry> entries) {
  final requiredIds = <String>[];
  final records = <Map<String, dynamic>>[];
  for (final e in entries) {
    if (e.required) requiredIds.add(e.typeId);
    if (!e.present) continue;
    final date = parseDateInput(e.erworbenAm);
    if (date == null) {
      return InitialQualificationPlan(error: '${e.name}: bitte das Erwerbsdatum als TT.MM.JJJJ angeben.');
    }
    records.add({'type_id': e.typeId, 'erworben_am': date});
  }
  requiredIds.sort();
  return InitialQualificationPlan(requiredIds: requiredIds, records: records);
}

// buildEmployeeWithQualificationsBody baut den Body fuer
// POST /employees/with-qualifications.
Map<String, dynamic> buildEmployeeWithQualificationsBody(Map<String, dynamic> employee, InitialQualificationPlan plan) => {
      'employee': employee,
      'required_type_ids': plan.requiredIds,
      'qualifications': plan.records,
    };

// initialQualificationSummary beschreibt, was ausser dem Mitarbeiter
// gespeichert wurde; null, wenn nichts.
String? initialQualificationSummary(Map<String, dynamic> result) {
  final reqs = (result['requirements'] as List?)?.length ?? 0;
  final recs = (result['qualifications'] as List?)?.length ?? 0;
  if (reqs == 0 && recs == 0) return null;
  final parts = <String>[
    if (reqs > 0) 'Anforderungen: $reqs',
    if (recs > 0) recs == 1 ? '1 Nachweis' : '$recs Nachweise',
  ];
  return 'Mitarbeiter angelegt (${parts.join(', ')}).';
}

// InitialQualificationFields: die Zeilen im Anlegedialog. onChanged baut
// den Dialog neu auf (StatefulBuilder).
class InitialQualificationFields extends StatelessWidget {
  const InitialQualificationFields({super.key, required this.entries, required this.onChanged});
  final List<InitialQualificationEntry> entries;
  final void Function(VoidCallback) onChanged;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 16),
        const Text('Qualifikationen', style: TextStyle(fontWeight: FontWeight.bold)),
        const Text('Welche Qualifikationen braucht die Stelle, und welche sind schon vorhanden?'),
        const SizedBox(height: 8),
        for (final e in entries)
          Row(
            key: ValueKey('iq-${e.typeId}'),
            children: [
              Expanded(child: Text(e.name, overflow: TextOverflow.ellipsis)),
              Checkbox(
                key: ValueKey('iq-req-${e.typeId}'),
                value: e.required,
                onChanged: (v) => onChanged(() => e.required = v == true),
              ),
              const Text('erforderlich'),
              const SizedBox(width: 8),
              Checkbox(
                key: ValueKey('iq-has-${e.typeId}'),
                value: e.present,
                onChanged: (v) => onChanged(() => e.present = v == true),
              ),
              const Text('vorhanden seit'),
              const SizedBox(width: 8),
              SizedBox(
                width: 120,
                child: TextField(
                  key: ValueKey('iq-date-${e.typeId}'),
                  enabled: e.present,
                  decoration: const InputDecoration(hintText: 'TT.MM.JJJJ', isDense: true),
                  onChanged: (v) => e.erworbenAm = v,
                ),
              ),
            ],
          ),
      ],
    );
  }
}
