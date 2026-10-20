import 'package:flutter/material.dart';
import '../api.dart';
import 'hr_calendar_page.dart' show bundeslaender, sortedBundeslaender;

// Arbeitstage und Urlaub am Mitarbeiter (Backlog F.8.6.4.2), Felder aus
// F.8.2.3/F.8.3.1: Bundesland (leer = wie der Mandant), Arbeitstage
// (ISO 1 = Montag ... 7 = Sonntag) und Jahresanspruch (leer = Vorgabe
// 4 Tage je woechentlichem Arbeitstag, zugleich die Untergrenze).

const weekdayShort = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'];

List<int> _days(Map<String, dynamic> emp) =>
    ((emp['arbeitstage'] as List?) ?? const [1, 2, 3, 4, 5]).map((e) => (e as num).toInt()).toList()..sort();

// workdaysLabel: "Mo, Di, Mi, Do, Fr".
String workdaysLabel(List<int> days) => (days.toList()..sort()).map((d) => weekdayShort[d - 1]).join(', ');

int defaultLeaveDays(int workdaysPerWeek) => 4 * workdaysPerWeek;

String _num(num v) => v == v.roundToDouble() ? v.toInt().toString() : v.toString().replaceAll('.', ',');

// leaveEntitlementLabel: "28 Tage" oder "20 Tage (Vorgabe)".
String leaveEntitlementLabel(Map<String, dynamic> emp) {
  final v = emp['urlaubstage_jahr'] as num?;
  if (v != null) return '${_num(v)} Tage';
  return '${defaultLeaveDays(_days(emp).length)} Tage (Vorgabe)';
}

// employeeBundeslandLabel: Name oder "wie Firma".
String employeeBundeslandLabel(Map<String, dynamic> emp) {
  final code = emp['bundesland']?.toString() ?? '';
  return code.isEmpty ? 'wie Firma' : (bundeslaender[code] ?? code);
}

// buildWorkSettingsPatch prueft die Eingaben und baut den Patch fuer
// PATCH /employees/{id}. bundesland "" = wie Firma (null), urlaub leer =
// Vorgabe (null).
({Map<String, dynamic>? body, String? error}) buildWorkSettingsPatch({
  required String bundesland,
  required Set<int> days,
  required String urlaub,
}) {
  if (days.isEmpty) return (body: null, error: 'Bitte mindestens einen Arbeitstag wählen.');
  final sorted = days.toList()..sort();
  double? leave;
  final t = urlaub.trim().replaceAll(',', '.');
  if (t.isNotEmpty) {
    final v = double.tryParse(t);
    if (v == null || v > 366 || (v * 2) != (v * 2).roundToDouble()) {
      return (body: null, error: 'Bitte den Jahresurlaub in ganzen oder halben Tagen angeben oder leer lassen.');
    }
    final minimum = defaultLeaveDays(sorted.length);
    if (v < minimum) {
      return (body: null, error: 'Mindestens $minimum Tage bei ${sorted.length} Arbeitstagen je Woche (gesetzlicher Mindesturlaub).');
    }
    leave = v;
  }
  return (
    body: {'bundesland': bundesland.isEmpty ? null : bundesland, 'arbeitstage': sorted, 'urlaubstage_jahr': leave},
    error: null,
  );
}

// showWorkSettingsDialog oeffnet den Dialog und speichert; liefert den
// geaenderten Mitarbeiter oder null (abgebrochen).
Future<Map<String, dynamic>?> showWorkSettingsDialog(BuildContext context, ApiClient api, Map<String, dynamic> emp) async {
  String bundesland = emp['bundesland']?.toString() ?? '';
  final days = _days(emp).toSet();
  final v = emp['urlaubstage_jahr'] as num?;
  final urlaubCtrl = TextEditingController(text: v == null ? '' : _num(v));
  String? dialogError;
  return showDialog<Map<String, dynamic>>(
    context: context,
    builder: (ctx) => StatefulBuilder(
      builder: (ctx, setStateDialog) => AlertDialog(
        title: const Text('Arbeitstage und Urlaub'),
        content: SizedBox(
          width: 460,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                DropdownButtonFormField<String>(
                  initialValue: bundesland,
                  isExpanded: true,
                  decoration: const InputDecoration(labelText: 'Bundesland (für die Feiertage)'),
                  items: [
                    const DropdownMenuItem(value: '', child: Text('wie Firma')),
                    for (final code in sortedBundeslaender()) DropdownMenuItem(value: code, child: Text(bundeslaender[code]!)),
                  ],
                  onChanged: (x) => setStateDialog(() => bundesland = x ?? ''),
                ),
                const SizedBox(height: 12),
                const Text('Arbeitstage'),
                Wrap(
                  spacing: 6,
                  children: [
                    for (var d = 1; d <= 7; d++)
                      FilterChip(
                        label: Text(weekdayShort[d - 1]),
                        selected: days.contains(d),
                        onSelected: (on) => setStateDialog(() => on ? days.add(d) : days.remove(d)),
                      ),
                  ],
                ),
                TextField(
                  controller: urlaubCtrl,
                  decoration: InputDecoration(
                    labelText: 'Jahresurlaub in Tagen (leer = Vorgabe ${defaultLeaveDays(days.length)})',
                  ),
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
              final r = buildWorkSettingsPatch(bundesland: bundesland, days: days, urlaub: urlaubCtrl.text);
              if (r.error != null) {
                setStateDialog(() => dialogError = r.error);
                return;
              }
              try {
                final updated = await api.updateEmployee(emp['id'].toString(), r.body!);
                if (ctx.mounted) Navigator.of(ctx).pop(updated);
              } on ApiException catch (e) {
                setStateDialog(() => dialogError = e.message);
              } catch (e) {
                setStateDialog(() => dialogError = '$e');
              }
            },
            child: const Text('Speichern'),
          ),
        ],
      ),
    ),
  );
}
