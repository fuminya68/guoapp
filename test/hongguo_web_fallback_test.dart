import 'package:flutter_test/flutter_test.dart';
import 'package:duanju_app/hongguo_web_fallback.dart';
import 'package:duanju_app/models.dart';

void main() {
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
