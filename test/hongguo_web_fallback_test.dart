import 'package:flutter_test/flutter_test.dart';
import 'package:duanju_app/hongguo_web_fallback.dart';
import 'package:duanju_app/models.dart';

void main() {
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
  Drama drama(String source, String sourceId) =>
      Drama(id: '$source:$sourceId', source: source, sourceId: sourceId, title: '測試');
  test('known public Hongguo detail deep link uses numeric series ID only', () {
    expect(officialHongguoDramaUrl(drama('hongguo', '7615112833881082905')),
      'https://hongguoduanju.com/detail?series_id=7615112833881082905');
  });
  Episode episode(String videoUrl) =>
      Episode({'videoUrl': videoUrl, 'id': 'episode', 'title': '第一集'}, 1);

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
