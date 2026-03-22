import 'dart:ui';
import 'package:flutter/material.dart';
import '../api.dart';
import 'quotes_page.dart';
import 'sales_orders_page.dart';

class InvoicesPage extends StatefulWidget {
  const InvoicesPage({
    super.key,
    required this.api,
    this.initialInvoiceId,
    this.showWorkflowHint = false,
  });
  final ApiClient api;
  final String? initialInvoiceId;
  final bool showWorkflowHint;

  @override
  State<InvoicesPage> createState() => _InvoicesPageState();
}

class _InvoicesPageState extends State<InvoicesPage> {
  List<dynamic> items = [];
  Map<String, dynamic>? selected;
  Map<String, dynamic>? _sourceSalesOrder;
  List<dynamic> _salesOrderInvoices = const [];
  bool loading = false;
  int limit = 50;
  int offset = 0;
  String? statusFilter;
  bool _salesOrderOnlyFilter = false;
  final searchCtrl = TextEditingController();
  final contactCtrl = TextEditingController();
  bool _initialSelectionHandled = false;
  bool _workflowHintDismissed = false;

  @override
  void initState() {
    super.initState();
    final initialInvoiceId = widget.initialInvoiceId?.trim();
    if (initialInvoiceId != null && initialInvoiceId.isNotEmpty) {
      searchCtrl.text = initialInvoiceId;
    }
    _loadList();
  }

  void _resetAndLoad() {
    setState(() => offset = 0);
    _loadList();
  }

  void _changePage(int delta) {
    final nextOffset = offset + delta * limit;
    if (nextOffset < 0) return;
    setState(() => offset = nextOffset);
    _loadList();
  }

  Future<void> _loadList() async {
    setState(() => loading = true);
    try {
      final list = await widget.api.listInvoicesOut(
        limit: limit,
        offset: offset,
        status: statusFilter,
        contactId:
            contactCtrl.text.trim().isEmpty ? null : contactCtrl.text.trim(),
        q: searchCtrl.text.trim().isEmpty ? null : searchCtrl.text.trim(),
      );
      final filteredList = _salesOrderOnlyFilter
          ? list.where((entry) {
              final item = (entry as Map).cast<String, dynamic>();
              return (item['source_sales_order_id'] ?? '')
                  .toString()
                  .trim()
                  .isNotEmpty;
            }).toList()
          : list;
      setState(() => items = filteredList);
      if (selected != null) {
        _loadDetail(selected!['id'] as String);
      } else if (!_initialSelectionHandled) {
        final initialInvoiceId = widget.initialInvoiceId?.trim();
        if (initialInvoiceId != null && initialInvoiceId.isNotEmpty) {
          _initialSelectionHandled = true;
          _loadDetail(initialInvoiceId);
        }
      }
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> _loadDetail(String id) async {
    try {
      final inv = await widget.api.getInvoiceOut(id);
      if (mounted) setState(() => selected = inv);
      await _loadSourceSalesOrder(inv['source_sales_order_id']?.toString());
      await _loadSalesOrderInvoices(inv['source_sales_order_id']?.toString());
    } catch (_) {}
  }

  Future<void> _loadSourceSalesOrder(String? salesOrderId) async {
    final normalized = salesOrderId?.trim() ?? '';
    if (normalized.isEmpty) {
      if (mounted) setState(() => _sourceSalesOrder = null);
      return;
    }
    try {
      final salesOrder = await widget.api.getSalesOrder(normalized);
      if (mounted) setState(() => _sourceSalesOrder = salesOrder);
    } catch (_) {
      if (mounted) setState(() => _sourceSalesOrder = null);
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

  Future<void> _openQuote(String quoteId) async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => QuotesPage(
          api: widget.api,
          initialQuoteId: quoteId,
        ),
      ),
    );
    if (!mounted) return;
    await _loadList();
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
    await _loadList();
  }

  Future<void> _createInvoiceDialog() async {
    final contactCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    final qtyCtrl = TextEditingController(text: '1');
    final priceCtrl = TextEditingController(text: '0');
    String taxCode = 'DE19';
    String revenueAccount = '8000';
    await showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('AR-Rechnung anlegen'),
        content: SizedBox(
          width: 420,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                    controller: contactCtrl,
                    decoration: const InputDecoration(labelText: 'Contact ID')),
                TextField(
                    controller: descCtrl,
                    decoration:
                        const InputDecoration(labelText: 'Positionstext')),
                Row(children: [
                  Expanded(
                      child: TextField(
                          controller: qtyCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Menge'))),
                  const SizedBox(width: 8),
                  Expanded(
                      child: TextField(
                          controller: priceCtrl,
                          decoration:
                              const InputDecoration(labelText: 'Preis'))),
                ]),
                DropdownButtonFormField<String>(
                  initialValue: taxCode,
                  decoration: const InputDecoration(labelText: 'Steuer'),
                  items: const [
                    DropdownMenuItem(value: 'DE19', child: Text('19 %')),
                    DropdownMenuItem(value: 'DE7', child: Text('7 %')),
                    DropdownMenuItem(value: 'DE0', child: Text('0 %')),
                  ],
                  onChanged: (v) => taxCode = v ?? 'DE19',
                ),
                TextField(
                    controller: TextEditingController(text: revenueAccount),
                    decoration: const InputDecoration(labelText: 'Erlöskonto'),
                    onChanged: (v) => revenueAccount = v),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Abbrechen')),
          FilledButton(
              onPressed: () async {
                try {
                  final qty = double.tryParse(qtyCtrl.text.trim()) ?? 0;
                  final price = double.tryParse(priceCtrl.text.trim()) ?? 0;
                  final body = {
                    'contact_id': contactCtrl.text.trim(),
                    'currency': 'EUR',
                    'items': [
                      {
                        'description': descCtrl.text.trim().isEmpty
                            ? 'Position'
                            : descCtrl.text.trim(),
                        'qty': qty,
                        'unit_price': price,
                        'tax_code': taxCode,
                        'account_code': revenueAccount,
                      }
                    ],
                  };
                  final inv = await widget.api.createInvoiceOut(body);
                  if (mounted) {
                    Navigator.of(ctx).pop();
                    setState(() {
                      selected = inv;
                      items.insert(0, inv);
                    });
                  }
                } catch (e) {
                  if (mounted)
                    ScaffoldMessenger.of(context)
                        .showSnackBar(SnackBar(content: Text('Fehler: $e')));
                }
              },
              child: const Text('Anlegen')),
        ],
      ),
    );
  }

