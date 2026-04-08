package aibot

import "time"

// Logger 日志接口
type Logger interface {
	Debug(message string, args ...interface{})
	Info(message string, args ...interface{})
	Warn(message string, args ...interface{})
	Error(message string, args ...interface{})
}

// WSClientOptions WSClient 配置选项
type WSClientOptions struct {
	// BotID 机器人 ID（在企业微信后台获取）
	BotID string

	// Secret 机器人 Secret（在企业微信后台获取）
	Secret string

	// ReconnectInterval WebSocket 重连基础延迟（毫秒），实际延迟按指数退避递增，默认 1000
	ReconnectInterval int

	// MaxReconnectAttempts 最大重连次数，默认 10，设为 -1 表示无限重连
	MaxReconnectAttempts int

	// HeartbeatInterval 心跳间隔（毫秒），默认 30000
	HeartbeatInterval int

	// RequestTimeout 请求超时时间（毫秒），默认 10000
	RequestTimeout int

	// WsURL 自定义 WebSocket 连接地址，默认 wss://openws.work.weixin.qq.com
	WsURL string

	// Logger 自定义日志实例
	Logger Logger
}

// WsCmd WebSocket 命令类型常量
type WsCmd string

const (
	// 开发者 → 企业微信
	WsCmdSubscribe  WsCmd = "aibot_subscribe"
	WsCmdHeartbeat  WsCmd = "ping"
	WsCmdResponse   WsCmd = "aibot_respond_msg"
	WsCmdResponseWelcome WsCmd = "aibot_respond_welcome_msg"
	WsCmdResponseUpdate WsCmd = "aibot_respond_update_msg"
	WsCmdSendMsg    WsCmd = "aibot_send_msg"

	// 上传临时素材
	WsCmdUploadMediaInit    WsCmd = "aibot_upload_media_init"
	WsCmdUploadMediaChunk   WsCmd = "aibot_upload_media_chunk"
	WsCmdUploadMediaFinish  WsCmd = "aibot_upload_media_finish"

	// 企业微信 → 开发者
	WsCmdCallback       WsCmd = "aibot_msg_callback"
	WsCmdEventCallback  WsCmd = "aibot_event_callback"
)

// MessageType 消息类型枚举
type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeMixed MessageType = "mixed"
	MessageTypeVoice MessageType = "voice"
	MessageTypeFile  MessageType = "file"
)

// EventType 事件类型枚举
type EventType string

const (
	EventTypeEnterChat        EventType = "enter_chat"
	EventTypeTemplateCardEvent EventType = "template_card_event"
	EventTypeFeedbackEvent    EventType = "feedback_event"
)

// TemplateCardType 卡片类型枚举
type TemplateCardType string

const (
	TemplateCardTypeTextNotice       TemplateCardType = "text_notice"
	TemplateCardTypeNewsNotice       TemplateCardType = "news_notice"
	TemplateCardTypeButtonInteraction TemplateCardType = "button_interaction"
	TemplateCardTypeVoteInteraction  TemplateCardType = "vote_interaction"
	TemplateCardTypeMultipleInteraction TemplateCardType = "multiple_interaction"
)

// WsFrame WebSocket 帧结构
type WsFrame struct {
	Cmd     WsCmd            `json:"cmd,omitempty"`
	Headers WsFrameHeaders   `json:"headers,omitempty"`
	Body    interface{}      `json:"body,omitempty"`
	ErrCode *int             `json:"errcode,omitempty"`
	ErrMsg  *string          `json:"errmsg,omitempty"`
}

// WsFrameHeaders WebSocket 帧请求头
type WsFrameHeaders map[string]interface{}

// DefaultReconnectInterval 默认重连基础延迟（毫秒）
const DefaultReconnectInterval = 1000

// DefaultMaxReconnectAttempts 默认最大重连次数
const DefaultMaxReconnectAttempts = 10

// DefaultHeartbeatInterval 默认心跳间隔（毫秒）
const DefaultHeartbeatInterval = 30000

// DefaultRequestTimeout 默认请求超时时间（毫秒）
const DefaultRequestTimeout = 10000

// DefaultReconnectMaxDelay 重连最大延迟（毫秒）
const DefaultReconnectMaxDelay = 30000

// Duration 毫秒转 time.Duration
func Duration(milliseconds int) time.Duration {
	return time.Duration(milliseconds) * time.Millisecond
}

// UploadMediaInitResponse 上传临时素材初始化响应
type UploadMediaInitResponse struct {
	UploadID string `json:"upload_id"`
}

// UploadMediaFinishResponse 上传临时素材完成响应
type UploadMediaFinishResponse struct {
	Type      string `json:"type"`
	MediaID   string `json:"media_id"`
	CreatedAt string `json:"created_at"`
}

// MediaType 上传媒体类型
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVoice MediaType = "voice"
	MediaTypeVideo MediaType = "video"
	MediaTypeFile  MediaType = "file"
)

// 文件大小限制（字节）
const (
	MaxImageSize = 10 * 1024 * 1024 // 10MB
	MaxVoiceSize = 2 * 1024 * 1024  // 2MB
	MaxVideoSize = 10 * 1024 * 1024 // 10MB
	MaxFileSize  = 20 * 1024 * 1024 // 20MB
)

// 分块上传配置
const (
	MaxChunks       = 100
	MaxChunkSize    = 512 * 1024 // 512KB (before base64 encoding)
	UploadTimeout   = 30 * time.Minute
)
