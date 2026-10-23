import 'package:duanju_app/lan_models.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const peer = LanPeer(
    id: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    pin: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    name: 'My phone',
    kind: 'phone',
    port: 53120,
  );

  test('newly discovered peer requires explicit approval', () {
    expect(lanPeerTrusted(null, peer), isFalse);
    expect(lanPeerTrusted({}, peer), isFalse);
    expect(lanPeerTrusted({'unrelated': peer.pin}, peer), isFalse);
  });

  test('only a previously approved exact certificate pin is trusted', () {
    expect(lanPeerTrusted({peer.id: peer.pin}, peer), isTrue);
    expect(lanPeerTrusted({peer.id: 'c' * 64}, peer), isFalse);
    expect(lanPeerTrusted({peer.id: 42}, peer), isFalse);
  });
}