  Future<void> _bookSelected() async {
    final id = selected?['id']?.toString();
    if (id == null) return;
    try {
      final inv = await widget.api.bookInvoiceOut(id);
      setState(() => selected = inv);
      _loadList();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Rechnung wurde gebucht')),
      );
    } catch (e) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Buchen fehlgeschlagen: $e')));
    }
  }

  Future<void> _addPayment() async {
    final id = selected?['id']?.toString();
    if (id == null) return;
    final amountCtrl = TextEditingController(
        text: selected?['gross_amount']?.toString() ?? '0');
    final refCtrl = TextEditingController();
    await showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Zahlung verbuchen'),
        content: SizedBox(
          width: 360,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                  controller: amountCtrl,
                  decoration: const InputDecoration(labelText: 'Betrag')),
              TextField(
                  controller: refCtrl,
                  decoration: const InputDecoration(labelText: 'Referenz')),
            ],
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Abbrechen')),
          FilledButton(
              onPressed: () async {
                try {
                  final amt = double.tryParse(amountCtrl.text.trim()) ?? 0;
                  await widget.api.addInvoicePayment(id, {
                    'amount': amt,
                    'currency': selected?['currency'] ?? 'EUR',
                    'method': 'bank',
                    'reference': refCtrl.text.trim(),
                    'date': DateTime.now().toIso8601String(),
                  });
                  if (mounted) Navigator.of(ctx).pop();
                  await _loadDetail(id);
                } catch (e) {
                  if (mounted)
                    ScaffoldMessenger.of(context)
                        .showSnackBar(SnackBar(content: Text('Fehler: $e')));
                }
              },
              child: const Text('Buchen')),
        ],
      ),
    );
  }

  Widget _statusChip(String status) {
    Color c = Colors.blue;
    if (status == 'booked') c = Colors.orange;
    if (status == 'paid') c = Colors.green;
    return Chip(
        label: Text(status),
        backgroundColor: c.withValues(alpha: 0.15),
        labelStyle: TextStyle(color: c));
  }

  String _invoiceSourceHeadline(Map<String, dynamic> invoice) {
    if ((invoice['source_sales_order_id'] ?? '').toString().isNotEmpty) {
      return 'Rechnung aus Auftrag erzeugt';
    }
    if ((invoice['source_quote_id'] ?? '').toString().isNotEmpty) {
      return 'Rechnung aus Angebot erzeugt';
    }
    return 'Rechnung';
  }

  String _invoiceSourceSummary(Map<String, dynamic> invoice) {
    final salesOrderId = (invoice['source_sales_order_id'] ?? '').toString();
    if (salesOrderId.isNotEmpty) {
      return 'Folgebeleg aus Auftrag $salesOrderId';
    }
    final quoteId = (invoice['source_quote_id'] ?? '').toString();
    if (quoteId.isNotEmpty) {
      return 'Folgebeleg aus Angebot $quoteId';
    }
    return 'Keine direkte Belegquelle';
  }

  double _toDouble(dynamic value) {
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '') ?? 0;
  }

  String _formatMoney(num? value, String currency) {
    final normalizedCurrency = currency.isEmpty ? 'EUR' : currency;
    return '${(value ?? 0).toDouble().toStringAsFixed(2)} $normalizedCurrency';
  }

  Widget _glassPanel(Widget child,
      {EdgeInsets padding = const EdgeInsets.all(12)}) {
    final scheme = Theme.of(context).colorScheme;
    return ClipRRect(
      borderRadius: BorderRadius.circular(18),
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 14, sigmaY: 14),
        child: Container(
          padding: padding,
          decoration: BoxDecoration(
            gradient: LinearGradient(
              colors: [
                Colors.white.withValues(alpha: 0.12),
                Colors.white.withValues(alpha: 0.04)
              ],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            borderRadius: BorderRadius.circular(18),
            border: Border.all(color: Colors.white.withValues(alpha: 0.14)),
            boxShadow: [
              BoxShadow(
                  color: Colors.black.withValues(alpha: 0.25),
                  blurRadius: 24,
                  offset: const Offset(0, 16)),
              BoxShadow(
                  color: scheme.primary.withValues(alpha: 0.16),
                  blurRadius: 28,
                  offset: const Offset(-10, -8)),
            ],
          ),
          child: child,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final sel = selected;
    final sourceSalesOrder = _sourceSalesOrder;
    final salesOrderInvoices = _salesOrderInvoices
        .cast<Map>()
        .map((item) => item.cast<String, dynamic>())
        .toList();
    final salesOrderInvoiceCount = salesOrderInvoices.length;
    final showWorkflowHint = widget.showWorkflowHint &&
        !_workflowHintDismissed &&
        sel != null &&
        (((sel['source_quote_id'] ?? '').toString().isNotEmpty) ||
            ((sel['source_sales_order_id'] ?? '').toString().isNotEmpty));
    return Scaffold(
      appBar: AppBar(
        title: const Text('Ausgangsrechnungen'),
        backgroundColor: Colors.transparent,
      ),
      floatingActionButton: FloatingActionButton(
          onPressed: _createInvoiceDialog, child: const Icon(Icons.add)),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
          child: Row(
            children: [
              Expanded(
                flex: 4,
                child: _glassPanel(
                  Column(
                    children: [
                      Padding(
                        padding: const EdgeInsets.only(bottom: 10),
                        child: Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          crossAxisAlignment: WrapCrossAlignment.center,
                          children: [
                            const Text('Rechnungen',
                                style: TextStyle(
                                    fontSize: 18, fontWeight: FontWeight.bold)),
                            IconButton(
                                onPressed: _resetAndLoad,
                                icon: const Icon(Icons.refresh)),
                          ],
                        ),
                      ),
                      Wrap(
                        spacing: 10,
                        runSpacing: 10,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          SizedBox(
                            width: 260,
                            child: TextField(
                              controller: searchCtrl,
                              decoration: const InputDecoration(
                                  hintText: 'Suche (Nummer/ID)',
                                  prefixIcon: Icon(Icons.search_rounded)),
                              onSubmitted: (_) => _resetAndLoad(),
                            ),
                          ),
                          SizedBox(
                            width: 170,
                            child: TextField(
                              controller: contactCtrl,
                              decoration:
                                  const InputDecoration(hintText: 'Kontakt-ID'),
                              onSubmitted: (_) => _resetAndLoad(),
                            ),
                          ),
                          DropdownButton<String?>(
                            value: statusFilter,
                            hint: const Text('Status'),
                            items: const [
                              DropdownMenuItem(
                                  value: null, child: Text('Alle')),
                              DropdownMenuItem(
                                  value: 'draft', child: Text('Draft')),
                              DropdownMenuItem(
                                  value: 'booked', child: Text('Gebucht')),
                              DropdownMenuItem(
                                  value: 'partial', child: Text('Teilgezahlt')),
                              DropdownMenuItem(
                                  value: 'paid', child: Text('Bezahlt')),
                            ],
                            onChanged: (v) {
                              setState(() => statusFilter = v);
                              _resetAndLoad();
                            },
                          ),
                          FilterChip(
                            label: const Text('Auftragsbezug'),
                            selected: _salesOrderOnlyFilter,
                            onSelected: (value) {
                              setState(() => _salesOrderOnlyFilter = value);
                              _resetAndLoad();
                            },
                          ),
                        ],
                      ),
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 10),
                        child: Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          crossAxisAlignment: WrapCrossAlignment.center,
                          children: [
                            Text(
                                'Zeile ${offset + 1} - ${offset + items.length} (Limit $limit)'),
                            IconButton(
                                tooltip: 'Zurück',
                                onPressed:
                                    offset == 0 ? null : () => _changePage(-1),
                                icon: const Icon(Icons.chevron_left)),
                            IconButton(
                                tooltip: 'Weiter',
                                onPressed: items.length < limit
                                    ? null
                                    : () => _changePage(1),
                                icon: const Icon(Icons.chevron_right)),
                          ],
                        ),
                      ),
                      if (loading) const LinearProgressIndicator(minHeight: 2),
                      Expanded(
                        child: ListView.separated(
                          itemCount: items.length,
                          separatorBuilder: (_, __) =>
                              const Divider(height: 1, color: Colors.white24),
                          itemBuilder: (ctx, i) {
                            final it = items[i] as Map<String, dynamic>;
                            final id = it['id']?.toString() ?? '';
                            final invoiceNumber = it['nummer']?.toString() ??
                                it['number']?.toString() ??
                                '';
                            final gross = it['gross_amount'] ?? 0;
                            final paid = it['paid_amount'] ?? 0;
                            final open = (gross - paid);
                            final cname =
                                (it['contact_name'] ?? it['contact_id'] ?? '')
                                    .toString();
                            return ListTile(
                              selected: sel != null && sel['id'] == id,
                              title: Text(
                                  invoiceNumber.isEmpty ? id : invoiceNumber),
                              subtitle: Text(
                                  '$cname  ·  Status: ${it['status']}  ·  Brutto: ${_formatMoney(gross as num?, (it['currency'] ?? 'EUR').toString())}  ·  Offen: ${_formatMoney(open, (it['currency'] ?? 'EUR').toString())}\n${_invoiceSourceSummary(it)}'),
                              isThreeLine: true,
                              trailing:
                                  _statusChip((it['status'] ?? '').toString()),
                              onTap: () {
                                setState(() => selected = it);
                                _loadDetail(id);
                              },
                            );
                          },
                        ),
                      ),
                    ],
                  ),
                  padding: const EdgeInsets.all(14),
                ),
              ),
              const SizedBox(width: 16),
              Expanded(
                flex: 7,
                child: _glassPanel(
                  sel == null
                      ? const Center(child: Text('Bitte Rechnung auswaehlen'))
                      : Padding(
                          padding: const EdgeInsets.all(6),
                          child: SingleChildScrollView(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                if (showWorkflowHint) ...[
                                  _WorkflowHintCard(
                                    title: _invoiceSourceHeadline(sel),
                                    isDraft: (sel['status'] ?? '') == 'draft',
                                    canWriteInvoices: widget.api
                                        .hasPermission('invoices_out.write'),
                                    onBook: (sel['status'] ?? '') == 'draft'
                                        ? _bookSelected
                                        : null,
                                    onDismiss: () => setState(
                                        () => _workflowHintDismissed = true),
                                    onAddPayment: widget.api
                                            .hasPermission('invoices_out.write')
                                        ? _addPayment
                                        : null,
                                  ),
                                  const SizedBox(height: 12),
                                ],
                                Wrap(
                                  spacing: 8,
                                  runSpacing: 8,
                                  crossAxisAlignment: WrapCrossAlignment.center,
                                  children: [
                                    Text(
                                        sel['nummer']?.toString() ??
                                            sel['number']?.toString() ??
                                            sel['id'],
                                        style: const TextStyle(
                                            fontSize: 20,
                                            fontWeight: FontWeight.bold)),
                                    _statusChip(
                                        (sel['status'] ?? '').toString()),
                                    if ((sel['status'] ?? '') == 'draft')
                                      FilledButton.icon(
                                          onPressed: _bookSelected,
                                          icon: const Icon(Icons.check),
                                          label: const Text('Buchen')),
                                    IconButton(
                                        onPressed: () => widget.api
                                            .downloadInvoiceOutPdf(
                                                sel['id'].toString()),
                                        icon: const Icon(Icons.picture_as_pdf),
                                        tooltip: 'PDF'),
                                    FilledButton.icon(
                                        onPressed: _addPayment,
                                        icon: const Icon(Icons.payments),
                                        label: const Text('Zahlung')),
                                  ],
                                ),
                                const SizedBox(height: 8),
                                Text(
                                    'Kontakt: ${sel['contact_name'] ?? ''} (${sel['contact_id'] ?? ''})'),
                                Wrap(
                                  spacing: 8,
                                  runSpacing: 8,
                                  children: [
                                    if ((sel['source_quote_id'] ?? '')
                                        .toString()
                                        .isNotEmpty)
                                      ActionChip(
                                        avatar: const Icon(
                                            Icons.request_quote_rounded,
                                            size: 18),
                                        label: Text(
                                            'Angebot ${(sel['source_quote_id'] ?? '').toString()}'),
                                        onPressed: () => _openQuote(
                                            (sel['source_quote_id'] ?? '')
                                                .toString()),
                                      ),
                                    if ((sel['source_sales_order_id'] ?? '')
                                        .toString()
                                        .isNotEmpty)
                                      ActionChip(
                                        avatar: const Icon(
                                            Icons.assignment_turned_in_rounded,
                                            size: 18),
                                        label: Text(
                                          sourceSalesOrder == null
                                              ? 'Auftrag ${(sel['source_sales_order_id'] ?? '').toString()}'
                                              : 'Auftrag ${(sourceSalesOrder['number'] ?? sel['source_sales_order_id']).toString()}',
                                        ),
                                        onPressed: () => _openSalesOrder(
                                            (sel['source_sales_order_id'] ?? '')
                                                .toString()),
                                      ),
                                  ],
                                ),
                                if ((sel['source_sales_order_id'] ?? '')
                                    .toString()
                                    .isNotEmpty)
                                  Padding(
                                    padding: const EdgeInsets.only(top: 8),
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          sourceSalesOrder == null
                                              ? 'Auftragskontext wird geladen.'
                                              : 'Auftragsstatus: ${(sourceSalesOrder['status'] ?? '-').toString()}  ·  Auftragswert: ${_formatMoney(sourceSalesOrder['gross_amount'] as num?, (sourceSalesOrder['currency'] ?? 'EUR').toString())}',
                                        ),
                                        if (salesOrderInvoiceCount > 0) ...[
                                          const SizedBox(height: 8),
                                          Text(
                                            'Weitere Folgebelege aus Auftrag ($salesOrderInvoiceCount)',
                                            style: const TextStyle(
                                                fontWeight: FontWeight.w600),
                                          ),
                                          const SizedBox(height: 6),
                                          ...salesOrderInvoices.map((invoice) {
                                            final invoiceId =
                                                (invoice['id'] ?? '')
                                                    .toString();
                                            final invoiceNumber =
                                                (invoice['number'] ??
                                                        invoice['nummer'] ??
                                                        invoiceId)
                                                    .toString();
                                            final isCurrent =
                                                invoiceId.isNotEmpty &&
                                                    invoiceId ==
                                                        (sel['id'] ?? '')
                                                            .toString();
                                            final invoiceCurrency =
                                                (invoice['currency'] ??
                                                        sel['currency'] ??
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
                                                '${isCurrent ? 'Aktuelle Rechnung' : 'Weitere Rechnung'}  •  ${_formatMoney(invoiceGross, invoiceCurrency)}  •  Offen ${_formatMoney(invoiceOpen, invoiceCurrency)}',
                                              ),
                                              trailing: isCurrent
                                                  ? const Icon(
                                                      Icons
                                                          .check_circle_outline_rounded,
                                                      size: 18)
                                                  : TextButton(
                                                      onPressed: () =>
                                                          _loadDetail(
                                                              invoiceId),
                                                      child:
                                                          const Text('Öffnen'),
                                                    ),
                                            );
                                          }),
                                        ],
                                      ],
                                    ),
                                  ),
                                Text(
                                    'Datum: ${sel['invoice_date'] ?? ''}  Fällig: ${sel['due_date'] ?? ''}'),
                                const SizedBox(height: 8),
                                Text(
                                    'Brutto: ${_formatMoney(sel['gross_amount'] as num?, (sel['currency'] ?? 'EUR').toString())}  Offen: ${_formatMoney(((sel['gross_amount'] ?? 0) - (sel['paid_amount'] ?? 0)) as num?, (sel['currency'] ?? 'EUR').toString())}'),
                                const SizedBox(height: 12),
                                const Text('Positionen',
                                    style:
                                        TextStyle(fontWeight: FontWeight.bold)),
                                const SizedBox(height: 6),
                                ...List<Widget>.from(
                                    ((sel['items'] ?? []) as List).map((it) {
                                  final m = it as Map<String, dynamic>;
                                  final qty = _toDouble(m['qty']);
                                  final unitPrice = _toDouble(m['unit_price']);
                                  final net = qty * unitPrice;
                                  final currency =
                                      (sel['currency'] ?? 'EUR').toString();
                                  return ListTile(
                                    dense: true,
                                    title: Text(
                                        m['description']?.toString() ?? ''),
                                    subtitle: Text(
                                        'Menge ${qty.toStringAsFixed(qty.truncateToDouble() == qty ? 0 : 2)} x ${_formatMoney(unitPrice, currency)}  ·  Steuer ${m['tax_code'] ?? ''}  ·  Konto ${m['account_code'] ?? ''}'),
                                    trailing: Text(_formatMoney(net, currency)),
                                  );
                                })),
                                const SizedBox(height: 12),
                                const Text('Zahlungen',
                                    style:
                                        TextStyle(fontWeight: FontWeight.bold)),
                                FutureBuilder<List<dynamic>>(
                                  future: widget.api.listInvoicePayments(
                                      sel['id'].toString()),
                                  builder: (context, snap) {
                                    if (snap.connectionState ==
                                        ConnectionState.waiting) {
                                      return const Padding(
                                          padding: EdgeInsets.all(8),
                                          child: LinearProgressIndicator(
                                              minHeight: 2));
                                    }
                                    if (snap.hasError) {
                                      return Padding(
                                          padding: const EdgeInsets.all(8),
                                          child: Text('Fehler: ${snap.error}'));
                                    }
                                    final pays = snap.data ?? [];
                                    if (pays.isEmpty) {
                                      return const Padding(
                                          padding: EdgeInsets.all(8),
                                          child: Text('Keine Zahlungen'));
                                    }
                                    return Column(children: [
                                      ...pays.map((p) {
                                        final m = p as Map<String, dynamic>;
                                        final currency =
                                            (m['currency'] ?? 'EUR').toString();
                                        return ListTile(
                                          dense: true,
                                          title: Text(
                                              'Betrag ${_formatMoney(_toDouble(m['amount']), currency)}'),
                                          subtitle: Text(
                                              '${m['method']}  ·  ${m['reference'] ?? ''}'),
                                        );
                                      })
                                    ]);
                                  },
                                ),
                              ],
                            ),
                          ),
                        ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _WorkflowHintCard extends StatelessWidget {
  const _WorkflowHintCard({
    required this.title,
    required this.isDraft,
    required this.canWriteInvoices,
    required this.onDismiss,
    this.onBook,
    this.onAddPayment,
  });

  final String title;
  final bool isDraft;
  final bool canWriteInvoices;
  final VoidCallback onDismiss;
  final VoidCallback? onBook;
  final VoidCallback? onAddPayment;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Card(
      color: scheme.primaryContainer.withValues(alpha: 0.65),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 10,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Icon(Icons.alt_route_rounded, color: scheme.primary),
                Text(
                  title,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                IconButton(
                  onPressed: onDismiss,
                  icon: const Icon(Icons.close_rounded),
                  tooltip: 'Hinweis schließen',
                ),
              ],
            ),
            Text(
              isDraft
                  ? 'Der Rechnungsentwurf ist geöffnet. Prüfe Positionen und buche den Beleg direkt weiter.'
                  : 'Der Folgebeleg wurde bereits weiterbearbeitet. Von hier aus kannst du PDF und Zahlung direkt fortführen.',
            ),
            if (canWriteInvoices) ...[
              const SizedBox(height: 12),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  if (isDraft && onBook != null)
                    FilledButton.icon(
                      onPressed: onBook,
                      icon: const Icon(Icons.check_circle_outline_rounded),
                      label: const Text('Jetzt buchen'),
                    ),
                  if (onAddPayment != null && !isDraft)
                    FilledButton.tonalIcon(
                      onPressed: onAddPayment,
                      icon: const Icon(Icons.payments_rounded),
                      label: const Text('Zahlung erfassen'),
                    ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}
