package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type MessageContent struct {
	Text string `json:"text"`
}
type LarkService struct {
	appID       string
	appSecret   string
	larkClient  *lark.Client // 新增：用于发送消息的 API 客户端
	wsClient    *larkws.Client
	lastMessage string
	msgCount    int
	mu          sync.RWMutex
}

func NewLarkService(appID, appSecret string, ai *AIService) *LarkService {
	s := &LarkService{
		appID:      appID,
		appSecret:  appSecret,
		larkClient: lark.NewClient(appID, appSecret), // 初始化 API Client
	}
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			// 1. Parse message immediately
			msgID := *event.Event.Message.MessageId
			userID := *event.Event.Sender.SenderId.OpenId
			var content struct {
				Text string `json:"text"`
			}
			json.Unmarshal([]byte(*event.Event.Message.Content), &content)
			if content.Text == "/reset" {
				ai.ClearHistory(userID)
				s.reply(context.Background(), msgID, "已清除对话记忆，开启新聊天！")
				return nil
			}
			// 2. ASYNC: Handle AI logic in a separate goroutine to avoid Lark timeout (3s)
			go func() {
				// Use background context for async task
				fmt.Printf("[Lark] User asked: %s\n", content.Text)
				answer, err := ai.ChatWithMemory(userID, content.Text)
				if err != nil {
					s.reply(context.Background(), msgID, "❌ AI Service Error: "+err.Error())
					return
				}
				s.reply(context.Background(), msgID, answer)
			}()
			// 3. RETURN IMMEDIATELY: Tell Lark we received the event
			return nil
		})
	s.wsClient = larkws.NewClient(appID, appSecret, larkws.WithEventHandler(eventHandler))
	return s
}

// reply 处理具体的 API 调用
func (s *LarkService) reply(ctx context.Context, messageID string, text string) error {
	// 1. 构造消息内容 (必须是转义后的 JSON 字符串)
	// 飞书的文本格式是 {"text":"内容"}
	replyContent, _ := json.Marshal(map[string]string{
		"text": text,
	})
	// 2. 构建请求对象
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			Content(string(replyContent)).
			Build()).
		Build()
	// 3. 发起调用
	// 注意：在 Go SDK 中，Reply 挂在 Im.Message 下
	resp, err := s.larkClient.Im.Message.Reply(ctx, req)
	if err != nil {
		fmt.Printf("[Lark-WS] API 调用系统错误: %v\n", err)
		return err
	}
	if !resp.Success() {
		fmt.Printf("[Lark-WS] 回复失败，错误码: %d, 错误信息: %s, RequestID: %s\n",
			resp.Code, resp.Msg, resp.RequestId())
		return fmt.Errorf("lark reply failed: %s", resp.Msg)
	}
	fmt.Println("[Lark-WS] 回复发送成功！")
	return nil
}
func (s *LarkService) Name() string {
	return "Lark-WS-Service"
}
func (s *LarkService) Start(ctx context.Context) error {
	fmt.Println("[Lark-WS] 正在建立长连接...")
	// Start 会阻塞，直到连接断开，所以我们要放在协程里或处理好阻塞
	// 这里直接调用 Start，由 Manager 在协程中启动它
	return s.wsClient.Start(ctx)
}
func (s *LarkService) Stop() error {
	// 长连接会随着 context 取消自动关闭
	fmt.Println("Stop LarkService...")
	return nil
}
func (s *LarkService) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fmt.Sprintf("已接收: %d | 最新: %s", s.msgCount, s.lastMessage)
}
