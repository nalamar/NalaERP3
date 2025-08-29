import 'package:flutter/material.dart';
import '../api.dart';
import 'dart:typed_data';
import '../web/browser.dart' as browser;

class SettingsPage extends StatefulWidget {
  const SettingsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage> {
  final poPatternCtrl = TextEditingController(text: 'PO-{YYYY}-{NNNN}');
  String preview = '';
  bool loading = false;
  // PDF Template Controls (purchase_order)
  final poHeaderCtrl = TextEditingController();
  final poFooterCtrl = TextEditingController();
  final poTopFirstCtrl = TextEditingController(text: '30');
  final poTopOtherCtrl = TextEditingController(text: '20');
  String? poLogoDocId;
  String? poBgFirstDocId;
  String? poBgOtherDocId;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(()=> loading = true);
    try {
      final cfg = await widget.api.getNumberingConfig('purchase_order');
      poPatternCtrl.text = (cfg['pattern'] ?? 'PO-{YYYY}-{NNNN}').toString();
      await _updatePreview();
      await _loadPdfTemplate();
    } catch (e) { /* ignore */ }
    setState(()=> loading = false);
  }

  Future<void> _updatePreview() async {
    try {
      final p = await widget.api.previewNumbering('purchase_order');
      setState(()=> preview = p);
    } catch (e) { setState(()=> preview = ''); }
  }

