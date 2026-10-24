package core

import (
  "context"
  "errors"
  "fmt"
  "net/http"
  "strings"
  "testing"
  "time"
)

// Investigates only public website/API metadata and playback-permission
// responses. Never fetches media, prints media URLs, keys or credentials.
func TestLiveHuangdouThreeReportedTitles(t *testing.T) {
  ctx,cancel:=context.WithTimeout(context.Background(),155*time.Second)
  defer cancel()
  engine,err:=newNativeEngine(t.TempDir())
  if err!=nil {t.Fatal(err)}
  t.Cleanup(engine.downloads.close)
  titles:=[]string{"琼明神女录","室友的共享女友","亂倫:夢境入侵"}
  selected:=map[string]string{}
  for _,host:=range []string{"https://tideember.cc","https://xqjurgek.top"}{
    req,err:=http.NewRequestWithContext(ctx,http.MethodGet,host+"/home",nil)
    if err!=nil {t.Fatal(err)}
    res,e:=engine.downloader.doCatalogRequestWithTimeout(req,8*time.Second)
    if e!=nil {t.Logf("H5_GET host=%s category=network_error kind=%T",host,e)}
    if e==nil {
      res.Body.Close()
      t.Logf("H5_GET host=%s code=%d",host,res.StatusCode)
    }
  }
  client:=newHuangdouAPIClient(engine.downloader)
  for _,entry:=range []struct{tab string; pages int}{{"all",2},{"mogai",2},{"search",2}} {
    for p:=1;p<=entry.pages;p++ {
      if ctx.Err()!=nil {t.Fatal(ctx.Err())}
      var out any
      err:=client.call(ctx,"/drama/rank",map[string]any{"tab":entry.tab,"page":fmt.Sprint(p)},&out)
      if err!=nil {
        var apiErr *huangdouAPIError
        if errors.As(err,&apiErr) {t.Logf("RANK tab=%s page=%d host=%s result=api_rejected code=%s",entry.tab,p,client.host,apiErr.code)}
        if !errors.As(err,&apiErr) {t.Logf("RANK tab=%s page=%d host=%s result=transport_error kind=%T",entry.tab,p,client.host,err)}
        continue
      }
      rows:=huangdouList(huangdouDataMap(out)["list"])
      if len(rows)==0 {rows=huangdouList(huangdouDataMap(out)["items"])}
      if len(rows)==0 {rows=huangdouList(huangdouDataMap(out)["data"])}
      t.Logf("RANK tab=%s page=%d host=%s result=ok rows=%d",entry.tab,p,client.host,len(rows))
      for _,r:=range rows {
        title:=mapString(r,"name","title","t")
        for _,wanted:=range titles {
          if strings.Contains(strings.ReplaceAll(title," ",""),strings.ReplaceAll(wanted," ","")) {
            id:=strings.TrimPrefix(mapString(r,"id","drama_id"),"rp_")
            if id!="" {selected[wanted]=id}
            t.Logf("MATCH from_rank tab=%s title=%q id_present=%v vip=%v",entry.tab,wanted,id!="",huangdouVIPFlag(r))
          }
        }
      }
    }
  }
  for _,wanted:=range titles {
    if selected[wanted]=="" {
      for _,arg:=range []map[string]any{
        {"keyword":wanted},
        {"key":wanted},
      } {
        var out any
        err:=client.call(ctx,"/drama/search",arg,&out)
        if err!=nil {
          var a *huangdouAPIError
          if errors.As(err,&a) {t.Logf("SEARCH title=%q code=%s",wanted,a.code)}
          if !errors.As(err,&a) {t.Logf("SEARCH title=%q error_kind=%T",wanted,err)}
          continue
        }
        rows:=huangdouList(huangdouDataMap(out)["list"])
        if len(rows)==0 {rows=huangdouList(huangdouDataMap(out)["items"])}
        if len(rows)==0 {rows=huangdouList(huangdouDataMap(out)["data"])}
        t.Logf("SEARCH title=%q rows=%d",wanted,len(rows))
        for _,r:=range rows {
          if strings.Contains(mapString(r,"name","title","t"),wanted) {
            selected[wanted]=strings.TrimPrefix(mapString(r,"id","drama_id"),"rp_")
            break
          }
        }
        if selected[wanted]!="" {break}
      }
    }
    id:=selected[wanted]
    if id=="" {t.Logf("TITLE title=%q status=not_found_in_probe",wanted);continue}
    detail,e:=engine.downloader.huangdouDetail(ctx,id)
    if e!=nil {t.Logf("DETAIL title=%q status=error kind=%T",wanted,e);continue}
    t.Logf("DETAIL title=%q matches=%v vip=%v free_episodes=%q count=%q",wanted,
      strings.Contains(mapString(detail,"name","title","t"),wanted),
      huangdouVIPFlag(detail),mapString(detail,"free_episodes"),mapString(detail,"episode_count"))
    var out any
    e=client.call(ctx,"/drama/play",map[string]any{"id":id,"seq":"1"},&out)
    if e!=nil {
      var a *huangdouAPIError
      if errors.As(e,&a) {t.Logf("PLAY title=%q result=api_rejected code=%s access_block=%v",wanted,a.code,isHuangdouAccessError(e))}
      if !errors.As(e,&a) {t.Logf("PLAY title=%q result=error kind=%T",wanted,e)}
      continue
    }
    d:=huangdouDataMap(out)
    media:=firstNonEmpty(mapString(d,"m3u8"),mapString(d,"url"),mapString(d,"play_url"),mapString(d,"playUrl"))
    t.Logf("PLAY title=%q result=ok media_present=%v preview=%v hls_key_present=%v",wanted,media!="",huangdouPreviewOnly(d,media,client.host),mapString(d,"hls_key")!="")
  }
}
