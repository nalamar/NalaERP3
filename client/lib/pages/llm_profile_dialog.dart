import 'package:flutter/material.dart';
import '../api.dart';
import 'llm_settings_page.dart' show llmAnbieter, llmMaxTimeout;

// Modellprofil anlegen oder bearbeiten (ADR 0046, Backlog I.8.2a), Routen aus
// I.8.1b. Der Schluessel wird nie angezeigt: leer lassen behaelt den
// gespeicherten, "Gespeicherten Schluessel loeschen" entfernt ihn. Die
// Schwelle (0 bis 1, Standard 0,9) entscheidet spaeter, welche Vorschlaege
// gesammelt bestaetigt werden koennen.

const llmMaxProfilName = 100;

// Schwelle fuer die Anzeige: 0.9 -> "0,9".
String llmSchwelleText(Object? v) =>
    v is num ? v.toString().replaceAll('.', ',') : '';

// Baut den Body fuer POST/PUT /settings/llm/profile oder liefert eine Meldung.
({Map<String, dynamic>? body, String? error}) buildLlmProfileBody({
  required String name,
  required bool aktiv,
  required String anbieter,
  required String endpunkt,
  required String modell,
  required String apiKey,
  required bool apiKeyLoeschen,
  required String timeout,
  required String schwelle,
}) {
  final n = name.trim();
  if (n.isEmpty || n.runes.length > llmMaxProfilName) {
    return (
      body: null,
      error: 'Name muss 1 bis $llmMaxProfilName Zeichen lang sein'
    );
  }
  int t = 0; // 0 = Standard des Servers
  if (timeout.trim().isNotEmpty) {
    final v = int.tryParse(timeout.trim());
    if (v == null || v < 1 || v > llmMaxTimeout) {
      return (
        body: null,
        error:
            'Zeitlimit muss eine Zahl zwischen 1 und $llmMaxTimeout Sekunden sein'
      );
    }
    t = v;
  }
  double? s; // null = Standard des Servers (0,9)
  if (schwelle.trim().isNotEmpty) {
    final v = double.tryParse(schwelle.trim().replaceAll(',', '.'));
    if (v == null || v.isNaN || v < 0 || v > 1) {
      return (
        body: null,
        error: 'Schwelle muss eine Zahl zwischen 0 und 1 sein (z. B. 0,9)'
      );
    }
    s = v;
  }
  if (apiKey.isNotEmpty && apiKeyLoeschen) {
    return (
      body: null,
      error: 'API-Schlüssel entweder neu setzen oder löschen'
    );
  }
  final body = <String, dynamic>{
    'name': n,
    'aktiv': aktiv,
    'anbieter': anbieter,
    'endpunkt': endpunkt.trim(),
    'modell': modell.trim(),
    'timeout_s': t,
    if (s != null) 'schwelle': s,
  };
  // Leer = gespeicherten Schluessel behalten; er wird nicht gekuerzt.
  if (apiKey.isNotEmpty) body['api_key'] = apiKey;
  if (apiKeyLoeschen) body['api_key_loeschen'] = true;
  return (body: body, error: null);
}

// Oeffnet den Dialog; liefert das gespeicherte Profil oder null.
Future<Map<String, dynamic>?> showLlmProfileDialog(
        BuildContext context, ApiClient api,
        {Map<String, dynamic>? profil}) =>
    showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) => LlmProfileDialog(api: api, profil: profil),
    );

class LlmProfileDialog extends StatefulWidget {
  const LlmProfileDialog({super.key, required this.api, this.profil});
  final ApiClient api;
  // null = neues Profil
  final Map<String, dynamic>? profil;

  @override
  State<LlmProfileDialog> createState() => _LlmProfileDialogState();
}

class _LlmProfileDialogState extends State<LlmProfileDialog> {
  late final nameCtrl =
      TextEditingController(text: widget.profil?['name']?.toString() ?? '');
  late final endpunktCtrl =
      TextEditingController(text: widget.profil?['endpunkt']?.toString() ?? '');
  late final modellCtrl =
      TextEditingController(text: widget.profil?['modell']?.toString() ?? '');
  final apiKeyCtrl = TextEditingController();
  late final timeoutCtrl = TextEditingController(
      text: widget.profil?['timeout_s']?.toString() ?? '');
  late final schwelleCtrl =
      TextEditingController(text: llmSchwelleText(widget.profil?['schwelle']));
  late bool aktiv = widget.profil?['aktiv'] == true;
  late String anbieter = llmAnbieter.containsKey(widget.profil?['anbieter'])
      ? widget.profil!['anbieter'].toString()
      : 'openai_compatible';
  bool apiKeyLoeschen = false;
  bool saving = false;
  String? error;

