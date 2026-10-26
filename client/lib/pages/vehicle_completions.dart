import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;
import 'vehicle_schedules_section.dart' show dueDateLabel, parseMonthInput, parsePositiveInt, scheduleTitle;

// Erledigungen der Fahrzeugtermine (Backlog G.2.4.2), Routen aus G.2.3:
// erfassen, ansehen, stornieren. Erledigungen sind Belege - nie geaendert,
// nur storniert und neu erfasst. Ohne Folgetermin schlaegt der Server einen
// aus dem Intervall vor, nach einer HU/UVV mit Maengeln oder nicht
// bestanden keinen (dann steht eine Nachpruefung an).

const completionErgebnisse = {
  'bestanden': 'bestanden',
  'maengel': 'Mängel',
  'nicht_bestanden': 'nicht bestanden',
  'erledigt': 'erledigt',
};

String _two(int v) => v.toString().padLeft(2, '0');

// ergebnisseFor: HU und UVV haben ein Pruefergebnis, alles andere wird
// "erledigt".
List<String> ergebnisseFor(String? art) =>
    art == 'hu' || art == 'uvv' ? const ['bestanden', 'maengel', 'nicht_bestanden'] : const ['erledigt'];

// buildCompletionBody: naechster_termin null = Vorschlag des Servers.
Map<String, dynamic> buildCompletionBody({
  required String scheduleId,
  required String erledigtAm,
  required String ergebnis,
  int? kmStand,
  String? naechsterTermin,
  int? naechsterKm,
  String pruefer = '',
  String notiz = '',
}) =>
    {
      'schedule_id': scheduleId,
      'erledigt_am': erledigtAm,
      'ergebnis': ergebnis,
      'km_stand': kmStand,
      'naechster_termin': naechsterTermin,
      'naechster_km': naechsterKm,
      'pruefer': pruefer.trim(),
      'notiz': notiz.trim(),
    };

// parseKmStand: wie parsePositiveInt, aber 0 ist ein gueltiger Stand.
({bool ok, int? value}) parseKmStand(String input) {
  final t = input.trim().replaceAll('.', '').replaceAll(' ', '');
  if (t.isEmpty) return (ok: true, value: null);
  final v = int.tryParse(t);
  if (v == null || v < 0) return (ok: false, value: null);
  return (ok: true, value: v);
}

// completionLine: "10.03.2026 · bestanden · TÜV · 42000 km · nächste
// 03/2028 · bei 72000 km · Notiz", storniert mit Grund.
String completionLine(String? art, Map<String, dynamic> c) {
  final parts = <String>[
    isoToGerman(c['erledigt_am']?.toString()),
    completionErgebnisse[c['ergebnis']] ?? (c['ergebnis']?.toString() ?? ''),
  ];
  final pruefer = c['pruefer']?.toString() ?? '';
  if (pruefer.isNotEmpty) parts.add(pruefer);
  if (c['km_stand'] is num) parts.add('${(c['km_stand'] as num).toInt()} km');
  final next = dueDateLabel(art, c['naechster_termin']?.toString());
  if (next.isNotEmpty) parts.add('nächste $next');
  if (c['naechster_km'] is num) parts.add('bei ${(c['naechster_km'] as num).toInt()} km');
  final notiz = c['notiz']?.toString() ?? '';
  if (notiz.isNotEmpty) parts.add(notiz);
  var line = parts.join(' · ');
  if (c['status'] == 'storniert') line += ' (storniert: ${c['storno_reason'] ?? ''})';
  return line;
}

void _snack(BuildContext context, String text) {
  if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
}

