package aibot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// replyQueueItem 回复队列中的单个任务项
type replyQueueItem struct {
	frame  *WsFrame
	result chan *WsFrame
}

// WsConnectionManager WebSocket 长连接管理器
// 负责维护与企业微信的 WebSocket 长连接，包括心跳、重连、认证等
type WsConnectionManager struct {
	logger Logger

	wsURL                string
	heartbeatInterval    time.Duration
	reconnectBaseDelay   time.Duration
	maxReconnectAttempts int
	reconnectMaxDelay    time.Duration

	mu              sync.RWMutex
	ws              *websocket.Conn
	heartbeatTask   context.CancelFunc
	receiveTaskCtx  context.Context
	receiveTaskCancel context.CancelFunc

	reconnectAttempts int
	isManualClose     bool

	// 认证凭证
	botID     string
	botSecret string

	// 心跳相关
	missedPongCount int
	maxMissedPong   int

	// 串行回复队列
	replyQueues   map[string][]*replyQueueItem
	pendingAcks   map[string]*ackItem
	processingQueues map[string]bool

	// 回调
	OnConnected       func()
	OnAuthenticated   func()
	OnDisconnected    func(reason string)
	OnMessage         func(frame *WsFrame)
	OnReconnecting    func(attempt int)
	OnError           func(err error)
}

// ackItem 待处理的 ACK 项
type ackItem struct {
	result chan *WsFrame
	timer  *time.Timer
}

// DefaultReconnectMaxDelay 重连最大延迟（毫秒）
const DefaultReconnectMaxDelayMillis = 30000

// NewWsConnectionManager 创建 WebSocket 连接管理器
func NewWsConnectionManager(
	logger Logger,
	heartbeatInterval int,
	reconnectBaseDelay int,
	maxReconnectAttempts int,
	wsURL string,
) *WsConnectionManager {
	if logger == nil {
		logger = NewDefaultLogger()
	}

	if heartbeatInterval <= 0 {
		heartbeatInterval = DefaultHeartbeatInterval
	}
	if reconnectBaseDelay <= 0 {
		reconnectBaseDelay = DefaultReconnectInterval
	}
	if wsURL == "" {
		wsURL = "wss://openws.work.weixin.qq.com"
	}

	return &WsConnectionManager{
		logger:               logger,
		wsURL:                wsURL,
		heartbeatInterval:    time.Duration(heartbeatInterval) * time.Millisecond,
		reconnectBaseDelay:   time.Duration(reconnectBaseDelay) * time.Millisecond,
		maxReconnectAttempts: maxReconnectAttempts,
		reconnectMaxDelay:    time.Duration(DefaultReconnectMaxDelayMillis) * time.Millisecond,

		replyQueues:        make(map[string][]*replyQueueItem),
		pendingAcks:        make(map[string]*ackItem),
		processingQueues:   make(map[string]bool),

		maxMissedPong: 2,
	}
}

// SetCredentials 设置认证凭证
func (m *WsConnectionManager) SetCredentials(botID, botSecret string) {
	m.botID = botID
	m.botSecret = botSecret
}

// Connect 建立 WebSocket 连接
func (m *WsConnectionManager) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.isManualClose = false

	// 清理旧连接
	m.cleanupWs()

	m.logger.Info("Connecting to WebSocket: %s...", m.wsURL)

	dialer := websocket.DefaultDialer
	dialer.EnableCompression = false

	conn, _, err := dialer.Dial(m.wsURL, nil)
	if err != nil {
		m.logger.Error("Failed to create WebSocket connection: %v", err)
		if m.OnError != nil {
			m.OnError(err)
		}
		go m.scheduleReconnect()
		return err
	}

	m.ws = conn
	m.reconnectAttempts = 0
	m.missedPongCount = 0

	m.logger.Info("WebSocket connection established, sending auth...")

	// 连接建立回调
	if m.OnConnected != nil {
		m.OnConnected()
	}

	// 启动消息接收循环
	ctx, cancel := context.WithCancel(context.Background())
	m.receiveTaskCtx = ctx
	m.receiveTaskCancel = cancel

	go m.receiveLoop()

	// 发送认证帧
	go func() {
		if err := m.sendAuth(); err != nil {
			m.logger.Error("Failed to send auth frame: %v", err)
		}
	}()

	return nil
}

// cleanupWs 清理 WebSocket 连接
func (m *WsConnectionManager) cleanupWs() {
	if m.receiveTaskCancel != nil {
		m.receiveTaskCancel()
	}

	if m.heartbeatTask != nil {
		m.heartbeatTask()
	}

	if m.ws != nil {
		m.ws.Close()
		m.ws = nil
	}
}

