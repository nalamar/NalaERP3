import 'package:flutter/material.dart';

import '../api.dart';
import 'invoices_page.dart';
import 'sales_orders_page.dart';

String _quoteErrorMessage(Object error,
    {String fallback = 'Vorgang fehlgeschlagen'}) {
  if (error is ApiException) {
    return error.message;
  }
  return '$fallback: $error';
}

class QuotesPage extends StatefulWidget {
  const QuotesPage({
    super.key,
    required this.api,
    this.initialProjectId,
    this.initialQuoteId,
    this.openCreateOnStart = false,
  });

  final ApiClient api;
  final String? initialProjectId;
  final String? initialQuoteId;
  final bool openCreateOnStart;

  @override
  State<QuotesPage> createState() => _QuotesPageState();
}

class _QuotesPageState extends State<QuotesPage> {
  bool _loading = true;
  List<dynamic> _items = const [];
  Map<String, dynamic>? _selected;
  Map<String, dynamic>? _linkedSalesOrder;
  List<dynamic> _salesOrderInvoices = const [];
  final _searchCtrl = TextEditingController();
  final _projectCtrl = TextEditingController();
  String? _statusFilter;
  bool _followUpOnlyFilter = false;

  @override
  void initState() {
    super.initState();
    if (widget.initialProjectId != null &&
        widget.initialProjectId!.trim().isNotEmpty) {
      _projectCtrl.text = widget.initialProjectId!.trim();
    }
    final initialQuoteId = widget.initialQuoteId?.trim();
    if (initialQuoteId != null && initialQuoteId.isNotEmpty) {
      _searchCtrl.text = initialQuoteId;
    }
    _load();
    if (widget.openCreateOnStart) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _openCreateDialog(projectId: widget.initialProjectId);
      });
    }
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
      final list = await widget.api.listQuotes(
        q: _searchCtrl.text.trim().isEmpty ? null : _searchCtrl.text.trim(),
        projectId:
            _projectCtrl.text.trim().isEmpty ? null : _projectCtrl.text.trim(),
        status: _statusFilter,
      );
      final filteredList = _followUpOnlyFilter
          ? list.where((entry) {
              final item = (entry as Map).cast<String, dynamic>();
              return (item['linked_sales_order_id'] ?? '')
                      .toString()
                      .trim()
                      .isNotEmpty ||
                  (item['linked_invoice_out_id'] ?? '')
                      .toString()
                      .trim()
                      .isNotEmpty;
            }).toList()
          : list;
      setState(() => _items = filteredList);
      final selectedId = _selected?['id']?.toString();
      if (selectedId != null && selectedId.isNotEmpty) {
        await _loadDetail(selectedId);
      } else {
        final initialQuoteId = widget.initialQuoteId?.trim();
        if (initialQuoteId != null && initialQuoteId.isNotEmpty) {
          await _loadDetail(initialQuoteId);
        }
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_quoteErrorMessage(e,
                fallback: 'Angebote konnten nicht geladen werden'))),
      );
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _loadDetail(String id) async {
    try {
      final detail = await widget.api.getQuote(id);
      if (mounted) setState(() => _selected = detail);
      await _loadLinkedSalesOrder(detail['linked_sales_order_id']?.toString());
      await _loadSalesOrderInvoices(
          detail['linked_sales_order_id']?.toString());
    } catch (_) {}
  }

  Future<void> _loadLinkedSalesOrder(String? salesOrderId) async {
    final normalized = salesOrderId?.trim() ?? '';
    if (normalized.isEmpty) {
      if (mounted) setState(() => _linkedSalesOrder = null);
      return;
    }
    try {
      final salesOrder = await widget.api.getSalesOrder(normalized);
      if (mounted) setState(() => _linkedSalesOrder = salesOrder);
    } catch (_) {
      if (mounted) setState(() => _linkedSalesOrder = null);
    }
  }

  Future<void> _loadSalesOrderInvoices(String? salesOrderId) async {
    final normalized = salesOrderId?.trim() ?? '';
    if (normalized.isEmpty) {
      if (mounted) setState(() => _salesOrderInvoices = const []);
      return;
    }
    try {
      final invoices = await widget.api.listInvoicesOut(
        sourceSalesOrderId: normalized,
        limit: 20,
      );
      if (mounted) setState(() => _salesOrderInvoices = invoices);
    } catch (_) {
      if (mounted) setState(() => _salesOrderInvoices = const []);
    }
  }

  double _toDouble(dynamic value) {
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '') ?? 0;
  }

  String _formatMoney(num? value, String currency) {
    final normalizedCurrency = currency.isEmpty ? 'EUR' : currency;
    return '${(value ?? 0).toDouble().toStringAsFixed(2)} $normalizedCurrency';
  }

  String _quoteFollowUpSummary(Map<String, dynamic> item) {
    final linkedSalesOrderId = (item['linked_sales_order_id'] ?? '').toString();
    final linkedInvoiceId = (item['linked_invoice_out_id'] ?? '').toString();
    if (linkedSalesOrderId.isNotEmpty && linkedInvoiceId.isNotEmpty) {
      return 'Auftrag $linkedSalesOrderId  •  Rechnung $linkedInvoiceId';
    }
    if (linkedSalesOrderId.isNotEmpty) {
      return 'In Auftrag $linkedSalesOrderId überführt';
    }
    if (linkedInvoiceId.isNotEmpty) {
      return 'Direkt in Rechnung $linkedInvoiceId überführt';
    }
    return 'Noch kein Folgebeleg';
  }

  Future<void> _openCreateDialog({String? projectId}) async {
    final created = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) =>
          _QuoteEditorDialog(api: widget.api, initialProjectId: projectId),
    );
    if (created == null || !mounted) return;
    setState(() => _selected = created);
    await _load();
  }

  Future<void> _openEditDialog() async {
    final selected = _selected;
    if (selected == null) return;
    final updated = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) => _QuoteEditorDialog(api: widget.api, initial: selected),
    );
    if (updated == null || !mounted) return;
    setState(() => _selected = updated);
    await _load();
  }

  Future<void> _updateStatus(String status) async {
    final id = _selected?['id']?.toString();
    if (id == null) return;
    try {
      final updated = await widget.api.updateQuoteStatus(id, status);
      setState(() => _selected = updated);
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_quoteErrorMessage(e,
                fallback: 'Statuswechsel fehlgeschlagen'))),
      );
    }
  }

  Future<void> _downloadPdf() async {
    final selected = _selected;
    final id = selected?['id']?.toString();
    if (id == null) return;
    try {
      final number = (selected?['number'] ?? '').toString().trim();
      await widget.api.downloadQuotePdf(id,
          filename: number.isEmpty ? null : 'Angebot_$number.pdf');
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Angebots-PDF wird heruntergeladen')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_quoteErrorMessage(e,
                fallback: 'PDF-Download fehlgeschlagen'))),
      );
    }
  }

  Future<void> _openInvoice(String invoiceId) async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => InvoicesPage(
          api: widget.api,
          initialInvoiceId: invoiceId,
          showWorkflowHint: true,
        ),
      ),
    );
    if (!mounted) return;
    await _load();
  }

  Future<void> _openSalesOrder(String salesOrderId) async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => SalesOrdersPage(
          api: widget.api,
          initialSalesOrderId: salesOrderId,
        ),
      ),
    );
    if (!mounted) return;
    await _load();
  }

  Future<void> _acceptQuoteFlow() async {
    final selected = _selected;
    final id = selected?['id']?.toString();
    if (id == null) return;
    final projectId = (selected?['project_id'] ?? '').toString();
    final request = await showDialog<_QuoteAcceptRequest>(
      context: context,
      builder: (_) => _QuoteAcceptDialog(
        allowProjectUpdate:
            projectId.isNotEmpty && widget.api.hasPermission('projects.write'),
      ),
    );
    if (request == null) return;
    try {
      final result = await widget.api.acceptQuote(
        id,
        projectStatus: request.projectStatus,
      );
      final updatedQuote =
          ((result['quote'] as Map?) ?? const {}).cast<String, dynamic>();
      final project =
          ((result['project'] as Map?) ?? const {}).cast<String, dynamic>();
      if (!mounted) return;
      setState(() => _selected = updatedQuote);
      await _load();
      if (!mounted) return;
      final projectStatus = (project['status'] ?? '').toString();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            projectStatus.isEmpty
                ? 'Angebot wurde angenommen'
                : 'Angebot wurde angenommen und Projekt auf $projectStatus gesetzt',
          ),
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(
                _quoteErrorMessage(e, fallback: 'Annahme fehlgeschlagen'))),
      );
    }
  }

  Future<void> _convertToInvoice() async {
    final selected = _selected;
    final id = selected?['id']?.toString();
    if (id == null) return;
    final request = await showDialog<_QuoteConvertRequest>(
      context: context,
      builder: (_) => const _QuoteConvertDialog(),
    );
    if (request == null) return;
    try {
      final result = await widget.api.convertQuoteToInvoice(
        id,
        revenueAccount: request.revenueAccount,
        invoiceDate: DateTime.now(),
        dueDate: request.dueDate,
      );
      final updatedQuote =
          ((result['quote'] as Map?) ?? const {}).cast<String, dynamic>();
      final invoice =
          ((result['invoice'] as Map?) ?? const {}).cast<String, dynamic>();
      final invoiceId = (invoice['id'] ?? '').toString();
      if (!mounted) return;
      setState(() => _selected = updatedQuote);
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            invoiceId.isEmpty
                ? 'Rechnung wurde aus dem Angebot erzeugt'
                : 'Rechnung $invoiceId wurde aus dem Angebot erzeugt',
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
            content: Text(_quoteErrorMessage(e,
                fallback: 'Rechnungserzeugung fehlgeschlagen'))),
      );
    }
  }

  Future<void> _convertToSalesOrder() async {
    final selected = _selected;
    final id = selected?['id']?.toString();
    if (id == null) return;
    try {
      final result = await widget.api.convertQuoteToSalesOrder(id);
      final salesOrder = result;
      final salesOrderId = (salesOrder['id'] ?? '').toString();
      if (!mounted) return;
      await _loadDetail(id);
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            salesOrderId.isEmpty
                ? 'Auftrag wurde aus dem Angebot erzeugt'
                : 'Auftrag $salesOrderId wurde aus dem Angebot erzeugt',
          ),
        ),
      );
      if (salesOrderId.isNotEmpty &&
          widget.api.hasPermission('sales_orders.read')) {
        await _openSalesOrder(salesOrderId);
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_quoteErrorMessage(e,
                fallback: 'Auftragserzeugung fehlgeschlagen'))),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final selected = _selected;
    final linkedSalesOrder = _linkedSalesOrder;
    final selectedStatus = (selected?['status'] ?? '').toString();
    final linkedInvoiceId =
        (selected?['linked_invoice_out_id'] ?? '').toString();
    final linkedSalesOrderId =
        (selected?['linked_sales_order_id'] ?? '').toString();
    final salesOrderInvoices = _salesOrderInvoices
        .cast<Map>()
        .map((item) => item.cast<String, dynamic>())
        .toList();
    final salesOrderInvoiceCount = salesOrderInvoices.length;
    final hasLinkedInvoice = linkedInvoiceId.isNotEmpty;
    final hasLinkedSalesOrder = linkedSalesOrderId.isNotEmpty;
    final hasFollowUp = hasLinkedInvoice || hasLinkedSalesOrder;
    final canWrite = widget.api.hasPermission('quotes.write');
    final canConvertInvoices = widget.api.hasPermission('invoices_out.write');
    final canOpenInvoices = widget.api.hasPermission('invoices_out.read');
    final canConvertSalesOrders =
        widget.api.hasPermission('sales_orders.write');
    final canOpenSalesOrders = widget.api.hasPermission('sales_orders.read');
    return Scaffold(
      appBar: AppBar(
        title: const Text('Angebote'),
        actions: [
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh_rounded)),
        ],
      ),
      floatingActionButton: canWrite
          ? FloatingActionButton.extended(
              onPressed: _openCreateDialog,
              icon: const Icon(Icons.add_rounded),
              label: const Text('Angebot'),
            )
          : null,
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
                              items: const [
                                DropdownMenuItem(
                                    value: null, child: Text('Alle')),
                                DropdownMenuItem(
                                    value: 'draft', child: Text('Entwurf')),
                                DropdownMenuItem(
                                    value: 'sent', child: Text('Versendet')),
                                DropdownMenuItem(
                                    value: 'accepted',
                                    child: Text('Angenommen')),
                                DropdownMenuItem(
                                    value: 'rejected',
                                    child: Text('Abgelehnt')),
                              ],
                              onChanged: (value) =>
                                  setState(() => _statusFilter = value),
                            ),
                          ),
                          FilterChip(
                            label: const Text('Mit Folgebeleg'),
                            selected: _followUpOnlyFilter,
                            onSelected: (value) {
                              setState(() => _followUpOnlyFilter = value);
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
                                  child: Text('Noch keine Angebote gefunden.'))
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
                                      title: Text((item['number'] ?? 'Angebot')
                                          .toString()),
                                      subtitle: Text(
                                        '${item['contact_name'] ?? '-'}  •  ${(item['status'] ?? '').toString()}  •  ${_formatMoney(item['gross_amount'] as num?, (item['currency'] ?? 'EUR').toString())}\n${_quoteFollowUpSummary(item)}',
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
                    ? const Center(child: Text('Angebot auswählen'))
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
                                      (selected['number'] ?? 'Angebot')
                                          .toString(),
                                      style: Theme.of(context)
                                          .textTheme
                                          .headlineSmall,
                                    ),
                                  ),
                                  if (canWrite && selectedStatus == 'draft')
                                    OutlinedButton.icon(
                                      onPressed: _openEditDialog,
                                      icon: const Icon(Icons.edit_rounded),
                                      label: const Text('Bearbeiten'),
                                    ),
                                  const SizedBox(width: 8),
                                  OutlinedButton.icon(
                                    onPressed: _downloadPdf,
                                    icon: const Icon(
                                        Icons.picture_as_pdf_rounded),
                                    label: const Text('PDF'),
                                  ),
                                ],
                              ),
                              const SizedBox(height: 12),
                              Wrap(
                                spacing: 8,
                                runSpacing: 8,
                                children: [
                                  Chip(label: Text('Status: $selectedStatus')),
                                  Chip(
                                      label: Text(
                                          'Kunde: ${(selected['contact_name'] ?? '-').toString()}')),
                                  Chip(
                                      label: Text(
                                          'Projekt: ${(selected['project_name'] ?? '-').toString()}')),
                                  if (hasLinkedInvoice)
                                    Chip(
                                        label:
                                            Text('Rechnung: $linkedInvoiceId')),
                                  if (hasLinkedSalesOrder)
                                    Chip(
                                      label: Text(
                                        linkedSalesOrder == null
                                            ? 'Auftrag: $linkedSalesOrderId'
                                            : 'Auftrag: ${(linkedSalesOrder['number'] ?? linkedSalesOrderId).toString()}',
                                      ),
                                    ),
                                  if (salesOrderInvoiceCount > 0)
                                    Chip(
                                        label: Text(
                                            'Folgebelege: $salesOrderInvoiceCount')),
                                ],
                              ),
                              const SizedBox(height: 12),
                              Text(
                                  'Hinweis: ${(selected['note'] ?? '').toString()}'),
                              const SizedBox(height: 4),
                              Text(
                                  'Quote-Date: ${(selected['quote_date'] ?? '').toString()}'),
                              Text(
                                  'Gueltig bis: ${(selected['valid_until'] ?? '-').toString()}'),
                              if ((selected['accepted_at'] ?? '')
                                  .toString()
                                  .isNotEmpty)
                                Text(
                                    'Angenommen am: ${(selected['accepted_at'] ?? '').toString()}'),
                              if (hasLinkedSalesOrder) ...[
                                const SizedBox(height: 16),
                                Card(
                                  margin: EdgeInsets.zero,
                                  child: Padding(
                                    padding: const EdgeInsets.all(12),
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          linkedSalesOrder == null
                                              ? 'Verknüpfter Auftrag wird geladen'
                                              : 'Verknüpfter Auftrag ${(linkedSalesOrder['number'] ?? linkedSalesOrderId).toString()}',
                                          style: const TextStyle(
                                              fontWeight: FontWeight.bold),
                                        ),
                                        const SizedBox(height: 6),
                                        if (linkedSalesOrder != null) ...[
                                          Text(
                                              'Status: ${(linkedSalesOrder['status'] ?? '-').toString()}'),
                                          Text(
                                              'Auftragswert: ${_formatMoney(linkedSalesOrder['gross_amount'] as num?, (linkedSalesOrder['currency'] ?? 'EUR').toString())}'),
                                          Text(
                                              'Positionen: ${((linkedSalesOrder['items'] as List?) ?? const []).length}'),
                                          if (salesOrderInvoiceCount > 0) ...[
                                            const SizedBox(height: 8),
                                            Text(
                                              'Rechnungen aus Auftrag ($salesOrderInvoiceCount)',
                                              style: const TextStyle(
                                                  fontWeight: FontWeight.w600),
                                            ),
                                            const SizedBox(height: 6),
                                            ...salesOrderInvoices
                                                .asMap()
                                                .entries
                                                .map((entry) {
                                              final invoice = entry.value;
                                              final invoiceId =
                                                  (invoice['id'] ?? '')
                                                      .toString();
                                              final invoiceNumber =
                                                  (invoice['number'] ??
                                                          invoice['nummer'] ??
                                                          invoiceId)
                                                      .toString();
                                              final isLatest =
                                                  invoiceId.isNotEmpty &&
                                                      (invoiceId ==
                                                              linkedInvoiceId ||
                                                          (linkedInvoiceId
                                                                  .isEmpty &&
                                                              entry.key == 0));
                                              final invoiceCurrency =
                                                  (invoice['currency'] ??
                                                          linkedSalesOrder[
                                                              'currency'] ??
                                                          'EUR')
                                                      .toString();
                                              final invoiceGross = _toDouble(
                                                  invoice['gross_amount']);
                                              final invoiceOpen = invoiceGross -
                                                  _toDouble(
                                                      invoice['paid_amount']);
                                              return ListTile(
                                                dense: true,
                                                contentPadding: EdgeInsets.zero,
                                                title: Text(invoiceNumber),
                                                subtitle: Text(
                                                  '${isLatest ? 'Letzte Rechnung' : 'Weitere Rechnung'}  •  ${_formatMoney(invoiceGross, invoiceCurrency)}  •  Offen ${_formatMoney(invoiceOpen, invoiceCurrency)}',
                                                ),
                                                trailing: canOpenInvoices
                                                    ? TextButton(
                                                        onPressed: () =>
                                                            _openInvoice(
                                                                invoiceId),
                                                        child: const Text(
                                                            'Öffnen'),
                                                      )
                                                    : null,
                                              );
                                            }),
                                          ],
                                        ] else
                                          const Text(
                                              'Status und Wert werden nachgeladen.'),
                                      ],
                                    ),
                                  ),
                                ),
                              ],
                              const SizedBox(height: 16),
                              const Text('Positionen',
                                  style:
                                      TextStyle(fontWeight: FontWeight.bold)),
                              const SizedBox(height: 8),
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
                                  final qty = _toDouble(item['qty']);
                                  final unitPrice =
                                      _toDouble(item['unit_price']);
                                  final lineNet = qty * unitPrice;
                                  return ListTile(
                                    title: Text(
                                        (item['description'] ?? 'Position')
                                            .toString()),
                                    subtitle: Text(
                                      'Menge ${qty.toStringAsFixed(qty.truncateToDouble() == qty ? 0 : 2)} ${(item['unit'] ?? '')}  •  Einzelpreis ${_formatMoney(unitPrice, currency)}  •  Steuer ${(item['tax_code'] ?? '').toString()}',
                                    ),
                                    trailing:
                                        Text(_formatMoney(lineNet, currency)),
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
                                        'Netto: ${_formatMoney(_toDouble(selected['net_amount']), (selected['currency'] ?? 'EUR').toString())}'),
                                    Text(
                                        'Steuer: ${_formatMoney(_toDouble(selected['tax_amount']), (selected['currency'] ?? 'EUR').toString())}'),
                                    Text(
                                      'Brutto: ${_formatMoney(_toDouble(selected['gross_amount']), (selected['currency'] ?? 'EUR').toString())}',
                                      style: const TextStyle(
                                          fontWeight: FontWeight.bold),
                                    ),
                                  ],
                                ),
                              ),
                              if (canWrite) ...[
                                const SizedBox(height: 16),
                                Wrap(
                                  spacing: 8,
                                  runSpacing: 8,
                                  children: [
                                    if (!hasFollowUp &&
                                        selectedStatus != 'draft')
                                      OutlinedButton(
                                          onPressed: () =>
                                              _updateStatus('draft'),
                                          child: const Text('Auf Entwurf')),
                                    if (!hasFollowUp &&
                                        selectedStatus != 'sent')
                                      FilledButton(
                                          onPressed: () =>
                                              _updateStatus('sent'),
                                          child: const Text('Versendet')),
                                    if (!hasFollowUp &&
                                        selectedStatus != 'accepted')
                                      FilledButton.tonalIcon(
                                        onPressed: _acceptQuoteFlow,
                                        icon:
                                            const Icon(Icons.task_alt_rounded),
                                        label: const Text('Annahme'),
                                      ),
                                    if (!hasFollowUp &&
                                        selectedStatus != 'rejected')
                                      FilledButton.tonal(
                                          onPressed: () =>
                                              _updateStatus('rejected'),
                                          child: const Text('Abgelehnt')),
                                    if (!hasFollowUp &&
                                        canConvertInvoices &&
                                        (selectedStatus == 'sent' ||
                                            selectedStatus == 'accepted'))
                                      FilledButton.icon(
                                        onPressed: _convertToInvoice,
                                        icon: const Icon(
                                            Icons.receipt_long_rounded),
                                        label: const Text('In Rechnung'),
                                      ),
                                    if (!hasFollowUp &&
                                        canConvertSalesOrders &&
                                        selectedStatus == 'accepted')
                                      FilledButton.icon(
                                        onPressed: _convertToSalesOrder,
                                        icon: const Icon(
                                            Icons.assignment_turned_in_rounded),
                                        label: const Text('In Auftrag'),
                                      ),
                                    if (hasLinkedInvoice && canOpenInvoices)
                                      FilledButton.icon(
                                        onPressed: () =>
                                            _openInvoice(linkedInvoiceId),
                                        icon: const Icon(
                                            Icons.open_in_new_rounded),
                                        label: const Text('Rechnung öffnen'),
                                      ),
                                    if (hasLinkedSalesOrder &&
                                        canOpenSalesOrders)
                                      FilledButton.icon(
                                        onPressed: () =>
                                            _openSalesOrder(linkedSalesOrderId),
                                        icon: const Icon(
                                            Icons.open_in_new_rounded),
                                        label: const Text('Auftrag öffnen'),
                                      ),
                                  ],
                                ),
                                if (hasFollowUp) ...[
                                  const SizedBox(height: 8),
                                  const Text(
                                    'Dieses Angebot hat bereits einen Folgebeleg. Weitere manuelle Statuswechsel sind gesperrt.',
                                  ),
                                ] else if ((selectedStatus == 'sent' ||
                                        selectedStatus == 'accepted') &&
                                    !canConvertInvoices) ...[
                                  const SizedBox(height: 8),
                                  const Text(
                                    'Für die Rechnungsumwandlung ist zusätzlich die Berechtigung invoices_out.write erforderlich.',
                                  ),
                                ] else if (selectedStatus == 'accepted' &&
                                    !canConvertSalesOrders) ...[
                                  const SizedBox(height: 8),
                                  const Text(
                                    'Für die Auftragsumwandlung ist zusätzlich die Berechtigung sales_orders.write erforderlich.',
                                  ),
                                ],
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

class _QuoteConvertRequest {
  const _QuoteConvertRequest({
    required this.revenueAccount,
    this.dueDate,
  });

  final String revenueAccount;
  final DateTime? dueDate;
}

class _QuoteAcceptRequest {
  const _QuoteAcceptRequest({
    this.projectStatus,
  });

  final String? projectStatus;
}

class _QuoteAcceptDialog extends StatefulWidget {
  const _QuoteAcceptDialog({
    required this.allowProjectUpdate,
  });

  final bool allowProjectUpdate;

  @override
  State<_QuoteAcceptDialog> createState() => _QuoteAcceptDialogState();
}

class _QuoteAcceptDialogState extends State<_QuoteAcceptDialog> {
  bool _updateProject = true;
  late final TextEditingController _projectStatusCtrl;

  @override
  void initState() {
    super.initState();
    _updateProject = widget.allowProjectUpdate;
    _projectStatusCtrl = TextEditingController(text: 'beauftragt');
  }

  @override
  void dispose() {
    _projectStatusCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Angebot annehmen'),
      content: SizedBox(
        width: 420,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'Das Angebot wird auf angenommen gesetzt, ohne sofort eine Rechnung zu erzeugen.',
            ),
            if (widget.allowProjectUpdate) ...[
              const SizedBox(height: 16),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                value: _updateProject,
                onChanged: (value) => setState(() => _updateProject = value),
                title: const Text('Projektstatus fortschreiben'),
                subtitle: const Text('Zum Beispiel auf beauftragt setzen'),
              ),
              if (_updateProject)
                TextField(
                  controller: _projectStatusCtrl,
                  decoration:
                      const InputDecoration(labelText: 'Neuer Projektstatus'),
                ),
            ],
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
              _QuoteAcceptRequest(
                projectStatus: widget.allowProjectUpdate && _updateProject
                    ? _projectStatusCtrl.text.trim()
                    : null,
              ),
            );
          },
          child: const Text('Annehmen'),
        ),
      ],
    );
  }
}

