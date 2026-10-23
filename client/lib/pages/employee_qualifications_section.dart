import 'package:flutter/material.dart';
import '../api.dart';
import '../web/browser.dart' as browser;

// Qualifikationen am Mitarbeiter (Backlog F.7.3.1): Stand je Art und die
// Anforderungen der Stelle. Vorgabe des Nutzers vom 2026-10-13: die
// Personalabteilung legt fest, welche Qualifikationen fuer die Stelle
// noetig sind; anstehende Erneuerungen mahnt das System an.
//
// Lesen mit qualifications.read, Anforderungen setzen mit
// qualifications.write.

String _date(String? iso) {
  if (iso == null || iso.length < 10) return '';
  final p = iso.substring(0, 10).split('-');
  return p.length == 3 ? '${p[2]}.${p[1]}.${p[0]}' : iso;
}

// qualificationStatusLabel beschreibt den Stand einer Art in Worten.
String qualificationStatusLabel(Map<String, dynamic> st) {
  final bis = _date(st['gueltig_bis']?.toString());
  final tage = st['tage_bis_ablauf'];
  switch (st['status']) {
    case 'fehlt':
      return 'fehlt';
    case 'abgelaufen':
      return 'abgelaufen seit $bis';
    case 'laeuft_ab':
      return tage == 0 ? 'läuft heute ab' : 'läuft ab am $bis (in $tage Tagen)';
    case 'gueltig':
      return bis.isEmpty ? 'gültig, unbefristet' : 'gültig bis $bis';
    default:
      return st['status']?.toString() ?? '';
  }
}

Color _statusColor(String? status) {
  switch (status) {
    case 'fehlt':
    case 'abgelaufen':
      return const Color(0xFFB00020);
    case 'laeuft_ab':
      return const Color(0xFFB26A00);
    default:
      return const Color(0xFF2E7D32);
  }
}

// requirementChoices baut die Auswahl fuer den Dialog: alle aktiven Arten
// des Katalogs, dazu Anforderungen auf inzwischen stillgelegte Arten. Ohne
// die stillgelegten wuerde das Speichern sie stillschweigend entfernen -
// PUT ersetzt die Menge vollstaendig.
List<Map<String, dynamic>> requirementChoices(List<dynamic> activeTypes, List<dynamic> requirements) {
  final out = <Map<String, dynamic>>[
    for (final t in activeTypes)
      {
        'type_id': (t as Map<String, dynamic>)['id'].toString(),
        'name': t['name']?.toString() ?? '',
        'pflicht_fuer_alle': t['pflicht_fuer_alle'] == true,
        'stillgelegt': false,
      },
  ];
  final known = out.map((c) => c['type_id']).toSet();
  for (final r in requirements) {
    final m = r as Map<String, dynamic>;
    final id = m['type_id'].toString();
    if (!known.contains(id)) {
      out.add({'type_id': id, 'name': m['name']?.toString() ?? id, 'pflicht_fuer_alle': false, 'stillgelegt': true});
    }
  }
  out.sort((a, b) => (a['name'] as String).toLowerCase().compareTo((b['name'] as String).toLowerCase()));
  return out;
}

// parseDateInput liest ein Datum als TT.MM.JJJJ (auch T.M.JJJJ) oder
// JJJJ-MM-TT und liefert es als JJJJ-MM-TT; null bei leerer oder
// ungueltiger Eingabe (z. B. 31.02.2026).
String? parseDateInput(String input) {
  final t = input.trim();
  int? y, m, d;
  final de = RegExp(r'^(\d{1,2})\.(\d{1,2})\.(\d{4})$').firstMatch(t);
  final iso = RegExp(r'^(\d{4})-(\d{2})-(\d{2})$').firstMatch(t);
  if (de != null) {
    d = int.parse(de.group(1)!);
    m = int.parse(de.group(2)!);
    y = int.parse(de.group(3)!);
  } else if (iso != null) {
    y = int.parse(iso.group(1)!);
    m = int.parse(iso.group(2)!);
    d = int.parse(iso.group(3)!);
  } else {
    return null;
  }
  final dt = DateTime.utc(y, m, d);
  if (dt.year != y || dt.month != m || dt.day != d) return null;
  String two(int v) => v.toString().padLeft(2, '0');
  return '${dt.year.toString().padLeft(4, '0')}-${two(dt.month)}-${two(dt.day)}';
}

