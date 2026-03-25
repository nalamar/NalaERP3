import 'commercial_navigation.dart';

class ProjectCommercialNavigationContext {
  const ProjectCommercialNavigationContext._({
    required this.projectId,
    required this.projectSearchQuery,
    required this.projectLabel,
  });

  factory ProjectCommercialNavigationContext.fromProject(
      Map<String, dynamic> project) {
    final projectId = _normalize(project['id']?.toString());
    final projectNumber = _normalize(project['nummer']?.toString());
    final projectName = _normalize(project['name']?.toString());
    final projectLabel =
        [projectNumber, projectName].whereType<String>().join(' • ');
    return ProjectCommercialNavigationContext._(
      projectId: projectId,
      projectSearchQuery: projectNumber ?? projectName,
      projectLabel: projectLabel,
    );
  }

  final String? projectId;
  final String? projectSearchQuery;
  final String projectLabel;

  CommercialFilterContext? get projectFilters => projectId == null
      ? null
      : CommercialFilterContext(
          projectId: projectId,
        );

  CommercialListContext? get purchaseOrdersContext => projectSearchQuery == null
      ? null
      : CommercialListContext.search(projectSearchQuery!);

  PurchaseOrderCreatePrefillContext? get purchaseOrderPrefill =>
      projectLabel.isEmpty
          ? null
          : PurchaseOrderCreatePrefillContext(
              note: 'Projektbezug: $projectLabel',
              itemDescription: 'Projektbedarf: $projectLabel',
            );

  StockMovementPrefillContext get stockMovementPrefill =>
      StockMovementPrefillContext(
        reference: projectSearchQuery,
        reason: 'Projektbedarf',
      );
}

class LinkedProjectMaterialNavigationContext {
  const LinkedProjectMaterialNavigationContext._({
    required this.materialId,
    required this.note,
    required this.description,
    required this.quantity,
    required this.unit,
    required this.movementReference,
  });

  final String materialId;
  final String note;
  final String description;
  final double quantity;
  final String unit;
  final String movementReference;

  static LinkedProjectMaterialNavigationContext? fromVariantMaterial({
    required Map<String, dynamic> variant,
    required Map<String, dynamic> item,
    required num? Function(dynamic value) parseNum,
  }) {
    final materialId = _normalize(item['material_id']?.toString());
    if (materialId == null) return null;
    final variantLabel = _normalize(variant['name']?.toString());
    final note = variantLabel == null
        ? 'Projektmaterial'
        : 'Projektmaterial aus Variante $variantLabel';
    final description = [
      _normalize(
        (item['article_code'] ?? item['description'] ?? item['configuration'])
            ?.toString(),
      ),
      _normalize(item['material_nummer']?.toString()),
    ].whereType<String>().join(' • ');
    final quantity =
        (parseNum(item['__computed_qty']) ?? parseNum(item['qty']) ?? 1)
            .toDouble();
    final unit = _normalize(item['unit']?.toString()) ?? 'Stk';
    return LinkedProjectMaterialNavigationContext._(
      materialId: materialId,
      note: note,
      description: description.isEmpty ? note : description,
      quantity: quantity,
      unit: unit,
      movementReference: variantLabel ?? note,
    );
  }

  MaterialListContext get materialListContext =>
      MaterialListContext.detail(materialId);

  StockMovementPrefillContext get stockMovementPrefill =>
      StockMovementPrefillContext(
        materialId: materialId,
        reason: 'Projektbedarf',
        reference: movementReference,
      );

  PurchaseOrderCreatePrefillContext get purchaseOrderPrefill =>
      PurchaseOrderCreatePrefillContext(
        note: note,
        itemDescription: description,
        itemMaterialId: materialId,
        itemQuantity: quantity,
        itemUnit: unit,
      );
}

String? _normalize(String? value) {
  final normalized = value?.trim();
  if (normalized == null || normalized.isEmpty) {
    return null;
  }
  return normalized;
}
