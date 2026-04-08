# wecom-aibot-go-sdk (Go)

企业微信智能机器人 Go SDK —— 基于 WebSocket 长连接通道，提供消息收发、流式回复、模板卡片、事件回调、文件下载解密等核心能力。

> 本项目是 [@wecom/aibot-node-sdk](https://www.npmjs.com/package/@wecom/aibot-node-sdk)（Node.js 版）和 [wecom-aibot-python-sdk](https://github.com/WecomTeam/wecom-aibot-python-sdk)（Python 版）的 Go 语言实现。

## ✨ 特性

- 🔗 **WebSocket 长连接** — 基于 `wss://openws.work.weixin.qq.com` 内置默认地址，开箱即用
- 🔐 **自动认证** — 连接建立后自动发送认证帧（bot_id + secret）
- 💓 **心跳保活** — 自动维护心跳，连续未收到 ack 时自动判定连接异常
- 🔄 **断线重连** — 指数退避重连策略（1s → 2s → 4s → ... → 30s 上限），支持自定义最大重连次数
- 📨 **消息分发** — 自动解析消息类型并触发对应事件（text / image / mixed / voice / file）
- 🌊 **流式回复** — 内置流式回复方法，支持 Markdown 和图文混排
- 🃏 **模板卡片** — 支持回复模板卡片消息、流式 + 卡片组合回复、更新卡片
- 📤 **主动推送** — 支持向指定会话主动发送 Markdown 或模板卡片消息，无需依赖回调帧
- 📡 **事件回调** — 支持进入会话、模板卡片按钮点击、用户反馈等事件
- ⏩ **串行回复队列** — 同一 req_id 的回复消息串行发送，自动等待回执
- 🔑 **文件下载解密** — 内置 AES-256-CBC 文件解密，每个图片/文件消息自带独立的 aeskey
- 🪵 **可插拔日志** — 支持自定义 Logger，内置带时间戳的 DefaultLogger

## 📦 安装

```bash
go get github.com/WecomTeam/wecom-aibot-go-sdk
```

**依赖：**
- Go >= 1.21
- gorilla/websocket
- golang.org/x/net

## ⚙️ 配置

```bash
# 复制示例配置文件
cp .env.example .env

# 编辑 .env 文件，填入真实配置
# WECHAT_BOT_ID=your-bot-id
# WECHAT_BOT_SECRET=your-bot-secret
```

## 🚀 快速开始

```go
package main

import (
    "fmt"
    "os"
    "time"

    "github.com/WecomTeam/wecom-aibot-go-sdk"
)

func main() {
    // 从环境变量读取配置
    botID := os.Getenv("WECHAT_BOT_ID")
    secret := os.Getenv("WECHAT_BOT_SECRET")

    // 创建客户端实例
    client := aibot.NewWSClient(&aibot.WSClientOptions{
        BotID:  botID,
        Secret: secret,
        Logger: aibot.NewDefaultLogger(),
    })

    // 监听认证成功
    client.On("authenticated", func(data interface{}) {
        fmt.Println("认证成功")
    })

    // 监听文本消息并进行流式回复
    client.On("message.text", func(data interface{}) {
        frame, _ := data.(*aibot.WsFrame)
        body := frame.Body.(map[string]interface{})
        text := body["text"].(map[string]interface{})
        content := text["content"].(string)

        fmt.Printf("收到文本消息：%s\n", content)

        streamID := aibot.GenerateReqID("stream")

        // 发送流式中间内容
        client.ReplyStream(frame, streamID, "正在思考中...", false, nil, nil)

        // 发送最终结果
        time.Sleep(1 * time.Second)
        client.ReplyStream(frame, streamID, fmt.Sprintf("你好！你说的是：\"%s\"", content), true, nil, nil)
    })

    // 监听进入会话事件（发送欢迎语）
    client.On("event.enter_chat", func(data interface{}) {
        frame, _ := data.(*aibot.WsFrame)
        client.ReplyWelcome(frame, map[string]interface{}{
            "msgtype": "text",
            "text":    map[string]interface{}{"content": "您好！我是智能助手，有什么可以帮您的吗？"},
        })
    })

    // 启动客户端（阻塞运行）
    client.Run()
}
```

或者手动管理连接：

```go
// 建立连接
_, err := client.Connect()
if err != nil {
    log.Fatal(err)
}

// 保持运行
select {}

// 断开连接
client.Disconnect()
```

## 📖 API 文档

### `WSClient`

核心客户端类，提供连接管理、消息收发等功能。

#### 构造函数

```go
client := aibot.NewWSClient(&aibot.WSClientOptions{
    BotID:              "your-bot-id",
    Secret:             "your-bot-secret",
    ReconnectInterval:  1000,              // 重连基础延迟（毫秒）
    MaxReconnectAttempts: 10,              // 最大重连次数（-1 表示无限）
    HeartbeatInterval:  30000,             // 心跳间隔（毫秒）
    RequestTimeout:     10000,             // HTTP 请求超时（毫秒）
    WsURL:              "",                // 自定义 WebSocket 地址
    Logger:             nil,               // 自定义日志
})
```

#### 方法

| 方法 | 说明 | 返回值 |
| --- | --- | --- |
| `Connect()` | 建立 WebSocket 连接，连接后自动认证 | `(*WSClient, error)` |
| `Disconnect()` | 主动断开连接 | `None` |
| `Reply(frame, body, cmd?)` | 通过 WebSocket 通道发送回复消息（通用方法） | `(*WsFrame, error)` |
| `ReplyStream(frame, streamID, content, finish, msgItem, feedback)` | 发送流式文本回复（支持 Markdown） | `(*WsFrame, error)` |
| `ReplyWelcome(frame, body)` | 发送欢迎语回复（支持文本或模板卡片） | `(*WsFrame, error)` |
| `ReplyTemplateCard(frame, templateCard, feedback?)` | 回复模板卡片消息 | `(*WsFrame, error)` |
| `ReplyStreamWithCard(...)` | 发送流式消息 + 模板卡片组合回复 | `(*WsFrame, error)` |
| `UpdateTemplateCard(frame, templateCard, userids?)` | 更新模板卡片 | `(*WsFrame, error)` |
| `SendMessage(chatid, body)` | 主动发送消息（支持 Markdown 或模板卡片） | `(*WsFrame, error)` |
| `DownloadFile(url, aesKey)` | 下载文件并使用 AES 密钥解密 | `([]byte, string, error)` |
| `Run()` | 便捷启动方法（阻塞运行） | `None` |
| `On(event, handler)` | 注册事件监听器 | `None` |
| `IsConnected()` | 获取当前连接状态 | `bool` |
| `API()` | 获取 API 客户端实例 | `*WeComApiClient` |

#### 事件列表

所有事件均通过 `client.On(event, handler)` 注册：

| 事件 | 回调参数 | 说明 |
| --- | --- | --- |
| `connected` | `nil` | WebSocket 连接建立 |
| `authenticated` | `nil` | 认证成功 |
| `disconnected` | `reason string` | 连接断开 |
| `reconnecting` | `attempt int` | 正在重连（第 N 次） |
| `error` | `error` | 发生错误 |
| `message` | `*WsFrame` | 收到消息（所有类型） |
| `message.text` | `*WsFrame` | 收到文本消息 |
| `message.image` | `*WsFrame` | 收到图片消息 |
| `message.mixed` | `*WsFrame` | 收到图文混排消息 |
| `message.voice` | `*WsFrame` | 收到语音消息 |
| `message.file` | `*WsFrame` | 收到文件消息 |
| `event` | `*WsFrame` | 收到事件回调（所有事件类型） |
| `event.enter_chat` | `*WsFrame` | 收到进入会话事件 |
| `event.template_card_event` | `*WsFrame` | 收到模板卡片事件 |
| `event.feedback_event` | `*WsFrame` | 收到用户反馈事件 |

### `ReplyStream` 详细说明

```go
frame, err := client.ReplyStream(
    frame,              // 收到的原始 WebSocket 帧（透传 req_id）
    streamID,           // 流式消息 ID（使用 GenerateReqID("stream") 生成）
    content,            // 回复内容（支持 Markdown）
    finish,             // 是否结束流式消息
    msgItem,            // 图文混排项（仅 finish=true 时有效）
    feedback,           // 反馈信息（仅首次回复时设置）
)
```

### `SendMessage` 详细说明

主动向指定会话推送消息，无需依赖收到的回调帧。

```go
// 发送 Markdown 消息
client.SendMessage("userid_or_chatid", map[string]interface{}{
    "msgtype": "markdown",
    "markdown": map[string]interface{}{
        "content": "这是一条**主动推送**的消息",
    },
})

// 发送模板卡片消息
client.SendMessage("userid_or_chatid", map[string]interface{}{
    "msgtype": "template_card",
    "template_card": map[string]interface{}{
        "card_type": "text_notice",
        "main_title": map[string]interface{}{"title": "通知"},
    },
})
```

### `DownloadFile` 使用示例

```go
client.On("message.image", func(data interface{}) {
    frame := data.(*aibot.WsFrame)
    body := frame.Body.(map[string]interface{})
    image := body["image"].(map[string]interface{})
    url := image["url"].(string)
    aesKey := image["aeskey"].(string)

    data, filename, err := client.DownloadFile(url, aesKey)
    if err != nil {
        log.Printf("下载失败：%v", err)
        return
    }

    // 保存文件
    os.WriteFile(filename, data, 0644)
})
```

## ⚙️ 配置选项

`WSClientOptions` 完整配置：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `BotID` | `string` | ✅ | — | 机器人 ID（企业微信后台获取） |
| `Secret` | `string` | ✅ | — | 机器人 Secret（企业微信后台获取） |
| `ReconnectInterval` | `int` | — | `1000` | 重连基础延迟（毫秒），实际延迟按指数退避递增 |
| `MaxReconnectAttempts` | `int` | — | `10` | 最大重连次数（`-1` 表示无限重连） |
| `HeartbeatInterval` | `int` | — | `30000` | 心跳间隔（毫秒） |
| `RequestTimeout` | `int` | — | `10000` | HTTP 请求超时时间（毫秒） |
| `WsURL` | `string` | — | `wss://openws.work.weixin.qq.com` | 自定义 WebSocket 连接地址 |
| `Logger` | `Logger` | — | `DefaultLogger` | 自定义日志实例 |

## 📋 消息类型

SDK 支持以下消息类型（`MessageType` 枚举）：

| 类型 | 值 | 说明 |
| --- | --- | --- |
| `MessageTypeText` | `"text"` | 文本消息 |
| `MessageTypeImage` | `"image"` | 图片消息 |
| `MessageTypeMixed` | `"mixed"` | 图文混排消息 |
| `MessageTypeVoice` | `"voice"` | 语音消息 |
| `MessageTypeFile` | `"file"` | 文件消息 |

SDK 支持以下事件类型（`EventType` 枚举）：

| 类型 | 值 | 说明 |
| --- | --- | --- |
| `EventTypeEnterChat` | `"enter_chat"` | 进入会话事件 |
| `EventTypeTemplateCardEvent` | `"template_card_event"` | 模板卡片事件 |
| `EventTypeFeedbackEvent` | `"feedback_event"` | 用户反馈事件 |

## 🪵 自定义日志

实现 `Logger` 接口即可自定义日志输出：

```go
type MyLogger struct{}

func (l *MyLogger) Debug(message string, args ...interface{}) {
    log.Printf("[DEBUG] "+message, args...)
}
func (l *MyLogger) Info(message string, args ...interface{}) {
    log.Printf("[INFO] "+message, args...)
}
func (l *MyLogger) Warn(message string, args ...interface{}) {
    log.Printf("[WARN] "+message, args...)
}
func (l *MyLogger) Error(message string, args ...interface{}) {
    log.Printf("[ERROR] "+message, args...)
}

client := aibot.NewWSClient(&aibot.WSClientOptions{
    BotID:  "your-bot-id",
    Secret: "your-bot-secret",
    Logger: &MyLogger{},
})
```

## 📂 项目结构

```
wecom-aibot-go-sdk/
├── aibot.go           # 包入口，导出公共类型
├── types.go           # 类型定义（枚举、配置、消息类型）
├── client.go          # WSClient 核心客户端
├── ws.go              # WebSocket 长连接管理器
├── message_handler.go # 消息解析与事件分发
├── api.go             # HTTP API 客户端（文件下载）
├── crypto.go          # AES-256-CBC 文件解密
├── logger.go          # 默认日志实现
├── utils.go           # 工具方法（GenerateReqID 等）
├── go.mod             # Go 模块配置
├── examples/
│   └── main.go        # 基础使用示例
├── README.md          # 本文件
└── .env.example       # 环境变量示例
```

## 📄 License

MIT
