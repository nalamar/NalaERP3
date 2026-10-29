import 'package:flutter/material.dart';
import '../api.dart';
import 'notice_mail_page.dart';

// E-Mail-Versand (ADR 0033, Backlog F.14.3), Routen aus F.14.2c, alles mit
// settings.manage:
//   - Zugang: Mail-Server, Verschluesselung (SSL 465 / STARTTLS 587), Port,
//     Benutzer, Passwort, Absender, Versand ein/aus.
//   - Das Passwort wird nie angezeigt: leer lassen behaelt das gespeicherte,
//     "Gespeichertes Passwort loeschen" entfernt es.
//   - Testversand mit dem GESPEICHERTEN Zugang, auch bei ausgeschaltetem
//     Versand; ist eine Umleitung eingerichtet, sagt die Meldung das.

const mailSicherheiten = {
  'ssl': 'SSL/TLS (Port 465)',
  'starttls': 'STARTTLS (Port 587)',
};

const mailStandardPorts = {'ssl': 465, 'starttls': 587};

// Baut den Body fuer PUT /settings/mail oder liefert eine Meldung.
({Map<String, dynamic>? body, String? error}) buildMailSettingsBody({
  required bool aktiv,
  required String host,
  required String sicherheit,
  required String port,
  required String benutzer,
  required String passwort,
  required bool passwortLoeschen,
  required String absenderAdresse,
  required String absenderName,
}) {
  final p = port.trim().isEmpty ? 0 : int.tryParse(port.trim());
  if (p == null || p < 0 || p > 65535) {
    return (body: null, error: 'Port muss eine Zahl zwischen 1 und 65535 sein');
  }
  if (passwort.isNotEmpty && passwortLoeschen) {
    return (body: null, error: 'Passwort entweder neu setzen oder löschen');
  }
  final body = <String, dynamic>{
    'aktiv': aktiv,
    'smtp_host': host.trim(),
    'smtp_port': p,
    'sicherheit': sicherheit,
    'benutzer': benutzer.trim(),
    'absender_adresse': absenderAdresse.trim(),
    'absender_name': absenderName.trim(),
  };
  // Leer = gespeichertes Passwort behalten; es wird nicht gekuerzt.
  if (passwort.isNotEmpty) body['passwort'] = passwort;
  if (passwortLoeschen) body['passwort_loeschen'] = true;
  return (body: body, error: null);
}

class MailSettingsPage extends StatefulWidget {
  const MailSettingsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<MailSettingsPage> createState() => _MailSettingsPageState();
}

