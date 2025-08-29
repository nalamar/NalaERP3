import 'dart:html' as html;
import 'dart:typed_data';

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
  reader.readAsArrayBuffer(file);
  await reader.onLoad.first;
  final data = reader.result as ByteBuffer;
  final bytes = Uint8List.view(data);
  final ct = file.type.isNotEmpty ? file.type : 'application/octet-stream';
  return _PickedFileRaw(bytes, file.name, ct);
}

void downloadUrl(String url, {String? filename}) {
  final a = html.AnchorElement(href: url);
  if (filename != null && filename.isNotEmpty) a.download = filename;
  a.click();
}

