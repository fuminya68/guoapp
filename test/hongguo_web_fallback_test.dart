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
  test('rejects non-Hongguo sources and untrusted identifiers', () {
    for (final id in ['../private', '1&next=https://evil.example', 'foo', '',List.filled(33, '0').join()]) {
      expect(officialHongguoDramaUrl(drama('hongguo', id)), isNull);
    }
    expect(officialHongguoDramaUrl(drama('huangguoai', '7615112833881082905')), isNull);
  });
}
