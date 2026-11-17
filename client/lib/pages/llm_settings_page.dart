import 'package:flutter/material.dart';
import '../api.dart';
import 'llm_profile_dialog.dart';

// Sprachmodell (ADR 0043/0046, Backlog I.3.5b, umgebaut in I.8.2b), Routen
// aus I.8.1b, alles mit settings.manage:
//   - Hauptschalter und Tageslimit gelten fuer alle Profile des Mandanten;
//     einschalten geht nur mit mindestens einem eingeschalteten Profil.
//   - Modellprofile (Anbieter, Endpunkt, Modell, Schluessel, Zeitlimit,
//     Schwelle) im Dialog llm_profile_dialog.dart; das erste wird Standard,
//     der Standard wechselt nur auf ein anderes Profil.
//   - Verbindungstest je Profil mit dem GESPEICHERTEN Stand, auch wenn das
//     Profil ausgeschaltet ist; er zaehlt zum Tageslimit.

const llmAnbieter = {
  'openai_compatible': 'OpenAI-kompatibel (z. B. OpenAI, lokaler Server)',
  'anthropic': 'Anthropic',
  'mistral': 'Mistral',
};

const llmMaxTimeout = 1800;
const llmMaxTageslimit = 100000;

// Baut den Body fuer PUT /settings/llm/mandant oder liefert eine Meldung.
({Map<String, dynamic>? body, String? error}) buildLlmMandantBody({
  required bool aktiv,
  required String tageslimit,
}) {
  int l = 0; // 0 = Standard des Servers
  if (tageslimit.trim().isNotEmpty) {
    final v = int.tryParse(tageslimit.trim());
    if (v == null || v < 1 || v > llmMaxTageslimit) {
      return (
        body: null,
        error: 'Tageslimit muss eine Zahl zwischen 1 und $llmMaxTageslimit sein'
      );
    }
    l = v;
  }
  return (body: {'aktiv': aktiv, 'tageslimit': l}, error: null);
}

// Meldung zum erfolgreichen Verbindungstest (Antwort ohne Inhalt).
String llmTestSummary(Map<String, dynamic> r) {
  final modell = (r['modell'] ?? '').toString();
  return 'Verbindung in Ordnung – Modell ${modell.isEmpty ? 'unbekannt' : modell}, '
      '${r['dauer_ms'] ?? 0} ms, ${r['eingabe_tokens'] ?? 0} Token gesendet, ${r['ausgabe_tokens'] ?? 0} Token erhalten';
}

// Kurzbeschreibung eines Profils fuer die Liste.
String llmProfilZeile(Map<String, dynamic> p) {
  final anbieter = (llmAnbieter[p['anbieter']] ?? p['anbieter'] ?? '')
      .toString()
      .split(' (')
      .first;
  final modell = (p['modell'] ?? '').toString();
  return [
    anbieter,
    if (modell.isNotEmpty) modell,
    'Schwelle ${llmSchwelleText(p['schwelle'])}',
    p['aktiv'] == true ? 'eingeschaltet' : 'ausgeschaltet',
  ].join(' · ');
}

class LlmSettingsPage extends StatefulWidget {
  const LlmSettingsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<LlmSettingsPage> createState() => _LlmSettingsPageState();
}

class _LlmSettingsPageState extends State<LlmSettingsPage> {
  final tageslimitCtrl = TextEditingController();
  bool aktiv = false;
  Map<String, dynamic>? mandant;
  List<Map<String, dynamic>> profile = [];
  bool loading = false;
  bool saving = false;
  String? error;
  String? formError;
  // je Profil-ID: laufender Test und Ergebnis
  final Set<String> testing = {};
  final Map<String, ({String text, bool failed})> testResults = {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    tageslimitCtrl.dispose();
    super.dispose();
  }

