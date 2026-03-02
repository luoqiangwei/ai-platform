package services

import (
	"ai-platform/utils"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AgentToolCall struct {
	Tool         string   `json:"tool"`
	Command      string   `json:"command"`
	Args         []string `json:"args"`
	Path         string   `json:"path"`
	Query        string   `json:"query"`
	Limit        int      `json:"limit"`
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Kind         string   `json:"kind"`
	Entrypoint   string   `json:"entrypoint"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	Source       string   `json:"source"`
	Version      string   `json:"version"`
	Instructions string   `json:"usage_instructions"`
	Restrictions string   `json:"usage_restrictions"`
}

type AgentOutput struct {
	Mode      string          `json:"mode"`
	ToolCalls []AgentToolCall `json:"tool_calls"`
	Final     string          `json:"final"`
}

type AgentToolResult struct {
	Tool     string `json:"tool"`
	Command  string `json:"command"`
	Args     string `json:"args"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func (s *AIService) RunAgent(ctx context.Context, userID string, userInput string, dbClient *utils.SQLiteClient, workspace string) (string, error) {
	parts := splitUserMessage(userInput)
	if aiDebugEnabled() {
		log.Printf("[Agent-Debug] Split parts=%d", len(parts))
		for i, p := range parts {
			log.Printf("[Agent-Debug] Part[%d]=%s", i, truncateForDebug(p, aiDebugMaxChars()))
		}
	}

	var replies []string
	for _, part := range parts {
		reply, err := s.runAgentSingle(ctx, userID, part, dbClient, workspace)
		if err != nil {
			return "", err
		}
		replies = append(replies, reply)
	}
	return strings.Join(replies, "\n\n"), nil
}

func splitUserMessage(text string) []string {
	raw := strings.ReplaceAll(text, "\r\n", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{""}
	}
	chunks := strings.Split(raw, "\n\n")
	var out []string
	for _, c := range chunks {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return []string{raw}
	}
	return out
}

func (s *AIService) runAgentSingle(ctx context.Context, userID string, userInput string, dbClient *utils.SQLiteClient, workspace string) (string, error) {
	if workspace == "" {
		workspace = "/aiworkspace"
	}
	_ = os.MkdirAll(workspace, 0o755)

	coreMem := loadLatestCoreMemory(dbClient, userID)
	shotMem := loadRecentShotMemory(dbClient, userID, 12)

	allowed := loadAllowedToolNames(dbClient, false)
	toolsHint := loadToolsHint(dbClient, allowed)

	allowedTools := strings.Join(allowed, ", ")
	system := "You are an autonomous assistant running inside a container.\n" +
		"Workspace: " + workspace + "\n" +
		"Rules:\n" +
		"- You can use tools when needed: " + allowedTools + ".\n" +
		"- Tool name must be exactly one of: " + allowedTools + ". Never call other tool names.\n" +
		"- If a capability seems missing, use web_search/browser/curl/wget/sh to build a solution instead of giving up.\n" +
		"- For weather queries, use weather_query first with the city/region. Only use web_search if weather_query fails.\n" +
		"- When the user asks for weather, your final answer must include specific values (e.g., temperature, wind, precipitation chance, max/min). Do not only give links.\n" +
		"- For weather queries, if the user's location is missing, ask for the city/region instead of guessing.\n" +
		"- Use the workspace for files. Prefer writing outputs into workspace files when useful.\n" +
		"- If the problem is too complex or missing credentials/permissions, ask the user for help.\n" +
		"Output JSON only with shape: {\"mode\":\"tool\"|\"final\",\"tool_calls\":[...],\"final\":\"...\"}.\n" +
		"Tool call schema: {\"tool\":\"" + strings.ReplaceAll(allowedTools, ", ", "|") + "\",\"command\":\"...\",\"args\":[...],\"path\":\"...\",\"query\":\"...\",\"limit\":5,\"name\":\"...\",\"url\":\"...\",\"kind\":\"tool|skill|mcp\",\"entrypoint\":\"...\",\"description\":\"...\"}.\n" +
		"If mode is final, tool_calls must be empty."

	var msgs []Message
	msgs = append(msgs, Message{Role: "system", Content: system})
	if toolsHint != "" {
		msgs = append(msgs, Message{Role: "system", Content: "Tool registry:\n" + toolsHint})
	}
	if coreMem != "" {
		msgs = append(msgs, Message{Role: "system", Content: "core_memory:\n" + coreMem})
	}
	if shotMem != "" {
		msgs = append(msgs, Message{Role: "system", Content: "recent shot_memory:\n" + shotMem})
	}
	if strings.Contains(userInput, "天气") {
		loc := normalizeWeatherLocation(userInput)
		if loc != "" {
			results, _ := s.executeToolCalls(ctx, userID, []AgentToolCall{{Tool: "weather_query", Query: loc}}, dbClient, workspace)
			if len(results) > 0 {
				b, _ := json.Marshal(results)
				msgs = append(msgs, Message{Role: "system", Content: "tool_results: " + string(b)})
				if results[0].ExitCode == 0 && strings.TrimSpace(results[0].Stdout) != "" {
					msgs = append(msgs, Message{Role: "system", Content: "Weather data is already available in tool_results. Produce the final answer now; do not call web_search unless weather_query failed."})
				}
			}
		}
	}
	msgs = append(msgs, Message{Role: "user", Content: userInput})

	for round := 0; round < 4; round++ {
		content, raw, err := s.callLLM(msgs)
		if err != nil {
			return "", err
		}
		if aiDebugEnabled() {
			log.Printf("[Agent-Debug] LLM raw=%s", truncateForDebug(raw, aiDebugMaxChars()))
			log.Printf("[Agent-Debug] LLM content=%s", truncateForDebug(content, aiDebugMaxChars()))
		}
		out := parseAgentOutput(content)
		if out.Mode == "" {
			out = parseAgentOutput(raw)
		}

		if strings.EqualFold(out.Mode, "final") {
			return strings.TrimSpace(out.Final), nil
		}

		if strings.EqualFold(out.Mode, "tool") && len(out.ToolCalls) == 0 {
			msgs = append(msgs, Message{Role: "system", Content: "Invalid tool output: mode=tool but tool_calls is empty. Output correct JSON schema."})
			continue
		}
		if !strings.EqualFold(out.Mode, "tool") || len(out.ToolCalls) == 0 {
			return strings.TrimSpace(content), nil
		}

		results, err := s.executeToolCalls(ctx, userID, out.ToolCalls, dbClient, workspace)
		if err != nil {
			msgs = append(msgs, Message{Role: "system", Content: "Tool execution error: " + err.Error()})
			continue
		}
		resBytes, _ := json.Marshal(results)
		msgs = append(msgs, Message{Role: "system", Content: "tool_results: " + string(resBytes)})

		var newlyInstalled []string
		for _, r := range results {
			if !strings.EqualFold(strings.TrimSpace(r.Tool), "tool_install") || r.ExitCode != 0 {
				continue
			}
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(r.Stdout), &obj); err != nil {
				continue
			}
			n, _ := obj["name"].(string)
			n = strings.TrimSpace(n)
			if n != "" {
				newlyInstalled = append(newlyInstalled, n)
			}
		}
		if len(newlyInstalled) > 0 {
			msgs = append(msgs, Message{Role: "system", Content: "Newly installed tools are now available and allowed: " + strings.Join(newlyInstalled, ", ")})
		}
	}

	return "", fmt.Errorf("agent exceeded max rounds")
}

func (s *AIService) RunAgentWithAutoAcquire(ctx context.Context, userID string, userInput string, dbClient *utils.SQLiteClient, workspace string) (string, error) {
	maxRounds := envInt("AI_ACQUIRE_MAX_ROUNDS", 5)
	reply, err := s.runAcquireAgent(ctx, userID, userInput, dbClient, workspace, maxRounds)
	if err != nil {
		return "", err
	}
	return reply, nil
}

func (s *AIService) runAcquireAgent(ctx context.Context, userID string, userInput string, dbClient *utils.SQLiteClient, workspace string, maxRounds int) (string, error) {
	if workspace == "" {
		workspace = "/aiworkspace"
	}
	_ = os.MkdirAll(workspace, 0o755)

	coreMem := loadLatestCoreMemory(dbClient, userID)
	shotMem := loadRecentShotMemory(dbClient, userID, 12)

	allowed := loadAllowedToolNames(dbClient, true)
	toolsHint := loadToolsHint(dbClient, allowed)

	allowedTools := strings.Join(allowed, ", ")
	system := "You are in tool-acquisition mode.\n" +
		"Workspace: " + workspace + "\n" +
		"Goal:\n" +
		"- If you can solve the user's request without installing new tools, answer directly.\n" +
		"- Otherwise, search for an open-source script/tool and install it via tool_install.\n" +
		"- After installation, call the installed tool by its name to solve the request.\n" +
		"Rules:\n" +
		"- Tool name must be exactly one of: " + allowedTools + ". Never call other tool names.\n" +
		"- For weather queries, use weather_query first with the city/region. Only use web_search if weather_query fails.\n" +
		"- When the user asks for weather, your final answer must include specific values (e.g., temperature, wind, precipitation chance, max/min). Do not only give links.\n" +
		"- Use tool_install for downloading/registration. Do not download arbitrary code with sh/curl/wget unless strictly required.\n" +
		"- If you still cannot solve after multiple attempts, ask the user to handle it.\n" +
		"Output JSON only with shape: {\"mode\":\"tool\"|\"final\",\"tool_calls\":[...],\"final\":\"...\"}.\n" +
		"Tool call schema: {\"tool\":\"" + strings.ReplaceAll(allowedTools, ", ", "|") + "\",\"command\":\"...\",\"args\":[...],\"path\":\"...\",\"query\":\"...\",\"limit\":5,\"name\":\"...\",\"url\":\"...\",\"kind\":\"tool|skill|mcp\",\"entrypoint\":\"...\",\"description\":\"...\"}.\n" +
		"If mode is final, tool_calls must be empty."

	var msgs []Message
	msgs = append(msgs, Message{Role: "system", Content: system})
	if toolsHint != "" {
		msgs = append(msgs, Message{Role: "system", Content: "Tool registry:\n" + toolsHint})
	}
	if coreMem != "" {
		msgs = append(msgs, Message{Role: "system", Content: "core_memory:\n" + coreMem})
	}
	if shotMem != "" {
		msgs = append(msgs, Message{Role: "system", Content: "recent shot_memory:\n" + shotMem})
	}
	if strings.Contains(userInput, "天气") {
		loc := normalizeWeatherLocation(userInput)
		if loc != "" {
			results, _ := s.executeToolCalls(ctx, userID, []AgentToolCall{{Tool: "weather_query", Query: loc}}, dbClient, workspace)
			if len(results) > 0 {
				b, _ := json.Marshal(results)
				msgs = append(msgs, Message{Role: "system", Content: "tool_results: " + string(b)})
				if results[0].ExitCode == 0 && strings.TrimSpace(results[0].Stdout) != "" {
					msgs = append(msgs, Message{Role: "system", Content: "Weather data is already available in tool_results. Produce the final answer now; do not call web_search unless weather_query failed."})
				}
			}
		}
	}
	msgs = append(msgs, Message{Role: "user", Content: userInput})

	if maxRounds <= 0 {
		maxRounds = 5
	}

	for round := 0; round < maxRounds; round++ {
		content, raw, err := s.callLLM(msgs)
		if err != nil {
			return "", err
		}
		if aiDebugEnabled() {
			log.Printf("[Acquire-Debug] LLM raw=%s", truncateForDebug(raw, aiDebugMaxChars()))
			log.Printf("[Acquire-Debug] LLM content=%s", truncateForDebug(content, aiDebugMaxChars()))
		}
		out := parseAgentOutput(content)
		if out.Mode == "" {
			out = parseAgentOutput(raw)
		}

		if strings.EqualFold(out.Mode, "final") {
			return strings.TrimSpace(out.Final), nil
		}
		if strings.EqualFold(out.Mode, "tool") && len(out.ToolCalls) == 0 {
			msgs = append(msgs, Message{Role: "system", Content: "Invalid tool output: mode=tool but tool_calls is empty. Output correct JSON schema."})
			continue
		}
		if !strings.EqualFold(out.Mode, "tool") || len(out.ToolCalls) == 0 {
			return strings.TrimSpace(content), nil
		}

		results, err := s.executeToolCalls(ctx, userID, out.ToolCalls, dbClient, workspace)
		if err != nil {
			msgs = append(msgs, Message{Role: "system", Content: "Tool execution error: " + err.Error()})
			continue
		}
		resBytes, _ := json.Marshal(results)
		msgs = append(msgs, Message{Role: "system", Content: "tool_results: " + string(resBytes)})
	}

	return "我尝试在受控条件下自动搜索并安装工具来解决这个问题，但在最大尝试轮数内仍未成功。请你手动处理这个问题，或提供你希望使用的工具链接/仓库地址，我可以继续帮你安装并验证。", nil
}

func (s *AIService) callLLM(messages []Message) (content string, raw string, err error) {
	requestBody := map[string]interface{}{
		"model":    s.model,
		"messages": messages,
	}
	jsonData, _ := json.Marshal(requestBody)
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] LLM request body: %s", truncateForDebug(string(jsonData), aiDebugMaxChars()))
	}
	req, err := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	raw = string(bodyBytes)
	if aiDebugEnabled() {
		log.Printf("[AI-Debug] LLM response status=%d body=%s", resp.StatusCode, truncateForDebug(raw, aiDebugMaxChars()))
	}
	if resp.StatusCode != 200 {
		return "", raw, fmt.Errorf("LLM API returned status %d", resp.StatusCode)
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", raw, err
	}
	if len(result.Choices) == 0 {
		return "", raw, fmt.Errorf("LLM empty choices")
	}
	return result.Choices[0].Message.Content, raw, nil
}