// sendAuth 发送认证帧
func (m *WsConnectionManager) sendAuth() error {
	frame := &WsFrame{
		Cmd: WsCmdSubscribe,
		Headers: WsFrameHeaders{
			"req_id": GenerateReqID(string(WsCmdSubscribe)),
		},
		Body: map[string]interface{}{
			"bot_id": m.botID,
			"secret": m.botSecret,
		},
	}

	if err := m.Send(frame); err != nil {
		return err
	}

	m.logger.Info("Auth frame sent")
	return nil
}

// receiveLoop 消息接收循环
func (m *WsConnectionManager) receiveLoop() {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("WebSocket receive loop panic: %v", r)
		}
	}()

	for {
		select {
		case <-m.receiveTaskCtx.Done():
			return
		default:
			_, message, err := m.ws.ReadMessage()
			if err != nil {
				m.handleConnectionError(err)
				return
			}

			var frame WsFrame
			if err := json.Unmarshal(message, &frame); err != nil {
				m.logger.Error("Failed to parse WebSocket message: %v", err)
				continue
			}

			m.handleFrame(&frame)
		}
	}
}

// handleConnectionError 处理连接错误
func (m *WsConnectionManager) handleConnectionError(err error) {
	closeError, ok := err.(*websocket.CloseError)
	var reasonStr string
	if ok {
		reasonStr = fmt.Sprintf("code: %d", closeError.Code)
	} else {
		reasonStr = err.Error()
	}

	m.logger.Warn("WebSocket connection closed: %s", reasonStr)
	m.stopHeartbeat()
	m.clearPendingMessages(fmt.Sprintf("WebSocket connection closed (%s)", reasonStr))

	if m.OnDisconnected != nil {
		m.OnDisconnected(reasonStr)
	}

	if !m.isManualClose {
		go m.scheduleReconnect()
	}
}

// handleFrame 处理收到的帧数据
func (m *WsConnectionManager) handleFrame(frame *WsFrame) {
	cmd := frame.Cmd

	// 消息推送
	if cmd == WsCmdCallback {
		m.logger.Debug("Received push message: %v", frame.Body)
		if m.OnMessage != nil {
			m.OnMessage(frame)
		}
		return
	}

	// 事件推送
	if cmd == WsCmdEventCallback {
		m.logger.Debug("Received event callback: %v", frame.Body)
		if m.OnMessage != nil {
			m.OnMessage(frame)
		}
		return
	}

	// 无 cmd 的帧：认证响应、心跳响应或回复消息回执
	reqID, hasReqID := frame.Headers["req_id"]
	if !hasReqID {
		m.logger.Warn("Received frame without req_id: %v", frame)
		return
	}

	reqIDStr, ok := reqID.(string)
	if !ok {
		m.logger.Warn("Received frame with non-string req_id")
		return
	}

	// 检查是否是回复消息的回执
	if m.isReplyAck(reqIDStr) {
		m.handleReplyAck(reqIDStr, frame)
		return
	}

	// 认证成功响应
	if startWith(reqIDStr, string(WsCmdSubscribe)) {
		m.handleAuthResponse(frame)
		return
	}

	// 心跳响应
	if startWith(reqIDStr, string(WsCmdHeartbeat)) {
		m.handleHeartbeatResponse(frame)
		return
	}

	// 未知帧类型（可能是其他类型的回执）
	m.logger.Debug("Received frame with req_id %s (possible ack)", reqIDStr)
}

