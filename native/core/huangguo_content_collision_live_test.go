package core

import (
 "context"
 "encoding/json"
 "crypto/sha256"
 "fmt"
 "strings"
 "net/url"
 "regexp"
 "testing"
 "time"
)

func TestHuangguoLiveCatalogDetailIdentityDiagnostics(t *testing.T) {
 ctx,cancel:=context.WithTimeout(context.Background(),135*time.Second)
 defer cancel()
 engine,err:=newNativeEngine(t.TempDir())
 if err!=nil {t.Fatal(err)}
 t.Cleanup(engine.downloads.close)
 routes:=[]string{"", "ai-mogai/", "ai-duanju/"}
 known:=[]string{"神瞳觉醒","神瞳覺醒","我的双胞胎妈妈","我的雙胞胎媽媽","高三爱情故事","高三愛情故事","双规","雙規"}
 seenIDs:=map[string]string{}
 count:=0
 for _,route:=range routes {
   address:="https://huangguoai.com/"+route
   body,err:=engine.downloader.fetchProviderText(ctx,address,"https://huangguoai.com/")
   if err!=nil {t.Logf("CATALOG path=%q error=%v",route,err);continue}
   dramas:=parseHuangguoAIDramaCards(body,address,"diagnostic")
   t.Logf("CATALOG path=%q cards=%d",route,len(dramas))
   for _,drama:=range dramas {
     for _,needle:=range known {
       if strings.Contains(drama.Title,needle) {
         t.Logf("KNOWN_ITEM title=%q id=%q",drama.Title,drama.SourceID)
       }
     }
     if _,ok:=seenIDs[drama.SourceID];!ok {seenIDs[drama.SourceID]=drama.Title}
   }
   // Compare a few independent titles, without downloading media or posters.
   for _,drama:=range dramas {
     if count>=7 {break}
     if drama.SourceID=="" {continue}
     count++
     detailURL:="https://huangguoai.com/detail/"+drama.SourceID+"/"
     detailHTML,err:=engine.downloader.fetchProviderText(ctx,detailURL,address)
     if err!=nil {t.Logf("DETAIL title=%q id=%q error=%v",drama.Title,drama.SourceID,err);continue}
     parsed:=huangguoDetailMetadata(detailHTML,detailURL,sourceHuangguoAI,drama.SourceID)
     eps:=parseHuangguoAIEpisodes(detailHTML,detailURL,drama.SourceID)
     var mediaHash string
     if len(eps)>0 {
       episodeHTML,e:=engine.downloader.fetchProviderText(ctx,eps[0].URL,detailURL)
       if e==nil {
         address:=parseAIVideoURL(episodeHTML,eps[0].URL)
         if address!="" {
           digest:=sha256.Sum256([]byte(address))
           mediaHash=fmt.Sprintf("%x",digest[:6])
         }
       }
     } else {
       address:=parseAIVideoURL(detailHTML,detailURL)
       if address!="" {
         digest:=sha256.Sum256([]byte(address))
         mediaHash=fmt.Sprintf("%x",digest[:6])
       }
     }
     t.Logf("DETAIL requested=%q id=%q resolvedTitle=%q episodeCount=%d mediaHash=%q",drama.Title,drama.SourceID,parsed.DisplayTitle(),len(eps),mediaHash)
   }
 }
 if len(seenIDs)==0 {t.Fatal("no Huangguo text catalog reachable")}
 t.Logf("UNIQUE_DRAMA_IDS=%d",len(seenIDs))
}

