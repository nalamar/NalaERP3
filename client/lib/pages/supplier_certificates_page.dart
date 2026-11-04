import 'package:flutter/material.dart';
import '../api.dart';
import 'employee_qualifications_section.dart' show parseDateInput;

// Zertifikate am Lieferanten (ADR 0035 §1, Backlog H.4.4c1/c2): Liste mit
// Hinweis "läuft bald ab", lesen mit purchase_orders.read; anlegen und
// stornieren mit purchase_orders.write.

const supplierCertificateArts = ['en1090', 'iso3834', 'sonstiges'];
const excKlassen = ['EXC1', 'EXC2', 'EXC3', 'EXC4'];

String supplierCertificateArtLabel(String art) {
  switch (art) {
    case 'en1090':
      return 'EN 1090';
    case 'iso3834':
      return 'ISO 3834';
    case 'sonstiges':
      return 'Sonstiges';
    default:
      return art;
  }
}

String _hintLabel(String hint) {
  switch (hint) {
    case 'zertifikat_laeuft_ab':
      return 'Läuft bald ab';
    default:
      return hint;
  }
}

// Datum ohne Umrechnung in die lokale Zeitzone: der Server liefert reine
// Tage als Mitternacht UTC.
String _date(dynamic value) {
  final parsed = DateTime.tryParse((value ?? '').toString());
  if (parsed == null) return '';
  return '${parsed.day.toString().padLeft(2, '0')}.'
      '${parsed.month.toString().padLeft(2, '0')}.${parsed.year}';
}

// buildSupplierCertificateBody baut den Body fuer POST
// /supplier-certificates. Daten kommen als TT.MM.JJJJ oder JJJJ-MM-TT und
// gehen als JJJJ-MM-TT; leere fallen weg. Die Ausfuehrungsklasse gibt es
// nur bei EN 1090.
Map<String, dynamic> buildSupplierCertificateBody({
  required String supplierId,
  required String art,
  String nummer = '',
  String stelle = '',
  String? excBis,
  String von = '',
  String bis = '',
  String notiz = '',
}) =>
    {
      'lieferant_id': supplierId,
      'art': art,
      'nummer': nummer.trim(),
      'stelle': stelle.trim(),
      if (art == 'en1090' && excBis != null) 'exc_bis': excBis,
      if (von.trim().isNotEmpty) 'gueltig_von': parseDateInput(von),
      if (bis.trim().isNotEmpty) 'gueltig_bis': parseDateInput(bis),
      'notiz': notiz.trim(),
    };

String? _dateValidator(String? v) {
  if (v == null || v.trim().isEmpty) return null;
  return parseDateInput(v) == null ? 'Datum als TT.MM.JJJJ' : null;
}

class SupplierCertificatesPage extends StatefulWidget {
  const SupplierCertificatesPage(
      {super.key,
      required this.api,
      required this.supplierId,
      required this.supplierName});
  final ApiClient api;
  final String supplierId;
  final String supplierName;

  @override
  State<SupplierCertificatesPage> createState() =>
      _SupplierCertificatesPageState();
}

class _SupplierCertificatesPageState extends State<SupplierCertificatesPage> {
  List<dynamic> items = [];
  bool loading = false;
  bool includeStorniert = false;
  String? error;

  @override
  void initState() {
    super.initState();
    _reload();
  }

