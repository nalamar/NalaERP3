import 'package:flutter/material.dart';

import '../api.dart';
import '../commercial_destinations.dart';
import '../commercial_navigation.dart';

String _salesOrderErrorMessage(Object error,
    {String fallback = 'Vorgang fehlgeschlagen'}) {
  if (error is ApiException) {
    return error.message;
  }
  return '$fallback: $error';
}

class SalesOrdersPage extends StatefulWidget {
  const SalesOrdersPage({
    super.key,
    required this.api,
    this.initialSalesOrderId,
    this.initialSearchQuery,
    this.initialContext,
    this.initialProjectId,
    this.initialFilters,
  });

  final ApiClient api;
  @Deprecated('Use initialContext instead.')
  final String? initialSalesOrderId;
  @Deprecated('Use initialContext instead.')
  final String? initialSearchQuery;
  final CommercialListContext? initialContext;
  @Deprecated('Use initialFilters instead.')
  final String? initialProjectId;
  final CommercialFilterContext? initialFilters;

  @override
  State<SalesOrdersPage> createState() => _SalesOrdersPageState();
}

class _SalesOrdersPageState extends State<SalesOrdersPage> {
  bool _loading = true;
  bool _actionInProgress = false;
  List<dynamic> _items = const [];
  List<String> _availableStatuses = const [
    'open',
    'released',
    'invoiced',
    'completed',
    'canceled'
  ];
  Map<String, dynamic>? _selected;
  Map<String, dynamic>? _sourceQuote;
  Map<String, dynamic>? _linkedInvoice;
  List<dynamic> _relatedInvoices = const [];
  final _searchCtrl = TextEditingController();
  final _projectCtrl = TextEditingController();
  String? _statusFilter;
  bool _partialOnlyFilter = false;
  bool _initialSelectionHandled = false;
  bool _workflowHintDismissed = false;

  @override
  void initState() {
    super.initState();
    final initialFilters = _resolvedInitialFilters();
    if (initialFilters.normalizedProjectId != null) {
      _projectCtrl.text = initialFilters.normalizedProjectId!;
    }
    final initialContext = _resolvedInitialContext();
    final initialSearch = initialContext.effectiveSearchQuery;
    if (initialSearch != null) {
      _searchCtrl.text = initialSearch;
    }
    _loadStatuses();
    _load();
  }

  CommercialListContext _resolvedInitialContext() {
    return widget.initialContext ??
        (widget.initialSalesOrderId?.trim().isNotEmpty ?? false
            ? CommercialListContext.detail(widget.initialSalesOrderId!.trim())
            : CommercialListContext(searchQuery: widget.initialSearchQuery));
  }

  CommercialFilterContext _resolvedInitialFilters() {
    return widget.initialFilters ??
        CommercialFilterContext(projectId: widget.initialProjectId);
  }

