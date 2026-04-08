package main

import (
	"fmt"
	"os"
	"path/filepath"

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
		fmt.Printf("📨 完整帧：cmd=%v, headers=%v, body=%v\n", frame.Cmd, frame.Headers, frame.Body)
	})

	// 监听文本消息并 Echo 回复（使用流式消息格式）
	client.On("message.text", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		body, _ := frame.Body.(map[string]interface{})

		// 获取发信人信息
		from, _ := body["from"].(map[string]interface{})
		userid, _ := from["userid"].(string)
		conversationID, _ := body["conversation_id"].(string)
		chatType, _ := body["chattype"].(string)

		textMap, _ := body["text"].(map[string]interface{})
		content, _ := textMap["content"].(string)

		fmt.Printf("💬 收到文本消息 | 发信人：%s | 会话：%s | 聊天类型：%s\n", userid, conversationID, chatType)
		fmt.Printf("   消息内容：%s\n", content)
		fmt.Println("🔁 开始 Echo 回复（流式消息）...")

		// 生成流式消息 ID
		streamID := aibot.GenerateReqID("stream")

		// 使用流式消息回复（finish=true 表示立即完成）
		_, err := client.ReplyStream(frame, streamID, fmt.Sprintf("[Echo] 你说的是：%s", content), true, nil, nil)
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
				"content": "您好！我是 Echo 测试机器人，支持测试：\n1. 文本消息 - Echo 回复\n2. 图片消息 - 下载并保存\n3. 文件消息 - 下载并保存\n\n发送任意消息开始测试！",
			},
		})
		if err != nil {
			fmt.Printf("⚠️  发送欢迎语失败：%v\n", err)
		}
	})

	// 监听图片消息并下载
	client.On("message.image", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		body, _ := frame.Body.(map[string]interface{})

		// 获取发信人信息
		from, _ := body["from"].(map[string]interface{})
		userid, _ := from["userid"].(string)
		conversationID, _ := body["conversation_id"].(string)

		imageMap, _ := body["image"].(map[string]interface{})
		url, _ := imageMap["url"].(string)
		aesKey, _ := imageMap["aeskey"].(string)

		fmt.Printf("🖼️  收到图片消息 | 发信人：%s | 会话：%s\n", userid, conversationID)
		fmt.Printf("   图片 URL: %s\n", url)
		fmt.Println("⬇️  开始下载图片...")

		go func() {
			data, filename, err := client.DownloadFile(url, aesKey)
			if err != nil {
				fmt.Printf("⚠️  图片下载失败：%v\n", err)
				return
			}

			// 保存图片到当前目录
			savePath := filepath.Join(".", "downloaded_"+filename)
			if err := os.WriteFile(savePath, data, 0644); err != nil {
				fmt.Printf("⚠️  保存文件失败：%v\n", err)
				return
			}

			fmt.Printf("✅ 图片下载成功！大小：%d bytes, 保存到：%s\n", len(data), savePath)

			// 回复确认
			streamID := aibot.GenerateReqID("stream")
			client.ReplyStream(frame, streamID, fmt.Sprintf("✅ 图片已收到并保存：%s (大小：%d KB)", filename, len(data)/1024), true, nil, nil)
		}()
	})

	// 监听文件消息并下载
	client.On("message.file", func(data interface{}) {
		frame, _ := data.(*aibot.WsFrame)
		body, _ := frame.Body.(map[string]interface{})

		// 获取发信人信息
		from, _ := body["from"].(map[string]interface{})
		userid, _ := from["userid"].(string)
		conversationID, _ := body["conversation_id"].(string)

		fileMap, _ := body["file"].(map[string]interface{})
		url, _ := fileMap["url"].(string)
		aesKey, _ := fileMap["aeskey"].(string)

		fmt.Printf("📎 收到文件消息 | 发信人：%s | 会话：%s\n", userid, conversationID)
		fmt.Printf("   文件 URL: %s\n", url)
		fmt.Println("⬇️  开始下载文件...")

		go func() {
			data, filename, err := client.DownloadFile(url, aesKey)
			if err != nil {
				fmt.Printf("⚠️  文件下载失败：%v\n", err)
				return
			}

			// 保存文件到当前目录
			savePath := filepath.Join(".", "downloaded_"+filename)
			if err := os.WriteFile(savePath, data, 0644); err != nil {
				fmt.Printf("⚠️  保存文件失败：%v\n", err)
				return
			}

			fmt.Printf("✅ 文件下载成功！大小：%d bytes, 保存到：%s\n", len(data), savePath)

			// 回复确认
			streamID := aibot.GenerateReqID("stream")
			client.ReplyStream(frame, streamID, fmt.Sprintf("✅ 文件已收到并保存：%s (大小：%d KB)", filename, len(data)/1024), true, nil, nil)
		}()
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
