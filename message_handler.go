package aibot

import (
	"encoding/json"
	"fmt"
)

// MessageHandler 消息处理器
// 负责解析 WebSocket 帧并分发为具体的消息事件和事件回调
type MessageHandler struct {
	logger Logger
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(logger Logger) *MessageHandler {
	if logger == nil {
		logger = NewDefaultLogger()
	}
	return &MessageHandler{
		logger: logger,
	}
}

// HandleFrame 处理收到的 WebSocket 帧，解析并触发对应的消息/事件
// emitter: 用于触发事件的回调函数
func (h *MessageHandler) HandleFrame(frame *WsFrame, emitFn func(event string, data interface{})) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Failed to handle message: %v", r)
		}
	}()

	bodyMap, ok := frame.Body.(map[string]interface{})
	if !ok || bodyMap == nil {
		h.logger.Warn("Received invalid message format: %v", frame)
		return
	}

	msgtype, hasMsgtype := bodyMap["msgtype"]
	if !hasMsgtype || msgtype == "" {
		h.logger.Warn("Received invalid message format: missing msgtype")
		return
	}

	// 事件推送回调处理
	if frame.Cmd == WsCmdEventCallback {
		h.handleEventCallback(frame, emitFn)
		return
	}

	// 消息推送回调处理
	h.handleMessageCallback(frame, emitFn, msgtype.(string))
}

// handleMessageCallback 处理消息推送回调 (aibot_msg_callback)
func (h *MessageHandler) handleMessageCallback(frame *WsFrame, emitFn func(event string, data interface{}), msgtype string) {
	// 触发通用消息事件
	emitFn("message", frame)

	// 根据 body 中的消息类型触发特定事件
	switch MessageType(msgtype) {
	case MessageTypeText:
		emitFn("message.text", frame)
	case MessageTypeImage:
		emitFn("message.image", frame)
	case MessageTypeMixed:
		emitFn("message.mixed", frame)
	case MessageTypeVoice:
		emitFn("message.voice", frame)
	case MessageTypeFile:
		emitFn("message.file", frame)
	default:
		h.logger.Debug("Received unhandled message type: %s", msgtype)
	}
}

// handleEventCallback 处理事件推送回调 (aibot_event_callback)
func (h *MessageHandler) handleEventCallback(frame *WsFrame, emitFn func(event string, data interface{})) {
	bodyMap, ok := frame.Body.(map[string]interface{})
	if !ok || bodyMap == nil {
		return
	}

	// 触发通用事件
	emitFn("event", frame)

	// 根据事件类型触发特定事件
	eventData, hasEvent := bodyMap["event"]
	if !hasEvent {
		return
	}

	eventMap, ok := eventData.(map[string]interface{})
	if !ok {
		h.logger.Debug("Received event callback with invalid event format")
		return
	}

	eventType, hasEventType := eventMap["eventtype"]
	if !hasEventType {
		h.logger.Debug("Received event callback without eventtype")
		return
	}

	eventTypeStr, ok := eventType.(string)
	if !ok {
		h.logger.Debug("Received event callback with non-string eventtype")
		return
	}

	emitFn(fmt.Sprintf("event.%s", eventTypeStr), frame)
}

// ParseFrameJSON 解析 JSON 字符串为 WsFrame
func ParseFrameJSON(jsonStr string) (*WsFrame, error) {
	var frame WsFrame
	if err := json.Unmarshal([]byte(jsonStr), &frame); err != nil {
		return nil, err
	}
	return &frame, nil
}