  CommercialFilterContext _currentFilterContext({String? salesOrderId}) {
    return CommercialFilterContext(
      projectId: _projectCtrl.text.trim(),
      sourceSalesOrderId: salesOrderId,
    );
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    _projectCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final list = await widget.api.listSalesOrders(
        q: _searchCtrl.text.trim().isEmpty ? null : _searchCtrl.text.trim(),
        projectId:
            _projectCtrl.text.trim().isEmpty ? null : _projectCtrl.text.trim(),
        status: _statusFilter,
      );
      final filteredList = _partialOnlyFilter
          ? list.where((entry) {
              final item = (entry as Map).cast<String, dynamic>();
              return (_toDouble(item['related_invoice_count'])).round() > 0 &&
                  _toDouble(item['remaining_gross_amount']) > 0.0001;
            }).toList()
          : list;
      setState(() => _items = filteredList);
      final selectedId = _selected?['id']?.toString();
      if (selectedId != null && selectedId.isNotEmpty) {
        await _loadDetail(selectedId);
      } else if (!_initialSelectionHandled) {
        final initialId = _resolvedInitialContext().normalizedDetailId;
        if (initialId != null) {
          _initialSelectionHandled = true;
          await _loadDetail(initialId);
        }
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_salesOrderErrorMessage(e,
                fallback: 'Aufträge konnten nicht geladen werden'))),
      );
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _loadStatuses() async {
    try {
      final statuses = await widget.api.listSalesOrderStatuses();
      if (!mounted || statuses.isEmpty) return;
      setState(() => _availableStatuses = statuses);
    } catch (_) {}
  }

  Future<void> _loadDetail(String id) async {
    try {
      final detail = await widget.api.getSalesOrder(id);
      if (!mounted) return;
      setState(() {
        _selected = detail;
        _workflowHintDismissed = false;
      });
      await _loadSourceQuote(detail['source_quote_id']?.toString());
      await _loadLinkedInvoice(detail['linked_invoice_out_id']?.toString());
      await _loadRelatedInvoices(detail['id']?.toString());
    } catch (_) {}
  }

  Future<void> _loadSourceQuote(String? quoteId) async {
    final normalized = quoteId?.trim() ?? '';
    if (normalized.isEmpty) {
      if (mounted) setState(() => _sourceQuote = null);
      return;
    }
    try {
      final quote = await widget.api.getQuote(normalized);
      if (!mounted) return;
      setState(() => _sourceQuote = quote);
    } catch (_) {
      if (!mounted) return;
      setState(() => _sourceQuote = null);
    }
  }

  Future<void> _loadLinkedInvoice(String? invoiceId) async {
    final normalized = invoiceId?.trim() ?? '';
    if (normalized.isEmpty) {
      if (mounted) setState(() => _linkedInvoice = null);
      return;
    }
    try {
      final invoice = await widget.api.getInvoiceOut(normalized);
      if (!mounted) return;
      setState(() => _linkedInvoice = invoice);
    } catch (_) {
      if (!mounted) return;
      setState(() => _linkedInvoice = null);
    }
  }

  Future<void> _loadRelatedInvoices(String? salesOrderId) async {
    final normalized = salesOrderId?.trim() ?? '';
    if (normalized.isEmpty) {
      if (mounted) setState(() => _relatedInvoices = const []);
      return;
    }
    try {
      final invoices = await widget.api.listInvoicesOut(
        sourceSalesOrderId: normalized,
        limit: 20,
      );
      if (!mounted) return;
      setState(() => _relatedInvoices = invoices);
    } catch (_) {
      if (!mounted) return;
      setState(() => _relatedInvoices = const []);
    }
  }

  Future<void> _openInvoice(String invoiceId) async {
    final selectedSalesOrderId = (_selected?['id'] ?? '').toString().trim();
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => buildInvoicesPage(
          api: widget.api,
          initialFilters: _currentFilterContext(
            salesOrderId:
                selectedSalesOrderId.isEmpty ? null : selectedSalesOrderId,
          ),
          initialContext: CommercialListContext.detail(invoiceId),
          showWorkflowHint: true,
        ),
      ),
    );
    if (!mounted) return;
    final selectedId = _selected?['id']?.toString();
    if (selectedId != null && selectedId.isNotEmpty) {
      await _loadDetail(selectedId);
      await _load();
    }
  }

  Future<void> _openSalesOrderInvoices(String salesOrderId) async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => buildInvoicesPage(
          api: widget.api,
          initialFilters:
              CommercialFilterContext(sourceSalesOrderId: salesOrderId),
          initialContext: const CommercialListContext.search(''),
          showWorkflowHint: true,
        ),
      ),
    );
    if (!mounted) return;
    final selectedId = _selected?['id']?.toString();
    if (selectedId != null && selectedId.isNotEmpty) {
      await _loadDetail(selectedId);
      await _load();
    }
  }

  Future<void> _openQuote(String quoteId) async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => buildQuotesPage(
          api: widget.api,
          initialContext: CommercialListContext.detail(quoteId),
        ),
      ),
    );
    if (!mounted) return;
    final selectedId = _selected?['id']?.toString();
    if (selectedId != null && selectedId.isNotEmpty) {
      await _loadDetail(selectedId);
      await _load();
    }
  }

  Future<void> _downloadPdf() async {
    final selected = _selected;
    final salesOrderId = (selected?['id'] ?? '').toString();
    if (salesOrderId.isEmpty) return;
    try {
      final number = (selected?['number'] ?? '').toString().trim();
      await widget.api.downloadSalesOrderPdf(
        salesOrderId,
        filename: number.isEmpty ? null : 'Auftrag_$number.pdf',
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Auftrags-PDF wird heruntergeladen')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_salesOrderErrorMessage(e,
                fallback: 'PDF-Download fehlgeschlagen'))),
      );
    }
  }

  Future<void> _convertToInvoiceFromSalesOrder() async {
    final salesOrderId = (_selected?['id'] ?? '').toString();
    if (salesOrderId.isEmpty) return;
    final openItems = ((_selected?['items'] as List?) ?? const [])
        .cast<Map>()
        .map((item) => item.cast<String, dynamic>())
        .where((item) => _toDouble(item['remaining_qty']) > 0.0001)
        .toList();
    if (openItems.isEmpty) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
            content:
                Text('Keine offenen Restmengen mehr zur Faktura vorhanden')),
      );
      return;
    }
    final request = await showDialog<_SalesOrderConvertRequest>(
      context: context,
      builder: (_) => _SalesOrderConvertDialog(
        items: openItems
            .map(
              (item) => _SalesOrderConvertLine(
                salesOrderItemId: (item['id'] ?? '').toString(),
                description: (item['description'] ?? 'Position').toString(),
                unit: (item['unit'] ?? 'Stk').toString(),
                remainingQty: _toDouble(item['remaining_qty']),
                invoicedQty: _toDouble(item['invoiced_qty']),
                unitPrice: _toDouble(item['unit_price']),
                taxCode: (item['tax_code'] ?? '').toString(),
              ),
            )
            .toList(),
      ),
    );
    if (request == null) return;
    setState(() => _actionInProgress = true);
    try {
      final result = await widget.api.convertSalesOrderToInvoice(
        salesOrderId,
        revenueAccount: request.revenueAccount,
        invoiceDate: DateTime.now(),
        dueDate: request.dueDate,
        items: request.items.map((item) => item.toJson()).toList(),
      );
      final salesOrder =
          ((result['sales_order'] as Map?) ?? const {}).cast<String, dynamic>();
      final invoice =
          ((result['invoice'] as Map?) ?? const {}).cast<String, dynamic>();
      final invoiceId = (invoice['id'] ?? '').toString();
      if (!mounted) return;
      setState(() {
        _selected = salesOrder;
        _linkedInvoice = invoice;
      });
      await _loadSourceQuote(salesOrder['source_quote_id']?.toString());
      await _loadRelatedInvoices(salesOrder['id']?.toString());
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            invoiceId.isEmpty
                ? 'Rechnung wurde aus dem Auftrag erzeugt'
                : 'Rechnung $invoiceId wurde aus dem Auftrag erzeugt',
          ),
        ),
      );
      if (invoiceId.isNotEmpty &&
          widget.api.hasPermission('invoices_out.read')) {
        await _openInvoice(invoiceId);
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_salesOrderErrorMessage(e,
                fallback: 'Rechnungserzeugung fehlgeschlagen'))),
      );
    } finally {
      if (mounted) setState(() => _actionInProgress = false);
    }
  }

  Future<void> _updateStatus(String status) async {
    final salesOrderId = (_selected?['id'] ?? '').toString();
    if (salesOrderId.isEmpty) return;
    setState(() => _actionInProgress = true);
    try {
      final updated =
          await widget.api.updateSalesOrderStatus(salesOrderId, status);
      if (!mounted) return;
      setState(() => _selected = updated);
      await _loadSourceQuote(updated['source_quote_id']?.toString());
      await _loadLinkedInvoice(updated['linked_invoice_out_id']?.toString());
      await _loadRelatedInvoices(updated['id']?.toString());
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text('Auftrag wurde auf ${_statusLabel(status)} gesetzt')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_salesOrderErrorMessage(e,
                fallback: 'Statuswechsel fehlgeschlagen'))),
      );
    } finally {
      if (mounted) setState(() => _actionInProgress = false);
    }
  }

  Future<void> _editSalesOrderHeader() async {
    final selected = _selected;
    final salesOrderId = (selected?['id'] ?? '').toString();
    if (selected == null || salesOrderId.isEmpty) return;
    final request = await showDialog<_SalesOrderHeaderUpdateRequest>(
      context: context,
      builder: (_) => _SalesOrderHeaderDialog(
        initialNumber: (selected['number'] ?? '').toString(),
        initialCurrency: (selected['currency'] ?? 'EUR').toString(),
        initialNote: (selected['note'] ?? '').toString(),
        initialOrderDate:
            DateTime.tryParse((selected['order_date'] ?? '').toString()),
      ),
    );
    if (request == null) return;
    setState(() => _actionInProgress = true);
    try {
      final updated = await widget.api.updateSalesOrder(salesOrderId, {
        'number': request.number,
        'currency': request.currency,
        'note': request.note,
        'order_date': request.orderDate.toUtc().toIso8601String(),
      });
      if (!mounted) return;
      setState(() => _selected = updated);
      await _loadSourceQuote(updated['source_quote_id']?.toString());
      await _loadLinkedInvoice(updated['linked_invoice_out_id']?.toString());
      await _loadRelatedInvoices(updated['id']?.toString());
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Auftragskopf wurde gespeichert')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_salesOrderErrorMessage(e,
                fallback: 'Auftrag konnte nicht gespeichert werden'))),
      );
    } finally {
      if (mounted) setState(() => _actionInProgress = false);
    }
  }

  Future<void> _createSalesOrderItem() async {
    final salesOrderId = (_selected?['id'] ?? '').toString();
    if (salesOrderId.isEmpty) return;
    final request = await showDialog<_SalesOrderItemRequest>(
      context: context,
      builder: (_) => const _SalesOrderItemDialog(),
    );
    if (request == null) return;
    setState(() => _actionInProgress = true);
    try {
      final result =
          await widget.api.createSalesOrderItem(salesOrderId, request.toJson());
      final order =
          ((result['sales_order'] as Map?) ?? const {}).cast<String, dynamic>();
      if (!mounted) return;
      setState(() => _selected = order);
      await _loadLinkedInvoice(order['linked_invoice_out_id']?.toString());
      await _loadRelatedInvoices(order['id']?.toString());
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Position wurde hinzugefügt')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_salesOrderErrorMessage(e,
                fallback: 'Position konnte nicht angelegt werden'))),
      );
    } finally {
      if (mounted) setState(() => _actionInProgress = false);
    }
  }

  Future<void> _editSalesOrderItem(Map<String, dynamic> item) async {
    final salesOrderId = (_selected?['id'] ?? '').toString();
    final itemId = (item['id'] ?? '').toString();
    if (salesOrderId.isEmpty || itemId.isEmpty) return;
    final request = await showDialog<_SalesOrderItemRequest>(
      context: context,
      builder: (_) => _SalesOrderItemDialog(
        initialDescription: (item['description'] ?? '').toString(),
        initialQty: (item['qty'] ?? 1).toString(),
        initialUnit: (item['unit'] ?? 'Stk').toString(),
        initialUnitPrice: (item['unit_price'] ?? 0).toString(),
        initialTaxCode: (item['tax_code'] ?? '').toString(),
      ),
    );
    if (request == null) return;
    setState(() => _actionInProgress = true);
    try {
      final result = await widget.api
          .updateSalesOrderItem(salesOrderId, itemId, request.toJson());
      final order =
          ((result['sales_order'] as Map?) ?? const {}).cast<String, dynamic>();
      if (!mounted) return;
      setState(() => _selected = order);
      await _loadLinkedInvoice(order['linked_invoice_out_id']?.toString());
      await _loadRelatedInvoices(order['id']?.toString());
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Position wurde aktualisiert')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_salesOrderErrorMessage(e,
                fallback: 'Position konnte nicht gespeichert werden'))),
      );
    } finally {
      if (mounted) setState(() => _actionInProgress = false);
    }
  }

  Future<void> _deleteSalesOrderItem(Map<String, dynamic> item) async {
    final salesOrderId = (_selected?['id'] ?? '').toString();
    final itemId = (item['id'] ?? '').toString();
    if (salesOrderId.isEmpty || itemId.isEmpty) return;
    final ok = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: const Text('Position löschen'),
        content: Text(
            'Position "${(item['description'] ?? 'Position').toString()}" wirklich löschen?'),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(context).pop(false),
              child: const Text('Abbrechen')),
          FilledButton(
              onPressed: () => Navigator.of(context).pop(true),
              child: const Text('Löschen')),
        ],
      ),
    );
    if (ok != true) return;
    setState(() => _actionInProgress = true);
    try {
      final order = await widget.api.deleteSalesOrderItem(salesOrderId, itemId);
      if (!mounted) return;
      setState(() => _selected = order);
      await _loadLinkedInvoice(order['linked_invoice_out_id']?.toString());
      await _loadRelatedInvoices(order['id']?.toString());
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Position wurde gelöscht')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_salesOrderErrorMessage(e,
                fallback: 'Position konnte nicht gelöscht werden'))),
      );
    } finally {
      if (mounted) setState(() => _actionInProgress = false);
    }
  }

  String _statusLabel(String status) {
    switch (status) {
      case 'open':
        return 'Offen';
      case 'draft':
        return 'Entwurf';
      case 'sent':
        return 'Versendet';
      case 'accepted':
        return 'Angenommen';
      case 'rejected':
        return 'Abgelehnt';
      case 'booked':
        return 'Gebucht';
      case 'partial':
        return 'Teilbezahlt';
      case 'paid':
        return 'Bezahlt';
      case 'released':
        return 'Freigegeben';
      case 'invoiced':
        return 'Fakturiert';
      case 'completed':
        return 'Abgeschlossen';
      case 'canceled':
        return 'Storniert';
      default:
        return status.isEmpty ? 'Unbekannt' : status;
    }
  }

  Color _statusColor(BuildContext context, String status) {
    switch (status) {
      case 'accepted':
      case 'paid':
        return Colors.green;
      case 'sent':
      case 'booked':
      case 'released':
      case 'invoiced':
        return Colors.orange;
      case 'rejected':
      case 'canceled':
        return Colors.red;
      case 'completed':
        return Colors.green;
      default:
        return Theme.of(context).colorScheme.primary;
    }
  }

  String _formatDate(BuildContext context, String value) {
    final parsed = DateTime.tryParse(value);
    if (parsed == null) return value.isEmpty ? '-' : value;
    return MaterialLocalizations.of(context).formatMediumDate(parsed.toLocal());
  }

  String _formatMoney(num? value, String currency) {
    final normalizedCurrency = currency.isEmpty ? 'EUR' : currency;
    return '${(value ?? 0).toDouble().toStringAsFixed(2)} $normalizedCurrency';
  }

  double _lineNet(Map<String, dynamic> item) {
    final qty = ((item['qty'] ?? 0) as num).toDouble();
    final unitPrice = ((item['unit_price'] ?? 0) as num).toDouble();
    return qty * unitPrice;
  }

  double _toDouble(dynamic value) {
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '') ?? 0;
  }

  double _lineGross(Map<String, dynamic> item) {
    final net = _lineNet(item);
    final taxCode = (item['tax_code'] ?? '').toString().trim().toUpperCase();
    switch (taxCode) {
      case 'DE19':
        return net * 1.19;
      case 'DE7':
        return net * 1.07;
      default:
        return net;
    }
  }

  double _remainingGross(Map<String, dynamic> item) {
    final remainingQty = _toDouble(item['remaining_qty']);
    final unitPrice = _toDouble(item['unit_price']);
    final net = remainingQty * unitPrice;
    final taxCode = (item['tax_code'] ?? '').toString().trim().toUpperCase();
    switch (taxCode) {
      case 'DE19':
        return net * 1.19;
      case 'DE7':
        return net * 1.07;
      default:
        return net;
    }
  }

  String _listInvoiceProgress(Map<String, dynamic> item) {
    final invoiceCount = (_toDouble(item['related_invoice_count'])).round();
    final remainingGross = _toDouble(item['remaining_gross_amount']);
    if (invoiceCount <= 0) {
      return 'Noch nicht fakturiert';
    }
    if (remainingGross > 0.0001) {
      return 'Teilfakturiert';
    }
    return 'Vollständig fakturiert';
  }

  String _listInvoiceHint(Map<String, dynamic> item) {
    final invoiceCount = (_toDouble(item['related_invoice_count'])).round();
    final remainingGross = _toDouble(item['remaining_gross_amount']);
    final currency = (item['currency'] ?? 'EUR').toString();
    if (invoiceCount <= 0) {
      return 'Noch keine Folgebelege';
    }
    if (remainingGross > 0.0001) {
      return '$invoiceCount ${invoiceCount == 1 ? 'Rechnung' : 'Rechnungen'}  •  Rest ${_formatMoney(remainingGross, currency)}';
    }
    return '$invoiceCount ${invoiceCount == 1 ? 'Rechnung' : 'Rechnungen'}  •  Kein Restbetrag';
  }

  Widget _statusChip(BuildContext context, String status) {
    final color = _statusColor(context, status);
    return Chip(
      label: Text(_statusLabel(status)),
      backgroundColor: color.withValues(alpha: 0.12),
      labelStyle: TextStyle(color: color),
    );
  }

  Widget _buildFlowStep(
    BuildContext context, {
    required IconData icon,
    required String title,
    required String subtitle,
    required bool active,
    required bool done,
  }) {
    final scheme = Theme.of(context).colorScheme;
    final Color color = done
        ? Colors.green
        : active
            ? scheme.primary
            : scheme.outline;
    return SizedBox(
      width: 240,
      child: Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
              color: color.withValues(alpha: active || done ? 0.45 : 0.2)),
          color: color.withValues(alpha: active || done ? 0.08 : 0.03),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(done ? Icons.check_circle_rounded : icon, color: color),
            const SizedBox(height: 8),
            Text(title, style: const TextStyle(fontWeight: FontWeight.w600)),
            const SizedBox(height: 4),
            Text(subtitle, style: Theme.of(context).textTheme.bodySmall),
          ],
        ),
      ),
    );
  }

  Widget _buildMetricCard(
    BuildContext context, {
    required String label,
    required String value,
    String? hint,
  }) {
    return SizedBox(
      width: 220,
      child: Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          color: Theme.of(context)
              .colorScheme
              .surfaceContainerHighest
              .withValues(alpha: 0.45),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(label, style: Theme.of(context).textTheme.labelMedium),
            const SizedBox(height: 6),
            Text(value,
                style: Theme.of(context)
                    .textTheme
                    .titleMedium
                    ?.copyWith(fontWeight: FontWeight.w700)),
            if (hint != null && hint.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(hint, style: Theme.of(context).textTheme.bodySmall),
            ],
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final selected = _selected;
    final sourceQuote = _sourceQuote;
    final linkedInvoice = _linkedInvoice;
    final status = (selected?['status'] ?? '').toString();
    final sourceQuoteId = (selected?['source_quote_id'] ?? '').toString();
    final linkedInvoiceId =
        (selected?['linked_invoice_out_id'] ?? '').toString();
    final quoteStatus = (sourceQuote?['status'] ?? '').toString();
    final currency = (selected?['currency'] ?? 'EUR').toString();
    final netAmount = _toDouble(selected?['net_amount']);
    final taxAmount = _toDouble(selected?['tax_amount']);
    final grossAmount = _toDouble(selected?['gross_amount']);
    final sourceQuoteGross = _toDouble(sourceQuote?['gross_amount']);
    final sourceQuoteNet = _toDouble(sourceQuote?['net_amount']);
    final linkedInvoiceGross = _toDouble(linkedInvoice?['gross_amount']);
    final linkedInvoicePaid = _toDouble(linkedInvoice?['paid_amount']);
    final linkedInvoiceOpen =
        linkedInvoice == null ? 0 : (linkedInvoiceGross - linkedInvoicePaid);
    final linkedInvoiceStatus = (linkedInvoice?['status'] ?? '').toString();
    final relatedInvoices = _relatedInvoices
        .cast<Map>()
        .map((item) => item.cast<String, dynamic>())
        .toList();
    final relatedInvoiceCount = relatedInvoices.length;
    final itemsCount = ((selected?['items'] as List?) ?? const []).length;
    final openInvoiceableItems = ((selected?['items'] as List?) ?? const [])
        .cast<Map>()
        .map((item) => item.cast<String, dynamic>())
        .where((item) => _toDouble(item['remaining_qty']) > 0.0001)
        .toList();
    final remainingGrossAmount = openInvoiceableItems.fold<double>(
      0,
      (sum, item) => sum + _remainingGross(item),
    );
    final hasPartialInvoicing =
        relatedInvoiceCount > 0 && remainingGrossAmount > 0.0001;
    final isFullyInvoiced = relatedInvoiceCount > 0 && !hasPartialInvoicing;
    final invoiceProgressLabel = hasPartialInvoicing
        ? 'Teilfakturiert'
        : isFullyInvoiced
            ? 'Vollständig fakturiert'
            : 'Noch nicht fakturiert';
    final marginToQuote =
        sourceQuote == null ? null : grossAmount - sourceQuoteGross;
    final canWriteOrders = widget.api.hasPermission('sales_orders.write');
    final canCreateInvoice =
        canWriteOrders && widget.api.hasPermission('invoices_out.write');
    final canOpenInvoices = widget.api.hasPermission('invoices_out.read');
    final canStartInvoiceFlow = canCreateInvoice &&
        openInvoiceableItems.isNotEmpty &&
        (status == 'open' || status == 'released' || status == 'invoiced');
    final canEditHeader = canWriteOrders &&
        linkedInvoiceId.isEmpty &&
        (status == 'open' || status == 'released');
    final canEditItems = canEditHeader;
    final editLockReason = linkedInvoiceId.isNotEmpty
        ? 'Positionen und Kopfdaten sind nach der Faktura gesperrt.'
        : status == 'completed' || status == 'canceled' || status == 'invoiced'
            ? 'Bearbeitung ist im aktuellen Status nicht mehr erlaubt.'
            : '';
    final showWorkflowHint = !_workflowHintDismissed && selected != null;
    final selectedSalesOrderId = (selected?['id'] ?? '').toString();
    return Scaffold(
      appBar: AppBar(
        title: const Text('Aufträge'),
        actions: [
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh_rounded)),
        ],
      ),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Expanded(
              flex: 4,
              child: Card(
                child: Column(
                  children: [
                    Padding(
                      padding: const EdgeInsets.all(12),
                      child: Wrap(
                        spacing: 12,
                        runSpacing: 12,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          SizedBox(
                            width: 280,
                            child: TextField(
                              controller: _searchCtrl,
                              decoration: const InputDecoration(
                                  labelText: 'Suche (Nummer/Kunde)'),
                              onSubmitted: (_) => _load(),
                            ),
                          ),
                          SizedBox(
                            width: 220,
                            child: TextField(
                              controller: _projectCtrl,
                              decoration: const InputDecoration(
                                  labelText: 'Projekt-ID'),
                              onSubmitted: (_) => _load(),
                            ),
                          ),
                          SizedBox(
                            width: 180,
                            child: DropdownButtonFormField<String?>(
                              isExpanded: true,
                              initialValue: _statusFilter,
                              decoration:
                                  const InputDecoration(labelText: 'Status'),
                              items: [
                                const DropdownMenuItem(
                                    value: null, child: Text('Alle')),
                                ..._availableStatuses.map(
                                  (value) => DropdownMenuItem(
                                      value: value,
                                      child: Text(_statusLabel(value))),
                                ),
                              ],
                              onChanged: (value) =>
                                  setState(() => _statusFilter = value),
                            ),
                          ),
                          FilterChip(
                            label: const Text('Teilfaktura'),
                            selected: _partialOnlyFilter,
                            onSelected: (value) {
                              setState(() => _partialOnlyFilter = value);
                              _load();
                            },
                          ),
                          FilledButton(
                              onPressed: _load, child: const Text('Filtern')),
                        ],
                      ),
                    ),
                    const Divider(height: 1),
                    Expanded(
                      child: _loading
                          ? const Center(child: CircularProgressIndicator())
                          : _items.isEmpty
                              ? const Center(
                                  child: Text('Noch keine Aufträge gefunden.'))
                              : ListView.separated(
                                  itemCount: _items.length,
                                  separatorBuilder: (_, __) =>
                                      const Divider(height: 1),
                                  itemBuilder: (context, index) {
                                    final item =
                                        _items[index] as Map<String, dynamic>;
                                    final id = item['id']?.toString();
                                    final selectedId =
                                        _selected?['id']?.toString();
                                    return ListTile(
                                      selected: id != null && id == selectedId,
                                      title: Text((item['number'] ?? 'Auftrag')
                                          .toString()),
                                      subtitle: Text(
                                        '${item['contact_name'] ?? '-'}  •  ${_statusLabel((item['status'] ?? '').toString())}  •  ${_listInvoiceProgress(item)}  •  ${_formatMoney(item['gross_amount'] as num?, (item['currency'] ?? 'EUR').toString())}\n${_listInvoiceHint(item)}',
                                      ),
                                      isThreeLine: true,
                                      onTap: id == null
                                          ? null
                                          : () => _loadDetail(id),
                                    );
                                  },
                                ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(width: 16),
            Expanded(
              flex: 5,
              child: Card(
                child: selected == null
                    ? const Center(child: Text('Auftrag auswählen'))
                    : Padding(
                        padding: const EdgeInsets.all(16),
                        child: SingleChildScrollView(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              Wrap(
                                spacing: 8,
                                runSpacing: 8,
                                crossAxisAlignment: WrapCrossAlignment.center,
                                children: [
                                  SizedBox(
                                    width: 260,
                                    child: Text(
                                      (selected['number'] ?? 'Auftrag')
                                          .toString(),
                                      style: Theme.of(context)
                                          .textTheme
                                          .headlineSmall,
                                    ),
                                  ),
                                  if (linkedInvoiceId.isNotEmpty &&
                                      canOpenInvoices)
                                    FilledButton.icon(
                                      onPressed: () =>
                                          _openInvoice(linkedInvoiceId),
                                      icon: const Icon(
                                          Icons.receipt_long_rounded),
                                      label: const Text('Rechnung öffnen'),
                                    ),
                                  if (selectedSalesOrderId.isNotEmpty &&
                                      relatedInvoiceCount > 0 &&
                                      canOpenInvoices)
                                    OutlinedButton.icon(
                                      onPressed: () => _openSalesOrderInvoices(
                                          selectedSalesOrderId),
                                      icon: const Icon(
                                          Icons.receipt_long_rounded),
                                      label: const Text(
                                          'Auftragsrechnungen öffnen'),
                                    ),
                                  if (canEditHeader)
                                    OutlinedButton.icon(
                                      onPressed: _actionInProgress
                                          ? null
                                          : _editSalesOrderHeader,
                                      icon: const Icon(Icons.edit_outlined),
                                      label: const Text('Bearbeiten'),
                                    ),
                                  OutlinedButton.icon(
                                    onPressed: _downloadPdf,
                                    icon: const Icon(
                                        Icons.picture_as_pdf_rounded),
                                    label: const Text('PDF'),
                                  ),
                                  if (sourceQuoteId.isNotEmpty)
                                    OutlinedButton.icon(
                                      onPressed: () =>
                                          _openQuote(sourceQuoteId),
                                      icon: const Icon(
                                          Icons.request_quote_rounded),
                                      label: const Text('Angebot öffnen'),
                                    ),
                                ],
                              ),
                              const SizedBox(height: 12),
                              Card(
                                margin: EdgeInsets.zero,
                                child: Padding(
                                  padding: const EdgeInsets.all(12),
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      const Text('Auftragskopf',
                                          style: TextStyle(
                                              fontWeight: FontWeight.bold)),
                                      const SizedBox(height: 8),
                                      Wrap(
                                        spacing: 12,
                                        runSpacing: 8,
                                        children: [
                                          Text(
                                              'Auftrags-ID: ${(selected['id'] ?? '-').toString()}'),
                                          Text(
                                              'Kontakt-ID: ${(selected['contact_id'] ?? '-').toString()}'),
                                          Text(
                                              'Projekt-ID: ${(selected['project_id'] ?? '-').toString().isEmpty ? '-' : (selected['project_id'] ?? '-').toString()}'),
                                          Text('Währung: $currency'),
                                        ],
                                      ),
                                    ],
                                  ),
                                ),
                              ),
                              const SizedBox(height: 12),
                              Wrap(
                                spacing: 8,
                                runSpacing: 8,
                                children: [
                                  _statusChip(context, status),
                                  Chip(
                                      label: Text(
                                          'Kunde: ${(selected['contact_name'] ?? '-').toString()}')),
                                  Chip(
                                      label: Text(
                                          'Projekt: ${(selected['project_name'] ?? '-').toString()}')),
                                  if (sourceQuoteId.isNotEmpty)
                                    Chip(
                                        label: Text(
                                            'Quelle Angebot: $sourceQuoteId')),
                                  if (linkedInvoiceId.isNotEmpty)
                                    Chip(
                                        label: Text(
                                            'Folgerechnung: $linkedInvoiceId')),
                                  if (relatedInvoiceCount > 0)
                                    Chip(label: Text(invoiceProgressLabel)),
                                ],
                              ),
                              if (showWorkflowHint) ...[
                                const SizedBox(height: 16),
                                _SalesOrderWorkflowCard(
                                  statusLabel: _statusLabel(status),
                                  quoteStatusLabel: _statusLabel(quoteStatus),
                                  hasInvoice: linkedInvoiceId.isNotEmpty,
                                  invoiceCount: relatedInvoiceCount,
                                  hasRemainingToInvoice:
                                      openInvoiceableItems.isNotEmpty,
                                  canCreateInvoice: canStartInvoiceFlow,
                                  actionInProgress: _actionInProgress,
                                  onDismiss: () => setState(
                                      () => _workflowHintDismissed = true),
                                  onCreateInvoice: sourceQuoteId.isNotEmpty &&
                                          canStartInvoiceFlow
                                      ? _convertToInvoiceFromSalesOrder
                                      : null,
                                  onOpenInvoice: linkedInvoiceId.isNotEmpty &&
                                          canOpenInvoices
                                      ? () => _openInvoice(linkedInvoiceId)
                                      : null,
                                ),
                                const SizedBox(height: 16),
                              ],
                              Wrap(
                                spacing: 12,
                                runSpacing: 12,
                                children: [
                                  _buildFlowStep(
                                    context,
                                    icon: Icons.request_quote_rounded,
                                    title: 'Angebot',
                                    subtitle: sourceQuoteId.isEmpty
                                        ? 'Kein Quellangebot verknüpft'
                                        : '${_statusLabel(quoteStatus)}${(sourceQuote?['accepted_at'] ?? '').toString().isNotEmpty ? ' seit ${_formatDate(context, (sourceQuote?['accepted_at'] ?? '').toString())}' : ''}',
                                    active: sourceQuoteId.isNotEmpty &&
                                        linkedInvoiceId.isEmpty,
                                    done: sourceQuoteId.isNotEmpty,
                                  ),
                                  _buildFlowStep(
                                    context,
                                    icon: Icons.assignment_turned_in_rounded,
                                    title: 'Auftrag',
                                    subtitle:
                                        'Status ${_statusLabel(status)} seit ${_formatDate(context, (selected['order_date'] ?? '').toString())}',
                                    active: status == 'open' &&
                                        linkedInvoiceId.isEmpty,
                                    done: linkedInvoiceId.isNotEmpty,
                                  ),
                                  _buildFlowStep(
                                    context,
                                    icon: Icons.receipt_long_rounded,
                                    title: 'Faktura',
                                    subtitle: linkedInvoiceId.isEmpty
                                        ? 'Noch keine Rechnung erzeugt'
                                        : linkedInvoice == null
                                            ? 'Folgebeleg $linkedInvoiceId ist vorhanden'
                                            : '$invoiceProgressLabel · $relatedInvoiceCount ${relatedInvoiceCount == 1 ? 'Rechnung' : 'Rechnungen'}',
                                    active: linkedInvoiceId.isEmpty,
                                    done: linkedInvoiceId.isNotEmpty,
                                  ),
                                ],
                              ),
                              const SizedBox(height: 12),
                              Text(
                                  'Hinweis: ${(selected['note'] ?? '').toString()}'),
                              const SizedBox(height: 4),
                              Text(
                                  'Auftragsdatum: ${_formatDate(context, (selected['order_date'] ?? '').toString())}'),
                              const SizedBox(height: 16),
                              Wrap(
                                spacing: 12,
                                runSpacing: 12,
                                children: [
                                  _buildMetricCard(
                                    context,
                                    label: 'Auftragswert',
                                    value: _formatMoney(grossAmount, currency),
                                    hint: '${itemsCount.toString()} Positionen',
                                  ),
                                  _buildMetricCard(
                                    context,
                                    label: 'Netto / Steuer',
                                    value:
                                        '${_formatMoney(netAmount, currency)} / ${_formatMoney(taxAmount, currency)}',
                                    hint: 'Summen aus aktueller Positionierung',
                                  ),
                                  _buildMetricCard(
                                    context,
                                    label: 'Abgleich Angebot',
                                    value: sourceQuote == null
                                        ? 'Kein Vergleich'
                                        : _formatMoney(marginToQuote, currency),
                                    hint: sourceQuote == null
                                        ? ''
                                        : 'Angebot ${_formatMoney(sourceQuoteGross, currency)} brutto, netto ${_formatMoney(sourceQuoteNet, currency)}',
                                  ),
                                  _buildMetricCard(
                                    context,
                                    label: 'Faktura / Offen',
                                    value: linkedInvoiceId.isEmpty
                                        ? 'Noch offen'
                                        : linkedInvoice == null
                                            ? 'Rechnung wird geladen'
                                            : '${_formatMoney(linkedInvoiceGross, (linkedInvoice['currency'] ?? currency).toString())} / ${_formatMoney(linkedInvoiceOpen, (linkedInvoice['currency'] ?? currency).toString())}',
                                    hint: linkedInvoiceId.isEmpty
                                        ? 'Noch kein Folgebeleg erzeugt'
                                        : linkedInvoice == null
                                            ? 'Status und Restbetrag werden nachgeladen'
                                            : 'Rechnungsstatus ${_statusLabel(linkedInvoiceStatus)}, bezahlt ${_formatMoney(linkedInvoicePaid, (linkedInvoice['currency'] ?? currency).toString())}',
                                  ),
                                  _buildMetricCard(
                                    context,
                                    label: 'Rest zur Faktura',
                                    value: _formatMoney(
                                        remainingGrossAmount, currency),
                                    hint: hasPartialInvoicing
                                        ? 'Teilfaktura aktiv: ${openInvoiceableItems.length} Positionen noch fakturierbar'
                                        : isFullyInvoiced
                                            ? 'Vollständig fakturiert über $relatedInvoiceCount ${relatedInvoiceCount == 1 ? 'Rechnung' : 'Rechnungen'}'
                                            : 'Keine Rechnung erzeugt',
                                  ),
                                ],
                              ),
                              if (sourceQuoteId.isNotEmpty) ...[
                                const SizedBox(height: 16),
                                const Text('Kontext',
                                    style:
                                        TextStyle(fontWeight: FontWeight.bold)),
                                const SizedBox(height: 8),
                                Card(
                                  margin: EdgeInsets.zero,
                                  child: Padding(
                                    padding: const EdgeInsets.all(12),
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Text('Quellangebot: $sourceQuoteId',
                                            style: const TextStyle(
                                                fontWeight: FontWeight.w600)),
                                        const SizedBox(height: 6),
                                        Text(
                                            'Status: ${sourceQuote == null ? 'wird geladen' : _statusLabel(quoteStatus)}'),
                                        if (sourceQuote != null) ...[
                                          Text(
                                              'Projekt: ${(sourceQuote['project_name'] ?? selected['project_name'] ?? '-').toString()}'),
                                          Text(
                                              'Gültig bis: ${_formatDate(context, (sourceQuote['valid_until'] ?? '').toString())}'),
                                        ],
                                        if (canStartInvoiceFlow)
                                          Padding(
                                            padding:
                                                const EdgeInsets.only(top: 8),
                                            child: Wrap(
                                              spacing: 8,
                                              runSpacing: 8,
                                              children: [
                                                FilledButton.icon(
                                                  onPressed: _actionInProgress
                                                      ? null
                                                      : _convertToInvoiceFromSalesOrder,
                                                  icon: const Icon(Icons
                                                      .receipt_long_rounded),
                                                  label: Text(linkedInvoiceId
                                                          .isNotEmpty
                                                      ? 'Weitere Rechnung erzeugen'
                                                      : 'In Rechnung weiterführen'),
                                                ),
                                              ],
                                            ),
                                          )
                                        else if (linkedInvoiceId.isNotEmpty)
                                          const Padding(
                                            padding: EdgeInsets.only(top: 8),
                                            child: Text(
                                                'Die Faktura wurde bereits aus dem zugrunde liegenden Angebot erzeugt und es sind keine offenen Restmengen mehr vorhanden.'),
                                          )
                                        else
                                          Padding(
                                            padding: EdgeInsets.only(top: 8),
                                            child: Text(
                                              canCreateInvoice
                                                  ? 'Direkte Faktura ist nur aus offenen, freigegebenen oder bereits teilweise fakturierten Aufträgen mit Restmengen erlaubt.'
                                                  : 'Für die direkte Faktura aus dem Auftrag sind sales_orders.write und invoices_out.write erforderlich.',
                                            ),
                                          ),
                                      ],
                                    ),
                                  ),
                                ),
                              ],
                              if (linkedInvoiceId.isNotEmpty) ...[
                                const SizedBox(height: 12),
                                Card(
                                  margin: EdgeInsets.zero,
                                  child: Padding(
                                    padding: const EdgeInsets.all(12),
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          linkedInvoice == null
                                              ? 'Verknüpfte Rechnung wird geladen'
                                              : 'Verknüpfte Rechnung ${(linkedInvoice['nummer'] ?? linkedInvoice['number'] ?? linkedInvoiceId).toString()}',
                                          style: const TextStyle(
                                              fontWeight: FontWeight.w600),
                                        ),
                                        const SizedBox(height: 6),
                                        if (linkedInvoice == null)
                                          const Text(
                                              'Status, Zahlungen und Restbetrag werden nachgeladen.')
                                        else ...[
                                          Text(
                                              'Status: ${_statusLabel(linkedInvoiceStatus)}'),
                                          Text(
                                              'Brutto: ${_formatMoney(linkedInvoiceGross, (linkedInvoice['currency'] ?? currency).toString())}'),
                                          Text(
                                              'Bezahlt: ${_formatMoney(linkedInvoicePaid, (linkedInvoice['currency'] ?? currency).toString())}'),
                                          Text(
                                              'Offen: ${_formatMoney(linkedInvoiceOpen, (linkedInvoice['currency'] ?? currency).toString())}'),
                                        ],
                                      ],
                                    ),
                                  ),
                                ),
                              ],
                              if (relatedInvoiceCount > 0) ...[
                                const SizedBox(height: 12),
                                Card(
                                  margin: EdgeInsets.zero,
                                  child: Padding(
                                    padding: const EdgeInsets.all(12),
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          'Folgerechnungen ($relatedInvoiceCount)',
                                          style: const TextStyle(
                                              fontWeight: FontWeight.w600),
                                        ),
                                        const SizedBox(height: 4),
                                        Text(invoiceProgressLabel),
                                        const SizedBox(height: 8),
                                        ...relatedInvoices.map((invoice) {
                                          final invoiceId =
                                              (invoice['id'] ?? '').toString();
                                          final invoiceNumber =
                                              (invoice['number'] ??
                                                      invoice['nummer'] ??
                                                      invoiceId)
                                                  .toString();
                                          final invoiceCurrency =
                                              (invoice['currency'] ?? currency)
                                                  .toString();
                                          final invoiceGross = _toDouble(
                                              invoice['gross_amount']);
                                          final invoicePaid =
                                              _toDouble(invoice['paid_amount']);
                                          final invoiceOpen =
                                              invoiceGross - invoicePaid;
                                          final isLatest =
                                              invoiceId.isNotEmpty &&
                                                  invoiceId == linkedInvoiceId;
                                          return Card(
                                            margin: const EdgeInsets.only(
                                                bottom: 8),
                                            child: ListTile(
                                              contentPadding:
                                                  const EdgeInsets.symmetric(
                                                      horizontal: 12,
                                                      vertical: 4),
                                              title: Text(invoiceNumber),
                                              subtitle: Text(
                                                '${isLatest ? 'Letzte Rechnung' : 'Weitere Rechnung'}  •  ${_statusLabel((invoice['status'] ?? '').toString())}  •  Brutto ${_formatMoney(invoiceGross, invoiceCurrency)}  •  Offen ${_formatMoney(invoiceOpen, invoiceCurrency)}',
                                              ),
                                              trailing: canOpenInvoices
                                                  ? TextButton(
                                                      onPressed: () =>
                                                          _openInvoice(
                                                              invoiceId),
                                                      child:
                                                          const Text('Öffnen'),
                                                    )
                                                  : Text(isLatest
                                                      ? 'Letzte Rechnung'
                                                      : 'Weitere Rechnung'),
                                            ),
                                          );
                                        }),
                                      ],
                                    ),
                                  ),
                                ),
                              ],
                              const SizedBox(height: 16),
                              Wrap(
                                spacing: 8,
                                runSpacing: 8,
                                crossAxisAlignment: WrapCrossAlignment.center,
                                children: [
                                  const Text('Positionen',
                                      style: TextStyle(
                                          fontWeight: FontWeight.bold)),
                                  if (canEditItems)
                                    OutlinedButton.icon(
                                      onPressed: _actionInProgress
                                          ? null
                                          : _createSalesOrderItem,
                                      icon: const Icon(Icons.add_rounded),
                                      label: const Text('Position'),
                                    ),
                                ],
                              ),
                              const SizedBox(height: 8),
                              if (!canEditItems && editLockReason.isNotEmpty)
                                Padding(
                                  padding: const EdgeInsets.only(bottom: 8),
                                  child: Material(
                                    color: Theme.of(context)
                                        .colorScheme
                                        .surfaceContainerHighest,
                                    borderRadius: BorderRadius.circular(12),
                                    child: Padding(
                                      padding: const EdgeInsets.all(12),
                                      child: Row(
                                        children: [
                                          const Icon(
                                              Icons.lock_outline_rounded),
                                          const SizedBox(width: 8),
                                          Expanded(child: Text(editLockReason)),
                                        ],
                                      ),
                                    ),
                                  ),
                                ),
                              ListView.separated(
                                shrinkWrap: true,
                                physics: const NeverScrollableScrollPhysics(),
                                itemCount:
                                    ((selected['items'] as List?) ?? const [])
                                        .length,
                                separatorBuilder: (_, __) =>
                                    const Divider(height: 1),
                                itemBuilder: (context, index) {
                                  final item = (selected['items']
                                      as List)[index] as Map<String, dynamic>;
                                  final currency =
                                      (selected['currency'] ?? 'EUR')
                                          .toString();
                                  final lineNet = _lineNet(item);
                                  final lineGross = _lineGross(item);
                                  final invoicedQty =
                                      _toDouble(item['invoiced_qty']);
                                  final remainingQty =
                                      _toDouble(item['remaining_qty']);
                                  return ListTile(
                                    title: Text(
                                        '${(item['position'] ?? index + 1).toString()}. ${(item['description'] ?? 'Position').toString()}'),
                                    subtitle: Text(
                                      'Menge ${(item['qty'] ?? 0)} ${(item['unit'] ?? '')}  •  Fakturiert ${invoicedQty.toStringAsFixed(invoicedQty.truncateToDouble() == invoicedQty ? 0 : 2)}  •  Offen ${remainingQty.toStringAsFixed(remainingQty.truncateToDouble() == remainingQty ? 0 : 2)}  •  Einzelpreis ${_formatMoney((item['unit_price'] ?? 0) as num?, currency)}  •  Steuer ${(item['tax_code'] ?? '').toString().isEmpty ? 'ohne' : (item['tax_code'] ?? '').toString()}  •  Netto ${_formatMoney(lineNet, currency)}',
                                    ),
                                    trailing: canEditItems
                                        ? Wrap(
                                            spacing: 4,
                                            children: [
                                              IconButton(
                                                onPressed: _actionInProgress
                                                    ? null
                                                    : () => _editSalesOrderItem(
                                                        item),
                                                icon: const Icon(
                                                    Icons.edit_outlined),
                                              ),
                                              IconButton(
                                                onPressed: _actionInProgress
                                                    ? null
                                                    : () =>
                                                        _deleteSalesOrderItem(
                                                            item),
                                                icon: const Icon(
                                                    Icons.delete_outline),
                                              ),
                                              Padding(
                                                padding: const EdgeInsets.only(
                                                    top: 12),
                                                child: Text(_formatMoney(
                                                    lineGross, currency)),
                                              ),
                                            ],
                                          )
                                        : Text(
                                            _formatMoney(lineGross, currency)),
                                  );
                                },
                              ),
                              const SizedBox(height: 12),
                              Align(
                                alignment: Alignment.centerRight,
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.end,
                                  children: [
                                    Text(
                                        'Netto: ${_formatMoney((selected['net_amount'] ?? 0) as num?, (selected['currency'] ?? 'EUR').toString())}'),
                                    Text(
                                        'Steuer: ${_formatMoney((selected['tax_amount'] ?? 0) as num?, (selected['currency'] ?? 'EUR').toString())}'),
                                    Text(
                                      'Brutto: ${_formatMoney((selected['gross_amount'] ?? 0) as num?, (selected['currency'] ?? 'EUR').toString())}',
                                      style: const TextStyle(
                                          fontWeight: FontWeight.bold),
                                    ),
                                  ],
                                ),
                              ),
                              if (canWriteOrders) ...[
                                const SizedBox(height: 16),
                                Wrap(
                                  spacing: 8,
                                  runSpacing: 8,
                                  children: [
                                    if (linkedInvoiceId.isEmpty &&
                                        status != 'open')
                                      OutlinedButton(
                                        onPressed: _actionInProgress
                                            ? null
                                            : () => _updateStatus('open'),
                                        child: const Text('Auf Offen'),
                                      ),
                                    if (linkedInvoiceId.isEmpty &&
                                        status != 'released' &&
                                        status != 'canceled' &&
                                        status != 'completed')
                                      FilledButton.tonalIcon(
                                        onPressed: _actionInProgress
                                            ? null
                                            : () => _updateStatus('released'),
                                        icon: const Icon(Icons.publish_rounded),
                                        label: const Text('Freigeben'),
                                      ),
                                    if (linkedInvoiceId.isEmpty &&
                                        status != 'canceled' &&
                                        status != 'completed')
                                      FilledButton.tonal(
                                        onPressed: _actionInProgress
                                            ? null
                                            : () => _updateStatus('canceled'),
                                        child: const Text('Stornieren'),
                                      ),
                                    if (linkedInvoiceId.isNotEmpty &&
                                        status != 'completed')
                                      FilledButton.icon(
                                        onPressed: _actionInProgress
                                            ? null
                                            : () => _updateStatus('completed'),
                                        icon:
                                            const Icon(Icons.task_alt_rounded),
                                        label: const Text('Abschließen'),
                                      ),
                                  ],
                                ),
                              ],
                            ],
                          ),
                        ),
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SalesOrderConvertRequest {
  const _SalesOrderConvertRequest({
    required this.revenueAccount,
    required this.items,
    this.dueDate,
  });

  final String revenueAccount;
  final List<_SalesOrderConvertItemRequest> items;
  final DateTime? dueDate;
}

