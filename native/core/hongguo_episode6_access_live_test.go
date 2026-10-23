package core

import (
  "context"
  "crypto/sha256"
  "encoding/hex"
  "sort"
  "strings"
  "testing"
  "time"
)

// Privacy: logs contain metadata booleans, IDs hashed, never page bodies,
// auth cookies, media URLs, response headers or tokens.
func TestHongguoFirstTenEpisodesBrowserAvailability(t *testing.T) {
  ctx,cancel:=context.WithTimeout(context.Background(),150*time.Second)
  defer cancel()
  engine,err:=newNativeEngine(t.TempDir())
  if err!=nil {t.Fatal(err)}
  t.Cleanup(engine.downloads.close)
  var chosen Drama
  for p:=1;p<=9;p++ {
    dramas,_,err:=engine.downloader.fetchHongguoWebCatalogCategory(ctx,"ai_series",p)
    if err!=nil {t.Logf("CATALOG_FAILURE page=%d err=%v",p,err);continue}
    for _,drama:=range dramas {
      if strings.Contains(drama.Title,"修仙满级回现代") {
        chosen=drama
        break
      }
    }
    if chosen.SourceID!="" {break}
  }
  if chosen.SourceID=="" {t.Fatal("target series not found in catalog")}
  _,episodes,err:=engine.downloader.fetchHongguoWebChapters(ctx,chosen.SourceID)
  if err!=nil {t.Fatal(err)}
  if len(episodes)<10 {t.Fatalf("only %d episodes",len(episodes))}
  t.Logf("SERIES title_match=%v episode_count=%d", strings.Contains(chosen.Title,"修仙满级回现代"),len(episodes))

  seen:=map[string]bool{}
  for _,i:=range []int{0,3,4,5,6,7,9,14} {
    vid:=strings.TrimPrefix(episodes[i].VideoURL,"hongguo-cenc://")
    if seen[vid] {t.Fatalf("duplicated video id in episode %d",i+1)}
    seen[vid]=true
    digest:=sha256.Sum256([]byte(vid))
    t.Logf("EPISODE episode=%d id_hash=%s",i+1,hex.EncodeToString(digest[:5]))
    for _,host:=range []string{"novelquickapp.com","hongguoduanju.com"} {
      endpoint:="https://"+host+"/player/"+chosen.SourceID+"/"+vid
      body,e:=engine.downloader.fetchProviderText(ctx,endpoint,"https://"+host+"/")
      if e!=nil {t.Logf("PLAYER episode=%d host=%s fetch_error=%v",i+1,host,e);continue}
      data:=parseRouterData(body)
      page:=routerLoaderMap(data,"player_","player_page")
      if len(page)==0 {t.Logf("PLAYER episode=%d host=%s no_player_data bytes=%d",i+1,host,len(body));continue}
      r:=nestedMap(page,"req")
      seo:=nestedMap(page,"publicVideoSeo")
      keys:=make([]string,0,len(page))
      for k:=range page {keys=append(keys,k)}
      sort.Strings(keys)
      t.Logf("PLAYER episode=%d host=%s bytes=%d match_series=%v match_video=%v protected=%v browser_allowed=%v login=%v pc_auth=%v seo_available=%v video_list=%d flags_keys=%q",
        i+1,host,len(body),
        mapString(page,"series_id")==chosen.SourceID,
        mapString(page,"vid")==vid,
        page["protectedPlayback"],r["encryptedBrowserAllowed"],r["pcLoginRequired"],r["pcAuthenticatedPlayback"],seo["available"],len(anyList(page["videoList"])),keys)
    }
  }
}
