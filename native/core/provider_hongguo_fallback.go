package core

import (
    "context"
    "errors"
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

type hongguoWebCatalogFetch func(context.Context, string, int) ([]Drama, int, error)

// If the app catalogue has failed, prefer a nonempty successful web result.
// Preserve the original failure when web fallback is inaccessible or empty,
// so the UI reports an actionable error rather than a false empty catalogue.
func recoverHongguoAppCatalog(
    ctx context.Context,
    category string,
    page int,
    appError error,
    fetch hongguoWebCatalogFetch,
) ([]Drama, int, error) {
    items, totalPages, err := fetch(ctx, category, page)
    if err != nil {
        return nil, 0, errors.Join(appError, fmt.Errorf("红果网页目录备援失败: %w", err))
    }
    if len(items) == 0 {
        return nil, 0, errors.Join(appError, errors.New("红果网页目录备援未返回剧集"))
    }
    return items, totalPages, nil
}
