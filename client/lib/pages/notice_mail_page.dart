import 'package:flutter/material.dart';
import '../api.dart';
import 'assets_page.dart' show isoToGerman;

// Hinweis-Mails (Backlog F.14.4c2, Server F.14.4c1):
//   - NoticeMailPrefsPage: jeder Benutzer bestellt die Sammelmail für sich
//     ab oder wieder an.
//   - NoticeMailLogSection: Versandprotokoll auf der Seite "E-Mail-Versand"
//     (settings.manage) - wann, an wen, Ergebnis; ohne Inhalt.

const noticeMailStufen = {
  'verbindung': 'Mail-Server nicht erreichbar',
  'tls': 'verschlüsselte Verbindung fehlgeschlagen',
  'anmeldung': 'Anmeldung fehlgeschlagen',
  'absender': 'Absender abgelehnt',
  'empfaenger': 'Empfänger abgelehnt',
  'nachricht': 'Nachricht abgelehnt',
  'eingabe': 'Adresse ungültig',
};

// noticeMailLogLine: "29.10.2026 · Anna Berg · Fehler: Anmeldung fehlgeschlagen".
String noticeMailLogLine(Map<String, dynamic> e) {
  final benutzer = (e['benutzer']?.toString() ?? '').isEmpty
      ? '(Benutzer entfernt)'
      : e['benutzer'].toString();
  final String ergebnis;
  switch (e['ergebnis']) {
    case 'gesendet':
      ergebnis = 'gesendet';
    case 'fehler':
      final stufe = e['fehler_stufe']?.toString() ?? '';
      ergebnis = 'Fehler: ${noticeMailStufen[stufe] ?? stufe}';
    default:
      ergebnis = 'Versand begonnen, nicht abgeschlossen';
  }
  return '${isoToGerman(e['versand_tag']?.toString())} · $benutzer · $ergebnis';
}

class NoticeMailPrefsPage extends StatefulWidget {
  const NoticeMailPrefsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<NoticeMailPrefsPage> createState() => _NoticeMailPrefsPageState();
}

class _NoticeMailPrefsPageState extends State<NoticeMailPrefsPage> {
  bool? abbestellt;
  bool saving = false;
  String? error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final v = await widget.api.getNoticeMailOptOut();
      if (mounted) setState(() => abbestellt = v);
    } on ApiException catch (e) {
      if (mounted)
        setState(() =>
            error = 'Einstellung konnte nicht geladen werden: ${e.message}');
    } catch (e) {
      if (mounted)
        setState(() => error = 'Einstellung konnte nicht geladen werden: $e');
    }
  }

  Future<void> _set(bool erhalten) async {
    setState(() {
      saving = true;
      error = null;
    });
    try {
      final v = await widget.api.setNoticeMailOptOut(!erhalten);
      if (!mounted) return;
      setState(() => abbestellt = v);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(v
              ? 'Hinweis-Mails abbestellt'
              : 'Hinweis-Mails wieder bestellt')));
    } on ApiException catch (e) {
      if (mounted)
        setState(() => error = 'Speichern fehlgeschlagen: ${e.message}');
    } catch (e) {
      if (mounted) setState(() => error = 'Speichern fehlgeschlagen: $e');
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final a = abbestellt;
    return Scaffold(
      appBar: AppBar(title: const Text('Hinweis-Mails')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          const Text(
              'Werktags um 7 Uhr kommt eine Mail mit Ihren Hinweisen – nur, wenn etwas fällig ist. '
              'Sie enthält nur Arten und Anzahlen wie das Dashboard, keine Namen.'),
          const SizedBox(height: 12),
          if (a == null && error == null)
            const LinearProgressIndicator(minHeight: 2),
          if (a != null)
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              title: const Text('Hinweis-Mails erhalten'),
              value: !a,
              onChanged: saving ? null : _set,
            ),
          if (error != null)
            Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        ],
      ),
    );
  }
}

class NoticeMailLogSection extends StatefulWidget {
  const NoticeMailLogSection({super.key, required this.api});
  final ApiClient api;

  @override
  State<NoticeMailLogSection> createState() => _NoticeMailLogSectionState();
}

class _NoticeMailLogSectionState extends State<NoticeMailLogSection> {
  List<dynamic>? entries;
  String? error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final list = await widget.api.listNoticeMailLog(limit: 50);
      if (mounted) setState(() => entries = list);
    } catch (e) {
      if (mounted)
        setState(() => error = 'Protokoll konnte nicht geladen werden: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final list = entries;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Versandprotokoll der Hinweis-Mails',
            style: Theme.of(context).textTheme.titleMedium),
        const Text('Die letzten 50 Einträge – ohne Inhalt und ohne Adressen.'),
        const SizedBox(height: 8),
        if (error != null)
          Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
        if (list == null && error == null)
          const LinearProgressIndicator(minHeight: 2),
        if (list != null && list.isEmpty)
          const Text('Noch keine Hinweis-Mail versandt.'),
        if (list != null)
          for (final e in list)
            Text(
              noticeMailLogLine(e as Map<String, dynamic>),
              style: e['ergebnis'] == 'fehler'
                  ? const TextStyle(color: Color(0xFFB00020))
                  : null,
            ),
      ],
    );
  }
}
