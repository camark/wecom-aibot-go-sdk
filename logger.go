package aibot

import (
	"fmt"
	"os"
	"time"
)

// DefaultLogger 默认日志实现，带有日志级别和时间戳的控制台日志
type DefaultLogger struct {
	prefix string
}

// NewDefaultLogger 创建默认日志实例
func NewDefaultLogger() *DefaultLogger {
	return NewDefaultLoggerWithPrefix("AiBotSDK")
}

// NewDefaultLoggerWithPrefix 创建带前缀的默认日志实例
func NewDefaultLoggerWithPrefix(prefix string) *DefaultLogger {
	return &DefaultLogger{
		prefix: prefix,
	}
}

// formatTime 格式化当前时间 (ISO 8601)
func (l *DefaultLogger) formatTime() string {
	return time.Now().Format(time.RFC3339)
}

// Debug 输出 debug 级别日志
func (l *DefaultLogger) Debug(message string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] [%s] [DEBUG] "+message+"\n", append([]any{l.formatTime(), l.prefix}, args...)...)
}

// Info 输出 info 级别日志
func (l *DefaultLogger) Info(message string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] [%s] [INFO] "+message+"\n", append([]any{l.formatTime(), l.prefix}, args...)...)
}

// Warn 输出 warn 级别日志
func (l *DefaultLogger) Warn(message string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] [%s] [WARN] "+message+"\n", append([]any{l.formatTime(), l.prefix}, args...)...)
}

// Error 输出 error 级别日志
func (l *DefaultLogger) Error(message string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] [%s] [ERROR] "+message+"\n", append([]any{l.formatTime(), l.prefix}, args...)...)
}