class _SalesOrderConvertItemRequest {
  const _SalesOrderConvertItemRequest({
    required this.salesOrderItemId,
    required this.qty,
  });

  final String salesOrderItemId;
  final double qty;

  Map<String, dynamic> toJson() => {
        'sales_order_item_id': salesOrderItemId,
        'qty': qty,
      };
}

class _SalesOrderConvertLine {
  const _SalesOrderConvertLine({
    required this.salesOrderItemId,
    required this.description,
    required this.unit,
    required this.remainingQty,
    required this.invoicedQty,
    required this.unitPrice,
    required this.taxCode,
  });

  final String salesOrderItemId;
  final String description;
  final String unit;
  final double remainingQty;
  final double invoicedQty;
  final double unitPrice;
  final String taxCode;
}

class _SalesOrderHeaderUpdateRequest {
  const _SalesOrderHeaderUpdateRequest({
    required this.number,
    required this.orderDate,
    required this.currency,
    required this.note,
  });

  final String number;
  final DateTime orderDate;
  final String currency;
  final String note;
}

class _SalesOrderHeaderDialog extends StatefulWidget {
  const _SalesOrderHeaderDialog({
    required this.initialNumber,
    required this.initialCurrency,
    required this.initialNote,
    this.initialOrderDate,
  });

