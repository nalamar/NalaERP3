import 'material_selection.dart';

class CommercialListContext {
  const CommercialListContext({
    this.detailId,
    this.searchQuery,
  });

  const CommercialListContext.detail(String id)
      : detailId = id,
        searchQuery = null;

  const CommercialListContext.search(String query)
      : detailId = null,
        searchQuery = query;

  final String? detailId;
  final String? searchQuery;

  String? get normalizedDetailId {
    final value = detailId?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }

  String? get normalizedSearchQuery {
    final value = searchQuery?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }

  String? get effectiveSearchQuery =>
      normalizedDetailId ?? normalizedSearchQuery;
}

class CommercialFilterContext {
  const CommercialFilterContext({
    this.projectId,
    this.sourceSalesOrderId,
  });

  final String? projectId;
  final String? sourceSalesOrderId;

  String? get normalizedProjectId {
    final value = projectId?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }

  String? get normalizedSourceSalesOrderId {
    final value = sourceSalesOrderId?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }
}

class PurchaseOrderFilterContext {
  const PurchaseOrderFilterContext({
    this.status,
  });

  final String? status;

  String? get normalizedStatus {
    final value = status?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }
}

class PurchaseOrderCreatePrefillContext {
  const PurchaseOrderCreatePrefillContext({
    this.note,
    this.itemDescription,
    this.itemMaterialId,
    this.itemQuantity,
    this.itemUnit,
  });

  const PurchaseOrderCreatePrefillContext.material({
    required String materialLabel,
    required String materialId,
    String? unit,
    num itemQuantity = 1,
  })  : note = 'Materialbezug: $materialLabel',
        itemDescription = materialLabel,
        itemMaterialId = materialId,
        itemQuantity = itemQuantity,
        itemUnit = unit;

  final String? note;
  final String? itemDescription;
  final String? itemMaterialId;
  final num? itemQuantity;
  final String? itemUnit;

  String? get normalizedNote {
    final value = note?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }

  String? get normalizedItemDescription {
    final value = itemDescription?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }

  String? get normalizedItemMaterialId {
    final value = itemMaterialId?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }

  num? get normalizedItemQuantity {
    final value = itemQuantity;
    if (value == null) return null;
    return value <= 0 ? null : value;
  }

  String? get normalizedItemUnit {
    final value = itemUnit?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }
}

abstract class PurchaseOrderDestinationContext {
  CommercialListContext? get purchaseOrdersContext;
  PurchaseOrderCreatePrefillContext? get purchaseOrderPrefill;
}

abstract class StockMovementDestinationContext {
  StockMovementPrefillContext get stockMovementPrefill;
}

abstract class MaterialDetailDestinationContext {
  const MaterialDetailDestinationContext();

  String get materialId;

  MaterialListContext get materialListContext =>
      MaterialListContext.detail(materialId);
}

class WarehouseSelectionContext {
  const WarehouseSelectionContext({
    this.warehouseId,
  });

  const WarehouseSelectionContext.detail(String id) : warehouseId = id;

  final String? warehouseId;

  String? get normalizedWarehouseId {
    final value = warehouseId?.trim();
    if (value == null || value.isEmpty) return null;
    return value;
  }
}

class StockMovementPrefillContext {
  const StockMovementPrefillContext({
    this.materialId,
    this.warehouseId,
    this.locationId,
    this.type,
    this.reason,
    this.reference,
  });

  const StockMovementPrefillContext.material(
    String materialId, {
    this.warehouseId,
    this.locationId,
    this.type,
    this.reason,
    this.reference,
  }) : materialId = materialId;

  const StockMovementPrefillContext.reference({
    String? materialId,
    this.warehouseId,
    this.locationId,
    this.type,
    this.reason,
    this.reference,
  }) : materialId = materialId;

  final String? materialId;
  final String? warehouseId;
  final String? locationId;
  final String? type;
  final String? reason;
  final String? reference;

  String? _normalize(String? value) {
    final normalized = value?.trim();
    if (normalized == null || normalized.isEmpty) return null;
    return normalized;
  }

  String? get normalizedMaterialId => _normalize(materialId);
  String? get normalizedWarehouseId => _normalize(warehouseId);
  String? get normalizedLocationId => _normalize(locationId);
  String? get normalizedType => _normalize(type);
  String? get normalizedReason => _normalize(reason);
  String? get normalizedReference => _normalize(reference);
}

class MaterialListContext {
  const MaterialListContext({
    this.detailId,
    this.searchQuery,
    this.type,
    this.category,
  });

  const MaterialListContext.detail(String id)
      : detailId = id,
        searchQuery = null,
        type = null,
        category = null;

  const MaterialListContext.search(String query)
      : detailId = null,
        searchQuery = query,
        type = null,
        category = null;

  final String? detailId;
  final String? searchQuery;
  final String? type;
  final String? category;

  String? _normalize(String? value) {
    final normalized = value?.trim();
    if (normalized == null || normalized.isEmpty) return null;
    return normalized;
  }

  String? get normalizedDetailId => _normalize(detailId);
  String? get normalizedSearchQuery => _normalize(searchQuery);
  String? get normalizedType => _normalize(type);
  String? get normalizedCategory => _normalize(category);
  String? get effectiveSearchQuery =>
      normalizedDetailId ?? normalizedSearchQuery;
}

class MaterialNavigationContext extends MaterialDetailDestinationContext
    implements
        PurchaseOrderDestinationContext,
        StockMovementDestinationContext {
  const MaterialNavigationContext._({
    required this.materialId,
    required this.materialLabel,
    required this.unit,
  });

  factory MaterialNavigationContext.fromMaterial(
      Map<String, dynamic> material) {
    final materialId = _normalizeValue(material['id']?.toString());
    if (materialId == null) {
      throw ArgumentError('material.id is required');
    }
    return MaterialNavigationContext._(
      materialId: materialId,
      materialLabel: materialReferenceLabel(material),
      unit: _normalizeValue(material['einheit']?.toString()),
    );
  }

  final String materialId;
  final String materialLabel;
  final String? unit;

  @override
  CommercialListContext? get purchaseOrdersContext => null;

  @override
  StockMovementPrefillContext get stockMovementPrefill =>
      StockMovementPrefillContext(
        materialId: materialId,
        reference: materialLabel,
      );

  @override
  PurchaseOrderCreatePrefillContext get purchaseOrderPrefill =>
      PurchaseOrderCreatePrefillContext.material(
        materialLabel: materialLabel,
        materialId: materialId,
        unit: unit,
      );

  static String? _normalizeValue(String? value) {
    final normalized = value?.trim();
    if (normalized == null || normalized.isEmpty) return null;
    return normalized;
  }
}
