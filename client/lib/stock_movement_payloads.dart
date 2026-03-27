Map<String, dynamic> buildStockMovementPayload({
  required String materialId,
  required String warehouseId,
  String? locationId,
  String? batchCode,
  required num quantity,
  required String unit,
  required String type,
  String? reason,
  String? reference,
  num? purchasePrice,
  String? currency,
  String? fallbackCurrency,
}) {
  final normalizedMaterialId = materialId.trim();
  final normalizedWarehouseId = warehouseId.trim();
  if (normalizedMaterialId.isEmpty) {
    throw ArgumentError('materialId is required');
  }
  if (normalizedWarehouseId.isEmpty) {
    throw ArgumentError('warehouseId is required');
  }

  final payload = <String, dynamic>{
    'material_id': normalizedMaterialId,
    'warehouse_id': normalizedWarehouseId,
    'menge': quantity,
    'einheit': unit.trim(),
    'typ': type.trim(),
  };

  _putIfNotEmpty(payload, 'location_id', locationId);
  _putIfNotEmpty(payload, 'batch_code', batchCode);
  _putIfNotEmpty(payload, 'grund', reason);
  _putIfNotEmpty(payload, 'referenz', reference);
  if (purchasePrice != null) {
    payload['ek_preis'] = purchasePrice;
  }

  final normalizedCurrency =
      normalizeStockMovementCurrency(currency, fallback: fallbackCurrency);
  if (normalizedCurrency != null) {
    payload['waehrung'] = normalizedCurrency;
  }

  return payload;
}

String? normalizeStockMovementCurrency(String? currency, {String? fallback}) {
  final normalized = currency?.trim().toUpperCase();
  if (normalized != null && normalized.isNotEmpty) {
    return normalized;
  }
  final normalizedFallback = fallback?.trim().toUpperCase();
  if (normalizedFallback != null && normalizedFallback.isNotEmpty) {
    return normalizedFallback;
  }
  return null;
}

void _putIfNotEmpty(Map<String, dynamic> payload, String key, String? value) {
  final normalized = value?.trim();
  if (normalized == null || normalized.isEmpty) {
    return;
  }
  payload[key] = normalized;
}
