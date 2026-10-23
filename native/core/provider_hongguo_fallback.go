package core

import (
    "context"
    "fmt"
    "strconv"
)

// Hongguo exposes a separate public web catalogue for each App feed.
// This allows catalogue browsing to recover when the App endpoint sends
// HTTP 200 with no body or an invalid response. It does not change any
// media credential, TLS verification, SSRF, or playback authorization.
func hongguoWebCategoryRoute(category string) (route, label string, ok bool) {
    switch category {
    case "", "short_play":
        return "real-drama", "真人剧", true
    case "comic_series":
        return "comic-drama", "漫剧", true
    case "ai_series":
        return "ai-drama", "AI剧", true
    default:
        return "", "", false
    }
}

func (d *Downloader) fetchHongguoWebCatalogCategory(ctx context.Context, category string, page int) ([]Drama, int, error) {
    route, label, ok := hongguoWebCategoryRoute(category)
    if !ok {
        return nil, 0, fmt.Errorf("红果不支持的网页分类：%q", category)
    }
    if page < 1 || page > 500 {
        return nil, 0, fmt.Errorf("红果网页分类页码无效：%d", page)
    }
    return d.fetchHongguoCategoryPage(ctx, route+"?page="+strconv.Itoa(page), label)
}
