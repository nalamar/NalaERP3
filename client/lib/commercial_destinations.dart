import 'api.dart';
import 'commercial_navigation.dart';
import 'pages/invoices_page.dart';
import 'pages/materialwirtschaft_screen.dart';
import 'pages/materials_page.dart';
import 'pages/purchase_orders_page.dart';
import 'pages/quotes_page.dart';
import 'pages/sales_orders_page.dart';
import 'pages/stock_movements_page.dart';
import 'pages/warehouses_page.dart';
import 'project_navigation.dart';

QuotesPage buildQuotesPage({
  required ApiClient api,
  CommercialListContext? initialContext,
  CommercialFilterContext? initialFilters,
  bool openCreateOnStart = false,
}) {
  return QuotesPage(
    api: api,
    initialFilters: initialFilters,
    initialContext: initialContext,
    openCreateOnStart: openCreateOnStart,
  );
}

MaterialsPage buildMaterialsPage({
  required ApiClient api,
  MaterialListContext? initialContext,
  bool openCreateOnStart = false,
}) {
  return MaterialsPage(
    api: api,
    initialContext: initialContext,
    openCreateOnStart: openCreateOnStart,
  );
}

SalesOrdersPage buildSalesOrdersPage({
  required ApiClient api,
  CommercialListContext? initialContext,
  CommercialFilterContext? initialFilters,
}) {
  return SalesOrdersPage(
    api: api,
    initialFilters: initialFilters,
    initialContext: initialContext,
  );
}

InvoicesPage buildInvoicesPage({
  required ApiClient api,
  CommercialListContext? initialContext,
  CommercialFilterContext? initialFilters,
  bool showWorkflowHint = false,
}) {
  return InvoicesPage(
    api: api,
    initialContext: initialContext,
    initialFilters: initialFilters,
    showWorkflowHint: showWorkflowHint,
  );
}

PurchaseOrdersPage buildPurchaseOrdersPage({
  required ApiClient api,
  CommercialListContext? initialContext,
  PurchaseOrderFilterContext? initialFilters,
  PurchaseOrderCreatePrefillContext? initialCreatePrefill,
  bool openCreateOnStart = false,
}) {
  return PurchaseOrdersPage(
    api: api,
    initialContext: initialContext,
    initialFilters: initialFilters,
    initialCreatePrefill: initialCreatePrefill,
    openCreateOnStart: openCreateOnStart,
  );
}

WarehousesPage buildWarehousesPage({
  required ApiClient api,
  WarehouseSelectionContext? initialSelection,
  bool openCreateOnStart = false,
}) {
  return WarehousesPage(
    api: api,
    initialSelection: initialSelection,
    openCreateOnStart: openCreateOnStart,
  );
}

StockMovementsPage buildStockMovementsPage({
  required ApiClient api,
  StockMovementPrefillContext? initialPrefill,
  bool openCreateOnStart = false,
}) {
  return StockMovementsPage(
    api: api,
    initialPrefill: initialPrefill,
    openCreateOnStart: openCreateOnStart,
  );
}