class _QuoteConvertDialog extends StatefulWidget {
  const _QuoteConvertDialog();

  @override
  State<_QuoteConvertDialog> createState() => _QuoteConvertDialogState();
}

class _QuoteConvertDialogState extends State<_QuoteConvertDialog> {
  late final TextEditingController _revenueAccountCtrl;
  DateTime? _dueDate;

  @override
  void initState() {
    super.initState();
    _revenueAccountCtrl = TextEditingController(text: '8000');
  }

  @override
  void dispose() {
    _revenueAccountCtrl.dispose();
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
      title: const Text('Angebot in Rechnung überführen'),
      content: SizedBox(
        width: 420,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'Es wird eine neue Ausgangsrechnung im Status Entwurf erzeugt und das Angebot auf angenommen gesetzt.',
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
              _QuoteConvertRequest(
                revenueAccount: _revenueAccountCtrl.text.trim().isEmpty
                    ? '8000'
                    : _revenueAccountCtrl.text.trim(),
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

class _QuoteEditorDialog extends StatefulWidget {
  const _QuoteEditorDialog(
      {required this.api, this.initial, this.initialProjectId});

  final ApiClient api;
  final Map<String, dynamic>? initial;
  final String? initialProjectId;

  @override
  State<_QuoteEditorDialog> createState() => _QuoteEditorDialogState();
}

class _QuoteEditorDialogState extends State<_QuoteEditorDialog> {
  late final TextEditingController _projectCtrl;
  late final TextEditingController _contactCtrl;
  late final TextEditingController _currencyCtrl;
  late final TextEditingController _noteCtrl;
  late final List<_QuoteItemDraft> _items;
  bool _saving = false;

  bool get _isEdit => widget.initial != null;

  @override
  void initState() {
    super.initState();
    final initial = widget.initial;
    _projectCtrl = TextEditingController(
      text:
          widget.initialProjectId ?? (initial?['project_id'] ?? '').toString(),
    );
    _contactCtrl =
        TextEditingController(text: (initial?['contact_id'] ?? '').toString());
    _currencyCtrl =
        TextEditingController(text: (initial?['currency'] ?? 'EUR').toString());
    _noteCtrl =
        TextEditingController(text: (initial?['note'] ?? '').toString());
    final rawItems = (initial?['items'] as List?) ?? const [];
    _items = rawItems.isEmpty
        ? [_QuoteItemDraft()]
        : rawItems
            .map((e) =>
                _QuoteItemDraft.fromJson((e as Map).cast<String, dynamic>()))
            .toList();
  }

  @override
  void dispose() {
    _projectCtrl.dispose();
    _contactCtrl.dispose();
    _currencyCtrl.dispose();
    _noteCtrl.dispose();
    for (final item in _items) {
      item.dispose();
    }
    super.dispose();
  }

  Future<void> _submit() async {
    setState(() => _saving = true);
    try {
      final body = {
        'project_id': _projectCtrl.text.trim(),
        'contact_id': _contactCtrl.text.trim(),
        'currency': _currencyCtrl.text.trim().isEmpty
            ? 'EUR'
            : _currencyCtrl.text.trim().toUpperCase(),
        'note': _noteCtrl.text.trim(),
        'items': _items.map((item) => item.toJson()).toList(),
      };
      final result = _isEdit
          ? await widget.api.updateQuote(widget.initial!['id'].toString(), body)
          : await widget.api.createQuote(body);
      if (!mounted) return;
      Navigator.of(context).pop(result);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_quoteErrorMessage(e,
                fallback: 'Angebot konnte nicht gespeichert werden'))),
      );
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(_isEdit ? 'Angebot bearbeiten' : 'Angebot anlegen'),
      content: SizedBox(
        width: 760,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                children: [
                  Expanded(
                      child: TextField(
                          controller: _projectCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Projekt-ID'))),
                  const SizedBox(width: 12),
                  Expanded(
                      child: TextField(
                          controller: _contactCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Kontakt-ID'))),
                  const SizedBox(width: 12),
                  SizedBox(
                      width: 120,
                      child: TextField(
                          controller: _currencyCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Währung'))),
                ],
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _noteCtrl,
                decoration: const InputDecoration(labelText: 'Hinweis'),
                maxLines: 2,
              ),
              const SizedBox(height: 16),
              Row(
                children: [
                  const Expanded(
                      child: Text('Positionen',
                          style: TextStyle(fontWeight: FontWeight.bold))),
                  TextButton.icon(
                    onPressed: () =>
                        setState(() => _items.add(_QuoteItemDraft())),
                    icon: const Icon(Icons.add_rounded),
                    label: const Text('Position'),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              for (var i = 0; i < _items.length; i++) ...[
                _QuoteItemRow(
                  key: ValueKey(_items[i]),
                  item: _items[i],
                  index: i,
                  onRemove: _items.length == 1
                      ? null
                      : () {
                          setState(() {
                            _items.removeAt(i).dispose();
                          });
                        },
                ),
                const SizedBox(height: 8),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
            onPressed: _saving ? null : () => Navigator.of(context).pop(),
            child: const Text('Abbrechen')),
        FilledButton(
          onPressed: _saving ? null : _submit,
          child: _saving
              ? const SizedBox(
                  height: 18,
                  width: 18,
                  child: CircularProgressIndicator(strokeWidth: 2))
              : Text(_isEdit ? 'Speichern' : 'Anlegen'),
        ),
      ],
    );
  }
}

class _QuoteItemDraft {
  _QuoteItemDraft({
    String description = '',
    String qty = '1',
    String unit = 'Stk',
    String unitPrice = '0',
    String taxCode = 'DE19',
  })  : descriptionCtrl = TextEditingController(text: description),
        qtyCtrl = TextEditingController(text: qty),
        unitCtrl = TextEditingController(text: unit),
        unitPriceCtrl = TextEditingController(text: unitPrice),
        taxCodeCtrl = TextEditingController(text: taxCode);

  factory _QuoteItemDraft.fromJson(Map<String, dynamic> json) {
    return _QuoteItemDraft(
      description: (json['description'] ?? '').toString(),
      qty: (json['qty'] ?? 1).toString(),
      unit: (json['unit'] ?? 'Stk').toString(),
      unitPrice: (json['unit_price'] ?? 0).toString(),
      taxCode: (json['tax_code'] ?? 'DE19').toString(),
    );
  }

  final TextEditingController descriptionCtrl;
  final TextEditingController qtyCtrl;
  final TextEditingController unitCtrl;
  final TextEditingController unitPriceCtrl;
  final TextEditingController taxCodeCtrl;

  Map<String, dynamic> toJson() => {
        'description': descriptionCtrl.text.trim(),
        'qty': double.tryParse(qtyCtrl.text.trim()) ?? 0,
        'unit': unitCtrl.text.trim().isEmpty ? 'Stk' : unitCtrl.text.trim(),
        'unit_price': double.tryParse(unitPriceCtrl.text.trim()) ?? 0,
        'tax_code':
            taxCodeCtrl.text.trim().isEmpty ? 'DE19' : taxCodeCtrl.text.trim(),
      };

  void dispose() {
    descriptionCtrl.dispose();
    qtyCtrl.dispose();
    unitCtrl.dispose();
    unitPriceCtrl.dispose();
    taxCodeCtrl.dispose();
  }
}

class _QuoteItemRow extends StatelessWidget {
  const _QuoteItemRow(
      {super.key, required this.item, required this.index, this.onRemove});

  final _QuoteItemDraft item;
  final int index;
  final VoidCallback? onRemove;

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          children: [
            Row(
              children: [
                Expanded(
                  child: Text('Position ${index + 1}',
                      style: const TextStyle(fontWeight: FontWeight.bold)),
                ),
                if (onRemove != null)
                  IconButton(
                      onPressed: onRemove,
                      icon: const Icon(Icons.delete_outline_rounded)),
              ],
            ),
            TextField(
                controller: item.descriptionCtrl,
                decoration: const InputDecoration(labelText: 'Beschreibung')),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                    child: TextField(
                        controller: item.qtyCtrl,
                        decoration: const InputDecoration(labelText: 'Menge'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: item.unitCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Einheit'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: item.unitPriceCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Einzelpreis'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: item.taxCodeCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Steuercode'))),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
