import 'package:flutter/widgets.dart';

import 'material_selection.dart';

typedef PurchaseOrderMaterialSelection = MaterialSelection;

PurchaseOrderMaterialSelection? resolvePurchaseOrderMaterialSelection(
  List<dynamic> materials,
  String? materialId,
) =>
    resolveMaterialSelection(materials, materialId);

void applyPurchaseOrderMaterialSelection({
  required List<dynamic> materials,
  required String? materialId,
  required TextEditingController descriptionController,
  required TextEditingController unitController,
}) {
  applyMaterialSelection(
    materials: materials,
    materialId: materialId,
    descriptionController: descriptionController,
    unitController: unitController,
  );
}
