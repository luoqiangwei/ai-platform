package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
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

type IntentResult struct {
	Intent   string   `json:"intent"`
	MCP      []string `json:"mcp"`
	Tools    []string `json:"tools"`
	Skills   []string `json:"skills"`
	DocTypes []string `json:"doc_types"`
}

type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func aiDebugEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("AI_DEBUG")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func aiDebugMaxChars() int {
	if v := strings.TrimSpace(os.Getenv("AI_DEBUG_MAX_CHARS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 12000
}

func truncateForDebug(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
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
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] Chat request body: %s", truncateForDebug(string(jsonData), aiDebugMaxChars()))
	}
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
	raw, _ := io.ReadAll(resp.Body)
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] Chat response status=%d body=%s", resp.StatusCode, truncateForDebug(string(raw), aiDebugMaxChars()))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error map[string]interface{} `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("AI response empty or error: %v", result.Error)
}

func (s *AIService) WebSearch(query string, limit int) ([]WebSearchResult, error) {
	endpoint := os.Getenv("WEB_SEARCH_ENDPOINT")
	if endpoint == "" {
		return s.duckDuckGoSearch(query, limit)
	}
	// TODO: 可以增加对不同搜索引擎的适配逻辑，目前只实现了 Google 的解析，其他搜索引擎可能需要不同的解析方式
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = q.Encode()

	if aiDebugEnabled() {
		log.Printf("[AI-Debug] web_search GET %s", u.String())
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	if apiKey := os.Getenv("WEB_SEARCH_API_KEY"); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] web_search response status=%d body=%s", resp.StatusCode, truncateForDebug(string(body), aiDebugMaxChars()))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("web_search status=%d body=%s", resp.StatusCode, string(body))
	}

	// var asObj struct {
	// 	Results []WebSearchResult `json:"results"`
	// }
	// if err := json.Unmarshal(body, &asObj); err == nil && len(asObj.Results) > 0 {
	// 	return asObj.Results, nil
	// }

	// var asArr []WebSearchResult
	// if err := json.Unmarshal(body, &asArr); err == nil && len(asArr) > 0 {
	// 	return asArr, nil
	// }

	// return []WebSearchResult{}, nil
	return parseHTML(string(body), limit), nil
}

func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}

	var result string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result += extractText(c) + " "
	}
	return strings.TrimSpace(result)
}

func findSnippet(n *html.Node) string {
	if n == nil {
		return ""
	}

	var f func(*html.Node) string
	f = func(node *html.Node) string {
		if node.Type == html.ElementNode && node.Data == "div" {
			for _, attr := range node.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "VwiC3b") {
					return extractText(node)
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if text := f(c); text != "" {
				return text
			}
		}
		return ""
	}

	return f(n)
}

func parseHTML(htmlContent string, limit int) []WebSearchResult {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil
	}

	var results []WebSearchResult

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "h3" {
			title := extractText(n)

			// 找父节点里最近的 snippet
			snippet := findSnippet(n.Parent)

			if title != "" {
				results = append(results, WebSearchResult{
					Title:   title,
					Snippet: snippet,
				})
			}

			if len(results) >= limit {
				return
			}
		}

		for c := n.FirstChild; c != nil && len(results) < limit; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)

	return results
}

func (s *AIService) duckDuckGoSearch(query string, limit int) ([]WebSearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return []WebSearchResult{}, nil
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	u := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] web_search fallback GET %s", u)
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "ai-platform/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 800*1024))
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] web_search fallback response status=%d body=%s", resp.StatusCode, truncateForDebug(body, aiDebugMaxChars()))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("web_search fallback status=%d", resp.StatusCode)
	}
	return parseDuckDuckGoHTML(body, limit), nil
}

func parseDuckDuckGoHTML(body string, limit int) []WebSearchResult {
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	reLinkA := regexp.MustCompile(`(?is)<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	reLinkB := regexp.MustCompile(`(?is)<a[^>]+class="result-link"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	reTag := regexp.MustCompile(`(?is)<[^>]+>`)

	var matches [][]string
	matches = reLinkA.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		matches = reLinkB.FindAllStringSubmatch(body, -1)
	}
	var out []WebSearchResult
	seen := map[string]bool{}
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		link := strings.TrimSpace(html.UnescapeString(m[1]))
		title := strings.TrimSpace(html.UnescapeString(reTag.ReplaceAllString(m[2], "")))
		if link == "" || title == "" {
			continue
		}
		if resolved := resolveDuckDuckGoRedirect(link); resolved != "" {
			link = resolved
		}
		key := strings.ToLower(link)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, WebSearchResult{Title: title, URL: link, Snippet: ""})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func resolveDuckDuckGoRedirect(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if !strings.Contains(strings.ToLower(u.Host), "duckduckgo.com") {
		return ""
	}
	q := u.Query()
	uddg := q.Get("uddg")
	if uddg == "" {
		return ""
	}
	if v, err := url.QueryUnescape(uddg); err == nil && strings.TrimSpace(v) != "" {
		return v
	}
	return ""
}

func (s *AIService) AnalyzeIntent(userInput string) (*IntentResult, error) {
	requestBody := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": "Analyze the user's message and output a compact JSON with keys: intent (string), mcp (string array), tools (string array), skills (string array), doc_types (string array). Output JSON only.\n\nConstraints:\n- tools must be chosen only from: [\"web_search\",\"weather_query\",\"sh\",\"curl\",\"wget\",\"cat\",\"browser\"].\n- If no suitable tool exists, output tools as []. Do not invent tool names. If a tool returns irrelevant information or the same result multiple times, do not retry the same query. Instead, try a different search query or explain the difficulty to the user."},
			{"role": "user", "content": userInput},
		},
	}
	jsonData, _ := json.Marshal(requestBody)
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] AnalyzeIntent request body: %s", truncateForDebug(string(jsonData), aiDebugMaxChars()))
	}
	req, err := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] AnalyzeIntent response status=%d body=%s", resp.StatusCode, truncateForDebug(string(raw), aiDebugMaxChars()))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return &IntentResult{Intent: "", MCP: []string{}, Tools: []string{}, Skills: []string{}, DocTypes: []string{}}, nil
	}
	content := result.Choices[0].Message.Content
	var ir IntentResult
	if err := json.Unmarshal([]byte(content), &ir); err != nil {
		start := -1
		end := -1
		for i := 0; i < len(content); i++ {
			if content[i] == '{' {
				start = i
				break
			}
		}
		for j := len(content) - 1; j >= 0; j-- {
			if content[j] == '}' {
				end = j
				break
			}
		}
		if start >= 0 && end >= 0 && end > start {
			if err := json.Unmarshal([]byte(content[start:end+1]), &ir); err != nil {
				ir = IntentResult{Intent: "", MCP: []string{}, Tools: []string{}, Skills: []string{}, DocTypes: []string{}}
			}
		} else {
			ir = IntentResult{Intent: "", MCP: []string{}, Tools: []string{}, Skills: []string{}, DocTypes: []string{}}
		}
	}
	return &ir, nil
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

	if aiDebugEnabled() {
		log.Printf("[AI-Debug] ChatWithMemory user=%s messages=%d body=%s", userID, len(msgs), truncateForDebug(string(jsonData), aiDebugMaxChars()))
	}

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
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] ChatWithMemory response body=%s", truncateForDebug(string(bodyBytes), aiDebugMaxChars()))
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