// showRecordCompletionDialog erfasst eine Erledigung; true, wenn gespeichert.
Future<bool> showRecordCompletionDialog(BuildContext context, ApiClient api, Map<String, dynamic> schedule, DateTime today) async {
  final art = schedule['art']?.toString();
  final ergebnisse = ergebnisseFor(art);
  String ergebnis = ergebnisse.first;
  final datumCtrl = TextEditingController(text: '${_two(today.day)}.${_two(today.month)}.${today.year}');
  final kmCtrl = TextEditingController();
  final prueferCtrl = TextEditingController();
  final nextCtrl = TextEditingController();
  final nextKmCtrl = TextEditingController();
  final notizCtrl = TextEditingController();
  String? dialogError;
  final body = await showDialog<Map<String, dynamic>>(
    context: context,
    builder: (ctx) => StatefulBuilder(
      builder: (ctx, setStateDialog) => AlertDialog(
        title: Text('${scheduleTitle(schedule)} erledigt'),
        content: SizedBox(
          width: 460,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                TextField(controller: datumCtrl, decoration: const InputDecoration(labelText: 'Datum (TT.MM.JJJJ)')),
                if (ergebnisse.length > 1)
                  DropdownButtonFormField<String>(
                    initialValue: ergebnis,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Ergebnis'),
                    items: [for (final e in ergebnisse) DropdownMenuItem(value: e, child: Text(completionErgebnisse[e]!))],
                    onChanged: (v) => setStateDialog(() => ergebnis = v ?? ergebnis),
                  ),
                if (ergebnis != 'bestanden' && ergebnis != 'erledigt')
                  const Padding(
                    padding: EdgeInsets.only(top: 8),
                    child: Text('Das Fahrzeug ist danach gesperrt, bis eine bestandene Prüfung erfasst ist.',
                        style: TextStyle(color: Color(0xFFB00020))),
                  ),
                TextField(controller: kmCtrl, decoration: const InputDecoration(labelText: 'Kilometerstand (optional)')),
                TextField(controller: prueferCtrl, decoration: const InputDecoration(labelText: 'Prüfstelle / Werkstatt')),
                TextField(
                  controller: nextCtrl,
                  decoration: InputDecoration(
                    labelText: art == 'hu'
                        ? (ergebnis == 'bestanden' ? 'Nächste HU (MM.JJJJ, optional)' : 'Nachprüfung bis (MM.JJJJ, optional)')
                        : 'Folgetermin (TT.MM.JJJJ, optional)',
                    helperText: ergebnis == 'bestanden' || ergebnis == 'erledigt'
                        ? 'Leer = Vorschlag aus dem Intervall des Termins.'
                        : 'Ohne Angabe gibt es keinen Folgetermin.',
                  ),
                ),
                if (art == 'wartung')
                  TextField(controller: nextKmCtrl, decoration: const InputDecoration(labelText: 'Nächste Wartung bei km (optional, leer = km-Stand + Intervall)')),
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
              final datum = parseDateInput(datumCtrl.text);
              final km = parseKmStand(kmCtrl.text);
              final nextText = nextCtrl.text.trim();
              final next = nextText.isEmpty ? null : (art == 'hu' ? parseMonthInput(nextText) : parseDateInput(nextText));
              final nextKm = parsePositiveInt(art == 'wartung' ? nextKmCtrl.text : '');
              String? problem;
              if (datum == null) {
                problem = 'Bitte das Datum als TT.MM.JJJJ angeben.';
              } else if (!km.ok) {
                problem = 'Bitte den Kilometerstand als ganze Zahl angeben oder leer lassen.';
              } else if (nextText.isNotEmpty && next == null) {
                problem = art == 'hu' ? 'Bitte den Termin als MM.JJJJ angeben oder leer lassen.' : 'Bitte den Folgetermin als TT.MM.JJJJ angeben oder leer lassen.';
              } else if (!nextKm.ok) {
                problem = 'Bitte die Kilometer als ganze Zahl angeben oder leer lassen.';
              }
              if (problem != null) {
                setStateDialog(() => dialogError = problem);
                return;
              }
              Navigator.of(ctx).pop(buildCompletionBody(
                scheduleId: schedule['id'].toString(),
                erledigtAm: datum!,
                ergebnis: ergebnis,
                kmStand: km.value,
                naechsterTermin: next,
                naechsterKm: nextKm.value,
                pruefer: prueferCtrl.text,
                notiz: notizCtrl.text,
              ));
            },
            child: const Text('Erfassen'),
          ),
        ],
      ),
    ),
  );
  if (body == null || !context.mounted) return false;
  try {
    await api.recordCompletion(body);
    _snack(context, 'Erledigung erfasst');
    return true;
  } catch (e) {
    _snack(context, 'Fehler: $e');
    return false;
  }
}

