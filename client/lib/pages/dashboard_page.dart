import 'package:flutter/material.dart';
import '../api.dart';
import '../commercial_destinations.dart';
import '../commercial_context.dart';
import '../widgets/commercial_summary_widgets.dart';
import 'contacts_screen.dart';
import 'settings_page.dart';
import 'projects_page.dart';

class DashboardPage extends StatelessWidget {
  const DashboardPage({
    super.key,
    required this.api,
    required this.onLogout,
    required this.permissions,
  });
  final ApiClient api;
  final Future<void> Function() onLogout;
  final Set<String> permissions;

  bool _can(String permission) => permissions.contains(permission);

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    final cards = <Widget>[
      if (_can('materials.read'))
        _DashCard(
          title: 'Materialwirtschaft',
          icon: Icons.inventory_2_rounded,
          color: color,
          onTap: () {
            Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) =>
                    buildMaterialwirtschaftScreenDestination(api: api),
              ),
            );
          },
        ),
      if (_can('projects.read'))
        _DashCard(
          title: 'Projekte',
          icon: Icons.workspaces_rounded,
          color: color,
          onTap: () {
            Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => ProjectsPage(api: api),
              ),
            );
          },
        ),
      if (_can('quotes.read'))
        _DashCard(
          title: 'Angebote',
          icon: Icons.request_quote_rounded,
          color: color,
          onTap: () {
            Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => buildQuotesPage(api: api),
              ),
            );
          },
        ),
      if (_can('quotes.read') && _can('sales_orders.read'))
        _WorkflowCockpitDashCard(
          api: api,
          color: color,
        ),
      if (_can('invoices_out.read'))
        _DashCard(
          title: 'Rechnungen',
          icon: Icons.receipt_long_rounded,
          color: color,
          onTap: () {
            Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => buildInvoicesPage(api: api),
              ),
            );
          },
        ),
      if (_can('sales_orders.read'))
        _SalesOrdersDashCard(
          api: api,
          color: color,
          onTap: () {
            Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => buildSalesOrdersPage(api: api),
              ),
            );
          },
        ),
      if (_can('contacts.read'))
        _DashCard(
          title: 'Kontakte',
          icon: Icons.people_alt_rounded,
          color: color,
          onTap: () {
            Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => ContactsScreen(api: api),
              ),
            );
          },
        ),
      if (_can('settings.manage'))
        _DashCard(
          title: 'Einstellungen',
          icon: Icons.settings_rounded,
          color: color,
          onTap: () {
            Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => SettingsPage(api: api),
              ),
            );
          },
        ),
    ];

    return Scaffold(
      appBar: AppBar(
        title: const Text('NalaERP3'),
        backgroundColor: color,
        foregroundColor: Colors.white,
        actions: [
          IconButton(
            onPressed: () async => onLogout(),
            icon: const Icon(Icons.logout_rounded),
            tooltip: 'Abmelden',
          ),
        ],
      ),
      body: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            colors: [color.withValues(alpha: 0.08), Colors.white],
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
          ),
        ),
        alignment: Alignment.center,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 900),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: cards.isEmpty
                ? const Center(
                    child: Text(
                        'Für diesen Benutzer sind aktuell keine Bereiche freigeschaltet.'),
                  )
                : Wrap(
                    alignment: WrapAlignment.center,
                    runSpacing: 24,
                    spacing: 24,
                    children: cards,
                  ),
          ),
        ),
      ),
    );
  }
}

class _SalesOrdersDashCard extends StatefulWidget {
  const _SalesOrdersDashCard({
    required this.api,
    required this.color,
    required this.onTap,
  });

  final ApiClient api;
  final Color color;
  final VoidCallback onTap;

  @override
  State<_SalesOrdersDashCard> createState() => _SalesOrdersDashCardState();
}

class _SalesOrdersDashCardState extends State<_SalesOrdersDashCard> {
  late Future<SalesOrderCommercialStats> _statsFuture;

  @override
  void initState() {
    super.initState();
    _statsFuture = _loadStats();
  }

  Future<SalesOrderCommercialStats> _loadStats() async {
    final salesOrders = await widget.api.listSalesOrders(limit: 200);
    return summarizeSalesOrders(salesOrders);
  }

