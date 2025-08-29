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
  Future<List<dynamic>> listMaterials() async {
    final r = await http.get(_u('/api/v1/materials'));
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
