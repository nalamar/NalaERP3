import 'dart:typed_data';
import 'browser_stub.dart' if (dart.library.html) 'browser_web.dart' as impl;

class PickedFile {
  final Uint8List bytes;
  final String filename;
  final String contentType;
  PickedFile(this.bytes, this.filename, this.contentType);
}

Future<PickedFile?> pickFile({String? accept}) async {
  final r = await impl.pickFileRaw(accept: accept);
  if (r == null) return null;
  return PickedFile(r.bytes, r.filename, r.contentType);
}

void downloadUrl(String url, {String? filename}) => impl.downloadUrl(url, filename: filename);

void downloadBytes(Uint8List bytes, {required String filename, String contentType = 'application/octet-stream'}) =>
    impl.downloadBytes(bytes, filename: filename, contentType: contentType);

String? readStorage(String key) => impl.readStorage(key);

void writeStorage(String key, String value) => impl.writeStorage(key, value);

void removeStorage(String key) => impl.removeStorage(key);

