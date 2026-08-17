import 'package:flutter/material.dart';

import '../api.dart';
import '../commercial_destinations.dart';
import '../commercial_navigation.dart';
import '../web/browser.dart' as browser;

typedef QuoteImportFilePicker = Future<browser.PickedFile?> Function({
  String? accept,
});

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
    this.quoteImportFilePicker,
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
  final QuoteImportFilePicker? quoteImportFilePicker;

  @override
  State<QuotesPage> createState() => _QuotesPageState();
}

class _QuotesPageState extends State<QuotesPage> {
  bool _loading = true;
  bool _importsLoading = false;
  bool _approvalReworkLoading = false;
  bool _approvalRequestsLoading = false;
  bool _approvalReworkExpanded = false;
  bool _approvalRequestsExpanded = false;
  bool _quoteImportsExpanded = false;
  String? _approvingApprovalRequestQueueItemId;
  String? _rejectingApprovalRequestQueueItemId;
  List<dynamic> _items = const [];
  List<dynamic> _quoteImports = const [];
  List<dynamic> _approvalReworkItems = const [];
  List<dynamic> _approvalRequestItems = const [];
  Map<String, dynamic>? _selected;
  Map<String, dynamic>? _linkedSalesOrder;
  List<dynamic> _salesOrderInvoices = const [];
  final Map<int, GlobalKey> _quoteItemJumpKeys = {};
  int? _highlightedRejectedApprovalPosition;
  int _quoteItemHighlightToken = 0;
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
    setState(() {
      _loading = true;
      _approvalReworkExpanded = false;
      _approvalRequestsExpanded = false;
      _quoteImportsExpanded = false;
    });
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
      await _loadApprovalReworkQueue();
      await _loadApprovalRequestQueue();
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
      if (mounted) {
        setState(() {
          _quoteImports = imports;
          if (_projectCtrl.text.trim().isEmpty || imports.length <= 3) {
            _quoteImportsExpanded = false;
          }
        });
      }
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

  Future<void> _loadApprovalReworkQueue() async {
    if (!widget.api.hasPermission('quotes.read')) return;
    if (mounted) setState(() => _approvalReworkLoading = true);
    try {
      final items = await widget.api.listQuoteApprovalRework(
        projectId:
            _projectCtrl.text.trim().isEmpty ? null : _projectCtrl.text.trim(),
      );
      if (mounted) {
        setState(() {
          _approvalReworkItems = items;
          if (items.length <= 3) {
            _approvalReworkExpanded = false;
          }
        });
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Nacharbeits-Queue konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) setState(() => _approvalReworkLoading = false);
    }
  }

