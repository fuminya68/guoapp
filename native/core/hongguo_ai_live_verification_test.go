package core

import (
  "context"
  "io"
  "net/http"
  "testing"
  "time"
)

func TestHongguoAILiveCatalogDetailAndMedia(t *testing.T) {
  ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
  defer cancel()
  engine, err := newNativeEngine(t.TempDir())
  if err != nil { t.Fatal(err) }
  t.Cleanup(engine.downloads.close)
  dramas, _, err := engine.downloader.fetchHongguoWebCatalogCategory(ctx, "ai_series", 1)
  if err != nil { t.Fatalf("AI web catalogue is unavailable: %v", err) }
  if len(dramas) == 0 { t.Fatal("AI catalogue returned zero titles") }
  t.Logf("AI catalogue returned %d entries", len(dramas))
  var failed error
  for i, drama := range dramas {
    if i >= 5 { break }
    _, chapters, err := engine.downloader.fetchHongguoChapters(ctx, drama.SourceID)
    if err != nil || len(chapters) == 0 { failed = err; continue }
    for j, chapter := range chapters {
      if j >= 2 { break }
      media, err := engine.downloader.resolveHongguoMedia(ctx, Task{DramaID: drama.ID, DramaTitle: drama.DisplayTitle(), Chapter: chapter, Index: j+1})
      if err != nil { failed = err; continue }
      choices := nativePlaybackChoices(media, 0)
      if len(choices.media) == 0 { continue }
      plan, err := engine.nativeOpenPlayback(ctx, choices)
      if err != nil { failed = err; continue }
      request, err := http.NewRequestWithContext(ctx, http.MethodGet, plan.URL, nil)
      if err != nil { engine.nativeReleasePlayback(plan.Session); failed = err; continue }
      response, err := http.DefaultClient.Do(request)
      if err != nil { engine.nativeReleasePlayback(plan.Session); failed = err; continue }
      n, readErr := io.CopyN(io.Discard, response.Body, 1024)
      response.Body.Close()
      engine.nativeReleasePlayback(plan.Session)
      if response.StatusCode == http.StatusOK && n > 0 && (readErr == nil || readErr == io.EOF) {
        t.Logf("AI media bytes received: bytes=%d (not a decoder/GUI playback verification)", n)
        return
      }
      failed = readErr
    }
  }
  t.Fatalf("AI catalogue was reachable, but no episode yielded stream bytes; last error: %v", failed)
}