  final String initialNumber;
  final String initialCurrency;
  final String initialNote;
  final DateTime? initialOrderDate;

  @override
  State<_SalesOrderHeaderDialog> createState() =>
      _SalesOrderHeaderDialogState();
}

class _SalesOrderHeaderDialogState extends State<_SalesOrderHeaderDialog> {
  late final TextEditingController _numberCtrl;
  late final TextEditingController _currencyCtrl;
  late final TextEditingController _noteCtrl;
  late DateTime _orderDate;

  @override
  void initState() {
    super.initState();
    _numberCtrl = TextEditingController(text: widget.initialNumber);
    _currencyCtrl = TextEditingController(text: widget.initialCurrency);
    _noteCtrl = TextEditingController(text: widget.initialNote);
    _orderDate = widget.initialOrderDate ?? DateTime.now();
  }

  @override
  void dispose() {
    _numberCtrl.dispose();
    _currencyCtrl.dispose();
    _noteCtrl.dispose();
    super.dispose();
  }

  Future<void> _pickOrderDate() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _orderDate,
      firstDate: DateTime(_orderDate.year - 5),
      lastDate: DateTime(_orderDate.year + 5),
    );
    if (picked == null || !mounted) return;
    setState(() => _orderDate = picked);
  }

  @override
  Widget build(BuildContext context) {
    final orderDateLabel =
        MaterialLocalizations.of(context).formatMediumDate(_orderDate);
    return AlertDialog(
      title: const Text('Auftragskopf bearbeiten'),
      content: SizedBox(
        width: 460,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            TextField(
              controller: _numberCtrl,
              decoration: const InputDecoration(labelText: 'Auftragsnummer'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _currencyCtrl,
              decoration: const InputDecoration(labelText: 'Währung'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _noteCtrl,
              maxLines: 3,
              decoration: const InputDecoration(labelText: 'Hinweis'),
            ),
            const SizedBox(height: 12),
            ListTile(
              contentPadding: EdgeInsets.zero,
              leading: const Icon(Icons.event_rounded),
              title: const Text('Auftragsdatum'),
              subtitle: Text(orderDateLabel),
              trailing: TextButton(
                onPressed: _pickOrderDate,
                child: const Text('Datum wählen'),
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: () {
            Navigator.of(context).pop(
              _SalesOrderHeaderUpdateRequest(
                number: _numberCtrl.text.trim(),
                orderDate: _orderDate,
                currency: _currencyCtrl.text.trim().toUpperCase(),
                note: _noteCtrl.text.trim(),
              ),
            );
          },
          child: const Text('Speichern'),
        ),
      ],
    );
  }
}

class _SalesOrderItemRequest {
  const _SalesOrderItemRequest({
    required this.description,
    required this.qty,
    required this.unit,
    required this.unitPrice,
    required this.taxCode,
  });

  final String description;
  final double qty;
  final String unit;
  final double unitPrice;
  final String taxCode;

  Map<String, dynamic> toJson() => {
        'description': description,
        'qty': qty,
        'unit': unit,
        'unit_price': unitPrice,
        'tax_code': taxCode,
      };
}

class _SalesOrderItemDialog extends StatefulWidget {
  const _SalesOrderItemDialog({
    this.initialDescription = '',
    this.initialQty = '1',
    this.initialUnit = 'Stk',
    this.initialUnitPrice = '0',
    this.initialTaxCode = 'DE19',
  });

  final String initialDescription;
  final String initialQty;
  final String initialUnit;
  final String initialUnitPrice;
  final String initialTaxCode;

  @override
  State<_SalesOrderItemDialog> createState() => _SalesOrderItemDialogState();
}

class _SalesOrderItemDialogState extends State<_SalesOrderItemDialog> {
  late final TextEditingController _descriptionCtrl;
  late final TextEditingController _qtyCtrl;
  late final TextEditingController _unitCtrl;
  late final TextEditingController _unitPriceCtrl;
  late final TextEditingController _taxCodeCtrl;

  @override
  void initState() {
    super.initState();
    _descriptionCtrl = TextEditingController(text: widget.initialDescription);
    _qtyCtrl = TextEditingController(text: widget.initialQty);
    _unitCtrl = TextEditingController(text: widget.initialUnit);
    _unitPriceCtrl = TextEditingController(text: widget.initialUnitPrice);
    _taxCodeCtrl = TextEditingController(text: widget.initialTaxCode);
  }

  @override
  void dispose() {
    _descriptionCtrl.dispose();
    _qtyCtrl.dispose();
    _unitCtrl.dispose();
    _unitPriceCtrl.dispose();
    _taxCodeCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Auftragsposition'),
      content: SizedBox(
        width: 460,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: _descriptionCtrl,
              decoration: const InputDecoration(labelText: 'Beschreibung'),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _qtyCtrl,
                    keyboardType:
                        const TextInputType.numberWithOptions(decimal: true),
                    decoration: const InputDecoration(labelText: 'Menge'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: TextField(
                    controller: _unitCtrl,
                    decoration: const InputDecoration(labelText: 'Einheit'),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _unitPriceCtrl,
                    keyboardType:
                        const TextInputType.numberWithOptions(decimal: true),
                    decoration: const InputDecoration(labelText: 'Einzelpreis'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: TextField(
                    controller: _taxCodeCtrl,
                    decoration: const InputDecoration(labelText: 'Steuercode'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: () {
            final qty =
                double.tryParse(_qtyCtrl.text.trim().replaceAll(',', '.'));
            final unitPrice = double.tryParse(
                _unitPriceCtrl.text.trim().replaceAll(',', '.'));
            final description = _descriptionCtrl.text.trim();
            final unit = _unitCtrl.text.trim();
            final taxCode = _taxCodeCtrl.text.trim().toUpperCase();
            if (qty == null || unitPrice == null) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                    content: Text('Menge und Preis müssen Zahlen sein')),
              );
              return;
            }
            if (description.isEmpty) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Beschreibung ist erforderlich')),
              );
              return;
            }
            if (unit.isEmpty) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Einheit ist erforderlich')),
              );
              return;
            }
            if (qty <= 0) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Menge muss größer als 0 sein')),
              );
              return;
            }
            if (unitPrice < 0) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                    content: Text('Einzelpreis darf nicht negativ sein')),
              );
              return;
            }
            if (taxCode.isNotEmpty && taxCode != 'DE19' && taxCode != 'DE7') {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                    content: Text('Steuercode muss leer, DE19 oder DE7 sein')),
              );
              return;
            }
            Navigator.of(context).pop(
              _SalesOrderItemRequest(
                description: description,
                qty: qty,
                unit: unit,
                unitPrice: unitPrice,
                taxCode: taxCode,
              ),
            );
          },
          child: const Text('Speichern'),
        ),
      ],
    );
  }
}