func parseAgentOutput(s string) AgentOutput {
	candidate := s
	start := strings.IndexByte(candidate, '{')
	end := strings.LastIndexByte(candidate, '}')
	if start >= 0 && end > start {
		candidate = candidate[start : end+1]
	}

	var out AgentOutput
	if err := json.Unmarshal([]byte(candidate), &out); err == nil {
		if strings.EqualFold(strings.TrimSpace(out.Mode), "tool") || strings.EqualFold(strings.TrimSpace(out.Mode), "final") {
			return out
		}
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(candidate), &m); err != nil {
		return AgentOutput{}
	}
	mode := strings.ToLower(strings.TrimSpace(toString(m["mode"])))
	if mode == "final" {
		return AgentOutput{Mode: "final", Final: toString(m["final"])}
	}

	if tcRaw, ok := m["tool_calls"]; ok {
		b, _ := json.Marshal(tcRaw)
		var tcs []AgentToolCall
		if err := json.Unmarshal(b, &tcs); err == nil && len(tcs) > 0 {
			return AgentOutput{Mode: "tool", ToolCalls: tcs}
		}
	}

	if tool := strings.TrimSpace(toString(m["tool"])); tool != "" {
		return AgentOutput{
			Mode: "tool",
			ToolCalls: []AgentToolCall{{
				Tool:       tool,
				Command:    toString(m["command"]),
				Path:       toString(m["path"]),
				Query:      toString(m["query"]),
				Limit:      intFromAny(m["limit"]),
				Name:       toString(m["name"]),
				URL:        toString(m["url"]),
				Kind:       toString(m["kind"]),
				Entrypoint: toString(m["entrypoint"]),
			}},
		}
	}

	if mode != "" && mode != "tool" && mode != "final" {
		if hasAnyKey(m, "query", "command", "path", "args", "url", "name", "kind") {
			return AgentOutput{
				Mode: "tool",
				ToolCalls: []AgentToolCall{{
					Tool:       mode,
					Command:    toString(m["command"]),
					Path:       toString(m["path"]),
					Query:      toString(m["query"]),
					Limit:      intFromAny(m["limit"]),
					Name:       toString(m["name"]),
					URL:        toString(m["url"]),
					Kind:       toString(m["kind"]),
					Entrypoint: toString(m["entrypoint"]),
				}},
			}
		}
	}

	return AgentOutput{}
}

