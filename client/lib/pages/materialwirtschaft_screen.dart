import 'package:flutter/material.dart';
import '../api.dart';
import '../commercial_destinations.dart';
import '../commercial_navigation.dart';

MaterialwirtschaftScreen buildMaterialwirtschaftScreen({
  required ApiClient api,
  int? initialSection,
  CommercialListContext? initialPurchaseOrdersContext,
  PurchaseOrderFilterContext? initialPurchaseOrderFilters,
  PurchaseOrderCreatePrefillContext? initialPurchaseOrderCreatePrefill,
  WarehouseSelectionContext? initialWarehouseSelection,
  StockMovementPrefillContext? initialStockMovementPrefill,
  MaterialListContext? initialMaterialsContext,
}) {
  return MaterialwirtschaftScreen(
    api: api,
    initialSection: initialSection,
    initialPurchaseOrdersContext: initialPurchaseOrdersContext,
    initialPurchaseOrderFilters: initialPurchaseOrderFilters,
    initialPurchaseOrderCreatePrefill: initialPurchaseOrderCreatePrefill,
    initialWarehouseSelection: initialWarehouseSelection,
    initialStockMovementPrefill: initialStockMovementPrefill,
    initialMaterialsContext: initialMaterialsContext,
  );
}

class MaterialwirtschaftScreen extends StatefulWidget {
  const MaterialwirtschaftScreen({
    super.key,
    required this.api,
    this.initialSection,
    this.initialPurchaseOrdersContext,
    this.initialPurchaseOrderFilters,
    this.initialPurchaseOrderCreatePrefill,
    this.initialWarehouseSelection,
    this.initialStockMovementPrefill,
    this.initialMaterialsContext,
  });
  final ApiClient api;
  final int? initialSection;
  final CommercialListContext? initialPurchaseOrdersContext;
  final PurchaseOrderFilterContext? initialPurchaseOrderFilters;
  final PurchaseOrderCreatePrefillContext? initialPurchaseOrderCreatePrefill;
  final WarehouseSelectionContext? initialWarehouseSelection;
  final StockMovementPrefillContext? initialStockMovementPrefill;
  final MaterialListContext? initialMaterialsContext;

  @override
  State<MaterialwirtschaftScreen> createState() =>
      _MaterialwirtschaftScreenState();
}

class _MaterialwirtschaftScreenState extends State<MaterialwirtschaftScreen> {
  int _section = 0; // 0: Materialien (default)

  @override
  void initState() {
    super.initState();
    _section = widget.initialSection ??
        ((widget.initialPurchaseOrdersContext != null ||
                widget.initialPurchaseOrderFilters != null ||
                widget.initialPurchaseOrderCreatePrefill != null)
            ? 3
            : widget.initialWarehouseSelection != null
                ? 1
                : widget.initialStockMovementPrefill != null
                    ? 2
                    : widget.initialMaterialsContext != null
                        ? 0
                        : 0);
  }

  Widget _bereichsLeiste(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      color: theme.colorScheme.surface,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Row(
        children: [
          Text('Bereich:',
              style: TextStyle(color: theme.colorScheme.onSurface)),
          const SizedBox(width: 10),
          Wrap(spacing: 8, children: [
            FilledButton.tonal(
              onPressed: () => setState(() {
                _section = 0;
              }),
              child: const Text('Materialien'),
            ),
            FilledButton.tonal(
              onPressed: () => setState(() {
                _section = 1;
              }),
              child: const Text('Lager'),
            ),
            FilledButton.tonal(
              onPressed: () => setState(() {
                _section = 2;
              }),
              child: const Text('Bestand'),
            ),
            FilledButton.tonal(
              onPressed: () => setState(() {
                _section = 3;
              }),
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
        child: _section == 0
            ? buildMaterialsPage(
                api: widget.api,
                initialContext: widget.initialMaterialsContext,
              )
            : _section == 1
                ? buildWarehousesPage(
                    api: widget.api,
                    initialSelection: widget.initialWarehouseSelection,
                  )
                : _section == 2
                    ? buildStockMovementsPage(
                        api: widget.api,
                        initialPrefill: widget.initialStockMovementPrefill,
                        openCreateOnStart:
                            widget.initialStockMovementPrefill != null,
                      )
                    : buildPurchaseOrdersPage(
                        api: widget.api,
                        initialContext: widget.initialPurchaseOrdersContext,
                        initialFilters: widget.initialPurchaseOrderFilters,
                        initialCreatePrefill:
                            widget.initialPurchaseOrderCreatePrefill,
                        openCreateOnStart:
                            widget.initialPurchaseOrderCreatePrefill != null,
                      ),
      ),
    );
  }
}