class _SalesOrderConvertDialog extends StatefulWidget {
  const _SalesOrderConvertDialog({required this.items});

  final List<_SalesOrderConvertLine> items;

  @override
  State<_SalesOrderConvertDialog> createState() =>
      _SalesOrderConvertDialogState();
}

class _SalesOrderConvertDialogState extends State<_SalesOrderConvertDialog> {
  late final TextEditingController _revenueAccountCtrl;
  DateTime? _dueDate;
  late final List<_SalesOrderConvertLineDraft> _lines;

  @override
  void initState() {
    super.initState();
    _revenueAccountCtrl = TextEditingController(text: '8000');
    _lines = widget.items.map(_SalesOrderConvertLineDraft.fromLine).toList();
  }

  @override
  void dispose() {
    _revenueAccountCtrl.dispose();
    for (final line in _lines) {
      line.dispose();
    }
    super.dispose();
  }

  Future<void> _pickDueDate() async {
    final now = DateTime.now();
    final picked = await showDatePicker(
      context: context,
      initialDate: _dueDate ?? now.add(const Duration(days: 14)),
      firstDate: DateTime(now.year - 1),
      lastDate: DateTime(now.year + 5),
    );
    if (picked == null || !mounted) return;
    setState(() => _dueDate = picked);
  }