func hasAnyKey(m map[string]interface{}, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func intFromAny(v interface{}) int {
	switch t := v.(type) {
	case nil:
		return 0
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	case []byte:
		n, _ := strconv.Atoi(strings.TrimSpace(string(t)))
		return n
	default:
		return 0
	}
}

func (s *AIService) executeToolCalls(ctx context.Context, userID string, calls []AgentToolCall, dbClient *utils.SQLiteClient, workspace string) ([]AgentToolResult, error) {
	var results []AgentToolResult
	for _, c := range calls {
		r := AgentToolResult{Tool: c.Tool, Command: c.Command, Args: strings.Join(c.Args, " ")}
		if aiDebugEnabled() {
			log.Printf("[Tool-Debug] tool=%s command=%s args=%s path=%s query=%s", c.Tool, c.Command, strings.Join(c.Args, " "), c.Path, c.Query)
		}
		tr, err := s.executeOneTool(ctx, c, workspace, dbClient)
		if err != nil {
			r.Stderr = err.Error()
			r.ExitCode = 1
		} else {
			r = tr
		}
		results = append(results, r)
		_ = insertToolRun(dbClient, userID, r)
	}
	return results, nil
}

func (s *AIService) executeOneTool(ctx context.Context, c AgentToolCall, workspace string, dbClient *utils.SQLiteClient) (AgentToolResult, error) {
	timeout := 60 * time.Second
	if v := strings.TrimSpace(os.Getenv("AI_TOOL_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tool := strings.ToLower(strings.TrimSpace(c.Tool))
	switch tool {
	case "web_search":
		limit := c.Limit
		if limit <= 0 {
			limit = 5
		}
		query := c.Query
		if query == "" {
			query = c.Command
		}
		results, err := s.WebSearch(query, limit)
		if err != nil {
			return AgentToolResult{Tool: "web_search", Command: query, ExitCode: 1, Stderr: err.Error()}, nil
		}
		b, _ := json.Marshal(results)
		return AgentToolResult{Tool: "web_search", Command: query, ExitCode: 0, Stdout: string(b)}, nil
	case "weather_query", "weather":
		location := strings.TrimSpace(c.Query)
		if location == "" {
			location = strings.TrimSpace(c.Command)
		}
		location = normalizeWeatherLocation(location)
		if location == "" || strings.EqualFold(location, "weather_query") || strings.EqualFold(location, "weather") {
			return AgentToolResult{Tool: "weather_query", ExitCode: 1, Stderr: "missing location; provide city/region in query or command"}, nil
		}
		geo, err := geocodeLocation(tctx, location)
		if err != nil {
			return AgentToolResult{Tool: "weather_query", Command: location, ExitCode: 1, Stderr: err.Error()}, nil
		}

		if r, err := s.weatherFromOpenMeteoGeo(tctx, location, geo); err == nil {
			return r, nil
		}
		if r, err := s.weatherFromMetNoGeo(tctx, location, geo); err == nil {
			return r, nil
		}
		if r, err := s.weatherFrom7TimerGeo(tctx, location, geo); err == nil {
			return r, nil
		}

		if envBool("AI_WEATHER_TRY_WTTR", false) {
			if r, ok := s.tryWttr(tctx, location, workspace); ok {
				return r, nil
			}
		}

		return AgentToolResult{Tool: "weather_query", Command: location, ExitCode: 1, Stderr: "all weather providers failed"}, nil
	case "tool_install":
		return s.installFromURL(tctx, c, dbClient)
	case "cat":
		p := c.Path
		if p == "" {
			p = c.Command
		}
		if p == "" {
			return AgentToolResult{Tool: "cat", ExitCode: 1, Stderr: "missing path"}, nil
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(workspace, p)
		}
		cmd := exec.CommandContext(tctx, "cat", p)
		cmd.Dir = workspace
		return runCmd(tool, c.Command, c.Args, cmd)
	case "curl":
		args := c.Args
		if c.Command != "" {
			args = append([]string{c.Command}, args...)
		}
		cmd := exec.CommandContext(tctx, "curl", args...)
		cmd.Dir = workspace
		return runCmd(tool, c.Command, c.Args, cmd)
	case "browser":
		args := []string{"-L"}
		if c.Command != "" {
			args = append(args, c.Command)
		} else if len(c.Args) > 0 {
			args = append(args, c.Args...)
		} else {
			return AgentToolResult{Tool: "browser", ExitCode: 1, Stderr: "missing url"}, nil
		}
		cmd := exec.CommandContext(tctx, "curl", args...)
		cmd.Dir = workspace
		return runCmd(tool, c.Command, c.Args, cmd)
	case "wget":
		args := c.Args
		if c.Command != "" {
			args = append([]string{c.Command}, args...)
		}
		cmd := exec.CommandContext(tctx, "wget", args...)
		cmd.Dir = workspace
		return runCmd(tool, c.Command, c.Args, cmd)
	case "sh":
		command := c.Command
		if command == "" && len(c.Args) > 0 {
			command = strings.Join(c.Args, " ")
		}
		if command == "" {
			return AgentToolResult{Tool: "sh", ExitCode: 1, Stderr: "missing command"}, nil
		}
		cmd := exec.CommandContext(tctx, "bash", "-lc", command)
		cmd.Dir = workspace
		return runCmd(tool, command, nil, cmd)
	default:
		if dbClient != nil {
			if tr, ok := executeExternalTool(tctx, c, workspace, dbClient); ok {
				return tr, nil
			}
		}
		return AgentToolResult{Tool: tool, ExitCode: 1, Stderr: "unknown tool"}, nil
	}
}

func normalizeWeatherLocation(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	replacements := []string{
		"天气怎么样", "",
		"天气如何", "",
		"天气", "",
		"今日", "",
		"今天", "",
		"现在", "",
	}
	t = strings.NewReplacer(replacements...).Replace(t)
	t = strings.TrimSpace(t)
	if fields := strings.Fields(t); len(fields) > 0 {
		t = fields[0]
	}
	t = strings.Trim(t, "，。？！,.!?")
	rs := []rune(t)
	if len(rs) > 20 {
		t = string(rs[:20])
	}
	return strings.TrimSpace(t)
}

type geoLocation struct {
	Lat     float64
	Lon     float64
	Display string
}

func geocodeLocation(ctx context.Context, location string) (geoLocation, error) {
	location = normalizeWeatherLocation(location)
	if location == "" {
		return geoLocation{}, fmt.Errorf("missing location")
	}
	type geoItem struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
		Name        string `json:"name"`
	}
	geoURL := "https://nominatim.openstreetmap.org/search?format=json&limit=1&q=" + url.QueryEscape(location)
	geoBody, geoStatus, err := httpGet(ctx, geoURL, map[string]string{
		"Accept":     "application/json",
		"User-Agent": "ai-platform/1.0",
	})
	if err != nil {
		return geoLocation{}, err
	}
	if geoStatus < 200 || geoStatus >= 300 {
		return geoLocation{}, fmt.Errorf("geocode status=%d", geoStatus)
	}
	var geo []geoItem
	if err := json.Unmarshal(geoBody, &geo); err != nil || len(geo) == 0 {
		return geoLocation{}, fmt.Errorf("geocode no result")
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(geo[0].Lat), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(geo[0].Lon), 64)
	if err1 != nil || err2 != nil {
		return geoLocation{}, fmt.Errorf("geocode parse failed")
	}
	display := strings.TrimSpace(geo[0].Name)
	if display == "" {
		display = strings.TrimSpace(geo[0].DisplayName)
	}
	if display == "" {
		display = location
	}
	return geoLocation{Lat: lat, Lon: lon, Display: display}, nil
}

func envBool(key string, def bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return def
	}
	if raw == "1" || raw == "true" || raw == "yes" || raw == "on" {
		return true
	}
	if raw == "0" || raw == "false" || raw == "no" || raw == "off" {
		return false
	}
	return def
}

func (s *AIService) tryWttr(ctx context.Context, location string, workspace string) (AgentToolResult, bool) {
	location = normalizeWeatherLocation(location)
	if location == "" {
		return AgentToolResult{}, false
	}
	uHTTPS := "https://wttr.in/" + url.PathEscape(location) + "?format=j1"
	cmd := exec.CommandContext(ctx, "curl", "-sS", "-L", "--connect-timeout", "5", "--max-time", "10", uHTTPS)
	cmd.Dir = workspace
	r, _ := runCmd("weather_query", uHTTPS, nil, cmd)
	if r.ExitCode == 0 && strings.TrimSpace(r.Stdout) != "" {
		return r, true
	}

	uHTTP := "http://wttr.in/" + url.PathEscape(location) + "?format=j1"
	cmd2 := exec.CommandContext(ctx, "curl", "-sS", "-L", "--connect-timeout", "5", "--max-time", "10", uHTTP)
	cmd2.Dir = workspace
	r2, _ := runCmd("weather_query", uHTTP, nil, cmd2)
	if r2.ExitCode == 0 && strings.TrimSpace(r2.Stdout) != "" {
		return r2, true
	}
	return AgentToolResult{}, false
}

func (s *AIService) weatherFromOpenMeteo(ctx context.Context, location string) (AgentToolResult, error) {
	geo, err := geocodeLocation(ctx, location)
	if err != nil {
		return AgentToolResult{}, err
	}
	return s.weatherFromOpenMeteoGeo(ctx, location, geo)
}

func (s *AIService) weatherFromOpenMeteoGeo(ctx context.Context, location string, geo geoLocation) (AgentToolResult, error) {
	meteoURL := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s&current_weather=true&timezone=auto&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max",
		strconv.FormatFloat(geo.Lat, 'f', 6, 64),
		strconv.FormatFloat(geo.Lon, 'f', 6, 64),
	)
	meteoBody, meteoStatus, err := httpGet(ctx, meteoURL, map[string]string{
		"Accept":     "application/json",
		"User-Agent": "ai-platform/1.0",
	})
	if err != nil {
		return AgentToolResult{}, err
	}
	if meteoStatus < 200 || meteoStatus >= 300 {
		return AgentToolResult{}, fmt.Errorf("open-meteo status=%d", meteoStatus)
	}

	var meteo map[string]interface{}
	if err := json.Unmarshal(meteoBody, &meteo); err != nil {
		return AgentToolResult{}, err
	}

	current := map[string]interface{}{}
	if cw, ok := meteo["current_weather"].(map[string]interface{}); ok {
		current["time"] = cw["time"]
		current["temperature_c"] = cw["temperature"]
		current["wind_speed_kmh"] = cw["windspeed"]
		current["wind_dir_deg"] = cw["winddirection"]
		current["weather_code"] = cw["weathercode"]
	}

	daily := map[string]interface{}{}
	if d, ok := meteo["daily"].(map[string]interface{}); ok {
		daily["time"] = d["time"]
		daily["temp_max_c"] = d["temperature_2m_max"]
		daily["temp_min_c"] = d["temperature_2m_min"]
		daily["precip_prob_max_pct"] = d["precipitation_probability_max"]
	}

	out := map[string]interface{}{
		"provider":  "open-meteo",
		"location":  geo.Display,
		"latitude":  geo.Lat,
		"longitude": geo.Lon,
		"current":   current,
		"daily":     daily,
		"raw":       meteo,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	b, _ := json.Marshal(out)
	return AgentToolResult{Tool: "weather_query", Command: normalizeWeatherLocation(location), ExitCode: 0, Stdout: string(b)}, nil
}

func (s *AIService) weatherFromMetNoGeo(ctx context.Context, location string, geo geoLocation) (AgentToolResult, error) {
	u := fmt.Sprintf("https://api.met.no/weatherapi/locationforecast/2.0/compact?lat=%s&lon=%s",
		strconv.FormatFloat(geo.Lat, 'f', 6, 64),
		strconv.FormatFloat(geo.Lon, 'f', 6, 64),
	)
	body, status, err := httpGet(ctx, u, map[string]string{
		"Accept":     "application/json",
		"User-Agent": "ai-platform/1.0",
	})
	if err != nil {
		return AgentToolResult{}, err
	}
	if status < 200 || status >= 300 {
		return AgentToolResult{}, fmt.Errorf("met.no status=%d", status)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return AgentToolResult{}, err
	}

	current := map[string]interface{}{}
	props, _ := obj["properties"].(map[string]interface{})
	ts, _ := props["timeseries"].([]interface{})
	if len(ts) > 0 {
		t0, _ := ts[0].(map[string]interface{})
		current["time"] = t0["time"]
		data, _ := t0["data"].(map[string]interface{})
		instant, _ := data["instant"].(map[string]interface{})
		details, _ := instant["details"].(map[string]interface{})
		if details != nil {
			current["temperature_c"] = details["air_temperature"]
			current["wind_speed_mps"] = details["wind_speed"]
			current["wind_dir_deg"] = details["wind_from_direction"]
			current["humidity_pct"] = details["relative_humidity"]
		}
		if next, ok := data["next_1_hours"].(map[string]interface{}); ok {
			if sum, ok := next["summary"].(map[string]interface{}); ok {
				current["symbol_code_1h"] = sum["symbol_code"]
			}
			if det, ok := next["details"].(map[string]interface{}); ok {
				current["precip_mm_1h"] = det["precipitation_amount"]
			}
		}
	}

	out := map[string]interface{}{
		"provider":  "met.no",
		"location":  geo.Display,
		"latitude":  geo.Lat,
		"longitude": geo.Lon,
		"current":   current,
		"raw":       obj,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	b, _ := json.Marshal(out)
	return AgentToolResult{Tool: "weather_query", Command: normalizeWeatherLocation(location), ExitCode: 0, Stdout: string(b)}, nil
}

func (s *AIService) weatherFrom7TimerGeo(ctx context.Context, location string, geo geoLocation) (AgentToolResult, error) {
	u := fmt.Sprintf("https://www.7timer.info/bin/api.pl?lon=%s&lat=%s&product=civil&output=json",
		strconv.FormatFloat(geo.Lon, 'f', 6, 64),
		strconv.FormatFloat(geo.Lat, 'f', 6, 64),
	)
	body, status, err := httpGet(ctx, u, map[string]string{
		"Accept":     "application/json",
		"User-Agent": "ai-platform/1.0",
	})
	if err != nil {
		return AgentToolResult{}, err
	}
	if status < 200 || status >= 300 {
		return AgentToolResult{}, fmt.Errorf("7timer status=%d", status)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return AgentToolResult{}, err
	}

	current := map[string]interface{}{}
	if ds, ok := obj["dataseries"].([]interface{}); ok && len(ds) > 0 {
		d0, _ := ds[0].(map[string]interface{})
		current["temp_c"] = d0["temp2m"]
		current["weather"] = d0["weather"]
		if w, ok := d0["wind10m"].(map[string]interface{}); ok {
			current["wind_dir"] = w["direction"]
			current["wind_speed"] = w["speed"]
		}
	}

	out := map[string]interface{}{
		"provider":  "7timer",
		"location":  geo.Display,
		"latitude":  geo.Lat,
		"longitude": geo.Lon,
		"current":   current,
		"raw":       obj,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	b, _ := json.Marshal(out)
	return AgentToolResult{Tool: "weather_query", Command: normalizeWeatherLocation(location), ExitCode: 0, Stdout: string(b)}, nil
}

func httpGet(ctx context.Context, urlStr string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (s *AIService) installFromURL(ctx context.Context, c AgentToolCall, dbClient *utils.SQLiteClient) (AgentToolResult, error) {
	if dbClient == nil {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "db unavailable"}, nil
	}

	name := strings.TrimSpace(c.Name)
	if name == "" {
		name = strings.TrimSpace(c.Command)
	}
	installURL := strings.TrimSpace(c.URL)
	if installURL == "" {
		if u, err := url.Parse(strings.TrimSpace(c.Command)); err == nil && u.Scheme != "" {
			installURL = strings.TrimSpace(c.Command)
		}
	}
	kind := strings.ToLower(strings.TrimSpace(c.Kind))
	if kind == "" {
		kind = "tool"
	}
	if kind == "agent" {
		kind = "mcp"
	}

	if name == "" || len(name) > 64 || strings.ContainsAny(name, " \t\r\n/\\") {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "invalid name"}, nil
	}
	if installURL == "" {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "missing url"}, nil
	}
	parsed, err := url.Parse(installURL)
	if err != nil {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "invalid url"}, nil
	}
	if strings.ToLower(parsed.Scheme) != "https" {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "only https is allowed"}, nil
	}
	host := strings.ToLower(parsed.Hostname())
	allowedHosts := map[string]bool{
		"github.com":                 true,
		"raw.githubusercontent.com":  true,
		"gist.githubusercontent.com": true,
		"gitlab.com":                 true,
		"raw.gitlabusercontent.com":  true,
	}
	if !allowedHosts[host] {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "unapproved host"}, nil
	}

	entry := strings.TrimSpace(c.Entrypoint)
	if entry == "" {
		entry = "run.sh"
	}
	if strings.Contains(entry, "..") || strings.ContainsAny(entry, "\r\n\t ") || strings.Contains(entry, string(filepath.Separator)) {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "invalid entrypoint"}, nil
	}
	if !strings.HasSuffix(strings.ToLower(entry), ".sh") && !strings.HasSuffix(strings.ToLower(entry), ".bash") {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "only .sh scripts are supported"}, nil
	}

	kindDir := ""
	switch kind {
	case "tool", "tools":
		kind = "tool"
		kindDir = "tools"
	case "skill", "skills":
		kind = "skill"
		kindDir = "skills"
	case "mcp", "agent", "agents":
		kind = "mcp"
		kindDir = "mcp"
	default:
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "invalid kind"}, nil
	}

	installDir := filepath.Join("/aistore", kindDir, name)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: err.Error()}, nil
	}

	maxBytes := int64(200 * 1024)
	req, err := http.NewRequestWithContext(ctx, "GET", installURL, nil)
	if err != nil {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: err.Error()}, nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: err.Error()}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "download failed: " + resp.Status}, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: err.Error()}, nil
	}
	if int64(len(body)) > maxBytes {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "file too large"}, nil
	}
	if bytes.IndexByte(body, 0) >= 0 {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "binary content rejected"}, nil
	}

	src := string(body)
	reasons := scanScriptForMalice(src)
	if len(reasons) > 0 {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: "rejected by safety scan: " + strings.Join(reasons, "; ")}, nil
	}

	scriptPath := filepath.Join(installDir, entry)
	if err := os.WriteFile(scriptPath, body, 0o755); err != nil {
		return AgentToolResult{Tool: "tool_install", ExitCode: 1, Stderr: err.Error()}, nil
	}

	mf := AistoreManifest{
		Type:              kind,
		Name:              name,
		Description:       strings.TrimSpace(c.Description),
		Category:          strings.TrimSpace(c.Category),
		Tags:              c.Tags,
		Version:           strings.TrimSpace(c.Version),
		Entrypoint:        entry,
		Path:              installDir,
		UsageInstructions: strings.TrimSpace(c.Instructions),
		UsageRestrictions: strings.TrimSpace(c.Restrictions),
		Source:            strings.TrimSpace(c.Source),
	}
	if mf.Category == "" {
		mf.Category = "downloaded"
	}
	if mf.Source == "" {
		mf.Source = "auto_download"
	}
	if mf.Version == "" {
		mf.Version = "1"
	}
	if mf.Description == "" {
		mf.Description = "Auto-downloaded tool"
	}
	if mf.UsageRestrictions == "" {
		mf.UsageRestrictions = "Auto-downloaded; executed in workspace; reviewed by heuristic scan"
	}
	enabled := true
	mf.Enabled = &enabled

	_ = os.WriteFile(filepath.Join(installDir, "manifest.json"), mustJSON(mf), 0o644)
	_ = upsertManifest(dbClient, kind, mf)

	out := map[string]interface{}{
		"installed":  true,
		"name":       name,
		"kind":       kind,
		"path":       installDir,
		"entrypoint": entry,
		"url":        installURL,
	}
	b, _ := json.Marshal(out)
	return AgentToolResult{Tool: "tool_install", Command: name, ExitCode: 0, Stdout: string(b)}, nil
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func scanScriptForMalice(src string) []string {
	s := strings.ToLower(src)
	var reasons []string
	pats := []struct {
		pat string
		msg string
	}{
		{"rm -rf /", "rm -rf /"},
		{"mkfs.", "mkfs"},
		{"dd if=/dev/zero", "dd if=/dev/zero"},
		{"dd if=/dev/", "dd if=/dev"},
		{"shutdown", "shutdown"},
		{"reboot", "reboot"},
		{"poweroff", "poweroff"},
		{":(){:|:&};:", "fork bomb"},
		{"curl ", "curl|bash/sh pattern"},
		{"| bash", "pipe to bash"},
		{"| sh", "pipe to sh"},
		{"nc -e", "netcat exec"},
		{"socat", "socat"},
		{"chmod 777 /", "chmod root"},
		{"chown root", "chown root"},
		{"/etc/", "writes to /etc"},
	}
	for _, p := range pats {
		if strings.Contains(s, p.pat) {
			if p.pat == "curl " && !(strings.Contains(s, "| bash") || strings.Contains(s, "| sh")) {
				continue
			}
			reasons = append(reasons, p.msg)
		}
	}
	return reasons
}