  Future<void> _loadApprovalRequestQueue() async {
    if (!widget.api.hasPermission('quotes.read')) return;
    if (mounted) setState(() => _approvalRequestsLoading = true);
    try {
      final items = await widget.api.listQuoteApprovalRequests(
        projectId:
            _projectCtrl.text.trim().isEmpty ? null : _projectCtrl.text.trim(),
      );
      if (mounted) {
        setState(() {
          _approvalRequestItems = items;
          if (items.length <= 3) {
            _approvalRequestsExpanded = false;
          }
        });
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback:
                  'Freigabeanforderungs-Queue konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) setState(() => _approvalRequestsLoading = false);
    }
  }

  Future<void> _loadDetail(String id) async {
    try {
      final detail = await widget.api.getQuote(id);
      if (mounted) {
        setState(() {
          _selected = detail;
          _highlightedRejectedApprovalPosition = null;
        });
      }
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

  List<int> _quoteRejectedApprovalDecisionPositions(
      Map<String, dynamic>? quote) {
    final items = (quote?['items'] as List?) ?? const [];
    final positions = <int>[];
    for (var index = 0; index < items.length; index++) {
      final rawItem = items[index];
      if (rawItem is! Map) continue;
      final item = rawItem.cast<dynamic, dynamic>();
      final rawDecision = item['latest_approval_decision'];
      if (rawDecision is! Map) continue;
      final decision = rawDecision.cast<dynamic, dynamic>();
      if ((decision['status'] ?? '').toString() == 'rejected') {
        positions.add(index + 1);
      }
    }
    return positions;
  }

  String _formatRejectedApprovalDecisionPositions(List<int> positions) {
    if (positions.isEmpty) return '';
    final visible = positions.take(3).map((position) => 'Pos. $position');
    final hiddenCount = positions.length - 3;
    final suffix = hiddenCount > 0 ? ' + $hiddenCount weitere' : '';
    final label =
        positions.length == 1 ? 'Betroffene Position' : 'Betroffene Positionen';
    return '$label: ${visible.join(', ')}$suffix';
  }

  String _approvalDecisionReasonLabel(Map<dynamic, dynamic> decision) {
    final reasonText = (decision['reason_text'] ?? '').toString().trim();
    if (reasonText.isNotEmpty) return reasonText;
    final reasonCode = (decision['reason_code'] ?? '').toString().trim();
    switch (reasonCode) {
      case 'negative_margin':
        return 'Negative Marge';
      case 'below_target_margin':
        return 'Unter Zielmarge';
      default:
        return reasonCode;
    }
  }

  String _approvalReworkReasonLabel(Map<String, dynamic> item) {
    final reasonText = (item['reason_text'] ?? '').toString().trim();
    if (reasonText.isNotEmpty) return reasonText;
    final reasonCode = (item['reason_code'] ?? '').toString().trim();
    switch (reasonCode) {
      case 'negative_margin':
        return 'Negative Marge';
      case 'below_target_margin':
        return 'Unter Zielmarge';
      default:
        return reasonCode.isEmpty ? 'Nacharbeit erforderlich' : reasonCode;
    }
  }

  String _approvalRequestReasonLabel(Map<String, dynamic> item) {
    final reasonText = (item['reason_text'] ?? '').toString().trim();
    if (reasonText.isNotEmpty) return reasonText;
    final reasonCode = (item['reason_code'] ?? '').toString().trim();
    switch (reasonCode) {
      case 'negative_margin':
        return 'Negative Marge';
      case 'below_target_margin':
        return 'Unter Zielmarge';
      default:
        return reasonCode.isEmpty ? 'Freigabe angefordert' : reasonCode;
    }
  }

  String _approvalRequestContext(Map<String, dynamic> item) {
    final parts = <String>[];
    final requestedByName = (item['requested_by_name'] ?? '').toString().trim();
    final requestedBy = (item['requested_by'] ?? '').toString().trim();
    final requestUser =
        requestedByName.isNotEmpty ? requestedByName : requestedBy;
    if (requestUser.isNotEmpty) parts.add(requestUser);
    final requestedAt = (item['requested_at'] ?? '').toString().trim();
    if (requestedAt.isNotEmpty && DateTime.tryParse(requestedAt) != null) {
      final formatted = _formatDateTime(requestedAt);
      if (formatted != '-') parts.add(formatted);
    }
    return parts.join('  •  ');
  }

  String _formatPercent(num value) {
    return '${value.toDouble().toStringAsFixed(2)} %';
  }

  String _formatSignedMoney(num value, String currency) {
    final sign = value > 0 ? '+' : '';
    return '$sign${_formatMoney(value, currency)}';
  }

  Widget _approvalQueueDecisionValueRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(top: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 132,
            child: Text(
              label,
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildApprovalQueueDecisionContext(Map<String, dynamic> item) {
    final theme = Theme.of(context);
    final currency = (item['currency'] ?? 'EUR').toString().trim();
    final normalizedCurrency = currency.isEmpty ? 'EUR' : currency;
    final position = (item['position'] as num?)?.toInt();
    final quoteNumber = (item['quote_number'] ?? 'Angebot').toString().trim();
    final description = (item['description'] ?? '').toString().trim();
    final projectName = (item['project_name'] ?? '').toString().trim();
    final contactName = (item['contact_name'] ?? '').toString().trim();
    final projectContact = [
      if (projectName.isNotEmpty) projectName,
      if (contactName.isNotEmpty) contactName,
    ].join('  •  ');
    final requestContext = _approvalRequestContext(item);
    final reason = _approvalRequestReasonLabel(item);
    final currentTargetContext = _approvalReworkCurrentTargetContext(item);
    final valueRows = <Widget>[];

    void addMoneyRow(String label, String key, {bool signed = false}) {
      final value = _toOptionalDouble(item[key]);
      if (value == null) return;
      valueRows.add(_approvalQueueDecisionValueRow(
        label,
        signed
            ? _formatSignedMoney(value, normalizedCurrency)
            : _formatMoney(value, normalizedCurrency),
      ));
    }

    addMoneyRow('Aktueller Preis', 'current_unit_price');
    addMoneyRow('Preis Anfrage', 'current_unit_price_snapshot');
    addMoneyRow('Kostenbasis', 'cost_basis_unit_price_snapshot');
    addMoneyRow('Zielpreis', 'target_unit_price_snapshot');
    final targetMargin =
        _toOptionalDouble(item['target_margin_percent_snapshot']);
    if (targetMargin != null) {
      valueRows.add(_approvalQueueDecisionValueRow(
        'Zielmarge',
        _formatPercent(targetMargin),
      ));
    }
    addMoneyRow('Zielabweichung', 'target_difference_snapshot', signed: true);

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color:
            theme.colorScheme.surfaceContainerHighest.withValues(alpha: 0.45),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.dividerColor),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '${quoteNumber.isEmpty ? 'Angebot' : quoteNumber} · Pos. ${position ?? '-'}',
            style: theme.textTheme.titleSmall,
          ),
          const SizedBox(height: 4),
          Text(
            description.isEmpty ? 'Position' : description,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.bodyMedium,
          ),
          if (projectContact.isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(
              projectContact,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.bodySmall,
            ),
          ],
          if (requestContext.isNotEmpty) ...[
            const SizedBox(height: 2),
            Text(
              requestContext,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.bodySmall,
            ),
          ],
          const SizedBox(height: 8),
          Text(
            'Grund: $reason',
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.bodySmall,
          ),
          if (valueRows.isNotEmpty) ...[
            const SizedBox(height: 8),
            ...valueRows,
          ],
          if (currentTargetContext.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(
              currentTargetContext,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.primary,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ],
      ),
    );
  }

  String _approvalReworkDecisionContext(Map<String, dynamic> item) {
    final parts = <String>[];
    final comment = (item['decision_comment'] ?? '').toString().trim();
    if (comment.isNotEmpty) parts.add('Kommentar: $comment');
    final decidedByName = (item['decided_by_name'] ?? '').toString().trim();
    final decidedBy = (item['decided_by'] ?? '').toString().trim();
    final decisionUser = decidedByName.isNotEmpty ? decidedByName : decidedBy;
    if (decisionUser.isNotEmpty) parts.add(decisionUser);
    final decidedAt = (item['decided_at'] ?? '').toString().trim();
    if (decidedAt.isNotEmpty && DateTime.tryParse(decidedAt) != null) {
      final formatted = _formatDateTime(decidedAt);
      if (formatted != '-') parts.add(formatted);
    }
    return parts.join('  •  ');
  }

  String _approvalReworkTargetStatusLabel(String status) {
    switch (status.trim()) {
      case 'below_cost':
        return 'Unter Kostenbasis';
      case 'below_target':
        return 'Unter Zielmarge';
      case 'on_target':
        return 'Zielmarge erreicht';
      case 'above_target':
        return 'Ueber Zielmarge';
      default:
        return status.trim();
    }
  }

  double? _toOptionalDouble(dynamic value) {
    if (value == null) return null;
    if (value is num) return value.toDouble();
    final raw = value.toString().trim();
    if (raw.isEmpty) return null;
    return double.tryParse(raw);
  }

  String _approvalReworkCurrentTargetContext(Map<String, dynamic> item) {
    final rawStatus = (item['current_target_status'] ?? '').toString().trim();
    if (rawStatus.isEmpty) return '';
    final status = _approvalReworkTargetStatusLabel(rawStatus);
    if (status.isEmpty) return '';
    final currency = (item['currency'] ?? 'EUR').toString().trim();
    final difference = _toOptionalDouble(item['current_target_difference']);
    if (difference == null) return 'Aktuell: $status';
    final formattedDifference = difference > 0
        ? '+${_formatMoney(difference, currency)}'
        : _formatMoney(difference, currency);
    return 'Aktuell: $status  •  Abweichung $formattedDifference';
  }

  String _quoteItemRejectedApprovalDecisionSummary(Map<String, dynamic> item) {
    final rawDecision = item['latest_approval_decision'];
    if (rawDecision is! Map) return '';
    final decision = rawDecision.cast<dynamic, dynamic>();
    if ((decision['status'] ?? '').toString() != 'rejected') return '';
    final details = <String>[];
    final reason = _approvalDecisionReasonLabel(decision);
    if (reason.isNotEmpty) details.add(reason);
    final comment = (decision['decision_comment'] ?? '').toString().trim();
    if (comment.isNotEmpty) details.add('Kommentar: $comment');
    if (details.isEmpty) return 'Nacharbeit erforderlich';
    return 'Nacharbeit: ${details.join('  •  ')}';
  }

  GlobalKey _quoteItemJumpKey(int position) {
    return _quoteItemJumpKeys.putIfAbsent(position, GlobalKey.new);
  }

  void _highlightRejectedApprovalPosition(int position) {
    final token = ++_quoteItemHighlightToken;
    setState(() => _highlightedRejectedApprovalPosition = position);
    Future.delayed(const Duration(seconds: 3), () {
      if (!mounted || token != _quoteItemHighlightToken) return;
      if (_highlightedRejectedApprovalPosition != position) return;
      setState(() => _highlightedRejectedApprovalPosition = null);
    });
  }

  void _scrollToFirstRejectedApprovalDecisionPosition() {
    final positions = _quoteRejectedApprovalDecisionPositions(_selected);
    if (positions.isEmpty) return;
    final firstPosition = positions.first;
    final targetContext = _quoteItemJumpKeys[firstPosition]?.currentContext;
    if (targetContext == null) return;
    _highlightRejectedApprovalPosition(firstPosition);
    Scrollable.ensureVisible(
      targetContext,
      duration: const Duration(milliseconds: 300),
      curve: Curves.easeInOut,
      alignment: 0.08,
    );
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

  Future<void> _openEditDialog({
    String? initialFocusItemId,
    int? initialFocusPosition,
  }) async {
    final selected = _selected;
    if (selected == null) return;
    final updated = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) => _QuoteEditorDialog(
        api: widget.api,
        initial: selected,
        initialFocusItemId: initialFocusItemId,
        initialFocusPosition: initialFocusPosition,
      ),
    );
    if (updated == null || !mounted) return;
    setState(() => _selected = updated);
    await _load();
  }

  Future<void> _openApprovalReworkItem(Map<String, dynamic> item) async {
    final quoteId = (item['quote_id'] ?? '').toString().trim();
    final quoteItemId = (item['quote_item_id'] ?? '').toString().trim();
    final position = (item['position'] as num?)?.toInt();
    if (quoteId.isEmpty) return;
    try {
      final quote = await widget.api.getQuote(quoteId);
      if (!mounted) return;
      setState(() => _selected = quote);
      await _loadLinkedSalesOrder(quote['linked_sales_order_id']?.toString());
      await _loadSalesOrderInvoices(quote['linked_sales_order_id']?.toString());
      if (!mounted) return;
      final canEdit = widget.api.hasPermission('quotes.write') &&
          (quote['status'] ?? '').toString() == 'draft';
      if (canEdit) {
        await _openEditDialog(
          initialFocusItemId: quoteItemId.isEmpty ? null : quoteItemId,
          initialFocusPosition:
              position != null && position > 0 ? position : null,
        );
      } else if (position != null && position > 0) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          final targetContext = _quoteItemJumpKeys[position]?.currentContext;
          if (targetContext == null) return;
          _highlightRejectedApprovalPosition(position);
          Scrollable.ensureVisible(
            targetContext,
            duration: const Duration(milliseconds: 300),
            curve: Curves.easeInOut,
            alignment: 0.08,
          );
        });
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Nacharbeitsposition konnte nicht geöffnet werden')),
        ),
      );
    }
  }

  Future<void> _openApprovalRequestItem(Map<String, dynamic> item) async {
    final quoteId = (item['quote_id'] ?? '').toString().trim();
    final quoteItemId = (item['quote_item_id'] ?? '').toString().trim();
    final position = (item['position'] as num?)?.toInt();
    if (quoteId.isEmpty) return;
    try {
      final quote = await widget.api.getQuote(quoteId);
      if (!mounted) return;
      setState(() => _selected = quote);
      await _loadLinkedSalesOrder(quote['linked_sales_order_id']?.toString());
      await _loadSalesOrderInvoices(quote['linked_sales_order_id']?.toString());
      if (!mounted) return;
      final canEdit = widget.api.hasPermission('quotes.write') &&
          (quote['status'] ?? '').toString() == 'draft';
      if (canEdit) {
        await _openEditDialog(
          initialFocusItemId: quoteItemId.isEmpty ? null : quoteItemId,
          initialFocusPosition:
              position != null && position > 0 ? position : null,
        );
      } else if (position != null && position > 0) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          final targetContext = _quoteItemJumpKeys[position]?.currentContext;
          if (targetContext == null) return;
          _highlightRejectedApprovalPosition(position);
          Scrollable.ensureVisible(
            targetContext,
            duration: const Duration(milliseconds: 300),
            curve: Curves.easeInOut,
            alignment: 0.08,
          );
        });
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback:
                  'Freigabeanforderungsposition konnte nicht geoeffnet werden')),
        ),
      );
    }
  }

  String _approvalRequestQueueActionId(Map<String, dynamic> item) {
    final requestId = (item['approval_request_id'] ?? '').toString().trim();
    if (requestId.isNotEmpty) return requestId;
    final quoteId = (item['quote_id'] ?? '').toString().trim();
    final quoteItemId = (item['quote_item_id'] ?? '').toString().trim();
    return '$quoteId:$quoteItemId';
  }

  Future<String?> _promptApprovalQueueDecisionComment({
    required String title,
    required String actionLabel,
    required Map<String, dynamic> item,
  }) {
    return showDialog<String>(
      context: context,
      builder: (context) => _QuoteApprovalDecisionCommentDialog(
        title: title,
        actionLabel: actionLabel,
        helperText: 'Optionaler Kommentar fuer die Freigabehistorie',
        contextSummary: _buildApprovalQueueDecisionContext(item),
      ),
    );
  }

  Future<void> _refreshAfterApprovalQueueDecision(
    String quoteId, {
    required bool reloadReworkQueue,
  }) async {
    await _loadApprovalRequestQueue();
    if (reloadReworkQueue) {
      await _loadApprovalReworkQueue();
    }
    final selectedId = (_selected?['id'] ?? '').toString().trim();
    if (selectedId == quoteId) {
      await _loadDetail(quoteId);
    }
  }

  Future<void> _approveApprovalRequestQueueItem(
      Map<String, dynamic> item) async {
    final quoteId = (item['quote_id'] ?? '').toString().trim();
    final quoteItemId = (item['quote_item_id'] ?? '').toString().trim();
    if (quoteId.isEmpty || quoteItemId.isEmpty) return;
    final comment = await _promptApprovalQueueDecisionComment(
      title: 'Freigabe genehmigen',
      actionLabel: 'Genehmigen',
      item: item,
    );
    if (!mounted || comment == null) return;
    final actionId = _approvalRequestQueueActionId(item);
    setState(() => _approvingApprovalRequestQueueItemId = actionId);
    try {
      await widget.api.approveQuoteItemApprovalRequest(
        quoteId,
        quoteItemId,
        comment: comment,
      );
      if (!mounted) return;
      await _refreshAfterApprovalQueueDecision(
        quoteId,
        reloadReworkQueue: false,
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Freigabe genehmigt')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Genehmigung der Freigabeanforderung fehlgeschlagen')),
        ),
      );
      await _loadApprovalRequestQueue();
    } finally {
      if (mounted) {
        setState(() => _approvingApprovalRequestQueueItemId = null);
      }
    }
  }

  Future<void> _rejectApprovalRequestQueueItem(
      Map<String, dynamic> item) async {
    final quoteId = (item['quote_id'] ?? '').toString().trim();
    final quoteItemId = (item['quote_item_id'] ?? '').toString().trim();
    if (quoteId.isEmpty || quoteItemId.isEmpty) return;
    final comment = await _promptApprovalQueueDecisionComment(
      title: 'Freigabe ablehnen',
      actionLabel: 'Ablehnen',
      item: item,
    );
    if (!mounted || comment == null) return;
    final actionId = _approvalRequestQueueActionId(item);
    setState(() => _rejectingApprovalRequestQueueItemId = actionId);
    try {
      await widget.api.rejectQuoteItemApprovalRequest(
        quoteId,
        quoteItemId,
        comment: comment,
      );
      if (!mounted) return;
      await _refreshAfterApprovalQueueDecision(
        quoteId,
        reloadReworkQueue: true,
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
            content: Text('Freigabe abgelehnt - Nacharbeit erforderlich')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Ablehnung der Freigabeanforderung fehlgeschlagen')),
        ),
      );
      await _loadApprovalRequestQueue();
    } finally {
      if (mounted) {
        setState(() => _rejectingApprovalRequestQueueItemId = null);
      }
    }
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
    final pickFile = widget.quoteImportFilePicker ?? browser.pickFile;
    final picked = await pickFile(
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

  Future<void> _processGAEBImport(String importId) async {
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (_) =>
          const _QuoteProgressDialog(text: 'GAEB-Import wird verarbeitet...'),
    );
    try {
      await widget.api.processGAEBQuoteImport(importId);
      if (!mounted) return;
      Navigator.of(context, rootNavigator: true).pop();
      await _loadQuoteImports();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('GAEB-Import wurde verarbeitet')),
      );
    } catch (e) {
      if (!mounted) return;
      Navigator.of(context, rootNavigator: true).pop();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(_quoteErrorMessage(e,
                fallback: 'GAEB-Import konnte nicht verarbeitet werden'))),
      );
    }
  }

  Future<void> _openQuoteImportDetail(String importId) async {
    try {
      Map<String, dynamic> detail = await widget.api.getQuoteImport(importId);
      List<dynamic> items = await widget.api.listQuoteImportItems(importId);
      if (!mounted) return;
      var itemsExpanded = false;
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
                if (nextItems.length <= 5) {
                  itemsExpanded = false;
                }
              });
            }

            final visibleItems =
                (itemsExpanded ? items : items.take(5)).cast<Map>().map((item) {
              return item.cast<String, dynamic>();
            }).toList();
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
                      if (items.length > 5)
                        TextButton(
                          onPressed: () => setDialogState(() {
                            itemsExpanded = !itemsExpanded;
                          }),
                          child: Text(
                            itemsExpanded
                                ? 'Weniger anzeigen'
                                : 'Alle anzeigen',
                          ),
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
    final rejectedApprovalDecisionPositions =
        _quoteRejectedApprovalDecisionPositions(selected);
    final rejectedApprovalDecisionCount =
        rejectedApprovalDecisionPositions.length;
    final rejectedApprovalDecisionPositionText =
        _formatRejectedApprovalDecisionPositions(
            rejectedApprovalDecisionPositions);
    final canWrite = widget.api.hasPermission('quotes.write');
    final canApproveQuotes = widget.api.hasPermission('quotes.approve');
    final canConvertInvoices = widget.api.hasPermission('invoices_out.write');
    final canOpenInvoices = widget.api.hasPermission('invoices_out.read');
    final canConvertSalesOrders =
        widget.api.hasPermission('sales_orders.write');
    final canOpenSalesOrders = widget.api.hasPermission('sales_orders.read');
    final visibleImports =
        (_quoteImportsExpanded ? _quoteImports : _quoteImports.take(3))
            .cast<Map>()
            .map((item) {
      return item.cast<String, dynamic>();
    }).toList();
    final visibleApprovalReworkItems = (_approvalReworkExpanded
            ? _approvalReworkItems
            : _approvalReworkItems.take(3))
        .cast<Map>()
        .map((item) {
      return item.cast<String, dynamic>();
    }).toList();
    final visibleApprovalRequestItems = (_approvalRequestsExpanded
            ? _approvalRequestItems
            : _approvalRequestItems.take(3))
        .cast<Map>()
        .map((item) {
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
                        color: _approvalRequestItems.isEmpty
                            ? null
                            : Colors.amber.shade50,
                        child: Padding(
                          padding: const EdgeInsets.all(12),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                children: [
                                  Expanded(
                                    child: Text(
                                      'Freigabeanforderungen (${_approvalRequestItems.length})',
                                      style: Theme.of(context)
                                          .textTheme
                                          .titleMedium,
                                    ),
                                  ),
                                  IconButton(
                                    tooltip:
                                        'Freigabeanforderungen aktualisieren',
                                    onPressed: _approvalRequestsLoading
                                        ? null
                                        : _loadApprovalRequestQueue,
                                    icon: const Icon(Icons.refresh_rounded),
                                  ),
                                ],
                              ),
                              if (_approvalRequestsLoading)
                                const Padding(
                                  padding: EdgeInsets.symmetric(vertical: 8),
                                  child: Center(
                                    child: CircularProgressIndicator(),
                                  ),
                                )
                              else if (visibleApprovalRequestItems.isEmpty)
                                const Text('Keine offene Freigabeanforderung.')
                              else ...[
                                ...visibleApprovalRequestItems.map((item) {
                                  final actionId =
                                      _approvalRequestQueueActionId(item);
                                  final approving =
                                      _approvingApprovalRequestQueueItemId ==
                                          actionId;
                                  final rejecting =
                                      _rejectingApprovalRequestQueueItemId ==
                                          actionId;
                                  final actionRunning = approving || rejecting;
                                  final position =
                                      (item['position'] as num?)?.toInt();
                                  final description =
                                      (item['description'] ?? 'Position')
                                          .toString();
                                  final contextLines = <String>[
                                    description.trim().isEmpty
                                        ? 'Position'
                                        : description,
                                    'Grund: ${_approvalRequestReasonLabel(item)}',
                                  ];
                                  final requestContext =
                                      _approvalRequestContext(item);
                                  if (requestContext.isNotEmpty) {
                                    contextLines.add(requestContext);
                                  }
                                  final currentTargetContext =
                                      _approvalReworkCurrentTargetContext(item);
                                  if (currentTargetContext.isNotEmpty) {
                                    contextLines.add(currentTargetContext);
                                  }
                                  return ListTile(
                                    dense: true,
                                    contentPadding: EdgeInsets.zero,
                                    title: Text(
                                      '${(item['quote_number'] ?? 'Angebot').toString()} · Pos. ${position ?? '-'}',
                                    ),
                                    subtitle: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: contextLines
                                          .take(4)
                                          .map(
                                            (line) => Text(
                                              line,
                                              maxLines: 1,
                                              overflow: TextOverflow.ellipsis,
                                            ),
                                          )
                                          .toList(),
                                    ),
                                    trailing: canApproveQuotes
                                        ? Wrap(
                                            spacing: 2,
                                            children: [
                                              IconButton(
                                                tooltip: 'Freigabe genehmigen',
                                                onPressed: actionRunning
                                                    ? null
                                                    : () =>
                                                        _approveApprovalRequestQueueItem(
                                                            item),
                                                icon: approving
                                                    ? const SizedBox(
                                                        height: 18,
                                                        width: 18,
                                                        child:
                                                            CircularProgressIndicator(
                                                          strokeWidth: 2,
                                                        ),
                                                      )
                                                    : const Icon(Icons
                                                        .check_circle_outline_rounded),
                                              ),
                                              IconButton(
                                                tooltip: 'Freigabe ablehnen',
                                                onPressed: actionRunning
                                                    ? null
                                                    : () =>
                                                        _rejectApprovalRequestQueueItem(
                                                            item),
                                                icon: rejecting
                                                    ? const SizedBox(
                                                        height: 18,
                                                        width: 18,
                                                        child:
                                                            CircularProgressIndicator(
                                                          strokeWidth: 2,
                                                        ),
                                                      )
                                                    : const Icon(Icons
                                                        .highlight_off_rounded),
                                              ),
                                              IconButton(
                                                tooltip: canWrite &&
                                                        (item['quote_status'] ??
                                                                    '')
                                                                .toString() ==
                                                            'draft'
                                                    ? 'Position bearbeiten'
                                                    : 'Position anzeigen',
                                                onPressed: actionRunning
                                                    ? null
                                                    : () =>
                                                        _openApprovalRequestItem(
                                                            item),
                                                icon: const Icon(
                                                    Icons.open_in_new_rounded),
                                              ),
                                            ],
                                          )
                                        : TextButton.icon(
                                            onPressed: () =>
                                                _openApprovalRequestItem(item),
                                            icon: const Icon(
                                                Icons.open_in_new_rounded),
                                            label: Text(canWrite &&
                                                    (item['quote_status'] ?? '')
                                                            .toString() ==
                                                        'draft'
                                                ? 'Bearbeiten'
                                                : 'Anzeigen'),
                                          ),
                                  );
                                }),
                                if (_approvalRequestItems.length > 3)
                                  TextButton(
                                    onPressed: () => setState(() {
                                      _approvalRequestsExpanded =
                                          !_approvalRequestsExpanded;
                                    }),
                                    child: Text(_approvalRequestsExpanded
                                        ? 'Weniger anzeigen'
                                        : 'Alle anzeigen'),
                                  ),
                              ],
                            ],
                          ),
                        ),
                      ),
                    ),
                    const Divider(height: 1),
                    Padding(
                      padding: const EdgeInsets.all(12),
                      child: Card(
                        margin: EdgeInsets.zero,
                        color: _approvalReworkItems.isEmpty
                            ? null
                            : Colors.red.shade50,
                        child: Padding(
                          padding: const EdgeInsets.all(12),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                children: [
                                  Expanded(
                                    child: Text(
                                      'Nacharbeits-Queue (${_approvalReworkItems.length})',
                                      style: Theme.of(context)
                                          .textTheme
                                          .titleMedium,
                                    ),
                                  ),
                                  IconButton(
                                    tooltip: 'Nacharbeits-Queue aktualisieren',
                                    onPressed: _approvalReworkLoading
                                        ? null
                                        : _loadApprovalReworkQueue,
                                    icon: const Icon(Icons.refresh_rounded),
                                  ),
                                ],
                              ),
                              if (_approvalReworkLoading)
                                const Padding(
                                  padding: EdgeInsets.symmetric(vertical: 8),
                                  child: Center(
                                    child: CircularProgressIndicator(),
                                  ),
                                )
                              else if (visibleApprovalReworkItems.isEmpty)
                                const Text('Keine offene Nacharbeit.')
                              else ...[
                                ...visibleApprovalReworkItems.map((item) {
                                  final position =
                                      (item['position'] as num?)?.toInt();
                                  final description =
                                      (item['description'] ?? 'Position')
                                          .toString();
                                  final contextLines = <String>[
                                    description.trim().isEmpty
                                        ? 'Position'
                                        : description,
                                    'Grund: ${_approvalReworkReasonLabel(item)}',
                                  ];
                                  final decisionContext =
                                      _approvalReworkDecisionContext(item);
                                  if (decisionContext.isNotEmpty) {
                                    contextLines.add(decisionContext);
                                  }
                                  final currentTargetContext =
                                      _approvalReworkCurrentTargetContext(item);
                                  if (currentTargetContext.isNotEmpty) {
                                    contextLines.add(currentTargetContext);
                                  }
                                  return ListTile(
                                    dense: true,
                                    contentPadding: EdgeInsets.zero,
                                    title: Text(
                                      '${(item['quote_number'] ?? 'Angebot').toString()} · Pos. ${position ?? '-'}',
                                    ),
                                    subtitle: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: contextLines
                                          .take(4)
                                          .map(
                                            (line) => Text(
                                              line,
                                              maxLines: 1,
                                              overflow: TextOverflow.ellipsis,
                                            ),
                                          )
                                          .toList(),
                                    ),
                                    trailing: TextButton.icon(
                                      onPressed: () =>
                                          _openApprovalReworkItem(item),
                                      icon:
                                          const Icon(Icons.open_in_new_rounded),
                                      label: Text(canWrite &&
                                              (item['quote_status'] ?? '')
                                                      .toString() ==
                                                  'draft'
                                          ? 'Bearbeiten'
                                          : 'Anzeigen'),
                                    ),
                                  );
                                }),
                                if (_approvalReworkItems.length > 3)
                                  TextButton(
                                    onPressed: () => setState(() {
                                      _approvalReworkExpanded =
                                          !_approvalReworkExpanded;
                                    }),
                                    child: Text(_approvalReworkExpanded
                                        ? 'Weniger anzeigen'
                                        : 'Alle anzeigen'),
                                  ),
                              ],
                            ],
                          ),
                        ),
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
                                  final importStatus =
                                      (item['status'] ?? '').toString();
                                  return ListTile(
                                    dense: true,
                                    contentPadding: EdgeInsets.zero,
                                    title: Text((item['source_filename'] ??
                                            'GAEB-Import')
                                        .toString()),
                                    subtitle: Text(
                                      '${(item['status'] ?? '-').toString()}  •  ${_formatDateTime(item['uploaded_at'])}',
                                    ),
                                    trailing: Wrap(
                                      children: [
                                        if (canWrite &&
                                            importStatus == 'uploaded')
                                          TextButton(
                                            onPressed: importId.isEmpty
                                                ? null
                                                : () => _processGAEBImport(
                                                    importId),
                                            child: const Text('Verarbeiten'),
                                          ),
                                        TextButton(
                                          onPressed: importId.isEmpty
                                              ? null
                                              : () => _openQuoteImportDetail(
                                                  importId),
                                          child: const Text('Details'),
                                        ),
                                      ],
                                    ),
                                  );
                                }),
                                if (_quoteImports.length > 3)
                                  TextButton(
                                    onPressed: () => setState(() {
                                      _quoteImportsExpanded =
                                          !_quoteImportsExpanded;
                                    }),
                                    child: Text(_quoteImportsExpanded
                                        ? 'Weniger anzeigen'
                                        : 'Alle anzeigen'),
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
                              if (rejectedApprovalDecisionCount > 0) ...[
                                Container(
                                  width: double.infinity,
                                  padding: const EdgeInsets.all(12),
                                  decoration: BoxDecoration(
                                    color: Colors.red.shade50,
                                    border:
                                        Border.all(color: Colors.red.shade200),
                                    borderRadius: BorderRadius.circular(6),
                                  ),
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        rejectedApprovalDecisionCount == 1
                                            ? 'Nacharbeit offen: 1 Position'
                                            : 'Nacharbeit offen: $rejectedApprovalDecisionCount Positionen',
                                        style: TextStyle(
                                          fontWeight: FontWeight.w700,
                                          color: Colors.red.shade800,
                                        ),
                                      ),
                                      if (rejectedApprovalDecisionPositionText
                                          .isNotEmpty) ...[
                                        const SizedBox(height: 4),
                                        Text(
                                          rejectedApprovalDecisionPositionText,
                                          style: TextStyle(
                                            color: Colors.red.shade800,
                                          ),
                                        ),
                                      ],
                                      const SizedBox(height: 8),
                                      TextButton.icon(
                                        onPressed:
                                            _scrollToFirstRejectedApprovalDecisionPosition,
                                        icon: const Icon(
                                            Icons.arrow_downward_rounded),
                                        label:
                                            const Text('Zur ersten Position'),
                                      ),
                                      const SizedBox(height: 4),
                                      Text(
                                        rejectedApprovalDecisionCount == 1
                                            ? 'Eine Position wurde zuletzt abgelehnt. Position pruefen, Preis/Material/Zielpreis nacharbeiten und erneut Freigabe anfordern.'
                                            : 'Mehrere Positionen wurden zuletzt abgelehnt. Positionen pruefen, Preis/Material/Zielpreis nacharbeiten und erneut Freigabe anfordern.',
                                        style: TextStyle(
                                          color: Colors.grey.shade800,
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                                const SizedBox(height: 12),
                              ],
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
                                  final position = index + 1;
                                  final isHighlighted =
                                      _highlightedRejectedApprovalPosition ==
                                          position;
                                  final rejectedApprovalDecisionSummary =
                                      _quoteItemRejectedApprovalDecisionSummary(
                                          item);
                                  final canEditRejectedPosition = canWrite &&
                                      selectedStatus == 'draft' &&
                                      rejectedApprovalDecisionSummary
                                          .isNotEmpty;
                                  return KeyedSubtree(
                                    key: _quoteItemJumpKey(position),
                                    child: ListTile(
                                      tileColor: isHighlighted
                                          ? Colors.red.shade50
                                          : null,
                                      title: Text(
                                          (item['description'] ?? 'Position')
                                              .toString()),
                                      subtitle: Column(
                                        crossAxisAlignment:
                                            CrossAxisAlignment.start,
                                        children: [
                                          Text(
                                            'Menge ${qty.toStringAsFixed(qty.truncateToDouble() == qty ? 0 : 2)} ${(item['unit'] ?? '')}  •  Einzelpreis ${_formatMoney(unitPrice, currency)}  •  Steuer ${(item['tax_code'] ?? '').toString()}',
                                          ),
                                          if (rejectedApprovalDecisionSummary
                                              .isNotEmpty) ...[
                                            const SizedBox(height: 4),
                                            Text(
                                              rejectedApprovalDecisionSummary,
                                              style: TextStyle(
                                                color: Colors.red.shade800,
                                                fontWeight: FontWeight.w600,
                                              ),
                                            ),
                                          ],
                                        ],
                                      ),
                                      trailing: Column(
                                        mainAxisSize: MainAxisSize.min,
                                        crossAxisAlignment:
                                            CrossAxisAlignment.end,
                                        children: [
                                          Text(_formatMoney(lineNet, currency)),
                                          if (canEditRejectedPosition) ...[
                                            const SizedBox(height: 4),
                                            TextButton.icon(
                                              onPressed: () => _openEditDialog(
                                                initialFocusItemId:
                                                    (item['id'] ?? '')
                                                        .toString(),
                                                initialFocusPosition: position,
                                              ),
                                              icon: const Icon(
                                                  Icons.edit_rounded),
                                              label: const Text('Bearbeiten'),
                                            ),
                                          ],
                                        ],
                                      ),
                                    ),
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

class _QuoteApprovalDecisionCommentDialog extends StatefulWidget {
  const _QuoteApprovalDecisionCommentDialog({
    required this.title,
    required this.actionLabel,
    required this.helperText,
    this.contextSummary,
  });

  final String title;
  final String actionLabel;
  final String helperText;
  final Widget? contextSummary;

  @override
  State<_QuoteApprovalDecisionCommentDialog> createState() =>
      _QuoteApprovalDecisionCommentDialogState();
}

class _QuoteApprovalDecisionCommentDialogState
    extends State<_QuoteApprovalDecisionCommentDialog> {
  final _commentCtrl = TextEditingController();

  @override
  void dispose() {
    _commentCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.title),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (widget.contextSummary != null) ...[
                widget.contextSummary!,
                const SizedBox(height: 12),
              ],
              TextField(
                controller: _commentCtrl,
                decoration: InputDecoration(
                  labelText: 'Kommentar',
                  helperText: widget.helperText,
                ),
                maxLines: 4,
                maxLength: 500,
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(null),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(_commentCtrl.text.trim()),
          child: Text(widget.actionLabel),
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
    this.initialFocusItemId,
    this.initialFocusPosition,
  });

  final ApiClient api;
  final Map<String, dynamic>? initial;
  final CommercialFilterContext? initialFilters;
  final String? initialFocusItemId;
  final int? initialFocusPosition;

  @override
  State<_QuoteEditorDialog> createState() => _QuoteEditorDialogState();
}

class _QuoteEditorDialogState extends State<_QuoteEditorDialog> {
  late final TextEditingController _projectCtrl;
  late final TextEditingController _contactCtrl;
  late final TextEditingController _currencyCtrl;
  late final TextEditingController _noteCtrl;
  late final List<_QuoteItemDraft> _items;
  final Map<String, GlobalKey> _itemFocusKeys = {};
  final Map<int, GlobalKey> _positionFocusKeys = {};
  String? _highlightedFocusItemId;
  int? _highlightedFocusPosition;
  bool _saving = false;
  String? _applyingCandidateItemId;
  String? _searchingMaterialItemId;
  String? _applyingSearchResultItemId;
  String? _loadingPriceSuggestionItemId;
  String? _loadingPriceHistoryItemId;
  String? _loadingPriceSourcePriorityItemId;
  String? _loadingPriceEvaluationItemId;
  String? _loadingPriceDecisionTransparencyItemId;
  String? _loadingPriceDecisionHistoryItemId;
  String? _loadingMarginAnchorItemId;
  String? _loadingApprovalHintItemId;
  String? _loadingApprovalRequestsItemId;
  String? _loadingTargetMarginAnchorItemId;
  String? _applyingPriceSuggestionItemId;
  String? _applyingPrimaryPriceSourceItemId;
  String? _applyingTargetPriceItemId;
  String? _requestingApprovalItemId;
  String? _cancellingApprovalItemId;
  String? _approvingApprovalItemId;
  String? _rejectingApprovalItemId;
  String? _resolvingApprovalReworkItemId;

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
    if (_isEdit &&
        ((widget.initialFocusItemId ?? '').trim().isNotEmpty ||
            widget.initialFocusPosition != null)) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        _focusInitialItem();
      });
    }
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

  GlobalKey _itemFocusKey(_QuoteItemDraft item, int position) {
    final itemId = item.id.trim();
    if (itemId.isNotEmpty) {
      return _itemFocusKeys.putIfAbsent(itemId, GlobalKey.new);
    }
    return _positionFocusKeys.putIfAbsent(position, GlobalKey.new);
  }

  bool _isFocusedItem(_QuoteItemDraft item, int position) {
    final highlightedItemId = _highlightedFocusItemId?.trim() ?? '';
    if (highlightedItemId.isNotEmpty && item.id.trim() == highlightedItemId) {
      return true;
    }
    return _highlightedFocusPosition == position;
  }

  void _focusInitialItem() {
    if (!_isEdit || _items.isEmpty) return;
    final requestedItemId = (widget.initialFocusItemId ?? '').trim();
    var targetIndex = -1;
    if (requestedItemId.isNotEmpty) {
      targetIndex =
          _items.indexWhere((item) => item.id.trim() == requestedItemId);
    }
    if (targetIndex < 0) {
      final requestedPosition = widget.initialFocusPosition;
      if (requestedPosition == null ||
          requestedPosition < 1 ||
          requestedPosition > _items.length) {
        return;
      }
      targetIndex = requestedPosition - 1;
    }
    final targetPosition = targetIndex + 1;
    final targetItem = _items[targetIndex];
    final targetContext =
        _itemFocusKey(targetItem, targetPosition).currentContext;
    if (targetContext == null) return;
    setState(() {
      _highlightedFocusItemId =
          targetItem.id.trim().isEmpty ? null : targetItem.id.trim();
      _highlightedFocusPosition = targetPosition;
    });
    Scrollable.ensureVisible(
      targetContext,
      duration: const Duration(milliseconds: 300),
      curve: Curves.easeInOut,
      alignment: 0.08,
    );
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

  Future<void> _searchMaterials(_QuoteItemDraft item, String query) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _searchingMaterialItemId = itemId);
    try {
      final results = await widget.api.searchQuoteItemMaterials(
        quoteId,
        itemId,
        query: query,
      );
      if (!mounted) return;
      setState(() {
        item.materialSearchPerformed = true;
        item.materialSearchResults = results
            .whereType<Map>()
            .map((entry) => _QuoteMaterialCandidateDraft.fromJson(
                entry.cast<String, dynamic>()))
            .toList();
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.materialSearchPerformed = true;
        item.materialSearchResults = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
              _quoteErrorMessage(e, fallback: 'Materialsuche fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _searchingMaterialItemId = null);
      }
    }
  }

  Future<void> _applyMaterialSearchResult(
    _QuoteItemDraft item,
    _QuoteMaterialCandidateDraft candidate,
  ) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    final query = item.materialSearchCtrl.text.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _applyingSearchResultItemId = itemId);
    try {
      final updated = await widget.api.applyQuoteMaterialSearchResult(
        quoteId,
        itemId,
        query: query,
        materialId: candidate.materialId,
      );
      if (!mounted) return;
      setState(() {
        _replaceDraftFromQuote(updated);
      });
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Suchtreffer-Uebernahme fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _applyingSearchResultItemId = null);
      }
    }
  }

  Future<void> _loadPriceSuggestion(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingPriceSuggestionItemId = itemId);
    try {
      final suggestion = await widget.api.getQuoteItemPriceSuggestion(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.priceSuggestionPerformed = true;
        item.priceSuggestion = _QuotePriceSuggestionDraft.fromJson(suggestion);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.priceSuggestionPerformed = true;
        item.priceSuggestion = null;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Preisvorschlag konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingPriceSuggestionItemId = null);
      }
    }
  }

  Future<void> _loadPriceHistory(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingPriceHistoryItemId = itemId);
    try {
      final history = await widget.api.getQuoteItemPriceHistory(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.priceHistoryPerformed = true;
        item.priceHistoryEntries = history
            .whereType<Map>()
            .map((entry) => _QuotePriceHistoryEntryDraft.fromJson(
                entry.cast<String, dynamic>()))
            .toList();
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.priceHistoryPerformed = true;
        item.priceHistoryEntries = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Preisquellen konnten nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingPriceHistoryItemId = null);
      }
    }
  }

  Future<void> _loadPriceSourcePriority(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingPriceSourcePriorityItemId = itemId);
    try {
      final priority = await widget.api.getQuoteItemPriceSourcePriority(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.priceSourcePriorityPerformed = true;
        item.priceSourcePriorityEntries = priority
            .whereType<Map>()
            .map((entry) => _QuotePriceSourcePriorityEntryDraft.fromJson(
                entry.cast<String, dynamic>()))
            .toList();
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.priceSourcePriorityPerformed = true;
        item.priceSourcePriorityEntries = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Quellen-Priorisierung konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingPriceSourcePriorityItemId = null);
      }
    }
  }

  Future<void> _loadPriceEvaluation(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingPriceEvaluationItemId = itemId);
    try {
      final evaluation = await widget.api.getQuoteItemPriceEvaluation(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.priceEvaluationPerformed = true;
        item.priceEvaluation = _QuotePriceEvaluationDraft.fromJson(evaluation);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.priceEvaluationPerformed = true;
        item.priceEvaluation = null;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Preisbewertung konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingPriceEvaluationItemId = null);
      }
    }
  }

  Future<void> _loadPriceDecisionTransparency(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingPriceDecisionTransparencyItemId = itemId);
    try {
      final transparency =
          await widget.api.getQuoteItemPriceDecisionTransparency(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.priceDecisionTransparencyPerformed = true;
        item.priceDecisionTransparency =
            _QuotePriceDecisionTransparencyDraft.fromJson(transparency);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.priceDecisionTransparencyPerformed = true;
        item.priceDecisionTransparency = null;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback:
                  'Preisentscheidungs-Transparenz konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingPriceDecisionTransparencyItemId = null);
      }
    }
  }

  Future<void> _loadPriceDecisionHistory(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingPriceDecisionHistoryItemId = itemId);
    try {
      final history = await widget.api.getQuoteItemPriceDecisionHistory(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.priceDecisionHistoryPerformed = true;
        item.priceDecisionHistoryEntries = history
            .whereType<Map>()
            .map((entry) => _QuotePriceDecisionHistoryEntryDraft.fromJson(
                entry.cast<String, dynamic>()))
            .toList();
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.priceDecisionHistoryPerformed = true;
        item.priceDecisionHistoryEntries = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Preisentscheidungen konnten nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingPriceDecisionHistoryItemId = null);
      }
    }
  }

  Future<void> _loadMarginAnchor(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingMarginAnchorItemId = itemId);
    try {
      final marginAnchor = await widget.api.getQuoteItemMarginAnchor(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.marginAnchorPerformed = true;
        item.marginAnchor = _QuoteItemMarginAnchorDraft.fromJson(marginAnchor);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.marginAnchorPerformed = true;
        item.marginAnchor = null;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Marge konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingMarginAnchorItemId = null);
      }
    }
  }

  Future<void> _loadApprovalHint(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingApprovalHintItemId = itemId);
    try {
      final approvalHint = await widget.api.getQuoteItemApprovalHint(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.approvalHintPerformed = true;
        item.approvalHint = _QuoteItemApprovalHintDraft.fromJson(approvalHint);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.approvalHintPerformed = true;
        item.approvalHint = null;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Freigabehinweis konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingApprovalHintItemId = null);
      }
    }
  }

  Future<void> _loadApprovalRequests(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingApprovalRequestsItemId = itemId);
    try {
      final history = await widget.api.getQuoteItemApprovalRequests(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.approvalRequestsPerformed = true;
        item.approvalRequests = history
            .whereType<Map>()
            .map((entry) => _QuoteItemApprovalRequestDraft.fromJson(
                entry.cast<String, dynamic>()))
            .toList();
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.approvalRequestsPerformed = true;
        item.approvalRequests = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Freigabehistorie konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingApprovalRequestsItemId = null);
      }
    }
  }

  Future<void> _loadTargetMarginAnchor(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _loadingTargetMarginAnchorItemId = itemId);
    try {
      final targetMarginAnchor =
          await widget.api.getQuoteItemTargetMarginAnchor(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.targetMarginAnchorPerformed = true;
        item.targetMarginAnchor =
            _QuoteItemTargetMarginAnchorDraft.fromJson(targetMarginAnchor);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        item.targetMarginAnchorPerformed = true;
        item.targetMarginAnchor = null;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Zielmarge konnte nicht geladen werden')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _loadingTargetMarginAnchorItemId = null);
      }
    }
  }

  Future<void> _applyPriceSuggestion(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _applyingPriceSuggestionItemId = itemId);
    try {
      final updated = await widget.api.applyQuoteItemPriceSuggestion(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        _replaceDraftFromQuote(updated);
      });
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Preisuebernahme fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _applyingPriceSuggestionItemId = null);
      }
    }
  }

  Future<void> _applyPrimaryPriceSource(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _applyingPrimaryPriceSourceItemId = itemId);
    try {
      final updated = await widget.api.applyQuoteItemPrimaryPriceSource(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        _replaceDraftFromQuote(updated);
      });
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Primaerpreis-Uebernahme fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _applyingPrimaryPriceSourceItemId = null);
      }
    }
  }

  Future<void> _applyTargetPrice(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _applyingTargetPriceItemId = itemId);
    try {
      final updated = await widget.api.applyQuoteItemTargetPrice(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        _replaceDraftFromQuote(updated);
      });
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Zielpreis-Uebernahme fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _applyingTargetPriceItemId = null);
      }
    }
  }

  Future<void> _requestApproval(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    setState(() => _requestingApprovalItemId = itemId);
    try {
      final created = await widget.api.requestQuoteItemApproval(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.approvalRequest = _QuoteItemApprovalRequestDraft.fromJson(created);
        item.approvalRequestsPerformed = false;
        item.approvalRequests = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Freigabe wurde angefordert')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Freigabeanforderung fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _requestingApprovalItemId = null);
      }
    }
  }

  Future<void> _cancelApproval(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty || item.approvalRequest == null) {
      return;
    }
    setState(() => _cancellingApprovalItemId = itemId);
    try {
      await widget.api.cancelQuoteItemApprovalRequest(
        quoteId,
        itemId,
      );
      if (!mounted) return;
      setState(() {
        item.approvalRequest = null;
        item.approvalRequestsPerformed = false;
        item.approvalRequests = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Freigabeanforderung storniert')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Storno der Freigabeanforderung fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _cancellingApprovalItemId = null);
      }
    }
  }

  Future<String?> _promptApprovalDecisionComment({
    required String title,
    required String actionLabel,
    required String helperText,
  }) {
    return showDialog<String>(
      context: context,
      builder: (context) => _QuoteApprovalDecisionCommentDialog(
        title: title,
        actionLabel: actionLabel,
        helperText: helperText,
      ),
    );
  }

  Future<void> _approveApproval(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty || item.approvalRequest == null) {
      return;
    }
    final comment = await _promptApprovalDecisionComment(
      title: 'Freigabe genehmigen',
      actionLabel: 'Genehmigen',
      helperText: 'Optionaler Kommentar fuer die Freigabehistorie',
    );
    if (!mounted || comment == null) return;
    setState(() => _approvingApprovalItemId = itemId);
    try {
      await widget.api.approveQuoteItemApprovalRequest(
        quoteId,
        itemId,
        comment: comment,
      );
      if (!mounted) return;
      setState(() {
        item.approvalRequest = null;
        item.approvalRequestsPerformed = false;
        item.approvalRequests = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Freigabe genehmigt')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Genehmigung der Freigabeanforderung fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _approvingApprovalItemId = null);
      }
    }
  }

  Future<void> _rejectApproval(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty || item.approvalRequest == null) {
      return;
    }
    final comment = await _promptApprovalDecisionComment(
      title: 'Freigabe ablehnen',
      actionLabel: 'Ablehnen',
      helperText: 'Optionaler Kommentar fuer die Freigabehistorie',
    );
    if (!mounted || comment == null) return;
    setState(() => _rejectingApprovalItemId = itemId);
    try {
      await widget.api.rejectQuoteItemApprovalRequest(
        quoteId,
        itemId,
        comment: comment,
      );
      if (!mounted) return;
      setState(() {
        item.approvalRequest = null;
        item.approvalRequestsPerformed = false;
        item.approvalRequests = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Freigabe abgelehnt')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Ablehnung der Freigabeanforderung fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _rejectingApprovalItemId = null);
      }
    }
  }

  Future<void> _resolveApprovalRework(_QuoteItemDraft item) async {
    final quoteId = widget.initial?['id']?.toString().trim() ?? '';
    final itemId = item.id.trim();
    if (quoteId.isEmpty || itemId.isEmpty) return;
    final comment = await _promptApprovalDecisionComment(
      title: 'Nacharbeit abschliessen',
      actionLabel: 'Abschliessen',
      helperText: 'Optionaler Kommentar fuer die erledigte Nacharbeit',
    );
    if (!mounted || comment == null) return;
    setState(() => _resolvingApprovalReworkItemId = itemId);
    try {
      await widget.api.resolveQuoteItemApprovalRework(
        quoteId,
        itemId,
        comment: comment,
      );
      final updated = await widget.api.getQuote(quoteId);
      if (!mounted) return;
      setState(() {
        _replaceDraftFromQuote(updated);
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Nacharbeit abgeschlossen')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_quoteErrorMessage(e,
              fallback: 'Abschluss der Nacharbeit fehlgeschlagen')),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _resolvingApprovalReworkItemId = null);
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
                KeyedSubtree(
                  key: _itemFocusKey(_items[i], i + 1),
                  child: _QuoteItemRow(
                    key: ValueKey(_items[i]),
                    item: _items[i],
                    index: i,
                    highlighted: _isFocusedItem(_items[i], i + 1),
                    canSearchMaterial: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    searchingMaterial:
                        _searchingMaterialItemId == _items[i].id.trim(),
                    applyingSearchResult:
                        _applyingSearchResultItemId == _items[i].id.trim(),
                    canLoadPriceSuggestion: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingPriceSuggestion:
                        _loadingPriceSuggestionItemId == _items[i].id.trim(),
                    onLoadPriceSuggestion: () =>
                        _loadPriceSuggestion(_items[i]),
                    canLoadPriceHistory: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingPriceHistory:
                        _loadingPriceHistoryItemId == _items[i].id.trim(),
                    onLoadPriceHistory: () => _loadPriceHistory(_items[i]),
                    canLoadPriceSourcePriority: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingPriceSourcePriority:
                        _loadingPriceSourcePriorityItemId ==
                            _items[i].id.trim(),
                    onLoadPriceSourcePriority: () =>
                        _loadPriceSourcePriority(_items[i]),
                    canLoadPriceEvaluation: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingPriceEvaluation:
                        _loadingPriceEvaluationItemId == _items[i].id.trim(),
                    onLoadPriceEvaluation: () =>
                        _loadPriceEvaluation(_items[i]),
                    canLoadPriceDecisionTransparency: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingPriceDecisionTransparency:
                        _loadingPriceDecisionTransparencyItemId ==
                            _items[i].id.trim(),
                    onLoadPriceDecisionTransparency: () =>
                        _loadPriceDecisionTransparency(_items[i]),
                    canLoadPriceDecisionHistory: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingPriceDecisionHistory:
                        _loadingPriceDecisionHistoryItemId ==
                            _items[i].id.trim(),
                    onLoadPriceDecisionHistory: () =>
                        _loadPriceDecisionHistory(_items[i]),
                    canLoadMarginAnchor: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingMarginAnchor:
                        _loadingMarginAnchorItemId == _items[i].id.trim(),
                    onLoadMarginAnchor: () => _loadMarginAnchor(_items[i]),
                    canLoadApprovalHint: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingApprovalHint:
                        _loadingApprovalHintItemId == _items[i].id.trim(),
                    onLoadApprovalHint: () => _loadApprovalHint(_items[i]),
                    canLoadApprovalRequests: _isEdit &&
                        widget.api.hasPermission('quotes.read') &&
                        !_saving,
                    loadingApprovalRequests:
                        _loadingApprovalRequestsItemId == _items[i].id.trim(),
                    onLoadApprovalRequests: () =>
                        _loadApprovalRequests(_items[i]),
                    canLoadTargetMarginAnchor: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    loadingTargetMarginAnchor:
                        _loadingTargetMarginAnchorItemId == _items[i].id.trim(),
                    onLoadTargetMarginAnchor: () =>
                        _loadTargetMarginAnchor(_items[i]),
                    onApplyPrimaryPriceSource: _items[i].priceEvaluation == null
                        ? null
                        : () => _applyPrimaryPriceSource(_items[i]),
                    applyingPrimaryPriceSource:
                        _applyingPrimaryPriceSourceItemId ==
                            _items[i].id.trim(),
                    onApplyTargetPrice: _items[i].targetMarginAnchor == null ||
                            _items[i].targetMarginAnchor!.targetUnitPrice ==
                                null
                        ? null
                        : () => _applyTargetPrice(_items[i]),
                    applyingTargetPrice:
                        _applyingTargetPriceItemId == _items[i].id.trim(),
                    onRequestApproval: _items[i].targetMarginAnchor == null ||
                            !_items[i].targetMarginAnchor!.canRequestApproval ||
                            _items[i].approvalRequest != null
                        ? null
                        : () => _requestApproval(_items[i]),
                    requestingApproval:
                        _requestingApprovalItemId == _items[i].id.trim(),
                    onCancelApproval: _items[i].approvalRequest == null
                        ? null
                        : () => _cancelApproval(_items[i]),
                    cancellingApproval:
                        _cancellingApprovalItemId == _items[i].id.trim(),
                    onApproveApproval: _items[i].approvalRequest == null ||
                            !widget.api.hasPermission('quotes.approve')
                        ? null
                        : () => _approveApproval(_items[i]),
                    approvingApproval:
                        _approvingApprovalItemId == _items[i].id.trim(),
                    onRejectApproval: _items[i].approvalRequest == null ||
                            !widget.api.hasPermission('quotes.approve')
                        ? null
                        : () => _rejectApproval(_items[i]),
                    rejectingApproval:
                        _rejectingApprovalItemId == _items[i].id.trim(),
                    onResolveApprovalRework: !_items[i].canResolveRework ||
                            !widget.api.hasPermission('quotes.approve')
                        ? null
                        : () => _resolveApprovalRework(_items[i]),
                    resolvingApprovalRework:
                        _resolvingApprovalReworkItemId == _items[i].id.trim(),
                    onApplyPriceSuggestion: _items[i].priceSuggestion == null
                        ? null
                        : () => _applyPriceSuggestion(_items[i]),
                    applyingPriceSuggestion:
                        _applyingPriceSuggestionItemId == _items[i].id.trim(),
                    onSearchMaterial: (query) =>
                        _searchMaterials(_items[i], query),
                    onApplySearchResult: (candidate) =>
                        _applyMaterialSearchResult(_items[i], candidate),
                    canApplyCandidate: _isEdit &&
                        widget.api.hasPermission('quotes.write') &&
                        !_saving,
                    applyingCandidate:
                        _applyingCandidateItemId == _items[i].id.trim(),
                    onApplyCandidate: (candidate) =>
                        _applyMaterialCandidate(_items[i], candidate),
                    onCommercialFieldsChanged: () {
                      setState(() {
                        _items[i].invalidateCommercialAnchors();
                      });
                    },
                    onRemove: _items.length == 1
                        ? null
                        : () {
                            setState(() {
                              _items[i].invalidateCommercialAnchors();
                              _items.removeAt(i).dispose();
                            });
                          },
                  ),
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
    String materialSearchQuery = '',
    List<_QuoteMaterialCandidateDraft> materialSearchResults = const [],
    bool materialSearchPerformed = false,
    _QuotePriceSuggestionDraft? priceSuggestion,
    bool priceSuggestionPerformed = false,
    List<_QuotePriceHistoryEntryDraft> priceHistoryEntries = const [],
    bool priceHistoryPerformed = false,
    List<_QuotePriceSourcePriorityEntryDraft> priceSourcePriorityEntries =
        const [],
    bool priceSourcePriorityPerformed = false,
    _QuotePriceEvaluationDraft? priceEvaluation,
    bool priceEvaluationPerformed = false,
    _QuotePriceDecisionTransparencyDraft? priceDecisionTransparency,
    bool priceDecisionTransparencyPerformed = false,
    List<_QuotePriceDecisionHistoryEntryDraft> priceDecisionHistoryEntries =
        const [],
    bool priceDecisionHistoryPerformed = false,
    _QuoteItemMarginAnchorDraft? marginAnchor,
    Map<String, dynamic>? marginAnchorJson,
    bool marginAnchorPerformed = false,
    _QuoteItemApprovalHintDraft? approvalHint,
    Map<String, dynamic>? approvalHintJson,
    bool approvalHintPerformed = false,
    List<_QuoteItemApprovalRequestDraft> approvalRequests = const [],
    bool approvalRequestsPerformed = false,
    _QuoteItemTargetMarginAnchorDraft? targetMarginAnchor,
    Map<String, dynamic>? targetMarginAnchorJson,
    bool targetMarginAnchorPerformed = false,
    _QuoteItemApprovalRequestDraft? approvalRequest,
    _QuoteItemApprovalDecisionBadgeDraft? latestApprovalDecision,
  })  : id = id,
        descriptionCtrl = TextEditingController(text: description),
        qtyCtrl = TextEditingController(text: qty),
        unitCtrl = TextEditingController(text: unit),
        unitPriceCtrl = TextEditingController(text: unitPrice),
        taxCodeCtrl = TextEditingController(text: taxCode),
        materialIdCtrl = TextEditingController(text: materialId),
        materialSearchCtrl = TextEditingController(text: materialSearchQuery),
        priceMappingStatus = _normalizePriceMappingStatus(priceMappingStatus),
        materialCandidateStatus =
            _normalizeMaterialCandidateStatus(materialCandidateStatus),
        materialCandidates =
            List<_QuoteMaterialCandidateDraft>.unmodifiable(materialCandidates),
        materialSearchResults = List<_QuoteMaterialCandidateDraft>.from(
          materialSearchResults,
        ),
        materialSearchPerformed = materialSearchPerformed,
        priceSuggestion = priceSuggestion,
        priceSuggestionPerformed = priceSuggestionPerformed,
        priceHistoryEntries = List<_QuotePriceHistoryEntryDraft>.from(
          priceHistoryEntries,
        ),
        priceHistoryPerformed = priceHistoryPerformed,
        priceSourcePriorityEntries =
            List<_QuotePriceSourcePriorityEntryDraft>.from(
          priceSourcePriorityEntries,
        ),
        priceSourcePriorityPerformed = priceSourcePriorityPerformed,
        priceEvaluation = priceEvaluation,
        priceEvaluationPerformed = priceEvaluationPerformed,
        priceDecisionTransparency = priceDecisionTransparency,
        priceDecisionTransparencyPerformed = priceDecisionTransparencyPerformed,
        priceDecisionHistoryEntries =
            List<_QuotePriceDecisionHistoryEntryDraft>.from(
          priceDecisionHistoryEntries,
        ),
        priceDecisionHistoryPerformed = priceDecisionHistoryPerformed,
        marginAnchor = marginAnchor ??
            (marginAnchorJson == null
                ? null
                : _QuoteItemMarginAnchorDraft.fromJson(marginAnchorJson)),
        marginAnchorPerformed = marginAnchorPerformed,
        approvalHint = approvalHint ??
            (approvalHintJson == null
                ? null
                : _QuoteItemApprovalHintDraft.fromJson(approvalHintJson)),
        approvalHintPerformed = approvalHintPerformed,
        approvalRequests = List<_QuoteItemApprovalRequestDraft>.from(
          approvalRequests,
        ),
        approvalRequestsPerformed = approvalRequestsPerformed,
        targetMarginAnchor = targetMarginAnchor ??
            (targetMarginAnchorJson == null
                ? null
                : _QuoteItemTargetMarginAnchorDraft.fromJson(
                    targetMarginAnchorJson)),
        targetMarginAnchorPerformed = targetMarginAnchorPerformed,
        approvalRequest = approvalRequest,
        latestApprovalDecision = latestApprovalDecision;

  factory _QuoteItemDraft.fromJson(Map<String, dynamic> json) {
    final rawApprovalRequest = json['active_approval_request'];
    final rawLatestApprovalDecision = json['latest_approval_decision'];
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
      approvalRequest: rawApprovalRequest is Map
          ? _QuoteItemApprovalRequestDraft.fromJson(
              rawApprovalRequest.cast<String, dynamic>())
          : null,
      latestApprovalDecision: rawLatestApprovalDecision is Map
          ? _QuoteItemApprovalDecisionBadgeDraft.fromJson(
              rawLatestApprovalDecision.cast<String, dynamic>())
          : null,
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
  final TextEditingController materialSearchCtrl;
  String priceMappingStatus;
  final String materialCandidateStatus;
  final List<_QuoteMaterialCandidateDraft> materialCandidates;
  List<_QuoteMaterialCandidateDraft> materialSearchResults;
  bool materialSearchPerformed;
  _QuotePriceSuggestionDraft? priceSuggestion;
  bool priceSuggestionPerformed;
  List<_QuotePriceHistoryEntryDraft> priceHistoryEntries;
  bool priceHistoryPerformed;
  List<_QuotePriceSourcePriorityEntryDraft> priceSourcePriorityEntries;
  bool priceSourcePriorityPerformed;
  _QuotePriceEvaluationDraft? priceEvaluation;
  bool priceEvaluationPerformed;
  _QuotePriceDecisionTransparencyDraft? priceDecisionTransparency;
  bool priceDecisionTransparencyPerformed;
  List<_QuotePriceDecisionHistoryEntryDraft> priceDecisionHistoryEntries;
  bool priceDecisionHistoryPerformed;
  _QuoteItemMarginAnchorDraft? marginAnchor;
  bool marginAnchorPerformed;
  _QuoteItemApprovalHintDraft? approvalHint;
  bool approvalHintPerformed;
  List<_QuoteItemApprovalRequestDraft> approvalRequests;
  bool approvalRequestsPerformed;
  _QuoteItemTargetMarginAnchorDraft? targetMarginAnchor;
  bool targetMarginAnchorPerformed;
  _QuoteItemApprovalRequestDraft? approvalRequest;
  _QuoteItemApprovalDecisionBadgeDraft? latestApprovalDecision;

  bool get canResolveRework {
    if (approvalRequest != null) return false;
    if (latestApprovalDecision?.requiresRework != true) return false;
    final targetStatus = targetMarginAnchor?.targetStatus ?? '';
    return targetStatus == 'on_target' || targetStatus == 'above_target';
  }

  void invalidateCommercialAnchors() {
    targetMarginAnchor = null;
    targetMarginAnchorPerformed = false;
    approvalHint = null;
    approvalHintPerformed = false;
    marginAnchor = null;
    marginAnchorPerformed = false;
    priceEvaluation = null;
    priceEvaluationPerformed = false;
  }

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
    materialSearchCtrl.dispose();
  }
}