  void _applyMandant(Map<String, dynamic> m) {
    mandant = m;
    aktiv = m['aktiv'] == true;
    tageslimitCtrl.text = m['tageslimit']?.toString() ?? '';
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final m = await widget.api.getLlmMandant();
      final list = await widget.api.listLlmProfiles();
      if (!mounted) return;
      setState(() {
        _applyMandant(m);
        profile = list.cast<Map<String, dynamic>>();
      });
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

  Future<void> _reloadProfile() async {
    try {
      final list = await widget.api.listLlmProfiles();
      if (mounted) setState(() => profile = list.cast<Map<String, dynamic>>());
    } on ApiException catch (e) {
      _snack('Profile konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      _snack('Profile konnten nicht geladen werden: $e');
    }
  }

  void _snack(String text) {
    if (mounted)
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _saveMandant() async {
    final r =
        buildLlmMandantBody(aktiv: aktiv, tageslimit: tageslimitCtrl.text);
    if (r.error != null) {
      setState(() => formError = r.error);
      return;
    }
    setState(() {
      saving = true;
      formError = null;
    });
    try {
      final m = await widget.api.saveLlmMandant(r.body!);
      if (!mounted) return;
      setState(() => _applyMandant(m));
      _snack('Einstellungen des Sprachmodells gespeichert');
    } on ApiException catch (e) {
      if (mounted) setState(() => formError = e.message);
    } catch (e) {
      if (mounted) setState(() => formError = '$e');
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  Future<void> _edit([Map<String, dynamic>? p]) async {
    final saved = await showLlmProfileDialog(context, widget.api, profil: p);
    if (saved == null) return;
    _snack(p == null ? 'Profil angelegt' : 'Profil gespeichert');
    await _reloadProfile();
  }

  Future<void> _setStandard(Map<String, dynamic> p) async {
    try {
      await widget.api.setLlmStandardProfile(p['id'].toString());
      _snack('„${p['name']}“ ist jetzt Standard');
    } on ApiException catch (e) {
      _snack(e.message);
    } catch (e) {
      _snack('$e');
    }
    await _reloadProfile();
  }

  Future<void> _test(Map<String, dynamic> p) async {
    final id = p['id'].toString();
    setState(() {
      testing.add(id);
      testResults.remove(id);
    });
    ({String text, bool failed}) result;
    try {
      result = (
        text: llmTestSummary(await widget.api.testLlmProfile(id)),
        failed: false
      );
    } on ApiException catch (e) {
      result = (
        text: e.message.startsWith('Test des Sprachmodells')
            ? e.message
            : 'Test fehlgeschlagen: ${e.message}',
        failed: true
      );
    } catch (e) {
      result = (text: 'Test fehlgeschlagen: $e', failed: true);
    }
    if (!mounted) return;
    setState(() {
      testing.remove(id);
      testResults[id] = result;
    });
    // Der Test zaehlt zum Tageslimit: Aufrufe neu laden, Eingaben bleiben.
    try {
      final m = await widget.api.getLlmMandant();
      if (mounted) setState(() => mandant = m);
    } catch (_) {}
  }

  Widget _warning(String text) => Card(
        color: const Color(0xFFFFF3E0),
        child: ListTile(
            leading: const Icon(Icons.warning_amber), title: Text(text)),
      );

  Widget _profilCard(Map<String, dynamic> p) {
    final id = p['id'].toString();
    final standard = p['standard'] == true;
    final result = testResults[id];
    return Card(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text((p['name'] ?? '').toString(),
                    style: Theme.of(context).textTheme.titleMedium),
                if (standard) const Chip(label: Text('Standard')),
              ],
            ),
            Text(llmProfilZeile(p)),
            if (p['api_key_gesetzt'] == true && p['api_key_lesbar'] != true)
              _warning(
                  'Der gespeicherte API-Schlüssel lässt sich nicht mehr entschlüsseln '
                  '(Schlüssel geändert) – bitte neu eintragen.'),
            Wrap(
              spacing: 8,
              children: [
                TextButton(
                  key: ValueKey('bearbeiten-$id'),
                  onPressed: () => _edit(p),
                  child: const Text('Bearbeiten'),
                ),
                if (!standard)
                  TextButton(
                    key: ValueKey('standard-$id'),
                    onPressed: () => _setStandard(p),
                    child: const Text('Als Standard'),
                  ),
                TextButton(
                  key: ValueKey('testen-$id'),
                  onPressed: testing.contains(id) ? null : () => _test(p),
                  child: Text(
                      testing.contains(id) ? 'Testet …' : 'Verbindung testen'),
                ),
              ],
            ),
            if (result != null)
              Text(result.text,
                  style: TextStyle(
                      color: result.failed
                          ? const Color(0xFFB00020)
                          : const Color(0xFF2E7D32))),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final m = mandant;
    return Scaffold(
      appBar: AppBar(title: const Text('Sprachmodell')),
      body: loading && m == null
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
                    if (m != null && m['schluessel_eingerichtet'] != true)
                      _warning(
                          'Auf dem Server ist kein Schlüssel für verschlüsselte Einstellungen eingerichtet '
                          '(SETTINGS_ENCRYPTION_KEY) – ein API-Schlüssel lässt sich erst danach speichern.'),
                    SwitchListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('Sprachmodell eingeschaltet'),
                      subtitle: const Text(
                          'Gilt für alle Profile; erst mit einem eingeschalteten Profil möglich'),
                      value: aktiv,
                      onChanged: (v) => setState(() => aktiv = v),
                    ),
                    TextField(
                      key: const ValueKey('tageslimit'),
                      controller: tageslimitCtrl,
                      keyboardType: TextInputType.number,
                      decoration: InputDecoration(
                        labelText: 'Tageslimit (Aufrufe, alle Profile)',
                        helperText:
                            'Heute: ${m?['aufrufe_heute'] ?? 0} Aufrufe',
                      ),
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
                        onPressed: saving ? null : _saveMandant,
                        child: Text(saving ? 'Speichert …' : 'Speichern'),
                      ),
                    ),
                    const Divider(height: 32),
                    Row(
                      children: [
                        Expanded(
                          child: Text('Modellprofile',
                              style: Theme.of(context).textTheme.titleMedium),
                        ),
                        OutlinedButton.icon(
                          onPressed: () => _edit(),
                          icon: const Icon(Icons.add),
                          label: const Text('Profil anlegen'),
                        ),
                      ],
                    ),
                    const Text(
                        'Beim Zuordnen wählt der Kalkulator ein Profil, ohne Auswahl gilt das Standardprofil. '
                        'Der Verbindungstest nutzt den gespeicherten Stand, ohne Daten aus dem ERP, und zählt zum Tageslimit.'),
                    const SizedBox(height: 8),
                    if (profile.isEmpty)
                      const Text(
                          'Noch kein Profil angelegt – das erste wird Standard.'),
                    for (final p in profile) _profilCard(p),
                  ],
                ),
    );
  }
}
