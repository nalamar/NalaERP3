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

String resolveProjectMaterialDefaultDescription(
  Map<String, dynamic> item, {
  required String kind,
}) {
  final fallback = switch (kind) {
    'profiles' => 'Profil',
    'articles' => 'Artikel',
    'glass' => 'Glas',
    _ => '',
  };
  final preferredValues = switch (kind) {
    'glass' => [
        _normalize(item['description']?.toString()),
        _normalize(item['configuration']?.toString()),
      ],
    _ => [
        _normalize(item['description']?.toString()),
        _normalize(item['article_code']?.toString()),
      ],
  };
  for (final value in preferredValues) {
    if (value != null) {
      return value;
    }
  }
  return fallback;
}

String resolveProjectMaterialDefaultNumber(
  Map<String, dynamic> item, {
  required String kind,
}) {
  switch (kind) {
    case 'profiles':
    case 'articles':
      final supplier = _normalize(item['supplier_code']?.toString());
      final code = _normalize(item['article_code']?.toString());
      final combinedNumber = [supplier, code].whereType<String>().join('-');
      if (combinedNumber.isNotEmpty) {
        return combinedNumber;
      }
      if (code != null) {
        return code;
      }
      return resolveProjectMaterialDefaultDescription(item, kind: kind)
          .toUpperCase();
    case 'glass':
      final configuration = _normalize(item['configuration']?.toString());
      if (configuration != null) {
        return 'GLAS-$configuration';
      }
      final defaultDescription =
          resolveProjectMaterialDefaultDescription(item, kind: kind);
      return defaultDescription == 'Glas' ? 'GLAS' : 'GLAS-$defaultDescription';
  }
  return '';
}

String buildProjectLinkedMaterialSuffix(Map<String, dynamic> item) {
  final linkedLabel = resolveProjectLinkedMaterialLabel(item);
  if (linkedLabel == null) {
    return '';
  }
  return '  •  verknüpft: $linkedLabel';
}

String buildLinkedProjectMaterialDescription(Map<String, dynamic> item) {
  final primaryLabel = _normalize(resolveProjectMaterialDisplayTitle(item));
  final linkedMaterialLabel = resolveProjectLinkedMaterialLabel(item);
  return [
    primaryLabel,
    linkedMaterialLabel,
  ].whereType<String>().join(' • ');
}

String buildProjectMaterialCandidateTitle(Map<String, dynamic> material) {
  final number = _normalize(material['nummer']?.toString());
  final description = _normalize(material['bezeichnung']?.toString());
  final label = [number, description].whereType<String>().join(' — ');
  if (label.isNotEmpty) {
    return label;
  }
  return _normalize(material['id']?.toString()) ?? '';
}

String buildProjectMaterialCandidateSubtitle(Map<String, dynamic> material) {
  return [
    _normalize(material['typ']?.toString()),
    _normalize(material['einheit']?.toString()),
    _normalize(material['kategorie']?.toString()),
  ].whereType<String>().join(' • ');
}

Map<String, dynamic> mergeProjectMaterialOverrides(
  Map<String, dynamic> base,
  Map<String, dynamic>? overrides,
) {
  if (overrides == null || overrides.isEmpty) {
    return Map<String, dynamic>.from(base);
  }
  return {
    ...base,
    ...overrides,
  };
}

Map<String, dynamic> buildProjectMaterialCreateBody({
  required Map<String, dynamic> defaults,
  required Map<String, dynamic> values,
  required String variantId,
  required String kind,
}) {
  return <String, dynamic>{
    'nummer': (values['nummer'] ?? defaults['nummer']).toString().trim(),
    'bezeichnung':
        (values['bezeichnung'] ?? defaults['bezeichnung']).toString().trim(),
    'typ': (values['typ'] ?? defaults['typ']).toString().trim(),
    'einheit': (values['einheit'] ?? defaults['einheit']).toString().trim(),
    if (_normalize(values['kategorie']?.toString()) != null)
      'kategorie': values['kategorie'].toString().trim(),
    if (_normalize(values['norm']?.toString()) != null)
      'norm': values['norm'].toString().trim(),
    if (_normalize(values['werkstoffnummer']?.toString()) != null)
      'werkstoffnummer': values['werkstoffnummer'].toString().trim(),
    'dichte': double.tryParse('${values['dichte'] ?? 0}') ?? 0,
    'attribute': <String, dynamic>{
      'source': 'logikal-import',
      'variant_id': variantId,
      'kind': kind,
    },
  };
}

bool hasValidProjectMaterialCreateBody(Map<String, dynamic> body) {
  return _normalize(body['nummer']?.toString()) != null &&
      _normalize(body['bezeichnung']?.toString()) != null;
}

String resolveProjectMaterialDuplicateSearchQuery(Map<String, dynamic> body) =>
    _normalize(body['nummer']?.toString()) ?? '';

bool matchesProjectMaterialDuplicateNumber(
  Map<String, dynamic> material,
  String searchQuery,
) {
  final normalizedMaterialNumber =
      _normalize(material['nummer']?.toString())?.toLowerCase();
  final normalizedSearchQuery = _normalize(searchQuery)?.toLowerCase();
  if (normalizedMaterialNumber == null || normalizedSearchQuery == null) {
    return false;
  }
  return normalizedMaterialNumber.contains(normalizedSearchQuery) ||
      normalizedSearchQuery.contains(normalizedMaterialNumber);
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
    final description = buildLinkedProjectMaterialDescription(item);
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
