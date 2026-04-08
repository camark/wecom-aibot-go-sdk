package aibot

import (
	"crypto/tls"
	"net/http"
	"time"
)

// 版本号
const Version = "1.0.0"

// NewHTTPClient 创建带 TLS 配置的 HTTP 客户端
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}
}
