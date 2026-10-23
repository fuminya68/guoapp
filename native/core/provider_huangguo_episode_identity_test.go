package core

import (
 "strings"
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
 valid := common + `<section><a href="/video/12/ep-1/">第1集</a><a href="/video/12/ep-2/">第2集</a></section>`
 got := parseHuangguoAIEpisodes(valid,detail,"12")
 if len(got)!=2 || got[0].Key!="ep-1" || got[1].Key!="ep-2" ||
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
