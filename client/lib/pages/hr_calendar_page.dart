import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;
import 'employee_qualifications_section.dart' show parseDateInput;

// Feiertage und Betriebsruhe (Backlog F.8.6.4.1), Routen aus F.8.2.2:
//   - Bundesland des Mandanten und die bayerischen Schalter (Mariae
//     Himmelfahrt, Augsburger Friedensfest) - settings.manage.
//   - Betriebliche freie Tage (Betriebsruhe, 24./31.12.) - leave_requests.
//     decide (wer Urlaub genehmigt, legt auch die Betriebsruhe fest).
//   - Vorschau aller freien Tage eines Jahres - jeder angemeldete Benutzer.
// Urlaubsantraege zaehlen Arbeitstage ohne diese freien Tage.

const bundeslaender = {
  'BW': 'Baden-Württemberg',
  'BY': 'Bayern',
  'BE': 'Berlin',
  'BB': 'Brandenburg',
  'HB': 'Bremen',
  'HH': 'Hamburg',
  'HE': 'Hessen',
  'MV': 'Mecklenburg-Vorpommern',
  'NI': 'Niedersachsen',
  'NW': 'Nordrhein-Westfalen',
  'RP': 'Rheinland-Pfalz',
  'SL': 'Saarland',
  'SN': 'Sachsen',
  'ST': 'Sachsen-Anhalt',
  'SH': 'Schleswig-Holstein',
  'TH': 'Thüringen',
};

// sortedBundeslaender: Kuerzel nach Namen sortiert.
List<String> sortedBundeslaender() => bundeslaender.keys.toList()..sort((a, b) => bundeslaender[a]!.compareTo(bundeslaender[b]!));

// calendarDayLine: "03.10.2026 Tag der Deutschen Einheit" bzw. mit
// "(betrieblich)".
String calendarDayLine(Map<String, dynamic> d) {
  final base = '${isoToGerman(d['date']?.toString())} ${d['name'] ?? ''}';
  return d['art'] == 'betrieblich' ? '$base (betrieblich)' : base;
}

// buildCompanyDay prueft Datum und Bezeichnung.
({String? datum, String? name, String? error}) buildCompanyDay(String datum, String name) {
  final d = parseDateInput(datum);
  if (d == null) return (datum: null, name: null, error: 'Bitte das Datum als TT.MM.JJJJ angeben.');
  if (name.trim().isEmpty) return (datum: null, name: null, error: 'Bitte eine Bezeichnung angeben.');
  return (datum: d, name: name.trim(), error: null);
}

class HrCalendarPage extends StatefulWidget {
  const HrCalendarPage({super.key, required this.api, this.now});
  final ApiClient api;
  final DateTime Function()? now;

  @override
  State<HrCalendarPage> createState() => _HrCalendarPageState();
}

class _HrCalendarPageState extends State<HrCalendarPage> {
  late int year;
  String bundesland = '';
  bool mariae = false;
  bool augsburg = false;
  bool settingsLoaded = false;
  List<dynamic> companyDays = [];
  Map<String, dynamic>? calendar;
  bool loading = false;
  bool saving = false;
  String? error;

  bool get _canSettings => widget.api.hasPermission('settings.manage');
  bool get _canDays => widget.api.hasPermission('leave_requests.decide');