  List<String> _buildSubtitleLines(SalesOrderCommercialStats stats) {
    if (stats.partialCount > 0) {
      return [
        '${stats.partialCount} Teilfaktura offen',
        'Rest ${stats.remainingGross.toStringAsFixed(2)} EUR',
      ];
    }
    if (stats.followUpCount > 0) {
      return [
        '${stats.followUpCount} Aufträge mit Folgebelegen',
        'Keine offene Teilfaktura',
      ];
    }
    return ['Teilfaktura, Folgebelege, PDF'];
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<SalesOrderCommercialStats>(
      future: _statsFuture,
      builder: (context, snapshot) {
        final subtitleLines = switch (snapshot.connectionState) {
          ConnectionState.waiting => ['Teilfaktura wird geladen…'],
          _ when snapshot.hasError => [
              'Teilfaktura-Status aktuell nicht verfügbar'
            ],
          _ => _buildSubtitleLines(
              snapshot.data ?? const SalesOrderCommercialStats(),
            ),
        };

        return _DashCard(
          title: 'Aufträge',
          subtitleWidget: CommercialSummaryText(
            lines: subtitleLines,
            style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
          ),
          icon: Icons.assignment_turned_in_rounded,
          color: widget.color,
          onTap: widget.onTap,
        );
      },
    );
  }
}

class _WorkflowCockpitDashCard extends StatefulWidget {
  const _WorkflowCockpitDashCard({
    required this.api,
    required this.color,
  });

  final ApiClient api;
  final Color color;

  @override
  State<_WorkflowCockpitDashCard> createState() =>
      _WorkflowCockpitDashCardState();
}

class _WorkflowCockpitDashCardState extends State<_WorkflowCockpitDashCard> {
  late Future<Map<String, dynamic>> _workflowFuture;

  @override
  void initState() {
    super.initState();
    _workflowFuture = widget.api.getCommercialWorkflow();
  }

  String _kindLabel(String kind) {
    switch (kind) {
      case 'quote_sent_pending':
        return 'Angebot wartet auf Entscheidung';
      case 'quote_accepted_pending_followup':
        return 'Angebot braucht Folgebeleg';
      case 'sales_order_pending_invoice':
        return 'Auftrag wartet auf Rechnung';
      case 'sales_order_partially_invoiced':
        return 'Auftrag ist teilfakturiert';
      default:
        return 'Offene Folgeaktion';
    }
  }

  String _itemLabel(Map<String, dynamic> item) {
    final number = ((item['quote_number'] ??
                item['sales_order_number'] ??
                item['invoice_number'] ??
                '')
            .toString())
        .trim();
    final contact = (item['contact_name'] ?? '').toString().trim();
    final action = (item['next_action_label'] ?? '').toString().trim();
    final base = _kindLabel((item['kind'] ?? '').toString());
    if (number.isNotEmpty && contact.isNotEmpty) {
      return '$base: $number • $contact';
    }
    if (number.isNotEmpty) {
      return '$base: $number';
    }
    if (contact.isNotEmpty) {
      return '$base: $contact';
    }
    if (action.isNotEmpty) {
      return '$base: $action';
    }
    return base;
  }

  List<String> _buildLines(Map<String, dynamic> data) {
    final items = ((data['items'] as List?) ?? const [])
        .whereType<Map>()
        .map((e) => e.cast<String, dynamic>())
        .toList();
    if (items.isEmpty) {
      return const ['Keine offenen Folgeaktionen'];
    }

    final lines = items.take(3).map(_itemLabel).toList();
    if (items.length > 3) {
      lines.add('+${items.length - 3} weitere');
    }
    return lines;
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>>(
      future: _workflowFuture,
      builder: (context, snapshot) {
        final subtitleLines = switch (snapshot.connectionState) {
          ConnectionState.waiting => ['Workflow wird geladen…'],
          _ when snapshot.hasError => ['Workflow aktuell nicht verfügbar'],
          _ => _buildLines(snapshot.data ?? const <String, dynamic>{}),
        };

        return _DashCard(
          title: 'Offene Folgeaktionen',
          subtitleWidget: CommercialSummaryText(
            lines: subtitleLines,
            style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
          ),
          icon: Icons.alt_route_rounded,
          color: widget.color,
        );
      },
    );
  }
}

class _DashCard extends StatelessWidget {
  const _DashCard(
      {required this.title,
      required this.icon,
      required this.color,
      this.onTap,
      this.subtitleWidget});
  final String title;
  final Widget? subtitleWidget;
  final IconData icon;
  final Color color;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: Ink(
        width: 260,
        height: 160,
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(16),
          boxShadow: [
            BoxShadow(
                color: Colors.black12,
                blurRadius: 12,
                offset: const Offset(0, 6))
          ],
          border: Border.all(color: color.withValues(alpha: 0.2)),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            CircleAvatar(
                backgroundColor: color.withValues(alpha: 0.12),
                radius: 28,
                child: Icon(icon, color: color, size: 30)),
            const SizedBox(height: 12),
            Text(title,
                style:
                    const TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
            if (subtitleWidget != null) ...[
              const SizedBox(height: 6),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 18),
                child: subtitleWidget,
              ),
            ],
          ],
        ),
      ),
    );
  }
}
