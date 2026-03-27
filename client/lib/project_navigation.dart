import 'commercial_navigation.dart';

const String projectFlowReasonLabel = 'Projektbedarf';

String resolveProjectMaterialDisplayTitle(Map<String, dynamic> item) {
  for (final value in [
    _normalize(item['article_code']?.toString()),
    _normalize(item['description']?.toString()),
    _normalize(item['configuration']?.toString()),
    _normalize(item['material_nummer']?.toString()),
  ]) {
    if (value != null) {
      return value;
    }
  }
  return '';
}

String? resolveProjectLinkedMaterialLabel(Map<String, dynamic> item) =>
    _normalize(item['material_nummer']?.toString());

String buildProjectLinkedMaterialSuffix(Map<String, dynamic> item) {
  final linkedLabel = resolveProjectLinkedMaterialLabel(item);
  if (linkedLabel == null) {
    return '';
  }
  return '  •  verknüpft: $linkedLabel';
}

String buildProjectPurchaseOrderNote(String projectLabel) =>
    'Projektbezug: $projectLabel';

String buildProjectPurchaseOrderDescription(String projectLabel) =>
    '$projectFlowReasonLabel: $projectLabel';

String buildLinkedProjectMaterialNote(String? variantLabel) =>
    variantLabel == null
        ? 'Projektmaterial'
        : 'Projektmaterial aus Variante $variantLabel';

class ProjectCommercialNavigationContext
    implements
        PurchaseOrderDestinationContext,
        StockMovementDestinationContext {
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

  @override
  CommercialFilterContext? get projectFilters => projectId == null
      ? null
      : CommercialFilterContext(
          projectId: projectId,
        );

  @override
  CommercialListContext? get purchaseOrdersContext => projectSearchQuery == null
      ? null
      : CommercialListContext.search(projectSearchQuery!);

  @override
  PurchaseOrderCreatePrefillContext? get purchaseOrderPrefill =>
      projectLabel.isEmpty
          ? null
          : PurchaseOrderCreatePrefillContext(
              note: buildProjectPurchaseOrderNote(projectLabel),
              itemDescription:
                  buildProjectPurchaseOrderDescription(projectLabel),
            );

  @override
  StockMovementPrefillContext get stockMovementPrefill =>
      StockMovementPrefillContext.reference(
        reference: projectSearchQuery,
        reason: projectFlowReasonLabel,
      );
}

class LinkedProjectMaterialNavigationContext
    extends MaterialDetailDestinationContext
    implements
        PurchaseOrderDestinationContext,
        StockMovementDestinationContext {
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

  @override
  CommercialListContext? get purchaseOrdersContext => null;

  static LinkedProjectMaterialNavigationContext? fromVariantMaterial({
    required Map<String, dynamic> variant,
    required Map<String, dynamic> item,
    required num? Function(dynamic value) parseNum,
  }) {
    final materialId = _normalize(item['material_id']?.toString());
    if (materialId == null) return null;
    final variantLabel = _normalize(variant['name']?.toString());
    final note = buildLinkedProjectMaterialNote(variantLabel);
    final primaryLabel = resolveProjectMaterialDisplayTitle(item);
    final linkedMaterialLabel = resolveProjectLinkedMaterialLabel(item);
    final description = [
      _normalize(primaryLabel),
      linkedMaterialLabel,
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

  @override
  StockMovementPrefillContext get stockMovementPrefill =>
      StockMovementPrefillContext.reference(
        materialId: materialId,
        reason: projectFlowReasonLabel,
        reference: movementReference,
      );

  @override
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
