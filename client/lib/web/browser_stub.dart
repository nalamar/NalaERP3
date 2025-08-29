import 'dart:typed_data';

class _PickedFileRaw {
  final Uint8List bytes;
  final String filename;
  final String contentType;
  _PickedFileRaw(this.bytes, this.filename, this.contentType);
}

Future<_PickedFileRaw?> pickFileRaw({String? accept}) async {
  return null; // not supported outside web
}

void downloadUrl(String url, {String? filename}) {
  // no-op in non-web
}

