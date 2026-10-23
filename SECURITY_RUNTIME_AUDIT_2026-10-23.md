# 真果鑑 / zhenguojian Windows 2026-10-23 網路與動態行為安全審查（第一輪）

## 結論與可信程度

本輪**發現並修補一項可重現的播放憑證外送風險**。第一次 Windows GUI 自動行為觀察沒有記錄到衍生程序或新增常駐項目，但程式啟動後有對外 HTTPS 網路連線。這不是惡意軟體無害或不存在後門的證明，也不適合單獨當作金融／公司機密電腦的安裝放行依據。

審查分支：`security/runtime-network-audit-v1`（以 `security/mpv-windows-integrations` 為基礎，不動 `main`）。

## 已確認問題：HTTP 重新導向時自訂播放 Token 可能外送

**來源**：`native/core/network_media.go` 的 `X-Preview-Token`，搭配 Go `http.Client` 的預設重新導向流程。Go 對 Authorization/Cookie 有特殊保護，但一般自訂 Header 會複製到重導向的 HTTP Request。若原站點（或可控制重導向回應的一方）返回 `Location: https://attacker.example/...`，跨網域請求可能攜帶播放用的 Token。既有 SSRF 保護主要限制本機、內網和保留 IP，**無法阻擋所有公網重導向**。

**影響**：可導致播放階段短時有效 Token 外洩。尚未證實真實站源受攻擊或任何第三方已取得 Token；不是完整電腦控制或 Windows 密碼外洩的證據。

**修補**：
- `native/core/network_media_redirect.go` 新增 `scopedMediaPreviewClient`；若重導向後的 `scheme://host[:port]` 與原始站源不同，即移除 `X-Preview-Token`。
- `native/core/network_media.go` 在發送預覽金鑰請求時啟用防護，保留原本的 `CheckRedirect`，不繞過既有 SSRF 防護。
- `native/core/network_media_redirect_test.go` 用不發出真實網路請求的模擬 302 測試相異主機、子網域、協定、埠與同源保留，並確認導向私有 IP 仍被拒絕。
- 驗證：GitHub Actions `Windows behavioral security and network attack audit` #39962957374 的 `Adversarial redirect/token tests` **SUCCESS**。

## Windows 動態行為取樣紀錄

**來源二進位**：既有通過 Windows CI 的內層 ZIP，SHA-256:
`5838900BCD7540F671C0E8B5314762DAA0DB6DA1076C8FDA764BB02EC0CBF8C9`
建置：https://github.com/fuminya68/guoapp/actions/runs/39956664273

**取樣執行**：https://github.com/fuminya68/guoapp/actions/runs/39962957374
啟動於 `2026-10-23T13:39:21Z`，Windows GitHub Actions runner，監測 16 秒，紀錄存放 `windows-runtime-security-observations` Artifact。

- 主程序：`zhenguojian.exe`；取樣期間**未觀察到衍生子程序**。
- TCP：8 條端點狀態記錄（含 SynSent 與 Established、相同連線重複狀態）；**4 個不同遠端公網 IP，分布於三組網段**，遠端連接埠均為 443：
  - `98.96.242.53`
  - `155.102.176.82` / `155.102.176.86`
  - `113.96.147.123`
- UDP：取樣期間未觀察到。
- Windows Run/RunOnce 登錄值、Startup 資料夾、已啟用排程工作：**沒有新增記錄**。
- 監測工具回報 `samplingErrors: []`。

**重要限制**：
- 這是**短時間 TCP/程序輪詢**，可能漏掉瞬間執行與瞬間連線，無法檢查 TLS 內的傳輸內容、文件存取、瀏覽器 Cookie/錢包存取，亦非 Sysmon/Procmon 完整事件軌跡。
- 尚未取得 DNS/SNI/HTTP Host 與上述 IP 的可靠關聯，不應臆測其站點、業者或將其直接認定為外傳。
- Windows Runner 是 GitHub 的暫時環境，不能取代在個人電腦的隔離 VM、不同功能及真實來源路徑觀察。
- 上述取樣採用**舊版已驗證 ZIP**；含本輪 Token 修補的新二進位需等待新 Windows CI 產物及其執行稽核，不能混稱來源相同。

## 仍需追加的審查

1. **全網路路徑**：`network_browser.go`、`network_cdn.go`、`provider_web.go`、站源目錄、封面與搜尋請求尚未得到逐路徑 DNS/redirect/SSRF 驗證。部分傳輸支援使用者開啟不安全 TLS 設定；必須保留預設安全與清楚風險提示。
2. **動態網路歸因**：在隔離 VM 收集 DNS 和 TLS SNI、比對請求的遠端服務/用途，並觀察登入、搜尋、播放、下載各情境及長時間執行，不應記錄真實使用者 Token。
3. **敏感檔案存取**：需要 EDR/Sysmon/Procmon 細粒度檢查是否不必要讀取瀏覽器密碼庫、SSH Key、錢包、公司文件。現有腳本無法檢查。
4. **故障與滲透測試**：HLS/JSON/XML/解碼器畸形輸入、跨站點 Token 驗證、LAN 配對跨 session、持續衍生程序觀察，仍可加深。
5. **現有下載／媒體庫測試**：在前次基線仍有 7 項失敗，已記錄在 `WINDOWS_SECURITY_ACCEPTANCE.md`；本輪未修復。

## 安裝放行建議

在新安全分支 CI 全部通過前請勿將其標記為正式安全版。即使通過，仍優先在 Windows Sandbox/隔離 VM 以普通使用者權限試用，停用 LAN 裝置互聯，並保持 Windows 防毒與安全更新。工作機、錢包私鑰或 API Key 所在環境應維持額外隔離。
