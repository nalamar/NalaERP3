import 'package:flutter/material.dart';
import '../api.dart';

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
                    TextField(controller: poPatternCtrl, decoration: const InputDecoration(labelText: 'Pattern', hintText: 'z. B. PO-{YYYY}-{NNNN}'), onChanged: (_){ _updatePreview(); }),
                    const SizedBox(height: 8),
                    Text('Vorschau: $preview'),
                    const SizedBox(height: 8),
                    const Text('Variablen: {YYYY}, {YY}, {MM}, {DD}, {NN}, {NNN}, {NNNN}'),
                    const SizedBox(height: 8),
                    Align(alignment: Alignment.centerRight, child: FilledButton.icon(onPressed: _save, icon: const Icon(Icons.save), label: const Text('Speichern'))),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