class _QuotePriceSuggestionDraft {
  const _QuotePriceSuggestionDraft({
    required this.materialId,
    required this.suggestedUnitPrice,
    required this.currency,
    required this.sourceLabel,
  });

  factory _QuotePriceSuggestionDraft.fromJson(Map<String, dynamic> json) {
    return _QuotePriceSuggestionDraft(
      materialId: (json['material_id'] ?? '').toString(),
      suggestedUnitPrice:
          ((json['suggested_unit_price'] as num?) ?? 0).toDouble(),
      currency: (json['currency'] ?? '').toString(),
      sourceLabel: (json['source_label'] ?? '').toString(),
    );
  }

  final String materialId;
  final double suggestedUnitPrice;
  final String currency;
  final String sourceLabel;

  String get displayPrice =>
      '${suggestedUnitPrice.toStringAsFixed(2)} ${currency.trim().isEmpty ? 'EUR' : currency}';
}

class _QuotePriceHistoryEntryDraft {
  const _QuotePriceHistoryEntryDraft({
    required this.sourceLabel,
    required this.unitPrice,
    required this.currency,
    required this.reference,
    required this.date,
  });

  factory _QuotePriceHistoryEntryDraft.fromJson(Map<String, dynamic> json) {
    return _QuotePriceHistoryEntryDraft(
      sourceLabel: (json['source_label'] ?? '').toString(),
      unitPrice: ((json['unit_price'] as num?) ?? 0).toDouble(),
      currency: (json['currency'] ?? '').toString(),
      reference: (json['reference'] ?? '').toString(),
      date: (json['date'] ?? '').toString(),
    );
  }