  @override
  void initState() {
    super.initState();
    year = (widget.now ?? DateTime.now)().year;
    _load();
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final st = _canSettings ? await widget.api.getHrCalendarSettings() : null;
      final days = _canDays ? await widget.api.listCompanyDays(year: year) : const <dynamic>[];
      final cal = await widget.api.getHolidays(year: year);
      if (!mounted) return;
      setState(() {
        if (st != null) {
          bundesland = st['bundesland']?.toString() ?? '';
          mariae = st['mariae_himmelfahrt'] == true;
          augsburg = st['augsburger_friedensfest'] == true;
          settingsLoaded = true;
        }
        companyDays = days;
        calendar = cal;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => error = 'Daten konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted) setState(() => error = 'Daten konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _run(Future<void> Function() action, String success) async {
    try {
      await action();
      _snack(success);
      await _load();
    } on ApiException catch (e) {
      _snack('Fehler: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _saveSettings() async {
    setState(() => saving = true);
    // Die Schalter wirken nur in Bayern - ausserhalb werden sie nicht
    // gespeichert, damit ein spaeterer Wechsel nach Bayern nicht
    // ueberraschend alte Werte aktiviert.
    final by = bundesland == 'BY';
    await _run(
        () => widget.api.setHrCalendarSettings({
              'bundesland': bundesland,
              'mariae_himmelfahrt': by && mariae,
              'augsburger_friedensfest': by && augsburg,
            }),
        'Feiertagskalender gespeichert');
    if (mounted) setState(() => saving = false);
  }

  Future<void> _addDayDialog() async {
    final datumCtrl = TextEditingController();
    final nameCtrl = TextEditingController();
    String? dialogError;
    final result = await showDialog<({String datum, String name})>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Betrieblichen freien Tag eintragen'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                TextField(controller: datumCtrl, decoration: const InputDecoration(labelText: 'Datum (TT.MM.JJJJ)')),
                TextField(controller: nameCtrl, decoration: const InputDecoration(labelText: 'Bezeichnung (z. B. Betriebsruhe)')),
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
                final r = buildCompanyDay(datumCtrl.text, nameCtrl.text);
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop((datum: r.datum!, name: r.name!));
              },
              child: const Text('Eintragen'),
            ),
          ],
        ),
      ),
    );
    if (result == null) return;
    await _run(() => widget.api.createCompanyDay(result.datum, result.name), 'Freier Tag eingetragen');
  }

  Future<void> _deleteDay(Map<String, dynamic> d) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Freien Tag entfernen'),
        content: Text('${isoToGerman(d['datum']?.toString())} ${d['name'] ?? ''}'),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: const Text('Entfernen')),
        ],
      ),
    );
    if (ok != true) return;
    await _run(() => widget.api.deleteCompanyDay(d['id'].toString()), 'Freier Tag entfernt');
  }

  Widget _header(String text) => Padding(
        padding: const EdgeInsets.only(top: 16, bottom: 4),
        child: Text(text, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
      );

  void _goToYear(int y) {
    setState(() => year = y);
    _load();
  }

  @override
  Widget build(BuildContext context) {
    final days = ((calendar?['days'] as List?) ?? const []).cast<Map<String, dynamic>>();
    return Scaffold(
      appBar: AppBar(
        title: const Text('Feiertage und Betriebsruhe'),
        actions: [IconButton(onPressed: _load, icon: const Icon(Icons.refresh), tooltip: 'Neu laden')],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (_canSettings && settingsLoaded) ...[
              _header('Bundesland'),
              const Text('Bestimmt die gesetzlichen Feiertage. Ohne Bundesland gelten nur die neun bundesweiten Feiertage.'),
              SizedBox(
                width: 360,
                child: DropdownButtonFormField<String>(
                  initialValue: bundesland,
                  isExpanded: true,
                  decoration: const InputDecoration(labelText: 'Bundesland'),
                  items: [
                    const DropdownMenuItem(value: '', child: Text('nicht eingestellt')),
                    for (final code in sortedBundeslaender()) DropdownMenuItem(value: code, child: Text(bundeslaender[code]!)),
                  ],
                  onChanged: (v) => setState(() => bundesland = v ?? ''),
                ),
              ),
              if (bundesland == 'BY') ...[
                CheckboxListTile(
                  contentPadding: EdgeInsets.zero,
                  value: mariae,
                  onChanged: (v) => setState(() => mariae = v ?? false),
                  title: const Text('Mariä Himmelfahrt (15.08.) - nur in überwiegend katholischen Gemeinden'),
                ),
                CheckboxListTile(
                  contentPadding: EdgeInsets.zero,
                  value: augsburg,
                  onChanged: (v) => setState(() => augsburg = v ?? false),
                  title: const Text('Augsburger Friedensfest (08.08.) - nur in Augsburg'),
                ),
              ],
              Align(
                alignment: Alignment.centerLeft,
                child: FilledButton(onPressed: saving ? null : _saveSettings, child: const Text('Speichern')),
              ),
            ],
            Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 8,
              children: [
                IconButton(onPressed: () => _goToYear(year - 1), icon: const Icon(Icons.chevron_left), tooltip: 'Vorjahr'),
                Text('Jahr $year', style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                IconButton(onPressed: () => _goToYear(year + 1), icon: const Icon(Icons.chevron_right), tooltip: 'Folgejahr'),
              ],
            ),
            if (_canDays) ...[
              _header('Betriebliche freie Tage'),
              if (!loading && companyDays.isEmpty) const Text('Keine betrieblichen freien Tage in diesem Jahr.'),
              for (final d in companyDays.cast<Map<String, dynamic>>())
                Row(
                  children: [
                    Expanded(child: Text('${isoToGerman(d['datum']?.toString())} ${d['name'] ?? ''}')),
                    IconButton(tooltip: 'Entfernen', icon: const Icon(Icons.delete_outline), onPressed: () => _deleteDay(d)),
                  ],
                ),
              Align(
                alignment: Alignment.centerLeft,
                child: OutlinedButton(onPressed: _addDayDialog, child: const Text('Freien Tag eintragen')),
              ),
            ],
            _header('Alle freien Tage $year'),
            if ((calendar?['bundesland']?.toString() ?? '').isEmpty && calendar != null)
              const Text('Kein Bundesland eingestellt - nur bundesweite Feiertage.', style: TextStyle(color: Color(0xFFB26A00))),
            for (final d in days) Text(calendarDayLine(d)),
          ],
        ),
      ),
    );
  }
}
