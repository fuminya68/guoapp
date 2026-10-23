import 'dart:io';

import 'models.dart';

String? officialHongguoDramaUrl(Drama drama) {
  if (drama.source != 'hongguo') return null;
  final sourceId = drama.sourceId;
  if (!RegExp(r'^[0-9]{1,32}$').hasMatch(sourceId)) return null;
  return 'https://hongguoduanju.com/detail?series_id=$sourceId';
}

Future<bool> openOfficialHongguoDramaInBrowser(Drama drama) async {
  if (!Platform.isWindows) return false;
  final url = officialHongguoDramaUrl(drama);
  if (url == null) return false;
  try {
    await Process.start(
      'explorer.exe',
      [url],
      mode: ProcessStartMode.detached,
    );
    return true;
  } catch (_) {
    return false;
  }
}
