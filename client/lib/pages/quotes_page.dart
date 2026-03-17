import 'package:flutter/material.dart';

import '../api.dart';

String _quoteErrorMessage(Object error, {String fallback = 'Vorgang fehlgeschlagen'}) {
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
    this.openCreateOnStart = false,
  });

  final ApiClient api;
  final String? initialProjectId;
  final bool openCreateOnStart;

  @override
  State<QuotesPage> createState() => _QuotesPageState();
}

class _QuotesPageState extends State<QuotesPage> {
  bool _loading = true;
  List<dynamic> _items = const [];
  Map<String, dynamic>? _selected;
  final _searchCtrl = TextEditingController();
  final _projectCtrl = TextEditingController();
  String? _statusFilter;

  @override
  void initState() {
    super.initState();
    if (widget.initialProjectId != null && widget.initialProjectId!.trim().isNotEmpty) {
      _projectCtrl.text = widget.initialProjectId!.trim();
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
        projectId: _projectCtrl.text.trim().isEmpty ? null : _projectCtrl.text.trim(),
        status: _statusFilter,
      );
      setState(() => _items = list);
      final selectedId = _selected?['id']?.toString();
      if (selectedId != null && selectedId.isNotEmpty) {
        await _loadDetail(selectedId);
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(_quoteErrorMessage(e, fallback: 'Angebote konnten nicht geladen werden'))),
      );
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _loadDetail(String id) async {
    try {
      final detail = await widget.api.getQuote(id);
      if (mounted) setState(() => _selected = detail);
    } catch (_) {}
  }