// startWith 简单的字符串前缀检查
func startWith(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

// isReplyAck 检查是否是回复消息的回执
func (m *WsConnectionManager) isReplyAck(reqID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.pendingAcks[reqID]
	return exists
}

// handleAuthResponse 处理认证响应
func (m *WsConnectionManager) handleAuthResponse(frame *WsFrame) {
	errCode := getErrCode(frame)
	if errCode != 0 {
		m.logger.Error("Authentication failed: errcode=%d, errmsg=%s", errCode, getErrMsg(frame))
		if m.OnError != nil {
			m.OnError(fmt.Errorf("authentication failed: %s (code: %d)", getErrMsg(frame), errCode))
		}
		return
	}

	m.logger.Info("Authentication successful")
	m.startHeartbeat()

	if m.OnAuthenticated != nil {
		m.OnAuthenticated()
	}
}

// handleHeartbeatResponse 处理心跳响应
func (m *WsConnectionManager) handleHeartbeatResponse(frame *WsFrame) {
	errCode := getErrCode(frame)
	if errCode != 0 {
		m.logger.Warn("Heartbeat ack error: errcode=%d, errmsg=%s", errCode, getErrMsg(frame))
		return
	}

	m.missedPongCount = 0
	m.logger.Debug("Received heartbeat ack")
}

// startHeartbeat 启动心跳定时器
func (m *WsConnectionManager) startHeartbeat() {
	m.stopHeartbeat()

	ctx, cancel := context.WithCancel(context.Background())
	m.heartbeatTask = cancel

	go m.heartbeatLoop(ctx)

	m.logger.Debug("Heartbeat timer started, interval: %v", m.heartbeatInterval)
}

// stopHeartbeat 停止心跳定时器
func (m *WsConnectionManager) stopHeartbeat() {
	if m.heartbeatTask != nil {
		m.heartbeatTask()
		m.heartbeatTask = nil
		m.logger.Debug("Heartbeat timer stopped")
	}
}

// heartbeatLoop 心跳循环
func (m *WsConnectionManager) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(m.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.sendHeartbeat(); err != nil {
				m.logger.Error("Failed to send heartbeat: %v", err)
			}
		}
	}
}

// sendHeartbeat 发送心跳
func (m *WsConnectionManager) sendHeartbeat() error {
	// 检查连续未收到 pong 的次数
	if m.missedPongCount >= m.maxMissedPong {
		m.logger.Warn("No heartbeat ack received for %d consecutive pings, connection considered dead", m.missedPongCount)
		m.stopHeartbeat()

		m.mu.Lock()
		if m.ws != nil {
			m.ws.Close()
		}
		m.mu.Unlock()
		return nil
	}

	m.missedPongCount++

	frame := &WsFrame{
		Cmd: WsCmdHeartbeat,
		Headers: WsFrameHeaders{
			"req_id": GenerateReqID(string(WsCmdHeartbeat)),
		},
	}

	if err := m.Send(frame); err != nil {
		return err
	}

	if m.missedPongCount > 1 {
		m.logger.Debug("Heartbeat sent (awaiting %d pong)", m.missedPongCount)
	} else {
		m.logger.Debug("Heartbeat sent")
	}

	return nil
}

// scheduleReconnect 安排重连
func (m *WsConnectionManager) scheduleReconnect() {
	m.mu.Lock()
	if m.maxReconnectAttempts != -1 && m.reconnectAttempts >= m.maxReconnectAttempts {
		m.mu.Unlock()
		m.logger.Error("Max reconnect attempts reached (%d), giving up", m.maxReconnectAttempts)
		if m.OnError != nil {
			m.OnError(fmt.Errorf("max reconnect attempts exceeded"))
		}
		return
	}

	m.reconnectAttempts++
	delay := m.reconnectBaseDelay * time.Duration(1<<(m.reconnectAttempts-1))
	if delay > m.reconnectMaxDelay {
		delay = m.reconnectMaxDelay
	}

	m.mu.Unlock()

	m.logger.Info("Reconnecting in %v (attempt %d)...", delay, m.reconnectAttempts)
	if m.OnReconnecting != nil {
		m.OnReconnecting(m.reconnectAttempts)
	}

	time.Sleep(delay)

	m.mu.Lock()
	if m.isManualClose {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	m.Connect()
}

// Send 发送数据帧
func (m *WsConnectionManager) Send(frame *WsFrame) error {
	m.mu.RLock()
	ws := m.ws
	m.mu.RUnlock()

	if ws == nil {
		return fmt.Errorf("WebSocket not connected, unable to send data")
	}

	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}

	return ws.WriteMessage(websocket.TextMessage, data)
}

// SendReply 通过 WebSocket 通道发送回复消息（串行队列版本）
func (m *WsConnectionManager) SendReply(reqID string, body interface{}, cmd WsCmd) (*WsFrame, error) {
	m.mu.Lock()

	if _, exists := m.replyQueues[reqID]; !exists {
		m.replyQueues[reqID] = []*replyQueueItem{}
	}

	resultChan := make(chan *WsFrame, 1)

	frame := &WsFrame{
		Cmd: cmd,
		Headers: WsFrameHeaders{
			"req_id": reqID,
		},
		Body: body,
	}

	item := &replyQueueItem{
		frame:  frame,
		result: resultChan,
	}

	queue := m.replyQueues[reqID]
	if len(queue) >= 100 {
		m.mu.Unlock()
		m.logger.Warn("Reply queue for reqId %s exceeds max size (100), rejecting new message", reqID)
		return nil, fmt.Errorf("reply queue for reqId %s exceeds max size", reqID)
	}

	queue = append(queue, item)
	m.replyQueues[reqID] = queue

	// 如果队列中只有这一条，立即开始处理
	if len(queue) == 1 && !m.processingQueues[reqID] {
		m.processingQueues[reqID] = true
		go m.processReplyQueue(reqID)
	}

	m.mu.Unlock()

	// 等待结果
	result := <-resultChan
	return result, nil
}

