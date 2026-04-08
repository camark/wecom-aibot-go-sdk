package aibot

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

// WSClient 企业微信智能机器人 Python SDK 核心客户端
// 基于 gorilla/websocket 的事件驱动架构，提供 WebSocket 长连接消息收发能力
type WSClient struct {
	options *WSClientOptions
	logger  Logger

	apiClient *WeComApiClient
	wsManager *WsConnectionManager
	handler   *MessageHandler

	started bool

	// 事件处理函数
	eventHandlers map[string][]func(interface{})
	mu            sync.RWMutex
}

// NewWSClient 创建 WSClient 实例
func NewWSClient(options *WSClientOptions) *WSClient {
	if options == nil {
		options = &WSClientOptions{}
	}

	// 设置默认值
	if options.ReconnectInterval <= 0 {
		options.ReconnectInterval = DefaultReconnectInterval
	}
	if options.MaxReconnectAttempts == 0 {
		options.MaxReconnectAttempts = DefaultMaxReconnectAttempts
	}
	if options.HeartbeatInterval <= 0 {
		options.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = DefaultRequestTimeout
	}

	logger := options.Logger
	if logger == nil {
		logger = NewDefaultLogger()
	}

	client := &WSClient{
		options:       options,
		logger:        logger,
		eventHandlers: make(map[string][]func(interface{})),
	}

	// 初始化 API 客户端（仅用于文件下载）
	client.apiClient = NewWeComApiClient(logger, options.RequestTimeout)

	// 初始化 WebSocket 管理器
	client.wsManager = NewWsConnectionManager(
		logger,
		options.HeartbeatInterval,
		options.ReconnectInterval,
		options.MaxReconnectAttempts,
		options.WsURL,
	)

	// 设置认证凭证
	client.wsManager.SetCredentials(options.BotID, options.Secret)

	// 初始化消息处理器
	client.handler = NewMessageHandler(logger)

	// 绑定 WebSocket 事件
	client.setupWsEvents()

	return client
}

// setupWsEvents 设置 WebSocket 事件处理
func (c *WSClient) setupWsEvents() {
	c.wsManager.OnConnected = func() {
		c.emit("connected", nil)
	}

	c.wsManager.OnAuthenticated = func() {
		c.logger.Info("Authenticated")
		c.emit("authenticated", nil)
	}

	c.wsManager.OnDisconnected = func(reason string) {
		c.emit("disconnected", reason)
	}

	c.wsManager.OnReconnecting = func(attempt int) {
		c.emit("reconnecting", attempt)
	}

	c.wsManager.OnError = func(err error) {
		c.emit("error", err)
	}

	c.wsManager.OnMessage = func(frame *WsFrame) {
		c.handler.HandleFrame(frame, c.emit)
	}
}

// Connect 建立 WebSocket 长连接
// SDK 使用内置默认地址建立连接，连接成功后自动发送认证帧（bot_id + secret）
// 返回：返回 self，支持链式调用
func (c *WSClient) Connect() (*WSClient, error) {
	if c.started {
		c.logger.Warn("Client already connected")
		return c, nil
	}

	c.logger.Info("Establishing WebSocket connection...")
	c.started = true

	if err := c.wsManager.Connect(); err != nil {
		return nil, err
	}

	return c, nil
}

// Disconnect 断开 WebSocket 连接
func (c *WSClient) Disconnect() {
	if !c.started {
		c.logger.Warn("Client not connected")
		return
	}

	c.logger.Info("Disconnecting...")
	c.started = false
	c.wsManager.Disconnect()
	c.logger.Info("Disconnected")
}

// Reply 通过 WebSocket 通道发送回复消息（通用方法）
// frame: 收到的原始 WebSocket 帧，透传 headers.req_id
// body: 回复消息体
// cmd: 发送的命令类型（可选，默认 RESPONSE）
// 返回：回执帧
func (c *WSClient) Reply(frame *WsFrame, body map[string]interface{}, cmd ...WsCmd) (*WsFrame, error) {
	headers := frame.Headers

	reqID := ""
	if rid, exists := headers["req_id"]; exists {
		reqID, _ = rid.(string)
	}

	cmdToUse := WsCmdResponse
	if len(cmd) > 0 {
		cmdToUse = cmd[0]
	}

	return c.wsManager.SendReply(reqID, body, cmdToUse)
}

