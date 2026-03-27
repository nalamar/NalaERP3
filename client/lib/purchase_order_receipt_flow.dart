import 'stock_movement_payloads.dart';

List<Map<String, dynamic>> buildPurchaseOrderReceiptStockMovements({
  required Map<String, dynamic> order,
  required List<dynamic> items,
  required String warehouseId,
  String? locationId,
  required String defaultCurrency,
}) {
  final normalizedWarehouseId = warehouseId.trim();
  if (normalizedWarehouseId.isEmpty) {
    throw ArgumentError('warehouseId is required');
  }
  final normalizedLocationId = locationId?.trim();
  final reference = (order['nummer'] ?? '').toString().trim();

  return items
      .whereType<Map<String, dynamic>>()
      .map((item) => buildPurchaseOrderReceiptStockMovement(
            item: item,
            warehouseId: normalizedWarehouseId,
            locationId: normalizedLocationId,
            reference: reference,
            defaultCurrency: defaultCurrency,
          ))
      .whereType<Map<String, dynamic>>()
      .toList();
}

Map<String, dynamic>? buildPurchaseOrderReceiptStockMovement({
  required Map<String, dynamic> item,
  required String warehouseId,
  String? locationId,
  required String reference,
  required String defaultCurrency,
}) {
  final materialId = item['material_id']?.toString().trim();
  if (materialId == null || materialId.isEmpty) {
    return null;
  }
  return buildStockMovementPayload(
    materialId: materialId,
    warehouseId: warehouseId,
    locationId: locationId,
    quantity: item['menge'] as num? ?? 0,
    unit: (item['einheit'] ?? '').toString(),
    type: 'purchase',
    reason: 'Wareneingang',
    reference: reference,
    purchasePrice: item['preis'] as num?,
    currency: item['waehrung']?.toString(),
    fallbackCurrency: defaultCurrency,
  );
}
