package core

import (
  "context"
  "crypto/sha256"
  "fmt"
 "strings"
  "testing"
  "time"
)

func TestHongguoAIRealFirstThreeEpisodePageIdentity(t *testing.T) {
  ctx,cancel:=context.WithTimeout(context.Background(),110*time.Second)
  defer cancel()
  engine,err:=newNativeEngine(t.TempDir())
  if err!=nil {t.Fatal(err)}
  t.Cleanup(engine.downloads.close)

  titleNeedle:="修仙满级回现代"
  var chosen Drama
  for page:=1;page<=9;page++ {
    items,_,e:=engine.downloader.fetchHongguoWebCatalogCategory(ctx,"ai_series",page)
    if e!=nil {t.Logf("CATALOG_PAGE page=%d err=%v",page,e);continue}
    for _,v:=range items {
      if len(v.Title)>=len(titleNeedle) && strings.Contains(v.Title,titleNeedle) {
        chosen=v
        break
      }
    }
    if chosen.SourceID!="" {break}
  }
  if chosen.SourceID=="" {t.Skip("exact title not on scanned category pages")}
  title,chs,err:=engine.downloader.fetchHongguoWebChapters(ctx,chosen.SourceID)
  if err!=nil{t.Fatal(err)}
  t.Logf("SERIES_FOUND title=%q title_from_detail=%q episodes=%d",chosen.Title,title,len(chs))
  if len(chs)<3 {t.Fatalf("expected >=3 episodes, got %d",len(chs))}
  urls:=map[string]bool{}
  for i:=0;i<3;i++ {
    id:=chs[i].VideoURL
    if urls[id] {t.Fatalf("duplicate video ID on episode %d",i+1)}
    urls[id]=true
    address:="https://novelquickapp.com/player/"+chosen.SourceID+"/"+strings.TrimPrefix(id,"hongguo-cenc://")
    raw,e:=engine.downloader.fetchProviderText(ctx,address,"https://novelquickapp.com/")
    if e!=nil {t.Logf("EPISODE i=%d page_err=%v",i+1,e);continue}
    page:=routerLoaderMap(parseRouterData(raw),"player_","player_page")
    vid:=mapString(page,"vid")
    sid:=mapString(page,"series_id")
    digest:=sha256.Sum256([]byte(id))
    t.Logf("EPISODE index=%d status=html_ok id_hash=%x matching_series=%v matching_vid=%v protected=%v pc_login=%v",i+1,digest[:5],sid==chosen.SourceID,vid==strings.TrimPrefix(id,"hongguo-cenc://"),page["protectedPlayback"]==true,nestedMap(page,"req")["pcLoginRequired"])
    if sid!=chosen.SourceID || vid!=strings.TrimPrefix(id,"hongguo-cenc://") {t.Fatalf("episode %d mismatch: %v",i+1,fmt.Sprintf("%v %v",sid!="",vid!=""))}
  }
}
