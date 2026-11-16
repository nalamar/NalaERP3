import 'package:flutter/material.dart';
import '../api.dart';
import '../web/browser.dart' as browser;
import 'employee_qualifications_section.dart'
    show QualificationFilePicker, formatFileSize;

// Leistungskatalog (ADR 0044, Backlog I.4.5) auf den Routen aus I.4.4b:
// Ziel des Matchings von LV-Positionen. Lesen mit quotes.read, pflegen und
// importieren mit leistungen.write. Leistungen werden nicht geloescht,
// sondern abgeschaltet. Preise stehen nicht im Katalog.

const leistungRCKlassen = ['', 'RC1', 'RC2', 'RC3', 'RC4', 'RC5', 'RC6'];
const maxLeistungenImportBytes = 5 * 1024 * 1024;
const leistungenCsvAccept = '.csv,text/csv';

// parseUWert liest den U-Wert mit Komma oder Punkt: leer = keiner (null),
// sonst eine Zahl ueber 0 bis 10. ok=false bei ungueltiger Eingabe.
({bool ok, double? wert}) parseUWert(String input) {
  final t = input.trim();
  if (t.isEmpty) return (ok: true, wert: null);
  final v = double.tryParse(t.replaceAll(',', '.'));
  if (v == null || v <= 0 || v > 10) return (ok: false, wert: null);
  return (ok: true, wert: v);
}

// formatUWert schreibt den U-Wert deutsch ohne ueberfluessige Nullen.
String formatUWert(Object? v) {
  if (v is! num) return '';
  var s = v.toStringAsFixed(3);
  s = s.replaceFirst(RegExp(r'0+$'), '').replaceFirst(RegExp(r'\.$'), '');
  return s.replaceAll('.', ',');
}

// buildLeistungBody baut den Body fuer POST/PUT oder liefert eine Meldung.
({Map<String, dynamic>? body, String? error}) buildLeistungBody({
  required String code,
  required String kurztext,
  String langtext = '',
  required String einheit,
  String gewerk = '',
  String profilserie = '',
  String verglasung = '',
  String rcKlasse = '',
  String uWert = '',
  String brandschutzklasse = '',
  bool? aktiv,
}) {
  final c = code.trim();
  if (c.isEmpty) return (body: null, error: 'Bitte einen Code angeben.');
  if (RegExp(r'\s').hasMatch(c))
    return (body: null, error: 'Der Code darf keine Leerzeichen enthalten.');
  if (kurztext.trim().isEmpty)
    return (body: null, error: 'Bitte einen Kurztext angeben.');
  if (einheit.trim().isEmpty)
    return (body: null, error: 'Bitte eine Einheit angeben.');
  final u = parseUWert(uWert);
  if (!u.ok)
    return (
      body: null,
      error: 'Der U-Wert muss eine Zahl über 0 bis 10 sein (z. B. 0,9).'
    );
  final body = <String, dynamic>{
    'code': c,
    'kurztext': kurztext.trim(),
    'langtext': langtext.trim(),
    'einheit': einheit.trim(),
    'gewerk': gewerk.trim(),
    'profilserie': profilserie.trim(),
    'verglasung': verglasung.trim(),
    'rc_klasse': rcKlasse,
    'u_wert': u.wert,
    'brandschutzklasse': brandschutzklasse.trim(),
  };
  if (aktiv != null) body['aktiv'] = aktiv;
  return (body: body, error: null);
}

// leistungDetails: Einheit, Gewerk und Systemvorgaben in einer Zeile.
String leistungDetails(Map<String, dynamic> l) => [
      l['einheit']?.toString() ?? '',
      if ((l['gewerk'] ?? '').toString().isNotEmpty) l['gewerk'].toString(),
      if ((l['profilserie'] ?? '').toString().isNotEmpty)
        l['profilserie'].toString(),
      if ((l['verglasung'] ?? '').toString().isNotEmpty)
        l['verglasung'].toString(),
      if ((l['rc_klasse'] ?? '').toString().isNotEmpty)
        l['rc_klasse'].toString(),
      if (l['u_wert'] is num) 'U ${formatUWert(l['u_wert'])}',
      if ((l['brandschutzklasse'] ?? '').toString().isNotEmpty)
        l['brandschutzklasse'].toString(),
      if (l['aktiv'] == false) 'abgeschaltet',
    ].where((s) => s.isNotEmpty).join(' · ');

class LeistungenPage extends StatefulWidget {
  const LeistungenPage({super.key, required this.api, this.filePicker});
  final ApiClient api;
  final QualificationFilePicker? filePicker;

  @override
  State<LeistungenPage> createState() => _LeistungenPageState();
}

