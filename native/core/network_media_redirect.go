package core

import (
    "errors"
    "net/http"
)

// Only the original provider origin may receive the ephemeral preview token.
// Go's default redirect machinery deliberately restricts Authorization and
// Cookie, but copies arbitrary custom headers such as X-Preview-Token even to
// unrelated HTTPS origins. Never allow that behavior for playback credentials.
func scopedMediaPreviewClient(original *http.Client, trustedOrigin string) *http.Client {
    scoped := *original
    existing := original.CheckRedirect
    scoped.CheckRedirect = func(request *http.Request, via []*http.Request) error {
        if len(via) >= 10 {
            return errors.New("媒体重定向过多")
        }
        if providerMediaOrigin(request.URL) != trustedOrigin {
            request.Header.Del("X-Preview-Token")
        }
        if existing != nil {
            return existing(request, via)
        }
        return nil
    }
    return &scoped
}
