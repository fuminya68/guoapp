package core

import (
    "io"
    "net/http"
    "net/url"
    "strings"
    "testing"
)

type mediaPreviewRoundTrip func(*http.Request) (*http.Response, error)

func (callback mediaPreviewRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
    return callback(r)
}

func mediaPreviewURL(text string) *url.URL {
    value, err := url.Parse(text)
    if err != nil { panic(err) }
    return value
}

func TestPlaybackPreviewTokenNeverLeavesProviderOriginViaRedirect(t *testing.T) {
    const secret = "SECRET-DO-NOT-LEAK-TO-OTHER-SITE"
    cases := []struct {
        name, target string
        allowed bool
    }{
        {"different-host", "https://attacker.example/collect", false},
        {"different-subdomain", "https://media.huangguo.video/collect", false},
        {"different-scheme", "http://huangguo.video/collect", false},
        {"different-port", "https://huangguo.video:8443/collect", false},
        {"same-origin", "https://huangguo.video/other-key", true},
    }
    for _, testcase := range cases {
        t.Run(testcase.name, func(t *testing.T) {
            visits := 0
            original := &http.Client{Transport: mediaPreviewRoundTrip(func(r *http.Request) (*http.Response, error) {
                visits++
                if visits == 1 {
                    if r.Header.Get("X-Preview-Token") != secret {
                        t.Error("initial source request lost its token")
                    }
                    return &http.Response{
                        StatusCode: http.StatusFound,
                        Header: http.Header{"Location": []string{testcase.target}},
                        Body: io.NopCloser(strings.NewReader("")),
                        Request: r,
                    }, nil
                }
                got := r.Header.Get("X-Preview-Token")
                if testcase.allowed && got != secret {
                    t.Errorf("redirect to original origin lost token: %q", got)
                }
                if !testcase.allowed && got != "" {
                    t.Errorf("preview token leaked to %s: %q", testcase.target, got)
                }
                return &http.Response{
                    StatusCode: http.StatusOK,
                    Header: make(http.Header),
                    Body: io.NopCloser(strings.NewReader("safe")),
                    Request: r,
                }, nil
            })}
            guarded := scopedMediaPreviewClient(original, "https://huangguo.video")
            req, err := http.NewRequest(http.MethodGet, "https://huangguo.video/api/hls_key/fixture", nil)
            if err != nil { t.Fatal(err) }
            req.Header.Set("X-Preview-Token", secret)
            response, err := guarded.Do(req)
            if err != nil { t.Fatal(err) }
            response.Body.Close()
            if visits != 2 { t.Fatalf("expected both request and redirect, got %d", visits) }
        })
    }
}

func TestPlaybackTokenRedirectPreservesSSRFRedirectGuard(t *testing.T) {
    client := &http.Client{CheckRedirect: mediaSSRFCheckRedirect}
    guarded := scopedMediaPreviewClient(client, "https://huangguo.video")
    for _, dest := range []string{"http://127.0.0.1/private", "http://169.254.169.254/latest/meta-data", "http://192.168.1.1/"} {
        req, err := http.NewRequest(http.MethodGet, dest, nil)
        if err != nil { t.Fatal(err) }
        req.Header.Set("X-Preview-Token", "SENSITIVE")
        if err := guarded.CheckRedirect(req, []*http.Request{{URL:mediaPreviewURL("https://huangguo.video/api/hls_key/test")}}); err == nil {
            t.Fatalf("redirect to nonpublic target was allowed: %s", dest)
        }
        if req.Header.Get("X-Preview-Token") != "" {
            t.Fatalf("token was not cleared on rejected redirect: %s", dest)
        }
    }
}
