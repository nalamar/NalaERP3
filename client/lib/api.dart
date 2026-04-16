import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart' show MediaType;
import 'web/browser.dart' as browser;

class AuthRequiredException implements Exception {
  const AuthRequiredException([this.message = 'Nicht angemeldet']);
  final String message;

  @override
  String toString() => message;
}

class ApiException implements Exception {
  const ApiException({
    required this.statusCode,
    required this.message,
    this.code,
  });

  final int statusCode;
  final String message;
  final String? code;

  @override
  String toString() => message;
}

class ApiClient {
  ApiClient({String? baseUrl}) : baseUrl = baseUrl ?? _defaultBaseUrl();
  final String baseUrl;
  static const _accessTokenKey = 'nalaerp.access_token';
  static const _refreshTokenKey = 'nalaerp.refresh_token';

  String? _accessToken;
  String? _refreshToken;
  final ValueNotifier<bool> _authenticated = ValueNotifier<bool>(false);
  Set<String> _permissions = const <String>{};

  ValueListenable<bool> get authState => _authenticated;
  Set<String> get permissions => Set.unmodifiable(_permissions);

  bool hasPermission(String permission) => _permissions.contains(permission);

  bool hasAnyPermission(Iterable<String> requiredPermissions) {
    for (final permission in requiredPermissions) {
      if (_permissions.contains(permission)) {
        return true;
      }
    }
    return false;
  }

  static String _defaultBaseUrl() {
    final host = (Uri.base.host.isEmpty ? 'localhost' : Uri.base.host);
    final scheme = (Uri.base.scheme.isEmpty ? 'http' : Uri.base.scheme);
    return '$scheme://$host:8080';
  }

  Uri _u(String path, [Map<String, String>? q]) =>
      Uri.parse('$baseUrl$path').replace(queryParameters: q);

  Map<String, String> _headers([Map<String, String>? extra]) {
    final out = <String, String>{};
    if (_accessToken != null && _accessToken!.isNotEmpty) {
      out['Authorization'] = 'Bearer $_accessToken';
    }
    if (extra != null) out.addAll(extra);
    return out;
  }

  void _restorePersistedTokens() {
    _accessToken ??= browser.readStorage(_accessTokenKey);
    _refreshToken ??= browser.readStorage(_refreshTokenKey);
  }

  void _persistTokens(String accessToken, String refreshToken) {
    _accessToken = accessToken;
    _refreshToken = refreshToken;
    browser.writeStorage(_accessTokenKey, accessToken);
    browser.writeStorage(_refreshTokenKey, refreshToken);
    if (!_authenticated.value) {
      _authenticated.value = true;
    }
  }

  void _clearTokens() {
    _accessToken = null;
    _refreshToken = null;
    _permissions = const <String>{};
    browser.removeStorage(_accessTokenKey);
    browser.removeStorage(_refreshTokenKey);
    if (_authenticated.value) {
      _authenticated.value = false;
    }
  }

  void _storeIdentity(Map<String, dynamic> identity) {
    final rawPermissions = (identity['permissions'] as List?) ?? const [];
    _permissions = rawPermissions.map((e) => e.toString()).toSet();
  }

  Future<http.Response> _sendWithAuth(
    Future<http.Response> Function(Map<String, String> headers) send, {
    bool allowRefresh = true,
  }) async {
    _restorePersistedTokens();
    var response = await send(_headers());
    if (response.statusCode != 401 || !allowRefresh) {
      return response;
    }
    final refreshToken = _refreshToken;
    if (refreshToken == null || refreshToken.isEmpty) {
      _clearTokens();
      throw const AuthRequiredException();
    }
    try {
      await refreshSession();
    } catch (_) {
      _clearTokens();
      throw const AuthRequiredException();
    }
    response = await send(_headers());
    if (response.statusCode == 401) {
      _clearTokens();
      throw const AuthRequiredException();
    }
    return response;
  }

  String _decodeBody(http.Response response) => utf8.decode(response.bodyBytes);

  ApiException _apiExceptionFromResponse(http.Response response,
      {String? fallbackMessage}) {
    final body = _decodeBody(response).trim();
    if (body.isNotEmpty) {
      try {
        final decoded = jsonDecode(body);
        if (decoded is Map<String, dynamic>) {
          final error = decoded['error'];
          if (error is Map) {
            final mapped = error.cast<dynamic, dynamic>();
            final code = mapped['code']?.toString();
            final message = mapped['message']?.toString();
            if (message != null && message.isNotEmpty) {
              return ApiException(
                statusCode: response.statusCode,
                code: code,
                message: message,
              );
            }
          }
        }
      } catch (_) {
        // Fallback below for plain-text or non-JSON responses.
      }
    }
    return ApiException(
      statusCode: response.statusCode,
      message: fallbackMessage ??
          (body.isNotEmpty ? body : 'Fehler: ${response.statusCode}'),
    );
  }

