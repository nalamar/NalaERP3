Map<String, dynamic> buildPurchaseOrderItemPayload({
  String? materialId,
  required String description,
  required String quantityText,
  required String unitText,
  required String priceText,
  required String currencyText,
  DateTime? deliveryDate,
  bool includeMaterialId = true,
}) {
  final payload = <String, dynamic>{
    'bezeichnung': description.trim(),
    'menge': double.tryParse(quantityText.trim()) ?? 0,
    'einheit': unitText.trim(),
    'preis': double.tryParse(priceText.trim()) ?? 0,
    'waehrung': normalizePurchaseOrderCurrency(currencyText),
  };
  final normalizedMaterialId = materialId?.trim();
  if (includeMaterialId &&
      normalizedMaterialId != null &&
      normalizedMaterialId.isNotEmpty) {
    payload['material_id'] = normalizedMaterialId;
  }
  if (deliveryDate != null) {
    payload['liefertermin'] =
        DateTime(deliveryDate.year, deliveryDate.month, deliveryDate.day)
            .toIso8601String();
  }
  return payload;
}

Map<String, dynamic> buildPurchaseOrderCreatePayload({
  String? supplierId,
  required String number,
  required String currencyText,
  required String status,
  required String note,
  Map<String, dynamic>? itemPayload,
}) {
  return <String, dynamic>{
    'lieferant_id': supplierId,
    'nummer': number.trim(),
    'waehrung': normalizePurchaseOrderCurrency(currencyText),
    'status': status,
    'notiz': note.trim(),
    'positionen': [
      if (itemPayload != null) itemPayload,
    ],
  };
}

String normalizePurchaseOrderCurrency(String currencyText,
    {String fallback = 'EUR'}) {
  final normalized = currencyText.trim().toUpperCase();
  if (normalized.isEmpty) {
    return fallback;
  }
  return normalized;
}
