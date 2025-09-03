import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart' show MediaType;
import 'web/browser.dart' as browser;

class ApiClient {
  ApiClient({String? baseUrl}) : baseUrl = baseUrl ?? _defaultBaseUrl();
  final String baseUrl;

  static String _defaultBaseUrl() {
    final host = (Uri.base.host.isEmpty ? 'localhost' : Uri.base.host);
    final scheme = (Uri.base.scheme.isEmpty ? 'http' : Uri.base.scheme);
    return '$scheme://$host:8080';
  }

  Uri _u(String path, [Map<String, String>? q]) => Uri.parse('$baseUrl$path').replace(queryParameters: q);

  Future<List<dynamic>> _getList(String path, [Map<String, String>? q]) async {
    final r = await http.get(_u(path, q));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }
  Future<Map<String, dynamic>> _getJson(String path, [Map<String, String>? q]) async {
    final r = await http.get(_u(path, q));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }
  Future<dynamic> _postJson(String path, Object body) async {
    final r = await http.post(_u(path), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    }
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> _putJson(String path, Object body) async {
    final r = await http.put(_u(path), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    }
  }
  Future<void> _delete(String path) async {
    final r = await http.delete(_u(path));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    }
  }

  // -------- Settings: Numbering --------
  Future<Map<String, dynamic>> getNumberingConfig(String entity) => _getJson('/api/v1/settings/numbering/$entity');
  Future<String> previewNumbering(String entity) async {
    final m = await _getJson('/api/v1/settings/numbering/$entity/preview');
    return (m['preview'] ?? '').toString();
  }
  Future<void> updateNumberingPattern(String entity, String pattern) => _putJson('/api/v1/settings/numbering/$entity', {'pattern': pattern});