  Never _throwApiException(http.Response response, {String? fallbackMessage}) {
    throw _apiExceptionFromResponse(response, fallbackMessage: fallbackMessage);
  }

  Future<List<dynamic>> _getList(String path, [Map<String, String>? q]) async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u(path, q), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> _getJson(String path,
      [Map<String, String>? q]) async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u(path, q), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<dynamic> _postJson(String path, Object body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u(path),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
    return jsonDecode(_decodeBody(r));
  }

  Future<void> _putJson(String path, Object body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u(path),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
  }

  Future<void> _delete(String path) async {
    final r = await _sendWithAuth(
        (headers) => http.delete(_u(path), headers: headers));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
  }

  Future<http.Response> _getBytesResponse(String path,
      {Map<String, String>? q}) async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u(path, q), headers: headers));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
    return r;
  }

  Future<bool> restoreSession() async {
    _restorePersistedTokens();
    if (_accessToken == null || _refreshToken == null) return false;
    try {
      await getCurrentUser();
      return true;
    } catch (_) {
      try {
        await refreshSession();
        await getCurrentUser();
        return true;
      } catch (_) {
        _clearTokens();
        return false;
      }
    }
  }

  Future<Map<String, dynamic>> login(String login, String password) async {
    final r = await http.post(
      _u('/api/v1/auth/login'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'login': login, 'password': password}),
    );
    if (r.statusCode != 200) {
      _throwApiException(r, fallbackMessage: 'Anmeldung fehlgeschlagen');
    }
    final body = jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
    final data = (body['data'] as Map).cast<String, dynamic>();
    final tokens = (data['tokens'] as Map).cast<String, dynamic>();
    _persistTokens((tokens['access_token'] ?? '').toString(),
        (tokens['refresh_token'] ?? '').toString());
    return data;
  }

  Future<Map<String, dynamic>> refreshSession() async {
    _restorePersistedTokens();
    final token = _refreshToken;
    if (token == null || token.isEmpty) {
      throw const ApiException(
          statusCode: 401,
          code: 'unauthorized',
          message: 'Keine Sitzung vorhanden');
    }
    final r = await http.post(
      _u('/api/v1/auth/refresh'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'refresh_token': token}),
    );
    if (r.statusCode != 200) {
      _clearTokens();
      _throwApiException(r, fallbackMessage: 'Sitzung abgelaufen');
    }
    final body = jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
    final data = (body['data'] as Map).cast<String, dynamic>();
    _persistTokens(
        (data['access_token'] ?? '').toString(), _refreshToken ?? '');
    if ((data['refresh_token'] ?? '').toString().isNotEmpty) {
      _persistTokens((data['access_token'] ?? '').toString(),
          (data['refresh_token'] ?? '').toString());
    }
    return data;
  }

  Future<void> logout() async {
    _restorePersistedTokens();
    final token = _refreshToken;
    try {
      if (token != null && token.isNotEmpty) {
        await http.post(
          _u('/api/v1/auth/logout'),
          headers: {'Content-Type': 'application/json'},
          body: jsonEncode({'refresh_token': token}),
        );
      }
    } finally {
      _clearTokens();
    }
  }

  Future<Map<String, dynamic>> getCurrentUser() async {
    _restorePersistedTokens();
    final r = await _sendWithAuth(
      (headers) => http.get(_u('/api/v1/auth/me'), headers: headers),
    );
    if (r.statusCode != 200) _throwApiException(r);
    final body = jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
    final data = (body['data'] as Map).cast<String, dynamic>();
    _storeIdentity(data);
    return data;
  }

  // -------- Settings: Numbering --------
  Future<Map<String, dynamic>> getNumberingConfig(String entity) =>
      _getJson('/api/v1/settings/numbering/$entity');
  Future<String> previewNumbering(String entity) async {
    final m = await _getJson('/api/v1/settings/numbering/$entity/preview');
    return (m['preview'] ?? '').toString();
  }

  Future<void> updateNumberingPattern(String entity, String pattern) =>
      _putJson('/api/v1/settings/numbering/$entity', {'pattern': pattern});

  // -------- Settings: Company --------
  Future<Map<String, dynamic>> getCompanyProfile() =>
      _getJson('/api/v1/settings/company/');
  Future<void> updateCompanyProfile(Map<String, dynamic> body) =>
      _putJson('/api/v1/settings/company/', body);
  Future<Map<String, dynamic>> getLocalizationSettings() =>
      _getJson('/api/v1/settings/company/localization');
  Future<void> updateLocalizationSettings(Map<String, dynamic> body) =>
      _putJson('/api/v1/settings/company/localization', body);
  Future<Map<String, dynamic>> getBrandingSettings() =>
      _getJson('/api/v1/settings/company/branding');
  Future<void> updateBrandingSettings(Map<String, dynamic> body) =>
      _putJson('/api/v1/settings/company/branding', body);
  Future<List<dynamic>> listCompanyBranches() =>
      _getList('/api/v1/settings/company/branches');
  Future<Map<String, dynamic>> createCompanyBranch(
      Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/settings/company/branches'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> updateCompanyBranch(
      String branchId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.patch(
        _u('/api/v1/settings/company/branches/$branchId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<void> deleteCompanyBranch(String branchId) =>
      _delete('/api/v1/settings/company/branches/$branchId');

  // -------- Settings: PDF Templates --------
  Future<Map<String, dynamic>> getPdfTemplate(String entity) =>
      _getJson('/api/v1/settings/pdf/$entity');
  Future<void> updatePdfTemplate(
    String entity, {
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

  Future<Map<String, dynamic>> uploadPdfImage(
      String entity, String kind, String filename, Uint8List bytes,
      {String? contentType}) async {
    final uri = _u('/api/v1/settings/pdf/$entity/upload/$kind');
    final req = http.MultipartRequest('POST', uri);
    req.headers.addAll(_headers());
    final mediaType = (contentType != null && contentType.isNotEmpty)
        ? MediaType.parse(contentType)
        : MediaType('application', 'octet-stream');
    req.files.add(http.MultipartFile.fromBytes('file', bytes,
        filename: filename, contentType: mediaType));
    final streamed = await req.send();
    final resp = await http.Response.fromStream(streamed);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwApiException(resp);
    }
    return jsonDecode(utf8.decode(resp.bodyBytes)) as Map<String, dynamic>;
  }

  Future<void> deletePdfImage(String entity, String kind) =>
      _delete('/api/v1/settings/pdf/$entity/upload/$kind');

  Future<void> downloadPurchaseOrderPdf(String id, {String? filename}) async {
    final r = await _getBytesResponse('/api/v1/purchase-orders/$id/pdf');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Bestellung_$id.pdf',
      contentType: r.headers['content-type'] ?? 'application/pdf',
    );
  }

  Future<List<dynamic>> listInvoicesOut({
    String? q,
    String? status,
    String? contactId,
    String? sourceSalesOrderId,
    int? limit,
    int? offset,
  }) async {
    final qp = <String, String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (contactId != null && contactId.isNotEmpty) qp['contact_id'] = contactId;
    if (sourceSalesOrderId != null && sourceSalesOrderId.isNotEmpty)
      qp['source_sales_order_id'] = sourceSalesOrderId;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/invoices-out', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> createInvoiceOut(
      Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/invoices-out/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getInvoiceOut(String id) =>
      _getJson('/api/v1/invoices-out/$id');

  Future<Map<String, dynamic>> bookInvoiceOut(String id) async {
    final r = await _sendWithAuth(
      (headers) =>
          http.post(_u('/api/v1/invoices-out/$id/book'), headers: headers),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> addInvoicePayment(
      String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/invoices-out/$id/payments'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<List<dynamic>> listInvoicePayments(String id) =>
      _getList('/api/v1/invoices-out/$id/payments');

  Future<void> downloadInvoiceOutPdf(String id, {String? filename}) async {
    final r = await _getBytesResponse('/api/v1/invoices-out/$id/pdf');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Rechnung_$id.pdf',
      contentType: r.headers['content-type'] ?? 'application/pdf',
    );
  }

  Future<List<dynamic>> listQuotes(
      {String? q,
      String? status,
      String? contactId,
      String? projectId,
      int? limit,
      int? offset}) async {
    final qp = <String, String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (contactId != null && contactId.isNotEmpty) qp['contact_id'] = contactId;
    if (projectId != null && projectId.isNotEmpty) qp['project_id'] = projectId;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/quotes', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> createQuote(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getQuote(String id) =>
      _getJson('/api/v1/quotes/$id');

  Future<Map<String, dynamic>> updateQuote(
      String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.patch(
        _u('/api/v1/quotes/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> applyQuoteMaterialCandidate(
    String quoteId,
    String itemId,
    String materialId,
  ) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/apply-material-candidate'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'material_id': materialId}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> updateQuoteStatus(
      String id, String status) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$id/status'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'status': status}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> acceptQuote(
    String id, {
    String? projectStatus,
  }) async {
    final body = <String, dynamic>{};
    if (projectStatus != null && projectStatus.trim().isNotEmpty) {
      body['project_status'] = projectStatus.trim();
    }
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$id/accept'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> convertQuoteToInvoice(
    String id, {
    String? revenueAccount,
    DateTime? invoiceDate,
    DateTime? dueDate,
  }) async {
    final body = <String, dynamic>{};
    if (revenueAccount != null && revenueAccount.trim().isNotEmpty) {
      body['revenue_account'] = revenueAccount.trim();
    }
    if (invoiceDate != null) {
      body['invoice_date'] = invoiceDate.toUtc().toIso8601String();
    }
    if (dueDate != null) {
      body['due_date'] = dueDate.toUtc().toIso8601String();
    }
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$id/convert-to-invoice'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> convertQuoteToSalesOrder(String id) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$id/convert-to-sales-order'),
        headers: headers,
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<void> downloadQuotePdf(String id, {String? filename}) async {
    final r = await _getBytesResponse('/api/v1/quotes/$id/pdf');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Angebot_$id.pdf',
      contentType: r.headers['content-type'] ?? 'application/pdf',
    );
  }

  Future<Map<String, dynamic>> uploadGAEBQuoteImport(
    String filename,
    Uint8List bytes, {
    required String projectId,
    String? contactId,
    String? contentType,
  }) async {
    final req =
        http.MultipartRequest('POST', _u('/api/v1/quotes/imports/gaeb'));
    req.headers.addAll(_headers());
    req.fields['project_id'] = projectId;
    if (contactId != null && contactId.trim().isNotEmpty) {
      req.fields['contact_id'] = contactId.trim();
    }
    req.files.add(
      http.MultipartFile.fromBytes(
        'file',
        bytes,
        filename: filename,
        contentType: contentType != null ? MediaType.parse(contentType) : null,
      ),
    );
    final streamed = await req.send();
    final r = await http.Response.fromStream(streamed);
    if (r.statusCode != 201) {
      _throwApiException(r, fallbackMessage: 'GAEB-Upload fehlgeschlagen');
    }
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<List<dynamic>> listQuoteImports({
    String? projectId,
    String? contactId,
    int? limit,
    int? offset,
  }) async {
    final qp = <String, String>{};
    if (projectId != null && projectId.isNotEmpty) qp['project_id'] = projectId;
    if (contactId != null && contactId.isNotEmpty) qp['contact_id'] = contactId;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/quotes/imports', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> getQuoteImport(String id) =>
      _getJson('/api/v1/quotes/imports/$id');

  Future<List<dynamic>> listQuoteImportItems(String importId) =>
      _getList('/api/v1/quotes/imports/$importId/items');

  Future<Map<String, dynamic>> getQuoteImportItem(
    String importId,
    String itemId,
  ) =>
      _getJson('/api/v1/quotes/imports/$importId/items/$itemId');

  Future<Map<String, dynamic>> updateQuoteImportItemReview({
    required String importId,
    required String itemId,
    required String reviewStatus,
    String reviewNote = '',
  }) async {
    final r = await _sendWithAuth(
      (headers) => http.patch(
        _u('/api/v1/quotes/imports/$importId/items/$itemId/review'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({
          'review_status': reviewStatus,
          'review_note': reviewNote,
        }),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> markQuoteImportReviewed(String importId) async {
    final r = await _sendWithAuth(
      (headers) => http.patch(
        _u('/api/v1/quotes/imports/$importId/review'),
        headers: headers,
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> applyQuoteImport(String importId) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/imports/$importId/apply'),
        headers: headers,
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<List<dynamic>> listSalesOrders(
      {String? q,
      String? status,
      String? contactId,
      String? projectId,
      int? limit,
      int? offset}) async {
    final qp = <String, String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (contactId != null && contactId.isNotEmpty) qp['contact_id'] = contactId;
    if (projectId != null && projectId.isNotEmpty) qp['project_id'] = projectId;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/sales-orders', qp.isEmpty ? null : qp);
  }

  Future<List<String>> listSalesOrderStatuses() async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/sales-orders/statuses'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(_decodeBody(r)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }

  Future<Map<String, dynamic>> getSalesOrder(String id) =>
      _getJson('/api/v1/sales-orders/$id');

  Future<Map<String, dynamic>> updateSalesOrder(
      String id, Map<String, dynamic> patch) async {
    final r = await _sendWithAuth(
      (headers) => http.patch(
        _u('/api/v1/sales-orders/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(patch),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> createSalesOrderItem(
      String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/sales-orders/$id/items'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> updateSalesOrderItem(
      String id, String itemId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.patch(
        _u('/api/v1/sales-orders/$id/items/$itemId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> deleteSalesOrderItem(
      String id, String itemId) async {
    final r = await _sendWithAuth(
      (headers) => http.delete(_u('/api/v1/sales-orders/$id/items/$itemId'),
          headers: headers),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<void> downloadSalesOrderPdf(String id, {String? filename}) async {
    final r = await _getBytesResponse('/api/v1/sales-orders/$id/pdf');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Auftrag_$id.pdf',
      contentType: r.headers['content-type'] ?? 'application/pdf',
    );
  }

  Future<Map<String, dynamic>> updateSalesOrderStatus(
      String id, String status) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/sales-orders/$id/status'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'status': status}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> convertSalesOrderToInvoice(
    String id, {
    String? revenueAccount,
    DateTime? invoiceDate,
    DateTime? dueDate,
    List<Map<String, dynamic>>? items,
  }) async {
    final body = <String, dynamic>{};
    if (revenueAccount != null && revenueAccount.trim().isNotEmpty) {
      body['revenue_account'] = revenueAccount.trim();
    }
    if (invoiceDate != null) {
      body['invoice_date'] = invoiceDate.toUtc().toIso8601String();
    }
    if (dueDate != null) {
      body['due_date'] = dueDate.toUtc().toIso8601String();
    }
    if (items != null && items.isNotEmpty) {
      body['items'] = items;
    }
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/sales-orders/$id/convert-to-invoice'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  // -------- Projects --------
  Future<List<dynamic>> listProjects(
      {String? q, String? status, int? limit, int? offset}) async {
    final qp = <String, String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/projects', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> createProject(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/projects/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> getProject(String id) =>
      _getJson('/api/v1/projects/$id');
  Future<Map<String, dynamic>> getProjectCommercialContext(String id) =>
      _getJson('/api/v1/projects/$id/commercial-context');
  Future<Map<String, dynamic>> getCommercialWorkflow({
    String? projectId,
    String? contactId,
    String? kind,
  }) {
    final query = <String, String>{};
    if (projectId != null && projectId.trim().isNotEmpty) {
      query['project_id'] = projectId.trim();
    }
    if (contactId != null && contactId.trim().isNotEmpty) {
      query['contact_id'] = contactId.trim();
    }
    if (kind != null && kind.trim().isNotEmpty) {
      query['kind'] = kind.trim();
    }
    return _getJson(
      '/api/v1/workflow/commercial',
      query.isEmpty ? null : query,
    );
  }

  Future<void> downloadProjectQuotePdf(String id, {String? filename}) async {
    final r = await _getBytesResponse('/api/v1/projects/$id/quote-pdf');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Angebot_$id.pdf',
      contentType: r.headers['content-type'] ?? 'application/pdf',
    );
  }

  Future<Map<String, dynamic>> importLogikalProject(
      String filename, Uint8List bytes,
      {String? contentType}) async {
    final uri = _u('/api/v1/projects/import/logikal');
    // 1) Versuche Multipart (stabil im Web, kein Custom-Header nötig)
    try {
      final req = http.MultipartRequest('POST', uri);
      req.headers.addAll(_headers());
      MediaType? mt;
      if (contentType != null && contentType.isNotEmpty) {
        try {
          mt = MediaType.parse(contentType);
        } catch (_) {
          mt = null;
        }
      }
      req.files.add(http.MultipartFile.fromBytes('file', bytes,
          filename: filename, contentType: mt));
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
    final r = await _sendWithAuth((authHeaders) =>
        http.post(uri, headers: {...authHeaders, ...headers}, body: bytes));
    if (r.statusCode != 201) {
      _throwApiException(r, fallbackMessage: 'Import fehlgeschlagen');
    }
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> analyzeLogikalProject(
      String filename, Uint8List bytes,
      {String? contentType}) async {
    final uri = _u('/api/v1/projects/analyze/logikal');
    try {
      final req = http.MultipartRequest('POST', uri);
      req.headers.addAll(_headers());
      MediaType? mt;
      if (contentType != null && contentType.isNotEmpty) {
        try {
          mt = MediaType.parse(contentType);
        } catch (_) {
          mt = null;
        }
      }
      req.files.add(http.MultipartFile.fromBytes('file', bytes,
          filename: filename, contentType: mt));
      final streamed = await req.send();
      final r = await http.Response.fromStream(streamed);
      if (r.statusCode != 200) {
        _throwApiException(r, fallbackMessage: 'Analyse fehlgeschlagen');
      }
      return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
    } catch (_) {
      final headers = <String, String>{
        'Content-Type': 'application/octet-stream',
        'X-Filename': filename
      };
      final r = await _sendWithAuth((authHeaders) =>
          http.post(uri, headers: {...authHeaders, ...headers}, body: bytes));
      if (r.statusCode != 200) {
        _throwApiException(r, fallbackMessage: 'Analyse fehlgeschlagen');
      }
      return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
    }
  }

  // -------- Project tree --------
  Future<List<dynamic>> listProjectPhases(String projectId) =>
      _getList('/api/v1/projects/$projectId/phases');
  Future<Map<String, dynamic>> createPhase(
      String projectId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/projects/$projectId/phases'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> getPhase(String projectId, String phaseId) =>
      _getJson('/api/v1/projects/$projectId/phases/$phaseId');
  Future<Map<String, dynamic>> updatePhase(
      String projectId, String phaseId, Map<String, dynamic> patch) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/projects/$projectId/phases/$phaseId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(patch)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deletePhase(String projectId, String phaseId) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/projects/$projectId/phases/$phaseId'),
        headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<List<dynamic>> listPhaseElevations(String projectId, String phaseId) =>
      _getList('/api/v1/projects/$projectId/phases/$phaseId/elevations');
  Future<Map<String, dynamic>> createElevation(
      String projectId, String phaseId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/projects/$projectId/phases/$phaseId/elevations'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> getElevation(
          String projectId, String phaseId, String elevationId) =>
      _getJson(
          '/api/v1/projects/$projectId/phases/$phaseId/elevations/$elevationId');
  Future<Map<String, dynamic>> updateElevation(String projectId, String phaseId,
      String elevationId, Map<String, dynamic> patch) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/projects/$projectId/phases/$phaseId/elevations/$elevationId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(patch)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deleteElevation(
      String projectId, String phaseId, String elevationId) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/projects/$projectId/phases/$phaseId/elevations/$elevationId'),
        headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<List<dynamic>> listElevationVariants(
          String projectId, String elevationId) =>
      _getList(
          '/api/v1/projects/$projectId/elevations/$elevationId/single-elevations');
  Future<Map<String, dynamic>> createVariant(
      String projectId, String elevationId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/projects/$projectId/elevations/$elevationId/single-elevations'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> getVariant(
          String projectId, String elevationId, String variantId) =>
      _getJson(
          '/api/v1/projects/$projectId/elevations/$elevationId/single-elevations/$variantId');
  Future<Map<String, dynamic>> updateVariant(String projectId,
      String elevationId, String variantId, Map<String, dynamic> patch) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/projects/$projectId/elevations/$elevationId/single-elevations/$variantId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(patch)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deleteVariant(
      String projectId, String elevationId, String variantId) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/projects/$projectId/elevations/$elevationId/single-elevations/$variantId'),
        headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<Map<String, dynamic>> getVariantMaterials(
          String projectId, String variantId) =>
      _getJson(
          '/api/v1/projects/$projectId/single-elevations/$variantId/materials');
  Future<void> linkVariantMaterial(String projectId, String variantId,
      String kind, String itemId, String materialId) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/projects/$projectId/single-elevations/$variantId/materials/$kind/$itemId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'material_id': materialId})));
    if (r.statusCode != 204) _throwApiException(r);
  }

  // -------- Project import logs --------
  Future<List<dynamic>> listProjectImports(String projectId) =>
      _getList('/api/v1/projects/$projectId/imports');
  Future<List<dynamic>> listImportChanges(String projectId, String importId) =>
      _getList('/api/v1/projects/$projectId/imports/$importId/changes');
  Future<void> downloadProjectImportChangesCsv(
      String projectId, String importId,
      {String? filename}) async {
    final r = await _getBytesResponse(
        '/api/v1/projects/$projectId/imports/$importId/changes',
        q: const {'format': 'csv'});
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'import-$importId.csv',
      contentType: r.headers['content-type'] ?? 'text/csv',
    );
  }

  Future<void> downloadProjectImportChangesJson(
      String projectId, String importId,
      {String? filename}) async {
    final r = await _getBytesResponse(
        '/api/v1/projects/$projectId/imports/$importId/changes');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'import-$importId.json',
      contentType: r.headers['content-type'] ?? 'application/json',
    );
  }

  Future<void> undoProjectImport(String projectId, String importId) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/projects/$projectId/imports/$importId/undo'),
        headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
  }

  Future<void> uploadProjectAssets(
      String projectId, String filename, Uint8List bytes) async {
    final uri = _u('/api/v1/projects/$projectId/assets');
    final req = http.MultipartRequest('POST', uri);
    req.headers.addAll(_headers());
    req.files.add(http.MultipartFile.fromBytes('file', bytes,
        filename: filename, contentType: MediaType('application', 'zip')));
    final streamed = await req.send();
    final resp = await http.Response.fromStream(streamed);
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      _throwApiException(resp, fallbackMessage: 'Assets-Upload fehlgeschlagen');
    }
  }

  String projectAssetUrl(String projectId, String relPath) =>
      _u('/api/v1/projects/$projectId/assets', {'path': relPath}).toString();

  // -------- Purchase Orders --------
  Future<List<String>> listPOStatuses() async =>
      (await _getList('/api/v1/purchase-orders/statuses'))
          .map((e) => e.toString())
          .toList();
  Future<List<dynamic>> listPurchaseOrders(
      {String? q, String? status, int? limit, int? offset}) async {
    final qp = <String, String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/purchase-orders', qp.isEmpty ? null : qp);
  }

  Future<dynamic> createPurchaseOrder(Map<String, dynamic> body) =>
      _postJson('/api/v1/purchase-orders/', body);
  Future<Map<String, dynamic>> getPurchaseOrder(String id) =>
      _getJson('/api/v1/purchase-orders/$id');
  Future<Map<String, dynamic>> updatePurchaseOrder(
      String id, Map<String, dynamic> patch) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/purchase-orders/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(patch)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> createPurchaseOrderItem(
      String orderId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/purchase-orders/$orderId/items'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> updatePurchaseOrderItem(
      String orderId, String itemId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/purchase-orders/$orderId/items/$itemId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deletePurchaseOrderItem(String orderId, String itemId) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/purchase-orders/$orderId/items/$itemId'),
        headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  // -------- Materials --------
  Future<List<dynamic>> listMaterials(
      {String? q,
      String? typ,
      String? kategorie,
      int? limit,
      int? offset}) async {
    final qp = <String, String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (typ != null && typ.isNotEmpty) qp['typ'] = typ;
    if (kategorie != null && kategorie.isNotEmpty) qp['kategorie'] = kategorie;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/materials', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> createMaterial(Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/materials/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> getMaterial(String id) async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u('/api/v1/materials/$id'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> updateMaterial(
      String id, Map<String, dynamic> patch) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/materials/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(patch)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deleteMaterial(String id) async {
    final r = await _sendWithAuth((headers) =>
        http.delete(_u('/api/v1/materials/$id'), headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<List<dynamic>> stockByMaterial(String id) async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/materials/$id/stock'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> uploadMaterialDocument(
      String materialId, String filename, Uint8List bytes,
      {String? contentType}) async {
    final req = http.MultipartRequest(
        'POST', _u('/api/v1/materials/$materialId/documents'));
    req.headers.addAll(_headers());
    req.files.add(http.MultipartFile.fromBytes('file', bytes,
        filename: filename,
        contentType:
            contentType != null ? MediaType.parse(contentType) : null));
    final streamed = await req.send();
    final r = await http.Response.fromStream(streamed);
    if (r.statusCode != 201) {
      _throwApiException(r, fallbackMessage: 'Upload fehlgeschlagen');
    }
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<List<String>> listMaterialTypes() async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u('/api/v1/materials/types'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }

  Future<List<String>> listMaterialCategories() async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/materials/categories'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }

  // Settings – Units
  Future<List<Map<String, dynamic>>> listUnits() async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u('/api/v1/settings/units/'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<void> upsertUnit(String code, String name) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/settings/units/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'code': code, 'name': name})));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
  }

  Future<void> deleteUnit(String code) async {
    final r = await _sendWithAuth((headers) =>
        http.delete(_u('/api/v1/settings/units/$code'), headers: headers));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
  }

  Future<List<Map<String, dynamic>>> listMaterialGroups() async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/settings/material-groups/'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<void> upsertMaterialGroup({
    required String code,
    required String name,
    String description = '',
    int sortOrder = 0,
    bool isActive = true,
  }) async {
    final r = await _sendWithAuth((headers) => http.post(
          _u('/api/v1/settings/material-groups/'),
          headers: {...headers, 'Content-Type': 'application/json'},
          body: jsonEncode({
            'code': code,
            'name': name,
            'description': description,
            'sort_order': sortOrder,
            'is_active': isActive,
          }),
        ));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
  }

  Future<void> deleteMaterialGroup(String code) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/settings/material-groups/$code'),
        headers: headers));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
  }

  Future<List<Map<String, dynamic>>> listQuoteTextBlocks() async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/settings/quote-text-blocks/'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<void> upsertQuoteTextBlock({
    String? id,
    required String code,
    required String name,
    required String category,
    required String body,
    int sortOrder = 0,
    bool isActive = true,
  }) async {
    final payload = <String, dynamic>{
      'code': code,
      'name': name,
      'category': category,
      'body': body,
      'sort_order': sortOrder,
      'is_active': isActive,
    };
    if (id != null && id.isNotEmpty) {
      payload['id'] = id;
    }
    final r = await _sendWithAuth((headers) => http.post(
          _u('/api/v1/settings/quote-text-blocks/'),
          headers: {...headers, 'Content-Type': 'application/json'},
          body: jsonEncode(payload),
        ));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
  }

  Future<void> deleteQuoteTextBlock(String id) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/settings/quote-text-blocks/$id'),
        headers: headers));
    if (r.statusCode < 200 || r.statusCode >= 300) {
      _throwApiException(r);
    }
  }

  // -------- Contacts --------
  Future<List<dynamic>> listContacts(
      {String? q,
      String? rolle,
      String? status,
      String? typ,
      int? limit,
      int? offset}) async {
    final qp = <String, String>{};
    if (q != null && q.isNotEmpty) qp['q'] = q;
    if (rolle != null && rolle.isNotEmpty) qp['rolle'] = rolle;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (typ != null && typ.isNotEmpty) qp['typ'] = typ;
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/contacts', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> createContact(Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/contacts/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> getContact(String id) async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u('/api/v1/contacts/$id'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<Map<String, dynamic>> updateContact(
      String id, Map<String, dynamic> patch) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/contacts/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(patch)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deleteContact(String id) async {
    final r = await _sendWithAuth(
        (headers) => http.delete(_u('/api/v1/contacts/$id'), headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<List<String>> listContactRoles() async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u('/api/v1/contacts/roles'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }

  Future<List<String>> listContactStatuses() async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/contacts/statuses'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }

  Future<List<String>> listContactTypes() async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u('/api/v1/contacts/types'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    final arr = jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
    return arr.map((e) => e.toString()).toList();
  }

  Future<Map<String, dynamic>> createContactAddress(
      String contactId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/contacts/$contactId/addresses'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<List<dynamic>> listContactAddresses(String contactId) async {
    final r = await _sendWithAuth((headers) => http
        .get(_u('/api/v1/contacts/$contactId/addresses'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> updateContactAddress(
      String contactId, String addrId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/contacts/$contactId/addresses/$addrId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deleteContactAddress(String contactId, String addrId) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/contacts/$contactId/addresses/$addrId'),
        headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<Map<String, dynamic>> createContactPerson(
      String contactId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/contacts/$contactId/persons'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<List<dynamic>> listContactPersons(String contactId) async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/contacts/$contactId/persons'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> updateContactPerson(
      String contactId, String personId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/contacts/$contactId/persons/$personId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deleteContactPerson(String contactId, String personId) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/contacts/$contactId/persons/$personId'),
        headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<Map<String, dynamic>> createContactNote(
      String contactId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/contacts/$contactId/notes'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<List<dynamic>> listContactNotes(String contactId) async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/contacts/$contactId/notes'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> updateContactNote(
      String contactId, String noteId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/contacts/$contactId/notes/$noteId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deleteContactNote(String contactId, String noteId) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/contacts/$contactId/notes/$noteId'),
        headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<Map<String, dynamic>> createContactTask(
      String contactId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/contacts/$contactId/tasks'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<List<dynamic>> listContactTasks(String contactId) async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/contacts/$contactId/tasks'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<List<dynamic>> listContactActivity(String contactId) async {
    final r = await _sendWithAuth((headers) =>
        http.get(_u('/api/v1/contacts/$contactId/activity'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> getContactCommercialContext(
      String contactId) async {
    final r = await _sendWithAuth((headers) => http.get(
        _u('/api/v1/contacts/$contactId/commercial-context'),
        headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> updateContactTask(
      String contactId, String taskId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.patch(
        _u('/api/v1/contacts/$contactId/tasks/$taskId'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<void> deleteContactTask(String contactId, String taskId) async {
    final r = await _sendWithAuth((headers) => http.delete(
        _u('/api/v1/contacts/$contactId/tasks/$taskId'),
        headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  Future<Map<String, dynamic>> uploadContactDocument(
      String contactId, String filename, Uint8List bytes,
      {String? contentType}) async {
    final req = http.MultipartRequest(
        'POST', _u('/api/v1/contacts/$contactId/documents'));
    req.headers.addAll(_headers());
    req.files.add(http.MultipartFile.fromBytes('file', bytes,
        filename: filename,
        contentType:
            contentType != null ? MediaType.parse(contentType) : null));
    final streamed = await req.send();
    final r = await http.Response.fromStream(streamed);
    if (r.statusCode != 201) {
      _throwApiException(r, fallbackMessage: 'Upload fehlgeschlagen');
    }
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<List<dynamic>> listContactDocuments(String contactId) async {
    final r = await _sendWithAuth((headers) => http
        .get(_u('/api/v1/contacts/$contactId/documents'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // -------- Warehouses & Locations --------
  Future<Map<String, dynamic>> createWarehouse(
      Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/warehouses/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<List<dynamic>> listWarehouses() async {
    final r = await _sendWithAuth(
        (headers) => http.get(_u('/api/v1/warehouses'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<Map<String, dynamic>> createLocation(
      String warehouseId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/warehouses/$warehouseId/locations'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<List<dynamic>> listLocations(String warehouseId) async {
    final r = await _sendWithAuth((headers) => http.get(
        _u('/api/v1/warehouses/$warehouseId/locations'),
        headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // -------- Stock Movements --------
  Future<Map<String, dynamic>> createStockMovement(
      Map<String, dynamic> body) async {
    final r = await _sendWithAuth((headers) => http.post(
        _u('/api/v1/stock-movements/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body)));
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  // -------- Documents --------
  Future<List<dynamic>> listMaterialDocuments(String materialId) async {
    final r = await _sendWithAuth((headers) => http
        .get(_u('/api/v1/materials/$materialId/documents'), headers: headers));
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  Future<void> downloadDocument(String docId, {String? filename}) async {
    final r = await _getBytesResponse('/api/v1/documents/$docId');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Dokument_$docId',
      contentType: r.headers['content-type'] ?? 'application/octet-stream',
    );
  }
}
