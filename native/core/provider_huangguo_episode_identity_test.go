package core

import (
 "strings"
 "context"
 "testing"
)

func TestHuangguoAIEpisodeParserRejectsGlobalHotAndRelatedTitles(t *testing.T) {
 detail := "https://huangguoai.com/detail/12/"
 common := `<nav><a class="hg-search-suggest__hot-title" href="/video/9919/">熱門其他作品</a>
 <a class="hg-search-suggest__hot-title" href="/video/5161/">另一部作品</a></nav>
 <section><a class="hg-drama-card__cover-link" href="/video/8954/">相關推薦</a></section>`
 if got := parseHuangguoAIEpisodes(common, detail, "12"); len(got)!=0 {
   t.Fatalf("unrelated video links were misidentified as drama episodes: %+v",got)
 }
 valid := common + `<section><a href="/video/12/ep-1/">觀看</a><a href="/video/12/ep-2/">觀看</a></section>`
 got := parseHuangguoAIEpisodes(valid,detail,"12")
 if len(got)!=2 || got[0].Key!="ep-1" || got[1].Key!="ep-2" ||
  got[0].Index!=1 || got[1].Index!=2 ||
  !strings.Contains(got[0].URL,"/video/12/ep-1/") {
   t.Fatalf("valid source-linked chapters lost: %+v",got)
 }
 single := parseHuangguoAIEpisodes(common+`<a href="/video/12/">播放此劇</a>`,detail,"12")
 if len(single)!=1 || single[0].Key!="1" {
   t.Fatalf("source-linked single-video detail was lost: %+v",single)
 }
}

func TestHuangguoAIInitialDataDoesNotBorrowAnotherEpisode(t *testing.T) {
 script := func(content string) string {return `<script id="videoInitialData">`+content+`</script>`}
 two := script(`{"epPlaySrcs":{"1":"https://media.example.test/first.m3u8","2":"https://media.example.test/second.m3u8"}}`)
 if got:=parseAIVideoInitialData(two,"https://huangguoai.com/video/12/ep-9/");got!="" {
   t.Fatalf("nonexistent episode borrowed a different stream: %q",got)
 }
 if got:=parseAIVideoInitialData(two,"https://huangguoai.com/detail/12/");got!="" {
   t.Fatalf("multi-episode detail without selector borrowed a stream: %q",got)
 }
 if got:=parseAIVideoInitialData(two,"https://huangguoai.com/video/12/ep-2/");got!="https://media.example.test/second.m3u8" {
   t.Fatalf("specified episode did not resolve: %q",got)
 }
 contradictory :=script(`{"ep":"1","videoSrc":"https://media.example.test/first.m3u8","epPlaySrcs":{"1":"https://media.example.test/first.m3u8","2":"https://media.example.test/second.m3u8"}}`)
 if got:=parseAIVideoInitialData(contradictory,"https://huangguoai.com/video/12/ep-2/");got!=""{
   t.Fatalf("contradictory episode metadata was accepted: %q",got)
 }
 one := script(`{"epPlaySrcs":{"1":"https://media.example.test/only.m3u8"}}`)
 if got:=parseAIVideoInitialData(one,"https://huangguoai.com/detail/12/");got!="https://media.example.test/only.m3u8" {
   t.Fatalf("single-video playback was lost: %q",got)
 }
}

func TestHuangguoAIDetailFirstEpisodeOnlyFromMatchingMetadata(t *testing.T) {
 makeScript:=func(episode,media string) string {
  return `<script id="videoInitialData">{"ep":"`+episode+`","videoSrc":"`+media+`"}</script>`
 }
 page:="https://huangguoai.com/detail/12/"
 got,ok:=huangguoAIDetailFirstEpisode(makeScript("1","https://media.example.test/one.m3u8"),page)
 if !ok || got.Index!=1 || got.Key!="ep-1" || got.HLS!="https://media.example.test/one.m3u8" || got.URL!=page {
  t.Fatalf("valid first episode missing: %+v found=%v",got,ok)
 }
 if _,ok:=huangguoAIDetailFirstEpisode(makeScript("2","https://media.example.test/two.m3u8"),page);ok {
  t.Fatal("the second episode was incorrectly inserted as episode one")
 }
}

func TestHuangguoAIRejectsStaleCrossDramaPlaybackTasks(t *testing.T) {
 for _,bad:=range []string{
   "https://huangguoai.com/video/9919/",
   "https://huangguoai.com/video/117/ep-1/",
   "https://huangguoai.com/detail/117/",
   "https://attacker.example/video/12/ep-1/",
 } {
  if huangguoAIPageBelongsToDrama(bad,"12") {
   t.Fatalf("unrelated page passed ownership validation: %s",bad)
  }
  downloader:=&Downloader{}
  _,err:=downloader.resolveProviderMedia(context.Background(),Task{
   DramaID:providerDramaID(sourceHuangguoAI,"12"),
   Chapter:Chapter{Source:sourceHuangguoAI,PageURL:bad,VideoURL:"https://cdn.example.test/fake.m3u8"},
  })
  if err==nil || !strings.Contains(err.Error(),"已停止播放") {
   t.Fatalf("stale task did not fail closed: %q %v",bad,err)
  }
 }
 for _,good:=range []string{
  "https://huangguoai.com/detail/12/",
  "https://huangguoai.com/video/12/",
  "https://huangguoai.com/video/12/ep-2/",
  "https://thu.ediayikma.cc/video/12/ep-2/",
 } {
  if !huangguoAIPageBelongsToDrama(good,"12") {
   t.Fatalf("valid page incorrectly rejected: %s",good)
  }
 }
}