func executeExternalTool(ctx context.Context, c AgentToolCall, workspace string, dbClient *utils.SQLiteClient) (AgentToolResult, bool) {
	toolName := strings.TrimSpace(c.Tool)
	if toolName == "" || dbClient == nil {
		return AgentToolResult{}, false
	}
	rows, err := dbClient.QueryData("tools", &utils.FieldID{ColumnName: "tool_name", Value: toolName}, 1, nil)
	if err != nil || len(rows) == 0 {
		return AgentToolResult{}, false
	}
	row := rows[0]
	if strings.TrimSpace(toString(row["enabled"])) == "0" {
		return AgentToolResult{Tool: toolName, ExitCode: 1, Stderr: "tool disabled"}, true
	}
	source := strings.ToLower(strings.TrimSpace(toString(row["source"])))
	if source == "ai_intent" {
		return AgentToolResult{Tool: toolName, ExitCode: 1, Stderr: "unverified tool"}, true
	}
	basePath := strings.TrimSpace(toString(row["path"]))
	entry := strings.TrimSpace(toString(row["entrypoint"]))
	if basePath == "" || entry == "" {
		return AgentToolResult{Tool: toolName, ExitCode: 1, Stderr: "missing tool path/entrypoint"}, true
	}
	if !strings.HasPrefix(basePath, "/aistore/") {
		return AgentToolResult{Tool: toolName, ExitCode: 1, Stderr: "tool path rejected"}, true
	}
	if strings.Contains(entry, "..") || strings.ContainsAny(entry, "\r\n\t ") || strings.Contains(entry, string(filepath.Separator)) {
		return AgentToolResult{Tool: toolName, ExitCode: 1, Stderr: "invalid entrypoint"}, true
	}
	scriptPath := entry
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(basePath, scriptPath)
	}
	if !strings.HasPrefix(scriptPath, basePath) {
		return AgentToolResult{Tool: toolName, ExitCode: 1, Stderr: "entrypoint outside tool dir"}, true
	}
	ext := strings.ToLower(filepath.Ext(scriptPath))
	if ext != ".sh" && ext != ".bash" {
		return AgentToolResult{Tool: toolName, ExitCode: 1, Stderr: "unsupported entrypoint type"}, true
	}

	var args []string
	args = append(args, scriptPath)
	if strings.TrimSpace(c.Command) != "" {
		args = append(args, c.Command)
	}
	args = append(args, c.Args...)
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Dir = workspace
	tr, _ := runCmd(toolName, scriptPath, args[1:], cmd)
	return tr, true
}

