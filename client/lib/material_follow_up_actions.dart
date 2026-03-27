import 'package:flutter/material.dart';

import 'api.dart';

bool canOpenLinkedMaterialDetail(ApiClient api) =>
    api.hasPermission('materials.read');

List<Widget> buildMaterialFollowUpActionButtons({
  required ApiClient api,
  required bool linked,
  required bool canManageLink,
  required bool isBusy,
  required VoidCallback onAdopt,
  required VoidCallback onOpenMaterial,
  required VoidCallback onOpenStockMovement,
  required VoidCallback onOpenPurchaseOrder,
  required VoidCallback onChangeLink,
  required VoidCallback onUnlink,
  bool includeOpenMaterialAction = true,
}) {
  if (!linked) {
    if (!canManageLink) {
      return const <Widget>[];
    }
    return [
      OutlinedButton.icon(
        onPressed: isBusy ? null : onAdopt,
        icon: const Icon(Icons.download_done_rounded),
        label: const Text('Übernehmen'),
      ),
    ];
  }

  return [
    if (includeOpenMaterialAction && canOpenLinkedMaterialDetail(api))
      OutlinedButton(
        onPressed: isBusy ? null : onOpenMaterial,
        child: const Text('Material'),
      ),
    if (api.hasPermission('stock_movements.write'))
      OutlinedButton(
        onPressed: isBusy ? null : onOpenStockMovement,
        child: const Text('Bewegung'),
      ),
    if (api.hasPermission('purchase_orders.write'))
      OutlinedButton(
        onPressed: isBusy ? null : onOpenPurchaseOrder,
        child: const Text('Bestellen'),
      ),
    if (canManageLink)
      OutlinedButton(
        onPressed: isBusy ? null : onChangeLink,
        child: const Text('Ändern'),
      ),
    if (canManageLink)
      OutlinedButton(
        onPressed: isBusy ? null : onUnlink,
        child: const Text('Lösen'),
      ),
  ];
}
