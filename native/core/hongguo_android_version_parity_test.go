package core

import (
  "bytes"
  "context"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "net/url"
  "strconv"
  "strings"
  "testing"
  "time"
)

func TestHongguoAndroidVersionRequestParity(t *testing.T) {
  ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
  defer cancel()
  engine, err := newNativeEngine(t.TempDir())
  if err != nil { t.Fatal(err) }
  t.Cleanup(engine.downloads.close)
  client := engine.downloader.hongguoClient()
  client.mu.Lock()
  id, iid := client.state.DeviceID, client.state.InstallID
  client.mu.Unlock()
  payload := map[string]any{
    "req_scene":"ai_series", "offset":0, "limit":18,
    "req_type":"only_content", "need_selector_panel":false, "client_req_type":3,
    "session_id":"", "filter_ids":"",
    "select_items":map[string]any{
      "genre":[]string{"ai_series"}, "sort":[]string{"online_time"},
      "gender":[]string{}, "category_dim_theme":[]string{},
      "category_dim_role":[]string{}, "category_dim_epoch":[]string{},
      "online_time":[]string{}, "creation_status":[]string{},
    },
  }
  body, err := json.Marshal(payload)
  if err != nil { t.Fatal(err) }
  profiles := []struct{name, code, version string}{
    {"source_baseline_73532","73532","7.3.5.32"},
    {"xiaomi_store_73632","73632","7.3.6.32"},
    {"later_listing_73932","73932","7.3.9.32"},
  }
  for _, version := range profiles {
    t.Run(version.name,func(t *testing.T) {
      query:=url.Values{
        "aid":{"8662"}, "app_name":{"novelread"},
        "version_code":{version.code}, "version_name":{version.version},
        "manifest_version_code":{version.code},
        "update_version_code":{version.code}, "channel":{"update_64"},
        "device_platform":{"android"}, "os":{"android"}, "ssmix":{"a"},
        "device_type":{"25053RT47C"}, "device_brand":{"Redmi"},
        "language":{"zh"}, "os_api":{"36"}, "os_version":{"16"},
        "resolution":{"1280*2772"}, "dpi":{"520"}, "ac":{"wifi"},
        "device_id":{id}, "iid":{iid},
      }
      request, err:=http.NewRequestWithContext(ctx,http.MethodPost,hongguoAppBaseURL+"/reading/distribution/category/landpage/v/",bytes.NewReader(body))
      if err!=nil {t.Fatal(err)}
      request.Header.Set("User-Agent",strings.Replace(hongguoAppUserAgent,"/73532","/"+version.code,1))
      request.Header.Set("Accept","application/json")
      request.Header.Set("X-XS-From-Web","0")
      request.Header.Set("Sdk-Version","2")
      request.Header.Set("Content-Type","application/json; charset=utf-8")
      response,err:=engine.downloader.doPreparedCatalogRequest(request,20*time.Second,func(prepared *http.Request){
        now:=time.Now()
        query.Set("_rticket",strconv.FormatInt(now.UnixMilli(),10))
        prepared.URL.RawQuery=query.Encode()
        signHongguoRequest(prepared,body,now)
      })
      if err!=nil {t.Logf("RESULT version=%s network_error=%v",version.code,err);return}
      defer response.Body.Close()
      buf,err:=io.ReadAll(io.LimitReader(response.Body,providerMaxBodyBytes+1))
      if err!=nil {t.Logf("RESULT version=%s http=%d read_error=%v",version.code,response.StatusCode,err);return}
      result:="empty"
      if len(buf)>0 {
        var obj map[string]any
        if json.Unmarshal(buf,&obj)==nil {
          result="json"
        }else{result="non-json"}
      }
      t.Logf("RESULT version=%s http=%d body_bytes=%d response=%s content_type=%q",version.code,response.StatusCode,len(buf),result,response.Header.Get("Content-Type"))
    })
  }
  fmt.Println("NOTE: This only compares signed Android-like request metadata over Go TLS; it does not run the vendor's Android binary.")
}
