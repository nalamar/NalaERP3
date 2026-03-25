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

MaterialwirtschaftScreen buildMaterialDetailDestination({
  required ApiClient api,
  required String materialId,
}) {
  return buildMaterialwirtschaftScreenDestination(
    api: api,
    initialMaterialsContext: MaterialListContext.detail(materialId),
  );
}

MaterialwirtschaftScreen buildMaterialStockMovementDestination({
  required ApiClient api,
  required String materialId,
  String? reference,
  String? reason,
}) {
  return buildMaterialwirtschaftScreenDestination(
    api: api,
    initialStockMovementPrefill: StockMovementPrefillContext(
      materialId: materialId,
      reference: reference,
      reason: reason,
    ),
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

MaterialwirtschaftScreen buildWarehouseSelectionDestination({
  required ApiClient api,
  String? warehouseId,
}) {
  return buildMaterialwirtschaftScreenDestination(
    api: api,
    initialWarehouseSelection: warehouseId == null
        ? null
        : WarehouseSelectionContext(warehouseId: warehouseId),
    initialSection: 1,
  );
}
