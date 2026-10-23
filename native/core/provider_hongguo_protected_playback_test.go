package core

import (
 "strings"
 "testing"
)

func TestHongguoWebProtectedPlaybackStatus(t *testing.T) {
 protected := hongguoUnavailableWebPlayback(map[string]any{
   "protectedPlayback": true,
   "publicVideoSeo": map[string]any{"available": true},
 })
 if !strings.Contains(protected.Error(),"受保護的瀏覽器播放") {
  t.Fatalf("failed to distinguish browser-protected playback: %v",protected)
 }
 if strings.Contains(protected.Error(),"登入") || strings.Contains(protected.Error(),"付費") {
  t.Fatal("protected browser playback should not be mislabeled as login or paywall")
 }
 missing := hongguoUnavailableWebPlayback(map[string]any{})
 if !strings.Contains(missing.Error(),"未提供公開播放地址") {
  t.Fatalf("incorrect generic media error: %v",missing)
 }
}