// buildQualificationBody baut den Body fuer POST /qualifications. Leere
// optionale Angaben werden weggelassen; fehlt gueltig_bis, setzt der
// Server es aus der Frist der Art.
Map<String, dynamic> buildQualificationBody({
  required String employeeId,
  required String typeId,
  required String erworbenAm,
  String? gueltigBis,
  String aussteller = '',
  String nachweisNr = '',
  String note = '',
}) {
  final body = <String, dynamic>{
    'employee_ids': [employeeId],
    'type_id': typeId,
    'erworben_am': erworbenAm,
  };
  if (gueltigBis != null && gueltigBis.isNotEmpty) body['gueltig_bis'] = gueltigBis;
  if (aussteller.trim().isNotEmpty) body['aussteller'] = aussteller.trim();
  if (nachweisNr.trim().isNotEmpty) body['nachweis_nr'] = nachweisNr.trim();
  if (note.trim().isNotEmpty) body['note'] = note.trim();
  return body;
}

// qualificationRecordLabel beschreibt einen Nachweis in einer Zeile.
String qualificationRecordLabel(Map<String, dynamic> r) {
  final parts = <String>['erworben ${_date(r['erworben_am']?.toString())}'];
  final bis = _date(r['gueltig_bis']?.toString());
  parts.add(bis.isEmpty ? 'unbefristet' : 'gültig bis $bis');
  final aussteller = r['aussteller']?.toString() ?? '';
  if (aussteller.isNotEmpty) parts.add(aussteller);
  return parts.join(' · ');
}

// Nachweisdokumente (Backlog F.10.2): Scan des Zertifikats, unterschriebene
// Teilnehmerliste. Der Server nimmt PDF/JPEG/PNG bis 10 MB an (F.10.1) und
// erkennt die Art am Inhalt; der Client prueft vorab nur die Groesse, damit
// eine zu grosse Datei nicht erst hochgeladen wird.
const maxQualificationDocumentBytes = 10 * 1024 * 1024;
const qualificationDocumentAccept = '.pdf,.jpg,.jpeg,.png,application/pdf,image/jpeg,image/png';

typedef QualificationFilePicker = Future<browser.PickedFile?> Function({String? accept});

// formatFileSize schreibt eine Dateigroesse deutsch: Bytes, KB (ganz), MB
// (eine Nachkommastelle mit Komma).
String formatFileSize(int bytes) {
  if (bytes < 1024) return '$bytes B';
  if (bytes < 1024 * 1024) return '${(bytes / 1024).round()} KB';
  return '${(bytes / (1024 * 1024)).toStringAsFixed(1).replaceAll('.', ',')} MB';
}

// qualificationUploadProblem: Grund, warum eine Datei nicht hochgeladen
// wird, sonst null.
String? qualificationUploadProblem(int length) {
  if (length == 0) return 'Die Datei ist leer.';
  if (length > maxQualificationDocumentBytes) {
    return 'Die Datei ist zu groß (${formatFileSize(length)}, höchstens 10 MB).';
  }
  return null;
}

// qualificationDocumentLabel beschreibt ein Dokument in einer Zeile.
String qualificationDocumentLabel(Map<String, dynamic> d) {
  final parts = <String>[d['filename']?.toString() ?? ''];
  final len = d['length'];
  if (len is int) parts.add(formatFileSize(len));
  final am = _date(d['uploaded_at']?.toString());
  if (am.isNotEmpty) parts.add('hochgeladen $am');
  return parts.join(' · ');
}

class EmployeeQualificationsSection extends StatefulWidget {
  const EmployeeQualificationsSection({super.key, required this.api, required this.employeeId, this.filePicker});
  final ApiClient api;
  final String employeeId;
  // Fuer Tests austauschbar; sonst die Dateiauswahl des Browsers.
  final QualificationFilePicker? filePicker;