  // -------- Settings: PDF Templates --------
  Future<Map<String, dynamic>> getPdfTemplate(String entity) => _getJson('/api/v1/settings/pdf/$entity');
  Future<void> updatePdfTemplate(String entity, {
    required String headerText,
    required String footerText,
    required double topFirstMm,
    required double topOtherMm,
  }) async {
    await _putJson('/api/v1/settings/pdf/$entity', {
      'header_text': headerText,
      'footer_text': footerText,
      'top_first_mm': topFirstMm,
      'top_other_mm': topOtherMm,
    });
  }
  Future<Map<String, dynamic>> uploadPdfImage(String entity, String kind, String filename, Uint8List bytes, {String? contentType}) async {
    final uri = _u('/api/v1/settings/pdf/$entity/upload/$kind');
    final req = http.MultipartRequest('POST', uri);
    final mediaType = (contentType != null && contentType.isNotEmpty) ? MediaType.parse(contentType) : MediaType('application', 'octet-stream');
    req.files.add(http.MultipartFile.fromBytes('file', bytes, filename: filename, contentType: mediaType));
    final streamed = await req.send();
    final resp = await http.Response.fromStream(streamed);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw Exception('Fehler: ${resp.statusCode} ${utf8.decode(resp.bodyBytes)}');
    }
    return jsonDecode(utf8.decode(resp.bodyBytes)) as Map<String, dynamic>;
  }
  Future<void> deletePdfImage(String entity, String kind) => _delete('/api/v1/settings/pdf/$entity/upload/$kind');

  void downloadPurchaseOrderPdf(String id, {String? filename}) {
    final url = _u('/api/v1/purchase-orders/$id/pdf').toString();
    browser.downloadUrl(url, filename: filename);
  }

  // -------- Projects --------
  Future<List<dynamic>> listProjects({String? q, String? status, int? limit, int? offset}) async {
    final qp = <String,String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/projects', qp.isEmpty ? null : qp);
  }
  Future<Map<String, dynamic>> createProject(Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/projects/'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) { throw Exception(utf8.decode(r.bodyBytes)); }
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> getProject(String id) => _getJson('/api/v1/projects/$id');

  Future<Map<String, dynamic>> importLogikalProject(String filename, Uint8List bytes, {String? contentType}) async {
    final uri = _u('/api/v1/projects/import/logikal');
    // 1) Versuche Multipart (stabil im Web, kein Custom-Header nötig)
    try {
      final req = http.MultipartRequest('POST', uri);
      MediaType? mt;
      if (contentType != null && contentType.isNotEmpty) {
        try { mt = MediaType.parse(contentType); } catch (_) { mt = null; }
      }
      req.files.add(http.MultipartFile.fromBytes('file', bytes, filename: filename, contentType: mt));
      final streamed = await req.send();
      final r = await http.Response.fromStream(streamed);
      if (r.statusCode == 201) {
        return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
      }
      // Fallback auf Raw nur wenn Server nicht 2xx liefert
    } catch (_) {
      // gehe zu Raw
    }
    // 2) Fallback: Raw POST (application/octet-stream) mit X-Filename
    final headers = <String, String>{
      'Content-Type': 'application/octet-stream',
      'X-Filename': filename,
    };
    final r = await http.post(uri, headers: headers, body: bytes);
    if (r.statusCode != 201) {
      throw Exception('Import fehlgeschlagen: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    }
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> analyzeLogikalProject(String filename, Uint8List bytes, {String? contentType}) async {
    final uri = _u('/api/v1/projects/analyze/logikal');
    try {
      final req = http.MultipartRequest('POST', uri);
      MediaType? mt;
      if (contentType != null && contentType.isNotEmpty) {
        try { mt = MediaType.parse(contentType); } catch (_) { mt = null; }
      }
      req.files.add(http.MultipartFile.fromBytes('file', bytes, filename: filename, contentType: mt));
      final streamed = await req.send();
      final r = await http.Response.fromStream(streamed);
      if (r.statusCode != 200) { throw Exception('Analyse fehlgeschlagen: ${r.statusCode} ${utf8.decode(r.bodyBytes)}'); }
      return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
    } catch (_) {
      final headers = <String, String>{ 'Content-Type': 'application/octet-stream', 'X-Filename': filename };
      final r = await http.post(uri, headers: headers, body: bytes);
      if (r.statusCode != 200) { throw Exception('Analyse fehlgeschlagen: ${r.statusCode} ${utf8.decode(r.bodyBytes)}'); }
      return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
    }
  }

  // -------- Project tree --------
  Future<List<dynamic>> listProjectPhases(String projectId) => _getList('/api/v1/projects/$projectId/phases');
  Future<Map<String, dynamic>> createPhase(String projectId, Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/projects/$projectId/phases'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> getPhase(String projectId, String phaseId) => _getJson('/api/v1/projects/$projectId/phases/$phaseId');
  Future<Map<String, dynamic>> updatePhase(String projectId, String phaseId, Map<String, dynamic> patch) async {
    final r = await http.patch(_u('/api/v1/projects/$projectId/phases/$phaseId'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(patch));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> deletePhase(String projectId, String phaseId) async {
    final r = await http.delete(_u('/api/v1/projects/$projectId/phases/$phaseId'));
    if (r.statusCode != 204) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
  }
  Future<List<dynamic>> listPhaseElevations(String projectId, String phaseId) => _getList('/api/v1/projects/$projectId/phases/$phaseId/elevations');
  Future<Map<String, dynamic>> createElevation(String projectId, String phaseId, Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/projects/$projectId/phases/$phaseId/elevations'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> getElevation(String projectId, String phaseId, String elevationId) => _getJson('/api/v1/projects/$projectId/phases/$phaseId/elevations/$elevationId');
  Future<Map<String, dynamic>> updateElevation(String projectId, String phaseId, String elevationId, Map<String, dynamic> patch) async {
    final r = await http.patch(_u('/api/v1/projects/$projectId/phases/$phaseId/elevations/$elevationId'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(patch));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> deleteElevation(String projectId, String phaseId, String elevationId) async {
    final r = await http.delete(_u('/api/v1/projects/$projectId/phases/$phaseId/elevations/$elevationId'));
    if (r.statusCode != 204) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
  }
  Future<List<dynamic>> listElevationVariants(String projectId, String elevationId) => _getList('/api/v1/projects/$projectId/elevations/$elevationId/single-elevations');
  Future<Map<String, dynamic>> createVariant(String projectId, String elevationId, Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/projects/$projectId/elevations/$elevationId/single-elevations'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> getVariant(String projectId, String elevationId, String variantId) => _getJson('/api/v1/projects/$projectId/elevations/$elevationId/single-elevations/$variantId');
  Future<Map<String, dynamic>> updateVariant(String projectId, String elevationId, String variantId, Map<String, dynamic> patch) async {
    final r = await http.patch(_u('/api/v1/projects/$projectId/elevations/$elevationId/single-elevations/$variantId'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(patch));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> deleteVariant(String projectId, String elevationId, String variantId) async {
    final r = await http.delete(_u('/api/v1/projects/$projectId/elevations/$elevationId/single-elevations/$variantId'));
    if (r.statusCode != 204) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
  }
  Future<Map<String, dynamic>> getVariantMaterials(String projectId, String variantId) => _getJson('/api/v1/projects/$projectId/single-elevations/$variantId/materials');
  Future<void> linkVariantMaterial(String projectId, String variantId, String kind, String itemId, String materialId) async {
    final r = await http.patch(_u('/api/v1/projects/$projectId/single-elevations/$variantId/materials/$kind/$itemId'), headers: {'Content-Type': 'application/json'}, body: jsonEncode({'material_id': materialId}));
    if (r.statusCode != 204) { throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}'); }
  }

  // -------- Project import logs --------
  Future<List<dynamic>> listProjectImports(String projectId) => _getList('/api/v1/projects/$projectId/imports');
  Future<List<dynamic>> listImportChanges(String projectId, String importId) => _getList('/api/v1/projects/$projectId/imports/$importId/changes');
  Future<void> uploadProjectAssets(String projectId, String filename, Uint8List bytes) async {
    final uri = _u('/api/v1/projects/$projectId/assets');
    final req = http.MultipartRequest('POST', uri);
    req.files.add(http.MultipartFile.fromBytes('file', bytes, filename: filename, contentType: MediaType('application', 'zip')));
    final streamed = await req.send();
    final resp = await http.Response.fromStream(streamed);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw Exception('Assets-Upload fehlgeschlagen: ${resp.statusCode} ${utf8.decode(resp.bodyBytes)}');
    }
  }
  String projectAssetUrl(String projectId, String relPath) => _u('/api/v1/projects/$projectId/assets', {'path': relPath}).toString();

  // -------- Purchase Orders --------
  Future<List<String>> listPOStatuses() async => (await _getList('/api/v1/purchase-orders/statuses')).map((e)=> e.toString()).toList();
  Future<List<dynamic>> listPurchaseOrders({String? q, String? status, int? limit, int? offset}) async {
    final qp = <String,String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/purchase-orders', qp.isEmpty ? null : qp);
  }
  Future<dynamic> createPurchaseOrder(Map<String, dynamic> body) => _postJson('/api/v1/purchase-orders/', body);
  Future<Map<String, dynamic>> getPurchaseOrder(String id) => _getJson('/api/v1/purchase-orders/$id');
  Future<Map<String, dynamic>> updatePurchaseOrder(String id, Map<String, dynamic> patch) async {
    final r = await http.patch(_u('/api/v1/purchase-orders/$id'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(patch));
    if (r.statusCode != 200) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> createPurchaseOrderItem(String orderId, Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/purchase-orders/$orderId/items'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> updatePurchaseOrderItem(String orderId, String itemId, Map<String, dynamic> body) async {
    final r = await http.patch(_u('/api/v1/purchase-orders/$orderId/items/$itemId'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 200) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> deletePurchaseOrderItem(String orderId, String itemId) async {
    final r = await http.delete(_u('/api/v1/purchase-orders/$orderId/items/$itemId'));
    if (r.statusCode != 204) throw Exception('Fehler: ${r.statusCode}');
  }

  // -------- Materials --------
  Future<List<dynamic>> listMaterials({String? q, String? typ, String? kategorie, int? limit, int? offset}) async {
    final qp = <String,String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (typ != null && typ.isNotEmpty) qp['typ'] = typ;
    if (kategorie != null && kategorie.isNotEmpty) qp['kategorie'] = kategorie;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    final r = await http.get(_u('/api/v1/materials', qp.isEmpty ? null : qp));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }
  Future<Map<String, dynamic>> createMaterial(Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/materials/'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> getMaterial(String id) async {
    final r = await http.get(_u('/api/v1/materials/$id'));
    if (r.statusCode != 200) throw Exception('Nicht gefunden');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> updateMaterial(String id, Map<String, dynamic> patch) async {
    final r = await http.patch(_u('/api/v1/materials/$id'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(patch));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> deleteMaterial(String id) async {
    final r = await http.delete(_u('/api/v1/materials/$id'));
    if (r.statusCode != 204) throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
  }
  Future<List<dynamic>> stockByMaterial(String id) async {
    final r = await http.get(_u('/api/v1/materials/$id/stock'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }
  Future<Map<String, dynamic>> uploadMaterialDocument(String materialId, String filename, Uint8List bytes, {String? contentType}) async {
    final req = http.MultipartRequest('POST', _u('/api/v1/materials/$materialId/documents'));
    req.files.add(http.MultipartFile.fromBytes('file', bytes, filename: filename, contentType: contentType != null ? MediaType.parse(contentType) : null));
    final streamed = await req.send();
    final r = await http.Response.fromStream(streamed);
    if (r.statusCode != 201) {
      throw Exception('Upload fehlgeschlagen: ${r.statusCode} ${r.body}');
    }
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<List<String>> listMaterialTypes() async {
    final r = await http.get(_u('/api/v1/materials/types'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }
  Future<List<String>> listMaterialCategories() async {
    final r = await http.get(_u('/api/v1/materials/categories'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }
  // Settings – Units
  Future<List<Map<String, dynamic>>> listUnits() async {
    final r = await http.get(_u('/api/v1/settings/units/'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e as Map<String, dynamic>).toList();
  }
  Future<void> upsertUnit(String code, String name) async {
    final r = await http.post(_u('/api/v1/settings/units/'), headers: {'Content-Type': 'application/json'}, body: jsonEncode({'code': code, 'name': name}));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    }
  }
  Future<void> deleteUnit(String code) async {
    final r = await http.delete(_u('/api/v1/settings/units/$code'));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      throw Exception('Fehler: ${r.statusCode} ${utf8.decode(r.bodyBytes)}');
    }
  }

  // -------- Contacts --------
  Future<List<dynamic>> listContacts({String? q, String? rolle, String? typ, int? limit, int? offset}) async {
    final qp = <String,String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (rolle != null && rolle.isNotEmpty) qp['rolle'] = rolle;
    if (typ != null && typ.isNotEmpty) qp['typ'] = typ;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    final r = await http.get(_u('/api/v1/contacts', qp.isEmpty ? null : qp));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }
  Future<Map<String, dynamic>> createContact(Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/contacts/'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> getContact(String id) async {
    final r = await http.get(_u('/api/v1/contacts/$id'));
    if (r.statusCode != 200) throw Exception('Nicht gefunden');
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<Map<String, dynamic>> updateContact(String id, Map<String, dynamic> patch) async {
    final r = await http.patch(_u('/api/v1/contacts/$id'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(patch));
    if (r.statusCode != 200) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> deleteContact(String id) async {
    final r = await http.delete(_u('/api/v1/contacts/$id'));
    if (r.statusCode != 204) throw Exception('Fehler: ${r.statusCode}');
  }
  Future<List<String>> listContactRoles() async {
    final r = await http.get(_u('/api/v1/contacts/roles'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }
  Future<List<String>> listContactTypes() async {
    final r = await http.get(_u('/api/v1/contacts/types'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }
  Future<Map<String, dynamic>> createContactAddress(String contactId, Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/contacts/$contactId/addresses'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<List<dynamic>> listContactAddresses(String contactId) async {
    final r = await http.get(_u('/api/v1/contacts/$contactId/addresses'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }
  Future<Map<String, dynamic>> updateContactAddress(String contactId, String addrId, Map<String, dynamic> body) async {
    final r = await http.patch(_u('/api/v1/contacts/$contactId/addresses/$addrId'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 200) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> deleteContactAddress(String contactId, String addrId) async {
    final r = await http.delete(_u('/api/v1/contacts/$contactId/addresses/$addrId'));
    if (r.statusCode != 204) throw Exception('Fehler: ${r.statusCode}');
  }
  Future<Map<String, dynamic>> createContactPerson(String contactId, Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/contacts/$contactId/persons'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<List<dynamic>> listContactPersons(String contactId) async {
    final r = await http.get(_u('/api/v1/contacts/$contactId/persons'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }
  Future<Map<String, dynamic>> updateContactPerson(String contactId, String personId, Map<String, dynamic> body) async {
    final r = await http.patch(_u('/api/v1/contacts/$contactId/persons/$personId'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 200) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<void> deleteContactPerson(String contactId, String personId) async {
    final r = await http.delete(_u('/api/v1/contacts/$contactId/persons/$personId'));
    if (r.statusCode != 204) throw Exception('Fehler: ${r.statusCode}');
  }

  // -------- Warehouses & Locations --------
  Future<Map<String, dynamic>> createWarehouse(Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/warehouses/'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<List<dynamic>> listWarehouses() async {
    final r = await http.get(_u('/api/v1/warehouses'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }
  Future<Map<String, dynamic>> createLocation(String warehouseId, Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/warehouses/$warehouseId/locations'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
  Future<List<dynamic>> listLocations(String warehouseId) async {
    final r = await http.get(_u('/api/v1/warehouses/$warehouseId/locations'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // -------- Stock Movements --------
  Future<Map<String, dynamic>> createStockMovement(Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/stock-movements/'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  // -------- Documents --------
  Future<List<dynamic>> listMaterialDocuments(String materialId) async {
    final r = await http.get(_u('/api/v1/materials/$materialId/documents'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }
  void downloadDocument(String docId, {String? filename}) {
    final url = _u('/api/v1/documents/$docId').toString();
    browser.downloadUrl(url, filename: filename);
  }
}
