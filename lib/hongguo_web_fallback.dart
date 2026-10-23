import 'dart:io';

import 'models.dart';

String? trustedWindowsExplorerPath(String? systemRoot) {
  if (systemRoot == null ||
      !RegExp(r'^[A-Za-z]:\\Windows$', caseSensitive: false).hasMatch(systemRoot)) {
    return null;
  }
  return '$systemRoot\\explorer.exe';
}

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


bool supportsHongguoInAppWebPlayback({
  required bool windows,
  required DramaDetail detail,
  required int episodeIndex,
  bool localOnly = false,
}) {
  return windows &&
      !localOnly &&
      episodeIndex >= 0 &&
      episodeIndex < detail.episodes.length &&
      publicHongguoEpisodeUrl(
        detail.drama,
        detail.episodes[episodeIndex],
      ) != null;
}

bool isTrustedHongguoWebviewNavigation(String address) {
  final uri = Uri.tryParse(address);
  if (uri == null || uri.scheme != 'https' ||
      uri.userInfo.isNotEmpty || uri.hasPort) return false;
  return const {
    'novelquickapp.com',
    'applink.novelquickapp.com',
    'hongguoduanju.com',
    'www.hongguoduanju.com',
  }.contains(uri.host.toLowerCase());
}

Future<bool> openPublicHongguoEpisodeInBrowser(
  Drama drama,
  Episode episode,
) async {
  if (!Platform.isWindows) return false;
  final url = publicHongguoEpisodeUrl(drama, episode);
  if (url == null) return false;
  final explorer = trustedWindowsExplorerPath(Platform.environment['SystemRoot']);
  if (explorer == null) return false;
  try {
    await Process.start(
      explorer,
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
  final explorer = trustedWindowsExplorerPath(Platform.environment['SystemRoot']);
  if (explorer == null) return false;
  try {
    await Process.start(
      explorer,
      [url],
      mode: ProcessStartMode.detached,
    );
    return true;
  } catch (_) {
    return false;
  }
}