// ReplyStream 发送流式文本回复（便捷方法）
// frame: 收到的原始 WebSocket 帧，透传 headers.req_id
// streamID: 流式消息 ID
// content: 回复内容（支持 Markdown）
// finish: 是否结束流式消息，默认 False
// msgItem: 图文混排项（仅在 finish=True 时有效）
// feedback: 反馈信息（仅在首次回复时设置）
// 返回：回执帧
func (c *WSClient) ReplyStream(
	frame *WsFrame,
	streamID string,
	content string,
	finish bool,
	msgItem []map[string]interface{},
	feedback map[string]interface{},
) (*WsFrame, error) {
	stream := map[string]interface{}{
		"id":      streamID,
		"finish":  finish,
		"content": content,
	}

	// msg_item 仅在 finish=True 时支持
	if finish && len(msgItem) > 0 {
		stream["msg_item"] = msgItem
	}

	// feedback 仅在首次回复时设置
	if feedback != nil {
		stream["feedback"] = feedback
	}

	return c.Reply(frame, map[string]interface{}{
		"msgtype": "stream",
		"stream":  stream,
	})
}

// ReplyWelcome 发送欢迎语回复
// frame: 对应事件的 WebSocket 帧
// body: 欢迎语消息体（支持文本或模板卡片格式）
// 返回：回执帧
func (c *WSClient) ReplyWelcome(frame *WsFrame, body map[string]interface{}) (*WsFrame, error) {
	return c.Reply(frame, body, WsCmdResponseWelcome)
}

// ReplyTemplateCard 回复模板卡片消息
// frame: 收到的原始 WebSocket 帧
// templateCard: 模板卡片内容
// feedback: 反馈信息（可选）
// 返回：回执帧
func (c *WSClient) ReplyTemplateCard(
	frame *WsFrame,
	templateCard map[string]interface{},
	feedback ...map[string]interface{},
) (*WsFrame, error) {
	var card map[string]interface{}

	if len(feedback) > 0 && feedback[0] != nil {
		card = make(map[string]interface{})
		for k, v := range templateCard {
			card[k] = v
		}
		card["feedback"] = feedback[0]
	} else {
		card = templateCard
	}

	body := map[string]interface{}{
		"msgtype":       "template_card",
		"template_card": card,
	}

	return c.Reply(frame, body)
}

// ReplyStreamWithCard 发送流式消息 + 模板卡片组合回复
// frame: 收到的原始 WebSocket 帧
// streamID: 流式消息 ID
// content: 回复内容（支持 Markdown）
// finish: 是否结束流式消息，默认 False
// msgItem: 图文混排项（仅在 finish=True 时有效）
// streamFeedback: 流式消息反馈信息（首次回复时设置）
// templateCard: 模板卡片内容（同一消息只能回复一次）
// cardFeedback: 模板卡片反馈信息
// 返回：回执帧
func (c *WSClient) ReplyStreamWithCard(
	frame *WsFrame,
	streamID string,
	content string,
	finish bool,
	msgItem []map[string]interface{},
	streamFeedback map[string]interface{},
	templateCard map[string]interface{},
	cardFeedback map[string]interface{},
) (*WsFrame, error) {
	stream := map[string]interface{}{
		"id":      streamID,
		"finish":  finish,
		"content": content,
	}

	if finish && len(msgItem) > 0 {
		stream["msg_item"] = msgItem
	}

	if streamFeedback != nil {
		stream["feedback"] = streamFeedback
	}

	body := map[string]interface{}{
		"msgtype": "stream_with_template_card",
		"stream":  stream,
	}

	if templateCard != nil {
		if cardFeedback != nil {
			card := make(map[string]interface{})
			for k, v := range templateCard {
				card[k] = v
			}
			card["feedback"] = cardFeedback
			body["template_card"] = card
		} else {
			body["template_card"] = templateCard
		}
	}

	return c.Reply(frame, body)
}

