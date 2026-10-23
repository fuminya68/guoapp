import 'dart:io';

import 'package:flutter/material.dart';

import 'core_bridge.dart';
import 'hongguo_web_fallback.dart';
import 'hongguo_web_player_screen.dart';
import 'local_store.dart';
import 'models.dart';
import 'player_screen.dart';

/// Use the browser's own player on Windows for protected Hongguo episodes.
/// Other sources and local-only playback keep the native mpv player.
Widget buildPreferredPlaybackScreen({
  required DramaDetail detail,
  required int initialIndex,
  required AppRepository repository,
  required LocalStore store,
  double initialPosition = 0,
  bool localOnly = false,
}) {
  if (supportsHongguoInAppWebPlayback(
    windows: Platform.isWindows,
    detail: detail,
    episodeIndex: initialIndex,
    localOnly: localOnly,
  )) {
    return HongguoWebPlayerScreen(
      detail: detail,
      initialIndex: initialIndex,
      store: store,
    );
  }
  return PlayerScreen(
    detail: detail,
    initialIndex: initialIndex,
    initialPosition: initialPosition,
    repository: repository,
    store: store,
    localOnly: localOnly,
  );
}