  @override
  Widget build(BuildContext context) {
    final dueDateLabel = _dueDate == null
        ? 'Keine Fälligkeit gesetzt'
        : MaterialLocalizations.of(context).formatMediumDate(_dueDate!);
    return AlertDialog(
      title: const Text('Auftrag in Rechnung weiterführen'),
      content: SizedBox(
        width: 620,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Wähle die offenen Restmengen aus, die jetzt fakturiert werden sollen.',
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _revenueAccountCtrl,
                decoration: const InputDecoration(
                  labelText: 'Erlöskonto',
                  helperText: 'Standard ist 8000',
                ),
              ),
              const SizedBox(height: 16),
              Row(
                children: [
                  Expanded(child: Text('Fälligkeit: $dueDateLabel')),
                  TextButton.icon(
                    onPressed: _pickDueDate,
                    icon: const Icon(Icons.event_rounded),
                    label: const Text('Wählen'),
                  ),
                ],
              ),
              if (_dueDate != null)
                Align(
                  alignment: Alignment.centerLeft,
                  child: TextButton(
                    onPressed: () => setState(() => _dueDate = null),
                    child: const Text('Fälligkeit entfernen'),
                  ),
                ),
              const SizedBox(height: 12),
              const Text('Offene Auftragspositionen',
                  style: TextStyle(fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              for (final line in _lines) ...[
                Card(
                  margin: const EdgeInsets.only(bottom: 8),
                  child: Padding(
                    padding: const EdgeInsets.all(12),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        CheckboxListTile(
                          contentPadding: EdgeInsets.zero,
                          value: line.selected,
                          onChanged: (value) =>
                              setState(() => line.selected = value ?? false),
                          title: Text(line.description),
                          subtitle: Text(
                            'Bereits fakturiert ${line.invoicedQty.toStringAsFixed(line.invoicedQty.truncateToDouble() == line.invoicedQty ? 0 : 2)} ${line.unit}  •  Offen ${line.remainingQty.toStringAsFixed(line.remainingQty.truncateToDouble() == line.remainingQty ? 0 : 2)} ${line.unit}  •  Einzelpreis ${line.unitPrice.toStringAsFixed(2)}  •  Steuer ${line.taxCode.isEmpty ? 'ohne' : line.taxCode}',
                          ),
                        ),
                        if (line.selected)
                          TextField(
                            controller: line.qtyCtrl,
                            keyboardType: const TextInputType.numberWithOptions(
                                decimal: true),
                            decoration: InputDecoration(
                              labelText: 'Jetzt fakturieren',
                              helperText:
                                  'Maximal ${line.remainingQty.toStringAsFixed(line.remainingQty.truncateToDouble() == line.remainingQty ? 0 : 2)} ${line.unit}',
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: () {
            final selectedItems = <_SalesOrderConvertItemRequest>[];
            for (final line in _lines) {
              if (!line.selected) continue;
              final qty = double.tryParse(
                  line.qtyCtrl.text.trim().replaceAll(',', '.'));
              if (qty == null) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                      content:
                          Text('Menge für "${line.description}" ist ungültig')),
                );
                return;
              }
              if (qty <= 0) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                      content: Text(
                          'Menge für "${line.description}" muss größer als 0 sein')),
                );
                return;
              }
              if (qty > line.remainingQty) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                      content: Text(
                          'Menge für "${line.description}" überschreitet die offene Restmenge')),
                );
                return;
              }
              selectedItems.add(
                _SalesOrderConvertItemRequest(
                  salesOrderItemId: line.salesOrderItemId,
                  qty: qty,
                ),
              );
            }
            if (selectedItems.isEmpty) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                    content: Text(
                        'Mindestens eine offene Position muss ausgewählt werden')),
              );
              return;
            }
            Navigator.of(context).pop(
              _SalesOrderConvertRequest(
                revenueAccount: _revenueAccountCtrl.text.trim().isEmpty
                    ? '8000'
                    : _revenueAccountCtrl.text.trim(),
                items: selectedItems,
                dueDate: _dueDate,
              ),
            );
          },
          child: const Text('Rechnung erzeugen'),
        ),
      ],
    );
  }
}

