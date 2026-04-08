package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WecomTeam/wecom-aibot-go-sdk"
)

func main() {
	// 加载环境变量（在实际使用中可以通过 godotenv 加载 .env 文件）
	botID := os.Getenv("WECHAT_BOT_ID")
	secret := os.Getenv("WECHAT_BOT_SECRET")

	if botID == "" || secret == "" {
		fmt.Fprintln(os.Stderr, "请设置环境变量：WECHAT_BOT_ID 和 WECHAT_BOT_SECRET")
		fmt.Fprintln(os.Stderr, "用法：set WECHAT_BOT_ID=your-bot-id && set WECHAT_BOT_SECRET=your-secret && go run main.go")
		os.Exit(1)
	}

	// 创建客户端实例
	client := aibot.NewWSClient(&aibot.WSClientOptions{
		BotID:   botID,
		Secret:  secret,
		Logger:  aibot.NewDefaultLogger(),
	})

	// 监听认证成功事件
	client.On("authenticated", func(data interface{}) {
		fmt.Println("认证成功")
	})

	// 监听 WebSocket 连接建立
	client.On("connected", func(data interface{}) {
		fmt.Println("WebSocket 已连接")
	})

	// 监听连接断开
	client.On("disconnected", func(data interface{}) {
		reason, _ := data.(string)
		fmt.Printf("连接已断开：%s\n", reason)
	})

	// 监听重连
	client.On("reconnecting", func(data interface{}) {
		attempt, _ := data.(int)
		fmt.Printf("正在进行第 %d 次重连...\n", attempt)
	})

	// 监听错误
	client.On("error", func(data interface{}) {
		err, _ := data.(error)
		fmt.Printf("发生错误：%v\n", err)
	})

	// 监听所有消息
	client.On("message", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		fmt.Printf("收到消息：%v\n", frame.Body)
	})

	// 监听文本消息并进行流式回复
	client.On("message.text", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		body, _ := frame.Body.(map[string]interface{})
		textMap, _ := body["text"].(map[string]interface{})
		content, _ := textMap["content"].(string)

		fmt.Printf("收到文本消息：%s\n", content)

		// 生成流式消息 ID
		streamID := aibot.GenerateReqID("stream")

		// 发送流式中间内容
		go func() {
			_, err := client.ReplyStream(frame, streamID, "正在思考中...", false, nil, nil)
			if err != nil {
				fmt.Printf("发送流式消息失败：%v\n", err)
				return
			}

			// 模拟异步处理
			time.Sleep(1 * time.Second)

			// 发送最终结果
			_, err = client.ReplyStream(frame, streamID, fmt.Sprintf("你好！你说的是：\"%s\"", content), true, nil, nil)
			if err != nil {
				fmt.Printf("发送流式消息失败：%v\n", err)
			}
		}()
	})

	// 监听进入会话事件（发送欢迎语）
	client.On("event.enter_chat", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		fmt.Println("用户进入会话")

		go func() {
			_, err := client.ReplyWelcome(frame, map[string]interface{}{
				"msgtype": "text",
				"text": map[string]interface{}{
					"content": "您好！我是智能助手，有什么可以帮您的吗？",
				},
			})
			if err != nil {
				fmt.Printf("发送欢迎语失败：%v\n", err)
			}
		}()
	})

	// 监听图片消息
	client.On("message.image", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		body, _ := frame.Body.(map[string]interface{})
		imageMap, _ := body["image"].(map[string]interface{})
		url, _ := imageMap["url"].(string)
		aesKey, _ := imageMap["aeskey"].(string)

		fmt.Printf("收到图片消息：%s\n", url)

		go func() {
			data, filename, err := client.DownloadFile(url, aesKey)
			if err != nil {
				fmt.Printf("图片下载失败：%v\n", err)
				return
			}
			fmt.Printf("图片下载成功，大小：%d bytes, 文件名：%s\n", len(data), filename)
		}()
	})

	// 监听文件消息
	client.On("message.file", func(data interface{}) {
		fmt.Println("收到文件消息")
	})

	// 启动客户端
	fmt.Println("正在启动企业微信机器人...")

	// 建立连接
	_, err := client.Connect()
	if err != nil {
		fmt.Printf("连接失败：%v\n", err)
		os.Exit(1)
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("按 Ctrl+C 退出")
	<-sigChan

	fmt.Println("\n正在停止机器人...")
	client.Disconnect()
	fmt.Println("机器人已停止")
}
