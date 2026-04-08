package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/WecomTeam/wecom-aibot-go-sdk"
)

func main() {
	// 加载 .env 文件
	if err := godotenv.Load(".env"); err != nil {
		fmt.Fprintln(os.Stderr, "加载 .env 文件失败:", err)
		fmt.Fprintln(os.Stderr, "请确保 .env 文件存在于当前目录")
		os.Exit(1)
	}

	botID := os.Getenv("WECHAT_BOT_ID")
	secret := os.Getenv("WECHAT_BOT_SECRET")

	if botID == "" || secret == "" {
		fmt.Fprintln(os.Stderr, "请设置环境变量：WECHAT_BOT_ID 和 WECHAT_BOT_SECRET")
		os.Exit(1)
	}

	fmt.Println("🤖 企业微信机器人 Echo 测试")
	fmt.Println("BotID:", botID)
	fmt.Println("Secret:", secret[:10]+"...")
	fmt.Println()

	// 创建客户端实例
	client := aibot.NewWSClient(&aibot.WSClientOptions{
		BotID:  botID,
		Secret: secret,
		Logger: aibot.NewDefaultLogger(),
	})

	// 监听认证成功事件
	client.On("authenticated", func(data interface{}) {
		fmt.Println("✅ 认证成功")
	})

	// 监听 WebSocket 连接建立
	client.On("connected", func(data interface{}) {
		fmt.Println("🔗 WebSocket 已连接")
	})

	// 监听连接断开
	client.On("disconnected", func(data interface{}) {
		reason, _ := data.(string)
		fmt.Printf("❌ 连接已断开：%s\n", reason)
	})

	// 监听错误
	client.On("error", func(data interface{}) {
		err, _ := data.(error)
		fmt.Printf("⚠️  发生错误：%v\n", err)
	})

	// 监听所有消息
	client.On("message", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		fmt.Printf("📨 收到消息：%v\n", frame.Body)
	})

	// 监听文本消息并 Echo 回复
	client.On("message.text", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		body, _ := frame.Body.(map[string]interface{})
		textMap, _ := body["text"].(map[string]interface{})
		content, _ := textMap["content"].(string)

		fmt.Printf("💬 收到文本消息：%s\n", content)
		fmt.Println("🔁 开始 Echo 回复...")

		// 直接回复原文
		_, err := client.Reply(frame, map[string]interface{}{
			"msgtype": "text",
			"text": map[string]interface{}{
				"content": fmt.Sprintf("[Echo] 你说的是：%s", content),
			},
		})
		if err != nil {
			fmt.Printf("⚠️  回复失败：%v\n", err)
		} else {
			fmt.Println("✅ Echo 回复成功")
		}
	})

	// 监听进入会话事件（发送欢迎语）
	client.On("event.enter_chat", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		fmt.Println("👋 用户进入会话")

		_, err := client.ReplyWelcome(frame, map[string]interface{}{
			"msgtype": "text",
			"text": map[string]interface{}{
				"content": "您好！我是 Echo 测试机器人，您发送的消息我会原样回复。\n发送任意消息开始测试！",
			},
		})
		if err != nil {
			fmt.Printf("⚠️  发送欢迎语失败：%v\n", err)
		}
	})

	// 启动客户端
	fmt.Println("正在启动企业微信机器人...")
	fmt.Println()

	// 建立连接
	_, err := client.Connect()
	if err != nil {
		fmt.Printf("❌ 连接失败：%v\n", err)
		os.Exit(1)
	}

	// 等待中断信号（使用 simpler 的方式）
	fmt.Println("按 Ctrl+C 退出")
	fmt.Println()

	// 使用 select 阻塞等待
	select {}
}