MaterialwirtschaftScreen buildMaterialwirtschaftScreenDestination({
  required ApiClient api,
  int? initialSection,
  CommercialListContext? initialPurchaseOrdersContext,
  PurchaseOrderFilterContext? initialPurchaseOrderFilters,
  PurchaseOrderCreatePrefillContext? initialPurchaseOrderCreatePrefill,
  WarehouseSelectionContext? initialWarehouseSelection,
  StockMovementPrefillContext? initialStockMovementPrefill,
  MaterialListContext? initialMaterialsContext,
}) {
  return buildMaterialwirtschaftScreen(
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

MaterialwirtschaftScreen buildMaterialListDestination({
  required ApiClient api,
  MaterialListContext? initialContext,
}) {
  return buildMaterialwirtschaftScreenDestination(
    api: api,
    initialMaterialsContext: initialContext,
  );
}

MaterialwirtschaftScreen buildMaterialDetailFlowDestination({
  required ApiClient api,
  required MaterialDetailDestinationContext destinationContext,
}) {
  return buildMaterialListDestination(
    api: api,
    initialContext: destinationContext.materialListContext,
  );
}

// Compatibility wrapper for raw material ids; prefer buildMaterialListDestination.
@Deprecated(
    'Use buildMaterialListDestination with MaterialListContext instead.')
MaterialwirtschaftScreen buildMaterialDetailDestination({
  required ApiClient api,
  required String materialId,
}) {
  return buildMaterialDetailFlowDestination(
    api: api,
    destinationContext: _RawMaterialDetailDestinationContext(materialId),
  );
}

// Compatibility wrapper for raw material ids; prefer
// buildMaterialStockMovementCreateDestination.
@Deprecated(
  'Use buildMaterialStockMovementCreateDestination with MaterialNavigationContext instead.',
)
MaterialwirtschaftScreen buildMaterialStockMovementDestination({
  required ApiClient api,
  required String materialId,
  String? reference,
  String? reason,
}) {
  return buildStockMovementCreateDestination(
    api: api,
    initialPrefill: StockMovementPrefillContext.material(
      materialId,
      reference: reference,
      reason: reason,
    ),
  );
}

MaterialwirtschaftScreen buildMaterialStockMovementCreateDestination({
  required ApiClient api,
  required MaterialNavigationContext materialContext,
}) {
  return buildStockMovementCreateDestination(
    api: api,
    initialPrefill: materialContext.stockMovementPrefill,
  );
}

MaterialwirtschaftScreen buildStockMovementCreateDestination({
  required ApiClient api,
  StockMovementPrefillContext? initialPrefill,
}) {
  return buildMaterialwirtschaftScreenDestination(
    api: api,
    initialStockMovementPrefill: initialPrefill,
  );
}

MaterialwirtschaftScreen buildPurchaseOrderListDestination({
  required ApiClient api,
  CommercialListContext? initialContext,
  PurchaseOrderFilterContext? initialFilters,
}) {
  return buildMaterialwirtschaftScreenDestination(
    api: api,
    initialPurchaseOrdersContext: initialContext,
    initialPurchaseOrderFilters: initialFilters,
  );
}

MaterialwirtschaftScreen buildPurchaseOrderCreateDestination({
  required ApiClient api,
  CommercialListContext? initialContext,
  PurchaseOrderFilterContext? initialFilters,
  PurchaseOrderCreatePrefillContext? initialCreatePrefill,
}) {
  return buildMaterialwirtschaftScreenDestination(
    api: api,
    initialPurchaseOrdersContext: initialContext,
    initialPurchaseOrderFilters: initialFilters,
    initialPurchaseOrderCreatePrefill: initialCreatePrefill,
  );
}

MaterialwirtschaftScreen buildPurchaseOrderFlowDestination({
  required ApiClient api,
  required PurchaseOrderDestinationContext destinationContext,
}) {
  return buildPurchaseOrderCreateDestination(
    api: api,
    initialContext: destinationContext.purchaseOrdersContext,
    initialCreatePrefill: destinationContext.purchaseOrderPrefill,
  );
}

MaterialwirtschaftScreen buildMaterialPurchaseOrderCreateDestination({
  required ApiClient api,
  required MaterialNavigationContext materialContext,
}) {
  return buildPurchaseOrderFlowDestination(
    api: api,
    destinationContext: materialContext,
  );
}

MaterialwirtschaftScreen buildProjectPurchaseOrderCreateDestination({
  required ApiClient api,
  required ProjectCommercialNavigationContext projectContext,
}) {
  return buildPurchaseOrderFlowDestination(
    api: api,
    destinationContext: projectContext,
  );
}

MaterialwirtschaftScreen buildLinkedProjectMaterialPurchaseOrderDestination({
  required ApiClient api,
  required LinkedProjectMaterialNavigationContext materialContext,
}) {
  return buildPurchaseOrderFlowDestination(
    api: api,
    destinationContext: materialContext,
  );
}

MaterialwirtschaftScreen buildStockMovementFlowDestination({
  required ApiClient api,
  required StockMovementDestinationContext destinationContext,
}) {
  return buildStockMovementCreateDestination(
    api: api,
    initialPrefill: destinationContext.stockMovementPrefill,
  );
}

MaterialwirtschaftScreen buildLinkedProjectMaterialStockMovementDestination({
  required ApiClient api,
  required LinkedProjectMaterialNavigationContext materialContext,
}) {
  return buildStockMovementFlowDestination(
    api: api,
    destinationContext: materialContext,
  );
}

MaterialwirtschaftScreen buildProjectStockMovementDestination({
  required ApiClient api,
  required ProjectCommercialNavigationContext projectContext,
}) {
  return buildStockMovementFlowDestination(
    api: api,
    destinationContext: projectContext,
  );
}

MaterialwirtschaftScreen buildWarehouseListDestination({
  required ApiClient api,
  WarehouseSelectionContext? initialSelection,
}) {
  return buildMaterialwirtschaftScreenDestination(
    api: api,
    initialWarehouseSelection: initialSelection,
    initialSection: 1,
  );
}

// Compatibility wrapper for raw warehouse ids; prefer buildWarehouseListDestination.
@Deprecated(
  'Use buildWarehouseListDestination with WarehouseSelectionContext instead.',
)
MaterialwirtschaftScreen buildWarehouseSelectionDestination({
  required ApiClient api,
  String? warehouseId,
}) {
  return buildWarehouseListDestination(
    api: api,
    initialSelection: warehouseId == null
        ? null
        : WarehouseSelectionContext.detail(warehouseId),
  );
}

class _RawMaterialDetailDestinationContext
    extends MaterialDetailDestinationContext {
  const _RawMaterialDetailDestinationContext(this.materialId);

  @override
  final String materialId;
}
