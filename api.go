package aibot

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// WeComApiClient 企业微信 API 客户端
// 仅负责文件下载等 HTTP 辅助功能，消息收发均走 WebSocket 通道
type WeComApiClient struct {
	logger  Logger
	timeout time.Duration
	client  *http.Client
}

// NewWeComApiClient 创建企业微信 API 客户端
func NewWeComApiClient(logger Logger, timeoutMillis int) *WeComApiClient {
	if logger == nil {
		logger = NewDefaultLogger()
	}

	client := &http.Client{
		Timeout: time.Duration(timeoutMillis) * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return &WeComApiClient{
		logger:  logger,
		timeout: time.Duration(timeoutMillis) * time.Millisecond,
		client:  client,
	}
}

// DownloadFileRaw 下载文件（返回原始 bytes 及文件名）
func (c *WeComApiClient) DownloadFileRaw(fileURL string) ([]byte, string, error) {
	c.logger.Info("Downloading file...")

	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("File download failed: %v", err)
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("file download failed with status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("Failed to read response body: %v", err)
		return nil, "", err
	}

	// 从 Content-Disposition 头中解析文件名
	filename := parseFilenameFromContentDisposition(resp.Header.Get("Content-Disposition"))

	c.logger.Info("File downloaded successfully")
	return data, filename, nil
}

// parseFilenameFromContentDisposition 从 Content-Disposition 头解析文件名
func parseFilenameFromContentDisposition(contentDisposition string) string {
	if contentDisposition == "" {
		return ""
	}

	// 优先匹配 filename*=UTF-8''xxx 格式（RFC 5987）
	utf8Regex := regexp.MustCompile(`filename\*=UTF-8''([^;\s]+)`)
	utf8Match := utf8Regex.FindStringSubmatch(contentDisposition)
	if len(utf8Match) > 1 {
		if decoded, err := url.PathUnescape(utf8Match[1]); err == nil {
			return decoded
		}
	}

	// 匹配 filename="xxx" 或 filename=xxx 格式
	quotedRegex := regexp.MustCompile(`filename="([^"]*)"`)
	quotedMatch := quotedRegex.FindStringSubmatch(contentDisposition)
	if len(quotedMatch) > 1 {
		return quotedMatch[1]
	}

	simpleRegex := regexp.MustCompile(`filename=([^;\s"]+)`)
	simpleMatch := simpleRegex.FindStringSubmatch(contentDisposition)
	if len(simpleMatch) > 1 {
		return simpleMatch[1]
	}

	return ""
}
