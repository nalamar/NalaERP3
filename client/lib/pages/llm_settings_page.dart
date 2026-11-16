import 'package:flutter/material.dart';
import '../api.dart';

// Sprachmodell (ADR 0043, Backlog I.3.5b), Routen aus I.3.5a2, alles mit
// settings.manage:
//   - Anbieter (OpenAI-kompatibel, Anthropic, Mistral), Endpunkt, Modell,
//     API-Schluessel, Zeitlimit je Aufruf, Tageslimit, ein/aus.
//   - Der Schluessel wird nie angezeigt: leer lassen behaelt den
//     gespeicherten, "Gespeicherten Schluessel loeschen" entfernt ihn.
//   - Verbindungstest mit den GESPEICHERTEN Einstellungen, auch wenn das
//     Sprachmodell ausgeschaltet ist; er zaehlt zum Tageslimit.

const llmAnbieter = {
  'openai_compatible': 'OpenAI-kompatibel (z. B. OpenAI, lokaler Server)',
  'anthropic': 'Anthropic',
  'mistral': 'Mistral',
};

const llmMaxTimeout = 1800;
const llmMaxTageslimit = 100000;

int? _wholeNumber(String text, {required int max}) {
  final t = text.trim();
  if (t.isEmpty) return 0; // 0 = Standard des Servers
  final n = int.tryParse(t);
  if (n == null || n < 1 || n > max) return null;
  return n;
}

// Baut den Body fuer PUT /settings/llm oder liefert eine Meldung.
({Map<String, dynamic>? body, String? error}) buildLlmSettingsBody({
  required bool aktiv,
  required String anbieter,
  required String endpunkt,
  required String modell,
  required String apiKey,
  required bool apiKeyLoeschen,
  required String timeout,
  required String tageslimit,
}) {
  final t = _wholeNumber(timeout, max: llmMaxTimeout);
  if (t == null) {
    return (
      body: null,
      error:
          'Zeitlimit muss eine Zahl zwischen 1 und $llmMaxTimeout Sekunden sein'
    );
  }
  final l = _wholeNumber(tageslimit, max: llmMaxTageslimit);
  if (l == null) {
    return (
      body: null,
      error: 'Tageslimit muss eine Zahl zwischen 1 und $llmMaxTageslimit sein'
    );
  }
  if (apiKey.isNotEmpty && apiKeyLoeschen) {
    return (
      body: null,
      error: 'API-Schlüssel entweder neu setzen oder löschen'
    );
  }
  final body = <String, dynamic>{
    'aktiv': aktiv,
    'anbieter': anbieter,
    'endpunkt': endpunkt.trim(),
    'modell': modell.trim(),
    'timeout_s': t,
    'tageslimit': l,
  };
  // Leer = gespeicherten Schluessel behalten; er wird nicht gekuerzt.
  if (apiKey.isNotEmpty) body['api_key'] = apiKey;
  if (apiKeyLoeschen) body['api_key_loeschen'] = true;
  return (body: body, error: null);
}

// Meldung zum erfolgreichen Verbindungstest (Antwort ohne Inhalt).
String llmTestSummary(Map<String, dynamic> r) {
  final modell = (r['modell'] ?? '').toString();
  return 'Verbindung in Ordnung – Modell ${modell.isEmpty ? 'unbekannt' : modell}, '
      '${r['dauer_ms'] ?? 0} ms, ${r['eingabe_tokens'] ?? 0} Token gesendet, ${r['ausgabe_tokens'] ?? 0} Token erhalten';
}

class LlmSettingsPage extends StatefulWidget {
  const LlmSettingsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<LlmSettingsPage> createState() => _LlmSettingsPageState();
}

