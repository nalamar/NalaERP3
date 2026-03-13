import 'dart:ui';
import 'package:flutter/material.dart';
import '../api.dart';

class BankStatementsPage extends StatefulWidget {
  const BankStatementsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<BankStatementsPage> createState() => _BankStatementsPageState();
}

class _BankStatementsPageState extends State<BankStatementsPage> {
  List<dynamic> items = [];
  bool loading = false;
  int limit = 50;
  int offset = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => loading = true);
    try {
      final list =
          await widget.api.listBankStatements(limit: limit, offset: offset);
      setState(() => items = list);
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void _changePage(int delta) {
    final next = offset + delta * limit;
    if (next < 0) return;
    setState(() => offset = next);
    _load();
  }

  Future<void> _match(String id) async {
    final invoiceCtrl = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Statement matchen'),
        content: SizedBox(
          width: 420,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              FutureBuilder<List<dynamic>>(
                future: widget.api.listInvoicesOut(limit: 100),
                builder: (context, snap) {
                  if (snap.connectionState == ConnectionState.waiting) {
                    return const Padding(
                        padding: EdgeInsets.all(8),
                        child: LinearProgressIndicator(minHeight: 2));
                  }
                  final list = snap.data ?? [];
                  if (list.isEmpty) {
                    return const Text('Keine offenen Rechnungen gefunden');
                  }
                  return DropdownButtonFormField<String>(
                    isExpanded: true,
                    decoration: const InputDecoration(
                        labelText: 'Invoice auswählen (optional)'),
                    items: [
                      const DropdownMenuItem(
                          value: '', child: Text('Automatisch finden')),
                      ...list.map((e) {
                        final m = e as Map<String, dynamic>;
                        final num = m['nummer'] ??
                            m['number'] ??
                            m['id']?.toString() ??
                            '';
                        final open =
                            (m['gross_amount'] ?? 0) - (m['paid_amount'] ?? 0);
                        final label =
                            '$num · offen ${open.toStringAsFixed(2)} · ${m['contact_name'] ?? m['contact_id'] ?? ''}';
                        return DropdownMenuItem(
                            value: m['id'].toString(), child: Text(label));
                      })
                    ],
                    onChanged: (v) {
                      invoiceCtrl.text = v ?? '';
                    },
                  );
                },
              ),
              const SizedBox(height: 8),
              TextField(
                controller: invoiceCtrl,
                decoration: const InputDecoration(
                    labelText: 'Invoice ID (optional, überschreibt Auswahl)'),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: const Text('Abbrechen')),
          FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: const Text('Match')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      final inv = invoiceCtrl.text.trim();
      await widget.api
          .matchBankStatement(id, invoiceId: inv.isEmpty ? null : inv);
      await _load();
      if (mounted)
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Match gebucht')));
    } catch (e) {
      if (mounted)
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  Widget _glassPanel(Widget child) {
    final scheme = Theme.of(context).colorScheme;
    return ClipRRect(
      borderRadius: BorderRadius.circular(18),
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 14, sigmaY: 14),
        child: Container(
          padding: const EdgeInsets.all(14),
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
    return Scaffold(
      appBar: AppBar(
        title: const Text('Bankauszüge'),
        backgroundColor: Colors.transparent,
        actions: [
          IconButton(onPressed: _load, icon: const Icon(Icons.refresh)),
        ],
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
          child: _glassPanel(
            Column(
              children: [
                if (loading) const LinearProgressIndicator(minHeight: 2),
                Padding(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 4, vertical: 8),
                  child: Row(
                    children: [
                      Text(
                          'Zeile ${offset + 1} - ${offset + items.length} (Limit $limit)'),
                      const Spacer(),
                      IconButton(
                          onPressed: offset == 0 ? null : () => _changePage(-1),
                          icon: const Icon(Icons.chevron_left)),
                      IconButton(
                          onPressed: items.length < limit
                              ? null
                              : () => _changePage(1),
                          icon: const Icon(Icons.chevron_right)),
                    ],
                  ),
                ),
                Expanded(
                  child: ListView.separated(
                    itemCount: items.length,
                    separatorBuilder: (_, __) =>
                        const Divider(height: 1, color: Colors.white24),
                    itemBuilder: (ctx, i) {
                      final it = items[i] as Map<String, dynamic>;
                      final amt = it['amount'];
                      final cur = it['currency'];
                      final ref = (it['reference'] ?? '').toString();
                      final cp = (it['counterparty'] ?? '').toString();
                      final matched =
                          it['matched_payment_id']?.toString() ?? '';
                      return ListTile(
                        title: Text('$amt $cur   ·   $cp'),
                        subtitle:
                            Text(ref.isEmpty ? '(ohne Verwendungszweck)' : ref),
                        trailing: matched.isNotEmpty
                            ? const Chip(
                                label: Text('gematcht'),
                                backgroundColor: Colors.greenAccent)
                            : FilledButton.tonal(
                                onPressed: () => _match(it['id'].toString()),
                                child: const Text('Match'),
                              ),
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
