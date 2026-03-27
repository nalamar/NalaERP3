import 'package:flutter/widgets.dart';

class MaterialSelection {
  const MaterialSelection._({
    required this.materialId,
    required this.label,
    required this.description,
    required this.unit,
  });

  factory MaterialSelection.fromMaterial(Map<String, dynamic> material) {
    final materialId = _normalize(material['id']?.toString());
    if (materialId == null) {
      throw ArgumentError('material.id is required');
    }
    final number = _normalize(material['nummer']?.toString());
    final description = _normalize(material['bezeichnung']?.toString()) ?? '';
    final label = [number, description].whereType<String>().join(' – ');
    return MaterialSelection._(
      materialId: materialId,
      label: label.isEmpty ? materialId : label,
      description: description,
      unit: _normalize(material['einheit']?.toString()),
    );
  }

  final String materialId;
  final String label;
  final String description;
  final String? unit;

  static String? _normalize(String? value) {
    final normalized = value?.trim();
    if (normalized == null || normalized.isEmpty) return null;
    return normalized;
  }
}

MaterialSelection? resolveMaterialSelection(
  List<dynamic> materials,
  String? materialId,
) {
  final normalizedMaterialId = materialId?.trim();
  if (normalizedMaterialId == null || normalizedMaterialId.isEmpty) {
    return null;
  }
  for (final entry in materials) {
    if (entry is! Map<String, dynamic>) continue;
    if (entry['id']?.toString() != normalizedMaterialId) continue;
    try {
      return MaterialSelection.fromMaterial(entry);
    } on ArgumentError {
      return null;
    }
  }
  return null;
}

String materialSelectionLabel(Map<String, dynamic> material) =>
    MaterialSelection.fromMaterial(material).label;

String materialReferenceLabel(Map<String, dynamic> material) {
  final selection = MaterialSelection.fromMaterial(material);
  final referenceLabel = [
    _normalizeMaterialPart(material['nummer']?.toString()),
    selection.description.isEmpty ? null : selection.description,
  ].whereType<String>().join(' • ');
  if (referenceLabel.isNotEmpty) {
    return referenceLabel;
  }
  return selection.label;
}

String resolveMaterialLabel(
  List<dynamic> materials,
  String? materialId, {
  String fallback = '',
}) {
  final selection = resolveMaterialSelection(materials, materialId);
  if (selection != null) {
    return selection.label;
  }
  final normalizedFallback = fallback.trim();
  if (normalizedFallback.isNotEmpty) {
    return normalizedFallback;
  }
  return materialId?.trim() ?? '';
}

String? _normalizeMaterialPart(String? value) {
  final normalized = value?.trim();
  if (normalized == null || normalized.isEmpty) {
    return null;
  }
  return normalized;
}

void applyMaterialSelection({
  required List<dynamic> materials,
  required String? materialId,
  TextEditingController? descriptionController,
  TextEditingController? unitController,
}) {
  final selection = resolveMaterialSelection(materials, materialId);
  if (selection == null) {
    return;
  }
  if (descriptionController != null) {
    descriptionController.text = selection.description;
  }
  if (unitController != null && selection.unit != null) {
    unitController.text = selection.unit!;
  }
}