  Future<void> _reload() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final list = await widget.api.listSupplierCertificates(widget.supplierId,
          includeStorniert: includeStorniert);
      if (!mounted) return;
      setState(() => items = list);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        items = [];
        error = e is ApiException
            ? e.message
            : 'Zertifikate konnten nicht geladen werden: $e';
      });
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  bool get _canWrite => widget.api.hasPermission('purchase_orders.write');

  Future<void> _create() async {
    final ok = await showDialog<bool>(
        context: context,
        builder: (_) => _CreateCertificateDialog(
            api: widget.api, supplierId: widget.supplierId));
    if (ok == true) await _reload();
  }

  Future<void> _storno(Map<String, dynamic> c) async {
    final ok = await showDialog<bool>(
        context: context,
        builder: (_) => _StornoCertificateDialog(api: widget.api, cert: c));
    if (ok == true) await _reload();
  }

  Widget _tile(Map<String, dynamic> c) {
    final nummer = (c['nummer'] ?? '').toString().trim();
    final stelle = (c['stelle'] ?? '').toString().trim();
    final exc = (c['exc_bis'] ?? '').toString().trim();
    final von = _date(c['gueltig_von']);
    final bis = _date(c['gueltig_bis']);
    final storniert = c['storniert_am'] != null;
    final hints = ((c['hinweise'] as List?) ?? const [])
        .map((h) => _hintLabel(h.toString()))
        .toList();
    final details = [
      if (stelle.isNotEmpty) 'Stelle: $stelle',
      if (exc.isNotEmpty) 'Ausführungsklasse bis $exc',
      if (von.isNotEmpty || bis.isNotEmpty)
        'gültig ${von.isEmpty ? '' : 'ab $von'}${von.isNotEmpty && bis.isNotEmpty ? ' ' : ''}${bis.isEmpty ? '' : 'bis $bis'}',
      if (von.isEmpty && bis.isEmpty) 'unbefristet',
    ];
    final errorColor = Theme.of(context).colorScheme.error;
    return ListTile(
      leading: Icon(storniert ? Icons.block : Icons.verified_outlined),
      title: Text([
        supplierCertificateArtLabel((c['art'] ?? '').toString()),
        if (nummer.isNotEmpty) nummer,
      ].join('  •  ')),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(details.join('  •  ')),
          if (storniert)
            Text('Storniert: ${(c['storno_grund'] ?? '').toString()}'),
          for (final h in hints) Text(h, style: TextStyle(color: errorColor)),
        ],
      ),
      trailing: _canWrite && !storniert
          ? IconButton(
              tooltip: 'Stornieren',
              onPressed: () => _storno(c),
              icon: const Icon(Icons.cancel_outlined))
          : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Zertifikate – ${widget.supplierName}'),
        actions: [
          IconButton(onPressed: _reload, icon: const Icon(Icons.refresh)),
        ],
      ),
      floatingActionButton: _canWrite
          ? FloatingActionButton(
              tooltip: 'Zertifikat anlegen',
              onPressed: _create,
              child: const Icon(Icons.add))
          : null,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SwitchListTile(
            title: const Text('Stornierte zeigen'),
            value: includeStorniert,
            onChanged: (v) {
              setState(() => includeStorniert = v);
              _reload();
            },
          ),
          if (loading) const LinearProgressIndicator(minHeight: 2),
          Expanded(
            child: error != null
                ? Center(child: Text(error!))
                : items.isEmpty && !loading
                    ? const Center(child: Text('Keine Zertifikate'))
                    : ListView(
                        children: [
                          for (final c in items)
                            _tile((c as Map).cast<String, dynamic>()),
                        ],
                      ),
          ),
        ],
      ),
    );
  }
}

class _CreateCertificateDialog extends StatefulWidget {
  const _CreateCertificateDialog({required this.api, required this.supplierId});
  final ApiClient api;
  final String supplierId;

  @override
  State<_CreateCertificateDialog> createState() =>
      _CreateCertificateDialogState();
}

