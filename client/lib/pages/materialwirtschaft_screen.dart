import 'package:flutter/material.dart';
import '../api.dart';
import 'materials_page.dart';
import 'warehouses_page.dart';
import 'stock_movements_page.dart';
import 'purchase_orders_page.dart';

class MaterialwirtschaftScreen extends StatefulWidget {
  const MaterialwirtschaftScreen({super.key, required this.api});
  final ApiClient api;

  @override
  State<MaterialwirtschaftScreen> createState() => _MaterialwirtschaftScreenState();
}

class _MaterialwirtschaftScreenState extends State<MaterialwirtschaftScreen> {
  int _section = 0; // 0: Materialien (default)

  Widget _bereichsLeiste(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      color: theme.colorScheme.surface,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Row(
        children: [
          Text('Bereich:', style: TextStyle(color: theme.colorScheme.onSurface)),
          const SizedBox(width: 10),
          Wrap(spacing: 8, children: [
            FilledButton.tonal(
              onPressed: () => setState(() { _section = 0; }),
              child: const Text('Materialien'),
            ),
            FilledButton.tonal(
              onPressed: () => setState(() { _section = 1; }),
              child: const Text('Lager'),
            ),
            FilledButton.tonal(
              onPressed: () => setState(() { _section = 2; }),
              child: const Text('Bestand'),
            ),
            FilledButton.tonal(
              onPressed: () => setState(() { _section = 3; }),
              child: const Text('Bestellungen'),
            ),
          ])
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    return Scaffold(
      appBar: AppBar(
        backgroundColor: color,
        foregroundColor: Colors.white,
        title: const Text('Materialwirtschaft'),
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(56),
          child: _bereichsLeiste(context),
        ),
      ),
      body: AnimatedSwitcher(
        duration: const Duration(milliseconds: 250),
        child: _section==0
          ? MaterialsPage(api: widget.api)
          : _section==1
            ? WarehousesPage(api: widget.api)
            : _section==2
              ? StockMovementsPage(api: widget.api)
              : PurchaseOrdersPage(api: widget.api),
      ),
    );
  }
}