  Future<void> _openCreateDialog({String? projectId}) async {
    final created = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) => _QuoteEditorDialog(api: widget.api, initialProjectId: projectId),
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
        SnackBar(content: Text(_quoteErrorMessage(e, fallback: 'Statuswechsel fehlgeschlagen'))),
      );
    }
  }

  Future<void> _downloadPdf() async {
    final selected = _selected;
    final id = selected?['id']?.toString();
    if (id == null) return;
    try {
      final number = (selected?['number'] ?? '').toString().trim();
      await widget.api.downloadQuotePdf(id, filename: number.isEmpty ? null : 'Angebot_$number.pdf');
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Angebots-PDF wird heruntergeladen')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(_quoteErrorMessage(e, fallback: 'PDF-Download fehlgeschlagen'))),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final selected = _selected;
    final selectedStatus = (selected?['status'] ?? '').toString();
    final canWrite = widget.api.hasPermission('quotes.write');
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
                      child: Row(
                        children: [
                          Expanded(
                            child: TextField(
                              controller: _searchCtrl,
                              decoration: const InputDecoration(labelText: 'Suche (Nummer/Kunde)'),
                              onSubmitted: (_) => _load(),
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: TextField(
                              controller: _projectCtrl,
                              decoration: const InputDecoration(labelText: 'Projekt-ID'),
                              onSubmitted: (_) => _load(),
                            ),
                          ),
                          const SizedBox(width: 12),
                          SizedBox(
                            width: 180,
                            child: DropdownButtonFormField<String?>(
                              initialValue: _statusFilter,
                              decoration: const InputDecoration(labelText: 'Status'),
                              items: const [
                                DropdownMenuItem(value: null, child: Text('Alle')),
                                DropdownMenuItem(value: 'draft', child: Text('Entwurf')),
                                DropdownMenuItem(value: 'sent', child: Text('Versendet')),
                                DropdownMenuItem(value: 'accepted', child: Text('Angenommen')),
                                DropdownMenuItem(value: 'rejected', child: Text('Abgelehnt')),
                              ],
                              onChanged: (value) => setState(() => _statusFilter = value),
                            ),
                          ),
                          const SizedBox(width: 12),
                          FilledButton(onPressed: _load, child: const Text('Filtern')),
                        ],
                      ),
                    ),
                    const Divider(height: 1),
                    Expanded(
                      child: _loading
                          ? const Center(child: CircularProgressIndicator())
                          : _items.isEmpty
                              ? const Center(child: Text('Noch keine Angebote gefunden.'))
                              : ListView.separated(
                                  itemCount: _items.length,
                                  separatorBuilder: (_, __) => const Divider(height: 1),
                                  itemBuilder: (context, index) {
                                    final item = _items[index] as Map<String, dynamic>;
                                    final id = item['id']?.toString();
                                    final selectedId = _selected?['id']?.toString();
                                    return ListTile(
                                      selected: id != null && id == selectedId,
                                      title: Text((item['number'] ?? 'Angebot').toString()),
                                      subtitle: Text(
                                        '${item['contact_name'] ?? '-'}  •  ${(item['status'] ?? '').toString()}  •  ${(item['gross_amount'] ?? 0).toString()} ${(item['currency'] ?? 'EUR')}',
                                      ),
                                      onTap: id == null ? null : () => _loadDetail(id),
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
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            Row(
                              children: [
                                Expanded(
                                  child: Text(
                                    (selected['number'] ?? 'Angebot').toString(),
                                    style: Theme.of(context).textTheme.headlineSmall,
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
                                  icon: const Icon(Icons.picture_as_pdf_rounded),
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
                                Chip(label: Text('Kunde: ${(selected['contact_name'] ?? '-').toString()}')),
                                Chip(label: Text('Projekt: ${(selected['project_name'] ?? '-').toString()}')),
                              ],
                            ),
                            const SizedBox(height: 12),
                            Text('Hinweis: ${(selected['note'] ?? '').toString()}'),
                            const SizedBox(height: 4),
                            Text('Quote-Date: ${(selected['quote_date'] ?? '').toString()}'),
                            Text('Gueltig bis: ${(selected['valid_until'] ?? '-').toString()}'),
                            const SizedBox(height: 16),
                            const Text('Positionen', style: TextStyle(fontWeight: FontWeight.bold)),
                            const SizedBox(height: 8),
                            Expanded(
                              child: ListView.separated(
                                itemCount: ((selected['items'] as List?) ?? const []).length,
                                separatorBuilder: (_, __) => const Divider(height: 1),
                                itemBuilder: (context, index) {
                                  final item = (selected['items'] as List)[index] as Map<String, dynamic>;
                                  return ListTile(
                                    title: Text((item['description'] ?? 'Position').toString()),
                                    subtitle: Text(
                                      'Menge ${(item['qty'] ?? 0)} ${(item['unit'] ?? '')}  •  Steuer ${(item['tax_code'] ?? '').toString()}',
                                    ),
                                    trailing: Text('${item['unit_price'] ?? 0}'),
                                  );
                                },
                              ),
                            ),
                            const SizedBox(height: 12),
                            Align(
                              alignment: Alignment.centerRight,
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.end,
                                children: [
                                  Text('Netto: ${(selected['net_amount'] ?? 0)} ${(selected['currency'] ?? 'EUR')}'),
                                  Text('Steuer: ${(selected['tax_amount'] ?? 0)} ${(selected['currency'] ?? 'EUR')}'),
                                  Text(
                                    'Brutto: ${(selected['gross_amount'] ?? 0)} ${(selected['currency'] ?? 'EUR')}',
                                    style: const TextStyle(fontWeight: FontWeight.bold),
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
                                  if (selectedStatus != 'draft')
                                    OutlinedButton(onPressed: () => _updateStatus('draft'), child: const Text('Auf Entwurf')),
                                  if (selectedStatus != 'sent')
                                    FilledButton(onPressed: () => _updateStatus('sent'), child: const Text('Versendet')),
                                  if (selectedStatus != 'accepted')
                                    FilledButton.tonal(onPressed: () => _updateStatus('accepted'), child: const Text('Angenommen')),
                                  if (selectedStatus != 'rejected')
                                    FilledButton.tonal(onPressed: () => _updateStatus('rejected'), child: const Text('Abgelehnt')),
                                ],
                              ),
                            ],
                          ],
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

class _QuoteEditorDialog extends StatefulWidget {
  const _QuoteEditorDialog({required this.api, this.initial, this.initialProjectId});

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
      text: widget.initialProjectId ?? (initial?['project_id'] ?? '').toString(),
    );
    _contactCtrl = TextEditingController(text: (initial?['contact_id'] ?? '').toString());
    _currencyCtrl = TextEditingController(text: (initial?['currency'] ?? 'EUR').toString());
    _noteCtrl = TextEditingController(text: (initial?['note'] ?? '').toString());
    final rawItems = (initial?['items'] as List?) ?? const [];
    _items = rawItems.isEmpty
        ? [_QuoteItemDraft()]
        : rawItems.map((e) => _QuoteItemDraft.fromJson((e as Map).cast<String, dynamic>())).toList();
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
        'currency': _currencyCtrl.text.trim().isEmpty ? 'EUR' : _currencyCtrl.text.trim().toUpperCase(),
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
        SnackBar(content: Text(_quoteErrorMessage(e, fallback: 'Angebot konnte nicht gespeichert werden'))),
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
                  Expanded(child: TextField(controller: _projectCtrl, decoration: const InputDecoration(labelText: 'Projekt-ID'))),
                  const SizedBox(width: 12),
                  Expanded(child: TextField(controller: _contactCtrl, decoration: const InputDecoration(labelText: 'Kontakt-ID'))),
                  const SizedBox(width: 12),
                  SizedBox(width: 120, child: TextField(controller: _currencyCtrl, decoration: const InputDecoration(labelText: 'Währung'))),
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
                  const Expanded(child: Text('Positionen', style: TextStyle(fontWeight: FontWeight.bold))),
                  TextButton.icon(
                    onPressed: () => setState(() => _items.add(_QuoteItemDraft())),
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
        TextButton(onPressed: _saving ? null : () => Navigator.of(context).pop(), child: const Text('Abbrechen')),
        FilledButton(
          onPressed: _saving ? null : _submit,
          child: _saving
              ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
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
        'tax_code': taxCodeCtrl.text.trim().isEmpty ? 'DE19' : taxCodeCtrl.text.trim(),
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
  const _QuoteItemRow({super.key, required this.item, required this.index, this.onRemove});

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
                  child: Text('Position ${index + 1}', style: const TextStyle(fontWeight: FontWeight.bold)),
                ),
                if (onRemove != null)
                  IconButton(onPressed: onRemove, icon: const Icon(Icons.delete_outline_rounded)),
              ],
            ),
            TextField(controller: item.descriptionCtrl, decoration: const InputDecoration(labelText: 'Beschreibung')),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(child: TextField(controller: item.qtyCtrl, decoration: const InputDecoration(labelText: 'Menge'))),
                const SizedBox(width: 8),
                Expanded(child: TextField(controller: item.unitCtrl, decoration: const InputDecoration(labelText: 'Einheit'))),
                const SizedBox(width: 8),
                Expanded(child: TextField(controller: item.unitPriceCtrl, decoration: const InputDecoration(labelText: 'Einzelpreis'))),
                const SizedBox(width: 8),
                Expanded(child: TextField(controller: item.taxCodeCtrl, decoration: const InputDecoration(labelText: 'Steuercode'))),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