class _CreateCertificateDialogState extends State<_CreateCertificateDialog> {
  final _formKey = GlobalKey<FormState>();
  final nummer = TextEditingController();
  final stelle = TextEditingController();
  final von = TextEditingController();
  final bis = TextEditingController();
  final notiz = TextEditingController();
  String art = 'en1090';
  String? exc;
  bool busy = false;
  String? error;

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      busy = true;
      error = null;
    });
    try {
      await widget.api.createSupplierCertificate(buildSupplierCertificateBody(
          supplierId: widget.supplierId,
          art: art,
          nummer: nummer.text,
          stelle: stelle.text,
          excBis: exc,
          von: von.text,
          bis: bis.text,
          notiz: notiz.text));
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        busy = false;
        error = e is ApiException ? e.message : 'Anlegen fehlgeschlagen: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Zertifikat anlegen'),
      content: SizedBox(
        width: 460,
        child: Form(
          key: _formKey,
          child: SingleChildScrollView(
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              DropdownButtonFormField<String>(
                decoration: const InputDecoration(labelText: 'Art'),
                initialValue: art,
                items: [
                  for (final a in supplierCertificateArts)
                    DropdownMenuItem(
                        value: a, child: Text(supplierCertificateArtLabel(a))),
                ],
                onChanged: (v) => setState(() => art = v ?? art),
              ),
              if (art == 'en1090')
                DropdownButtonFormField<String>(
                  decoration:
                      const InputDecoration(labelText: 'Ausführungsklasse bis'),
                  initialValue: exc,
                  items: [
                    for (final k in excKlassen)
                      DropdownMenuItem(value: k, child: Text(k)),
                  ],
                  onChanged: (v) => setState(() => exc = v),
                  validator: (v) =>
                      v == null ? 'Pflichtfeld bei EN 1090' : null,
                ),
              TextFormField(
                  controller: nummer,
                  decoration: const InputDecoration(labelText: 'Nummer')),
              TextFormField(
                  controller: stelle,
                  decoration: const InputDecoration(
                      labelText: 'Stelle (Zertifizierer)')),
              TextFormField(
                  controller: von,
                  decoration: const InputDecoration(
                      labelText: 'Gültig von (TT.MM.JJJJ)'),
                  validator: _dateValidator),
              TextFormField(
                  controller: bis,
                  decoration: const InputDecoration(
                      labelText: 'Gültig bis (TT.MM.JJJJ)'),
                  validator: _dateValidator),
              TextFormField(
                  controller: notiz,
                  decoration: const InputDecoration(labelText: 'Notiz')),
              if (error != null)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Text(error!,
                      style: TextStyle(
                          color: Theme.of(context).colorScheme.error)),
                ),
            ]),
          ),
        ),
      ),
      actions: [
        TextButton(
            onPressed: busy ? null : () => Navigator.of(context).pop(false),
            child: const Text('Abbrechen')),
        FilledButton(
            onPressed: busy ? null : _save, child: const Text('Anlegen')),
      ],
    );
  }
}

class _StornoCertificateDialog extends StatefulWidget {
  const _StornoCertificateDialog({required this.api, required this.cert});
  final ApiClient api;
  final Map<String, dynamic> cert;

  @override
  State<_StornoCertificateDialog> createState() =>
      _StornoCertificateDialogState();
}

class _StornoCertificateDialogState extends State<_StornoCertificateDialog> {
  final reason = TextEditingController();
  bool busy = false;
  String? error;

  Future<void> _save() async {
    setState(() {
      busy = true;
      error = null;
    });
    try {
      await widget.api.stornoSupplierCertificate(
          widget.cert['id'].toString(), reason.text.trim());
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        busy = false;
        error = e is ApiException ? e.message : 'Stornieren fehlgeschlagen: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final nummer = (widget.cert['nummer'] ?? '').toString().trim();
    return AlertDialog(
      title: const Text('Zertifikat stornieren'),
      content: SizedBox(
        width: 420,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text([
              supplierCertificateArtLabel(
                  (widget.cert['art'] ?? '').toString()),
              if (nummer.isNotEmpty) nummer,
            ].join('  •  ')),
            const SizedBox(height: 8),
            const Text(
                'Ein Zertifikat wird nicht gelöscht, sondern mit Begründung storniert.'),
            TextField(
              controller: reason,
              decoration: const InputDecoration(labelText: 'Begründung'),
              onChanged: (_) => setState(() {}),
            ),
            if (error != null)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(error!,
                    style:
                        TextStyle(color: Theme.of(context).colorScheme.error)),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
            onPressed: busy ? null : () => Navigator.of(context).pop(false),
            child: const Text('Abbrechen')),
        FilledButton(
            onPressed: busy || reason.text.trim().isEmpty ? null : _save,
            child: const Text('Stornieren')),
      ],
    );
  }
}
