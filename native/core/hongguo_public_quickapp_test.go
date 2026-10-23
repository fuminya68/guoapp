package core

import (
 "context"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "strings"
 "testing"
 "time"
)

func TestHongguoPublicPlayerEndpointAI(t *testing.T) {
 ctx,cancel:=context.WithTimeout(context.Background(),100*time.Second)
 defer cancel()
 engine,err:=newNativeEngine(t.TempDir())
 if err!=nil {t.Fatal(err)}
 t.Cleanup(engine.downloads.close)
 dramas,_,err:=engine.downloader.fetchHongguoCategoryPage(ctx,"ai-drama?page=1","AI剧")
 if err!=nil {t.Fatal(err)}
 t.Logf("AI_DRAMA_COUNT=%d",len(dramas))
 if len(dramas)==0 {t.Fatal("empty AI catalog")}
 positives:=0
 for index,drama:=range dramas {
   if index>=6 {break}
   _,chapters,err:=engine.downloader.fetchHongguoChapters(ctx,drama.SourceID)
   if err!=nil||len(chapters)==0 {t.Logf("DETAIL sample=%d err=%v",index+1,err);continue}
   videoID:=strings.TrimPrefix(chapters[0].VideoURL,"hongguo-cenc://")
   if !hongguoNumericID.MatchString(videoID) || !hongguoNumericID.MatchString(drama.SourceID){continue}
   endpoint:="https://novelquickapp.com/player/"+url.PathEscape(drama.SourceID)+"/"+url.PathEscape(videoID)
   req,err:=http.NewRequestWithContext(ctx,http.MethodGet,endpoint,nil)
   if err!=nil {t.Fatal(err)}
   req.Header.Set("User-Agent","Mozilla/5.0 (Linux; Android 16; 25053RT47C) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36")
   req.Header.Set("Referer","https://novelquickapp.com/")
   req.Header.Set("Accept","text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
   response,err:=engine.downloader.doCatalogRequestWithTimeout(req,14*time.Second)
   if err!=nil {t.Logf("PLAYER sample=%d transport_error=%v",index+1,err);continue}
   buf,readErr:=io.ReadAll(io.LimitReader(response.Body,3<<20))
   response.Body.Close()
   if readErr!=nil {t.Logf("PLAYER sample=%d read_error=%v",index+1,readErr);continue}
   page:=routerLoaderMap(parseRouterData(string(buf)),"player_","player_page")
   if len(page)==0 {
     for _,key:=range []string{"player_page","player_","player"} {
       if candidate:=routerLoaderMap(parseRouterData(string(buf)),key);len(candidate)>len(page){page=candidate}
     }
   }
   info:=nestedMap(page,"video_player_info")
   urls:=hongguoMediaAddresses(info)
   validIDs:=mapString(page,"vid")==videoID && mapString(page,"series_id")==drama.SourceID
   matched:=validIDs && len(urls)>0
   if matched {positives++}
   t.Logf("PLAYER sample=%d HTTP=%d SIZE=%d TYPE=%q ROUTER_KEYS=%d MATCHES_EPISODE=%v MEDIA_ADDRS=%d HTML_VIDEO_INFO=%v",
    index+1,response.StatusCode,len(buf),response.Header.Get("Content-Type"),len(page),validIDs,len(urls),strings.Contains(string(buf),"video_player_info"))
   if len(urls)>0 {
     parsed,parseErr:=url.Parse(urls[0])
     if parseErr==nil {t.Logf("MEDIA sample=%d HOST=%s SCHEME=%s",index+1,parsed.Host,parsed.Scheme)}
   }
 }
 if positives==0 {t.Fatal(fmt.Sprintf("No exact-episode playable media found from %d sampled AI dramas",min(6,len(dramas))))}
}
