package core

import (
    "context"
    "errors"
    "strings"
    "testing"
)

func TestHongguoWebCategoryRoute(t *testing.T) {
    cases := []struct {
        category string
        route string
        label string
        allowed bool
    }{
        {"", "real-drama", "真人剧", true},
        {"short_play", "real-drama", "真人剧", true},
        {"comic_series", "comic-drama", "漫剧", true},
        {"ai_series", "ai-drama", "AI剧", true},
        {"unknown", "", "", false},
        {"../../admin", "", "", false},
    }
    for _, tc := range cases {
        t.Run(tc.category, func(t *testing.T) {
            route, label, ok := hongguoWebCategoryRoute(tc.category)
            if route != tc.route || label != tc.label || ok != tc.allowed {
                t.Fatalf("category %q: got (%q,%q,%v), want (%q,%q,%v)",
                    tc.category, route, label, ok, tc.route, tc.label, tc.allowed)
            }
        })
    }
}

func TestRecoverHongguoAppCatalogFromAIWeb(t *testing.T) {
    apiErr := errors.New("App API HTTP 200 empty body")
    hit := false
    expected := Drama{ID: "hongguo:123", Source: "hongguo", Title: "AI剧"}
    items, pages, err := recoverHongguoAppCatalog(
        context.Background(), "ai_series", 2, apiErr,
        func(ctx context.Context, category string, page int) ([]Drama, int, error) {
            hit = true
            route, _, ok := hongguoWebCategoryRoute(category)
            if !ok || route != "ai-drama" || page != 2 {
                t.Fatalf("wrong fallback: %s page=%d", route, page)
            }
            return []Drama{expected}, 3, nil
        })
    if !hit || err != nil || len(items) != 1 || items[0].ID != expected.ID || pages != 3 {
        t.Fatalf("fallback did not recover AI feed: hit=%v pages=%d items=%v err=%v",
            hit, pages, items, err)
    }
}

func TestRecoverHongguoAppCatalogPreservesBothFailures(t *testing.T) {
    appErr := errors.New("app API empty")
    webErr := errors.New("web API unavailable")
    items, _, err := recoverHongguoAppCatalog(
        context.Background(), "ai_series", 1, appErr,
        func(context.Context, string, int) ([]Drama, int, error) {
            return nil, 0, webErr
        })
    if len(items) != 0 || !errors.Is(err, appErr) || !errors.Is(err, webErr) {
        t.Fatalf("missing original or web error: %v", err)
    }
}

func TestRecoverHongguoAppCatalogEmptyWebResponseIsFailure(t *testing.T) {
    appErr := errors.New("app API empty")
    items, _, err := recoverHongguoAppCatalog(
        context.Background(), "ai_series", 1, appErr,
        func(context.Context, string, int) ([]Drama, int, error) {
            return nil, 1, nil
        })
    if len(items) != 0 || !errors.Is(err, appErr) || !strings.Contains(err.Error(), "备援未返回剧集") {
        t.Fatalf("empty web response incorrectly treated as successful: %v", err)
    }
}

func TestHongguoWebCategoryRejectsOutOfRangePageBeforeRequest(t *testing.T) {
    d := &Downloader{}
    for _, page := range []int{0, -1, 501} {
        _, _, err := d.fetchHongguoWebCatalogCategory(context.Background(), "ai_series", page)
        if err == nil {
            t.Errorf("page %d unexpectedly allowed", page)
        }
    }
    if _, _, err := d.fetchHongguoWebCatalogCategory(context.Background(), "https://untrusted.example/", 1); err == nil {
        t.Fatal("unexpected category unexpectedly allowed")
    }
}
