package services

import (
	"ai-platform/model"
	"ai-platform/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type MessageContent struct {
	Text string `json:"text"`
}

type ShotMemoryUpdate struct {
	UserID   string `json:"user_id"`
	Message  string `json:"message"`
	Response string `json:"response"`
}
type CoreMemoryUpdate struct {
	UserID     string `json:"user_id"`
	CoreMemory string `json:"core_memory"`
}
type TaskMemoryUpdate struct {
	UserID           string    `json:"user_id"`
	TaskDescription  string    `json:"task_description"`
	DueDate          time.Time `json:"due_date"`
	Status           string    `json:"status"`
	Progress         int       `json:"progress"`
	NextTriggerEvent string    `json:"next_trigger_event"`
}
type FinalPayload struct {
	Reply      string            `json:"reply"`
	ShotMemory *ShotMemoryUpdate `json:"shot_memory,omitempty"`
	CoreMemory *CoreMemoryUpdate `json:"core_memory,omitempty"`
	TaskMemory *TaskMemoryUpdate `json:"task_memory,omitempty"`
	Skills     []string          `json:"skills,omitempty"`
	Agents     []string          `json:"agents,omitempty"`
	Tools      []string          `json:"tools,omitempty"`
}
type LarkService struct {
	appID       string
	appSecret   string
	larkClient  *lark.Client // 新增：用于发送消息的 API 客户端
	wsClient    *larkws.Client
	lastMessage string
	msgCount    int
	mu          sync.RWMutex
	dbClient    *utils.SQLiteClient
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
			chatID := ""
			if event.Event.Message.ChatId != nil {
				chatID = *event.Event.Message.ChatId
			}
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
				if aiDebugEnabled() {
					fmt.Printf("[Lark-Debug] msg_id=%s user_id=%s chat_id=%s text=%s\n", msgID, userID, chatID, content.Text)
				}

				ir, ierr := ai.AnalyzeIntent(content.Text)
				if aiDebugEnabled() && ierr == nil && ir != nil {
					b, _ := json.Marshal(ir)
					fmt.Printf("[Lark-Debug] intent=%s\n", string(b))
				}

				now := time.Now()
				if ierr == nil && ir != nil {
					if len(ir.Skills) > 0 {
						for _, sk := range ir.Skills {
							_ = upsertByName(s.dbClient, "skills", "skill_name", sk, []utils.FieldData{
								{Name: "skill_name", Value: sk},
								{Name: "skill_description", Value: ""},
								{Name: "restricted_scenarios", Value: ""},
								{Name: "category", Value: "ai_inferred"},
								{Name: "tags", Value: ""},
								{Name: "source", Value: "ai_intent"},
								{Name: "version", Value: "1"},
								{Name: "expires_at", Value: nil},
								{Name: "path", Value: ""},
								{Name: "entrypoint", Value: ""},
								{Name: "enabled", Value: 1},
								{Name: "created_at", Value: now},
								{Name: "updated_at", Value: now},
							})
						}
					}
					if len(ir.Tools) > 0 {
						executableBuiltins := map[string]bool{
							"web_search":    true,
							"weather_query": true,
							"sh":            true,
							"curl":          true,
							"wget":          true,
							"cat":           true,
							"browser":       true,
						}
						for _, tl := range ir.Tools {
							enabled := 0
							if executableBuiltins[strings.ToLower(strings.TrimSpace(tl))] {
								enabled = 1
							}
							_ = upsertByName(s.dbClient, "tools", "tool_name", tl, []utils.FieldData{
								{Name: "tool_name", Value: tl},
								{Name: "usage_instructions", Value: ""},
								{Name: "usage_restrictions", Value: ""},
								{Name: "category", Value: "ai_inferred"},
								{Name: "tags", Value: ""},
								{Name: "source", Value: "ai_intent"},
								{Name: "version", Value: "1"},
								{Name: "expires_at", Value: nil},
								{Name: "path", Value: ""},
								{Name: "entrypoint", Value: ""},
								{Name: "enabled", Value: enabled},
								{Name: "created_at", Value: now},
								{Name: "updated_at", Value: now},
							})
						}
					}
					if len(ir.MCP) > 0 {
						for _, ag := range ir.MCP {
							_ = upsertByName(s.dbClient, "agents", "agent_name", ag, []utils.FieldData{
								{Name: "agent_name", Value: ag},
								{Name: "agent_description", Value: ""},
								{Name: "capabilities", Value: ""},
								{Name: "category", Value: "ai_inferred"},
								{Name: "tags", Value: ""},
								{Name: "source", Value: "ai_intent"},
								{Name: "version", Value: "1"},
								{Name: "expires_at", Value: nil},
								{Name: "path", Value: ""},
								{Name: "entrypoint", Value: ""},
								{Name: "enabled", Value: 1},
								{Name: "created_at", Value: now},
								{Name: "updated_at", Value: now},
							})
						}
					}
					if len(ir.DocTypes) > 0 {
						_ = s.dbClient.InsertData("bootstrap", []utils.FieldData{
							{Name: "context", Value: strings.Join(ir.DocTypes, ",")},
							{Name: "risks", Value: ""},
							{Name: "notes", Value: ""},
						})
					}
				}

				workspaceDir := os.Getenv("AI_WORKSPACE_DIR")
				if workspaceDir == "" {
					workspaceDir = "/aiworkspace"
				}
				var answer string
				var err error
				if ierr == nil && ir != nil && len(ir.Tools) == 0 {
					answer, err = ai.RunAgentWithAutoAcquire(context.Background(), userID, content.Text, s.dbClient, workspaceDir)
				} else {
					answer, err = ai.RunAgent(context.Background(), userID, content.Text, s.dbClient, workspaceDir)
				}
				if err != nil {
					s.reply(context.Background(), msgID, "❌ AI Service Error: "+err.Error())
					return
				}
				_ = s.dbClient.InsertData("shot_memory", []utils.FieldData{
					{Name: "user_id", Value: userID},
					{Name: "message", Value: content.Text},
					{Name: "response", Value: answer},
					{Name: "message_id", Value: msgID},
					{Name: "chat_id", Value: chatID},
					{Name: "channel", Value: "lark"},
				})
				_ = ai.MaybeUpdateCoreMemory(context.Background(), userID, s.dbClient)
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
	// 连接数据库
	dbClient, err := utils.OpenSQLite("/data/ai_brain.db", "")
	if err != nil {
		return fmt.Errorf("failed to connect to SQLite: %v", err)
	}
	s.dbClient = dbClient

	// 创建所需的所有表
	model.CreateShotMemoryTable(dbClient)
	model.CreateCoreMemoryTable(dbClient)
	model.CreateTaskMemoryTable(dbClient)
	model.CreateAgentsTable(dbClient)
	model.CreateSkillsTable(dbClient)
	model.CreateBootstrapTable(dbClient)
	model.CreateHeadbeatTable(dbClient)
	model.CreateIdentityTable(dbClient)
	model.CreateSoulTable(dbClient)
	model.CreateToolsTable(dbClient)
	model.CreateUserTable(dbClient)
	model.CreateToolRunsTable(dbClient)

	if err := model.EnsureAIBrainSchema(dbClient); err != nil {
		return err
	}
	if err := EnsureBuiltInTools(dbClient); err != nil {
		return err
	}
	_ = LoadAistoreIntoDB(dbClient, os.Getenv("AISTORE_DIR"))

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

func needsWebSearch(ir *IntentResult) bool {
	for _, t := range ir.Tools {
		if strings.EqualFold(t, "web_search") || strings.EqualFold(t, "web") || strings.EqualFold(t, "internet") {
			return true
		}
	}
	for _, dt := range ir.DocTypes {
		if strings.EqualFold(dt, "web") || strings.EqualFold(dt, "internet") || strings.EqualFold(dt, "online") {
			return true
		}
	}
	intentLower := strings.ToLower(ir.Intent)
	if strings.Contains(intentLower, "search") || strings.Contains(intentLower, "web") || strings.Contains(ir.Intent, "搜索") {
		return true
	}
	return false
}