class _MailSettingsPageState extends State<MailSettingsPage> {
  final hostCtrl = TextEditingController();
  final portCtrl = TextEditingController();
  final benutzerCtrl = TextEditingController();
  final passwortCtrl = TextEditingController();
  final absenderCtrl = TextEditingController();
  final absenderNameCtrl = TextEditingController();
  final testAnCtrl = TextEditingController();
  bool aktiv = false;
  String sicherheit = 'ssl';
  bool passwortLoeschen = false;
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
    for (final c in [hostCtrl, portCtrl, benutzerCtrl, passwortCtrl, absenderCtrl, absenderNameCtrl, testAnCtrl]) {
      c.dispose();
    }
    super.dispose();
  }

  void _apply(Map<String, dynamic> s) {
    stand = s;
    aktiv = s['aktiv'] == true;
    sicherheit = mailSicherheiten.containsKey(s['sicherheit']) ? s['sicherheit'].toString() : 'ssl';
    hostCtrl.text = s['smtp_host']?.toString() ?? '';
    portCtrl.text = s['smtp_port']?.toString() ?? '';
    benutzerCtrl.text = s['benutzer']?.toString() ?? '';
    absenderCtrl.text = s['absender_adresse']?.toString() ?? '';
    absenderNameCtrl.text = s['absender_name']?.toString() ?? '';
    passwortCtrl.clear();
    passwortLoeschen = false;
  }

  Future<void> _load() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final s = await widget.api.getMailSettings();
      if (!mounted) return;
      setState(() => _apply(s));
    } on ApiException catch (e) {
      if (mounted) setState(() => error = 'Einstellungen konnten nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted) setState(() => error = 'Einstellungen konnten nicht geladen werden: $e');
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  // Wechsel der Verschluesselung: ein Standard-Port wandert mit.
  void _setSicherheit(String v) {
    setState(() {
      final old = mailStandardPorts[sicherheit].toString();
      if (portCtrl.text.trim().isEmpty || portCtrl.text.trim() == old) {
        portCtrl.text = mailStandardPorts[v].toString();
      }
      sicherheit = v;
    });
  }

  Future<void> _save() async {
    final r = buildMailSettingsBody(
      aktiv: aktiv,
      host: hostCtrl.text,
      sicherheit: sicherheit,
      port: portCtrl.text,
      benutzer: benutzerCtrl.text,
      passwort: passwortCtrl.text,
      passwortLoeschen: passwortLoeschen,
      absenderAdresse: absenderCtrl.text,
      absenderName: absenderNameCtrl.text,
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
      final s = await widget.api.saveMailSettings(r.body!);
      if (!mounted) return;
      setState(() => _apply(s));
      _snack('E-Mail-Einstellungen gespeichert');
    } on ApiException catch (e) {
      if (mounted) setState(() => formError = e.message);
    } catch (e) {
      if (mounted) setState(() => formError = '$e');
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  Future<void> _test() async {
    final an = testAnCtrl.text.trim();
    if (an.isEmpty) {
      setState(() {
        testResult = 'Empfänger erforderlich';
        testFailed = true;
      });
      return;
    }
    setState(() {
      testing = true;
      testResult = null;
    });
    try {
      final r = await widget.api.sendMailTest(an);
      if (!mounted) return;
      setState(() {
        testFailed = false;
        testResult = r['umgeleitet'] == true
            ? 'Testnachricht gesendet – umgeleitet an die eingerichtete Entwicklungsadresse, nicht an $an'
            : 'Testnachricht an $an gesendet';
      });
    } on ApiException catch (e) {
      if (mounted) {
        setState(() {
          testFailed = true;
          testResult = 'Testversand fehlgeschlagen: ${e.message}';
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          testFailed = true;
          testResult = 'Testversand fehlgeschlagen: $e';
        });
      }
    } finally {
      if (mounted) setState(() => testing = false);
    }
  }

  Widget _warning(String text) => Card(
        color: const Color(0xFFFFF3E0),
        child: ListTile(leading: const Icon(Icons.warning_amber), title: Text(text)),
      );

  @override
  Widget build(BuildContext context) {
    final s = stand;
    final gesetzt = s?['passwort_gesetzt'] == true;
    final lesbar = s?['passwort_lesbar'] == true;
    final schluessel = s?['schluessel_eingerichtet'] == true;
    return Scaffold(
      appBar: AppBar(title: const Text('E-Mail-Versand')),
      body: loading && s == null
          ? const Center(child: CircularProgressIndicator())
          : error != null
              ? Center(child: Text(error!))
              : ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    if (s != null && !schluessel)
                      _warning('Auf dem Server ist kein Schlüssel für verschlüsselte Einstellungen eingerichtet '
                          '(SETTINGS_ENCRYPTION_KEY) – ein Passwort lässt sich erst danach speichern.'),
                    if (gesetzt && !lesbar)
                      _warning('Das gespeicherte Passwort lässt sich nicht mehr entschlüsseln '
                          '(Schlüssel geändert) – bitte neu eintragen.'),
                    SwitchListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('E-Mail-Versand eingeschaltet'),
                      subtitle: const Text('Erst mit vollständigem Zugang möglich'),
                      value: aktiv,
                      onChanged: (v) => setState(() => aktiv = v),
                    ),
                    TextField(controller: hostCtrl, decoration: const InputDecoration(labelText: 'Mail-Server (z. B. mail.example.de)')),
                    Row(
                      children: [
                        Expanded(
                          flex: 2,
                          child: DropdownButtonFormField<String>(
                            key: const ValueKey('sicherheit'),
                            initialValue: sicherheit,
                            decoration: const InputDecoration(labelText: 'Verschlüsselung'),
                            items: [
                              for (final e in mailSicherheiten.entries) DropdownMenuItem(value: e.key, child: Text(e.value)),
                            ],
                            onChanged: (v) {
                              if (v != null) _setSicherheit(v);
                            },
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: TextField(
                            controller: portCtrl,
                            keyboardType: TextInputType.number,
                            decoration: const InputDecoration(labelText: 'Port'),
                          ),
                        ),
                      ],
                    ),
                    TextField(controller: benutzerCtrl, decoration: const InputDecoration(labelText: 'Benutzer')),
                    TextField(
                      key: const ValueKey('passwort'),
                      controller: passwortCtrl,
                      obscureText: true,
                      enabled: !passwortLoeschen,
                      decoration: InputDecoration(
                        labelText: 'Passwort',
                        helperText: gesetzt ? 'Gespeichert – leer lassen, um es zu behalten' : 'Noch kein Passwort gespeichert',
                      ),
                    ),
                    if (gesetzt)
                      CheckboxListTile(
                        contentPadding: EdgeInsets.zero,
                        title: const Text('Gespeichertes Passwort löschen'),
                        value: passwortLoeschen,
                        onChanged: (v) => setState(() {
                          passwortLoeschen = v == true;
                          if (passwortLoeschen) passwortCtrl.clear();
                        }),
                      ),
                    TextField(controller: absenderCtrl, decoration: const InputDecoration(labelText: 'Absenderadresse')),
                    TextField(controller: absenderNameCtrl, decoration: const InputDecoration(labelText: 'Absendername (optional)')),
                    if (formError != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: Text(formError!, style: const TextStyle(color: Color(0xFFB00020))),
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
                    Text('Testversand', style: Theme.of(context).textTheme.titleMedium),
                    const Text('Verwendet die gespeicherten Einstellungen – vorher speichern.'),
                    TextField(
                      key: const ValueKey('testAn'),
                      controller: testAnCtrl,
                      decoration: const InputDecoration(labelText: 'Empfänger der Testnachricht'),
                    ),
                    const SizedBox(height: 8),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: OutlinedButton(
                        onPressed: testing ? null : _test,
                        child: Text(testing ? 'Sendet …' : 'Testnachricht senden'),
                      ),
                    ),
                    if (testResult != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: Text(testResult!, style: TextStyle(color: testFailed ? const Color(0xFFB00020) : const Color(0xFF2E7D32))),
                      ),
                    const Divider(height: 32),
                    NoticeMailLogSection(api: widget.api),
                  ],
                ),
    );
  }
}