func runCmd(tool string, command string, args []string, cmd *exec.Cmd) (AgentToolResult, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		exitCode = 1
		if errors.Is(err, context.DeadlineExceeded) {
			exitCode = 124
		}
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}
	outStr := stdout.String()
	errStr := stderr.String()
	if err != nil && strings.TrimSpace(errStr) == "" {
		if errors.Is(err, context.DeadlineExceeded) {
			errStr = "timeout"
		} else {
			errStr = err.Error()
		}
	}
	if aiDebugEnabled() {
		log.Printf("[Tool-Debug] tool=%s exit=%d stdout=%s stderr=%s", tool, exitCode, truncateForDebug(outStr, aiDebugMaxChars()), truncateForDebug(errStr, aiDebugMaxChars()))
	}
	return AgentToolResult{
		Tool:     tool,
		Command:  command,
		Args:     strings.Join(args, " "),
		ExitCode: exitCode,
		Stdout:   truncateForDebug(outStr, aiDebugMaxChars()),
		Stderr:   truncateForDebug(errStr, aiDebugMaxChars()),
	}, nil
}

func loadLatestCoreMemory(dbClient *utils.SQLiteClient, userID string) string {
	rows, err := dbClient.QueryData("core_memory", &utils.FieldID{ColumnName: "user_id", Value: userID}, 1, []utils.OrderDescription{{ColumnName: "timestamp", IsAscending: false}})
	if err != nil || len(rows) == 0 {
		return ""
	}
	v, ok := rows[0]["core_memory"]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func loadRecentShotMemory(dbClient *utils.SQLiteClient, userID string, limit int) string {
	rows, err := dbClient.QueryData("shot_memory", &utils.FieldID{ColumnName: "user_id", Value: userID}, limit, []utils.OrderDescription{{ColumnName: "timestamp", IsAscending: false}})
	if err != nil || len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	for i := len(rows) - 1; i >= 0; i-- {
		m := rows[i]
		msg := toString(m["message"])
		resp := toString(m["response"])
		b.WriteString("- user: ")
		b.WriteString(msg)
		b.WriteString("\n  assistant: ")
		b.WriteString(resp)
		b.WriteString("\n")
	}
	return b.String()
}

func loadAllowedToolNames(dbClient *utils.SQLiteClient, includeInstaller bool) []string {
	base := []string{"web_search", "weather_query", "sh", "curl", "wget", "cat", "browser"}
	if includeInstaller {
		base = append(base, "tool_install")
	}
	seen := map[string]bool{}
	var out []string
	for _, n := range base {
		key := strings.ToLower(strings.TrimSpace(n))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, n)
	}
	if dbClient == nil {
		return out
	}
	rows, err := dbClient.QueryData("tools", nil, 100, []utils.OrderDescription{{ColumnName: "tool_name", IsAscending: true}})
	if err != nil || len(rows) == 0 {
		return out
	}
	for _, r := range rows {
		name := strings.TrimSpace(toString(r["tool_name"]))
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		enabled := toString(r["enabled"])
		if enabled == "0" {
			continue
		}
		source := strings.ToLower(strings.TrimSpace(toString(r["source"])))
		if source == "ai_intent" {
			continue
		}
		path := strings.TrimSpace(toString(r["path"]))
		entry := strings.TrimSpace(toString(r["entrypoint"]))
		if path == "" || entry == "" {
			continue
		}
		if !strings.HasPrefix(path, "/aistore/") {
			continue
		}
		if strings.Contains(entry, "..") || strings.ContainsAny(entry, "\r\n\t ") || strings.Contains(entry, string(filepath.Separator)) {
			continue
		}
		out = append(out, name)
		seen[key] = true
	}
	return out
}

