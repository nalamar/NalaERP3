// ignore_for_file: deprecated_member_use

import 'dart:html' as html;
import 'dart:typed_data';
import 'dart:convert';

class _PickedFileRaw {
  final Uint8List bytes;
  final String filename;
  final String contentType;
  _PickedFileRaw(this.bytes, this.filename, this.contentType);
}

Future<_PickedFileRaw?> pickFileRaw({String? accept}) async {
  final input = html.FileUploadInputElement();
  if (accept != null && accept.isNotEmpty) input.accept = accept;
  input.click();
  await input.onChange.first;
  if (input.files == null || input.files!.isEmpty) return null;
  final file = input.files!.first;
  final reader = html.FileReader();
  try {
    reader.readAsArrayBuffer(file);
    await reader.onLoad.first;
  } catch (_) {}
  Uint8List bytes;
  final res = reader.result;
  if (res is ByteBuffer) {
    bytes = Uint8List.fromList(res.asUint8List());
  } else if (res is Uint8List) {
    bytes = Uint8List.fromList(res);
  } else if (res is List<int>) {
    bytes = Uint8List.fromList(res);
  } else if (res is String && res.startsWith('data:')) {
    // Fallback: base64 data URL
    final comma = res.indexOf(',');
    final b64 = comma >= 0 ? res.substring(comma + 1) : res;
    bytes = base64.decode(b64);
  } else {
    // Retry via data URL
    final reader2 = html.FileReader();
    reader2.readAsDataUrl(file);
    await reader2.onLoad.first;
    final s = reader2.result as String;
    final comma = s.indexOf(',');
    final b64 = comma >= 0 ? s.substring(comma + 1) : s;
    bytes = base64.decode(b64);
  }
  final ct = file.type.isNotEmpty ? file.type : 'application/octet-stream';
  return _PickedFileRaw(bytes, file.name, ct);
}

void downloadUrl(String url, {String? filename}) {
  final a = html.AnchorElement(href: url);
  if (filename != null && filename.isNotEmpty) a.download = filename;
  a.click();
}

void downloadBytes(Uint8List bytes, {required String filename, String contentType = 'application/octet-stream'}) {
  final blob = html.Blob([bytes], contentType);
  final url = html.Url.createObjectUrlFromBlob(blob);
  final a = html.AnchorElement(href: url)..download = filename;
  a.click();
  html.Url.revokeObjectUrl(url);
}

String? readStorage(String key) => html.window.localStorage[key];

void writeStorage(String key, String value) {
  html.window.localStorage[key] = value;
}

void removeStorage(String key) {
  html.window.localStorage.remove(key);
}