class _SalesOrderConvertLineDraft {
  _SalesOrderConvertLineDraft({
    required this.salesOrderItemId,
    required this.description,
    required this.unit,
    required this.remainingQty,
    required this.invoicedQty,
    required this.unitPrice,
    required this.taxCode,
    required this.selected,
  }) : qtyCtrl = TextEditingController(
          text: remainingQty.truncateToDouble() == remainingQty
              ? remainingQty.toStringAsFixed(0)
              : remainingQty.toStringAsFixed(2),
        );

  factory _SalesOrderConvertLineDraft.fromLine(_SalesOrderConvertLine line) {
    return _SalesOrderConvertLineDraft(
      salesOrderItemId: line.salesOrderItemId,
      description: line.description,
      unit: line.unit,
      remainingQty: line.remainingQty,
      invoicedQty: line.invoicedQty,
      unitPrice: line.unitPrice,
      taxCode: line.taxCode,
      selected: true,
    );
  }

  final String salesOrderItemId;
  final String description;
  final String unit;
  final double remainingQty;
  final double invoicedQty;
  final double unitPrice;
  final String taxCode;
  bool selected;
  final TextEditingController qtyCtrl;

  void dispose() {
    qtyCtrl.dispose();
  }
}

class _SalesOrderWorkflowCard extends StatelessWidget {
  const _SalesOrderWorkflowCard({
    required this.statusLabel,
    required this.quoteStatusLabel,
    required this.hasInvoice,
    required this.invoiceCount,
    required this.hasRemainingToInvoice,
    required this.canCreateInvoice,
    required this.actionInProgress,
    required this.onDismiss,
    this.onCreateInvoice,
    this.onOpenInvoice,
  });