  @override
  State<EmployeeQualificationsSection> createState() => _EmployeeQualificationsSectionState();
}

class _EmployeeQualificationsSectionState extends State<EmployeeQualificationsSection> {
  List<dynamic> standing = [];
  // Aktive Nachweise, die juengsten zuerst (fuer Anzeige und Storno).
  List<dynamic> records = [];
  // Namen aller Arten, auch stillgelegter - die Nachweise tragen nur die ID.
  Map<String, String> typeNames = {};
  bool loading = false;
  String? error;

  bool get _canWrite => widget.api.hasPermission('qualifications.write');

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
      final st = await widget.api.getQualificationStanding(widget.employeeId);
      final recs = await widget.api.listEmployeeQualifications(widget.employeeId);
      Map<String, String> names = typeNames;
      try {
        final types = await widget.api.listQualificationTypes(includeInactive: true);
        names = {
          for (final t in types)
            (t as Map<String, dynamic>)['id'].toString(): t['name']?.toString() ?? t['id'].toString(),
        };
      } catch (_) {
        // Ohne Katalog bleibt die Liste benutzbar; angezeigt wird dann die ID.
      }
      if (mounted) {
        setState(() {
          standing = st;
          records = recs;
          typeNames = names;
        });
      }
    } catch (e) {
      if (mounted) setState(() => error = 'Qualifikationen konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> _editRequirements() async {
    List<dynamic> types;
    List<dynamic> requirements;
    try {
      types = await widget.api.listQualificationTypes();
      requirements = await widget.api.listQualificationRequirements(widget.employeeId);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      return;
    }
    if (!mounted) return;
    final choices = requirementChoices(types, requirements);
    final selected = <String>{for (final r in requirements) (r as Map<String, dynamic>)['type_id'].toString()};
    final result = await showDialog<Set<String>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Anforderungen der Stelle'),
          content: SizedBox(
            width: 460,
            child: choices.isEmpty
                ? const Text('Der Katalog ist leer. Arten legt die Personalabteilung im Qualifikationskatalog an.')
                : SingleChildScrollView(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        for (final c in choices)
                          CheckboxListTile(
                            dense: true,
                            value: selected.contains(c['type_id']),
                            title: Text(c['name'] as String),
                            subtitle: c['stillgelegt'] == true
                                ? const Text('stillgelegt – zählt im Stand nicht mehr')
                                : (c['pflicht_fuer_alle'] == true ? const Text('ohnehin Pflicht für alle') : null),
                            onChanged: (v) => setStateDialog(() {
                              if (v == true) {
                                selected.add(c['type_id'] as String);
                              } else {
                                selected.remove(c['type_id']);
                              }
                            }),
                          ),
                      ],
                    ),
                  ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(onPressed: () => Navigator.of(ctx).pop(Set<String>.from(selected)), child: const Text('Speichern')),
          ],
        ),
      ),
    );
    if (result == null) return;
    try {
      final ids = result.toList()..sort();
      await widget.api.setQualificationRequirements(widget.employeeId, ids);
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _recordDialog() async {
    List<dynamic> types;
    try {
      types = await widget.api.listQualificationTypes();
    } catch (e) {
      _snack('Fehler: $e');
      return;
    }
    if (!mounted) return;
    if (types.isEmpty) {
      _snack('Der Katalog ist leer. Arten legt die Personalabteilung im Qualifikationskatalog an.');
      return;
    }
    String? typeId;
    String? dialogError;
    final erworbenCtrl = TextEditingController();
    final bisCtrl = TextEditingController();
    final ausstellerCtrl = TextEditingController();
    final nrCtrl = TextEditingController();
    final noteCtrl = TextEditingController();
    final body = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Nachweis erfassen'),
          content: SizedBox(
            width: 420,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  DropdownButtonFormField<String>(
                    initialValue: typeId,
                    decoration: const InputDecoration(labelText: 'Qualifikation / Unterweisung'),
                    items: [
                      for (final t in types)
                        DropdownMenuItem<String>(
                          value: (t as Map<String, dynamic>)['id'].toString(),
                          child: Text(t['name']?.toString() ?? t['id'].toString()),
                        ),
                    ],
                    onChanged: (v) => setStateDialog(() => typeId = v),
                  ),
                  TextField(controller: erworbenCtrl, decoration: const InputDecoration(labelText: 'Erworben am (TT.MM.JJJJ)')),
                  TextField(
                      controller: bisCtrl,
                      decoration: const InputDecoration(labelText: 'Gültig bis (TT.MM.JJJJ, leer = nach Frist der Art)')),
                  TextField(controller: ausstellerCtrl, decoration: const InputDecoration(labelText: 'Aussteller')),
                  TextField(controller: nrCtrl, decoration: const InputDecoration(labelText: 'Nachweis-Nr.')),
                  TextField(controller: noteCtrl, decoration: const InputDecoration(labelText: 'Notiz')),
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
                final erworben = parseDateInput(erworbenCtrl.text);
                final bisText = bisCtrl.text.trim();
                final bis = bisText.isEmpty ? null : parseDateInput(bisText);
                String? problem;
                if (typeId == null) {
                  problem = 'Bitte eine Qualifikation auswählen.';
                } else if (erworben == null) {
                  problem = 'Bitte das Erwerbsdatum als TT.MM.JJJJ angeben.';
                } else if (bisText.isNotEmpty && bis == null) {
                  problem = 'Bitte "Gültig bis" als TT.MM.JJJJ angeben oder leer lassen.';
                }
                if (problem != null) {
                  setStateDialog(() => dialogError = problem);
                  return;
                }
                Navigator.of(ctx).pop(buildQualificationBody(
                  employeeId: widget.employeeId,
                  typeId: typeId!,
                  erworbenAm: erworben!,
                  gueltigBis: bis,
                  aussteller: ausstellerCtrl.text,
                  nachweisNr: nrCtrl.text,
                  note: noteCtrl.text,
                ));
              },
              child: const Text('Erfassen'),
            ),
          ],
        ),
      ),
    );
    if (body == null) return;
    try {
      await widget.api.createQualification(body);
      _snack('Nachweis erfasst');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _stornoDialog(Map<String, dynamic> record) async {
    final reasonCtrl = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) => AlertDialog(
          title: const Text('Nachweis stornieren'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${typeNames[record['type_id']?.toString()] ?? record['type_id']} – ${qualificationRecordLabel(record)}'),
                const SizedBox(height: 8),
                const Text('Ein Nachweis wird nicht gelöscht, sondern mit Begründung storniert.'),
                TextField(
                  controller: reasonCtrl,
                  decoration: const InputDecoration(labelText: 'Begründung'),
                  onChanged: (_) => setStateDialog(() {}),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
            FilledButton(
              onPressed: reasonCtrl.text.trim().isEmpty ? null : () => Navigator.of(ctx).pop(reasonCtrl.text.trim()),
              child: const Text('Stornieren'),
            ),
          ],
        ),
      ),
    );
    if (reason == null) return;
    try {
      await widget.api.stornoQualification(record['id'].toString(), reason);
      _snack('Nachweis storniert');
      await _load();
    } catch (e) {
      _snack('Fehler: $e');
    }
  }

  Future<void> _documentsDialog(Map<String, dynamic> record) async {
    final qualId = record['id'].toString();
    final title = '${typeNames[record['type_id']?.toString()] ?? record['type_id']} – ${qualificationRecordLabel(record)}';
    List<dynamic> docs = [];
    String? dialogError;
    bool busy = true;
    bool started = false;
    await showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setStateDialog) {
          // Laden und Hochladen laufen asynchron weiter, auch wenn der
          // Dialog schon geschlossen ist.
          void update(VoidCallback f) {
            if (ctx.mounted) setStateDialog(f);
          }

          Future<void> reload() async {
            try {
              final out = await widget.api.listQualificationDocuments(qualId);
              update(() {
                docs = out;
                dialogError = null;
              });
            } catch (e) {
              update(() => dialogError = 'Dokumente konnten nicht geladen werden: $e');
            } finally {
              update(() => busy = false);
            }
          }

          Future<void> upload() async {
            final pick = widget.filePicker ?? browser.pickFile;
            final picked = await pick(accept: qualificationDocumentAccept);
            if (picked == null) return;
            final problem = qualificationUploadProblem(picked.bytes.length);
            if (problem != null) {
              update(() => dialogError = problem);
              return;
            }
            update(() {
              busy = true;
              dialogError = null;
            });
            try {
              await widget.api.uploadQualificationDocument(qualId, picked.filename, picked.bytes, contentType: picked.contentType);
            } catch (e) {
              update(() {
                busy = false;
                dialogError = 'Upload fehlgeschlagen: $e';
              });
              return;
            }
            await reload();
          }

          Future<void> download(Map<String, dynamic> d) async {
            try {
              await widget.api.downloadQualificationDocument(qualId, d['id'].toString(), filename: d['filename']?.toString());
            } catch (e) {
              update(() => dialogError = 'Download fehlgeschlagen: $e');
            }
          }

          if (!started) {
            started = true;
            reload();
          }
          return AlertDialog(
            title: const Text('Nachweisdokumente'),
            content: SizedBox(
              width: 480,
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title),
                    const SizedBox(height: 8),
                    if (busy) const LinearProgressIndicator(minHeight: 2),
                    if (!busy && docs.isEmpty && dialogError == null) const Text('Keine Dokumente.'),
                    for (final d in docs)
                      Row(
                        children: [
                          Expanded(
                            child: Text(qualificationDocumentLabel(d as Map<String, dynamic>), overflow: TextOverflow.ellipsis),
                          ),
                          IconButton(
                            tooltip: 'Herunterladen',
                            icon: const Icon(Icons.download, size: 18),
                            onPressed: () => download(d),
                          ),
                        ],
                      ),
                    if (_canWrite) ...[
                      const SizedBox(height: 8),
                      const Text('PDF, JPEG oder PNG, höchstens 10 MB. Dokumente werden nicht gelöscht.'),
                    ],
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
              if (_canWrite)
                TextButton.icon(
                  onPressed: busy ? null : upload,
                  icon: const Icon(Icons.upload_file),
                  label: const Text('Datei hochladen'),
                ),
              FilledButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Schließen')),
            ],
          );
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Qualifikationen', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
        if (_canWrite)
          Wrap(
            spacing: 8,
            children: [
              TextButton.icon(
                onPressed: _recordDialog,
                icon: const Icon(Icons.add_task),
                label: const Text('Nachweis erfassen'),
              ),
              TextButton.icon(
                onPressed: _editRequirements,
                icon: const Icon(Icons.checklist),
                label: const Text('Anforderungen festlegen'),
              ),
            ],
          ),
        if (loading) const LinearProgressIndicator(minHeight: 2),
        if (error != null) Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (!loading && error == null && standing.isEmpty)
          const Text('Keine Anforderungen und keine Nachweise.'),
        for (final s in standing)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 2),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    '${(s as Map<String, dynamic>)['name'] ?? ''}${s['erforderlich'] == true ? ' (erforderlich)' : ''}',
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                const SizedBox(width: 8),
                Text(qualificationStatusLabel(s), style: TextStyle(color: _statusColor(s['status']?.toString()))),
              ],
            ),
          ),
        if (records.isNotEmpty) ...[
          const SizedBox(height: 12),
          const Text('Nachweise', style: TextStyle(fontWeight: FontWeight.bold)),
          for (final r in records)
            Row(
              children: [
                Expanded(
                  child: Text(
                    '${typeNames[(r as Map<String, dynamic>)['type_id']?.toString()] ?? r['type_id']}: ${qualificationRecordLabel(r)}',
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                IconButton(
                  tooltip: 'Dokumente',
                  icon: const Icon(Icons.attach_file, size: 18),
                  onPressed: () => _documentsDialog(r),
                ),
                if (_canWrite)
                  IconButton(
                    tooltip: 'Stornieren',
                    icon: const Icon(Icons.block, size: 18),
                    onPressed: () => _stornoDialog(r),
                  ),
              ],
            ),
        ],
      ],
    );
  }
}
