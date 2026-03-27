import 'package:flutter/material.dart';

import 'api.dart';

bool canOpenProjectPurchaseOrderFlow(ApiClient api, {required bool canWrite}) =>
    canWrite && api.hasPermission('purchase_orders.write');

bool canOpenProjectStockMovementFlow(ApiClient api) =>
    api.hasPermission('stock_movements.write');

List<Widget> buildProjectMaterialFlowActionButtons({
  required ApiClient api,
  required bool canWrite,
  required VoidCallback onOpenPurchaseOrder,
  required VoidCallback onOpenStockMovement,
}) {
  return [
    if (canOpenProjectPurchaseOrderFlow(api, canWrite: canWrite))
      FilledButton.icon(
        onPressed: onOpenPurchaseOrder,
        icon: const Icon(Icons.shopping_cart_checkout_rounded),
        label: const Text('Bestellung'),
      ),
    if (canOpenProjectStockMovementFlow(api))
      FilledButton.icon(
        onPressed: onOpenStockMovement,
        icon: const Icon(Icons.swap_horiz_rounded),
        label: const Text('Materialbewegung'),
      ),
  ];
}