class _LeistungenPageState extends State<LeistungenPage> {
  final suchCtrl = TextEditingController();
  List<dynamic> leistungen = [];
  bool loading = false;
  bool zeigeAbgeschaltete = false;
  bool importing = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('leistungen.write');

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    suchCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final list = await widget.api
          .listLeistungen(q: suchCtrl.text, nurAktive: !zeigeAbgeschaltete);
      if (mounted) setState(() => leistungen = list);
    } on ApiException catch (e) {
      if (mounted)
        setState(
            () => error = 'Katalog konnte nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted)
        setState(() => error = 'Katalog konnte nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted)
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _import() async {
    final picked = await (widget.filePicker ?? browser.pickFile)(
        accept: leistungenCsvAccept);
    if (picked == null) return;
    if (picked.bytes.isEmpty) {
      _snack('Die Datei ist leer.');
      return;
    }
    if (picked.bytes.length > maxLeistungenImportBytes) {
      _snack(
          'Die Datei ist zu groß (${formatFileSize(picked.bytes.length)}, höchstens 5 MB).');
      return;
    }
    setState(() => importing = true);
    try {
      final r =
          await widget.api.importLeistungenCsv(picked.filename, picked.bytes);
      _snack(
          'Import abgeschlossen: ${r['neu'] ?? 0} neu, ${r['geaendert'] ?? 0} geändert.');
      await _load();
    } on ApiException catch (e) {
      _snack('Import abgelehnt, nichts gespeichert: ${e.message}');
    } catch (e) {
      _snack('Import fehlgeschlagen: $e');
    } finally {
      if (mounted) setState(() => importing = false);
    }
  }

  // _dialog: neue Leistung (l == null) oder Leistung aendern.
  Future<void> _dialog({Map<String, dynamic>? l}) async {
    final editing = l != null;
    String text(String key) => l?[key]?.toString() ?? '';
    final codeCtrl = TextEditingController(text: text('code'));
    final kurzCtrl = TextEditingController(text: text('kurztext'));
    final langCtrl = TextEditingController(text: text('langtext'));
    final einheitCtrl = TextEditingController(text: text('einheit'));
    final gewerkCtrl = TextEditingController(text: text('gewerk'));
    final profilCtrl = TextEditingController(text: text('profilserie'));
    final verglasungCtrl = TextEditingController(text: text('verglasung'));
    final uCtrl = TextEditingController(text: formatUWert(l?['u_wert']));
    final brandCtrl = TextEditingController(text: text('brandschutzklasse'));
    String rc =
        leistungRCKlassen.contains(text('rc_klasse')) ? text('rc_klasse') : '';
    bool aktiv = l == null || l['aktiv'] != false;
    String? dialogError;
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: Text(editing ? 'Leistung bearbeiten' : 'Neue Leistung'),
          content: SizedBox(
            width: 520,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(
                      key: const ValueKey('code'),
                      controller: codeCtrl,
                      decoration: const InputDecoration(labelText: 'Code')),
                  TextField(
                      key: const ValueKey('kurztext'),
                      controller: kurzCtrl,
                      decoration: const InputDecoration(labelText: 'Kurztext')),
                  TextField(
                    key: const ValueKey('langtext'),
                    controller: langCtrl,
                    minLines: 2,
                    maxLines: 5,
                    decoration:
                        const InputDecoration(labelText: 'Langtext (optional)'),
                  ),
                  Row(children: [
                    Expanded(
                        child: TextField(
                            key: const ValueKey('einheit'),
                            controller: einheitCtrl,
                            decoration:
                                const InputDecoration(labelText: 'Einheit'))),
                    const SizedBox(width: 12),
                    Expanded(
                        child: TextField(
                            key: const ValueKey('gewerk'),
                            controller: gewerkCtrl,
                            decoration: const InputDecoration(
                                labelText: 'Gewerk (optional)'))),
                  ]),
                  const SizedBox(height: 12),
                  Text('Systemvorgaben (optional)',
                      style: Theme.of(ctx).textTheme.titleSmall),
                  TextField(
                      key: const ValueKey('profilserie'),
                      controller: profilCtrl,
                      decoration:
                          const InputDecoration(labelText: 'Profilserie')),
                  TextField(
                      key: const ValueKey('verglasung'),
                      controller: verglasungCtrl,
                      decoration:
                          const InputDecoration(labelText: 'Verglasung')),
                  Row(children: [
                    Expanded(
                      child: DropdownButtonFormField<String>(
                        key: const ValueKey('rc'),
                        initialValue: rc,
                        decoration:
                            const InputDecoration(labelText: 'RC-Klasse'),
                        items: [
                          for (final k in leistungRCKlassen)
                            DropdownMenuItem(
                                value: k, child: Text(k.isEmpty ? 'keine' : k)),
                        ],
                        onChanged: (v) => setStateDialog(() => rc = v ?? ''),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                        child: TextField(
                            key: const ValueKey('uwert'),
                            controller: uCtrl,
                            decoration: const InputDecoration(
                                labelText: 'U-Wert W/(m²K)'))),
                  ]),
                  TextField(
                      key: const ValueKey('brandschutz'),
                      controller: brandCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Brandschutzklasse')),
                  if (editing)
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      value: aktiv,
                      title: const Text('Aktiv'),
                      subtitle: const Text(
                          'Abgeschaltete Leistungen werden nicht mehr zugeordnet; bestehende Zuordnungen bleiben.'),
                      onChanged: (v) => setStateDialog(() => aktiv = v == true),
                    ),
                  if (dialogError != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(dialogError!,
                          style: const TextStyle(color: Color(0xFFB00020))),
                    ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.of(ctx).pop(),
                child: const Text('Abbrechen')),
            FilledButton(
              onPressed: () {
                final r = buildLeistungBody(
                  code: codeCtrl.text,
                  kurztext: kurzCtrl.text,
                  langtext: langCtrl.text,
                  einheit: einheitCtrl.text,
                  gewerk: gewerkCtrl.text,
                  profilserie: profilCtrl.text,
                  verglasung: verglasungCtrl.text,
                  rcKlasse: rc,
                  uWert: uCtrl.text,
                  brandschutzklasse: brandCtrl.text,
                  aktiv: editing ? aktiv : null,
                );
                if (r.error != null) {
                  setStateDialog(() => dialogError = r.error);
                  return;
                }
                Navigator.of(ctx).pop(r.body);
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
        await widget.api.updateLeistung(l['id'].toString(), body);
        _snack('Leistung gespeichert');
      } else {
        await widget.api.createLeistung(body);
        _snack('Leistung angelegt');
      }
      await _load();
    } on ApiException catch (e) {
      _snack('Nicht gespeichert: ${e.message}');
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Leistungskatalog'),
        actions: [
          if (_canWrite)
            TextButton.icon(
              onPressed: importing ? null : _import,
              icon: const Icon(Icons.upload_file),
              label: Text(importing ? 'Importiert …' : 'CSV importieren'),
            ),
          IconButton(
              onPressed: _load,
              icon: const Icon(Icons.refresh),
              tooltip: 'Neu laden'),
        ],
      ),
      floatingActionButton: _canWrite
          ? FloatingActionButton(
              onPressed: () => _dialog(),
              tooltip: 'Neue Leistung',
              child: const Icon(Icons.add))
          : null,
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const Text(
                'Ziel der Zuordnung von LV-Positionen. Preise stehen nicht im Katalog. '
                'CSV-Import: Kopfzeile mit code;kurztext;einheit und optional langtext, gewerk, profilserie, verglasung, '
                'rc_klasse, u_wert, brandschutzklasse, aktiv - Leistungen werden über den Code angelegt oder geändert.'),
            const SizedBox(height: 8),
            Row(children: [
              Expanded(
                child: TextField(
                  key: const ValueKey('suche'),
                  controller: suchCtrl,
                  decoration: const InputDecoration(
                      labelText: 'Suche (Code oder Kurztext)',
                      prefixIcon: Icon(Icons.search)),
                  onSubmitted: (_) => _load(),
                ),
              ),
              const SizedBox(width: 12),
              FilterChip(
                label: const Text('Abgeschaltete anzeigen'),
                selected: zeigeAbgeschaltete,
                onSelected: (v) {
                  setState(() => zeigeAbgeschaltete = v);
                  _load();
                },
              ),
            ]),
            if (loading) const LinearProgressIndicator(minHeight: 2),
            if (error != null)
              Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
            if (!loading && error == null && leistungen.isEmpty)
              Text(_canWrite
                  ? 'Keine Leistungen. Legen Sie Leistungen an oder importieren Sie eine CSV-Datei.'
                  : 'Keine Leistungen.'),
            for (final item in leistungen)
              Builder(builder: (context) {
                final m = item as Map<String, dynamic>;
                return ListTile(
                  title: Text('${m['code'] ?? ''} – ${m['kurztext'] ?? ''}'),
                  subtitle: Text(leistungDetails(m)),
                  trailing: _canWrite
                      ? IconButton(
                          tooltip: 'Bearbeiten',
                          icon: const Icon(Icons.edit),
                          onPressed: () => _dialog(l: m))
                      : null,
                );
              }),
          ],
        ),
      ),
    );
  }
}
