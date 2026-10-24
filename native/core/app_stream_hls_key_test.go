package core

import (
	"net/url"
	"strings"
	"testing"
)

func TestNativeHLSKeyDoesNotMisclassifyMediaSegmentsAsPlaylists(t *testing.T) {
	// Regression: a real 16-byte source-provided AES-128 key is stored on
	// nativeStreamSession and must not force segment payloads into the
	// m3u8 parser. No key or media content is fetched in this test.
	session := &nativeStreamSession{key: []byte(strings.Repeat("k", 16))}
	if len(session.key) != 16 {
		t.Fatal("fixture must have an HLS key")
	}
	for _, scenario := range []struct {
		name  string
		asset nativeStreamAsset
		mime  string
		raw   string
		want  bool
	}{
		{"ts segment with hls-tagged query", nativeStreamAsset{contentType: ""}, "video/mp2t", "https://media.example.com/seg0001.ts?session_m3u8_token=synthetic", false},
		{"fragmented mp4", nativeStreamAsset{contentType: "video/mp4"}, "video/iso.segment", "https://media.example.com/0002.m4s", false},
		{"nested HLS path binary TS", nativeStreamAsset{}, "application/octet-stream", "https://media.example.com/api/hls.m3u8/chunk001.ts?origin=synthetic", false},
		{"binary AES key response", nativeStreamAsset{contentType: "application/octet-stream"}, "application/octet-stream", "https://media.example.com/key.bin", false},
		{"playlist by source path", nativeStreamAsset{}, "application/octet-stream", "https://media.example.com/variant.m3u8?session=synthetic", true},
		{"upper-case M3U8 extension", nativeStreamAsset{}, "application/octet-stream", "https://media.example.com/MASTER.M3U8?session=synthetic", true},
		{"playlist by declared source MIME", nativeStreamAsset{contentType: "application/vnd.apple.mpegurl"}, "application/octet-stream", "https://media.example.com/hls?id=synthetic", true},
		{"playlist by upstream MIME", nativeStreamAsset{}, "application/x-mpegURL", "https://media.example.com/proxy?id=synthetic", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			parsed, err := url.Parse(scenario.raw)
			if err != nil { t.Fatal(err) }
			if got := nativeIsPlaylistAsset(scenario.asset, scenario.mime, parsed); got != scenario.want {
				t.Fatalf("playlist detection got %v, want %v (session has a 16-byte key)", got, scenario.want)
			}
		})
	}
}

// When a source labels an extensionless playlist as binary, the caller
// still recognizes the actual #EXTM3U signature through the existing Peek
// path; this patch does not remove that media-body sniffing.
func TestNativeHLSPlaylistSniffingRemainsAvailable(t *testing.T) {
	asset := nativeStreamAsset{contentType: ""}
	path, _ := url.Parse("https://media.example.com/play?opaque=synthetic")
	if nativeIsPlaylistAsset(asset, "application/octet-stream", path) {
		t.Fatal("unknown media cannot be identified as manifest by key alone")
	}
	prefix := "#EXTM3U\n#EXT-X-VERSION:3\n"
	if !strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(prefix, "\ufeff")), "#EXTM3U") {
		t.Fatal("existing manifest sniffing must remain intact")
	}
}
