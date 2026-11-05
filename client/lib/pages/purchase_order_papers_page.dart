import 'package:flutter/material.dart';
import '../api.dart';
import 'employee_qualifications_section.dart' show qualificationDocumentLabel;

// Lieferpapiere an der Bestellung (ADR 0035 §4, Backlog H.4.5b1):
// Leistungserklaerung, Werkstoffzeugnisse nach EN 10204, Lieferschein.
// Lesen und herunterladen mit purchase_orders.read.

const purchaseOrderPaperArts = [
  'leistungserklaerung',
  'zeugnis_2_2',
  'zeugnis_3_1',
  'zeugnis_3_2',
  'lieferschein',
  'sonstiges',
];

String purchaseOrderPaperArtLabel(String art) {
  switch (art) {
    case 'leistungserklaerung':
      return 'Leistungserklärung';
    case 'zeugnis_2_2':
      return 'Werkszeugnis 2.2';
    case 'zeugnis_3_1':
      return 'Abnahmeprüfzeugnis 3.1';
    case 'zeugnis_3_2':
      return 'Abnahmeprüfzeugnis 3.2';
    case 'lieferschein':
      return 'Lieferschein';
    case 'sonstiges':
      return 'Sonstiges';
    default:
      return art;
  }
}

// Bezug des Papiers: Position der Bestellung oder, ohne Position, die
// ganze Bestellung (ADR 0035 Praezisierung H.4.3a).
String purchaseOrderPaperScope(
    Map<String, dynamic> paper, List<dynamic> items) {
  final itemId = (paper['position_id'] ?? '').toString();
  if (itemId.isEmpty) return 'ganze Bestellung';
  for (final it in items) {
    final m = (it as Map).cast<String, dynamic>();
    if ((m['id'] ?? '').toString() == itemId) {
      final label = (m['bezeichnung'] ?? '').toString().trim();
      return 'Position ${m['position'] ?? ''}${label.isEmpty ? '' : ' – $label'}';
    }
  }
  return 'Position';
}

class PurchaseOrderPapersPage extends StatefulWidget {
  const PurchaseOrderPapersPage(
      {super.key,
      required this.api,
      required this.orderId,
      required this.orderNumber,
      this.items = const []});
  final ApiClient api;
  final String orderId;
  final String orderNumber;
  // Positionen der Bestellung, fuer die Anzeige des Bezugs.
  final List<dynamic> items;

  @override
  State<PurchaseOrderPapersPage> createState() =>
      _PurchaseOrderPapersPageState();
}

class _PurchaseOrderPapersPageState extends State<PurchaseOrderPapersPage> {
  List<dynamic> papers = [];
  bool loading = false;
  bool includeStorniert = false;
  String? error;

  @override
  void initState() {
    super.initState();
    _reload();
  }

  String _message(Object e, String fallback) =>
      e is ApiException ? e.message : '$fallback: $e';

  Future<void> _reload() async {
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final list = await widget.api.listPurchaseOrderPapers(widget.orderId,
          includeStorniert: includeStorniert);
      if (!mounted) return;
      setState(() => papers = list);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        papers = [];
        error = _message(e, 'Lieferpapiere konnten nicht geladen werden');
      });
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> _download(Map<String, dynamic> p) async {
    try {
      await widget.api.downloadPurchaseOrderPaper(
          widget.orderId, p['id'].toString(),
          filename: p['filename']?.toString());
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(_message(e, 'Download fehlgeschlagen'))));
      }
    }
  }

  Widget _tile(Map<String, dynamic> p) {
    final charge = (p['charge'] ?? '').toString().trim();
    final notiz = (p['notiz'] ?? '').toString().trim();
    final storniert = p['storniert_am'] != null;
    final details = [
      purchaseOrderPaperScope(p, widget.items),
      if (charge.isNotEmpty) 'Charge: $charge',
      qualificationDocumentLabel(p),
    ];
    return ListTile(
      leading: Icon(storniert ? Icons.block : Icons.description_outlined),
      title: Text(purchaseOrderPaperArtLabel((p['art'] ?? '').toString())),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(details.join('  •  ')),
          if (notiz.isNotEmpty) Text(notiz),
          if (storniert)
            Text('Storniert: ${(p['storno_grund'] ?? '').toString()}'),
        ],
      ),
      trailing: IconButton(
          tooltip: 'Herunterladen',
          onPressed: () => _download(p),
          icon: const Icon(Icons.download)),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Lieferpapiere – Bestellung ${widget.orderNumber}'),
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
                : papers.isEmpty && !loading
                    ? const Center(child: Text('Keine Lieferpapiere'))
                    : ListView(
                        children: [
                          for (final p in papers)
                            _tile((p as Map).cast<String, dynamic>()),
                        ],
                      ),
          ),
        ],
      ),
    );
  }
}