func TestHuangguoKnownTitlesCollisionSignatures(t *testing.T) {
 ctx,cancel:=context.WithTimeout(context.Background(),90*time.Second)
 defer cancel()
 engine,err:=newNativeEngine(t.TempDir())
 if err!=nil {t.Fatal(err)}
 t.Cleanup(engine.downloads.close)
 for _,item:=range []struct{id,title string}{
  {"12","神瞳覺醒 第一季"},
  {"117","高三愛情故事"},
  {"417","AI魔改 雙規"},
  {"5962","不同首頁作品對照"},
 }{
  detail:="https://huangguoai.com/detail/"+item.id+"/"
  raw,err:=engine.downloader.fetchProviderText(ctx,detail,"https://huangguoai.com/")
  if err!=nil {t.Logf("TITLE_ID=%s title=%q DETAIL_ERROR=%v",item.id,item.title,err);continue}
  episodes:=parseHuangguoAIEpisodes(raw,detail,item.id)
  if len(episodes)==0 {t.Logf("TITLE_ID=%s title=%q NO_EPISODES detail_media=%v",item.id,item.title,parseAIVideoURL(raw,detail)!="");continue}
  for eindex,episode:=range episodes {
   if eindex>=2 {break}
   rawEpisode,err:=engine.downloader.fetchProviderText(ctx,episode.URL,detail)
   if err!=nil{t.Logf("TITLE_ID=%s EP=%d ERR=%v",item.id,eindex+1,err);continue}
   media:=parseAIVideoURL(rawEpisode,episode.URL)
   initial:=parseAIVideoInitialData(rawEpisode,episode.URL)
   category:="generic-regex"
   if initial!="" && media==initial {category="videoInitialData"}
   if m:=reDataPlaySrc.FindStringSubmatch(rawEpisode);len(m)>1 && normalizeProviderMediaURL(episode.URL,m[1])==media {category="data-play-src"}
   hashed:=""
   targetHost:=""
   if media!="" {digest:=sha256.Sum256([]byte(media));hashed=fmt.Sprintf("%x",digest[:6]);if parsed,e:=url.Parse(media);e==nil{targetHost=parsed.Hostname()}}
   t.Logf("TITLE_ID=%s title=%q EP=%d PAGE=%q MEDIA_METHOD=%s MEDIA_HOST=%q MEDIA_FINGERPRINT=%s HAS_INITIAL=%v",item.id,item.title,eindex+1,episode.Key,category,targetHost,hashed,strings.Contains(rawEpisode,"videoInitialData"))
  }
 }
}

func TestHuangguoDetailVideoAnchorOwnership(t *testing.T) {
 ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second)
 defer cancel()
 engine,err:=newNativeEngine(t.TempDir())
 if err!=nil{t.Fatal(err)}
 t.Cleanup(engine.downloads.close)
 classExpr:=regexp.MustCompile(`(?is)class=["']([^"']+)["']`)
 for _,id:=range []string{"12","117","417"} {
  page:="https://huangguoai.com/detail/"+id+"/"
  raw,err:=engine.downloader.fetchProviderText(ctx,page,"https://huangguoai.com/")
  if err!=nil{t.Fatalf("detail %s: %v",id,err)}
  links:=reAIEpisodeLink.FindAllStringSubmatchIndex(raw,-1)
  matches:=0
  for _,match:=range links {
    href:=raw[match[2]:match[3]]
    if aiEpisodeKey(href,id)=="1" || strings.HasPrefix(href,"/video/"+id+"/") {matches++}
  }
  fromDetail:=parseAIVideoURL(raw,page)
  summary:=""
  if fromDetail!="" {sum:=sha256.Sum256([]byte(fromDetail)); summary=fmt.Sprintf("%x",sum[:6])}
  t.Logf("SOURCE_VALIDITY id=%s matching_owned_links=%d detail_has_media=%v detail_media_fingerprint=%s initial_data=%v",id,matches,fromDetail!="",summary,strings.Contains(raw,"videoInitialData"))
  initialRe:=regexp.MustCompile(`(?is)<script\b[^>]*id=["']videoInitialData["'][^>]*>(.*?)</script>`)
  found:=initialRe.FindStringSubmatch(raw)
  if len(found)>1 {
   var initial map[string]any
   if json.Unmarshal([]byte(found[1]),&initial)==nil {
    tracks,_:=initial["epPlaySrcs"].(map[string]any)
    t.Logf("DETAIL_EPISODE_META id=%s declared_ep=%q declared_episode=%q direct_src=%v direct_url=%v direct_play=%v mapped_episodes=%d",id,mapString(initial,"ep"),mapString(initial,"episode"),mapString(initial,"videoSrc")!="",mapString(initial,"videoUrl")!="",mapString(initial,"playUrl")!="",len(tracks))
   }
  }
  t.Logf("DETAIL_PAGE id=%s all_video_anchors=%d",id,len(links))
  for index,m:=range links {
   if index>=12{break}
   href:=raw[m[2]:m[3]]
   block:=raw[m[0]:m[1]]
   start:=m[0]-420
   if start<0{start=0}
   classes:=classExpr.FindAllStringSubmatch(raw[start:m[0]],-1)
   parent:=""
   if len(classes)>0{parent=classes[len(classes)-1][1]}
   title:=cleanText(block)
   if len(title)>55{title=title[:55]}
   t.Logf("ANCHOR id=%s index=%d href=%q title=%q class=%q parent=%q",id,index+1,href,title,extractAttr(block,"class"),parent)
  }
 }
}
