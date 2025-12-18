import 'package:flutter/material.dart';
import '../api.dart';

class InvoicesPage extends StatefulWidget {
  const InvoicesPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<InvoicesPage> createState() => _InvoicesPageState();
}

class _InvoicesPageState extends State<InvoicesPage> {
  List<dynamic> items = [];
  Map<String, dynamic>? selected;
  bool loading = false;
  int limit = 50;
  int offset = 0;
  String? statusFilter;
  final searchCtrl = TextEditingController();
  final contactCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();
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
      setState(() => items = list);
      if (selected != null) {
        _loadDetail(selected!['id'] as String);
      }
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> _loadDetail(String id) async {
    try {
      final inv = await widget.api.getInvoiceOut(id);
      if (mounted) setState(() => selected = inv);
    } catch (_) {}
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

  @override
  Widget build(BuildContext context) {
    final sel = selected;
    return Scaffold(
      appBar: AppBar(title: const Text('Ausgangsrechnungen')),
      floatingActionButton: FloatingActionButton(
          onPressed: _createInvoiceDialog, child: const Icon(Icons.add)),
      body: Row(
        children: [
          SizedBox(
            width: 380,
            child: Column(
              children: [
                Padding(
                  padding: const EdgeInsets.all(12),
                  child: Row(
                    children: [
                      const Text('Rechnungen',
                          style: TextStyle(
                              fontSize: 18, fontWeight: FontWeight.bold)),
                      const Spacer(),
                      SizedBox(
                        width: 150,
                        child: TextField(
                          controller: searchCtrl,
                          decoration: InputDecoration(
                            isDense: true,
                            hintText: 'Suche (Nummer/ID)',
                            suffixIcon: IconButton(
                              icon: const Icon(Icons.clear),
                              onPressed: () {
                                searchCtrl.clear();
                                _resetAndLoad();
                              },
                            ),
                          ),
                          onSubmitted: (_) => _resetAndLoad(),
                        ),
                      ),
                      const SizedBox(width: 8),
                      SizedBox(
                        width: 140,
                        child: TextField(
                          controller: contactCtrl,
                          decoration: const InputDecoration(
                              isDense: true, hintText: 'Kontakt-ID'),
                          onSubmitted: (_) => _resetAndLoad(),
                        ),
                      ),
                      const SizedBox(width: 8),
                      DropdownButton<String?>(
                        value: statusFilter,
                        hint: const Text('Status'),
                        items: const [
                          DropdownMenuItem(value: null, child: Text('Alle')),
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
                      IconButton(
                          onPressed: _resetAndLoad,
                          icon: const Icon(Icons.refresh)),
                    ],
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  child: Row(
                    children: [
                      Text(
                          'Zeile ${offset + 1} - ${offset + items.length} (Limit $limit)'),
                      const Spacer(),
                      IconButton(
                          tooltip: 'Zurück',
                          onPressed: offset == 0 ? null : () => _changePage(-1),
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
                    separatorBuilder: (_, __) => const Divider(height: 1),
                    itemBuilder: (ctx, i) {
                      final it = items[i] as Map<String, dynamic>;
                      final id = it['id']?.toString() ?? '';
                      final num = it['nummer']?.toString() ??
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
                        title: Text(num.isEmpty ? id : num),
                        subtitle: Text(
                            '$cname  •  Status: ${it['status']}  •  Brutto: $gross  •  Offen: ${open.toStringAsFixed(2)}'),
                        trailing: _statusChip((it['status'] ?? '').toString()),
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
          ),
          const VerticalDivider(width: 1),
          Expanded(
            child: sel == null
                ? const Center(child: Text('Bitte Rechnung auswählen'))
                : Padding(
                    padding: const EdgeInsets.all(16),
                    child: SingleChildScrollView(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Text(
                                  sel['nummer']?.toString() ??
                                      sel['number']?.toString() ??
                                      sel['id'],
                                  style: const TextStyle(
                                      fontSize: 20,
                                      fontWeight: FontWeight.bold)),
                              const SizedBox(width: 12),
                              _statusChip((sel['status'] ?? '').toString()),
                              const Spacer(),
                              if ((sel['status'] ?? '') == 'draft')
                                FilledButton.icon(
                                    onPressed: _bookSelected,
                                    icon: const Icon(Icons.check),
                                    label: const Text('Buchen')),
                              const SizedBox(width: 8),
                              FilledButton.icon(
                                  onPressed: _addPayment,
                                  icon: const Icon(Icons.payments),
                                  label: const Text('Zahlung')),
                            ],
                          ),
                          const SizedBox(height: 8),
                          Text(
                              'Kontakt: ${sel['contact_name'] ?? ''} (${sel['contact_id'] ?? ''})'),
                          Text(
                              'Datum: ${sel['invoice_date'] ?? ''}  Fällig: ${sel['due_date'] ?? ''}'),
                          const SizedBox(height: 8),
                          Text(
                              'Brutto: ${sel['gross_amount'] ?? ''}  Offen: ${(sel['gross_amount'] ?? 0) - (sel['paid_amount'] ?? 0)}'),
                          const SizedBox(height: 12),
                          const Text('Positionen',
                              style: TextStyle(fontWeight: FontWeight.bold)),
                          const SizedBox(height: 6),
                          ...List<Widget>.from(
                              ((sel['items'] ?? []) as List).map((it) {
                            final m = it as Map<String, dynamic>;
                            final net =
                                (m['qty'] ?? 0) * (m['unit_price'] ?? 0);
                            return ListTile(
                              dense: true,
                              title: Text(m['description']?.toString() ?? ''),
                              subtitle: Text(
                                  'Menge ${m['qty']} x ${m['unit_price']}  •  Steuer ${m['tax_code'] ?? ''}  •  Konto ${m['account_code'] ?? ''}'),
                              trailing: Text(net.toString()),
                            );
                          })),
                          const SizedBox(height: 12),
                          const Text('Zahlungen',
                              style: TextStyle(fontWeight: FontWeight.bold)),
                          FutureBuilder<List<dynamic>>(
                            future: widget.api
                                .listInvoicePayments(sel['id'].toString()),
                            builder: (context, snap) {
                              if (snap.connectionState ==
                                  ConnectionState.waiting)
                                return const Padding(
                                    padding: EdgeInsets.all(8),
                                    child:
                                        LinearProgressIndicator(minHeight: 2));
                              if (snap.hasError)
                                return Padding(
                                    padding: const EdgeInsets.all(8),
                                    child: Text('Fehler: ${snap.error}'));
                              final pays = snap.data ?? [];
                              if (pays.isEmpty)
                                return const Padding(
                                    padding: EdgeInsets.all(8),
                                    child: Text('Keine Zahlungen'));
                              return Column(children: [
                                ...pays.map((p) {
                                  final m = p as Map<String, dynamic>;
                                  return ListTile(
                                    dense: true,
                                    title: Text(
                                        'Betrag ${m['amount']} ${m['currency']}'),
                                    subtitle: Text(
                                        '${m['method']}  •  ${m['reference'] ?? ''}'),
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
        ],
      ),
    );
  }
}
