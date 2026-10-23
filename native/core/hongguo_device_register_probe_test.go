package core

import (
 "bytes"
 "compress/gzip"
 "context"
 "crypto/aes"
 "crypto/cipher"
 "crypto/md5"
 "crypto/rand"
 "crypto/sha512"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/http/cookiejar"
 "net/url"
 "regexp"
 "strings"
 "testing"
 "time"
)

func TestHongguoAnonymousDeviceRegistrationCompatibility(t *testing.T) {
 ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second)
 defer cancel()
 engine,err:=newNativeEngine(t.TempDir())
 if err!=nil{t.Fatal(err)}
 t.Cleanup(engine.downloads.close)
 hexSalt:="4dd4c2e6b83162090e52b3c7a6733ba41cb2462b829ab58a196b39db57177524f49baf7f08e8d68d26a72e37c1a95a2f1f05a51892aef2949732b62a38aadd58"
 salt,err:=hex.DecodeString(hexSalt)
 if err!=nil{t.Fatal(err)}
 randomHex:=func(n int) string {
   v:=make([]byte,n)
   if _,err:=rand.Read(v);err!=nil{t.Fatal(err)}
   return hex.EncodeToString(v)
 }
 now:=time.Now().UnixMilli()
 header:=map[string]any{
   "display_name":"红果免费短剧","aid":8662,"channel":"69310258a","package":"com.phoenix.read",
   "version_code":73932,"update_version_code":73932,"manifest_version_code":73932,
   "app_version":"7.3.9.32","app_version_minor":"7.3.9.32",
   "sdk_version":"3.7.0-rc.34-fanqie-xiaoshuo","sdk_target_version":29,
   "git_hash":"3a88f4e","release_build":"b5e187f",
   "os":"Android","os_version":"12","os_api":32,
   "device_model":"AOSP on Intel Platform","device_brand":"SAMSUNG","device_manufacturer":"Intel",
   "cpu_abi":"arm64-v8a","density_dpi":360,"display_density":"mdpi","resolution":"1440x2560",
   "language":"zh","timezone":8,"access":"wifi","carrier":"CHINA MOBILE","mcc_mnc":"46000",
   "rom":"eng.root.20231116.160340","rom_version":"W528JS release-keys",
   "cdid":randomHex(16),"openudid":randomHex(8),"clientudid":randomHex(16),
   "sig_hash":"56a962410c494bbaf0b58dba20cae56f",
   "ipv6_list":[]string{},"region":"CN","tz_name":"Asia/Shanghai","tz_offset":28800,
   "sim_region":"cn","sim_serial_number":[]string{},"oaid_may_support":false,
   "req_id":randomHex(16),"device_platform":"android",
   "custom":map[string]any{"account_region_src":"fallback","host_bit":64,"account_region":"cn","dragon_device_type":"pad"},
   "pre_installed_channel":"","apk_first_install_time":now,
   "is_system_app":0,"sdk_flavor":"china","guest_mode":0,"not_request_sender":0,
 }
 payload,err:=json.Marshal(map[string]any{"magic_tag":"ss_app_log","header":header,"_gen_time":now})
 if err!=nil{t.Fatal(err)}
 var zipped bytes.Buffer
 gz:=gzip.NewWriter(&zipped)
 if _,err:=gz.Write(payload);err!=nil{t.Fatal(err)}
 if err:=gz.Close();err!=nil{t.Fatal(err)}
 digest:=sha512.Sum512(zipped.Bytes())
 plain:=append(digest[:],zipped.Bytes()...)
 padding:=aes.BlockSize-len(plain)%aes.BlockSize
 for i:=0;i<padding;i++{plain=append(plain,byte(padding))}
 rnd:=make([]byte,32)
 if _,err:=rand.Read(rnd);err!=nil{t.Fatal(err)}
 hashedRnd:=sha512.Sum512(rnd)
 material:=sha512.Sum512(append(hashedRnd[:],salt...))
 block,err:=aes.NewCipher(material[:16])
 if err!=nil{t.Fatal(err)}
 encrypted:=make([]byte,len(plain))
 cipher.NewCBCEncrypter(block,material[16:32]).CryptBlocks(encrypted,plain)
 binaryBody:=append([]byte{0x74,0x63,0x05,0x10,0,0},rnd...)
 binaryBody=append(binaryBody,encrypted...)
 address:="https://log5-applog-lf.fqnovel.com/service/2/device_register/?tt_data=a&cronet_version=8d40f833_2026-03-03&ttnet_version=4.2.243.31-douyin&use_store_region_cookie=1"
 req,err:=http.NewRequestWithContext(ctx,http.MethodPost,address,bytes.NewReader(binaryBody))
 if err!=nil{t.Fatal(err)}
 req.Header.Set("User-Agent","com.phoenix.read/73932 (Linux; U; Android 12; zh_CN; AOSP on Intel Platform)")
 req.Header.Set("Content-Type","application/octet-stream;tt-data=a")
 req.Header.Set("log-encode-type","gzip")
 req.Header.Set("x-ss-dp","8662")
 req.Header.Set("x-ss-req-ticket",fmt.Sprint(now))
 req.Header.Set("x-vc-bdturing-sdk-version","4.0.3.cn")
 req.Header.Set("lc","101")
 req.Header.Set("sdk-version","2")
 req.Header.Set("passport-sdk-version","5051452")
 req.Header.Set("x-neptune","-8|50:51:59:30:40:47:49:39")
 md5sum:=md5.Sum(binaryBody)
 req.Header.Set("x-ss-stub",strings.ToUpper(hex.EncodeToString(md5sum[:])))
 response,err:=engine.downloader.doCatalogRequestWithTimeout(req,20*time.Second)
 if err!=nil {t.Fatalf("registration request: %v",err)}
 defer response.Body.Close()
 content,err:=io.ReadAll(io.LimitReader(response.Body,128<<10))
 if err!=nil{t.Fatal(err)}
 var parsed map[string]any
 err=json.Unmarshal(content,&parsed)
 valid:=false
 ids:=regexp.MustCompile(`^[0-9]{10,32}$`)
 if err==nil {
  device:=fmt.Sprint(parsed["device_id_str"])
  install:=fmt.Sprint(parsed["install_id_str"])
  if device=="<nil>" {device=fmt.Sprint(parsed["device_id"])}
  if install=="<nil>" {install=fmt.Sprint(parsed["install_id"])}
  valid=ids.MatchString(device)&&ids.MatchString(install)
 }
 t.Logf("REGISTER HTTP=%d RESPONSE_BYTES=%d JSON_VALID=%v DEVICE_ID_ISSUED=%v SET_COOKIE_COUNT=%d",
  response.StatusCode,len(content),err==nil,valid,len(response.Header.Values("Set-Cookie")))
 if !valid {t.Fatal("anonymous registration was not accepted; no credentials retained")}
 device:=fmt.Sprint(parsed["device_id_str"])
 install:=fmt.Sprint(parsed["install_id_str"])
 if device=="<nil>" {device=fmt.Sprint(parsed["device_id"])}
 if install=="<nil>" {install=fmt.Sprint(parsed["install_id"])}
 hc:=engine.downloader.hongguoClient()
 hc.mu.Lock()
 hc.state.DeviceID=device
 hc.state.InstallID=install
 hc.mu.Unlock()
 apiPayload:=map[string]any{
   "req_scene":"ai_series","offset":0,"limit":18,"req_type":"only_content",
   "need_selector_panel":false,"client_req_type":3,"session_id":"","filter_ids":"",
   "select_items":map[string]any{"genre":[]string{"ai_series"},
   "sort":[]string{"online_time"},"gender":[]string{},
   "category_dim_theme":[]string{},"category_dim_role":[]string{},
   "category_dim_epoch":[]string{},"online_time":[]string{},"creation_status":[]string{}},
 }
 result,reqErr:=engine.downloader.hongguoAppRequest(ctx,http.MethodPost,"/reading/distribution/category/landpage/v/",nil,apiPayload)
 t.Logf("API_REGISTERED_WITHOUT_COOKIES JSON=%v ERR=%v",len(result)>0,reqErr)
 jar,err:=cookiejar.New(nil)
 if err!=nil{t.Fatal(err)}
 accepted:=map[string]bool{"store-region":true,"store-region-src":true,"install_id":true,"ttreq":true}
 var copied []*http.Cookie
 for _,cookie:=range response.Cookies() {
  if accepted[cookie.Name] && cookie.Value!=""{
    copied=append(copied,&http.Cookie{Name:cookie.Name,Value:cookie.Value,Path:"/"})
  }
 }
 for _,host:=range []string{"api5-normal-sinfonlineb.fqnovel.com","api5-normal-sinfonlinea.fqnovel.com","api5-normal-sinfonlinec.fqnovel.com"}{
  target:=&url.URL{Scheme:"https",Host:host}
  jar.SetCookies(target,copied)
 }
 engine.downloader.client.Jar=jar
 t.Logf("REGISTERED_COOKIE_COUNT=%d",len(copied))
 for _,host:=range []string{"api5-normal-sinfonlineb.fqnovel.com","api5-normal-sinfonlinea.fqnovel.com","api5-normal-sinfonlinec.fqnovel.com"}{
   hc.mu.Lock()
   hc.baseURL="https://"+host
   hc.mu.Unlock()
   data,e:=engine.downloader.hongguoAppRequest(ctx,http.MethodPost,"/reading/distribution/category/landpage/v/",nil,apiPayload)
   t.Logf("API_REGISTERED_WITH_COOKIES HOST=%s JSON=%v ERR=%v",host,len(data)>0,e)
 }
 dramas,_,err:=engine.downloader.fetchHongguoCategoryPage(ctx,"ai-drama?page=1","AI劇")
 if err!=nil || len(dramas)==0 {t.Logf("MEDIA_DIAGNOSTIC_WEB_CATALOG err=%v count=%d",err,len(dramas));return}
 _,chapters,err:=engine.downloader.fetchHongguoWebChapters(ctx,dramas[0].SourceID)
 if err!=nil || len(chapters)==0 {t.Logf("MEDIA_DIAGNOSTIC_WEB_EPISODES err=%v count=%d",err,len(chapters));return}
 videoID:=strings.TrimPrefix(chapters[0].VideoURL,"hongguo-cenc://")
 for _,host:=range []string{"api5-normal-sinfonlineb.fqnovel.com","api5-normal-sinfonlinea.fqnovel.com","api5-normal-sinfonlinec.fqnovel.com"} {
   hc.mu.Lock()
   hc.baseURL="https://"+host
   hc.mu.Unlock()
   media,e:=engine.downloader.resolveHongguoAppMedia(ctx,videoID)
   t.Logf("MEDIA_REGISTERED_WITH_COOKIES HOST=%s HAS_URL=%v ERR=%v",host,media.URL!="",e)
 }
}

