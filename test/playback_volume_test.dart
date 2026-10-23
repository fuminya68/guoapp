import 'package:flutter_test/flutter_test.dart';
import 'package:guoapp/playback_preferences.dart';

void main() {
  test('volume is stored across playback sessions and sources', () {
    const initial = PlaybackPreferences(volume: 37.5);
    final loaded = PlaybackPreferences.fromJson(initial.toJson());
    expect(loaded.volume, 37.5);
    expect(loaded.copyWith(quality: 1080).volume, 37.5);
    expect(PlaybackPreferences.fromJson({}).volume, 100);
  });

  test('invalid volume is rejected rather than saved', () {
    for (final value in <double>[-1, 101, double.nan, double.infinity]) {
      expect(
        () => PlaybackPreferences.fromJson({'volume': value}),
        throwsFormatException,
      );
    }
  });
}
