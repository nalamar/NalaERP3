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
            colors: [color.withOpacity(0.08), Colors.white],
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

class _DashCard extends StatelessWidget {
  const _DashCard(
      {required this.title,
      required this.icon,
      required this.color,
      required this.onTap,
      this.subtitle,
      this.subtitleWidget});
  final String title;
  final String? subtitle;
  final Widget? subtitleWidget;
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

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
          border: Border.all(color: color.withOpacity(0.2)),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            CircleAvatar(
                backgroundColor: color.withOpacity(0.12),
                radius: 28,
                child: Icon(icon, color: color, size: 30)),
            const SizedBox(height: 12),
            Text(title,
                style:
                    const TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
            if (subtitle != null || subtitleWidget != null) ...[
              const SizedBox(height: 6),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 18),
                child: subtitleWidget ??
                    Text(
                      subtitle!,
                      textAlign: TextAlign.center,
                      style:
                          TextStyle(fontSize: 12, color: Colors.grey.shade700),
                    ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
