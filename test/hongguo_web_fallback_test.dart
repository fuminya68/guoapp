import 'package:flutter_test/flutter_test.dart';
import 'package:duanju_app/hongguo_web_fallback.dart';
import 'package:duanju_app/models.dart';

void main() {
  Drama drama(String source, String sourceId) =>
      Drama(id: '$source:$sourceId', source: source, sourceId: sourceId, title: '測試');
  Episode episode(String videoUrl) =>
      Episode({'videoUrl': videoUrl, 'id': 'episode', 'title': '第一集'}, 1);

  test('Windows direct play selects only validated Hongguo URLs', () {
    final item = Drama(id:'hongguo:123', source:'hongguo', sourceId:'123', title:'修仙滿級回現代');
    final detail = DramaDetail(item, [
      Episode({'videoUrl': 'hongguo-cenc://456', 'currentEpisode': 1}, 1),
      Episode({'videoUrl': 'https://not-allowed.example/movie', 'currentEpisode': 2}, 2),
    ]);
    expect(supportsHongguoInAppWebPlayback(windows:true, detail:detail, episodeIndex:0), isTrue);
    expect(supportsHongguoInAppWebPlayback(windows:true, detail:detail, episodeIndex:1), isFalse);
    expect(supportsHongguoInAppWebPlayback(windows:true, detail:detail, episodeIndex:-1), isFalse);
    expect(supportsHongguoInAppWebPlayback(windows:true, detail:detail, episodeIndex:2), isFalse);
    expect(supportsHongguoInAppWebPlayback(windows:true, detail:detail, episodeIndex:0,localOnly:true), isFalse);
    expect(supportsHongguoInAppWebPlayback(windows:false, detail:detail, episodeIndex:0), isFalse);
  });

  test('in-app top-level navigation only allows trusted HTTPS hosts', () {
    expect(isTrustedHongguoWebviewNavigation('https://novelquickapp.com/player/123/456'),isTrue);
    expect(isTrustedHongguoWebviewNavigation('https://hongguoduanju.com/detail?series_id=123'),isTrue);
    for (final u in [
      'http://novelquickapp.com/player/123/456',
      'https://novelquickapp.com.evil.example/player/123/456',
      'https://novelquickapp.com:8443/player/123/456',
      'https://attacker@novelquickapp.com/player/123/456',
      'javascript:alert(1)',
      'file:///C:/Windows/System32/',
      'https://example.com/',
    ]) {expect(isTrustedHongguoWebviewNavigation(u),isFalse);}
  });


  test('website episode navigation only accepts actual current-series episodes', () {
    final detail = DramaDetail(
      drama('hongguo', '123'),
      [
        episode('hongguo-cenc://456'),
        episode('hongguo-cenc://789'),
      ],
    );
    expect(hongguoEpisodeIndexFromWebviewUrl(
      detail, 'https://novelquickapp.com/player/123/789'
    ), 1);
    for (final bad in [
      'https://novelquickapp.com/player/124/789',
      'https://novelquickapp.com/player/123/888',
      'https://novelquickapp.com/player/123/789/extra',
      'https://novelquickapp.com.evil.test/player/123/789',
      'file:///C:/Windows/',
      'https://novelquickapp.com:8443/player/123/789',
    ]) {
      expect(hongguoEpisodeIndexFromWebviewUrl(detail,bad), isNull);
    }
  });

  test('popup handler allows only matching-series browser episode routes', () {
    final script = hongguoSameSeriesNavigationScript(drama('hongguo','123'));
    expect(script, isNotNull);
    expect(script, contains("const seriesId = '123'"));
    expect(script, contains("window.location.assign(url)"));
    expect(script, contains("document.addEventListener('click'"));
    expect(hongguoSameSeriesNavigationScript(drama('hongguo','../123')), isNull);
    expect(hongguoSameSeriesNavigationScript(drama('huangguoai','123')), isNull);
  });

  test('later Hongguo episodes keep exact public-browser URLs', () {
    final show = drama('hongguo', '7615112833881082905');
    final chapters = List.generate(
      10,
      (i) => Episode({
        'videoUrl': 'hongguo-cenc://${1000 + i}',
        'currentEpisode': i + 1,
      }, i + 1),
    );
    final detail = DramaDetail(show, chapters);
    for (final i in [0, 4, 5, 6, 9]) {
      expect(
        publicHongguoEpisodeUrl(detail.drama, detail.episodes[i]),
        'https://novelquickapp.com/player/7615112833881082905/${1000 + i}',
      );
      expect(
        hongguoEpisodeIndexFromWebviewUrl(
          detail,
          'https://novelquickapp.com/player/7615112833881082905/${1000 + i}',
        ),
        i,
      );
    }
  });

  test('working browser URLs for episodes 16/17 are exact, but embedded site stays unchanged', () {
    final show = drama('hongguo', '7697209252699442201');
    final chapters = [
      episode('hongguo-cenc://7697250072865541144'),
      episode('hongguo-cenc://7697249981073198104'),
    ];
    final detail = DramaDetail(show, chapters);
    expect(hongguoEpisodeUrlForSystemBrowser(show, chapters[0]),
      'https://hongguoduanju.com/player/7697209252699442201/7697250072865541144');
    expect(hongguoEpisodeUrlForSystemBrowser(show, chapters[1]),
      'https://hongguoduanju.com/player/7697209252699442201/7697249981073198104');
    expect(publicHongguoEpisodeUrl(show, chapters[1]),
      'https://novelquickapp.com/player/7697209252699442201/7697249981073198104');
    expect(hongguoEpisodeIndexFromWebviewUrl(detail,
      'https://hongguoduanju.com/player/7697209252699442201/7697249981073198104'), 1);
    expect(hongguoEpisodeIndexFromWebviewUrl(detail,
      'https://novelquickapp.com/player/7697209252699442201/7697250072865541144'), 0);
    expect(hongguoEpisodeIndexFromWebviewUrl(detail,
      'https://hongguoduanju.com.evil.example/player/7697209252699442201/7697250072865541144'), isNull);
    expect(hongguoEpisodeUrlForSystemBrowser(
      drama('huangguoai','7697209252699442201'), chapters[0]), isNull);
  });

  test('official embedded mode and old direct mode stay distinct', () {
    final show = drama('hongguo', '7697209252699442201');
    final ep6 = episode('hongguo-cenc://7697250072865541144');
    expect(hongguoWebviewEpisodeUrl(show, ep6, useOfficialWebsite: true),
      'https://hongguoduanju.com/player/7697209252699442201/7697250072865541144');
    expect(hongguoWebviewEpisodeUrl(show, ep6, useOfficialWebsite: false),
      'https://novelquickapp.com/player/7697209252699442201/7697250072865541144');
    expect(officialHongguoDramaUrl(show),
      'https://hongguoduanju.com/detail?series_id=7697209252699442201');
    for (final source in ['huangguoai', 'evil']) {
      expect(hongguoWebviewEpisodeUrl(drama(source,'7697209252699442201'),
        ep6, useOfficialWebsite: true), isNull);
    }
  });

  test('Windows browser launches only from protected system directory', () {
    expect(trustedWindowsExplorerPath(r'C:\Windows'), r'C:\Windows\explorer.exe');
    expect(trustedWindowsExplorerPath(r'D:\WINDOWS'), r'D:\WINDOWS\explorer.exe');
    for (final root in [
      null,
      '',
      'explorer.exe',
      r'C:\Temp',
      r'C:\Windows\..\Temp',
      r'\\attacker\share',
      r'C:/Windows',
    ]) {
      expect(trustedWindowsExplorerPath(root), isNull);
    }
  });
  test('known public Hongguo detail deep link uses numeric series ID only', () {
    expect(officialHongguoDramaUrl(drama('hongguo', '7615112833881082905')),
      'https://hongguoduanju.com/detail?series_id=7615112833881082905');
  });

  test('opens only the exact public episode for the requested series', () {
    expect(
      publicHongguoEpisodeUrl(
        drama('hongguo', '7615112833881082905'),
        episode('hongguo-cenc://7615112833881082906'),
      ),
      'https://novelquickapp.com/player/7615112833881082905/7615112833881082906',
    );
  });

  test('rejects untrusted or incomplete episode identifiers', () {
    for (final url in [
      'https://untrusted.example/',
      'hongguo-cenc://../private',
      'hongguo-cenc://7?next=https://bad.example',
      'hongguo-cenc://',
      'hongguo-cenc://${List.filled(33, '1').join()}',
    ]) {
      expect(publicHongguoEpisodeUrl(drama('hongguo', '123'), episode(url)), isNull);
    }
    expect(publicHongguoEpisodeUrl(drama('huangguoai', '123'), episode('hongguo-cenc://321')), isNull);
    expect(publicHongguoEpisodeUrl(drama('hongguo', '123/456'), episode('hongguo-cenc://321')), isNull);
  });

  test('rejects non-Hongguo sources and untrusted identifiers', () {
    for (final id in ['../private', '1&next=https://evil.example', 'foo', '',List.filled(33, '0').join()]) {
      expect(officialHongguoDramaUrl(drama('hongguo', id)), isNull);
    }
    expect(officialHongguoDramaUrl(drama('huangguoai', '7615112833881082905')), isNull);
  });
}
