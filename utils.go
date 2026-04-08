package aibot

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// generateRandomString 生成随机十六进制字符串
func generateRandomString(length int) string {
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		// fallback 使用时间戳
		return fmt.Sprintf("%x", time.Now().UnixNano())[:length]
	}
	return hex.EncodeToString(bytes)[:length]
}

// GenerateReqID 生成唯一请求 ID
// 格式：{prefix}_{timestamp}_{random}
func GenerateReqID(prefix string) string {
	timestamp := time.Now().UnixMilli()
	randomStr := generateRandomString(8)
	return fmt.Sprintf("%s_%d_%s", prefix, timestamp, randomStr)
}
