import 'dart:io';

import 'models.dart';

String? officialHongguoDramaUrl(Drama drama) {
  if (drama.source != 'hongguo') return null;
  final sourceId = drama.sourceId;
  if (!RegExp(r'^[0-9]{1,32}$').hasMatch(sourceId)) return null;
  return 'https://hongguoduanju.com/detail?series_id=$sourceId';
}

String? publicHongguoEpisodeUrl(Drama drama, Episode episode) {
  if (drama.source != 'hongguo') return null;
  final seriesId = drama.sourceId;
  final videoUrl = episode.raw['videoUrl'];
  if (videoUrl is! String || !videoUrl.startsWith('hongguo-cenc://')) {
    return null;
  }
  final videoId = videoUrl.substring('hongguo-cenc://'.length);
  final idPattern = RegExp(r'^[0-9]{1,32}$');
  if (!idPattern.hasMatch(seriesId) || !idPattern.hasMatch(videoId)) {
    return null;
  }
  return 'https://novelquickapp.com/player/$seriesId/$videoId';
}

Future<bool> openPublicHongguoEpisodeInBrowser(
  Drama drama,
  Episode episode,
) async {
  if (!Platform.isWindows) return false;
  final url = publicHongguoEpisodeUrl(drama, episode);
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
