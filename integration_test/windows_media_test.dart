import 'dart:io';

import 'package:duanju_app/core_bridge.dart';
import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/main.dart';
import 'package:duanju_app/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Windows-only real media_kit/GUI integration smoke, using only local fixtures.
/// No live provider account and no external websites are contacted.
const fixtureBase = String.fromEnvironment('FIXTURE_BASE_URL');

class _WindowsFixtureRepository extends AppRepository {
  static const drama = Drama(
    id: 'hongguo:windows-test',
    source: 'hongguo',
    title: 'Windows 媒体测试',
    episodes: 2,
  );
  final resolved = <int>[];

  @override
  Future<void> initialize() async {}

  @override
  Future<CatalogPage> cached(String source, {String category = ''}) async =>
      CatalogPage([]);

  @override
  Future<CatalogPage> catalog(
    String source, {
    int page = 1,
    String query = '',
    String category = '',
    bool force = false,
  }) async => CatalogPage([drama]);

  @override
  Future<String> cover(Drama drama, {bool force = false}) async => '';

  @override
  Future<DramaDetail> detail(Drama drama) async => DramaDetail(drama, [
    Episode({
      'id': '1',
      'source': 'hongguo',
      'currentEpisode': 1,
      'title': '第1集',
      'videoUrl': '$fixtureBase/clear.mp4',
    }, 1),
    Episode({
      'id': '2',
      'source': 'hongguo',
      'currentEpisode': 2,
      'title': '第2集',
      'videoUrl': '$fixtureBase/index.m3u8',
    }, 2),
  ]);

  @override
  Future<PlaybackPlan> resolve(
    Drama drama,
    Episode episode, {
    int quality = 0,
  }) async {
    resolved.add(episode.number);
    return PlaybackPlan(
      url: '$fixtureBase/${episode.number == 1 ? "clear.mp4" : "index.m3u8"}',
    );
  }

  @override
  Future<PlaybackPlan> fallback(PlaybackPlan current) async => current;

  @override
  Future<void> cancelPlayback() async {}

  @override
  Future<void> release(String session) async {}
}

void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  testWidgets('Windows UI: catalog, MP4 decoding, episode switch and HLS decoding',
      (tester) async {
    expect(Platform.isWindows, isTrue);
    expect(fixtureBase, startsWith('http://127.0.0.1:'));
    expect(const bool.fromEnvironment('DISABLE_REMOTE_IMAGES'), isTrue);
    SharedPreferences.setMockInitialValues({});
    MediaKit.ensureInitialized();

    final repository = _WindowsFixtureRepository();
    final store = LocalStore(await SharedPreferences.getInstance());
    final observations = <Map<String, Object?>>[];

    Future<void> until(bool Function() condition, String label) async {
      final elapsed = Stopwatch()..start();
      while (!condition()) {
        if (elapsed.elapsed > const Duration(seconds: 45)) {
          fail('Windows integration timeout: $label. Resolved: ${repository.resolved}');
        }
        await tester.pump(const Duration(milliseconds: 200));
      }
    }

    await tester.pumpWidget(DuanjuApp(repository: repository, store: store));
    await until(
      () => find.text(_WindowsFixtureRepository.drama.title).evaluate().isNotEmpty,
      'catalog load',
    );
    // Home-screen drama tiles intentionally start playback directly. The
    // detail-only 'episode-1' key is not part of that navigation route.
    await tester.tap(find.text(_WindowsFixtureRepository.drama.title).first);
    await until(() => find.byType(Video).evaluate().isNotEmpty, 'direct player navigation');
    await until(
      () => find.byKey(const ValueKey('play-episode-2')).evaluate().isNotEmpty,
      'desktop player episode controls',
    );
    final player = tester.widget<Video>(find.byType(Video)).controller.player;
    await player.setVolume(0);

    Future<void> decoded(int episode) async {
      await until(
        () => repository.resolved.isNotEmpty &&
          repository.resolved.last == episode &&
          (player.state.width ?? 0) > 0 &&
          player.state.duration.inSeconds >= 4 &&
          player.state.position.inMilliseconds >= 350,
        'media decode for episode $episode',
      );
      expect(find.text('暂时无法播放'), findsNothing);
      observations.add({
        'episode': episode,
        'width': player.state.width,
        'height': player.state.height,
        'positionMs': player.state.position.inMilliseconds,
        'durationMs': player.state.duration.inMilliseconds,
      });
    }

    await decoded(1);
    await tester.tap(find.byKey(const ValueKey('play-episode-2')));
    await decoded(2);
    expect(repository.resolved, containsAllInOrder([1, 2]));
    expect(tester.takeException(), isNull);
    binding.reportData ??= {};
    binding.reportData!['windowsMedia'] = observations;

    // PlayerScreen owns its Player and releases it asynchronously on unmount.
    // Disposing the same instance in the test triggers media_kit's
    // "[Player] has been disposed" assertion after both decodes passed.
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump(const Duration(milliseconds: 400));
    store.dispose();
  }, timeout: const Timeout(Duration(minutes: 4)));
}