  final String sourceLabel;
  final double unitPrice;
  final String currency;
  final String reference;
  final String date;

  String get displayPrice =>
      '${unitPrice.toStringAsFixed(2)} ${currency.trim().isEmpty ? 'EUR' : currency}';

  String get displayMeta {
    final parts = <String>[];
    if (reference.trim().isNotEmpty) {
      parts.add(reference.trim());
    }
    if (date.trim().isNotEmpty) {
      parts.add(date.trim().split('T').first);
    }
    return parts.join('  •  ');
  }
}

class _QuotePriceSourcePriorityEntryDraft {
  const _QuotePriceSourcePriorityEntryDraft({
    required this.sourceLabel,
    required this.unitPrice,
    required this.currency,
    required this.reference,
    required this.date,
    required this.priorityRank,
    required this.priorityReason,
  });

  factory _QuotePriceSourcePriorityEntryDraft.fromJson(
      Map<String, dynamic> json) {
    return _QuotePriceSourcePriorityEntryDraft(
      sourceLabel: (json['source_label'] ?? '').toString(),
      unitPrice: ((json['unit_price'] as num?) ?? 0).toDouble(),
      currency: (json['currency'] ?? '').toString(),
      reference: (json['reference'] ?? '').toString(),
      date: (json['date'] ?? '').toString(),
      priorityRank: ((json['priority_rank'] as num?) ?? 0).toInt(),
      priorityReason: (json['priority_reason'] ?? '').toString(),
    );
  }

