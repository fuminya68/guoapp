package core

import (
 "bytes"
 "context"
 "encoding/json"
 "io"
 "net/http"
 "net/url"
 "strconv"
 "strings"
 "testing"
 "time"

 fhttp "github.com/bogdanfinn/fhttp"
 tlsclient "github.com/bogdanfinn/tls-client"
 "github.com/bogdanfinn/tls-client/profiles"
)

func TestHongguoAppTLSTransportFingerprintComparison(t *testing.T) {
 ctx,cancel:=context.WithTimeout(context.Background(),160*time.Second)
 defer cancel()
 engine,err:=newNativeEngine(t.TempDir())
 if err!=nil{t.Fatal(err)}
 t.Cleanup(engine.downloads.close)
 hc:=engine.downloader.hongguoClient()
 hc.mu.Lock()
 dev,iid:=hc.state.DeviceID,hc.state.InstallID
 hc.mu.Unlock()
 payload:=map[string]any{"req_scene":"ai_series","offset":0,"limit":18,
   "req_type":"only_content","need_selector_panel":false,"client_req_type":3,
   "session_id":"","filter_ids":"",
   "select_items":map[string]any{"genre":[]string{"ai_series"},"sort":[]string{"online_time"},
     "gender":[]string{},"category_dim_theme":[]string{},"category_dim_role":[]string{},
     "category_dim_epoch":[]string{},"online_time":[]string{},"creation_status":[]string{}}}
 body,err:=json.Marshal(payload)
 if err!=nil{t.Fatal(err)}
 makeSignedRequest:=func() *http.Request {
   query:=url.Values{
    "aid":{"8662"},"app_name":{"novelread"},"version_code":{"73532"},
    "version_name":{"7.3.5.32"},"manifest_version_code":{"73532"},
    "update_version_code":{"73532"},"channel":{"update_64"},
    "device_platform":{"android"},"os":{"android"},"ssmix":{"a"},
    "device_type":{"25053RT47C"},"device_brand":{"Redmi"},"language":{"zh"},
    "os_api":{"36"},"os_version":{"16"},"resolution":{"1280*2772"},
    "dpi":{"520"},"ac":{"wifi"},"device_id":{dev},"iid":{iid},
   }
   req,err:=http.NewRequestWithContext(ctx,http.MethodPost,
    hongguoAppBaseURL+"/reading/distribution/category/landpage/v/",bytes.NewReader(body))
   if err!=nil{t.Fatal(err)}
   req.Header.Set("User-Agent",hongguoAppUserAgent)
   req.Header.Set("Accept","application/json")
   req.Header.Set("X-XS-From-Web","0")
   req.Header.Set("Sdk-Version","2")
   req.Header.Set("Content-Type","application/json; charset=utf-8")
   now:=time.Now()
   query.Set("_rticket",strconv.FormatInt(now.UnixMilli(),10))
   req.URL.RawQuery=query.Encode()
   signHongguoRequest(req,body,now)
   return req
 }
 logResponse:=func(label string,status int,n int,contentType string,body []byte){
   state:="empty"
   if n>0 {
     var object map[string]any
     if json.Unmarshal(body,&object)==nil {state="json"}else{state="non-json"}
   }
   t.Logf("PROFILE=%s HTTP=%d BODY_BYTES=%d BODY_FORMAT=%s MIME=%q",label,status,n,state,contentType)
 }
 t.Run("go-stdlib",func(t *testing.T){
   req:=makeSignedRequest()
   response,err:=engine.downloader.doCatalogRequestWithTimeout(req,20*time.Second)
   if err!=nil {t.Logf("PROFILE=go-stdlib ERROR=%T: %v",err,err);return}
   defer response.Body.Close()
   body,err:=io.ReadAll(io.LimitReader(response.Body,1<<20))
   if err!=nil {t.Logf("PROFILE=go-stdlib READ_ERROR=%v",err);return}
   logResponse("go-stdlib",response.StatusCode,len(body),response.Header.Get("Content-Type"),body)
 })
 for _,tc:=range []struct{name string;profile profiles.ClientProfile}{
   {"okhttp4-android13",profiles.Okhttp4Android13},
   {"confirmed-android",profiles.ConfirmedAndroid},
   {"chromium-150",profiles.Chrome_150},
 }{
  t.Run(tc.name,func(t *testing.T){
   client,err:=tlsclient.NewHttpClient(tlsclient.NewNoopLogger(),
      tlsclient.WithClientProfile(tc.profile),
      tlsclient.WithDisableHttp3(),
      tlsclient.WithNotFollowRedirects(),
      tlsclient.WithTimeoutSeconds(20),
      tlsclient.WithCookieJar(tlsclient.NewCookieJar()))
   if err!=nil {t.Fatalf("INIT %v",err)}
   defer client.CloseIdleConnections()
   signed:=makeSignedRequest()
   req,err:=fhttp.NewRequestWithContext(ctx,signed.Method,signed.URL.String(),bytes.NewReader(body))
   if err!=nil {t.Fatal(err)}
   req.Header=make(fhttp.Header)
   for name,values:=range signed.Header {
     req.Header[name]=append([]string(nil),values...)
   }
   req.Header[fhttp.HeaderOrderKey]=[]string{"user-agent","accept","x-xs-from-web","sdk-version","content-type","x-khronos","x-gorgon","x-ss-stub","x-ss-req-ticket"}
   response,err:=client.Do(req)
   if err!=nil {t.Logf("PROFILE=%s ERROR=%T: %v",tc.name,err,err);return}
   defer response.Body.Close()
   content,err:=io.ReadAll(io.LimitReader(response.Body,1<<20))
   if err!=nil {t.Logf("PROFILE=%s READ_ERROR=%v",tc.name,err);return}
   logResponse(tc.name,response.StatusCode,len(content),strings.Join(response.Header.Values("Content-Type"),","),content)
  })
 }
}