// UpdateTemplateCard 更新模板卡片
// frame: 对应事件的 WebSocket 帧
// templateCard: 模板卡片内容（task_id 需跟回调收到的 task_id 一致）
// userids: 要替换模版卡片消息的 userid 列表（可选）
// 返回：回执帧
func (c *WSClient) UpdateTemplateCard(
	frame *WsFrame,
	templateCard map[string]interface{},
	userids ...[]string,
) (*WsFrame, error) {
	body := map[string]interface{}{
		"response_type": "update_template_card",
		"template_card": templateCard,
	}

	if len(userids) > 0 && len(userids[0]) > 0 {
		body["userids"] = userids[0]
	}

	return c.Reply(frame, body, WsCmdResponseUpdate)
}

// SendMessage 主动发送消息
// chatid: 会话 ID，单聊填用户的 userid，群聊填对应群聊的 chatid
// body: 消息体（支持 markdown 或 template_card 格式）
// 返回：回执帧
func (c *WSClient) SendMessage(chatid string, body map[string]interface{}) (*WsFrame, error) {
	reqID := GenerateReqID(string(WsCmdSendMsg))

	fullBody := make(map[string]interface{})
	fullBody["chatid"] = chatid
	for k, v := range body {
		fullBody[k] = v
	}

	return c.wsManager.SendReply(reqID, fullBody, WsCmdSendMsg)
}

// DownloadFile 下载文件并使用 AES 密钥解密（保存到系统 temp 目录或配置目录）
// url: 文件下载地址
// aesKey: AES 解密密钥（Base64 编码），取自消息中 image.aeskey 或 file.aeskey
// 返回：(解密后的文件数据，保存路径，error)
func (c *WSClient) DownloadFile(url string, aesKey string) ([]byte, string, error) {
	// 确定保存目录
	saveDir := c.options.FileDownloadPath
	if saveDir == "" {
		saveDir = os.TempDir()
	}

	return c.DownloadFileToDir(url, aesKey, saveDir)
}

// DownloadFileToDir 下载文件并使用 AES 密钥解密到指定目录
// url: 文件下载地址
// aesKey: AES 解密密钥（Base64 编码）
// saveDir: 保存目录
// 返回：(解密后的文件数据，完整保存路径，error)
func (c *WSClient) DownloadFileToDir(url string, aesKey string, saveDir string) ([]byte, string, error) {
	c.logger.Info("Downloading and decrypting file...")

	// 下载加密的文件数据
	encryptedData, filename, err := c.apiClient.DownloadFileRaw(url)
	if err != nil {
		return nil, "", err
	}

	// 如果没有提供 aes_key，直接返回原始数据
	if aesKey == "" {
		c.logger.Warn("No aes_key provided, returning raw file data")
		return encryptedData, filename, nil
	}

	// 使用 AES-256-CBC 解密
	decryptedData, err := DecryptFile(encryptedData, aesKey)
	if err != nil {
		c.logger.Error("File decrypt failed: %v", err)
		return nil, "", err
	}

	// 确保保存目录存在
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		c.logger.Error("Failed to create directory: %v", err)
		return nil, "", err
	}

	// 保存文件到指定目录
	savePath := filepath.Join(saveDir, filename)
	if err := os.WriteFile(savePath, decryptedData, 0644); err != nil {
		c.logger.Error("Failed to save file: %v", err)
		return nil, "", err
	}

	c.logger.Info("File downloaded and decrypted successfully, saved to: %s", savePath)
	return decryptedData, savePath, nil
}

