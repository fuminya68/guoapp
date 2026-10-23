package core

import (
 "context"
 "crypto/sha256"
 "fmt"
 "strings"
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