  final String sourceLabel;
  final double unitPrice;
  final String currency;
  final String reference;
  final String date;
  final int priorityRank;
  final String priorityReason;

  String get displayPrice =>
      '${unitPrice.toStringAsFixed(2)} ${currency.trim().isEmpty ? 'EUR' : currency}';

  String get displayPriority {
    switch (priorityRank) {
      case 1:
        return 'Primaer';
      case 2:
        return 'Sekundaer';
      default:
        return 'Rang $priorityRank';
    }
  }

  String get displayMeta {
    final parts = <String>[];
    if (priorityReason.trim().isNotEmpty) parts.add(priorityReason.trim());
    if (reference.trim().isNotEmpty) parts.add(reference.trim());
    if (date.trim().isNotEmpty) parts.add(date.trim());
    return parts.join('  •  ');
  }
}

class _QuotePriceEvaluationDraft {
  const _QuotePriceEvaluationDraft({
    required this.currentUnitPrice,
    required this.currency,
    required this.primarySourceLabel,
    required this.primarySourceUnitPrice,
    required this.primarySourceReference,
    required this.primarySourceDate,
    required this.absoluteDelta,
    required this.relativeDeltaPercent,
    required this.evaluationStatus,
    required this.evaluationReason,
  });

  factory _QuotePriceEvaluationDraft.fromJson(Map<String, dynamic> json) {
    final rawRelative = json['relative_delta_percent'];
    return _QuotePriceEvaluationDraft(
      currentUnitPrice: ((json['current_unit_price'] as num?) ?? 0).toDouble(),
      currency: (json['currency'] ?? '').toString(),
      primarySourceLabel: (json['primary_source_label'] ?? '').toString(),
      primarySourceUnitPrice:
          ((json['primary_source_unit_price'] as num?) ?? 0).toDouble(),
      primarySourceReference:
          (json['primary_source_reference'] ?? '').toString(),
      primarySourceDate: (json['primary_source_date'] ?? '').toString(),
      absoluteDelta: ((json['absolute_delta'] as num?) ?? 0).toDouble(),
      relativeDeltaPercent: rawRelative is num ? rawRelative.toDouble() : null,
      evaluationStatus: (json['evaluation_status'] ?? '').toString(),
      evaluationReason: (json['evaluation_reason'] ?? '').toString(),
    );
  }

  final double currentUnitPrice;
  final String currency;
  final String primarySourceLabel;
  final double primarySourceUnitPrice;
  final String primarySourceReference;
  final String primarySourceDate;
  final double absoluteDelta;
  final double? relativeDeltaPercent;
  final String evaluationStatus;
  final String evaluationReason;

  String get displayCurrentPrice =>
      '${currentUnitPrice.toStringAsFixed(2)} ${displayCurrency}';

  String get displayPrimaryPrice =>
      '${primarySourceUnitPrice.toStringAsFixed(2)} ${displayCurrency}';

  String get displayDelta {
    final sign = absoluteDelta > 0 ? '+' : '';
    final percent = relativeDeltaPercent;
    if (percent == null) {
      return '$sign${absoluteDelta.toStringAsFixed(2)} ${displayCurrency}';
    }
    final percentSign = percent > 0 ? '+' : '';
    return '$sign${absoluteDelta.toStringAsFixed(2)} ${displayCurrency}  •  $percentSign${percent.toStringAsFixed(2)} %';
  }

  String get displayStatus {
    switch (evaluationStatus) {
      case 'below_cost_basis':
        return 'Unter Kostenbasis';
      case 'at_cost_basis':
        return 'Auf Kostenbasis';
      case 'above_cost_basis':
        return 'Ueber Kostenbasis';
      default:
        return evaluationStatus.trim().isEmpty
            ? 'Keine Bewertung'
            : evaluationStatus;
    }
  }

  String get displayCurrency => currency.trim().isEmpty ? 'EUR' : currency;

  String get displaySourceMeta {
    final parts = <String>[];
    if (primarySourceReference.trim().isNotEmpty) {
      parts.add(primarySourceReference.trim());
    }
    if (primarySourceDate.trim().isNotEmpty) {
      parts.add(primarySourceDate.trim().split('T').first);
    }
    return parts.join('  •  ');
  }
}

class _QuotePriceDecisionTransparencyDraft {
  const _QuotePriceDecisionTransparencyDraft({
    required this.currentUnitPrice,
    required this.currency,
    required this.primarySourceLabel,
    required this.primarySourceUnitPrice,
    required this.primarySourceReference,
    required this.primarySourceDate,
    required this.absoluteDelta,
    required this.relativeDeltaPercent,
    required this.decisionStatus,
    required this.decisionReason,
  });

  factory _QuotePriceDecisionTransparencyDraft.fromJson(
      Map<String, dynamic> json) {
    final rawRelative = json['relative_delta_percent'];
    return _QuotePriceDecisionTransparencyDraft(
      currentUnitPrice: ((json['current_unit_price'] as num?) ?? 0).toDouble(),
      currency: (json['currency'] ?? '').toString(),
      primarySourceLabel: (json['primary_source_label'] ?? '').toString(),
      primarySourceUnitPrice:
          ((json['primary_source_unit_price'] as num?) ?? 0).toDouble(),
      primarySourceReference:
          (json['primary_source_reference'] ?? '').toString(),
      primarySourceDate: (json['primary_source_date'] ?? '').toString(),
      absoluteDelta: ((json['absolute_delta'] as num?) ?? 0).toDouble(),
      relativeDeltaPercent: rawRelative is num ? rawRelative.toDouble() : null,
      decisionStatus: (json['decision_status'] ?? '').toString(),
      decisionReason: (json['decision_reason'] ?? '').toString(),
    );
  }

  final double currentUnitPrice;
  final String currency;
  final String primarySourceLabel;
  final double primarySourceUnitPrice;
  final String primarySourceReference;
  final String primarySourceDate;
  final double absoluteDelta;
  final double? relativeDeltaPercent;
  final String decisionStatus;
  final String decisionReason;

  String get displayCurrentPrice =>
      '${currentUnitPrice.toStringAsFixed(2)} $displayCurrency';

  String get displayPrimaryPrice =>
      '${primarySourceUnitPrice.toStringAsFixed(2)} $displayCurrency';

  String get displayDelta {
    final sign = absoluteDelta > 0 ? '+' : '';
    final percent = relativeDeltaPercent;
    if (percent == null) {
      return '$sign${absoluteDelta.toStringAsFixed(2)} $displayCurrency';
    }
    final percentSign = percent > 0 ? '+' : '';
    return '$sign${absoluteDelta.toStringAsFixed(2)} $displayCurrency  •  $percentSign${percent.toStringAsFixed(2)} %';
  }

  String get displayStatus {
    switch (decisionStatus) {
      case 'matches_primary_source':
        return 'Entspricht primaerer Quelle';
      case 'differs_from_primary_source':
        return 'Weicht von primaerer Quelle ab';
      case 'no_primary_source':
        return 'Keine primaere Quelle';
      default:
        return decisionStatus.trim().isEmpty
            ? 'Keine Transparenz'
            : decisionStatus;
    }
  }

  String get displayCurrency => currency.trim().isEmpty ? 'EUR' : currency;

  String get displaySourceMeta {
    final parts = <String>[];
    if (primarySourceReference.trim().isNotEmpty) {
      parts.add(primarySourceReference.trim());
    }
    if (primarySourceDate.trim().isNotEmpty) {
      parts.add(primarySourceDate.trim().split('T').first);
    }
    return parts.join('  •  ');
  }
}

class _QuotePriceDecisionHistoryEntryDraft {
  const _QuotePriceDecisionHistoryEntryDraft({
    required this.id,
    required this.decisionType,
    required this.materialId,
    required this.sourceLabel,
    required this.sourceUnitPrice,
    required this.appliedUnitPrice,
    required this.currency,
    required this.sourceReference,
    required this.sourceDate,
    required this.createdAt,
  });

  factory _QuotePriceDecisionHistoryEntryDraft.fromJson(
      Map<String, dynamic> json) {
    return _QuotePriceDecisionHistoryEntryDraft(
      id: (json['id'] ?? '').toString(),
      decisionType: (json['decision_type'] ?? '').toString(),
      materialId: (json['material_id'] ?? '').toString(),
      sourceLabel: (json['source_label'] ?? '').toString(),
      sourceUnitPrice: ((json['source_unit_price'] as num?) ?? 0).toDouble(),
      appliedUnitPrice: ((json['applied_unit_price'] as num?) ?? 0).toDouble(),
      currency: (json['currency'] ?? '').toString(),
      sourceReference: (json['source_reference'] ?? '').toString(),
      sourceDate: (json['source_date'] ?? '').toString(),
      createdAt: (json['created_at'] ?? '').toString(),
    );
  }

  final String id;
  final String decisionType;
  final String materialId;
  final String sourceLabel;
  final double sourceUnitPrice;
  final double appliedUnitPrice;
  final String currency;
  final String sourceReference;
  final String sourceDate;
  final String createdAt;

  String get displayDecisionType {
    switch (decisionType) {
      case 'primary_source_applied':
        return 'Primaerpreis uebernommen';
      case 'target_price_applied':
        return 'Zielpreis uebernommen';
      default:
        return decisionType.trim().isEmpty ? 'Preisentscheidung' : decisionType;
    }
  }

  String get displayAppliedPrice =>
      '${appliedUnitPrice.toStringAsFixed(2)} $displayCurrency';

  String get displaySourcePrice =>
      '${sourceUnitPrice.toStringAsFixed(2)} $displayCurrency';

  String get displayCurrency => currency.trim().isEmpty ? 'EUR' : currency;

  String get displayCreatedAt =>
      createdAt.trim().isEmpty ? '' : createdAt.trim().split('.').first;

  String get displaySourceMeta {
    final parts = <String>[];
    if (sourceReference.trim().isNotEmpty) {
      parts.add(sourceReference.trim());
    }
    if (sourceDate.trim().isNotEmpty) {
      parts.add(sourceDate.trim().split('T').first);
    }
    return parts.join('  •  ');
  }
}

class _QuoteItemMarginAnchorDraft {
  const _QuoteItemMarginAnchorDraft({
    required this.currentUnitPrice,
    required this.costBasisUnitPrice,
    required this.currency,
    required this.absoluteMargin,
    required this.marginPercent,
    required this.marginStatus,
    required this.decisionId,
    required this.decisionType,
    required this.sourceLabel,
    required this.sourceReference,
    required this.sourceDate,
    required this.decisionCreatedAt,
  });

  factory _QuoteItemMarginAnchorDraft.fromJson(Map<String, dynamic> json) {
    final rawMarginPercent = json['margin_percent'];
    return _QuoteItemMarginAnchorDraft(
      currentUnitPrice: ((json['current_unit_price'] as num?) ?? 0).toDouble(),
      costBasisUnitPrice:
          ((json['cost_basis_unit_price'] as num?) ?? 0).toDouble(),
      currency: (json['currency'] ?? '').toString(),
      absoluteMargin: ((json['absolute_margin'] as num?) ?? 0).toDouble(),
      marginPercent:
          rawMarginPercent is num ? rawMarginPercent.toDouble() : null,
      marginStatus: (json['margin_status'] ?? '').toString(),
      decisionId: (json['decision_id'] ?? '').toString(),
      decisionType: (json['decision_type'] ?? '').toString(),
      sourceLabel: (json['source_label'] ?? '').toString(),
      sourceReference: (json['source_reference'] ?? '').toString(),
      sourceDate: (json['source_date'] ?? '').toString(),
      decisionCreatedAt: (json['decision_created_at'] ?? '').toString(),
    );
  }

  final double currentUnitPrice;
  final double costBasisUnitPrice;
  final String currency;
  final double absoluteMargin;
  final double? marginPercent;
  final String marginStatus;
  final String decisionId;
  final String decisionType;
  final String sourceLabel;
  final String sourceReference;
  final String sourceDate;
  final String decisionCreatedAt;