  @override
  void dispose() {
    for (final c in [
      nameCtrl,
      endpunktCtrl,
      modellCtrl,
      apiKeyCtrl,
      timeoutCtrl,
      schwelleCtrl
    ]) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _save() async {
    final r = buildLlmProfileBody(
      name: nameCtrl.text,
      aktiv: aktiv,
      anbieter: anbieter,
      endpunkt: endpunktCtrl.text,
      modell: modellCtrl.text,
      apiKey: apiKeyCtrl.text,
      apiKeyLoeschen: apiKeyLoeschen,
      timeout: timeoutCtrl.text,
      schwelle: schwelleCtrl.text,
    );
    if (r.error != null) {
      setState(() => error = r.error);
      return;
    }
    setState(() {
      saving = true;
      error = null;
    });
    try {
      final id = widget.profil?['id']?.toString();
      final saved = id == null
          ? await widget.api.createLlmProfile(r.body!)
          : await widget.api.updateLlmProfile(id, r.body!);
      if (mounted) Navigator.of(context).pop(saved);
    } on ApiException catch (e) {
      if (mounted) setState(() => error = e.message);
    } catch (e) {
      if (mounted) setState(() => error = '$e');
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final gesetzt = widget.profil?['api_key_gesetzt'] == true;
    final openai = anbieter == 'openai_compatible';
    return AlertDialog(
      title:
          Text(widget.profil == null ? 'Profil anlegen' : 'Profil bearbeiten'),
      content: SizedBox(
        width: 520,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                key: const ValueKey('profilName'),
                controller: nameCtrl,
                decoration: const InputDecoration(
                    labelText: 'Name',
                    helperText: 'z. B. „Claude“ oder „Lokal“'),
              ),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Profil eingeschaltet'),
                subtitle: const Text(
                    'Im Lauf wählbar; erst mit vollständigen Angaben möglich'),
                value: aktiv,
                onChanged: (v) => setState(() => aktiv = v),
              ),
              DropdownButtonFormField<String>(
                key: const ValueKey('profilAnbieter'),
                isExpanded: true,
                initialValue: anbieter,
                decoration: const InputDecoration(labelText: 'Anbieter'),
                items: [
                  for (final e in llmAnbieter.entries)
                    DropdownMenuItem(value: e.key, child: Text(e.value)),
                ],
                onChanged: (v) {
                  if (v != null) setState(() => anbieter = v);
                },
              ),
              TextField(
                key: const ValueKey('profilEndpunkt'),
                controller: endpunktCtrl,
                decoration: InputDecoration(
                  labelText: openai ? 'Endpunkt' : 'Endpunkt (optional)',
                  helperText: openai
                      ? 'z. B. https://api.openai.com/v1 oder http://localhost:11434/v1'
                      : 'Leer lassen für die Adresse des Anbieters',
                ),
              ),
              TextField(
                key: const ValueKey('profilModell'),
                controller: modellCtrl,
                decoration: InputDecoration(
                  labelText: openai ? 'Modell' : 'Modell (optional)',
                  helperText: openai
                      ? null
                      : 'Leer lassen für das Standardmodell der Anbindung',
                ),
              ),
              TextField(
                key: const ValueKey('profilApiKey'),
                controller: apiKeyCtrl,
                obscureText: true,
                enabled: !apiKeyLoeschen,
                decoration: InputDecoration(
                  labelText: 'API-Schlüssel',
                  helperText: gesetzt
                      ? 'Gespeichert – leer lassen, um ihn zu behalten'
                      : 'Noch kein Schlüssel gespeichert',
                ),
              ),
              if (gesetzt)
                CheckboxListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('Gespeicherten API-Schlüssel löschen'),
                  value: apiKeyLoeschen,
                  onChanged: (v) => setState(() {
                    apiKeyLoeschen = v == true;
                    if (apiKeyLoeschen) apiKeyCtrl.clear();
                  }),
                ),
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      key: const ValueKey('profilTimeout'),
                      controller: timeoutCtrl,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                          labelText: 'Zeitlimit je Aufruf (Sekunden)',
                          helperText: 'Standard 300'),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: TextField(
                      key: const ValueKey('profilSchwelle'),
                      controller: schwelleCtrl,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                          labelText: 'Schwelle (0 bis 1)',
                          helperText: 'Standard 0,9'),
                    ),
                  ),
                ],
              ),
              if (error != null)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Text(error!,
                      style: const TextStyle(color: Color(0xFFB00020))),
                ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: saving ? null : () => Navigator.of(context).pop(),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: saving ? null : _save,
          child: Text(saving ? 'Speichert …' : 'Speichern'),
        ),
      ],
    );
  }
}