// processReplyQueue 处理指定 req_id 的回复队列
func (m *WsConnectionManager) processReplyQueue(reqID string) {
	for {
		m.mu.Lock()
		queue := m.replyQueues[reqID]
		if len(queue) == 0 {
			delete(m.replyQueues, reqID)
			delete(m.processingQueues, reqID)
			m.mu.Unlock()
			break
		}

		item := queue[0]
		m.mu.Unlock()

		// 发送消息
		if err := m.Send(item.frame); err != nil {
			m.logger.Error("Failed to send reply for reqId %s: %v", reqID, err)
			m.mu.Lock()
			queue = m.replyQueues[reqID]
			if len(queue) > 0 {
				queue = queue[1:]
				m.replyQueues[reqID] = queue
			}
			item.result <- nil
			m.mu.Unlock()
			continue
		}

		m.logger.Debug("Reply message sent via WebSocket, reqId: %s", reqID)

		// 等待回执
		resultChan := make(chan *WsFrame, 1)
		timer := time.AfterFunc(5*time.Second, func() {
			m.handleReplyAckTimeout(reqID, resultChan)
		})

		m.mu.Lock()
		m.pendingAcks[reqID] = &ackItem{
			result: resultChan,
			timer:  timer,
		}
		m.mu.Unlock()

		// 等待回执结果
		ackFrame := <-resultChan

		m.mu.Lock()
		queue = m.replyQueues[reqID]
		if len(queue) > 0 {
			queue = queue[1:]
			m.replyQueues[reqID] = queue
		}
		item.result <- ackFrame
		m.mu.Unlock()
	}
}

// handleReplyAckTimeout 回复回执超时回调
func (m *WsConnectionManager) handleReplyAckTimeout(reqID string, resultChan chan *WsFrame) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.pendingAcks[reqID]
	if !exists {
		return
	}

	delete(m.pendingAcks, reqID)
	m.logger.Warn("Reply ack timeout (5s) for reqId: %s", reqID)

	select {
	case resultChan <- nil:
	default:
	}
}

// handleReplyAck 处理回复消息的回执
func (m *WsConnectionManager) handleReplyAck(reqID string, frame *WsFrame) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.pendingAcks[reqID]
	if !exists {
		return
	}

	// 取消超时
	if item.timer != nil {
		item.timer.Stop()
	}

	delete(m.pendingAcks, reqID)

	errCode := getErrCode(frame)
	if errCode != 0 {
		m.logger.Warn("Reply ack error: reqId=%s, errcode=%d, errmsg=%s", reqID, errCode, getErrMsg(frame))
		select {
		case item.result <- frame:
		default:
		}
	} else {
		m.logger.Debug("Reply ack received for reqId: %s", reqID)
		select {
		case item.result <- frame:
		default:
		}
	}
}

// clearPendingMessages 清理所有待处理的消息和回执
func (m *WsConnectionManager) clearPendingMessages(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for reqID, item := range m.pendingAcks {
		if item.timer != nil {
			item.timer.Stop()
		}
		delete(m.pendingAcks, reqID)
	}

	for reqID, queue := range m.replyQueues {
		for _, item := range queue {
			select {
			case item.result <- nil:
			default:
			}
		}
		delete(m.replyQueues, reqID)
	}
}

// Disconnect 主动断开连接
func (m *WsConnectionManager) Disconnect() {
	m.isManualClose = true
	m.stopHeartbeat()
	m.clearPendingMessages("Connection manually closed")

	m.mu.Lock()
	if m.ws != nil {
		go func() {
			m.ws.Close()
		}()
	}
	m.mu.Unlock()

	m.logger.Info("WebSocket connection manually closed")
}

// IsConnected 获取当前连接状态
func (m *WsConnectionManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ws != nil
}

// 辅助函数
func getErrCode(frame *WsFrame) int {
	if frame.ErrCode == nil {
		return -1
	}
	return *frame.ErrCode
}

func getErrMsg(frame *WsFrame) string {
	if frame.ErrMsg == nil {
		return ""
	}
	return *frame.ErrMsg
}