  String get displayCurrentPrice =>
      '${currentUnitPrice.toStringAsFixed(2)} $displayCurrency';

  String get displayCostBasis =>
      '${costBasisUnitPrice.toStringAsFixed(2)} $displayCurrency';

  String get displayMargin {
    final sign = absoluteMargin > 0 ? '+' : '';
    final percent = marginPercent;
    if (percent == null) {
      return '$sign${absoluteMargin.toStringAsFixed(2)} $displayCurrency';
    }
    final percentSign = percent > 0 ? '+' : '';
    return '$sign${absoluteMargin.toStringAsFixed(2)} $displayCurrency  •  $percentSign${percent.toStringAsFixed(2)} %';
  }

  String get displayStatus {
    switch (marginStatus) {
      case 'negative_margin':
        return 'Negative Marge';
      case 'zero_margin':
        return 'Keine Marge';
      case 'positive_margin':
        return 'Positive Marge';
      default:
        return marginStatus.trim().isEmpty ? 'Keine Marge' : marginStatus;
    }
  }

  String get displayDecisionType {
    switch (decisionType) {
      case 'primary_source_applied':
        return 'Primaerpreis uebernommen';
      default:
        return decisionType.trim().isEmpty ? 'Preisentscheidung' : decisionType;
    }
  }

  String get displayCurrency => currency.trim().isEmpty ? 'EUR' : currency;

  String get displayDecisionCreatedAt => decisionCreatedAt.trim().isEmpty
      ? ''
      : decisionCreatedAt.trim().split('.').first;

  String get displaySourceMeta {
    final parts = <String>[];
    if (sourceReference.trim().isNotEmpty) {
      parts.add(sourceReference.trim());
    }
    if (sourceDate.trim().isNotEmpty) {
      parts.add(sourceDate.trim().split('T').first);
    }
    return parts.join('  •  ');
  }
}

class _QuoteItemApprovalHintDraft {
  const _QuoteItemApprovalHintDraft({
    required this.approvalStatus,
    required this.approvalReason,
    required this.marginStatus,
    required this.currentUnitPrice,
    required this.costBasisUnitPrice,
    required this.currency,
    required this.absoluteMargin,
    required this.marginPercent,
    required this.decisionId,
    required this.decisionType,
    required this.sourceLabel,
    required this.decisionCreatedAt,
  });

  factory _QuoteItemApprovalHintDraft.fromJson(Map<String, dynamic> json) {
    final rawCurrentUnitPrice = json['current_unit_price'];
    final rawCostBasisUnitPrice = json['cost_basis_unit_price'];
    final rawAbsoluteMargin = json['absolute_margin'];
    final rawMarginPercent = json['margin_percent'];
    return _QuoteItemApprovalHintDraft(
      approvalStatus: (json['approval_status'] ?? '').toString(),
      approvalReason: (json['approval_reason'] ?? '').toString(),
      marginStatus: (json['margin_status'] ?? '').toString(),
      currentUnitPrice:
          rawCurrentUnitPrice is num ? rawCurrentUnitPrice.toDouble() : null,
      costBasisUnitPrice: rawCostBasisUnitPrice is num
          ? rawCostBasisUnitPrice.toDouble()
          : null,
      currency: (json['currency'] ?? '').toString(),
      absoluteMargin:
          rawAbsoluteMargin is num ? rawAbsoluteMargin.toDouble() : null,
      marginPercent:
          rawMarginPercent is num ? rawMarginPercent.toDouble() : null,
      decisionId: (json['decision_id'] ?? '').toString(),
      decisionType: (json['decision_type'] ?? '').toString(),
      sourceLabel: (json['source_label'] ?? '').toString(),
      decisionCreatedAt: (json['decision_created_at'] ?? '').toString(),
    );
  }

  final String approvalStatus;
  final String approvalReason;
  final String marginStatus;
  final double? currentUnitPrice;
  final double? costBasisUnitPrice;
  final String currency;
  final double? absoluteMargin;
  final double? marginPercent;
  final String decisionId;
  final String decisionType;
  final String sourceLabel;
  final String decisionCreatedAt;

  String get displayStatus {
    switch (approvalStatus) {
      case 'approval_not_required':
        return 'Keine Freigabeempfehlung';
      case 'approval_recommended':
        return 'Freigabe empfohlen';
      case 'approval_blocked_until_margin_available':
        return 'Kostenbasis fehlt';
      default:
        return approvalStatus.trim().isEmpty
            ? 'Kein Freigabehinweis'
            : approvalStatus;
    }
  }

  String get displayMargin {
    final margin = absoluteMargin;
    if (margin == null) return '';
    final sign = margin > 0 ? '+' : '';
    final percent = marginPercent;
    if (percent == null) {
      return '$sign${margin.toStringAsFixed(2)} $displayCurrency';
    }
    final percentSign = percent > 0 ? '+' : '';
    return '$sign${margin.toStringAsFixed(2)} $displayCurrency  •  $percentSign${percent.toStringAsFixed(2)} %';
  }

  String get displayCurrentPrice {
    final value = currentUnitPrice;
    if (value == null) return '';
    return '${value.toStringAsFixed(2)} $displayCurrency';
  }

  String get displayCostBasis {
    final value = costBasisUnitPrice;
    if (value == null) return '';
    return '${value.toStringAsFixed(2)} $displayCurrency';
  }

  String get displayDecisionType {
    switch (decisionType) {
      case 'primary_source_applied':
        return 'Primaerpreis uebernommen';
      default:
        return decisionType.trim().isEmpty ? 'Preisentscheidung' : decisionType;
    }
  }

  String get displayCurrency => currency.trim().isEmpty ? 'EUR' : currency;

  String get displayDecisionCreatedAt => decisionCreatedAt.trim().isEmpty
      ? ''
      : decisionCreatedAt.trim().split('.').first;
}

class _QuoteItemApprovalRequestDraft {
  const _QuoteItemApprovalRequestDraft({
    required this.id,
    required this.status,
    required this.reasonCode,
    required this.reasonText,
    required this.currentUnitPriceSnapshot,
    required this.costBasisUnitPriceSnapshot,
    required this.targetUnitPriceSnapshot,
    required this.targetMarginPercentSnapshot,
    required this.targetDifferenceSnapshot,
    required this.marginPercentSnapshot,
    required this.requestedBy,
    required this.requestedByName,
    required this.requestedAt,
    required this.cancelledBy,
    required this.cancelledByName,
    required this.cancelledAt,
    required this.decidedBy,
    required this.decidedByName,
    required this.decidedAt,
    required this.decisionComment,
    required this.approvedUnitPriceSnapshot,
    required this.approvedTargetMarginPercentSnapshot,
  });

  factory _QuoteItemApprovalRequestDraft.fromJson(Map<String, dynamic> json) {
    return _QuoteItemApprovalRequestDraft(
      id: (json['id'] ?? '').toString(),
      status: (json['status'] ?? '').toString(),
      reasonCode: (json['reason_code'] ?? '').toString(),
      reasonText: (json['reason_text'] ?? '').toString(),
      currentUnitPriceSnapshot:
          ((json['current_unit_price_snapshot'] as num?) ?? 0).toDouble(),
      costBasisUnitPriceSnapshot:
          ((json['cost_basis_unit_price_snapshot'] as num?) ?? 0).toDouble(),
      targetUnitPriceSnapshot:
          ((json['target_unit_price_snapshot'] as num?) ?? 0).toDouble(),
      targetMarginPercentSnapshot:
          ((json['target_margin_percent_snapshot'] as num?) ?? 0).toDouble(),
      targetDifferenceSnapshot:
          ((json['target_difference_snapshot'] as num?) ?? 0).toDouble(),
      marginPercentSnapshot: json['margin_percent_snapshot'] is num
          ? (json['margin_percent_snapshot'] as num).toDouble()
          : null,
      requestedBy: (json['requested_by'] ?? '').toString(),
      requestedByName: (json['requested_by_name'] ?? '').toString(),
      requestedAt: (json['requested_at'] ?? '').toString(),
      cancelledBy: (json['cancelled_by'] ?? '').toString(),
      cancelledByName: (json['cancelled_by_name'] ?? '').toString(),
      cancelledAt: (json['cancelled_at'] ?? '').toString(),
      decidedBy: (json['decided_by'] ?? '').toString(),
      decidedByName: (json['decided_by_name'] ?? '').toString(),
      decidedAt: (json['decided_at'] ?? '').toString(),
      decisionComment: (json['decision_comment'] ?? '').toString(),
      approvedUnitPriceSnapshot: json['approved_unit_price_snapshot'] is num
          ? (json['approved_unit_price_snapshot'] as num).toDouble()
          : null,
      approvedTargetMarginPercentSnapshot:
          json['approved_target_margin_percent_snapshot'] is num
              ? (json['approved_target_margin_percent_snapshot'] as num)
                  .toDouble()
              : null,
    );
  }

  final String id;
  final String status;
  final String reasonCode;
  final String reasonText;
  final double currentUnitPriceSnapshot;
  final double costBasisUnitPriceSnapshot;
  final double targetUnitPriceSnapshot;
  final double targetMarginPercentSnapshot;
  final double targetDifferenceSnapshot;
  final double? marginPercentSnapshot;
  final String requestedBy;
  final String requestedByName;
  final String requestedAt;
  final String cancelledBy;
  final String cancelledByName;
  final String cancelledAt;
  final String decidedBy;
  final String decidedByName;
  final String decidedAt;
  final String decisionComment;
  final double? approvedUnitPriceSnapshot;
  final double? approvedTargetMarginPercentSnapshot;

  String get displayStatus {
    switch (status) {
      case 'requested':
        return 'Freigabe angefordert';
      case 'cancelled':
        return 'Freigabe zurueckgenommen';
      case 'approved':
        return 'Freigabe genehmigt';
      case 'rejected':
        return 'Freigabe abgelehnt';
      case 'rework_resolved':
        return 'Nacharbeit erledigt';
      default:
        return status.trim().isEmpty ? 'Freigabeanforderung' : status;
    }
  }

  String get displayReason {
    if (reasonText.trim().isNotEmpty) {
      return reasonText.trim();
    }
    switch (reasonCode) {
      case 'negative_margin':
        return 'Negative Marge';
      case 'below_target_margin':
        return 'Unter Zielmarge';
      default:
        return reasonCode;
    }
  }

  String get displayRequestedAt =>
      requestedAt.trim().isEmpty ? '' : requestedAt.trim().split('.').first;

  String get displayRequestedBy =>
      requestedByName.trim().isNotEmpty ? requestedByName.trim() : requestedBy;

  String get displayCancelledAt =>
      cancelledAt.trim().isEmpty ? '' : cancelledAt.trim().split('.').first;

  String get displayCancelledBy =>
      cancelledByName.trim().isNotEmpty ? cancelledByName.trim() : cancelledBy;

  String get displayDecidedAt =>
      decidedAt.trim().isEmpty ? '' : decidedAt.trim().split('.').first;

  String get displayDecidedBy =>
      decidedByName.trim().isNotEmpty ? decidedByName.trim() : decidedBy;

  String get displayMargin {
    final margin = marginPercentSnapshot;
    if (margin == null) return '';
    return '${margin.toStringAsFixed(2)} %';
  }

  String get displayTargetMargin =>
      '${targetMarginPercentSnapshot.toStringAsFixed(2)} %';

  String get displayPriceSnapshot =>
      '${currentUnitPriceSnapshot.toStringAsFixed(2)} EUR';

  String get displayCostBasisSnapshot =>
      '${costBasisUnitPriceSnapshot.toStringAsFixed(2)} EUR';

  String get displayTargetPriceSnapshot =>
      '${targetUnitPriceSnapshot.toStringAsFixed(2)} EUR';

  String get displayTargetDifference =>
      '${targetDifferenceSnapshot.toStringAsFixed(2)} EUR';

  String get displayApprovedSnapshot {
    final approvedPrice = approvedUnitPriceSnapshot;
    if (approvedPrice == null) return '';
    final approvedTargetMargin = approvedTargetMarginPercentSnapshot;
    if (approvedTargetMargin == null) {
      return '${approvedPrice.toStringAsFixed(2)} EUR';
    }
    return '${approvedPrice.toStringAsFixed(2)} EUR  •  Zielmarge ${approvedTargetMargin.toStringAsFixed(2)} %';
  }
}

class _QuoteItemApprovalDecisionBadgeDraft {
  const _QuoteItemApprovalDecisionBadgeDraft({
    required this.id,
    required this.status,
    required this.reasonCode,
    required this.reasonText,
    required this.decidedBy,
    required this.decidedByName,
    required this.decidedAt,
    required this.decisionComment,
    required this.approvedUnitPriceSnapshot,
    required this.approvedTargetMarginPercentSnapshot,
  });

  factory _QuoteItemApprovalDecisionBadgeDraft.fromJson(
      Map<String, dynamic> json) {
    return _QuoteItemApprovalDecisionBadgeDraft(
      id: (json['id'] ?? '').toString(),
      status: (json['status'] ?? '').toString(),
      reasonCode: (json['reason_code'] ?? '').toString(),
      reasonText: (json['reason_text'] ?? '').toString(),
      decidedBy: (json['decided_by'] ?? '').toString(),
      decidedByName: (json['decided_by_name'] ?? '').toString(),
      decidedAt: (json['decided_at'] ?? '').toString(),
      decisionComment: (json['decision_comment'] ?? '').toString(),
      approvedUnitPriceSnapshot: json['approved_unit_price_snapshot'] is num
          ? (json['approved_unit_price_snapshot'] as num).toDouble()
          : null,
      approvedTargetMarginPercentSnapshot:
          json['approved_target_margin_percent_snapshot'] is num
              ? (json['approved_target_margin_percent_snapshot'] as num)
                  .toDouble()
              : null,
    );
  }

  final String id;
  final String status;
  final String reasonCode;
  final String reasonText;
  final String decidedBy;
  final String decidedByName;
  final String decidedAt;
  final String decisionComment;
  final double? approvedUnitPriceSnapshot;
  final double? approvedTargetMarginPercentSnapshot;

  bool get requiresRework => status == 'rejected';

  String get displayStatus {
    switch (status) {
      case 'approved':
        return 'Freigabe genehmigt';
      case 'rejected':
        return 'Freigabe abgelehnt';
      case 'rework_resolved':
        return 'Nacharbeit erledigt';
      default:
        return status.trim().isEmpty ? 'Freigabe entschieden' : status;
    }
  }

  String get displayReason {
    if (reasonText.trim().isNotEmpty) {
      return reasonText.trim();
    }
    switch (reasonCode) {
      case 'negative_margin':
        return 'Negative Marge';
      case 'below_target_margin':
        return 'Unter Zielmarge';
      default:
        return reasonCode;
    }
  }

  String get displayDecidedAt =>
      decidedAt.trim().isEmpty ? '' : decidedAt.trim().split('.').first;

  String get displayDecidedBy =>
      decidedByName.trim().isNotEmpty ? decidedByName.trim() : decidedBy;

  String get displayApprovedSnapshot {
    final approvedPrice = approvedUnitPriceSnapshot;
    if (approvedPrice == null) return '';
    final approvedTargetMargin = approvedTargetMarginPercentSnapshot;
    if (approvedTargetMargin == null) {
      return '${approvedPrice.toStringAsFixed(2)} EUR';
    }
    return '${approvedPrice.toStringAsFixed(2)} EUR  •  Zielmarge ${approvedTargetMargin.toStringAsFixed(2)} %';
  }
}

class _QuoteItemTargetMarginAnchorDraft {
  const _QuoteItemTargetMarginAnchorDraft({
    required this.targetStatus,
    required this.targetReason,
    required this.targetMarginPercent,
    required this.currentUnitPrice,
    required this.costBasisUnitPrice,
    required this.targetUnitPrice,
    required this.currency,
    required this.absoluteMargin,
    required this.marginPercent,
    required this.targetDifference,
    required this.targetDifferencePercent,
    required this.marginStatus,
    required this.decisionId,
    required this.decisionType,
    required this.sourceLabel,
    required this.decisionCreatedAt,
  });