  Future<void> _save() async {
    try {
      await widget.api.updateNumberingPattern('purchase_order', poPatternCtrl.text.trim());
      await _updatePreview();
      if (mounted) { ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Gespeichert'))); }
    } catch (e) {
      if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); }
    }
  }

  Future<void> _loadPdfTemplate() async {
    try {
      final t = await widget.api.getPdfTemplate('purchase_order');
      poHeaderCtrl.text = (t['header_text'] ?? '').toString();
      poFooterCtrl.text = (t['footer_text'] ?? '').toString();
      final tf = double.tryParse('${t['top_first_mm'] ?? '30'}') ?? 30;
      final to = double.tryParse('${t['top_other_mm'] ?? '20'}') ?? 20;
      poTopFirstCtrl.text = tf.toStringAsFixed(0);
      poTopOtherCtrl.text = to.toStringAsFixed(0);
      poLogoDocId = (t['logo_doc_id'] as String?);
      poBgFirstDocId = (t['bg_first_doc_id'] as String?);
      poBgOtherDocId = (t['bg_other_doc_id'] as String?);
      if (mounted) setState((){});
    } catch (_) {
      // ignore
    }
  }

  Future<void> _savePdfTemplate() async {
    try {
      final tf = double.tryParse(poTopFirstCtrl.text.trim().replaceAll(',', '.')) ?? 30;
      final to = double.tryParse(poTopOtherCtrl.text.trim().replaceAll(',', '.')) ?? 20;
      await widget.api.updatePdfTemplate('purchase_order', headerText: poHeaderCtrl.text, footerText: poFooterCtrl.text, topFirstMm: tf, topOtherMm: to);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('PDF-Template gespeichert')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _pickAndUpload(String kind) async {
    final picked = await browser.pickFile(accept: 'image/*,application/pdf');
    if (picked == null) return;
    try {
      final res = await widget.api.uploadPdfImage('purchase_order', kind, picked.filename, picked.bytes, contentType: picked.contentType);
      final id = (res['document_id'] ?? '').toString();
      setState((){
        if (kind == 'logo') poLogoDocId = id;
        if (kind == 'bg-first') poBgFirstDocId = id;
        if (kind == 'bg-other') poBgOtherDocId = id;
      });
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Upload erfolgreich')));
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  Future<void> _deleteImage(String kind) async {
    try {
      await widget.api.deletePdfImage('purchase_order', kind);
      setState((){
        if (kind == 'logo') poLogoDocId = null;
        if (kind == 'bg-first') poBgFirstDocId = null;
        if (kind == 'bg-other') poBgOtherDocId = null;
      });
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Bild entfernt')));
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    return Scaffold(
      appBar: AppBar(
        backgroundColor: color,
        foregroundColor: Colors.white,
        title: const Text('Einstellungen'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('Nummernkreise', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              const SizedBox(height: 12),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text('Bestellungen'),
                      const SizedBox(height: 8),
                      TextField(
                        controller: poPatternCtrl,
                        decoration: const InputDecoration(labelText: 'Pattern', hintText: 'z. B. PO-{YYYY}-{NNNN}'),
                        onChanged: (_) { _updatePreview(); },
                      ),
                      const SizedBox(height: 8),
                      Text('Vorschau: $preview'),
                      const SizedBox(height: 8),
                      const Text('Variablen: {YYYY}, {YY}, {MM}, {DD}, {NN}, {NNN}, {NNNN}'),
                      const SizedBox(height: 8),
                      Align(
                        alignment: Alignment.centerRight,
                        child: FilledButton.icon(onPressed: _save, icon: const Icon(Icons.save), label: const Text('Speichern')),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 24),
              const Text('PDF-Templates', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              const SizedBox(height: 12),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text('Bestellungen (purchase_order)'),
                      const SizedBox(height: 8),
                      TextField(
                        controller: poHeaderCtrl,
                        maxLines: 3,
                        decoration: const InputDecoration(labelText: 'Kopftext', hintText: 'z. B. Firmenname, Adresse, Kontaktdaten'),
                      ),
                      const SizedBox(height: 8),
                      TextField(
                        controller: poFooterCtrl,
                        maxLines: 2,
                        decoration: const InputDecoration(labelText: 'Fußtext', hintText: 'z. B. Bankdaten, USt-IdNr.'),
                      ),
                      const SizedBox(height: 12),
                      Row(children: [
                        Expanded(child: TextField(controller: poTopFirstCtrl, decoration: const InputDecoration(labelText: 'Start Höhe Seite 1 (mm)'), keyboardType: TextInputType.number)),
                        const SizedBox(width: 12),
                        Expanded(child: TextField(controller: poTopOtherCtrl, decoration: const InputDecoration(labelText: 'Start Höhe Folgeseiten (mm)'), keyboardType: TextInputType.number)),
                      ]),
                      const SizedBox(height: 12),
                      Wrap(spacing: 12, runSpacing: 8, children: [
                        _imageRow('Logo', poLogoDocId, onUpload: () => _pickAndUpload('logo')),
                        _imageRow('Hintergrund (Seite 1)', poBgFirstDocId, onUpload: () => _pickAndUpload('bg-first')),
                        _imageRow('Hintergrund (Folge)', poBgOtherDocId, onUpload: () => _pickAndUpload('bg-other')),
                      ]),
                      const SizedBox(height: 12),
                      Align(
                        alignment: Alignment.centerRight,
                        child: FilledButton.icon(onPressed: _savePdfTemplate, icon: const Icon(Icons.save), label: const Text('Speichern')),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _imageRow(String label, String? docId, {required VoidCallback onUpload}) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(label),
        const SizedBox(width: 8),
        if (docId != null) ...[
          const Icon(Icons.check_circle, color: Colors.green, size: 18),
          const SizedBox(width: 4),
          Text(docId.substring(0, docId.length >= 8 ? 8 : docId.length)),
          const SizedBox(width: 8),
          TextButton.icon(onPressed: (){ widget.api.downloadDocument(docId, filename: 'preview'); }, icon: const Icon(Icons.visibility), label: const Text('Anzeigen')),
          const SizedBox(width: 8),
          TextButton.icon(onPressed: (){ _deleteImage(_kindFromLabel(label)); }, icon: const Icon(Icons.delete), label: const Text('Entfernen')),
        ] else ...[
          const Text('— nicht gesetzt —'),
        ],
        const SizedBox(width: 8),
        OutlinedButton.icon(onPressed: onUpload, icon: const Icon(Icons.upload), label: const Text('Hochladen')),
      ],
    );
  }

  String _kindFromLabel(String label) {
    switch (label) {
      case 'Logo': return 'logo';
      case 'Hintergrund (Seite 1)': return 'bg-first';
      case 'Hintergrund (Folge)': return 'bg-other';
      default: return 'logo';
    }
  }
}
