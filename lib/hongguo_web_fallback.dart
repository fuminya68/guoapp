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


int? hongguoEpisodeIndexFromWebviewUrl(DramaDetail detail, String address) {
  if (!isTrustedHongguoWebviewNavigation(address)) return null;
  final uri = Uri.tryParse(address);
  if (uri == null ||
      !const {'novelquickapp.com', 'hongguoduanju.com'}
          .contains(uri.host.toLowerCase()) ||
      uri.pathSegments.length != 3 ||
      uri.pathSegments[0] != 'player' ||
      uri.pathSegments[1] != detail.drama.sourceId) {
    return null;
  }
  final vid = uri.pathSegments[2];
  if (!RegExp(r'^[0-9]{1,32}$').hasMatch(vid)) return null;
  for (var i = 0; i < detail.episodes.length; i++) {
    final expected = publicHongguoEpisodeUrl(
      detail.drama,
      detail.episodes[i],
    );
    if (expected != null &&
        Uri.parse(expected).pathSegments[2] == vid) {
      return i;
    }
  }
  return null;
}

/// Rewrite only same-series episode popups. Native popup policy remains deny.
/// Does not read cookies, playback keys or DOM media information.
String? hongguoSameSeriesNavigationScript(Drama drama) {
  if (drama.source != 'hongguo' ||
      !RegExp(r'^[0-9]{1,32}$').hasMatch(drama.sourceId)) {
    return null;
  }
  final seriesId = drama.sourceId;
  return r'''
(() => {
  'use strict';
  const seriesId = '__SERIES_ID__';
  const allowed = (target) => {
    try {
      const uri = new URL(target, window.location.href);
      const parts = uri.pathname.split('/').filter(Boolean);
      return uri.protocol === 'https:' &&
        ['hongguoduanju.com', 'novelquickapp.com'].includes(uri.hostname) &&
        !uri.port && !uri.username && !uri.password &&
        parts.length === 3 && parts[0] === 'player' &&
        parts[1] === seriesId && /^[0-9]{1,32}$/.test(parts[2])
          ? uri.href : null;
    } catch (_) { return null; }
  };
  window.open = (target) => {
    const url = typeof target === 'string' ? allowed(target) : null;
    if (url) { window.location.assign(url); return window; }
    return null;
  };
  document.addEventListener('click', (event) => {
    const element = event.target instanceof Element
      ? event.target.closest('a[target="_blank"]') : null;
    if (!element) return;
    const url = allowed(element.href);
    if (!url) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    window.location.assign(url);
  }, true);
})();
'''.replaceAll('__SERIES_ID__', seriesId);
}

/// The user's ordinary browser uses the Hongguo website portal, which can
/// retain its *own* legitimate access session. Never copy credentials.
/// Use the website's normal first-party route for the embedded browser mode.
/// Authorization remains entirely owned by the website. No unlock is inferred.
String? hongguoWebviewEpisodeUrl(
  Drama drama,
  Episode episode, {
  required bool useOfficialWebsite,
}) {
  return useOfficialWebsite
      ? hongguoEpisodeUrlForSystemBrowser(drama, episode)
      : publicHongguoEpisodeUrl(drama, episode);
}

String? hongguoEpisodeUrlForSystemBrowser(Drama drama, Episode episode) {
  final embedded = publicHongguoEpisodeUrl(drama, episode);
  if (embedded == null) return null;
  final parts = Uri.parse(embedded).pathSegments;
  return 'https://hongguoduanju.com/player/${parts[1]}/${parts[2]}';
}

Future<bool> openPublicHongguoEpisodeInBrowser(
  Drama drama,
  Episode episode,
) async {
  if (!Platform.isWindows) return false;
  final url = hongguoEpisodeUrlForSystemBrowser(drama, episode);
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