  factory _QuoteItemTargetMarginAnchorDraft.fromJson(
      Map<String, dynamic> json) {
    final rawCurrentUnitPrice = json['current_unit_price'];
    final rawCostBasisUnitPrice = json['cost_basis_unit_price'];
    final rawTargetUnitPrice = json['target_unit_price'];
    final rawAbsoluteMargin = json['absolute_margin'];
    final rawMarginPercent = json['margin_percent'];
    final rawTargetDifference = json['target_difference'];
    final rawTargetDifferencePercent = json['target_difference_percent'];
    return _QuoteItemTargetMarginAnchorDraft(
      targetStatus: (json['target_status'] ?? '').toString(),
      targetReason: (json['target_reason'] ?? '').toString(),
      targetMarginPercent:
          ((json['target_margin_percent'] as num?) ?? 0).toDouble(),
      currentUnitPrice:
          rawCurrentUnitPrice is num ? rawCurrentUnitPrice.toDouble() : null,
      costBasisUnitPrice: rawCostBasisUnitPrice is num
          ? rawCostBasisUnitPrice.toDouble()
          : null,
      targetUnitPrice:
          rawTargetUnitPrice is num ? rawTargetUnitPrice.toDouble() : null,
      currency: (json['currency'] ?? '').toString(),
      absoluteMargin:
          rawAbsoluteMargin is num ? rawAbsoluteMargin.toDouble() : null,
      marginPercent:
          rawMarginPercent is num ? rawMarginPercent.toDouble() : null,
      targetDifference:
          rawTargetDifference is num ? rawTargetDifference.toDouble() : null,
      targetDifferencePercent: rawTargetDifferencePercent is num
          ? rawTargetDifferencePercent.toDouble()
          : null,
      marginStatus: (json['margin_status'] ?? '').toString(),
      decisionId: (json['decision_id'] ?? '').toString(),
      decisionType: (json['decision_type'] ?? '').toString(),
      sourceLabel: (json['source_label'] ?? '').toString(),
      decisionCreatedAt: (json['decision_created_at'] ?? '').toString(),
    );
  }

  final String targetStatus;
  final String targetReason;
  final double targetMarginPercent;
  final double? currentUnitPrice;
  final double? costBasisUnitPrice;
  final double? targetUnitPrice;
  final String currency;
  final double? absoluteMargin;
  final double? marginPercent;
  final double? targetDifference;
  final double? targetDifferencePercent;
  final String marginStatus;
  final String decisionId;
  final String decisionType;
  final String sourceLabel;
  final String decisionCreatedAt;

  String get displayStatus {
    switch (targetStatus) {
      case 'below_cost':
        return 'Unter Kostenbasis';
      case 'below_target':
        return 'Unter Zielmarge';
      case 'on_target':
        return 'Zielmarge erreicht';
      case 'above_target':
        return 'Ueber Zielmarge';
      case 'target_blocked_until_margin_available':
        return 'Kostenbasis fehlt';
      default:
        return targetStatus.trim().isEmpty ? 'Keine Zielmarge' : targetStatus;
    }
  }

  String get displayTargetMargin =>
      '${targetMarginPercent.toStringAsFixed(2)} %';

  String get displayCurrentPrice {
    final value = currentUnitPrice;
    if (value == null) return '';
    return '${value.toStringAsFixed(2)} $displayCurrency';
  }

  String get displayCostBasis {
    final value = costBasisUnitPrice;
    if (value == null) return '';
    return '${value.toStringAsFixed(2)} $displayCurrency';
  }

  String get displayTargetPrice {
    final value = targetUnitPrice;
    if (value == null) return '';
    return '${value.toStringAsFixed(2)} $displayCurrency';
  }

  String get displayTargetDifference {
    final difference = targetDifference;
    if (difference == null) return '';
    final sign = difference > 0 ? '+' : '';
    final percent = targetDifferencePercent;
    if (percent == null) {
      return '$sign${difference.toStringAsFixed(2)} $displayCurrency';
    }
    final percentSign = percent > 0 ? '+' : '';
    return '$sign${difference.toStringAsFixed(2)} $displayCurrency  •  $percentSign${percent.toStringAsFixed(2)} %';
  }

  String get displayMargin {
    final margin = absoluteMargin;
    if (margin == null) return '';
    final sign = margin > 0 ? '+' : '';
    final percent = marginPercent;
    if (percent == null) {
      return '$sign${margin.toStringAsFixed(2)} $displayCurrency';
    }
    final percentSign = percent > 0 ? '+' : '';
    return '$sign${margin.toStringAsFixed(2)} $displayCurrency  •  $percentSign${percent.toStringAsFixed(2)} %';
  }

  String get displayDecisionType {
    switch (decisionType) {
      case 'primary_source_applied':
        return 'Primaerpreis uebernommen';
      default:
        return decisionType.trim().isEmpty ? 'Preisentscheidung' : decisionType;
    }
  }

  String get displayCurrency => currency.trim().isEmpty ? 'EUR' : currency;

  String get displayDecisionCreatedAt => decisionCreatedAt.trim().isEmpty
      ? ''
      : decisionCreatedAt.trim().split('.').first;

  bool get canRequestApproval =>
      targetStatus == 'below_cost' || targetStatus == 'below_target';
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
      required this.canSearchMaterial,
      required this.searchingMaterial,
      required this.applyingSearchResult,
      required this.canLoadPriceSuggestion,
      required this.loadingPriceSuggestion,
      required this.canLoadPriceHistory,
      required this.loadingPriceHistory,
      required this.canLoadPriceSourcePriority,
      required this.loadingPriceSourcePriority,
      required this.canLoadPriceEvaluation,
      required this.loadingPriceEvaluation,
      required this.canLoadPriceDecisionTransparency,
      required this.loadingPriceDecisionTransparency,
      required this.canLoadPriceDecisionHistory,
      required this.loadingPriceDecisionHistory,
      required this.canLoadMarginAnchor,
      required this.loadingMarginAnchor,
      required this.canLoadApprovalHint,
      required this.loadingApprovalHint,
      required this.canLoadApprovalRequests,
      required this.loadingApprovalRequests,
      required this.canLoadTargetMarginAnchor,
      required this.loadingTargetMarginAnchor,
      required this.applyingPriceSuggestion,
      required this.applyingPrimaryPriceSource,
      required this.applyingTargetPrice,
      required this.requestingApproval,
      required this.cancellingApproval,
      required this.approvingApproval,
      required this.rejectingApproval,
      required this.resolvingApprovalRework,
      required this.canApplyCandidate,
      required this.applyingCandidate,
      this.highlighted = false,
      this.onLoadPriceSuggestion,
      this.onLoadPriceHistory,
      this.onLoadPriceSourcePriority,
      this.onLoadPriceEvaluation,
      this.onLoadPriceDecisionTransparency,
      this.onLoadPriceDecisionHistory,
      this.onLoadMarginAnchor,
      this.onLoadApprovalHint,
      this.onLoadApprovalRequests,
      this.onLoadTargetMarginAnchor,
      this.onApplyPriceSuggestion,
      this.onApplyPrimaryPriceSource,
      this.onApplyTargetPrice,
      this.onRequestApproval,
      this.onCancelApproval,
      this.onApproveApproval,
      this.onRejectApproval,
      this.onResolveApprovalRework,
      this.onSearchMaterial,
      this.onApplySearchResult,
      this.onApplyCandidate,
      this.onCommercialFieldsChanged,
      this.onRemove});

  final _QuoteItemDraft item;
  final int index;
  final bool canSearchMaterial;
  final bool searchingMaterial;
  final bool applyingSearchResult;
  final bool canLoadPriceSuggestion;
  final bool loadingPriceSuggestion;
  final bool canLoadPriceHistory;
  final bool loadingPriceHistory;
  final bool canLoadPriceSourcePriority;
  final bool loadingPriceSourcePriority;
  final bool canLoadPriceEvaluation;
  final bool loadingPriceEvaluation;
  final bool canLoadPriceDecisionTransparency;
  final bool loadingPriceDecisionTransparency;
  final bool canLoadPriceDecisionHistory;
  final bool loadingPriceDecisionHistory;
  final bool canLoadMarginAnchor;
  final bool loadingMarginAnchor;
  final bool canLoadApprovalHint;
  final bool loadingApprovalHint;
  final bool canLoadApprovalRequests;
  final bool loadingApprovalRequests;
  final bool canLoadTargetMarginAnchor;
  final bool loadingTargetMarginAnchor;
  final bool applyingPriceSuggestion;
  final bool applyingPrimaryPriceSource;
  final bool applyingTargetPrice;
  final bool requestingApproval;
  final bool cancellingApproval;
  final bool approvingApproval;
  final bool rejectingApproval;
  final bool resolvingApprovalRework;
  final bool canApplyCandidate;
  final bool applyingCandidate;
  final bool highlighted;
  final VoidCallback? onLoadPriceSuggestion;
  final VoidCallback? onLoadPriceHistory;
  final VoidCallback? onLoadPriceSourcePriority;
  final VoidCallback? onLoadPriceEvaluation;
  final VoidCallback? onLoadPriceDecisionTransparency;
  final VoidCallback? onLoadPriceDecisionHistory;
  final VoidCallback? onLoadMarginAnchor;
  final VoidCallback? onLoadApprovalHint;
  final VoidCallback? onLoadApprovalRequests;
  final VoidCallback? onLoadTargetMarginAnchor;
  final VoidCallback? onApplyPriceSuggestion;
  final VoidCallback? onApplyPrimaryPriceSource;
  final VoidCallback? onApplyTargetPrice;
  final VoidCallback? onRequestApproval;
  final VoidCallback? onCancelApproval;
  final VoidCallback? onApproveApproval;
  final VoidCallback? onRejectApproval;
  final VoidCallback? onResolveApprovalRework;
  final ValueChanged<String>? onSearchMaterial;
  final ValueChanged<_QuoteMaterialCandidateDraft>? onApplySearchResult;
  final ValueChanged<_QuoteMaterialCandidateDraft>? onApplyCandidate;
  final VoidCallback? onCommercialFieldsChanged;
  final VoidCallback? onRemove;

  @override
  State<_QuoteItemRow> createState() => _QuoteItemRowState();
}

