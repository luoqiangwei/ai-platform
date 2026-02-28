package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	return &AIService{
		// Fetch API Key from environment variables
		apiKey:     os.Getenv("DASHSCOPE_API_KEY"),
		apiURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		model:      "qwen-plus",
		client:     &http.Client{Timeout: 60 * time.Second},
		history:    make(map[string][]Message),
		maxHistory: 10, // Keep last 10 messages
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

	// 1. Get or initialize history
	msgs := s.history[userID]
	if len(msgs) == 0 {
		msgs = append(msgs, Message{Role: "system", Content: "You are a helpful assistant."})
	}

	// 2. Append new user message
	msgs = append(msgs, Message{Role: "user", Content: userInput})

	// 3. Prepare request body
	requestBody := map[string]interface{}{
		"model":    s.model,
		"messages": msgs,
	}

	jsonData, _ := json.Marshal(requestBody)
	req, _ := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Choices) > 0 {
		aiMsg := result.Choices[0].Message
		// 4. Save AI's response to history
		msgs = append(msgs, aiMsg)

		// 5. Sliding window: trim history if it's too long
		if len(msgs) > s.maxHistory {
			// Keep system prompt + the latest N messages
			msgs = append([]Message{msgs[0]}, msgs[len(msgs)-s.maxHistory:]...)
		}
		s.history[userID] = msgs

		return aiMsg.Content, nil
	}

	return "", fmt.Errorf("AI empty response")
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