func loadToolsHint(dbClient *utils.SQLiteClient, allowed []string) string {
	if dbClient == nil {
		return ""
	}
	rows, err := dbClient.QueryData("tools", nil, 80, []utils.OrderDescription{{ColumnName: "tool_name", IsAscending: true}})
	if err != nil || len(rows) == 0 {
		return ""
	}
	allow := map[string]bool{}
	for _, n := range allowed {
		allow[strings.ToLower(strings.TrimSpace(n))] = true
	}
	var b strings.Builder
	for _, r := range rows {
		name := strings.TrimSpace(toString(r["tool_name"]))
		if name == "" {
			continue
		}
		if !allow[strings.ToLower(name)] {
			continue
		}
		ins := toString(r["usage_instructions"])
		res := toString(r["usage_restrictions"])
		b.WriteString("- ")
		b.WriteString(name)
		if ins != "" {
			b.WriteString(": ")
			b.WriteString(ins)
		}
		if res != "" {
			b.WriteString(" | restrictions: ")
			b.WriteString(res)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func envInt(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func insertToolRun(dbClient *utils.SQLiteClient, userID string, r AgentToolResult) error {
	if dbClient == nil {
		return nil
	}
	return dbClient.InsertData("tool_runs", []utils.FieldData{
		{Name: "user_id", Value: userID},
		{Name: "tool", Value: r.Tool},
		{Name: "command", Value: r.Command},
		{Name: "args", Value: r.Args},
		{Name: "stdout", Value: r.Stdout},
		{Name: "stderr", Value: r.Stderr},
		{Name: "exit_code", Value: r.ExitCode},
	})
}

func (s *AIService) MaybeUpdateCoreMemory(ctx context.Context, userID string, dbClient *utils.SQLiteClient) error {
	if dbClient == nil {
		return nil
	}
	v := strings.TrimSpace(strings.ToLower(os.Getenv("AI_CORE_MEMORY")))
	if v == "0" || v == "false" || v == "off" || v == "no" {
		return nil
	}

	last, err := dbClient.QueryData("shot_memory", &utils.FieldID{ColumnName: "user_id", Value: userID}, 1, []utils.OrderDescription{{ColumnName: "timestamp", IsAscending: false}})
	if err != nil || len(last) == 0 {
		return nil
	}
	idAny := last[0]["id"]
	id := int64(0)
	switch t := idAny.(type) {
	case int64:
		id = t
	case int:
		id = int64(t)
	case []byte:
		parsed, _ := strconv.ParseInt(string(t), 10, 64)
		id = parsed
	}
	if id == 0 || id%5 != 0 {
		return nil
	}

	core := loadLatestCoreMemory(dbClient, userID)
	shots := loadRecentShotMemory(dbClient, userID, 30)
	if shots == "" {
		return nil
	}

	system := "You update long-term memory.\n" +
		"Given recent conversations and existing core_memory, produce a refined, compact core_memory.\n" +
		"Also infer the user's attitude and thoughts.\n" +
		"Output JSON only: {\"core_memory\":\"...\",\"attitude\":\"...\",\"thoughts\":\"...\"}."

	var msgs []Message
	msgs = append(msgs, Message{Role: "system", Content: system})
	if core != "" {
		msgs = append(msgs, Message{Role: "system", Content: "existing core_memory:\n" + core})
	}
	msgs = append(msgs, Message{Role: "system", Content: "recent shot_memory:\n" + shots})

	content, raw, err := s.callLLM(msgs)
	if err != nil {
		return err
	}
	if aiDebugEnabled() {
		log.Printf("[Memory-Debug] core_memory raw=%s", truncateForDebug(raw, aiDebugMaxChars()))
		log.Printf("[Memory-Debug] core_memory content=%s", truncateForDebug(content, aiDebugMaxChars()))
	}

	type memOut struct {
		CoreMemory string `json:"core_memory"`
		Attitude   string `json:"attitude"`
		Thoughts   string `json:"thoughts"`
	}
	var mo memOut
	if err := json.Unmarshal([]byte(content), &mo); err != nil {
		start := strings.IndexByte(content, '{')
		end := strings.LastIndexByte(content, '}')
		if start >= 0 && end > start {
			_ = json.Unmarshal([]byte(content[start:end+1]), &mo)
		}
	}

	mo.CoreMemory = strings.TrimSpace(mo.CoreMemory)
	if mo.CoreMemory != "" && mo.CoreMemory != core {
		_ = dbClient.InsertData("core_memory", []utils.FieldData{
			{Name: "user_id", Value: userID},
			{Name: "core_memory", Value: mo.CoreMemory},
			{Name: "source", Value: "auto"},
		})
	}

	if strings.TrimSpace(mo.Attitude) != "" || strings.TrimSpace(mo.Thoughts) != "" {
		now := time.Now()
		_ = upsertByName(dbClient, "user", "user_id", userID, []utils.FieldData{
			{Name: "user_id", Value: userID},
			{Name: "attitude", Value: mo.Attitude},
			{Name: "thoughts", Value: mo.Thoughts},
			{Name: "timestamp", Value: now},
		})
	}

	return nil
}