  final String statusLabel;
  final String quoteStatusLabel;
  final bool hasInvoice;
  final int invoiceCount;
  final bool hasRemainingToInvoice;
  final bool canCreateInvoice;
  final bool actionInProgress;
  final VoidCallback onDismiss;
  final VoidCallback? onCreateInvoice;
  final VoidCallback? onOpenInvoice;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Card(
      color: scheme.secondaryContainer.withValues(alpha: 0.5),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.alt_route_rounded, color: scheme.primary),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    'Auftragsworkflow',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                IconButton(
                  onPressed: onDismiss,
                  icon: const Icon(Icons.close_rounded),
                  tooltip: 'Hinweis schließen',
                ),
              ],
            ),
            Text(
              hasInvoice
                  ? hasRemainingToInvoice
                      ? 'Zum Auftrag existieren bereits ${invoiceCount <= 1 ? 'eine Rechnung' : '$invoiceCount Rechnungen'}. Weitere offene Restmengen können direkt weiterfakturiert werden.'
                      : 'Zum Auftrag existieren bereits ${invoiceCount <= 1 ? 'ein Rechnungsfolgebeleg' : '$invoiceCount Rechnungsfolgebelege'}. Du kannst direkt in die letzte Faktura springen.'
                  : 'Das Quellangebot steht auf $quoteStatusLabel, der Auftrag auf $statusLabel. Von hier aus kann jetzt der nächste kaufmännische Schritt angestoßen werden.',
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                if (hasInvoice && onOpenInvoice != null)
                  FilledButton.icon(
                    onPressed: onOpenInvoice,
                    icon: const Icon(Icons.open_in_new_rounded),
                    label: const Text('Zur Rechnung'),
                  ),
                if (canCreateInvoice && onCreateInvoice != null)
                  FilledButton.icon(
                    onPressed: actionInProgress ? null : onCreateInvoice,
                    icon: const Icon(Icons.receipt_long_rounded),
                    label: Text(
                      actionInProgress
                          ? 'Erzeuge Rechnung ...'
                          : hasInvoice
                              ? 'Weitere Rechnung'
                              : 'Rechnung erzeugen',
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
