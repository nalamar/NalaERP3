import 'package:flutter/material.dart';

import '../api.dart';
import '../commercial_destinations.dart';
import '../commercial_navigation.dart';
import '../web/browser.dart' as browser;

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
    this.initialFilters,
    this.initialQuoteId,
    this.initialSearchQuery,
    this.initialContext,
    this.openCreateOnStart = false,
  });

  final ApiClient api;
  @Deprecated('Use initialFilters instead.')
  final String? initialProjectId;
  final CommercialFilterContext? initialFilters;
  @Deprecated('Use initialContext instead.')
  final String? initialQuoteId;
  @Deprecated('Use initialContext instead.')
  final String? initialSearchQuery;
  final CommercialListContext? initialContext;
  final bool openCreateOnStart;

  @override
  State<QuotesPage> createState() => _QuotesPageState();
}

class _QuotesPageState extends State<QuotesPage> {
  bool _loading = true;
  bool _importsLoading = false;
  List<dynamic> _items = const [];
  List<dynamic> _quoteImports = const [];
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
    final initialFilters = _resolvedInitialFilters();
    if (initialFilters.normalizedProjectId != null) {
      _projectCtrl.text = initialFilters.normalizedProjectId!;
    }
    final initialContext = _resolvedInitialContext();
    final initialSearch = initialContext.effectiveSearchQuery;
    if (initialSearch != null) {
      _searchCtrl.text = initialSearch;
    }
    _load();
    if (widget.openCreateOnStart) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          _openCreateDialog(initialFilters: initialFilters);
        }
      });
    }
  }

  CommercialListContext _resolvedInitialContext() {
    return widget.initialContext ??
        // ignore: deprecated_member_use_from_same_package
        (widget.initialQuoteId?.trim().isNotEmpty ?? false
            // ignore: deprecated_member_use_from_same_package
            ? CommercialListContext.detail(widget.initialQuoteId!.trim())
            // ignore: deprecated_member_use_from_same_package
            : CommercialListContext(searchQuery: widget.initialSearchQuery));
  }

  CommercialFilterContext _resolvedInitialFilters() {
    return widget.initialFilters ??
        // ignore: deprecated_member_use_from_same_package
        CommercialFilterContext(projectId: widget.initialProjectId);
  }

  CommercialFilterContext _currentFilterContext() {
    return CommercialFilterContext(projectId: _projectCtrl.text.trim());
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
        final initialDetailId = _resolvedInitialContext().normalizedDetailId;
        if (initialDetailId != null) {
          await _loadDetail(initialDetailId);
        }
      }
      await _loadQuoteImports();
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

  Future<void> _loadQuoteImports() async {
    if (!widget.api.hasPermission('quotes.read')) return;
    if (mounted) setState(() => _importsLoading = true);
    try {
      final imports = await widget.api.listQuoteImports(
        projectId:
            _projectCtrl.text.trim().isEmpty ? null : _projectCtrl.text.trim(),
        limit: 6,
      );
      if (mounted) setState(() => _quoteImports = imports);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'GAEB-Importe konnten nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) setState(() => _importsLoading = false);
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

  Future<void> _openCreateDialog(
      {CommercialFilterContext? initialFilters}) async {
    final created = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) => _QuoteEditorDialog(
        api: widget.api,
        initialFilters: initialFilters ?? _currentFilterContext(),
      ),
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
        builder: (_) => buildInvoicesPage(
          api: widget.api,
          initialContext: CommercialListContext.detail(invoiceId),
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
        builder: (_) => buildSalesOrdersPage(
          api: widget.api,
          initialFilters: _currentFilterContext(),
          initialContext: CommercialListContext.detail(salesOrderId),
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

  Future<void> _importGAEB() async {
    final projectId = _projectCtrl.text.trim();
    if (projectId.isEmpty) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
              'Für den GAEB-Import bitte zuerst eine Projekt-ID im Filter setzen.'),
        ),
      );
      return;
    }
    final picked = await browser.pickFile(
      accept: '.x83,.x84,.d83,.p83,.gaeb,.xml',
    );
    if (picked == null) return;
    if (!mounted) return;

    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (_) =>
          const _QuoteProgressDialog(text: 'GAEB-Import wird hochgeladen...'),
    );
    try {
      await widget.api.uploadGAEBQuoteImport(
        picked.filename,
        picked.bytes,
        projectId: projectId,
        contentType: picked.contentType,
      );
      if (!mounted) return;
      Navigator.of(context, rootNavigator: true).pop();
      await _loadQuoteImports();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text('GAEB-Datei ${picked.filename} wurde hochgeladen')),
      );
    } catch (e) {
      if (!mounted) return;
      Navigator.of(context, rootNavigator: true).pop();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
              _quoteErrorMessage(e, fallback: 'GAEB-Upload fehlgeschlagen')),
        ),
      );
    }
  }

  Future<void> _openQuoteImportDetail(String importId) async {
    try {
      Map<String, dynamic> detail = await widget.api.getQuoteImport(importId);
      List<dynamic> items = await widget.api.listQuoteImportItems(importId);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => StatefulBuilder(
          builder: (context, setDialogState) {
            Future<void> refreshImportState() async {
              final nextDetail = await widget.api.getQuoteImport(importId);
              final nextItems = await widget.api.listQuoteImportItems(importId);
              if (!mounted) return;
              setDialogState(() {
                detail = nextDetail;
                items = nextItems;
              });
            }

            final visibleItems = items.take(5).cast<Map>().map((item) {
              return item.cast<String, dynamic>();
            }).toList();
            final remainingItems = items.length - visibleItems.length;
            final status = (detail['status'] ?? '').toString();
            final createdQuoteId =
                (detail['created_quote_id'] ?? '').toString().trim();
            final acceptedCount =
                ((detail['accepted_count'] ?? 0) as num?) ?? 0;
            final rejectedCount =
                ((detail['rejected_count'] ?? 0) as num?) ?? 0;
            final pendingCount = ((detail['pending_count'] ?? 0) as num?) ?? 0;
            final canReviewImport =
                widget.api.hasPermission('quotes.write') && status == 'parsed';
            final canApplyImport = widget.api.hasPermission('quotes.write') &&
                status == 'reviewed';

            return AlertDialog(
              title:
                  Text((detail['source_filename'] ?? 'GAEB-Import').toString()),
              content: SizedBox(
                width: 720,
                child: SingleChildScrollView(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Status: ${(detail['status'] ?? '-').toString()}'),
                      Text(
                          'Quelle: ${(detail['source_kind'] ?? '-').toString()}'),
                      Text(
                          'Projekt: ${(detail['project_id'] ?? '-').toString()}'),
                      Text(
                          'Kontakt: ${((detail['contact_id'] ?? '').toString().trim().isEmpty ? '-' : detail['contact_id']).toString()}'),
                      Text(
                          'Format: ${((detail['detected_format'] ?? '').toString().trim().isEmpty ? '-' : detail['detected_format']).toString()}'),
                      Text(
                          'Parser-Version: ${((detail['parser_version'] ?? '').toString().trim().isEmpty ? '-' : detail['parser_version']).toString()}'),
                      Text(
                          'Dokument-ID: ${(detail['source_document_id'] ?? '-').toString()}'),
                      Text(
                          'Positionen: ${(detail['item_count'] ?? items.length).toString()}'),
                      const SizedBox(height: 8),
                      Text(
                        'Review-Summary: $acceptedCount übernommen, '
                        '$rejectedCount abgelehnt, $pendingCount offen',
                      ),
                      Text(
                          'Hochgeladen: ${_formatDateTime(detail['uploaded_at'])}'),
                      Text(
                          'Aktualisiert: ${_formatDateTime(detail['updated_at'])}'),
                      if (createdQuoteId.isNotEmpty)
                        Text('Erzeugte Quote: $createdQuoteId'),
                      if (createdQuoteId.isNotEmpty)
                        const Padding(
                          padding: EdgeInsets.only(top: 4),
                          child: Text(
                            'Die Quote wurde erzeugt und kann jetzt geöffnet werden.',
                          ),
                        ),
                      if ((detail['error_message'] ?? '')
                          .toString()
                          .trim()
                          .isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text(
                            'Fehler: ${(detail["error_message"] ?? "").toString()}',
                            style: TextStyle(
                              color: Theme.of(context).colorScheme.error,
                            ),
                          ),
                        ),
                      const SizedBox(height: 16),
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: [
                          if (canReviewImport)
                            FilledButton.tonalIcon(
                              onPressed: () async {
                                showDialog<void>(
                                  context: dialogContext,
                                  barrierDismissible: false,
                                  builder: (_) => const _QuoteProgressDialog(
                                    text:
                                        'Importlauf wird zur Übernahme freigegeben...',
                                  ),
                                );
                                try {
                                  final updated = await widget.api
                                      .markQuoteImportReviewed(importId);
                                  if (!mounted) return;
                                  Navigator.of(dialogContext,
                                          rootNavigator: true)
                                      .pop();
                                  setDialogState(() => detail = updated);
                                  await refreshImportState();
                                  await _loadQuoteImports();
                                  if (!mounted) return;
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    const SnackBar(
                                      content:
                                          Text('Importlauf wurde freigegeben'),
                                    ),
                                  );
                                } catch (e) {
                                  if (!mounted) return;
                                  Navigator.of(dialogContext,
                                          rootNavigator: true)
                                      .pop();
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    SnackBar(
                                      content: Text(_quoteErrorMessage(e,
                                          fallback:
                                              'Importlauf konnte nicht freigegeben werden')),
                                    ),
                                  );
                                }
                              },
                              icon: const Icon(Icons.verified_rounded),
                              label: const Text('Zur Übernahme freigeben'),
                            ),
                          if (canApplyImport)
                            FilledButton.icon(
                              onPressed: () async {
                                showDialog<void>(
                                  context: dialogContext,
                                  barrierDismissible: false,
                                  builder: (_) => const _QuoteProgressDialog(
                                    text:
                                        'Draft-Quote aus Importlauf wird erzeugt...',
                                  ),
                                );
                                try {
                                  final applied = await widget.api
                                      .applyQuoteImport(importId);
                                  if (!mounted) return;
                                  Navigator.of(dialogContext,
                                          rootNavigator: true)
                                      .pop();
                                  final nextDetail =
                                      ((applied['import'] as Map?) ?? const {})
                                          .cast<String, dynamic>();
                                  setDialogState(() => detail = nextDetail);
                                  await refreshImportState();
                                  await _load();
                                  if (!mounted) return;
                                  final quote =
                                      ((applied['quote'] as Map?) ?? const {})
                                          .cast<String, dynamic>();
                                  final quoteNumber =
                                      (quote['number'] ?? '').toString().trim();
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    SnackBar(
                                      content: Text(
                                        quoteNumber.isEmpty
                                            ? 'Draft-Quote wurde aus dem Importlauf erzeugt'
                                            : 'Draft-Quote $quoteNumber wurde aus dem Importlauf erzeugt',
                                      ),
                                    ),
                                  );
                                } catch (e) {
                                  if (!mounted) return;
                                  Navigator.of(dialogContext,
                                          rootNavigator: true)
                                      .pop();
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    SnackBar(
                                      content: Text(_quoteErrorMessage(e,
                                          fallback:
                                              'Quote konnte aus dem Importlauf nicht erzeugt werden')),
                                    ),
                                  );
                                }
                              },
                              icon: const Icon(Icons.note_add_rounded),
                              label: const Text('Draft-Quote erzeugen'),
                            ),
                          if (createdQuoteId.isNotEmpty &&
                              widget.api.hasPermission('quotes.read'))
                            FilledButton.tonalIcon(
                              onPressed: () async {
                                Navigator.of(dialogContext).pop();
                                try {
                                  setState(() {
                                    _statusFilter = null;
                                    _followUpOnlyFilter = false;
                                  });
                                  final createdQuote =
                                      await widget.api.getQuote(createdQuoteId);
                                  if (!mounted) return;
                                  setState(() => _selected = createdQuote);
                                  await _load();
                                  if (!mounted) return;
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    const SnackBar(
                                      content:
                                          Text('Erzeugte Quote wurde geöffnet'),
                                    ),
                                  );
                                } catch (e) {
                                  if (!mounted) return;
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    SnackBar(
                                      content: Text(_quoteErrorMessage(e,
                                          fallback:
                                              'Erzeugte Quote konnte nicht geöffnet werden')),
                                    ),
                                  );
                                }
                              },
                              icon: const Icon(Icons.open_in_new_rounded),
                              label: const Text('Quote öffnen'),
                            ),
                        ],
                      ),
                      const SizedBox(height: 16),
                      const Text(
                        'Importierte Rohpositionen',
                        style: TextStyle(fontWeight: FontWeight.bold),
                      ),
                      const SizedBox(height: 8),
                      if (visibleItems.isEmpty)
                        const Text('Noch keine geparsten Positionen vorhanden.')
                      else
                        ...visibleItems.map(
                          (item) => ListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text(
                              ((item['position_no'] ?? '')
                                          .toString()
                                          .trim()
                                          .isEmpty
                                      ? 'Ohne Positionsnummer'
                                      : 'Position ${(item['position_no'] ?? '').toString()}')
                                  .toString(),
                            ),
                            subtitle: Text(
                              ((item['description'] ?? '')
                                          .toString()
                                          .trim()
                                          .isEmpty
                                      ? '-'
                                      : (item['description'] ?? '').toString())
                                  .toString(),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                            trailing: const Icon(Icons.chevron_right_rounded),
                            onTap: () => _openQuoteImportItemDetail(
                              importId: importId,
                              itemId: (item['id'] ?? '').toString(),
                            ),
                          ),
                        ),
                      if (remainingItems > 0)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text('+$remainingItems weitere Positionen'),
                        ),
                    ],
                  ),
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(dialogContext).pop(),
                  child: const Text('Schließen'),
                ),
              ],
            );
          },
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'GAEB-Import konnte nicht geladen werden')),
        ),
      );
    }
  }

  Future<void> _openQuoteImportItemDetail({
    required String importId,
    required String itemId,
  }) async {
    try {
      var detail = await widget.api.getQuoteImportItem(importId, itemId);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => StatefulBuilder(
          builder: (context, setDialogState) => AlertDialog(
            title: Text(
              ((detail['position_no'] ?? '').toString().trim().isEmpty
                      ? 'Importposition'
                      : 'Position ${(detail['position_no'] ?? '').toString()}')
                  .toString(),
            ),
            content: Builder(
              builder: (context) {
                final linkedQuoteId =
                    (detail['linked_quote_id'] ?? '').toString().trim();
                final linkedQuoteItemId =
                    (detail['linked_quote_item_id'] ?? '').toString().trim();
                final linkedQuotePosition =
                    ((detail['linked_quote_position'] ?? 0) as num?) ?? 0;

                return SizedBox(
                  width: 620,
                  child: SingleChildScrollView(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                            'Gliederung: ${((detail['outline_no'] ?? '').toString().trim().isEmpty ? '-' : detail['outline_no']).toString()}'),
                        Text(
                            'Menge: ${((detail['qty'] ?? '').toString().trim().isEmpty ? '-' : detail['qty']).toString()}'),
                        Text(
                            'Einheit: ${((detail['unit'] ?? '').toString().trim().isEmpty ? '-' : detail['unit']).toString()}'),
                        Text(
                            'Optional: ${(detail['is_optional'] == true) ? 'Ja' : 'Nein'}'),
                        Text(
                            'Review-Status: ${((detail['review_status'] ?? '').toString().trim().isEmpty ? '-' : detail['review_status']).toString()}'),
                        Text(
                            'Parser-Hinweis: ${((detail['parser_hint'] ?? '').toString().trim().isEmpty ? '-' : detail['parser_hint']).toString()}'),
                        Text(
                            'Review-Notiz: ${((detail['review_note'] ?? '').toString().trim().isEmpty ? '-' : detail['review_note']).toString()}'),
                        const SizedBox(height: 12),
                        const Text(
                          'Beschreibung',
                          style: TextStyle(fontWeight: FontWeight.bold),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          ((detail['description'] ?? '')
                                      .toString()
                                      .trim()
                                      .isEmpty
                                  ? '-'
                                  : detail['description'])
                              .toString(),
                        ),
                        const SizedBox(height: 12),
                        const Text(
                          'Quote-Verknüpfung',
                          style: TextStyle(fontWeight: FontWeight.bold),
                        ),
                        const SizedBox(height: 4),
                        if (linkedQuoteId.isEmpty)
                          const Text('Noch nicht in eine Quote übernommen.')
                        else ...[
                          Text('Quote: $linkedQuoteId'),
                          Text(
                              'Quote-Position: ${linkedQuotePosition <= 0 ? "-" : linkedQuotePosition.toString()}'),
                          Text(
                              'Quote-Item-ID: ${linkedQuoteItemId.isEmpty ? "-" : linkedQuoteItemId}'),
                        ],
                      ],
                    ),
                  ),
                );
              },
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(),
                child: const Text('Schließen'),
              ),
              if ((detail['linked_quote_id'] ?? '')
                      .toString()
                      .trim()
                      .isNotEmpty &&
                  widget.api.hasPermission('quotes.read'))
                FilledButton.tonalIcon(
                  onPressed: () async {
                    final linkedQuoteId =
                        (detail['linked_quote_id'] ?? '').toString().trim();
                    Navigator.of(dialogContext).pop();
                    try {
                      setState(() {
                        _statusFilter = null;
                        _followUpOnlyFilter = false;
                      });
                      final linkedQuote =
                          await widget.api.getQuote(linkedQuoteId);
                      if (!mounted) return;
                      setState(() => _selected = linkedQuote);
                      await _load();
                      if (!mounted) return;
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(
                          content: Text('Verknüpfte Quote wurde geöffnet'),
                        ),
                      );
                    } catch (e) {
                      if (!mounted) return;
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text(_quoteErrorMessage(e,
                              fallback:
                                  'Verknüpfte Quote konnte nicht geöffnet werden')),
                        ),
                      );
                    }
                  },
                  icon: const Icon(Icons.open_in_new_rounded),
                  label: const Text('Quote öffnen'),
                ),
              if (widget.api.hasPermission('quotes.write'))
                FilledButton.tonalIcon(
                  onPressed: () async {
                    final currentStatus =
                        (detail['review_status'] ?? 'pending').toString();
                    final next = await showDialog<_QuoteImportReviewDecision>(
                      context: dialogContext,
                      builder: (_) => _QuoteImportReviewDialog(
                        initialStatus: currentStatus.trim().isEmpty
                            ? 'pending'
                            : currentStatus,
                        initialNote: (detail['review_note'] ?? '').toString(),
                      ),
                    );
                    if (next == null) return;
                    try {
                      final updated =
                          await widget.api.updateQuoteImportItemReview(
                        importId: importId,
                        itemId: itemId,
                        reviewStatus: next.status,
                        reviewNote: next.note,
                      );
                      if (!mounted) return;
                      setDialogState(() => detail = updated);
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(
                          content:
                              Text('Review-Entscheidung wurde gespeichert'),
                        ),
                      );
                    } catch (e) {
                      if (!mounted) return;
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text(_quoteErrorMessage(e,
                              fallback:
                                  'Review-Entscheidung konnte nicht gespeichert werden')),
                        ),
                      );
                    }
                  },
                  icon: const Icon(Icons.rule_rounded),
                  label: const Text('Review setzen'),
                ),
            ],
          ),
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Importposition konnte nicht geladen werden')),
        ),
      );
    }
  }

  String _formatDateTime(dynamic value) {
    final raw = value?.toString() ?? '';
    if (raw.trim().isEmpty) return '-';
    final parsed = DateTime.tryParse(raw);
    if (parsed == null) return raw;
    final local = parsed.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(local.day)}.${two(local.month)}.${local.year} ${two(local.hour)}:${two(local.minute)}';
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
    final visibleImports = _quoteImports.take(3).cast<Map>().map((item) {
      return item.cast<String, dynamic>();
    }).toList();
    return Scaffold(
      appBar: AppBar(
        title: const Text('Angebote'),
        actions: [
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh_rounded)),
        ],
      ),
      floatingActionButton: canWrite
          ? FloatingActionButton.extended(
              onPressed: () => _openCreateDialog(),
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
                          if (canWrite)
                            FilledButton.tonalIcon(
                              onPressed: _importGAEB,
                              icon: const Icon(Icons.upload_file_rounded),
                              label: const Text('GAEB-Import'),
                            ),
                        ],
                      ),
                    ),
                    const Divider(height: 1),
                    Padding(
                      padding: const EdgeInsets.all(12),
                      child: Card(
                        margin: EdgeInsets.zero,
                        child: Padding(
                          padding: const EdgeInsets.all(12),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                children: [
                                  Expanded(
                                    child: Text(
                                      'GAEB-Importe',
                                      style: Theme.of(context)
                                          .textTheme
                                          .titleMedium,
                                    ),
                                  ),
                                  IconButton(
                                    tooltip: 'Importe aktualisieren',
                                    onPressed: _importsLoading
                                        ? null
                                        : _loadQuoteImports,
                                    icon: const Icon(Icons.refresh_rounded),
                                  ),
                                ],
                              ),
                              if (_projectCtrl.text.trim().isEmpty)
                                const Text(
                                  'Für projektbezogene Importe bitte oben eine Projekt-ID setzen.',
                                )
                              else if (_importsLoading)
                                const Padding(
                                  padding: EdgeInsets.symmetric(vertical: 8),
                                  child: Center(
                                    child: CircularProgressIndicator(),
                                  ),
                                )
                              else if (visibleImports.isEmpty)
                                const Text(
                                  'Noch keine GAEB-Importe für dieses Projekt vorhanden.',
                                )
                              else ...[
                                const SizedBox(height: 8),
                                ...visibleImports.map((item) {
                                  final importId =
                                      (item['id'] ?? '').toString();
                                  return ListTile(
                                    dense: true,
                                    contentPadding: EdgeInsets.zero,
                                    title: Text((item['source_filename'] ??
                                            'GAEB-Import')
                                        .toString()),
                                    subtitle: Text(
                                      '${(item['status'] ?? '-').toString()}  •  ${_formatDateTime(item['uploaded_at'])}',
                                    ),
                                    trailing: TextButton(
                                      onPressed: importId.isEmpty
                                          ? null
                                          : () =>
                                              _openQuoteImportDetail(importId),
                                      child: const Text('Details'),
                                    ),
                                  );
                                }),
                                if (_quoteImports.length >
                                    visibleImports.length)
                                  Padding(
                                    padding: const EdgeInsets.only(top: 4),
                                    child: Text(
                                      '+${_quoteImports.length - visibleImports.length} weitere',
                                      style:
                                          Theme.of(context).textTheme.bodySmall,
                                    ),
                                  ),
                              ],
                            ],
                          ),
                        ),
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

