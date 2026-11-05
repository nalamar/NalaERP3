import 'package:flutter/material.dart';
import '../api.dart';
import '../web/browser.dart' as browser;
import 'employee_qualifications_section.dart'
    show
        QualificationFilePicker,
        qualificationDocumentAccept,
        qualificationDocumentLabel,
        qualificationUploadProblem;

// Lieferpapiere an der Bestellung (ADR 0035 §4, Backlog H.4.5b1/b2):
// Leistungserklaerung, Werkstoffzeugnisse nach EN 10204, Lieferschein.
// Lesen und herunterladen mit purchase_orders.read; hochladen (nicht an
// stornierter Bestellung) und stornieren mit purchase_orders.write.

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
      this.orderStatus = '',
      this.items = const [],
      this.filePicker});
  final ApiClient api;
  final String orderId;
  final String orderNumber;
  // An einer stornierten Bestellung (canceled) kommt nichts hinzu.
  final String orderStatus;
  // Positionen der Bestellung, fuer Bezug und Auswahl beim Hochladen.
  final List<dynamic> items;
  // Fuer Tests; sonst die Dateiauswahl des Browsers.
  final QualificationFilePicker? filePicker;

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

  bool get _canWrite => widget.api.hasPermission('purchase_orders.write');
  bool get _canUpload => _canWrite && widget.orderStatus != 'canceled';

  Future<void> _upload() async {
    final ok = await showDialog<bool>(
        context: context,
        builder: (_) => _UploadPaperDialog(
            api: widget.api,
            orderId: widget.orderId,
            items: widget.items,
            filePicker: widget.filePicker ?? browser.pickFile));
    if (ok == true) await _reload();
  }

  Future<void> _storno(Map<String, dynamic> p) async {
    final ok = await showDialog<bool>(
        context: context,
        builder: (_) => _StornoPaperDialog(
            api: widget.api, orderId: widget.orderId, paper: p));
    if (ok == true) await _reload();
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
      trailing: Row(mainAxisSize: MainAxisSize.min, children: [
        IconButton(
            tooltip: 'Herunterladen',
            onPressed: () => _download(p),
            icon: const Icon(Icons.download)),
        if (_canWrite && !storniert)
          IconButton(
              tooltip: 'Stornieren',
              onPressed: () => _storno(p),
              icon: const Icon(Icons.cancel_outlined)),
      ]),
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
      floatingActionButton: _canUpload
          ? FloatingActionButton(
              tooltip: 'Lieferpapier hochladen',
              onPressed: _upload,
              child: const Icon(Icons.upload_file))
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

class _UploadPaperDialog extends StatefulWidget {
  const _UploadPaperDialog(
      {required this.api,
      required this.orderId,
      required this.items,
      required this.filePicker});
  final ApiClient api;
  final String orderId;
  final List<dynamic> items;
  final QualificationFilePicker filePicker;

  @override
  State<_UploadPaperDialog> createState() => _UploadPaperDialogState();
}

class _UploadPaperDialogState extends State<_UploadPaperDialog> {
  String? art;
  String? positionId;
  final charge = TextEditingController();
  final notiz = TextEditingController();
  browser.PickedFile? file;
  bool busy = false;
  String? error;

  Future<void> _pick() async {
    final picked = await widget.filePicker(accept: qualificationDocumentAccept);
    if (picked == null || !mounted) return;
    final problem = qualificationUploadProblem(picked.bytes.length);
    setState(() {
      file = problem == null ? picked : null;
      error = problem;
    });
  }

  Future<void> _save() async {
    final f = file;
    final a = art;
    if (f == null || a == null) return;
    setState(() {
      busy = true;
      error = null;
    });
    try {
      await widget.api.uploadPurchaseOrderPaper(
          widget.orderId, f.filename, f.bytes,
          art: a,
          positionId: positionId,
          charge: charge.text,
          notiz: notiz.text,
          contentType: f.contentType);
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        busy = false;
        error = e is ApiException ? e.message : 'Upload fehlgeschlagen: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final f = file;
    return AlertDialog(
      title: const Text('Lieferpapier hochladen'),
      content: SizedBox(
        width: 480,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<String>(
                initialValue: art,
                items: [
                  for (final a in purchaseOrderPaperArts)
                    DropdownMenuItem(
                        value: a, child: Text(purchaseOrderPaperArtLabel(a)))
                ],
                onChanged: (v) => setState(() => art = v),
                decoration: const InputDecoration(labelText: 'Art'),
              ),
              DropdownButtonFormField<String?>(
                initialValue: positionId,
                items: [
                  const DropdownMenuItem<String?>(
                      value: null, child: Text('ganze Bestellung')),
                  for (final it in widget.items)
                    DropdownMenuItem<String?>(
                        value: (it as Map)['id'].toString(),
                        child: Text(
                            purchaseOrderPaperScope(
                                {'position_id': it['id']}, widget.items),
                            overflow: TextOverflow.ellipsis)),
                ],
                onChanged: (v) => setState(() => positionId = v),
                decoration: const InputDecoration(labelText: 'Bezug'),
              ),
              TextField(
                  controller: charge,
                  decoration: const InputDecoration(labelText: 'Charge')),
              TextField(
                  controller: notiz,
                  decoration: const InputDecoration(labelText: 'Notiz')),
              const SizedBox(height: 12),
              Row(children: [
                OutlinedButton.icon(
                    onPressed: busy ? null : _pick,
                    icon: const Icon(Icons.attach_file),
                    label: const Text('Datei wählen')),
                const SizedBox(width: 8),
                Expanded(
                    child: Text(
                        f == null
                            ? 'Keine Datei gewählt'
                            : qualificationDocumentLabel({
                                'filename': f.filename,
                                'length': f.bytes.length
                              }),
                        overflow: TextOverflow.ellipsis)),
              ]),
              const SizedBox(height: 4),
              const Text(
                  'PDF, JPEG oder PNG, höchstens 10 MB. Lieferpapiere werden nicht gelöscht.'),
              if (error != null)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Text(error!,
                      style: TextStyle(
                          color: Theme.of(context).colorScheme.error)),
                ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
            onPressed: busy ? null : () => Navigator.of(context).pop(false),
            child: const Text('Abbrechen')),
        FilledButton(
            onPressed: busy || art == null || f == null ? null : _save,
            child: const Text('Hochladen')),
      ],
    );
  }
}

class _StornoPaperDialog extends StatefulWidget {
  const _StornoPaperDialog(
      {required this.api, required this.orderId, required this.paper});
  final ApiClient api;
  final String orderId;
  final Map<String, dynamic> paper;

  @override
  State<_StornoPaperDialog> createState() => _StornoPaperDialogState();
}

class _StornoPaperDialogState extends State<_StornoPaperDialog> {
  final reason = TextEditingController();
  bool busy = false;
  String? error;

  Future<void> _save() async {
    setState(() {
      busy = true;
      error = null;
    });
    try {
      await widget.api.stornoPurchaseOrderPaper(
          widget.orderId, widget.paper['id'].toString(), reason.text.trim());
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
    return AlertDialog(
      title: const Text('Lieferpapier stornieren'),
      content: SizedBox(
        width: 420,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text([
              purchaseOrderPaperArtLabel(
                  (widget.paper['art'] ?? '').toString()),
              qualificationDocumentLabel(widget.paper),
            ].join('  •  ')),
            const SizedBox(height: 8),
            const Text(
                'Ein Lieferpapier wird nicht gelöscht, sondern mit Begründung storniert.'),
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
