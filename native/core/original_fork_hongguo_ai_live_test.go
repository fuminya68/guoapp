package core

import (
  "context"
  "fmt"
  "io"
  "net/http"
  "testing"
  "time"
)

func TestOriginalForkHongguoAILiveParity(t *testing.T) {
  ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
  defer cancel()
  engine, err := newNativeEngine(t.TempDir())
  if err != nil { t.Fatal(err) }
  t.Cleanup(engine.downloads.close)
  nativeItems, nativeErr := engine.downloader.fetchHongguoAppCatalogCategory(ctx, "ai_series")
  t.Logf("STAGE_APP_CATALOG items=%d err=%v", len(nativeItems), nativeErr)

  items, _, err := engine.downloader.fetchHongguoCategoryPage(ctx, "ai-drama?page=1", "AI剧")
  if err != nil { t.Fatalf("STAGE_WEB_CATALOG err=%v", err) }
  if len(items) == 0 { t.Fatal("STAGE_WEB_CATALOG empty") }
  t.Logf("STAGE_WEB_CATALOG items=%d", len(items))

  detailOK, streamOK := 0, 0
  var last error
  for i, drama := range items {
    if i >= 5 { break }
    _, chapters, detailErr := engine.downloader.fetchHongguoChapters(ctx, drama.SourceID)
    if detailErr != nil || len(chapters) == 0 {
      last = detailErr
      t.Logf("STAGE_DETAIL sample=%d ok=false err=%v", i+1, detailErr)
      continue
    }
    detailOK++
    t.Logf("STAGE_DETAIL sample=%d ok=true episodes=%d", i+1, len(chapters))
    for j, chapter := range chapters {
      if j >= 2 { break }
      media, mediaErr := engine.downloader.resolveHongguoMedia(ctx, Task{DramaID: drama.ID, DramaTitle: drama.DisplayTitle(), Chapter: chapter, Index: j+1})
      if mediaErr != nil {
        last = mediaErr
        t.Logf("STAGE_MEDIA sample=%d episode=%d ok=false err=%v", i+1, j+1, mediaErr)
        continue
      }
      choices := nativePlaybackChoices(media, 0)
      if len(choices.media) == 0 { t.Logf("STAGE_MEDIA sample=%d episode=%d empty-choices", i+1, j+1); continue }
      plan, planErr := engine.nativeOpenPlayback(ctx, choices)
      if planErr != nil { last = planErr; continue }
      request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, plan.URL, nil)
      if requestErr != nil { last = requestErr; engine.nativeReleasePlayback(plan.Session); continue }
      response, requestErr := http.DefaultClient.Do(request)
      if requestErr != nil { last = requestErr; engine.nativeReleasePlayback(plan.Session); continue }
      n, readErr := io.CopyN(io.Discard, response.Body, 1024)
      response.Body.Close()
      engine.nativeReleasePlayback(plan.Session)
      if response.StatusCode == http.StatusOK && n > 0 && (readErr == nil || readErr == io.EOF) {
        streamOK++
        t.Logf("STAGE_STREAM sample=%d episode=%d ok=true bytes=%d",i+1,j+1,n)
        break
      }
      last=fmt.Errorf("HTTP %d, read=%v, bytes=%d",response.StatusCode,readErr,n)
      t.Logf("STAGE_STREAM sample=%d episode=%d ok=false %v",i+1,j+1,last)
    }
    if streamOK > 0 { break }
  }
  if streamOK == 0 { t.Fatalf("ORIGINAL_FORK_MEDIA_FAILURE web_titles=%d detail_ok=%d stream_ok=0 last=%v",len(items),detailOK,last) }
  t.Log("ORIGINAL_FORK_MEDIA_SUCCESS: stream bytes received, not decoded video-frame verification")
}