class _QuoteProgressDialog extends StatelessWidget {
  const _QuoteProgressDialog({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      content: Row(
        children: [
          const SizedBox(width: 8),
          const CircularProgressIndicator(),
          const SizedBox(width: 16),
          Flexible(child: Text(text)),
        ],
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

class _QuoteImportReviewDecision {
  const _QuoteImportReviewDecision({
    required this.status,
    required this.note,
  });

  final String status;
  final String note;
}

class _QuoteImportReviewDialog extends StatefulWidget {
  const _QuoteImportReviewDialog({
    required this.initialStatus,
    required this.initialNote,
  });

  final String initialStatus;
  final String initialNote;

  @override
  State<_QuoteImportReviewDialog> createState() =>
      _QuoteImportReviewDialogState();
}

class _QuoteImportReviewDialogState extends State<_QuoteImportReviewDialog> {
  static const _statuses = ['pending', 'accepted', 'rejected'];
  late String _status;
  late final TextEditingController _noteCtrl;

  @override
  void initState() {
    super.initState();
    _status = _statuses.contains(widget.initialStatus)
        ? widget.initialStatus
        : 'pending';
    _noteCtrl = TextEditingController(text: widget.initialNote);
  }

  @override
  void dispose() {
    _noteCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Review-Entscheidung'),
      content: SizedBox(
        width: 420,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            DropdownButtonFormField<String>(
              initialValue: _status,
              decoration: const InputDecoration(labelText: 'Review-Status'),
              items: _statuses
                  .map(
                    (status) => DropdownMenuItem<String>(
                      value: status,
                      child: Text(status),
                    ),
                  )
                  .toList(),
              onChanged: (value) {
                if (value == null) return;
                setState(() => _status = value);
              },
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _noteCtrl,
              decoration: const InputDecoration(labelText: 'Review-Notiz'),
              maxLines: 3,
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
          onPressed: () => Navigator.of(context).pop(
            _QuoteImportReviewDecision(
              status: _status,
              note: _noteCtrl.text.trim(),
            ),
          ),
          child: const Text('Speichern'),
        ),
      ],
    );
  }
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
  const _QuoteEditorDialog({
    required this.api,
    this.initial,
    this.initialFilters,
  });

  final ApiClient api;
  final Map<String, dynamic>? initial;
  final CommercialFilterContext? initialFilters;

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
  String? _applyingCandidateItemId;

  bool get _isEdit => widget.initial != null;

  @override
  void initState() {
    super.initState();
    final initial = widget.initial;
    final initialFilters =
        widget.initialFilters ?? const CommercialFilterContext();
    _projectCtrl = TextEditingController(
      text: initialFilters.normalizedProjectId ??
          (initial?['project_id'] ?? '').toString(),
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

  void _replaceDraftFromQuote(Map<String, dynamic> quote) {
    _projectCtrl.text = (quote['project_id'] ?? '').toString();
    _contactCtrl.text = (quote['contact_id'] ?? '').toString();
    _currencyCtrl.text = (quote['currency'] ?? 'EUR').toString();
    _noteCtrl.text = (quote['note'] ?? '').toString();
    for (final item in _items) {
      item.dispose();
    }
    _items
      ..clear()
      ..addAll((((quote['items'] as List?) ?? const []).whereType<Map>().map(
          (entry) => _QuoteItemDraft.fromJson(entry.cast<String, dynamic>()))));
    if (_items.isEmpty) {
      _items.add(_QuoteItemDraft());
    }
  }

  Future<void> _applyMaterialCandidate(
    _QuoteItemDraft item,
    _QuoteMaterialCandidateDraft candidate,
  ) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _applyingCandidateItemId = itemId);
    try {
      final updated = await widget.api.applyQuoteMaterialCandidate(
        quoteId,
        itemId,
        candidate.materialId,
      );
      if (!mounted) return;
      setState(() {
        _replaceDraftFromQuote(updated);
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            'Materialkandidat ${candidate.materialId} wurde uebernommen',
          ),
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Kandidatenuebernahme fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _applyingCandidateItemId = null);
      }
    }
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
                  canApplyCandidate: _isEdit &&
                      widget.api.hasPermission('quotes.write') &&
                      !_saving,
                  applyingCandidate:
                      _applyingCandidateItemId == _items[i].id.trim(),
                  onApplyCandidate: (candidate) =>
                      _applyMaterialCandidate(_items[i], candidate),
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
    String id = '',
    String description = '',
    String qty = '1',
    String unit = 'Stk',
    String unitPrice = '0',
    String taxCode = 'DE19',
    String materialId = '',
    String priceMappingStatus = 'open',
    String materialCandidateStatus = 'none',
    List<_QuoteMaterialCandidateDraft> materialCandidates = const [],
  })  : id = id,
        descriptionCtrl = TextEditingController(text: description),
        qtyCtrl = TextEditingController(text: qty),
        unitCtrl = TextEditingController(text: unit),
        unitPriceCtrl = TextEditingController(text: unitPrice),
        taxCodeCtrl = TextEditingController(text: taxCode),
        materialIdCtrl = TextEditingController(text: materialId),
        priceMappingStatus = _normalizePriceMappingStatus(priceMappingStatus),
        materialCandidateStatus =
            _normalizeMaterialCandidateStatus(materialCandidateStatus),
        materialCandidates =
            List<_QuoteMaterialCandidateDraft>.unmodifiable(materialCandidates);

  factory _QuoteItemDraft.fromJson(Map<String, dynamic> json) {
    return _QuoteItemDraft(
      id: (json['id'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      qty: (json['qty'] ?? 1).toString(),
      unit: (json['unit'] ?? 'Stk').toString(),
      unitPrice: (json['unit_price'] ?? 0).toString(),
      taxCode: (json['tax_code'] ?? 'DE19').toString(),
      materialId: (json['material_id'] ?? '').toString(),
      priceMappingStatus: (json['price_mapping_status'] ?? 'open').toString(),
      materialCandidateStatus:
          (json['material_candidate_status'] ?? 'none').toString(),
      materialCandidates: ((json['material_candidates'] as List?) ?? const [])
          .whereType<Map>()
          .map((entry) => _QuoteMaterialCandidateDraft.fromJson(
              entry.cast<String, dynamic>()))
          .toList(),
    );
  }

  static String _normalizePriceMappingStatus(String value) {
    switch (value.trim().toLowerCase()) {
      case 'manual':
        return 'manual';
      default:
        return 'open';
    }
  }

  static String _normalizeMaterialCandidateStatus(String value) {
    switch (value.trim().toLowerCase()) {
      case 'available':
        return 'available';
      default:
        return 'none';
    }
  }

  final String id;
  final TextEditingController descriptionCtrl;
  final TextEditingController qtyCtrl;
  final TextEditingController unitCtrl;
  final TextEditingController unitPriceCtrl;
  final TextEditingController taxCodeCtrl;
  final TextEditingController materialIdCtrl;
  String priceMappingStatus;
  final String materialCandidateStatus;
  final List<_QuoteMaterialCandidateDraft> materialCandidates;

  Map<String, dynamic> toJson() => {
        'description': descriptionCtrl.text.trim(),
        'qty': double.tryParse(qtyCtrl.text.trim()) ?? 0,
        'unit': unitCtrl.text.trim().isEmpty ? 'Stk' : unitCtrl.text.trim(),
        'unit_price': double.tryParse(unitPriceCtrl.text.trim()) ?? 0,
        'tax_code':
            taxCodeCtrl.text.trim().isEmpty ? 'DE19' : taxCodeCtrl.text.trim(),
        'material_id': materialIdCtrl.text.trim(),
        'price_mapping_status': priceMappingStatus,
      };

  void dispose() {
    descriptionCtrl.dispose();
    qtyCtrl.dispose();
    unitCtrl.dispose();
    unitPriceCtrl.dispose();
    taxCodeCtrl.dispose();
    materialIdCtrl.dispose();
  }
}

class _QuoteMaterialCandidateDraft {
  const _QuoteMaterialCandidateDraft({
    required this.materialId,
    required this.materialNo,
    required this.materialLabel,
  });

  factory _QuoteMaterialCandidateDraft.fromJson(Map<String, dynamic> json) {
    return _QuoteMaterialCandidateDraft(
      materialId: (json['material_id'] ?? '').toString(),
      materialNo: (json['material_no'] ?? '').toString(),
      materialLabel: (json['material_label'] ?? '').toString(),
    );
  }

  final String materialId;
  final String materialNo;
  final String materialLabel;

  String get displayTitle {
    final no = materialNo.trim();
    final label = materialLabel.trim();
    if (no.isNotEmpty && label.isNotEmpty) {
      return '$no  •  $label';
    }
    if (label.isNotEmpty) return label;
    if (no.isNotEmpty) return no;
    return materialId;
  }
}

class _QuoteItemRow extends StatefulWidget {
  const _QuoteItemRow(
      {super.key,
      required this.item,
      required this.index,
      required this.canApplyCandidate,
      required this.applyingCandidate,
      this.onApplyCandidate,
      this.onRemove});

  final _QuoteItemDraft item;
  final int index;
  final bool canApplyCandidate;
  final bool applyingCandidate;
  final ValueChanged<_QuoteMaterialCandidateDraft>? onApplyCandidate;
  final VoidCallback? onRemove;

  @override
  State<_QuoteItemRow> createState() => _QuoteItemRowState();
}

class _QuoteItemRowState extends State<_QuoteItemRow> {
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
                  child: Text('Position ${widget.index + 1}',
                      style: const TextStyle(fontWeight: FontWeight.bold)),
                ),
                if (widget.onRemove != null)
                  IconButton(
                      onPressed: widget.onRemove,
                      icon: const Icon(Icons.delete_outline_rounded)),
              ],
            ),
            TextField(
                controller: widget.item.descriptionCtrl,
                decoration: const InputDecoration(labelText: 'Beschreibung')),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                    child: TextField(
                        controller: widget.item.qtyCtrl,
                        decoration: const InputDecoration(labelText: 'Menge'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: widget.item.unitCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Einheit'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: widget.item.unitPriceCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Einzelpreis'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: widget.item.taxCodeCtrl,
                        decoration:
                            const InputDecoration(labelText: 'Steuercode'))),
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                  flex: 2,
                  child: TextField(
                    controller: widget.item.materialIdCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Material-ID',
                      helperText: 'Optionaler manueller Materialbezug',
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: DropdownButtonFormField<String>(
                    initialValue: widget.item.priceMappingStatus,
                    decoration: const InputDecoration(
                      labelText: 'Preisstatus',
                      helperText: 'Kleiner manueller Mapping-Status',
                    ),
                    items: const [
                      DropdownMenuItem(
                        value: 'open',
                        child: Text('open'),
                      ),
                      DropdownMenuItem(
                        value: 'manual',
                        child: Text('manual'),
                      ),
                    ],
                    onChanged: (value) {
                      setState(() {
                        widget.item.priceMappingStatus =
                            value == 'manual' ? 'manual' : 'open';
                      });
                    },
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Align(
              alignment: Alignment.centerLeft,
              child: Text(
                widget.item.materialCandidateStatus == 'available'
                    ? 'Materialkandidat: verfuegbar'
                    : 'Materialkandidat: keiner sichtbar',
                style: TextStyle(
                  fontSize: 12,
                  color: widget.item.materialCandidateStatus == 'available'
                      ? Colors.orange.shade800
                      : Colors.grey.shade700,
                  fontWeight: widget.item.materialCandidateStatus == 'available'
                      ? FontWeight.w600
                      : FontWeight.w400,
                ),
              ),
            ),
            if (widget.item.materialCandidates.isNotEmpty) ...[
              const SizedBox(height: 6),
              Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  'Kandidatenliste',
                  style: TextStyle(
                    fontSize: 12,
                    color: Colors.grey.shade800,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const SizedBox(height: 4),
              ...widget.item.materialCandidates.map(
                (candidate) => Padding(
                  padding: const EdgeInsets.only(bottom: 4),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(
                        child: Text(
                          '${candidate.displayTitle}  •  ${candidate.materialId}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                      ),
                      if (widget.canApplyCandidate &&
                          widget.onApplyCandidate != null &&
                          widget.item.id.trim().isNotEmpty)
                        TextButton(
                          onPressed: widget.applyingCandidate
                              ? null
                              : () => widget.onApplyCandidate!(candidate),
                          child: widget.applyingCandidate
                              ? const SizedBox(
                                  height: 14,
                                  width: 14,
                                  child:
                                      CircularProgressIndicator(strokeWidth: 2),
                                )
                              : const Text('Uebernehmen'),
                        ),
                    ],
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