// showCompletionsDialog zeigt die Erledigungen eines Termins, mit
// fleet.write stornierbar (auch am abgemeldeten Fahrzeug - Berichtigung
// der Historie). true, wenn etwas storniert wurde.
Future<bool> showCompletionsDialog(BuildContext context, ApiClient api, Map<String, dynamic> schedule, {required bool canStorno}) async {
  final art = schedule['art']?.toString();
  var changed = false;
  List<dynamic> list = [];
  String? error;
  var loaded = false;
  var started = false;
  await showDialog<void>(
    context: context,
    builder: (ctx) => StatefulBuilder(
      builder: (ctx, setStateDialog) {
        void update(VoidCallback f) {
          if (ctx.mounted) setStateDialog(f);
        }

        Future<void> reload() async {
          try {
            final out = await api.listCompletions(schedule['id'].toString(), includeStorniert: true);
            update(() {
              list = out;
              error = null;
            });
          } catch (e) {
            update(() => error = 'Erledigungen konnten nicht geladen werden: $e');
          } finally {
            update(() => loaded = true);
          }
        }

        Future<void> storno(Map<String, dynamic> c) async {
          final reasonCtrl = TextEditingController();
          final reason = await showDialog<String>(
            context: ctx,
            builder: (c2) => StatefulBuilder(
              builder: (c2, setStateStorno) => AlertDialog(
                title: const Text('Erledigung stornieren'),
                content: SizedBox(
                  width: 420,
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(completionLine(art, c)),
                      const SizedBox(height: 8),
                      const Text('Eine Erledigung wird nicht gelöscht, sondern mit Begründung storniert.'),
                      TextField(
                        controller: reasonCtrl,
                        decoration: const InputDecoration(labelText: 'Begründung'),
                        onChanged: (_) => setStateStorno(() {}),
                      ),
                    ],
                  ),
                ),
                actions: [
                  TextButton(onPressed: () => Navigator.of(c2).pop(), child: const Text('Abbrechen')),
                  FilledButton(
                    onPressed: reasonCtrl.text.trim().isEmpty ? null : () => Navigator.of(c2).pop(reasonCtrl.text.trim()),
                    child: const Text('Stornieren'),
                  ),
                ],
              ),
            ),
          );
          if (reason == null) return;
          try {
            await api.stornoCompletion(c['id'].toString(), reason);
            changed = true;
            await reload();
          } catch (e) {
            update(() => error = 'Fehler: $e');
          }
        }

        if (!started) {
          started = true;
          reload();
        }
        return AlertDialog(
          title: Text('Erledigungen – ${scheduleTitle(schedule)}'),
          content: SizedBox(
            width: 560,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (!loaded) const LinearProgressIndicator(minHeight: 2),
                  if (loaded && list.isEmpty && error == null) const Text('Noch keine Erledigungen.'),
                  for (final c in list)
                    Row(
                      children: [
                        Expanded(child: Text(completionLine(art, c as Map<String, dynamic>))),
                        if (canStorno && c['status'] == 'active')
                          IconButton(tooltip: 'Erledigung stornieren', icon: const Icon(Icons.block, size: 18), onPressed: () => storno(c)),
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
          actions: [FilledButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Schließen'))],
        );
      },
    ),
  );
  return changed;
}
