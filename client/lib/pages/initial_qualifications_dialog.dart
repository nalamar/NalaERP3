import 'package:flutter/material.dart';
import '../api.dart';
import 'employee_qualifications_section.dart';

// Zweiter Schritt beim Anlegen eines Mitarbeiters (Backlog F.7.3.3).
// Vorgabe des Nutzers vom 2026-10-13: "Beim Anlegen eines neuen
// Mitarbeiters soll die Personalabteilung festlegen koennen, welche
// Qualifikationen fuer die Stelle noetig sind und ob sie schon vorhanden
// sind."
//
// Der Server kennt dafuer keinen einzelnen Aufruf (atomare Anlage: Backlog
// F.10). Der Mitarbeiter ist an dieser Stelle schon angelegt; danach
// werden die Anforderungen gesetzt und die vorhandenen Nachweise einzeln
// erfasst. Schlaegt ein Schritt fehl, sagt das Ergebnis genau, was
// gespeichert ist und was nicht.

class InitialQualificationEntry {
  InitialQualificationEntry({required this.typeId, required this.name});
  final String typeId;
  final String name;
  bool required = false;
  bool present = false;
  String erworbenAm = '';
}

class InitialQualificationPlan {
  const InitialQualificationPlan({this.requiredIds = const [], this.records = const [], this.error});
  final List<String> requiredIds;
  // Je vorhandener Qualifikation: Name (fuer die Meldung) und Body.
  final List<MapEntry<String, Map<String, dynamic>>> records;
  final String? error;
}

// buildInitialQualificationPlan prueft die Eingaben VOR jedem Aufruf: ein
// "vorhanden" ohne gueltiges Erwerbsdatum wird abgelehnt, statt still
// uebergangen.
InitialQualificationPlan buildInitialQualificationPlan(String employeeId, List<InitialQualificationEntry> entries) {
  final requiredIds = <String>[];
  final records = <MapEntry<String, Map<String, dynamic>>>[];
  for (final e in entries) {
    if (e.required) requiredIds.add(e.typeId);
    if (!e.present) continue;
    final date = parseDateInput(e.erworbenAm);
    if (date == null) {
      return InitialQualificationPlan(error: '${e.name}: bitte das Erwerbsdatum als TT.MM.JJJJ angeben.');
    }
    records.add(MapEntry(e.name, buildQualificationBody(employeeId: employeeId, typeId: e.typeId, erworbenAm: date)));
  }
  requiredIds.sort();
  return InitialQualificationPlan(requiredIds: requiredIds, records: records);
}

// Ergebnis eines Schritts: Zeile fuer die Rueckmeldung und ob er gelang.
class InitialQualificationStep {
  const InitialQualificationStep(this.line, {required this.ok});
  final String line;
  final bool ok;
}

// applyInitialQualifications fuehrt den Plan aus und liefert je Schritt
// ein Ergebnis. Anforderungen werden nur gesetzt, wenn
// welche gewaehlt sind (ein neuer Mitarbeiter hat keine). Ein fehlerhafter
// Nachweis haelt die uebrigen nicht auf.
Future<List<InitialQualificationStep>> applyInitialQualifications(
    ApiClient api, String employeeId, InitialQualificationPlan plan) async {
  final lines = <InitialQualificationStep>[];
  if (plan.requiredIds.isNotEmpty) {
    try {
      await api.setQualificationRequirements(employeeId, plan.requiredIds);
      lines.add(InitialQualificationStep('Anforderungen gespeichert (${plan.requiredIds.length}).', ok: true));
    } catch (e) {
      lines.add(InitialQualificationStep('Anforderungen NICHT gespeichert: $e', ok: false));
    }
  }
  for (final r in plan.records) {
    try {
      await api.createQualification(r.value);
      lines.add(InitialQualificationStep('Nachweis "${r.key}" erfasst.', ok: true));
    } catch (e) {
      lines.add(InitialQualificationStep('Nachweis "${r.key}" NICHT erfasst: $e', ok: false));
    }
  }
  return lines;
}

// showInitialQualificationsDialog zeigt den zweiten Schritt. Ohne aktive
// Arten im Katalog gibt es nichts zu fragen.
Future<void> showInitialQualificationsDialog(BuildContext context, ApiClient api, String employeeId, String employeeName) async {
  List<dynamic> types;
  try {
    types = await api.listQualificationTypes();
  } catch (e) {
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Qualifikationen konnten nicht geladen werden: $e')));
    }
    return;
  }
  if (types.isEmpty || !context.mounted) return;
  final entries = [
    for (final t in types)
      InitialQualificationEntry(
        typeId: (t as Map<String, dynamic>)['id'].toString(),
        name: t['name']?.toString() ?? t['id'].toString(),
      ),
  ]..sort((a, b) => a.name.toLowerCase().compareTo(b.name.toLowerCase()));

  final plan = await showDialog<InitialQualificationPlan>(
    context: context,
    barrierDismissible: false,
    builder: (ctx) {
      String? error;
      return StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text('Qualifikationen für $employeeName'),
          content: SizedBox(
            width: 620,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
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
                          onChanged: (v) => setStateDialog(() => e.required = v == true),
                        ),
                        const Text('erforderlich'),
                        const SizedBox(width: 8),
                        Checkbox(
                          key: ValueKey('iq-has-${e.typeId}'),
                          value: e.present,
                          onChanged: (v) => setStateDialog(() => e.present = v == true),
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
                  if (error != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
                    ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Überspringen')),
            FilledButton(
              onPressed: () {
                final p = buildInitialQualificationPlan(employeeId, entries);
                if (p.error != null) {
                  setStateDialog(() => error = p.error);
                  return;
                }
                Navigator.of(ctx).pop(p);
              },
              child: const Text('Speichern'),
            ),
          ],
        ),
      );
    },
  );
  if (plan == null || (plan.requiredIds.isEmpty && plan.records.isEmpty)) return;
  final lines = await applyInitialQualifications(api, employeeId, plan);
  if (!context.mounted) return;
  final failed = lines.any((l) => !l.ok);
  if (!failed) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(lines.map((l) => l.line).join(' '))));
    return;
  }
  // Teilerfolg nicht in einer fluechtigen Meldung verstecken.
  await showDialog<void>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('Nicht alles gespeichert'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('Der Mitarbeiter ist angelegt. Fehlendes bitte im Abschnitt "Qualifikationen" nachtragen.'),
          const SizedBox(height: 8),
          for (final l in lines) Text(l.line, style: l.ok ? null : const TextStyle(color: Color(0xFFB00020))),
        ],
      ),
      actions: [FilledButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('OK'))],
    ),
  );
}
