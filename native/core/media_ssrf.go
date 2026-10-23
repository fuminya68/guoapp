package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Media sources and HLS playlists are untrusted. A URL-only DNS check is not
// sufficient: the server could return a different address when the socket dials.
// This transport connects to the IP that was checked, preserving Host and TLS
// server-name verification for the original URL.
func mediaSSRFAllowedURL(address *url.URL) error {
	if address == nil || (address.Scheme != "http" && address.Scheme != "https") ||
		address.Hostname() == "" || address.User != nil ||
		address.Fragment != "" {
		return errors.New("媒体 URL 不是允许的 HTTP(S) 地址")
	}
	host := strings.ToLower(strings.TrimSuffix(address.Hostname(), "."))
	if host == "" || !strings.Contains(host, ".") && net.ParseIP(host) == nil ||
		host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".invalid") {
		return errors.New("媒体 URL 指向本地或内部主机")
	}
	if strings.ContainsAny(host, "%\\ \t\r\n") {
		return errors.New("媒体 URL 的主机名无效")
	}
	if port := address.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return errors.New("媒体 URL 端口无效")
		}
	}
	if literal, err := netip.ParseAddr(host); err == nil {
		if err := validateCDNAddresses([]string{literal.String()}); err != nil {
			return fmt.Errorf("媒体 URL 指向非公网 IP: %w", err)
		}
	}
	return nil
}

func mediaSSRFValidateAddress(address string) error {
	return validateCDNAddresses([]string{address})
}

func mediaSSRFDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if port == "" {
		return nil, errors.New("媒体端口为空")
	}
	var destinations []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		destinations = []netip.Addr{ip}
	} else {
		// Resolve all answers, fail closed if any maps into a restricted range,
		// then dial the checked answer itself. There is no second DNS lookup.
		destinations, err = net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("媒体域名解析失败: %w", err)
		}
	}
	if len(destinations) == 0 {
		return nil, errors.New("媒体域名没有可用地址")
	}
	for _, ip := range destinations {
		if err := mediaSSRFValidateAddress(ip.String()); err != nil {
			return nil, fmt.Errorf("拒绝媒体连接到非公网地址: %w", err)
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range destinations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func newMediaSSRFTransport() *http.Transport {
	return &http.Transport{
		Proxy: nil, // Remote proxies could connect to internal hosts on our behalf.
		DialContext: mediaSSRFDial,
		TLSHandshakeTimeout: 10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout: 45 * time.Second,
		MaxIdleConns: 16,
		MaxIdleConnsPerHost: 4,
	}
}

func mediaSSRFCheckRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("媒体重定向过多")
	}
	return mediaSSRFAllowedURL(request.URL)
}

// Returns a copy to avoid mutating the shared downloader client.
func mediaSSRFClient(original *http.Client, transport *http.Transport) *http.Client {
	copy := *original
	copy.Transport = transport
	copy.Jar = nil
	originalCheck := original.CheckRedirect
	copy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := mediaSSRFCheckRedirect(request, via); err != nil {
			return err
		}
		if originalCheck != nil {
			return originalCheck(request, via)
		}
		return nil
	}
	return &copy
}