class _LlmSettingsPageState extends State<LlmSettingsPage> {
  final endpunktCtrl = TextEditingController();
  final modellCtrl = TextEditingController();
  final apiKeyCtrl = TextEditingController();
  final timeoutCtrl = TextEditingController();
  final tageslimitCtrl = TextEditingController();
  bool aktiv = false;
  String anbieter = 'openai_compatible';
  bool apiKeyLoeschen = false;
  Map<String, dynamic>? stand;
  bool loading = false;
  bool saving = false;
  bool testing = false;
  String? error;
  String? formError;
  String? testResult;
  bool testFailed = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    for (final c in [
      endpunktCtrl,
      modellCtrl,
      apiKeyCtrl,
      timeoutCtrl,
      tageslimitCtrl
    ]) {
      c.dispose();
    }
    super.dispose();
  }

  void _apply(Map<String, dynamic> s) {
    stand = s;
    aktiv = s['aktiv'] == true;
    anbieter = llmAnbieter.containsKey(s['anbieter'])
        ? s['anbieter'].toString()
        : 'openai_compatible';
    endpunktCtrl.text = s['endpunkt']?.toString() ?? '';
    modellCtrl.text = s['modell']?.toString() ?? '';
    timeoutCtrl.text = s['timeout_s']?.toString() ?? '';
    tageslimitCtrl.text = s['tageslimit']?.toString() ?? '';
    apiKeyCtrl.clear();
    apiKeyLoeschen = false;
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final s = await widget.api.getLlmSettings();
      if (!mounted) return;
      setState(() => _apply(s));
    } on ApiException catch (e) {
      if (mounted)
        setState(() =>
            error = 'Einstellungen konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted)
        setState(
            () => error = 'Einstellungen konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted)
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _save() async {
    final r = buildLlmSettingsBody(
      aktiv: aktiv,
      anbieter: anbieter,
      endpunkt: endpunktCtrl.text,
      modell: modellCtrl.text,
      apiKey: apiKeyCtrl.text,
      apiKeyLoeschen: apiKeyLoeschen,
      timeout: timeoutCtrl.text,
      tageslimit: tageslimitCtrl.text,
    );
    if (r.error != null) {
      setState(() => formError = r.error);
      return;
    }
    setState(() {
      saving = true;
      formError = null;
    });
    try {
      final s = await widget.api.saveLlmSettings(r.body!);
      if (!mounted) return;
      setState(() => _apply(s));
      _snack('Einstellungen des Sprachmodells gespeichert');
    } on ApiException catch (e) {
      if (mounted) setState(() => formError = e.message);
    } catch (e) {
      if (mounted) setState(() => formError = '$e');
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  Future<void> _test() async {
    setState(() {
      testing = true;
      testResult = null;
    });
    try {
      final r = await widget.api.testLlmSettings();
      if (!mounted) return;
      setState(() {
        testFailed = false;
        testResult = llmTestSummary(r);
      });
    } on ApiException catch (e) {
      if (mounted) {
        setState(() {
          testFailed = true;
          testResult = e.message.startsWith('Test des Sprachmodells')
              ? e.message
              : 'Test fehlgeschlagen: ${e.message}';
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          testFailed = true;
          testResult = 'Test fehlgeschlagen: $e';
        });
      }
    } finally {
      if (mounted) setState(() => testing = false);
    }
    // Der Test zaehlt zum Tageslimit: Stand neu laden, Eingaben bleiben.
    try {
      final s = await widget.api.getLlmSettings();
      if (mounted) setState(() => stand = s);
    } catch (_) {}
  }

  Widget _warning(String text) => Card(
        color: const Color(0xFFFFF3E0),
        child: ListTile(
            leading: const Icon(Icons.warning_amber), title: Text(text)),
      );

  @override
  Widget build(BuildContext context) {
    final s = stand;
    final gesetzt = s?['api_key_gesetzt'] == true;
    final lesbar = s?['api_key_lesbar'] == true;
    final schluessel = s?['schluessel_eingerichtet'] == true;
    final openai = anbieter == 'openai_compatible';
    return Scaffold(
      appBar: AppBar(title: const Text('Sprachmodell')),
      body: loading && s == null
          ? const Center(child: CircularProgressIndicator())
          : error != null
              ? Center(child: Text(error!))
              : ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    const Text(
                        'Das Sprachmodell hilft beim Zuordnen von Leistungsverzeichnis-Positionen. '
                        'Jeder Vorschlag braucht eine Freigabe; Preise kommen nie vom Modell.'),
                    const SizedBox(height: 8),
                    if (s != null && !schluessel)
                      _warning(
                          'Auf dem Server ist kein Schlüssel für verschlüsselte Einstellungen eingerichtet '
                          '(SETTINGS_ENCRYPTION_KEY) – ein API-Schlüssel lässt sich erst danach speichern.'),
                    if (gesetzt && !lesbar)
                      _warning(
                          'Der gespeicherte API-Schlüssel lässt sich nicht mehr entschlüsseln '
                          '(Schlüssel geändert) – bitte neu eintragen.'),
                    SwitchListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('Sprachmodell eingeschaltet'),
                      subtitle: const Text(
                          'Erst mit vollständigen Einstellungen möglich'),
                      value: aktiv,
                      onChanged: (v) => setState(() => aktiv = v),
                    ),
                    DropdownButtonFormField<String>(
                      key: const ValueKey('anbieter'),
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
                      key: const ValueKey('endpunkt'),
                      controller: endpunktCtrl,
                      decoration: InputDecoration(
                        labelText: openai ? 'Endpunkt' : 'Endpunkt (optional)',
                        helperText: openai
                            ? 'z. B. https://api.openai.com/v1 oder http://localhost:11434/v1'
                            : 'Leer lassen für die Adresse des Anbieters',
                      ),
                    ),
                    TextField(
                      key: const ValueKey('modell'),
                      controller: modellCtrl,
                      decoration: InputDecoration(
                        labelText: openai ? 'Modell' : 'Modell (optional)',
                        helperText: openai
                            ? null
                            : 'Leer lassen für das Standardmodell der Anbindung',
                      ),
                    ),
                    TextField(
                      key: const ValueKey('apiKey'),
                      controller: apiKeyCtrl,
                      obscureText: true,
                      enabled: !apiKeyLoeschen,
                      decoration: InputDecoration(
                        labelText: 'API-Schlüssel',
                        helperText: gesetzt
                            ? 'Gespeichert – leer lassen, um ihn zu behalten'
                            : openai
                                ? 'Noch kein Schlüssel gespeichert (bei lokalen Servern oft nicht nötig)'
                                : 'Noch kein Schlüssel gespeichert',
                      ),
                    ),
                    if (gesetzt)
                      CheckboxListTile(
                        contentPadding: EdgeInsets.zero,
                        title:
                            const Text('Gespeicherten API-Schlüssel löschen'),
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
                            key: const ValueKey('timeout'),
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
                            key: const ValueKey('tageslimit'),
                            controller: tageslimitCtrl,
                            keyboardType: TextInputType.number,
                            decoration: InputDecoration(
                              labelText: 'Tageslimit (Aufrufe)',
                              helperText:
                                  'Heute: ${s?['aufrufe_heute'] ?? 0} Aufrufe',
                            ),
                          ),
                        ),
                      ],
                    ),
                    if (formError != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: Text(formError!,
                            style: const TextStyle(color: Color(0xFFB00020))),
                      ),
                    const SizedBox(height: 12),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: FilledButton(
                        onPressed: saving ? null : _save,
                        child: Text(saving ? 'Speichert …' : 'Speichern'),
                      ),
                    ),
                    const Divider(height: 32),
                    Text('Verbindungstest',
                        style: Theme.of(context).textTheme.titleMedium),
                    const Text(
                        'Schickt eine kurze Anfrage ohne Daten aus dem ERP mit den gespeicherten Einstellungen – '
                        'vorher speichern. Zählt zum Tageslimit.'),
                    const SizedBox(height: 8),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: OutlinedButton(
                        onPressed: testing ? null : _test,
                        child: Text(testing ? 'Testet …' : 'Verbindung testen'),
                      ),
                    ),
                    if (testResult != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: Text(testResult!,
                            style: TextStyle(
                                color: testFailed
                                    ? const Color(0xFFB00020)
                                    : const Color(0xFF2E7D32))),
                      ),
                  ],
                ),
    );
  }
}
