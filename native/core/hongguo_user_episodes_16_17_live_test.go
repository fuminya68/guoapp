package core

import (
 "context"
 "crypto/sha256"
 "fmt"
 "io"
 "net/http"
 "strings"
 "testing"
 "time"
)

func TestHongguoUserEpisode16And17SiteParity(t *testing.T) {
 const seriesID="7697209252699442201"
 expected:=map[int]string{16:"7697250072865541144",17:"7697249981073198104"}
 ctx,cancel:=context.WithTimeout(context.Background(),110*time.Second)
 defer cancel()
 engine,err:=newNativeEngine(t.TempDir())
 if err!=nil{t.Fatal(err)}
 t.Cleanup(engine.downloads.close)
 title,chapters,err:=engine.downloader.fetchHongguoWebChapters(ctx,seriesID)
 if err!=nil{t.Fatalf("catalog: %v",err)}
 t.Logf("CATALOG chapters=%d title=%q",len(chapters),title)
 for index,vid:=range expected {
   if len(chapters)<index{t.Errorf("episode %d missing from catalog",index);continue}
   real:=strings.TrimPrefix(chapters[index-1].VideoURL,"hongguo-cenc://")
   t.Logf("CATALOG_EPISODE number=%d id_match_user=%v",index,real==vid)
   if real!=vid {t.Errorf("episode %d video ID does not match user URL",index)}
 }
 for index,vid:=range expected {
  for _,host:=range []string{"hongguoduanju.com","novelquickapp.com"}{
   address:="https://"+host+"/player/"+seriesID+"/"+vid
   req,err:=http.NewRequestWithContext(ctx,"GET",address,nil)
   if err!=nil{t.Fatal(err)}
   req.Header.Set("User-Agent","Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0.0.0 Safari/537.36")
   req.Header.Set("Accept","text/html,application/xhtml+xml")
   resp,e:=engine.downloader.doCatalogRequestWithTimeout(req,14*time.Second)
   if e!=nil{t.Logf("FETCH episode=%d host=%s error=%v",index,host,e);continue}
   body,_:=io.ReadAll(io.LimitReader(resp.Body,1<<20))
   resp.Body.Close()
   result:=routerLoaderMap(parseRouterData(string(body)),"player_","player_page")
   digest:=sha256.Sum256(body)
   t.Logf("FETCH episode=%d host=%s status=%d final_host=%s bytes=%d digest_short=%x match_series=%v match_vid=%v protected=%v pc_login=%v",
    index,host,resp.StatusCode,resp.Request.URL.Host,len(body),digest[:5],
    mapString(result,"series_id")==seriesID,mapString(result,"vid")==vid,
    result["protectedPlayback"],nestedMap(result,"req")["pcLoginRequired"])
   if resp.StatusCode==http.StatusOK && len(result)>0 {
     returned:=mapString(result,"vid")
     targetIndex:=0
     for k,e:=range chapters {
       if strings.TrimPrefix(e.VideoURL,"hongguo-cenc://")==returned {
         targetIndex=k+1
         break
       }
     }
     req:=nestedMap(result,"req")
     t.Logf("PAGE_IDENTITY requested=%d host=%s actual_catalog_episode=%d vid_empty=%v request_fields_series=%v request_fields_vid=%v",index,host,targetIndex,returned=="",mapString(req,"series_id")==seriesID,mapString(req,"vid")==vid)
   }
  }
 }
 fmt.Println("DIAGNOSTIC_COMPLETED")
}
