package core

import (
	"context"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestMediaSSRFURLRejectsLocalAndMalformedTargets(t *testing.T) {
	rejected := []string{
		"http://127.0.0.1:8080/admin", "https://localhost/", "https://foo.localhost/",
		"http://192.168.1.1/", "https://10.1.2.3/", "http://172.16.0.1/",
		"http://169.254.169.254/latest/meta-data/", "http://100.100.100.200/",
		"http://[::1]:8080/", "https://[fc00::1]/", "https://[::ffff:127.0.0.1]/",
		"http://[fe80::1]/", "http://foo.internal/", "https://printer.local/",
		"file:///etc/passwd", "gopher://example.org/", "https://user:pass@example.org/",
		"https://example.org:99999/", "http://metadata/", "http://example.test/",
	}
	for _, raw := range rejected {
		parsed, err := url.Parse(raw)
		if err == nil && mediaSSRFAllowedURL(parsed) == nil {
			t.Errorf("unsafe media URL was accepted: %s", raw)
		}
	}
	allowed := []string{
		"https://video.example.org/movie.m3u8?signature=a%2Fb",
		"http://93.184.216.34:8081/segment.ts",
		"https://[2606:4700:4700::1111]/master.m3u8",
	}
	for _, raw := range allowed {
		parsed, err := url.Parse(raw)
		if err != nil || mediaSSRFAllowedURL(parsed) != nil {
			t.Errorf("public media URL rejected: %s (%v)", raw, err)
		}
	}
}

func TestMediaSSRFDialRejectsPrivateIPsBeforeConnecting(t *testing.T) {
	for _, address := range []string{"127.0.0.1:9999", "10.0.0.1:443", "169.254.169.254:80", "[::1]:443"} {
		conn, err := mediaSSRFDial(context.Background(), "tcp", address)
		if conn != nil {
			conn.Close()
			t.Fatalf("private address was dialed: %s", address)
		}
		if err == nil || !strings.Contains(err.Error(), "非公网") {
			t.Errorf("unexpected result for %s: %v", address, err)
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254",
		"::1", "fc00::1", "::ffff:192.168.0.1", "100.64.0.1", "198.18.0.1"} {
		if err := mediaSSRFValidateAddress(ip); err == nil {
			t.Errorf("invalid resolved IP was allowed: %s", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		if addr, err := netip.ParseAddr(ip); err != nil || mediaSSRFValidateAddress(addr.String()) != nil {
			t.Errorf("public resolved IP blocked: %s", ip)
		}
	}
}

func TestMediaSSRFRedirectCannotTargetLocalService(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1:1234/private",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"http://home.local/",
	} {
		request, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := mediaSSRFCheckRedirect(request, nil); err == nil {
			t.Errorf("unsafe redirect accepted: %s", target)
		}
	}
	public, _ := http.NewRequest(http.MethodGet, "https://cdn.example.org/seg.ts", nil)
	if err := mediaSSRFCheckRedirect(public, nil); err != nil {
		t.Fatal("valid redirect blocked:", err)
	}
}

func TestMediaSSRFPlaylistRejectsPrivateSegments(t *testing.T) {
	session := &nativeStreamSession{assets: make(map[string]nativeStreamAsset)}
	server := &nativeStreamServer{address: "http://127.0.0.1:9988"}
	for _, raw := range []string{
		"#EXTM3U\nhttp://127.0.0.1:8111/admin\n",
		"#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"http://169.254.169.254/latest/meta-data/\"\n#EXTINF:5,\nhttps://video.example.org/movie.ts\n",
	} {
		if _, err := server.nativeRewrite("synthetic-session", session, raw, "https://video.example.org/playlist.m3u8"); err == nil {
			t.Fatalf("private HLS reference accepted: %s", raw)
		}
	}
	rewritten, err := server.nativeRewrite("synthetic-session", session, "#EXTM3U\n#EXTINF:5,\nsegment.ts\n", "https://video.example.org/playlist.m3u8")
	if err != nil || !strings.Contains(rewritten, "http://127.0.0.1:9988/synthetic-session/") {
		t.Fatalf("public HLS segment was not rewritten: %s, %v", rewritten, err)
	}
}
