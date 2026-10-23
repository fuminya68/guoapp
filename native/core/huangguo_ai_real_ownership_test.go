package core

import (
 "context"
 "crypto/sha256"
 "fmt"
 "net/url"
 "strings"
 "testing"
 "time"
)

func TestHuangguoAIRealEpisodeOwnershipLive(t *testing.T) {
 ctx,cancel:=context.WithTimeout(context.Background(),115*time.Second)
 defer cancel()
 engine,err:=newNativeEngine(t.TempDir())
 if err!=nil {t.Fatal(err)}
 t.Cleanup(engine.downloads.close)
 type sample struct {id,title string}
 samples:=[]sample{
  {"12","神瞳覺醒 第一季"},
  {"117","高三愛情故事"},
  {"417","AI魔改 雙規"},
 }
 firstURLs:=map[string]string{}
 for _,sample:=range samples {
  drama,chapters,err:=engine.downloader.fetchHuangguoAIDetail(ctx,sample.id)
  if err!=nil {t.Fatalf("cannot fetch %s (%s): %v",sample.title,sample.id,err)}
  if len(chapters)==0 {t.Fatalf("no drama-owned episodes for %s (%s)",sample.title,sample.id)}
  for _,chapter:=range chapters {
   if chapter.PageURL=="" {continue}
   parsed,pErr:=url.Parse(chapter.PageURL)
   if pErr!=nil {t.Fatal(pErr)}
   if strings.HasPrefix(parsed.Path,"/video/") && aiEpisodeKey(chapter.PageURL,sample.id)=="" {
    t.Fatalf("cross-drama video link in %q: %q",sample.title,chapter.PageURL)
   }
  }
  first:=chapters[0]
  if chapterEpisodeNumber(first,1)!=1 {
    t.Fatalf("first episode omitted for series %s: first=%s",sample.id,first.ID)
  }
  if sample.id!="417" && !strings.HasSuffix(first.ID,":ep-1") {
    t.Fatalf("the published first episode was not recovered for series %s: first=%s",sample.id,first.ID)
  }

  if first.PageURL!="" {
   if previous,exists:=firstURLs[first.PageURL];exists {
    t.Fatalf("two unrelated series %s and %s share the same first-episode page",previous,sample.title)
   }
   firstURLs[first.PageURL]=sample.title
  }
  t.Logf("OWNED_SERIES id=%s title=%q resolved_title=%q chapters=%d first_chapter_id=%q",sample.id,sample.title,drama.DisplayTitle(),len(chapters),first.ID)
  media,mediaErr:=engine.downloader.resolveProviderMedia(ctx,Task{
   DramaID:providerDramaID(sourceHuangguoAI,sample.id),
   Chapter:first, Index:1,
  })
  if mediaErr!=nil {
   t.Logf("MEDIA_STATUS series=%s resolved=false reason=%v",sample.id,mediaErr)
   continue
  }
  digest:=sha256.Sum256([]byte(media.URL))
  t.Logf("MEDIA_STATUS series=%s resolved=true address_fingerprint=%s playlist=%v",sample.id,fmt.Sprintf("%x",digest[:6]),media.Playlist!="")
 }
}
