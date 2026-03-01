package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

type AIService struct {
	apiURL  string
	modelID string
	client  *http.Client

	// We don't store message history anymore. OpenClaw handles that!
	// We only store a session version for each user to implement the "/reset" feature.
	sessionVersions map[string]int64
	mu              sync.RWMutex
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewAIService() *AIService {
	// Use the OpenClaw Gateway URL from the environment (e.g., http://openclaw:9090)
	// Fallback to localhost if running outside Docker
	openclawURL := os.Getenv("OPENCLAW_URL")
	if openclawURL == "" {
		openclawURL = "http://localhost:9090"
	}

	modelID := os.Getenv("OPENCLAW_MODEL_ID")
	if modelID == "" {
		modelID = "default/qwen-flash"
	}

	fmt.Printf("[AIService] Initializing with OpenClaw Gateway at %s, model: %s\n", openclawURL, modelID)

	return &AIService{
		apiURL:          openclawURL + "/v1/chat/completions",
		client:          &http.Client{Timeout: 120 * time.Second}, // Agent execution takes time
		sessionVersions: make(map[string]int64),
	}
}

// AskOpenClaw sends the task to the OpenClaw gateway
func (s *AIService) AskOpenClaw(userID string, userInput string, chatType string) (string, error) {
	// 1. Generate a unique session ID for the user based on their reset version
	s.mu.RLock()
	version := s.sessionVersions[userID]
	s.mu.RUnlock()

	// Example: "ou_12345_1700000000"
	sessionID := fmt.Sprintf("%s_%d", userID, version)

	// 2. Add context to the user input so OpenClaw knows the environment
	enrichedInput := fmt.Sprintf("[Source: %s]\n%s", chatType, userInput)

	// 3. Build the payload. OpenClaw uses the "user" field for memory isolation.
	requestBody := map[string]interface{}{
		"model": s.modelID, // OpenClaw routes this to whatever model is configured in its config.yaml
		"messages": []Message{
			{Role: "user", Content: enrichedInput},
		},
		"user": sessionID, // Critical: Tells OpenClaw which memory vault to use
	}

	jsonData, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("[AIService] Failed to create request: %v\n", err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	// OpenClaw might require a local auth token if configured, otherwise it ignores this
	req.Header.Set("Authorization", "Bearer openclaw-local-token")

	resp, err := s.client.Do(req)
	if err != nil {
		fmt.Printf("[AIService] Request to OpenClaw failed: %v\n", err)
		return "", fmt.Errorf("failed to reach OpenClaw: %w", err)
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

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("OpenClaw Raw Response: %s\n", string(bodyBytes))
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		fmt.Printf("[AIService] Failed to decode OpenClaw response: %v\n", err)
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("OpenClaw error: %v", result.Error)
}

// ResetUserSession forces OpenClaw to start a new memory session for the user
func (s *AIService) ResetUserSession(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Update the version to current Unix time, effectively starting a new session ID
	s.sessionVersions[userID] = time.Now().Unix()
}

// Interface compliance
func (s *AIService) Name() string                    { return "OpenClaw-Gateway-Service" }
func (s *AIService) Start(ctx context.Context) error { return nil }
func (s *AIService) Stop() error                     { return nil }
func (s *AIService) Status() string                  { return "Delegating AI to OpenClaw" }
