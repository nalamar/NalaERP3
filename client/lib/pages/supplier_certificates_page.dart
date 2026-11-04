import 'package:flutter/material.dart';
import '../api.dart';

// Zertifikate am Lieferanten (ADR 0035 §1, Backlog H.4.4c1): Liste mit
// Hinweis "läuft bald ab", lesen mit purchase_orders.read.

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
