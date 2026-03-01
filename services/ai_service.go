package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type AIService struct {
	apiKey string
	apiURL string
	model  string
	client *http.Client
	// Memory: Key is UserID (OpenID), Value is the conversation history
	history    map[string][]Message
	historyMu  sync.RWMutex
	maxHistory int // Limit the number of messages to save tokens
}
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewAIService() *AIService {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	apiURL := os.Getenv("BASE_URL")
	model := os.Getenv("MODEL_ID")
	maxHistory := 10

	// --- Debug Info Start ---
	log.Println("[AI-Init] Checking environment variables...")

	if apiKey == "" {
		log.Println("[AI-Init] ❌ WARNING: DASHSCOPE_API_KEY is empty!")
	} else {
		// Only print length and first 4 chars for security
		maskKey := ""
		if len(apiKey) > 4 {
			maskKey = apiKey[:4] + "****"
		}
		log.Printf("[AI-Init] ✅ API Key detected: %s (Total length: %d)", maskKey, len(apiKey))
	}

	if apiURL == "" {
		log.Println("[AI-Init] ❌ WARNING: BASE_URL is empty! Requests will fail.")
	} else {
		log.Printf("[AI-Init] ✅ API URL: %s", apiURL)
	}

	if model == "" {
		log.Println("[AI-Init] ❌ WARNING: MODEL_ID is empty!")
	} else {
		log.Printf("[AI-Init] ✅ Model ID: %s", model)
	}

	log.Printf("[AI-Init] Configuration: MaxHistory=%d, Timeout=60s", maxHistory)
	// --- Debug Info End ---

	return &AIService{
		apiKey:     apiKey,
		apiURL:     apiURL,
		model:      model,
		client:     &http.Client{Timeout: 60 * time.Second},
		history:    make(map[string][]Message),
		maxHistory: maxHistory,
	}
}

// Chat sends user input to the LLM and returns the generated response
func (s *AIService) Chat(userInput string) (string, error) {
	requestBody := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a professional assistant."},
			{"role": "user", "content": userInput},
		},
	}
	jsonData, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("AI response empty or error: %v", result.Error)
}

func (s *AIService) ChatWithMemory(userID string, userInput string) (string, error) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	// 1. 获取或初始化历史记录
	msgs := s.history[userID]
	if len(msgs) == 0 {
		msgs = append(msgs, Message{Role: "system", Content: "You are a helpful assistant."})
	}
	msgs = append(msgs, Message{Role: "user", Content: userInput})

	// 2. 准备请求
	requestBody := map[string]interface{}{
		"model":    s.model,
		"messages": msgs,
	}
	jsonData, _ := json.Marshal(requestBody)

	// 打印发送的信息（调试用）
	log.Printf("[AI-Debug] Sending request for User: %s, Messages Count: %d", userID, len(msgs))

	req, _ := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[AI-Error] Request failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	// 3. 读取原始响应体以备调试
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		log.Printf("[AI-Error] Status Code: %d, Body: %s", resp.StatusCode, string(bodyBytes))
		return "", fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	// 4. 解析结果
	var result struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		// 增加错误字段捕获
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		log.Printf("[AI-Error] JSON Unmarshal failed: %v, Raw Body: %s", err, string(bodyBytes))
		return "", err
	}

	// 5. 检查 Choice 逻辑
	if len(result.Choices) > 0 {
		aiMsg := result.Choices[0].Message

		// 保存到历史记录
		msgs = append(msgs, aiMsg)

		// 滑动窗口
		if len(msgs) > s.maxHistory {
			// 保留 System Prompt 并截断中间的消息
			systemPrompt := msgs[0]
			newHistory := append([]Message{systemPrompt}, msgs[len(msgs)-s.maxHistory+1:]...)
			msgs = newHistory
		}

		s.history[userID] = msgs
		return aiMsg.Content, nil
	}

	// 如果没有 Choice 但有 Error 信息
	if result.Error.Message != "" {
		return "", fmt.Errorf("AI API Error: %s (Type: %s)", result.Error.Message, result.Error.Type)
	}

	return "", fmt.Errorf("AI empty response, raw body: %s", string(bodyBytes))
}

func (s *AIService) ClearHistory(userID string) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	delete(s.history, userID)
}

// These satisfy the core.Service interface if you want to register it as a service
func (s *AIService) Name() string                    { return "AI-Core-Service" }
func (s *AIService) Start(ctx context.Context) error { return nil }
func (s *AIService) Stop() error                     { return nil }
func (s *AIService) Status() string                  { return "Ready" }