class _QuoteItemRowState extends State<_QuoteItemRow> {
  @override
  Widget build(BuildContext context) {
    return Card(
      margin: EdgeInsets.zero,
      color: widget.highlighted ? Colors.red.shade50 : null,
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
                onChanged: (_) => widget.onCommercialFieldsChanged?.call(),
                decoration: const InputDecoration(labelText: 'Beschreibung')),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                    child: TextField(
                        controller: widget.item.qtyCtrl,
                        onChanged: (_) =>
                            widget.onCommercialFieldsChanged?.call(),
                        decoration: const InputDecoration(labelText: 'Menge'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: widget.item.unitCtrl,
                        onChanged: (_) =>
                            widget.onCommercialFieldsChanged?.call(),
                        decoration:
                            const InputDecoration(labelText: 'Einheit'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: widget.item.unitPriceCtrl,
                        onChanged: (_) =>
                            widget.onCommercialFieldsChanged?.call(),
                        decoration:
                            const InputDecoration(labelText: 'Einzelpreis'))),
                const SizedBox(width: 8),
                Expanded(
                    child: TextField(
                        controller: widget.item.taxCodeCtrl,
                        onChanged: (_) =>
                            widget.onCommercialFieldsChanged?.call(),
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
                    onChanged: (_) => widget.onCommercialFieldsChanged?.call(),
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
                        widget.onCommercialFieldsChanged?.call();
                      });
                    },
                  ),
                ),
              ],
            ),
            if (widget.canLoadPriceSuggestion &&
                widget.onLoadPriceSuggestion != null &&
                widget.item.id.trim().isNotEmpty &&
                widget.item.materialIdCtrl.text.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Preisvorschlag',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingPriceSuggestion
                        ? null
                        : widget.onLoadPriceSuggestion,
                    child: widget.loadingPriceSuggestion
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.priceSuggestion != null) ...[
                const SizedBox(height: 6),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '${widget.item.priceSuggestion!.displayPrice}  •  ${widget.item.priceSuggestion!.sourceLabel}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    OutlinedButton(
                      onPressed: widget.applyingPriceSuggestion
                          ? null
                          : widget.onApplyPriceSuggestion,
                      child: widget.applyingPriceSuggestion
                          ? const SizedBox(
                              height: 16,
                              width: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Text('Preis uebernehmen'),
                    ),
                  ],
                ),
              ] else if (widget.item.priceSuggestionPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Kein Preisvorschlag sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canLoadPriceHistory &&
                widget.onLoadPriceHistory != null &&
                widget.item.id.trim().isNotEmpty &&
                widget.item.materialIdCtrl.text.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Preisquellen',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingPriceHistory
                        ? null
                        : widget.onLoadPriceHistory,
                    child: widget.loadingPriceHistory
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.priceHistoryEntries.isNotEmpty) ...[
                const SizedBox(height: 6),
                ...widget.item.priceHistoryEntries.map(
                  (entry) => Padding(
                    padding: const EdgeInsets.only(bottom: 4),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '${entry.displayPrice}  •  ${entry.sourceLabel}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                        if (entry.displayMeta.isNotEmpty)
                          Text(
                            entry.displayMeta,
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ] else if (widget.item.priceHistoryPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Preisquellen sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canLoadPriceSourcePriority &&
                widget.onLoadPriceSourcePriority != null &&
                widget.item.id.trim().isNotEmpty &&
                widget.item.materialIdCtrl.text.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Quellen-Priorisierung',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingPriceSourcePriority
                        ? null
                        : widget.onLoadPriceSourcePriority,
                    child: widget.loadingPriceSourcePriority
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.priceSourcePriorityEntries.isNotEmpty) ...[
                const SizedBox(height: 6),
                ...widget.item.priceSourcePriorityEntries.map(
                  (entry) => Padding(
                    padding: const EdgeInsets.only(bottom: 4),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '${entry.displayPriority}  •  ${entry.displayPrice}  •  ${entry.sourceLabel}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                        if (entry.displayMeta.isNotEmpty)
                          Text(
                            entry.displayMeta,
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ] else if (widget.item.priceSourcePriorityPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Quellen-Priorisierung sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canLoadPriceEvaluation &&
                widget.onLoadPriceEvaluation != null &&
                widget.item.id.trim().isNotEmpty &&
                widget.item.materialIdCtrl.text.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Preisbewertung',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingPriceEvaluation
                        ? null
                        : widget.onLoadPriceEvaluation,
                    child: widget.loadingPriceEvaluation
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.priceEvaluation != null) ...[
                const SizedBox(height: 6),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${widget.item.priceEvaluation!.displayStatus}  •  ${widget.item.priceEvaluation!.displayDelta}',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text(
                      'Aktuell ${widget.item.priceEvaluation!.displayCurrentPrice}  •  Basis ${widget.item.priceEvaluation!.displayPrimaryPrice} (${widget.item.priceEvaluation!.primarySourceLabel})',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                      ),
                    ),
                    if (widget
                        .item.priceEvaluation!.displaySourceMeta.isNotEmpty)
                      Text(
                        widget.item.priceEvaluation!.displaySourceMeta,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                    if (widget.item.priceEvaluation!.evaluationReason
                        .trim()
                        .isNotEmpty)
                      Text(
                        widget.item.priceEvaluation!.evaluationReason,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                    const SizedBox(height: 6),
                    Align(
                      alignment: Alignment.centerRight,
                      child: OutlinedButton(
                        onPressed: widget.applyingPrimaryPriceSource
                            ? null
                            : widget.onApplyPrimaryPriceSource,
                        child: widget.applyingPrimaryPriceSource
                            ? const SizedBox(
                                height: 16,
                                width: 16,
                                child:
                                    CircularProgressIndicator(strokeWidth: 2),
                              )
                            : const Text('Primaerpreis uebernehmen'),
                      ),
                    ),
                  ],
                ),
              ] else if (widget.item.priceEvaluationPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Preisbewertung sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canLoadPriceDecisionTransparency &&
                widget.onLoadPriceDecisionTransparency != null &&
                widget.item.id.trim().isNotEmpty &&
                widget.item.materialIdCtrl.text.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Preisentscheidung',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingPriceDecisionTransparency
                        ? null
                        : widget.onLoadPriceDecisionTransparency,
                    child: widget.loadingPriceDecisionTransparency
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.priceDecisionTransparency != null) ...[
                const SizedBox(height: 6),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${widget.item.priceDecisionTransparency!.displayStatus}  •  ${widget.item.priceDecisionTransparency!.displayDelta}',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text(
                      'Aktuell ${widget.item.priceDecisionTransparency!.displayCurrentPrice}  •  Primaer ${widget.item.priceDecisionTransparency!.displayPrimaryPrice} (${widget.item.priceDecisionTransparency!.primarySourceLabel})',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                      ),
                    ),
                    if (widget.item.priceDecisionTransparency!.displaySourceMeta
                        .isNotEmpty)
                      Text(
                        widget
                            .item.priceDecisionTransparency!.displaySourceMeta,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                    if (widget.item.priceDecisionTransparency!.decisionReason
                        .trim()
                        .isNotEmpty)
                      Text(
                        widget.item.priceDecisionTransparency!.decisionReason,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                  ],
                ),
              ] else if (widget.item.priceDecisionTransparencyPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Preisentscheidung sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canLoadPriceDecisionHistory &&
                widget.onLoadPriceDecisionHistory != null &&
                widget.item.id.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Preisentscheidungen',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingPriceDecisionHistory
                        ? null
                        : widget.onLoadPriceDecisionHistory,
                    child: widget.loadingPriceDecisionHistory
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.priceDecisionHistoryEntries.isNotEmpty) ...[
                const SizedBox(height: 6),
                ...widget.item.priceDecisionHistoryEntries.map(
                  (entry) => Padding(
                    padding: const EdgeInsets.only(bottom: 4),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '${entry.displayDecisionType}  •  ${entry.displayAppliedPrice}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        Text(
                          '${entry.sourceLabel}  •  Quelle ${entry.displaySourcePrice}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                        if (entry.displaySourceMeta.isNotEmpty)
                          Text(
                            entry.displaySourceMeta,
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                        if (entry.displayCreatedAt.isNotEmpty)
                          Text(
                            entry.displayCreatedAt,
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ] else if (widget.item.priceDecisionHistoryPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Preisentscheidungen sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canLoadMarginAnchor &&
                widget.onLoadMarginAnchor != null &&
                widget.item.id.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Marge',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingMarginAnchor
                        ? null
                        : widget.onLoadMarginAnchor,
                    child: widget.loadingMarginAnchor
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.marginAnchor != null) ...[
                const SizedBox(height: 6),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${widget.item.marginAnchor!.displayStatus}  •  ${widget.item.marginAnchor!.displayMargin}',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text(
                      'Aktuell ${widget.item.marginAnchor!.displayCurrentPrice}  •  Kostenbasis ${widget.item.marginAnchor!.displayCostBasis}',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                      ),
                    ),
                    Text(
                      '${widget.item.marginAnchor!.displayDecisionType}  •  ${widget.item.marginAnchor!.sourceLabel}',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                      ),
                    ),
                    if (widget.item.marginAnchor!.displaySourceMeta.isNotEmpty)
                      Text(
                        widget.item.marginAnchor!.displaySourceMeta,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                    if (widget
                        .item.marginAnchor!.displayDecisionCreatedAt.isNotEmpty)
                      Text(
                        widget.item.marginAnchor!.displayDecisionCreatedAt,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                  ],
                ),
              ] else if (widget.item.marginAnchorPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Marge sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canLoadApprovalHint &&
                widget.onLoadApprovalHint != null &&
                widget.item.id.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Freigabehinweis',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingApprovalHint
                        ? null
                        : widget.onLoadApprovalHint,
                    child: widget.loadingApprovalHint
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.approvalHint != null) ...[
                const SizedBox(height: 6),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      widget.item.approvalHint!.displayStatus,
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    if (widget.item.approvalHint!.approvalReason
                        .trim()
                        .isNotEmpty)
                      Text(
                        widget.item.approvalHint!.approvalReason,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    if (widget.item.approvalHint!.displayMargin.isNotEmpty)
                      Text(
                        'Marge ${widget.item.approvalHint!.displayMargin}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    if (widget.item.approvalHint!.displayCurrentPrice
                            .isNotEmpty &&
                        widget.item.approvalHint!.displayCostBasis.isNotEmpty)
                      Text(
                        'Aktuell ${widget.item.approvalHint!.displayCurrentPrice}  •  Kostenbasis ${widget.item.approvalHint!.displayCostBasis}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    if (widget.item.approvalHint!.sourceLabel.trim().isNotEmpty)
                      Text(
                        '${widget.item.approvalHint!.displayDecisionType}  •  ${widget.item.approvalHint!.sourceLabel}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                    if (widget
                        .item.approvalHint!.displayDecisionCreatedAt.isNotEmpty)
                      Text(
                        widget.item.approvalHint!.displayDecisionCreatedAt,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                  ],
                ),
              ] else if (widget.item.approvalHintPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Kein Freigabehinweis sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canLoadApprovalRequests &&
                widget.onLoadApprovalRequests != null &&
                widget.item.id.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Freigabehistorie',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingApprovalRequests
                        ? null
                        : widget.onLoadApprovalRequests,
                    child: widget.loadingApprovalRequests
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.approvalRequests.isNotEmpty) ...[
                const SizedBox(height: 6),
                ...widget.item.approvalRequests.map(
                  (entry) => Padding(
                    padding: const EdgeInsets.only(bottom: 6),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '${entry.displayStatus}  •  ${entry.displayReason}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        if (entry.displayRequestedAt.isNotEmpty)
                          Text(
                            entry.displayRequestedBy.trim().isEmpty
                                ? 'Angefordert ${entry.displayRequestedAt}'
                                : 'Angefordert ${entry.displayRequestedAt} von ${entry.displayRequestedBy}',
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                        if (entry.displayCancelledAt.isNotEmpty)
                          Text(
                            entry.displayCancelledBy.trim().isEmpty
                                ? 'Storniert ${entry.displayCancelledAt}'
                                : 'Storniert ${entry.displayCancelledAt} von ${entry.displayCancelledBy}',
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                        if (entry.displayDecidedAt.isNotEmpty)
                          Text(
                            entry.displayDecidedBy.trim().isEmpty
                                ? 'Entschieden ${entry.displayDecidedAt}'
                                : 'Entschieden ${entry.displayDecidedAt} von ${entry.displayDecidedBy}',
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                        Text(
                          'Preis ${entry.displayPriceSnapshot}  •  Kostenbasis ${entry.displayCostBasisSnapshot}  •  Ziel ${entry.displayTargetPriceSnapshot}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                        Text(
                          'Zielmarge ${entry.displayTargetMargin}  •  Abweichung ${entry.displayTargetDifference}${entry.displayMargin.isEmpty ? '' : '  •  Marge ${entry.displayMargin}'}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                        if (entry.decisionComment.trim().isNotEmpty)
                          Text(
                            entry.decisionComment.trim(),
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                        if (entry.displayApprovedSnapshot.isNotEmpty)
                          Text(
                            'Genehmigt ${entry.displayApprovedSnapshot}',
                            style: TextStyle(
                              fontSize: 12,
                              color: Colors.grey.shade600,
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ] else if (widget.item.approvalRequestsPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Freigabehistorie sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.item.latestApprovalDecision != null) ...[
              const SizedBox(height: 8),
              Builder(builder: (context) {
                final decision = widget.item.latestApprovalDecision!;
                final isPositiveDecision = decision.status == 'approved' ||
                    decision.status == 'rework_resolved';
                final color = isPositiveDecision
                    ? Colors.green.shade700
                    : Colors.red.shade700;
                final background = isPositiveDecision
                    ? Colors.green.shade50
                    : Colors.red.shade50;
                final border = isPositiveDecision
                    ? Colors.green.shade200
                    : Colors.red.shade200;
                return Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: background,
                    border: Border.all(color: border),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '${decision.displayStatus}${decision.displayReason.trim().isEmpty ? '' : '  •  ${decision.displayReason}'}',
                        style: TextStyle(
                          fontSize: 12,
                          color: color,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      if (decision.displayDecidedAt.isNotEmpty)
                        Text(
                          decision.displayDecidedBy.trim().isEmpty
                              ? 'Entschieden ${decision.displayDecidedAt}'
                              : 'Entschieden ${decision.displayDecidedAt} von ${decision.displayDecidedBy}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                      if (decision.decisionComment.trim().isNotEmpty)
                        Text(
                          decision.decisionComment.trim(),
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                      if (decision.displayApprovedSnapshot.isNotEmpty)
                        Text(
                          decision.status == 'rework_resolved'
                              ? 'Abgeschlossen ${decision.displayApprovedSnapshot}'
                              : 'Genehmigt ${decision.displayApprovedSnapshot}',
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                      if (decision.requiresRework) ...[
                        const SizedBox(height: 8),
                        Container(
                          width: double.infinity,
                          padding: const EdgeInsets.all(8),
                          decoration: BoxDecoration(
                            color: Colors.red.shade100,
                            borderRadius: BorderRadius.circular(6),
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                'Nacharbeit erforderlich',
                                style: TextStyle(
                                  fontSize: 12,
                                  color: Colors.red.shade800,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                              Text(
                                'Preis, Material oder Zielpreis anpassen und erneut Freigabe anfordern.',
                                style: TextStyle(
                                  fontSize: 12,
                                  color: Colors.grey.shade800,
                                ),
                              ),
                              if (widget.onResolveApprovalRework != null) ...[
                                const SizedBox(height: 8),
                                Align(
                                  alignment: Alignment.centerRight,
                                  child: FilledButton(
                                    onPressed: widget.resolvingApprovalRework
                                        ? null
                                        : widget.onResolveApprovalRework,
                                    child: widget.resolvingApprovalRework
                                        ? const SizedBox(
                                            height: 16,
                                            width: 16,
                                            child: CircularProgressIndicator(
                                                strokeWidth: 2),
                                          )
                                        : const Text('Nacharbeit abschliessen'),
                                  ),
                                ),
                              ],
                            ],
                          ),
                        ),
                      ],
                    ],
                  ),
                );
              }),
            ],
            if (widget.canLoadTargetMarginAnchor &&
                widget.onLoadTargetMarginAnchor != null &&
                widget.item.id.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Zielmarge',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade800,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  FilledButton(
                    onPressed: widget.loadingTargetMarginAnchor
                        ? null
                        : widget.onLoadTargetMarginAnchor,
                    child: widget.loadingTargetMarginAnchor
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Anzeigen'),
                  ),
                ],
              ),
              if (widget.item.targetMarginAnchor != null) ...[
                const SizedBox(height: 6),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${widget.item.targetMarginAnchor!.displayStatus}  •  Ziel ${widget.item.targetMarginAnchor!.displayTargetMargin}',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade700,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    if (widget.item.targetMarginAnchor!.targetReason
                        .trim()
                        .isNotEmpty)
                      Text(
                        widget.item.targetMarginAnchor!.targetReason,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    if (widget.item.targetMarginAnchor!.displayTargetPrice
                            .isNotEmpty &&
                        widget.item.targetMarginAnchor!.displayCurrentPrice
                            .isNotEmpty)
                      Text(
                        'Aktuell ${widget.item.targetMarginAnchor!.displayCurrentPrice}  •  Ziel ${widget.item.targetMarginAnchor!.displayTargetPrice}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    if (widget
                        .item.targetMarginAnchor!.displayCostBasis.isNotEmpty)
                      Text(
                        'Kostenbasis ${widget.item.targetMarginAnchor!.displayCostBasis}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    if (widget.item.targetMarginAnchor!.displayTargetDifference
                        .isNotEmpty)
                      Text(
                        'Abweichung ${widget.item.targetMarginAnchor!.displayTargetDifference}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    if (widget
                        .item.targetMarginAnchor!.displayMargin.isNotEmpty)
                      Text(
                        'Marge ${widget.item.targetMarginAnchor!.displayMargin}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                        ),
                      ),
                    if (widget.item.targetMarginAnchor!.sourceLabel
                        .trim()
                        .isNotEmpty)
                      Text(
                        '${widget.item.targetMarginAnchor!.displayDecisionType}  •  ${widget.item.targetMarginAnchor!.sourceLabel}',
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                    if (widget.item.targetMarginAnchor!.displayDecisionCreatedAt
                        .isNotEmpty)
                      Text(
                        widget
                            .item.targetMarginAnchor!.displayDecisionCreatedAt,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade600,
                        ),
                      ),
                    if (widget.item.approvalRequest != null) ...[
                      const SizedBox(height: 6),
                      Text(
                        widget.item.approvalRequest!.displayStatus,
                        style: TextStyle(
                          fontSize: 12,
                          color: Colors.grey.shade700,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      if (widget.item.approvalRequest!.displayReason
                          .trim()
                          .isNotEmpty)
                        Text(
                          widget.item.approvalRequest!.displayReason,
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade700,
                          ),
                        ),
                      if (widget
                          .item.approvalRequest!.displayRequestedAt.isNotEmpty)
                        Text(
                          widget.item.approvalRequest!.displayRequestedAt,
                          style: TextStyle(
                            fontSize: 12,
                            color: Colors.grey.shade600,
                          ),
                        ),
                      const SizedBox(height: 6),
                      Align(
                        alignment: Alignment.centerRight,
                        child: Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          alignment: WrapAlignment.end,
                          children: [
                            if (widget.onApproveApproval != null)
                              FilledButton(
                                onPressed: widget.approvingApproval
                                    ? null
                                    : widget.onApproveApproval,
                                child: widget.approvingApproval
                                    ? const SizedBox(
                                        height: 16,
                                        width: 16,
                                        child: CircularProgressIndicator(
                                            strokeWidth: 2),
                                      )
                                    : const Text('Genehmigen'),
                              ),
                            if (widget.onRejectApproval != null)
                              OutlinedButton(
                                onPressed: widget.rejectingApproval
                                    ? null
                                    : widget.onRejectApproval,
                                child: widget.rejectingApproval
                                    ? const SizedBox(
                                        height: 16,
                                        width: 16,
                                        child: CircularProgressIndicator(
                                            strokeWidth: 2),
                                      )
                                    : const Text('Ablehnen'),
                              ),
                            OutlinedButton(
                              onPressed: widget.cancellingApproval
                                  ? null
                                  : widget.onCancelApproval,
                              child: widget.cancellingApproval
                                  ? const SizedBox(
                                      height: 16,
                                      width: 16,
                                      child: CircularProgressIndicator(
                                          strokeWidth: 2),
                                    )
                                  : const Text('Freigabe stornieren'),
                            ),
                          ],
                        ),
                      ),
                    ],
                    if (widget.item.targetMarginAnchor!.canRequestApproval &&
                        widget.item.approvalRequest == null) ...[
                      const SizedBox(height: 6),
                      Align(
                        alignment: Alignment.centerRight,
                        child: OutlinedButton(
                          onPressed: widget.requestingApproval
                              ? null
                              : widget.onRequestApproval,
                          child: widget.requestingApproval
                              ? const SizedBox(
                                  height: 16,
                                  width: 16,
                                  child:
                                      CircularProgressIndicator(strokeWidth: 2),
                                )
                              : const Text('Freigabe anfordern'),
                        ),
                      ),
                    ],
                    if (widget.item.targetMarginAnchor!.targetUnitPrice !=
                            null &&
                        widget.item.targetMarginAnchor!.targetStatus !=
                            'target_blocked_until_margin_available') ...[
                      const SizedBox(height: 6),
                      Align(
                        alignment: Alignment.centerRight,
                        child: OutlinedButton(
                          onPressed: widget.applyingTargetPrice
                              ? null
                              : widget.onApplyTargetPrice,
                          child: widget.applyingTargetPrice
                              ? const SizedBox(
                                  height: 16,
                                  width: 16,
                                  child:
                                      CircularProgressIndicator(strokeWidth: 2),
                                )
                              : const Text('Zielpreis uebernehmen'),
                        ),
                      ),
                    ],
                  ],
                ),
              ] else if (widget.item.targetMarginAnchorPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Zielmarge sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
            if (widget.canSearchMaterial &&
                widget.onSearchMaterial != null &&
                widget.item.id.trim().isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: widget.item.materialSearchCtrl,
                      decoration: const InputDecoration(
                        labelText: 'Materialsuche',
                        helperText:
                            'Kleiner Suchpfad nach Nummer oder Bezeichnung',
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  FilledButton(
                    onPressed: widget.searchingMaterial
                        ? null
                        : () => widget.onSearchMaterial!(
                            widget.item.materialSearchCtrl.text.trim()),
                    child: widget.searchingMaterial
                        ? const SizedBox(
                            height: 16,
                            width: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Suchen'),
                  ),
                ],
              ),
              if (widget.item.materialSearchResults.isNotEmpty) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Suchtreffer',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade800,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                const SizedBox(height: 4),
                ...widget.item.materialSearchResults.map(
                  (candidate) => Padding(
                    padding: const EdgeInsets.only(bottom: 4),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Expanded(
                          child: Align(
                            alignment: Alignment.centerLeft,
                            child: Text(
                              '${candidate.displayTitle}  •  ${candidate.materialId}',
                              style: TextStyle(
                                fontSize: 12,
                                color: Colors.grey.shade700,
                              ),
                            ),
                          ),
                        ),
                        if (widget.canSearchMaterial &&
                            widget.onApplySearchResult != null &&
                            widget.item.id.trim().isNotEmpty)
                          TextButton(
                            onPressed: widget.applyingSearchResult
                                ? null
                                : () => widget.onApplySearchResult!(candidate),
                            child: widget.applyingSearchResult
                                ? const SizedBox(
                                    height: 14,
                                    width: 14,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : const Text('Uebernehmen'),
                          ),
                      ],
                    ),
                  ),
                ),
              ] else if (widget.item.materialSearchPerformed) ...[
                const SizedBox(height: 6),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Keine Suchtreffer sichtbar',
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey.shade700,
                    ),
                  ),
                ),
              ],
            ],
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
