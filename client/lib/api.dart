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

  // Eigenes Passwort aendern (Backlog 0.49) - auch, solange der Server den
  // Wechsel erzwingt. Erwartet 204.
  Future<void> changePassword(String currentPassword, String newPassword) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/auth/change-password'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'current_password': currentPassword, 'new_password': newPassword}),
      ),
    );
    if (r.statusCode != 204) _throwApiException(r);
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
  Future<Map<String, dynamic>> getQuoteCalculationSettings() =>
      _getJson('/api/v1/settings/quote-calculation');
  Future<void> updateQuoteCalculationSettings(Map<String, dynamic> body) =>
      _putJson('/api/v1/settings/quote-calculation', body);
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

  Future<List<dynamic>> listQuoteApprovalRework({
    String? projectId,
    String? contactId,
    String? quoteId,
  }) async {
    final qp = <String, String>{};
    if (projectId != null && projectId.isNotEmpty) {
      qp['project_id'] = projectId;
    }
    if (contactId != null && contactId.isNotEmpty) {
      qp['contact_id'] = contactId;
    }
    if (quoteId != null && quoteId.isNotEmpty) {
      qp['quote_id'] = quoteId;
    }
    final response = await _getJson(
      '/api/v1/quotes/approval-rework',
      qp.isEmpty ? null : qp,
    );
    return (response['items'] as List?) ?? const [];
  }

  Future<List<dynamic>> listQuoteApprovalRequests({
    String? projectId,
    String? contactId,
    String? quoteId,
  }) async {
    final qp = <String, String>{};
    if (projectId != null && projectId.isNotEmpty) {
      qp['project_id'] = projectId;
    }
    if (contactId != null && contactId.isNotEmpty) {
      qp['contact_id'] = contactId;
    }
    if (quoteId != null && quoteId.isNotEmpty) {
      qp['quote_id'] = quoteId;
    }
    final response = await _getJson(
      '/api/v1/quotes/approval-requests',
      qp.isEmpty ? null : qp,
    );
    return (response['items'] as List?) ?? const [];
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

  Future<List<dynamic>> searchQuoteItemMaterials(
    String quoteId,
    String itemId, {
    required String query,
  }) async {
    final qp = <String, String>{'q': query};
    return _getList(
      '/api/v1/quotes/$quoteId/items/$itemId/material-search',
      qp,
    );
  }

  Future<Map<String, dynamic>> applyQuoteMaterialSearchResult(
    String quoteId,
    String itemId, {
    required String query,
    required String materialId,
  }) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/apply-material-search-result'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({
          'query': query,
          'material_id': materialId,
        }),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getQuoteItemPriceSuggestion(
    String quoteId,
    String itemId,
  ) async {
    return _getJson('/api/v1/quotes/$quoteId/items/$itemId/price-suggestion');
  }

  Future<List<dynamic>> getQuoteItemPriceHistory(
    String quoteId,
    String itemId,
  ) async {
    return _getList('/api/v1/quotes/$quoteId/items/$itemId/price-history');
  }

  Future<List<dynamic>> getQuoteItemPriceSourcePriority(
    String quoteId,
    String itemId,
  ) async {
    return _getList(
        '/api/v1/quotes/$quoteId/items/$itemId/price-source-priority');
  }

  Future<Map<String, dynamic>> getQuoteItemPriceEvaluation(
    String quoteId,
    String itemId,
  ) async {
    return _getJson('/api/v1/quotes/$quoteId/items/$itemId/price-evaluation');
  }

  Future<Map<String, dynamic>> getQuoteItemPriceDecisionTransparency(
    String quoteId,
    String itemId,
  ) async {
    return _getJson(
        '/api/v1/quotes/$quoteId/items/$itemId/price-decision-transparency');
  }

  Future<List<dynamic>> getQuoteItemPriceDecisionHistory(
    String quoteId,
    String itemId,
  ) async {
    return _getList(
        '/api/v1/quotes/$quoteId/items/$itemId/price-decision-history');
  }

  Future<Map<String, dynamic>> getQuoteItemMarginAnchor(
    String quoteId,
    String itemId,
  ) async {
    return _getJson('/api/v1/quotes/$quoteId/items/$itemId/margin-anchor');
  }

  Future<Map<String, dynamic>> getQuoteItemApprovalHint(
    String quoteId,
    String itemId,
  ) async {
    return _getJson('/api/v1/quotes/$quoteId/items/$itemId/approval-hint');
  }

  Future<List<dynamic>> getQuoteItemApprovalRequests(
    String quoteId,
    String itemId,
  ) async {
    return _getList('/api/v1/quotes/$quoteId/items/$itemId/approval-requests');
  }

  Future<Map<String, dynamic>> getQuoteItemTargetMarginAnchor(
    String quoteId,
    String itemId,
  ) async {
    return _getJson(
        '/api/v1/quotes/$quoteId/items/$itemId/target-margin-anchor');
  }

  Future<Map<String, dynamic>> applyQuoteItemPriceSuggestion(
    String quoteId,
    String itemId,
  ) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/apply-price-suggestion'),
        headers: headers,
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> applyQuoteItemPrimaryPriceSource(
    String quoteId,
    String itemId,
  ) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/apply-primary-price-source'),
        headers: headers,
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> applyQuoteItemTargetPrice(
    String quoteId,
    String itemId,
  ) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/apply-target-price'),
        headers: headers,
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> requestQuoteItemApproval(
    String quoteId,
    String itemId, {
    String comment = '',
  }) async {
    final body = <String, String>{};
    final trimmedComment = comment.trim();
    if (trimmedComment.isNotEmpty) {
      body['comment'] = trimmedComment;
    }
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/approval-requests'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> cancelQuoteItemApprovalRequest(
    String quoteId,
    String itemId,
  ) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/approval-requests/cancel'),
        headers: headers,
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> approveQuoteItemApprovalRequest(
    String quoteId,
    String itemId, {
    String comment = '',
  }) async {
    final body = <String, String>{};
    final trimmedComment = comment.trim();
    if (trimmedComment.isNotEmpty) {
      body['comment'] = trimmedComment;
    }
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/approval-requests/approve'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> rejectQuoteItemApprovalRequest(
    String quoteId,
    String itemId, {
    String comment = '',
  }) async {
    final body = <String, String>{};
    final trimmedComment = comment.trim();
    if (trimmedComment.isNotEmpty) {
      body['comment'] = trimmedComment;
    }
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/approval-requests/reject'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(_decodeBody(r)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> resolveQuoteItemApprovalRework(
    String quoteId,
    String itemId, {
    String comment = '',
  }) async {
    final body = <String, String>{};
    final trimmedComment = comment.trim();
    if (trimmedComment.isNotEmpty) {
      body['comment'] = trimmedComment;
    }
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/$quoteId/items/$itemId/approval-rework/resolve'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
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

  /// [alternatives]: IDs gewählter Alternativpositionen; jede ersetzt im
  /// Auftrag ihre Grundposition (ADR 0042).
  Future<Map<String, dynamic>> convertQuoteToSalesOrder(String id,
      {List<String> alternatives = const []}) async {
    final r = await _sendWithAuth(
      (headers) => alternatives.isEmpty
          ? http.post(
              _u('/api/v1/quotes/$id/convert-to-sales-order'),
              headers: headers,
            )
          : http.post(
              _u('/api/v1/quotes/$id/convert-to-sales-order'),
              headers: {...headers, 'Content-Type': 'application/json'},
              body: jsonEncode({'alternatives': alternatives}),
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

  /// Gliederung und Vorbemerkungen eines GAEB-Importlaufs (Backlog I.2.2b).
  Future<Map<String, dynamic>> getQuoteImportStructure(String importId) =>
      _getJson('/api/v1/quotes/imports/$importId/structure');

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

  Future<Map<String, dynamic>> processGAEBQuoteImport(String importId) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/quotes/imports/$importId/process'),
        headers: headers,
      ),
    );
    if (r.statusCode != 200) {
      _throwApiException(r,
          fallbackMessage: 'GAEB-Import konnte nicht verarbeitet werden');
    }
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

  /// Bedarfspositionen des Quellangebots mit abgerufenen und offenen
  /// Mengen (ADR 0042).
  Future<List<dynamic>> listSalesOrderProvisionalItems(String orderId) =>
      _getList('/api/v1/sales-orders/$orderId/provisional-items');

  /// Legt einen Nachtrag im Entwurf an (ADR 0010).
  Future<Map<String, dynamic>> createSalesOrderAddendum(
          String orderId, String begruendung) async =>
      (await _postJson('/api/v1/sales-orders/$orderId/addenda',
          {'begruendung': begruendung})) as Map<String, dynamic>;

  /// Ruft eine Bedarfsposition in einen Nachtrag im Entwurf ab (ADR 0042).
  Future<Map<String, dynamic>> createSalesOrderAddendumCallOff(
          String orderId, String addendumId,
          {required String quoteItemId, required double qty}) async =>
      (await _postJson(
          '/api/v1/sales-orders/$orderId/addenda/$addendumId/call-offs',
          {'quote_item_id': quoteItemId, 'qty': qty})) as Map<String, dynamic>;

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

  // -------- Zertifikate am Lieferanten (ADR 0035) --------
  Future<List<dynamic>> listSupplierCertificates(String supplierId,
          {bool includeStorniert = false}) =>
      _getList('/api/v1/supplier-certificates/', {
        'lieferant_id': supplierId,
        if (includeStorniert) 'include_storniert': 'true',
      });

  Future<Map<String, dynamic>> createSupplierCertificate(
          Map<String, dynamic> body) async =>
      (await _postJson('/api/v1/supplier-certificates/', body))
          as Map<String, dynamic>;

  Future<Map<String, dynamic>> stornoSupplierCertificate(
          String id, String reason) async =>
      (await _postJson('/api/v1/supplier-certificates/$id/storno',
          {'reason': reason})) as Map<String, dynamic>;

  // Dateien am Zertifikat: PDF/JPEG/PNG bis 10 MB (Server prueft den
  // Inhalt). Bewusst nicht ueber /documents/{id}.
  Future<List<dynamic>> listSupplierCertificateDocuments(
          String certificateId) =>
      _getList('/api/v1/supplier-certificates/$certificateId/documents');

  Future<Map<String, dynamic>> uploadSupplierCertificateDocument(
      String certificateId, String filename, Uint8List bytes,
      {String? contentType}) async {
    MediaType? mt;
    if (contentType != null && contentType.isNotEmpty) {
      try {
        mt = MediaType.parse(contentType);
      } catch (_) {
        mt = null;
      }
    }
    final r = await _sendWithAuth((headers) async {
      final req = http.MultipartRequest('POST',
          _u('/api/v1/supplier-certificates/$certificateId/documents'));
      req.headers.addAll(headers);
      req.files.add(http.MultipartFile.fromBytes('file', bytes,
          filename: filename, contentType: mt));
      return http.Response.fromStream(await req.send());
    });
    if (r.statusCode != 201) {
      _throwApiException(r, fallbackMessage: 'Upload fehlgeschlagen');
    }
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<void> downloadSupplierCertificateDocument(
      String certificateId, String documentId,
      {String? filename}) async {
    final r = await _getBytesResponse(
        '/api/v1/supplier-certificates/$certificateId/documents/$documentId');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Zertifikat_$documentId',
      contentType: r.headers['content-type'] ?? 'application/octet-stream',
    );
  }

  // Lieferpapiere an der Bestellung (ADR 0035 Praezisierung H.4.3b).
  // Bewusst nicht ueber /documents/{id}.
  Future<List<dynamic>> listPurchaseOrderPapers(String orderId,
          {bool includeStorniert = false}) =>
      _getList('/api/v1/purchase-orders/$orderId/papers/',
          includeStorniert ? {'include_storniert': 'true'} : null);

  Future<void> downloadPurchaseOrderPaper(String orderId, String paperId,
      {String? filename}) async {
    final r = await _getBytesResponse(
        '/api/v1/purchase-orders/$orderId/papers/$paperId');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Lieferpapier_$paperId',
      contentType: r.headers['content-type'] ?? 'application/octet-stream',
    );
  }

  // Multipart: Datei im Feld "file", Angaben als Formularfelder; ohne
  // Position gilt das Papier fuer die ganze Bestellung.
  Future<Map<String, dynamic>> uploadPurchaseOrderPaper(
      String orderId, String filename, Uint8List bytes,
      {required String art,
      String? positionId,
      String charge = '',
      String notiz = '',
      String? contentType}) async {
    MediaType? mt;
    if (contentType != null && contentType.isNotEmpty) {
      try {
        mt = MediaType.parse(contentType);
      } catch (_) {
        mt = null;
      }
    }
    final r = await _sendWithAuth((headers) async {
      final req = http.MultipartRequest(
          'POST', _u('/api/v1/purchase-orders/$orderId/papers/'));
      req.headers.addAll(headers);
      req.fields['art'] = art;
      if (positionId != null && positionId.isNotEmpty) {
        req.fields['position_id'] = positionId;
      }
      req.fields['charge'] = charge.trim();
      req.fields['notiz'] = notiz.trim();
      req.files.add(http.MultipartFile.fromBytes('file', bytes,
          filename: filename, contentType: mt));
      return http.Response.fromStream(await req.send());
    });
    if (r.statusCode != 201) {
      _throwApiException(r, fallbackMessage: 'Upload fehlgeschlagen');
    }
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> stornoPurchaseOrderPaper(
          String orderId, String paperId, String reason) async =>
      (await _postJson('/api/v1/purchase-orders/$orderId/papers/$paperId/storno',
          {'reason': reason})) as Map<String, dynamic>;

  // Wareneingang am Stueck: alle Positionen und Status in einer Transaktion
  // im Server (Backlog H.4.z2).
  Future<Map<String, dynamic>> receivePurchaseOrder(String orderId,
      {required String warehouseId,
      String? locationId,
      bool setReceived = true}) async {
    final loc = locationId?.trim() ?? '';
    return (await _postJson('/api/v1/purchase-orders/$orderId/receive', {
      'warehouse_id': warehouseId.trim(),
      if (loc.isNotEmpty) 'location_id': loc,
      'status_received': setReceived,
    })) as Map<String, dynamic>;
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

  // -------- Personal (Backlog F.7, Server-Routen aus F.4.4) --------
  // Der Mitarbeiterstamm ist der Personalabteilung vorbehalten
  // (employees.read / employees.write). Die Liste enthaelt auch inaktive
  // (ausgeschiedene) Mitarbeiter; sortiert nach Nachname, Vorname.
  Future<List<dynamic>> listEmployees({int? limit, int? offset}) {
    final qp = <String, String>{};
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/employees/', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> getEmployee(String id) =>
      _getJson('/api/v1/employees/$id');

  Future<List<dynamic>> listTeams() => _getList('/api/v1/employees/teams');

  // Neue Mitarbeiter sind aktiv, solange nicht ausdruecklich
  // "active": false mitgegeben wird (ADR 0025).
  Future<Map<String, dynamic>> createEmployee(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/employees/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Legt Mitarbeiter, Anforderungen und vorhandene Nachweise in einem
  // Aufruf an, ganz oder gar nicht (Backlog F.10.5, Server F.10.5.1). Body:
  // employee, required_type_ids, qualifications (type_id, erworben_am
  // JJJJ-MM-TT). Liefert {employee, requirements, qualifications}. Braucht
  // employees.write und qualifications.write.
  Future<Map<String, dynamic>> createEmployeeWithQualifications(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/employees/with-qualifications'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Aendert nur die genannten Felder (PATCH). Zulaessig sind first_name,
  // last_name, email, phone, role, location, cost_center, active und
  // team_id; jedes andere Feld lehnt der Server ab. Liefert den
  // Mitarbeiter nach der Aenderung.
  Future<Map<String, dynamic>> updateEmployee(
      String id, Map<String, dynamic> patch) async {
    final r = await _sendWithAuth(
      (headers) => http.patch(
        _u('/api/v1/employees/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(patch),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Logins des Mandanten zur Auswahl beim Verknuepfen (F.7.2.1): user_id,
  // email, name und - falls schon verknuepft - employee_id. Verlangt
  // employees.write.
  Future<List<dynamic>> listLoginUsers() =>
      _getList('/api/v1/employees/login-users');

  // Verknuepft einen Mitarbeiter mit einem Login (userId) oder loest die
  // Verknuepfung (userId == null). Liefert den Mitarbeiter danach.
  Future<Map<String, dynamic>> setEmployeeUser(
      String employeeId, String? userId) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/employees/$employeeId/user'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'user_id': userId}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Qualifikationen (Backlog F.7.3, Server-Routen aus F.2.5) --------
  // Katalog der Arten; ohne includeInactive nur die aktiven.
  Future<List<dynamic>> listQualificationTypes({bool includeInactive = false}) =>
      _getList('/api/v1/qualification-types/',
          includeInactive ? {'include_inactive': 'true'} : null);

  // Stand eines Mitarbeiters: je Art fehlt / abgelaufen / laeuft_ab /
  // gueltig, das Dringendste zuerst. Fremde Mitarbeiter mit
  // qualifications.read.
  Future<List<dynamic>> getQualificationStanding(String employeeId) =>
      _getList('/api/v1/qualifications/standing', {'employee_id': employeeId});

  // Anforderungen der Stelle eines Mitarbeiters (auch auf inzwischen
  // stillgelegte Arten, type_aktiv=false).
  Future<List<dynamic>> listQualificationRequirements(String employeeId) =>
      _getList('/api/v1/qualifications/requirements', {'employee_id': employeeId});

  // Ersetzt die Anforderungen VOLLSTAENDIG durch typeIds (leere Liste =
  // keine). Liefert die Anforderungen danach.
  Future<List<dynamic>> setQualificationRequirements(
      String employeeId, List<String> typeIds) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/qualifications/requirements', {'employee_id': employeeId}),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'type_ids': typeIds}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // Nachweise eines Mitarbeiters, die juengsten zuerst; stornierte nur auf
  // Wunsch. Die Nachweise tragen nur die type_id - den Namen liefert der
  // Katalog (auch stillgelegte Arten).
  Future<List<dynamic>> listEmployeeQualifications(String employeeId,
      {bool includeStorniert = false}) {
    final qp = <String, String>{'employee_id': employeeId};
    if (includeStorniert) qp['include_storniert'] = 'true';
    return _getList('/api/v1/qualifications/', qp);
  }

  // Erfasst einen Nachweis fuer einen oder mehrere Mitarbeiter (body:
  // employee_ids, type_id, erworben_am JJJJ-MM-TT, optional gueltig_bis,
  // aussteller, nachweis_nr, note). Liefert je Mitarbeiter den Nachweis.
  Future<List<dynamic>> createQualification(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/qualifications/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // Storniert einen Nachweis (einmalig, mit Begruendung).
  Future<Map<String, dynamic>> stornoQualification(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/qualifications/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Hinweise fuer den angemeldeten Benutzer (Backlog F.10.4.2, Server
  // F.10.4.1): je Art Anzahlen nach Stand, nur Bereiche, die er sehen darf.
  // Keine Namen - die stehen in den Listen, die ein Hinweis oeffnet.
  Future<List<dynamic>> listNotices() => _getList('/api/v1/notices');

  // Zwischenbestaetigungen eines Nachweises (Backlog F.10.3.4, Server-Routen
  // aus F.10.3.3). Lesen wie der Nachweis selbst, erfassen und stornieren
  // mit qualifications.write. Die juengsten zuerst.
  Future<List<dynamic>> listQualificationConfirmations(String qualificationId,
      {bool includeStorniert = false}) =>
      _getList('/api/v1/qualifications/$qualificationId/confirmations',
          includeStorniert ? {'include_storniert': 'true'} : null);

  // body: bestaetigt_am JJJJ-MM-TT, bestaetigt_durch, optional note.
  Future<Map<String, dynamic>> createQualificationConfirmation(
      String qualificationId, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/qualifications/$qualificationId/confirmations'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Storniert eine Bestaetigung (einmalig, mit Begruendung).
  Future<Map<String, dynamic>> stornoQualificationConfirmation(
      String confirmationId, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/qualifications/confirmations/$confirmationId/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Nachweisdokumente (Backlog F.10.2, Server-Routen aus F.10.1). Lesen wie
  // der Nachweis selbst, hochladen mit qualifications.write. Der Server
  // nimmt nur PDF/JPEG/PNG bis 10 MB an und erkennt die Art am Inhalt.
  Future<List<dynamic>> listQualificationDocuments(String qualificationId) =>
      _getList('/api/v1/qualifications/$qualificationId/documents');

  // Die Multipart-Anfrage wird je Versuch neu gebaut, damit nach einer
  // Token-Erneuerung (_sendWithAuth) erneut gesendet werden kann.
  Future<Map<String, dynamic>> uploadQualificationDocument(
      String qualificationId, String filename, Uint8List bytes,
      {String? contentType}) async {
    MediaType? mt;
    if (contentType != null && contentType.isNotEmpty) {
      try {
        mt = MediaType.parse(contentType);
      } catch (_) {
        mt = null;
      }
    }
    final r = await _sendWithAuth((headers) async {
      final req = http.MultipartRequest(
          'POST', _u('/api/v1/qualifications/$qualificationId/documents'));
      req.headers.addAll(headers);
      req.files.add(http.MultipartFile.fromBytes('file', bytes,
          filename: filename, contentType: mt));
      return http.Response.fromStream(await req.send());
    });
    if (r.statusCode != 201) {
      _throwApiException(r, fallbackMessage: 'Upload fehlgeschlagen');
    }
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Bewusst nicht ueber downloadDocument (/documents/{id}) - den liefert
  // der Server fuer Nachweisdokumente nicht aus.
  Future<void> downloadQualificationDocument(
      String qualificationId, String documentId,
      {String? filename}) async {
    final r = await _getBytesResponse(
        '/api/v1/qualifications/$qualificationId/documents/$documentId');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Nachweis_$documentId',
      contentType: r.headers['content-type'] ?? 'application/octet-stream',
    );
  }

  // -------- Betriebsmittel (Backlog F.7.4, Server-Routen aus F.3.6.2) --------
  // Ausgaben von Betriebsmitteln, die neuesten zuerst. Mit assets.read alle,
  // mit assets.self nur die eigenen.
  Future<List<dynamic>> listAssetAssignments({
    String? assetId,
    String? employeeId,
    bool nurOffen = false,
    bool includeStorniert = false,
  }) {
    final qp = <String, String>{};
    if (assetId != null && assetId.isNotEmpty) qp['asset_id'] = assetId;
    if (employeeId != null && employeeId.isNotEmpty) qp['employee_id'] = employeeId;
    if (nurOffen) qp['nur_offen'] = 'true';
    if (includeStorniert) qp['include_storniert'] = 'true';
    return _getList('/api/v1/asset-assignments/', qp.isEmpty ? null : qp);
  }

  // -------- Zeiterfassung (Backlog F.5, Server-Routen aus F.1) --------
  // Eintraege und Tagesauswertung fuer die Arbeitstage from bis to (beide
  // einschliesslich, JJJJ-MM-TT). Ohne employeeId die eigenen
  // (time_entries.self); fremde mit time_entries.read. Liefert
  // {"entries": [...], "days": [...]}.
  Future<Map<String, dynamic>> listTimeEntries({
    required String from,
    required String to,
    String? employeeId,
    bool includeStorniert = false,
  }) {
    final qp = <String, String>{'from': from, 'to': to};
    if (employeeId != null && employeeId.isNotEmpty) qp['employee_id'] = employeeId;
    if (includeStorniert) qp['include_storniert'] = 'true';
    return _getJson('/api/v1/time-entries/', qp);
  }

  // Kommen: stempelt fuer den eigenen Mitarbeiter mit der Uhr des SERVERS
  // (ohne started_at) - die Uhr des Geraets ist dafuer keine verlaessliche
  // Quelle.
  Future<Map<String, dynamic>> clockIn() async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/time-entries/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(<String, dynamic>{}),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Gehen: beendet den laufenden Eintrag mit der Uhr des Servers und den
  // angegebenen Pausenminuten.
  Future<Map<String, dynamic>> clockOut(String entryId, {int breakMinutes = 0}) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/time-entries/$entryId/stop'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'break_minutes': breakMinutes}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Nachtragen: Eintrag mit Beginn und Ende (RFC 3339, UTC). Ohne
  // employeeId fuer den eigenen Mitarbeiter.
  Future<Map<String, dynamic>> createTimeEntry({
    required String startedAt,
    required String endedAt,
    int breakMinutes = 0,
    String note = '',
    String? employeeId,
  }) async {
    final body = <String, dynamic>{
      'started_at': startedAt,
      'ended_at': endedAt,
      'break_minutes': breakMinutes,
      'note': note,
    };
    if (employeeId != null && employeeId.isNotEmpty) body['employee_id'] = employeeId;
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/time-entries/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Korrigieren: storniert den Eintrag und legt den Ersatz an (mit
  // Begruendung). Liefert den NEUEN Eintrag.
  Future<Map<String, dynamic>> correctTimeEntry(
    String id, {
    required String startedAt,
    required String endedAt,
    int breakMinutes = 0,
    String note = '',
    required String reason,
  }) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/time-entries/$id/correct'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({
          'started_at': startedAt,
          'ended_at': endedAt,
          'break_minutes': breakMinutes,
          'note': note,
          'reason': reason,
        }),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Stornieren ohne Ersatz (z. B. Fehlstempelung), mit Begruendung.
  Future<Map<String, dynamic>> stornoTimeEntry(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/time-entries/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Qualifikationskatalog pflegen (Backlog F.13.1, Routen F.2.5.1) --------
  // Neue Art: code, name, kategorie, gueltigkeit_monate (null =
  // unbefristet), bestaetigung_monate (null = keine Zwischenbestaetigung),
  // pflicht_fuer_alle, hinweis. Erwartet 201.
  Future<Map<String, dynamic>> createQualificationType(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/qualification-types/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Ersetzt eine Art VOLLSTAENDIG. Der Server verlangt alle sieben Felder
  // (name, kategorie, gueltigkeit_monate, bestaetigung_monate,
  // pflicht_fuer_alle, aktiv, hinweis); der Code ist nicht aenderbar.
  Future<Map<String, dynamic>> updateQualificationType(String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/qualification-types/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Uebernimmt den Vorschlagskatalog; liefert die NEU angelegten Arten
  // (bereits vorhandene Codes bleiben unberuehrt).
  Future<List<dynamic>> adoptDefaultQualificationTypes() async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/qualification-types/adopt-defaults'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(<String, dynamic>{}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // -------- Stellen (Backlog F.11.5, Server-Routen aus F.11.2/F.11.4) --------
  // Stellen als Vorlagen fuer erforderliche Qualifikationen (ADR 0031).
  // Lesen mit einem der Qualifikationsrechte, pflegen mit
  // qualifications.write. Ohne includeInactive nur die aktiven.
  Future<List<dynamic>> listPositions({bool includeInactive = false}) =>
      _getList('/api/v1/positions/', includeInactive ? {'include_inactive': 'true'} : null);

  // body: code, name, hinweis, type_ids.
  Future<Map<String, dynamic>> createPosition(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/positions/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // PUT ersetzt ALLES: name, aktiv, hinweis, type_ids sind Pflicht, code
  // ist nicht aenderbar.
  Future<Map<String, dynamic>> updatePosition(String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/positions/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Stellen eines Mitarbeiters (F.11.6, Route F.11.3.1), nach Name, auch
  // stillgelegte (aktiv=false; sie zaehlen im Stand nicht mehr).
  Future<List<dynamic>> listEmployeePositions(String employeeId) =>
      _getList('/api/v1/qualifications/positions', {'employee_id': employeeId});

  // Ersetzt die Stellen VOLLSTAENDIG durch positionIds (leer = keine).
  // Liefert die Stellen danach. Nur mit qualifications.write.
  Future<List<dynamic>> setEmployeePositions(String employeeId, List<String> positionIds) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/qualifications/positions', {'employee_id': employeeId}),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'position_ids': positionIds}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // Vorschlagskatalog uebernehmen: liefert die NEU angelegten Stellen, je
  // mit fehlende_arten (Codes, die der Mandant nicht als aktive Art hat).
  Future<List<dynamic>> adoptDefaultPositions() async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/positions/adopt-defaults'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(<String, dynamic>{}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // Betriebsweite Liste (F.13.2, Route F.2.6.2): fuer alle aktiven
  // Mitarbeiter, was fehlt, abgelaufen ist oder innerhalb der Vorwarnzeit
  // ablaeuft - das Dringendste zuerst. Ohne warnDays der Standard des
  // Servers (60 Tage). Nur mit qualifications.read.
  Future<List<dynamic>> listQualificationsDue({int? warnDays}) => _getList(
      '/api/v1/qualifications/due', warnDays == null ? null : {'warn_tage': '$warnDays'});

  // -------- Betriebsmittel-Bestand (Backlog F.13.3, Routen F.3.6.1) --------
  // Bestand nach Inventarnummer; Filter Kategorie, Ausgemusterte auf Wunsch.
  Future<List<dynamic>> listAssets({String? kategorie, bool includeAusgemustert = false}) {
    final qp = <String, String>{};
    if (kategorie != null && kategorie.isNotEmpty) qp['kategorie'] = kategorie;
    if (includeAusgemustert) qp['include_ausgemustert'] = 'true';
    return _getList('/api/v1/assets/', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> createAsset(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/assets/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Ersetzt die Stammdaten VOLLSTAENDIG - der Server verlangt alle zwoelf
  // Felder (sonst machte ein vergessenes Pruefintervall ein Werkzeug still
  // pruefungsfrei).
  Future<Map<String, dynamic>> updateAsset(String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/assets/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Ausmustern: endgueltig, mit Datum (JJJJ-MM-TT) und Grund.
  Future<Map<String, dynamic>> retireAsset(String id, {required String datum, required String grund}) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/assets/$id/retire'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'datum': datum, 'grund': grund}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Fuhrpark: Fahrzeugstamm (Backlog G.1.4, Routen G.1.3) --------
  // Fahrzeuge und Anhaenger nach Kennzeichen; Filter Art, abgemeldete auf
  // Wunsch. Lesen mit fleet.read, pflegen mit fleet.write (nur Fuhrpark und
  // Admin, Entscheidung des Nutzers vom 2026-10-25).
  Future<List<dynamic>> listVehicles({String? art, bool includeAbgemeldete = false}) {
    final qp = <String, String>{};
    if (art != null && art.isNotEmpty) qp['art'] = art;
    if (includeAbgemeldete) qp['include_abgemeldete'] = 'true';
    return _getList('/api/v1/fleet/vehicles/', qp.isEmpty ? null : qp);
  }

  Future<Map<String, dynamic>> createVehicle(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/vehicles/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Ersetzt die Stammdaten VOLLSTAENDIG - der Server verlangt alle zwoelf
  // Felder (sonst schaltete ein vergessenes "fahrtenbuch" es still ab).
  Future<Map<String, dynamic>> updateVehicle(String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/fleet/vehicles/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Abmelden: endgueltig, mit Datum (JJJJ-MM-TT) und Grund.
  Future<Map<String, dynamic>> deregisterVehicle(String id, {required String datum, required String grund}) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/vehicles/$id/deregister'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'datum': datum, 'grund': grund}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Fuhrpark: Termine (Backlog G.2.4, Routen G.2.3) --------
  // Termine eines Fahrzeugs mit Stand (HU, UVV, Wartung, Versicherung,
  // Leasing, Sonstiges); stillgelegte nur auf Wunsch.
  Future<List<dynamic>> listVehicleSchedules(String vehicleId, {bool includeInactive = false}) =>
      _getList('/api/v1/fleet/vehicles/$vehicleId/schedules', includeInactive ? {'include_inactive': 'true'} : null);

  // body: vehicle_id, art, bezeichnung (nur sonstige), starttermin
  // (JJJJ-MM-TT; HU legt der Server aufs Monatsende), start_km,
  // intervall_monate (null = Vorschlag der Art), intervall_km, notiz.
  Future<Map<String, dynamic>> createSchedule(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/schedules/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Ersetzt die Planung VOLLSTAENDIG: starttermin, start_km,
  // intervall_monate, intervall_km, notiz, aktiv sind Pflicht.
  Future<Map<String, dynamic>> updateSchedule(String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/fleet/schedules/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Betriebsweite Faelligkeitsliste der Fahrzeugtermine: gesperrt,
  // ueberfaellig, faellig bald - das Dringendste zuerst. Ohne warnDays der
  // Standard des Servers (30 Tage).
  Future<List<dynamic>> listScheduleDue({int? warnDays}) =>
      _getList('/api/v1/fleet/schedules/due', warnDays == null ? null : {'warn_tage': '$warnDays'});

  // Ein Fahrzeug (fleet.read), z. B. fuer die Detailseite aus der
  // Faelligkeitsliste.
  Future<Map<String, dynamic>> getVehicle(String id) => _getJson('/api/v1/fleet/vehicles/$id');

  // Erledigungen eines Termins, die juengste zuerst; stornierte auf Wunsch.
  Future<List<dynamic>> listCompletions(String scheduleId, {bool includeStorniert = false}) =>
      _getList('/api/v1/fleet/schedules/$scheduleId/completions', includeStorniert ? {'include_storniert': 'true'} : null);

  // body: schedule_id, erledigt_am (JJJJ-MM-TT), ergebnis, km_stand,
  // naechster_termin (null = Vorschlag des Servers aus dem Intervall; nach
  // HU/UVV mit Maengeln oder nicht bestanden keiner), naechster_km,
  // pruefer, notiz.
  Future<Map<String, dynamic>> recordCompletion(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/completions/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Storniert eine Erledigung (einmalig, mit Begruendung).
  Future<Map<String, dynamic>> stornoCompletion(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/completions/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Fahrerzuordnung (Backlog G.3a.3, Routen G.3a.2) --------
  // Zuordnungen, die juengsten zuerst. Mit fleet.read alle, mit fleet.self
  // nur die eigenen.
  Future<List<dynamic>> listFleetAssignments({
    String? vehicleId,
    String? employeeId,
    bool nurOffen = false,
    bool includeStorniert = false,
  }) {
    final qp = <String, String>{};
    if (vehicleId != null && vehicleId.isNotEmpty) qp['vehicle_id'] = vehicleId;
    if (employeeId != null && employeeId.isNotEmpty) qp['employee_id'] = employeeId;
    if (nurOffen) qp['nur_offen'] = 'true';
    if (includeStorniert) qp['include_storniert'] = 'true';
    return _getList('/api/v1/fleet/assignments/', qp.isEmpty ? null : qp);
  }

  // Zuordnen (Beginn JJJJ-MM-TT). Die Antwort enthaelt ggf. "hinweise"
  // (Termin ueberfaellig, keine gueltige Fuehrerscheinkontrolle); bei HU/UVV
  // mit Maengeln oder nicht bestanden lehnt der Server mit 409 ab. Erwartet 201.
  Future<Map<String, dynamic>> assignVehicle({
    required String vehicleId,
    required String employeeId,
    required String beginn,
    String notiz = '',
  }) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/assignments/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'vehicle_id': vehicleId, 'employee_id': employeeId, 'beginn': beginn, 'notiz': notiz}),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Zuordnung beenden: einmalig, Datum JJJJ-MM-TT.
  Future<Map<String, dynamic>> endFleetAssignment(String id, String ende) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/assignments/$id/end'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'ende': ende}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Zuordnung stornieren (falsch erfasst), mit Begruendung.
  Future<Map<String, dynamic>> stornoFleetAssignment(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/assignments/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Fahrtenbuch (Backlog G.3c.4, Routen G.3c.3) --------
  // Fahrten, die juengsten zuerst. Mit fleet.read alle, mit fleet.self nur
  // die eigenen. von/bis als JJJJ-MM-TT.
  Future<List<dynamic>> listFleetTrips({
    String? vehicleId,
    String? employeeId,
    String? von,
    String? bis,
    bool includeStorniert = false,
  }) {
    final qp = <String, String>{};
    if (vehicleId != null && vehicleId.isNotEmpty) qp['vehicle_id'] = vehicleId;
    if (employeeId != null && employeeId.isNotEmpty) qp['employee_id'] = employeeId;
    if (von != null && von.isNotEmpty) qp['von'] = von;
    if (bis != null && bis.isNotEmpty) qp['bis'] = bis;
    if (includeStorniert) qp['include_storniert'] = 'true';
    return _getList('/api/v1/fleet/trips/', qp.isEmpty ? null : qp);
  }

  // Nur die eigenen Fahrten - auch mit fleet.read (nur_eigene, G.3c.4b).
  Future<List<dynamic>> listOwnFleetTrips({String? vehicleId, bool includeStorniert = false}) {
    final qp = <String, String>{'nur_eigene': 'true'};
    if (vehicleId != null && vehicleId.isNotEmpty) qp['vehicle_id'] = vehicleId;
    if (includeStorniert) qp['include_storniert'] = 'true';
    return _getList('/api/v1/fleet/trips/', qp);
  }

  // Nur die eigenen Zuordnungen - auch mit fleet.read (nur_eigene); jede
  // enthaelt "fahrtenbuch" des Fahrzeugs.
  Future<List<dynamic>> listOwnFleetAssignments({bool nurOffen = false}) =>
      _getList('/api/v1/fleet/assignments/', {'nur_eigene': 'true', if (nurOffen) 'nur_offen': 'true'});

  // km-Stand ("km_stand", null ohne Fahrt) und Luecken ("luecken":
  // [{von_km, bis_km}]) eines Fahrzeugs.
  Future<Map<String, dynamic>> getFleetTripChain(String vehicleId) =>
      _getJson('/api/v1/fleet/trips/chain', {'vehicle_id': vehicleId});

  // Fahrt erfassen. body wie POST /fleet/trips; ohne employee_id gilt sie
  // dem eigenen Mitarbeiter. Erwartet 201.
  Future<Map<String, dynamic>> recordFleetTrip(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/trips/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Fahrt stornieren (falsch erfasst), mit Begruendung; nur fleet.write.
  Future<Map<String, dynamic>> stornoFleetTrip(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/trips/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Fahrzeugkosten (Backlog G.3d.3, Routen G.3d.2) --------
  // Belege, die juengsten zuerst (fleet.read). von/bis als JJJJ-MM-TT.
  Map<String, String> _fleetCostQuery(String? vehicleId, String? kostenart, String? von, String? bis, bool includeStorniert) => {
        if (vehicleId != null && vehicleId.isNotEmpty) 'vehicle_id': vehicleId,
        if (kostenart != null && kostenart.isNotEmpty) 'kostenart': kostenart,
        if (von != null && von.isNotEmpty) 'von': von,
        if (bis != null && bis.isNotEmpty) 'bis': bis,
        if (includeStorniert) 'include_storniert': 'true',
      };

  Future<List<dynamic>> listFleetCosts({String? vehicleId, String? kostenart, String? von, String? bis, bool includeStorniert = false}) {
    final qp = _fleetCostQuery(vehicleId, kostenart, von, bis, includeStorniert);
    return _getList('/api/v1/fleet/costs/', qp.isEmpty ? null : qp);
  }

  // Summen je Kostenart ("summen": [{kostenart, anzahl, betrag_netto}])
  // und "gesamt"; stornierte zaehlen nie.
  Future<Map<String, dynamic>> getFleetCostSummary({String? vehicleId, String? kostenart, String? von, String? bis}) {
    final qp = _fleetCostQuery(vehicleId, kostenart, von, bis, false);
    return _getJson('/api/v1/fleet/costs/summary', qp.isEmpty ? null : qp);
  }

  // Beleg erfassen (fleet.write); body wie POST /fleet/costs. Erwartet 201.
  Future<Map<String, dynamic>> recordFleetCost(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/costs/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Beleg stornieren (falsch erfasst), mit Begruendung; fleet.write.
  Future<Map<String, dynamic>> stornoFleetCost(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/fleet/costs/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Kostenstellen (cost_centers.read); ohne includeInactive nur aktive.
  Future<List<dynamic>> listCostCenters({bool includeInactive = false}) =>
      _getList('/api/v1/cost-centers/', includeInactive ? {'include_inactive': 'true'} : null);

  // -------- Fertigungsauftraege (Backlog H.1.4, Routen H.1.3) --------
  // Liste nach Liefertermin (production.read). Stornierte nur mit
  // includeStorniert oder status=storniert. Jeder Auftrag traegt
  // "hinweise" (kein_kundenauftrag, menge_ueberschritten,
  // position_entfernt, position_geaendert).
  Future<List<dynamic>> listProductionOrders({String? projectId, String? elevationId, String? status, bool includeStorniert = false}) {
    final qp = <String, String>{
      if (projectId != null) 'project_id': projectId,
      if (elevationId != null) 'elevation_id': elevationId,
      if (status != null && status.isNotEmpty) 'status': status,
      if (includeStorniert) 'include_storniert': 'true',
    };
    return _getList('/api/v1/production/orders/', qp.isEmpty ? null : qp);
  }

  // Ein Auftrag, ab der Freigabe mit "items" (Fertigungsstueckliste).
  Future<Map<String, dynamic>> getProductionOrder(String id) => _getJson('/api/v1/production/orders/$id');

  // Entwurf anlegen (production.write): project_id, elevation_id,
  // single_elevation_id, stueckzahl, optional liefertermin (JJJJ-MM-TT),
  // ausfuehrungsklasse, notiz. Erwartet 201.
  Future<Map<String, dynamic>> createProductionOrder(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/production/orders/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Entwurf aendern: stueckzahl, liefertermin, ausfuehrungsklasse, notiz -
  // immer alle, leer heisst "keine Angabe".
  Future<Map<String, dynamic>> updateProductionOrderDraft(String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/production/orders/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Statusschritt ohne Eingabe: aktion = freigeben, beginnen oder
  // fertigmelden.
  Future<Map<String, dynamic>> productionOrderStep(String id, String aktion) async {
    final r = await _sendWithAuth(
      (headers) => http.post(_u('/api/v1/production/orders/$id/$aktion'), headers: headers),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Freigeben; mit lagerId wird aus diesem Lager reserviert (H.1.5),
  // ohne wie productionOrderStep(id, 'freigeben').
  Future<Map<String, dynamic>> releaseProductionOrder(String id, {String? lagerId}) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/production/orders/$id/freigeben'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({if (lagerId != null && lagerId.isNotEmpty) 'lager_id': lagerId}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Storno mit Begruendung, aus jedem Status vor "fertig".
  Future<Map<String, dynamic>> stornoProductionOrder(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/production/orders/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Grobplanung: Arbeitsplaetze und Vorlagen (Backlog H.2.5a, Routen H.2.2) --------
  // Lesen mit production.read, pflegen mit production.write. Deaktivieren
  // statt loeschen: PUT mit "aktiv": false.
  Future<List<dynamic>> listWorkCenters({bool includeInactive = false}) =>
      _getList('/api/v1/production/work-centers/', includeInactive ? {'include_inactive': 'true'} : null);

  // name, stunden_je_tag. Erwartet 201.
  Future<Map<String, dynamic>> createWorkCenter(Map<String, dynamic> body) => _sendProductionJson('POST', '/api/v1/production/work-centers/', body, 201);

  // name, stunden_je_tag, optional aktiv (weggelassen = unveraendert).
  Future<Map<String, dynamic>> updateWorkCenter(String id, Map<String, dynamic> body) =>
      _sendProductionJson('PUT', '/api/v1/production/work-centers/$id', body, 200);

  // In der Reihenfolge des Vorschlags.
  Future<List<dynamic>> listOperationTemplates({bool includeInactive = false}) =>
      _getList('/api/v1/production/operation-templates/', includeInactive ? {'include_inactive': 'true'} : null);

  // name, work_center_id, reihenfolge, ruestzeit_min, minuten_je_stueck.
  // Erwartet 201.
  Future<Map<String, dynamic>> createOperationTemplate(Map<String, dynamic> body) =>
      _sendProductionJson('POST', '/api/v1/production/operation-templates/', body, 201);

  // Alle Angaben, optional aktiv (weggelassen = unveraendert).
  Future<Map<String, dynamic>> updateOperationTemplate(String id, Map<String, dynamic> body) =>
      _sendProductionJson('PUT', '/api/v1/production/operation-templates/$id', body, 200);

  // -------- Arbeitsgaenge am Fertigungsauftrag (Backlog H.2.5b, Routen H.2.3b) --------
  // Je Arbeitsgang plan_minuten, geplante_woche (Montag) und "hinweise"
  // (nach_liefertermin). Aenderbar bis fertig/storniert (sonst 409).
  Future<List<dynamic>> listProductionOperations(String orderId) => _getList('/api/v1/production/orders/$orderId/operations', null);

  // Vorschlag aus den aktiven Vorlagen, nur ohne vorhandene Arbeitsgaenge.
  // Erwartet 201 mit der Liste.
  Future<List<dynamic>> suggestProductionOperations(String orderId) async {
    final r = await _sendWithAuth(
      (headers) => http.post(_u('/api/v1/production/orders/$orderId/operations/vorschlagen'), headers: headers),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as List<dynamic>;
  }

  // name, work_center_id, ruestzeit_min, minuten_je_stueck, geplante_woche
  // (JJJJ-MM-TT, leer = ungeplant). Erwartet 201.
  Future<Map<String, dynamic>> addProductionOperation(String orderId, Map<String, dynamic> body) =>
      _sendProductionJson('POST', '/api/v1/production/orders/$orderId/operations', body, 201);

  // Alle Angaben auf einmal.
  Future<Map<String, dynamic>> updateProductionOperation(String orderId, String opId, Map<String, dynamic> body) =>
      _sendProductionJson('PUT', '/api/v1/production/orders/$orderId/operations/$opId', body, 200);

  // Erwartet 204.
  Future<void> deleteProductionOperation(String orderId, String opId) async {
    final r = await _sendWithAuth(
      (headers) => http.delete(_u('/api/v1/production/orders/$orderId/operations/$opId'), headers: headers),
    );
    if (r.statusCode != 204) _throwApiException(r);
  }

  // Rueckmeldung je Arbeitsgang (H.3.5d): erledigt melden mit optional
  // ist_minuten und notiz, zuruecknehmen nur mit Begruendung; beides nur bei
  // begonnenen Auftraegen (sonst 409). Antwort ist der Arbeitsgang.
  Future<Map<String, dynamic>> reportProductionOperationDone(String orderId, String opId, Map<String, dynamic> body) =>
      _sendProductionJson('POST', '/api/v1/production/orders/$orderId/operations/$opId/erledigt', body, 200);

  Future<Map<String, dynamic>> revertProductionOperationDone(String orderId, String opId, String reason) =>
      _sendProductionJson('POST', '/api/v1/production/orders/$orderId/operations/$opId/zuruecknehmen', {'reason': reason}, 200);

  // Soll/Ist je Auftrag (H.3.5c): arbeitsgaenge, erledigt, soll_min,
  // soll_erledigt_min, mit_ist, soll_mit_ist_min, ist_min.
  Future<Map<String, dynamic>> getProductionSollIst(String orderId) => _getJson('/api/v1/production/orders/$orderId/soll-ist', null);

  // Auslastung je Arbeitsplatz und Woche (H.2.4); von/bis JJJJ-MM-TT, je ein
  // beliebiger Tag der Woche, hoechstens 26 Wochen. Ohne Angabe acht Wochen
  // ab der aktuellen.
  Future<Map<String, dynamic>> getProductionCapacity({String? von, String? bis}) => _getJson('/api/v1/production/capacity', {
        if (von != null) 'von': von,
        if (bis != null) 'bis': bis,
      });

  // Entnahmen je Fertigungsauftrag (H.3.2c): Liste neueste zuerst mit
  // Stornos (production.read). Entnehmen (production.write): item_id oder
  // material_id, warehouse_id, optional location_id, menge, notiz; Antwort
  // mit "hinweise" (z. B. mehrentnahme). Storno nur mit Begruendung.
  Future<List<dynamic>> listProductionWithdrawals(String orderId) => _getList('/api/v1/production/orders/$orderId/withdrawals', null);

  Future<Map<String, dynamic>> createProductionWithdrawal(String orderId, Map<String, dynamic> body) =>
      _sendProductionJson('POST', '/api/v1/production/orders/$orderId/withdrawals', body, 201);

  Future<Map<String, dynamic>> stornoProductionWithdrawal(String orderId, String withdrawalId, String reason) =>
      _sendProductionJson('POST', '/api/v1/production/orders/$orderId/withdrawals/$withdrawalId/storno', {'reason': reason}, 200);

  // Montagetermine je Fertigungsauftrag (H.3.6d): Liste nach Beginn mit
  // Stornos, je Termin von/bis, team_name, kennzeichen, baustelle, notiz und
  // "hinweise" (production.read). Anlegen und aendern (production.write):
  // von, bis (JJJJ-MM-TT), team_id, vehicle_id, baustelle, notiz; nicht bei
  // stornierten Auftraegen (409). Storno nur mit Begruendung.
  Future<List<dynamic>> listProductionAssembly(String orderId) => _getList('/api/v1/production/orders/$orderId/montage', null);

  Future<Map<String, dynamic>> createProductionAssembly(String orderId, Map<String, dynamic> body) =>
      _sendProductionJson('POST', '/api/v1/production/orders/$orderId/montage', body, 201);

  Future<Map<String, dynamic>> updateProductionAssembly(String orderId, String id, Map<String, dynamic> body) =>
      _sendProductionJson('PUT', '/api/v1/production/orders/$orderId/montage/$id', body, 200);

  Future<Map<String, dynamic>> cancelProductionAssembly(String orderId, String id, String reason) =>
      _sendProductionJson('POST', '/api/v1/production/orders/$orderId/montage/$id/stornieren', {'reason': reason}, 200);

  // Auswahl ohne Personendaten: teams {id, name}, fahrzeuge {id,
  // kennzeichen, bezeichnung, abgemeldet_am}.
  Future<Map<String, dynamic>> getProductionAssemblyChoices() => _getJson('/api/v1/production/montage/auswahl', null);

  // Wochenuebersicht (H.3.6f, production.read): {von, bis, teams: [{team_id,
  // team_name, termine}]}, alle Teams nach Name, "ohne Team" zuletzt; nur
  // nicht stornierte Termine. Ohne Angabe ab der aktuellen Woche acht Wochen.
  Future<Map<String, dynamic>> getProductionAssemblyWeeks({String? von, String? bis}) => _getJson('/api/v1/production/montage', {
        if (von != null) 'von': von,
        if (bis != null) 'bis': bis,
      });

  // Kommissionierliste als PDF (H.3.3b, production.read), nur bei
  // freigegebenen und begonnenen Auftraegen.
  Future<void> downloadProductionPickListPdf(String orderId, {String? filename}) async {
    final r = await _getBytesResponse('/api/v1/production/orders/$orderId/kommissionierliste.pdf');
    browser.downloadBytes(
      r.bodyBytes,
      filename: filename ?? 'Kommissionierliste_$orderId.pdf',
      contentType: r.headers['content-type'] ?? 'application/pdf',
    );
  }

  Future<Map<String, dynamic>> _sendProductionJson(String method, String path, Map<String, dynamic> body, int expected) async {
    final r = await _sendWithAuth(
      (headers) {
        final h = {...headers, 'Content-Type': 'application/json'};
        return method == 'PUT'
            ? http.put(_u(path), headers: h, body: jsonEncode(body))
            : http.post(_u(path), headers: h, body: jsonEncode(body));
      },
    );
    if (r.statusCode != expected) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Ausgabe von Betriebsmitteln (Backlog F.13.4.1, Routen F.3.6.2) --------
  // Ausgeben an einen Mitarbeiter (Datum JJJJ-MM-TT). Die Antwort enthaelt
  // ggf. "hinweise" (Pruefung ueberfaellig, nie geprueft); ein gesperrtes
  // Stueck lehnt der Server mit 409 ab. Erwartet 201.
  Future<Map<String, dynamic>> issueAsset({
    required String assetId,
    required String employeeId,
    required String ausgegebenAm,
    String notiz = '',
  }) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/asset-assignments/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'asset_id': assetId, 'employee_id': employeeId, 'ausgegeben_am': ausgegebenAm, 'notiz': notiz}),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Zuruecknehmen: einmalig, Datum JJJJ-MM-TT, Zustand optional.
  Future<Map<String, dynamic>> returnAsset(String assignmentId, {required String datum, String zustand = ''}) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/asset-assignments/$assignmentId/return'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'datum': datum, 'zustand': zustand}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Ausgabe stornieren (falsch erfasst), mit Begruendung.
  Future<Map<String, dynamic>> stornoAssetAssignment(String assignmentId, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/asset-assignments/$assignmentId/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Pruefungen von Betriebsmitteln (Backlog F.13.4.2, Routen F.3.6.3) --------
  // Pruefstand eines Stuecks heute; Vorwarnzeit Standard 30 Tage (Server).
  Future<Map<String, dynamic>> getAssetInspectionStanding(String assetId) =>
      _getJson('/api/v1/assets/$assetId/inspection-standing');

  // Pruefungen eines Stuecks, die juengste zuerst; stornierte auf Wunsch.
  Future<List<dynamic>> listAssetInspections(String assetId, {bool includeStorniert = false}) {
    final qp = <String, String>{'asset_id': assetId};
    if (includeStorniert) qp['include_storniert'] = 'true';
    return _getList('/api/v1/asset-inspections/', qp);
  }

  // Pruefung erfassen (body: asset_id, geprueft_am, ergebnis,
  // naechste_pruefung_am optional, pruefer_name, notiz). Erwartet 201.
  Future<Map<String, dynamic>> recordAssetInspection(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/asset-inspections/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> stornoAssetInspection(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/asset-inspections/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Betriebsweite Liste: gesperrt, ueberfaellig, nie geprueft, bald faellig.
  Future<List<dynamic>> listAssetInspectionsDue({int? warnDays}) => _getList(
      '/api/v1/asset-inspections/due', warnDays == null ? null : {'warn_tage': '$warnDays'});

  // Kontoauszuege (Server-Routen /bank-statements, bank.read/bank.write).
  // Fehlten bisher, obwohl bank_statements_page.dart sie aufruft (Backlog
  // 0.48). Der Server wertet derzeit nur limit aus, offset wird ignoriert.
  Future<List<dynamic>> listBankStatements({int? limit, int? offset}) {
    final qp = <String, String>{};
    if (limit != null) qp['limit'] = '$limit';
    if (offset != null) qp['offset'] = '$offset';
    return _getList('/api/v1/bank-statements', qp.isEmpty ? null : qp);
  }

  // Ordnet einen Kontoauszug zu; ohne invoiceId sucht der Server selbst.
  Future<Map<String, dynamic>> matchBankStatement(String id, {String? invoiceId}) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/bank-statements/$id/match'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({if (invoiceId != null) 'invoice_id': invoiceId}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // -------- Urlaub und Abwesenheiten (Backlog F.8.6, Routen F.8.2-F.8.4) --------
  // Antraege: ohne employeeId die eigenen (leave_requests.self); mit
  // leave_requests.read und ohne employeeId alle des Mandanten.
  Future<List<dynamic>> listLeaveRequests({String? employeeId, String? status, int? limit}) {
    final qp = <String, String>{};
    if (employeeId != null && employeeId.isNotEmpty) qp['employee_id'] = employeeId;
    if (status != null && status.isNotEmpty) qp['status'] = status;
    if (limit != null) qp['limit'] = '$limit';
    return _getList('/api/v1/leave-requests/', qp.isEmpty ? null : qp);
  }

  // Antrag stellen: start_date, end_date (JJJJ-MM-TT), typ, optional days
  // (sonst zaehlt der Server die Arbeitstage), reason. Erwartet 201.
  Future<Map<String, dynamic>> createLeaveRequest(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/leave-requests/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Offenen Antrag zurueckziehen.
  Future<Map<String, dynamic>> withdrawLeaveRequest(String id) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/leave-requests/$id/withdraw'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(<String, dynamic>{}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Urlaubskonto eines Jahres; ohne employeeId das eigene.
  Future<Map<String, dynamic>> getLeaveAccount({String? employeeId, int? year}) {
    final qp = <String, String>{};
    if (employeeId != null && employeeId.isNotEmpty) qp['employee_id'] = employeeId;
    if (year != null) qp['year'] = '$year';
    return _getJson('/api/v1/leave-accounts/', qp.isEmpty ? null : qp);
  }

  // Krankmeldung: start_date, end_date; ohne employee_id fuer sich selbst
  // (dann ohne Attest). Erwartet 201.
  Future<Map<String, dynamic>> reportSick(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/absences/sick'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Abwesenheiten im Zeitraum (JJJJ-MM-TT); ohne leave_requests.read nur
  // die eigenen.
  Future<List<dynamic>> listAbsences({
    required String from,
    required String to,
    String? employeeId,
    String? teamId,
    bool includeStorniert = false,
  }) {
    final qp = <String, String>{'from': from, 'to': to};
    if (employeeId != null && employeeId.isNotEmpty) qp['employee_id'] = employeeId;
    if (teamId != null && teamId.isNotEmpty) qp['team_id'] = teamId;
    if (includeStorniert) qp['include_storniert'] = 'true';
    return _getList('/api/v1/absences/', qp);
  }

  // Antrag genehmigen (approve true) oder ablehnen - leave_requests.decide.
  Future<Map<String, dynamic>> decideLeaveRequest(String id, bool approve) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/leave-requests/$id/decide'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'approve': approve}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Genehmigten Antrag mit Begruendung stornieren; die Tage gehen aufs Konto zurueck.
  Future<Map<String, dynamic>> stornoLeaveRequest(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/leave-requests/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Selbst gemeldete, noch nicht bestaetigte Krankmeldungen (leave_requests.read).
  Future<List<dynamic>> listNewSickReports() => _getList('/api/v1/absences/sick/new');

  // Attest ja/nein an einer Krankmeldung festhalten.
  Future<Map<String, dynamic>> setAbsenceAttest(String id, bool attest) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/absences/$id/attest'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'attest': attest}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Selbstmeldung bestaetigen - sie verschwindet aus "neue Krankmeldungen".
  Future<Map<String, dynamic>> confirmAbsence(String id) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/absences/$id/confirm'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(<String, dynamic>{}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Krankmeldung mit Begruendung stornieren.
  Future<Map<String, dynamic>> stornoAbsence(String id, String reason) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/absences/$id/storno'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'reason': reason}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Liste "Resturlaub" eines Jahres (leave_requests.read).
  Future<List<dynamic>> listRemainingLeave({int? year}) =>
      _getList('/api/v1/leave-accounts/remaining', year == null ? null : {'year': '$year'});

  // Urlaubskonto eines Jahres festlegen (leave_requests.decide):
  // anspruch_tage (Pflicht), uebertrag_tage, hinweis_erteilt_am
  // (JJJJ-MM-TT), notiz. Ersetzt das Konto vollstaendig.
  Future<Map<String, dynamic>> setLeaveAccount(String employeeId, int year, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/leave-accounts/$employeeId/$year'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Freie Tage eines Jahres fuer den Mandanten: gesetzliche (nach
  // eingestelltem Bundesland) und betriebliche (F.8.2.2). Jeder angemeldete
  // Benutzer.
  Future<Map<String, dynamic>> getHolidays({int? year}) =>
      _getJson('/api/v1/holidays/', year == null ? null : {'year': '$year'});

  // Feiertagskalender des Mandanten (F.8.2.2): Bundesland ("" = nicht
  // eingestellt) und bayerische Schalter - settings.manage.
  Future<Map<String, dynamic>> getHrCalendarSettings() => _getJson('/api/v1/settings/hr-calendar/');

  Future<Map<String, dynamic>> setHrCalendarSettings(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/settings/hr-calendar/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Mail-Zugang des Mandanten (F.14.2c) - settings.manage. Das Passwort
  // wird nur gesendet, nie geliefert ("passwort_gesetzt", "passwort_lesbar").
  Future<Map<String, dynamic>> getMailSettings() => _getJson('/api/v1/settings/mail/');

  Future<Map<String, dynamic>> saveMailSettings(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/settings/mail/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Testversand mit dem gespeicherten Zugang: {"gesendet", "umgeleitet"}.
  Future<Map<String, dynamic>> sendMailTest(String an) async =>
      (await _postJson('/api/v1/settings/mail/test', {'an': an})) as Map<String, dynamic>;

  // Sprachmodell des Mandanten (I.3.5a2) - settings.manage. Der API-Schluessel
  // wird nur gesendet, nie geliefert ("api_key_gesetzt", "api_key_lesbar").
  Future<Map<String, dynamic>> getLlmSettings() => _getJson('/api/v1/settings/llm/');

  Future<Map<String, dynamic>> saveLlmSettings(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/settings/llm/'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Verbindungstest mit den gespeicherten Einstellungen:
  // {"ok", "modell", "dauer_ms", "eingabe_tokens", "ausgabe_tokens"}.
  Future<Map<String, dynamic>> testLlmSettings() async =>
      (await _postJson('/api/v1/settings/llm/test', const {})) as Map<String, dynamic>;

  // -------- Hinweis-Mails (Backlog F.14.4c2, Routen F.14.4c1) --------
  // Eigene Abbestellung - jeder angemeldete Benutzer fuer sich.
  Future<bool> getNoticeMailOptOut() async =>
      (await _getJson('/api/v1/notice-mails/me'))['abbestellt'] == true;

  Future<bool> setNoticeMailOptOut(bool abbestellt) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/notice-mails/me'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'abbestellt': abbestellt}),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return (jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>)['abbestellt'] == true;
  }

  // Versandprotokoll (settings.manage), juengste zuerst; ohne Inhalt.
  Future<List<dynamic>> listNoticeMailLog({int? limit}) =>
      _getList('/api/v1/notice-mails/log', limit == null ? null : {'limit': '$limit'});

  // Betriebliche freie Tage (Betriebsruhe) - leave_requests.decide.
  Future<List<dynamic>> listCompanyDays({int? year}) =>
      _getList('/api/v1/holidays/company-days', year == null ? null : {'year': '$year'});

  Future<Map<String, dynamic>> createCompanyDay(String datum, String name) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/holidays/company-days'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode({'datum': datum, 'name': name}),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<void> deleteCompanyDay(String id) async {
    final r = await _sendWithAuth((headers) => http.delete(_u('/api/v1/holidays/company-days/$id'), headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
  }

  // Teams pflegen (F.8.5) - employees.write. Body: name, lead_employee_id
  // (null = kein Teamleiter).
  Future<Map<String, dynamic>> createTeam(Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.post(
        _u('/api/v1/employees/teams'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 201) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> updateTeam(String id, Map<String, dynamic> body) async {
    final r = await _sendWithAuth(
      (headers) => http.put(
        _u('/api/v1/employees/teams/$id'),
        headers: {...headers, 'Content-Type': 'application/json'},
        body: jsonEncode(body),
      ),
    );
    if (r.statusCode != 200) _throwApiException(r);
    return jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
  }

  // Nur ohne Mitglieder - sonst 409.
  Future<void> deleteTeam(String id) async {
    final r = await _sendWithAuth((headers) => http.delete(_u('/api/v1/employees/teams/$id'), headers: headers));
    if (r.statusCode != 204) _throwApiException(r);
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