// UploadMedia 上传临时素材（便捷方法）
// 自动处理分块上传的完整流程：初始化 -> 分块上传 -> 完成
// mediaType: 媒体类型 (image/voice/video/file)
// filePath: 本地文件路径
// 返回：(media_id, error)
func (c *WSClient) UploadMedia(mediaType MediaType, filePath string) (string, error) {
	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	fileSize := len(data)
	filename := filepath.Base(filePath)

	// 检查文件大小限制
	var maxSize int
	switch mediaType {
	case MediaTypeImage:
		maxSize = MaxImageSize
	case MediaTypeVoice:
		maxSize = MaxVoiceSize
	case MediaTypeVideo:
		maxSize = MaxVideoSize
	case MediaTypeFile:
		maxSize = MaxFileSize
	default:
		return "", fmt.Errorf("unknown media type: %s", mediaType)
	}

	if fileSize > maxSize {
		return "", fmt.Errorf("file size (%d bytes) exceeds limit (%d bytes)", fileSize, maxSize)
	}

	if fileSize == 0 {
		return "", fmt.Errorf("file is empty")
	}

	// 计算 MD5
	md5Hash := fmt.Sprintf("%x", md5.Sum(data))

	// 计算分块
	totalChunks := (fileSize + MaxChunkSize - 1) / MaxChunkSize
	if totalChunks > MaxChunks {
		return "", fmt.Errorf("file requires %d chunks, exceeds maximum %d", totalChunks, MaxChunks)
	}

	c.logger.Info("Uploading media: type=%s, filename=%s, size=%d, chunks=%d", mediaType, filename, fileSize, totalChunks)

	// 步骤 1: 初始化上传
	uploadID, err := c.wsManager.UploadMediaInit(mediaType, filename, fileSize, totalChunks, md5Hash)
	if err != nil {
		return "", fmt.Errorf("upload init failed: %w", err)
	}

	// 步骤 2: 上传分块
	for i := 0; i < totalChunks; i++ {
		start := i * MaxChunkSize
		end := start + MaxChunkSize
		if end > fileSize {
			end = fileSize
		}

		chunkData := data[start:end]
		base64Data := base64.StdEncoding.EncodeToString(chunkData)

		if err := c.wsManager.UploadMediaChunk(uploadID, i+1, base64Data); err != nil {
			return "", fmt.Errorf("upload chunk %d failed: %w", i+1, err)
		}

		c.logger.Debug("Chunk %d/%d uploaded", i+1, totalChunks)
	}

	// 步骤 3: 完成上传
	mediaID, err := c.wsManager.UploadMediaFinish(uploadID)
	if err != nil {
		return "", fmt.Errorf("upload finish failed: %w", err)
	}

	c.logger.Info("Media uploaded successfully: media_id=%s", mediaID)
	return mediaID, nil
}

// IsConnected 获取当前连接状态
func (c *WSClient) IsConnected() bool {
	return c.wsManager.IsConnected()
}

// API 获取 API 客户端实例（供高级用途使用）
func (c *WSClient) API() *WeComApiClient {
	return c.apiClient
}

// On 注册事件监听器
func (c *WSClient) On(event string, handler func(interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventHandlers[event] = append(c.eventHandlers[event], handler)
}

// emit 触发事件
func (c *WSClient) emit(event string, data interface{}) {
	c.mu.RLock()
	handlers := c.eventHandlers[event]
	c.mu.RUnlock()

	for _, handler := range handlers {
		handler(data)
	}
}

// Run 便捷方法：启动事件循环并连接
// 阻塞运行，直到收到中断信号
func (c *WSClient) Run() {
	c.logger.Info("Starting WSClient...")

	// 连接
	if _, err := c.Connect(); err != nil {
		c.logger.Error("Failed to connect: %v", err)
		return
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	c.logger.Info("Shutting down...")
	c.Disconnect()
}

// RunWithContext 带上下文取消的启动方法
func (c *WSClient) RunWithContext(ctx context.Context) error {
	if _, err := c.Connect(); err != nil {
		return err
	}

	<-ctx.Done()
	c.Disconnect()
	return nil
}
