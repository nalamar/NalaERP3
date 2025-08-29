import 'dart:convert';
import 'dart:html' as html show AnchorElement, window;
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart' show MediaType;

class ApiClient {
  ApiClient({String? baseUrl})
      : baseUrl = baseUrl ?? _defaultBaseUrl();

  final String baseUrl;

  static String _defaultBaseUrl() {
    final loc = html.window.location;
    final hn = (loc.hostname ?? '').trim();
    final host = hn.isEmpty ? 'localhost' : hn;
    // API läuft standardmäßig auf 8080
    return 'http://$host:8080';
  }

  Uri _u(String path, [Map<String, String>? q]) => Uri.parse('$baseUrl$path').replace(queryParameters: q);

  Future<Map<String, dynamic>> version() async {
    final r = await http.get(_u('/version'));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  // Materials
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

  Future<List<dynamic>> stockByMaterial(String id) async {
    final r = await http.get(_u('/api/v1/materials/$id/stock'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // Documents
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

  // Facets
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

  // Contacts API
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

  Future<List<dynamic>> listMaterialDocuments(String materialId) async {
    final r = await http.get(_u('/api/v1/materials/$materialId/documents'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  void downloadDocument(String docId, {String? filename}) {
    final url = _u('/api/v1/documents/$docId').toString();
    final a = html.AnchorElement(href: url);
    if (filename != null && filename.isNotEmpty) a.download = filename;
    a.click();
  }

  // Warehouses & Locations
  Future<List<dynamic>> listWarehouses() async {
    final r = await http.get(_u('/api/v1/warehouses'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> createWarehouse(Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/warehouses/'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
    }

  Future<List<dynamic>> listLocations(String warehouseId) async {
    final r = await http.get(_u('/api/v1/warehouses/$warehouseId/locations'));
    if (r.statusCode != 200) throw Exception('Fehler: ${r.statusCode}');
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> createLocation(String warehouseId, Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/warehouses/$warehouseId/locations'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  // Stock movements
  Future<Map<String, dynamic>> createStockMovement(Map<String, dynamic> body) async {
    final r = await http.post(_u('/api/v1/stock-movements/'), headers: {'Content-Type': 'application/json'}, body: jsonEncode(body));
    if (r.statusCode != 201) throw Exception(utf8.decode(r.bodyBytes));
    return jsonDecode(utf8.decode(r.bodyBytes));
  }
}
