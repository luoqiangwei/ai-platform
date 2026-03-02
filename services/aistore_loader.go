package services

import (
	"ai-platform/utils"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AistoreManifest struct {
	Type                string   `json:"type"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Category            string   `json:"category"`
	Tags                []string `json:"tags"`
	Version             string   `json:"version"`
	ExpiresAt           string   `json:"expires_at"`
	Enabled             *bool    `json:"enabled"`
	Entrypoint          string   `json:"entrypoint"`
	Path                string   `json:"path"`
	Capabilities        string   `json:"capabilities"`
	UsageInstructions   string   `json:"usage_instructions"`
	UsageRestrictions   string   `json:"usage_restrictions"`
	RestrictedScenarios string   `json:"restricted_scenarios"`
	Source              string   `json:"source"`
}

func EnsureBuiltInTools(dbClient *utils.SQLiteClient) error {
	now := time.Now()
	defs := []struct {
		name string
		ins  string
		res  string
		tags string
		ent  string
	}{
		{
			name: "web_search",
			ins:  "HTTP GET ${WEB_SEARCH_ENDPOINT}?q=...&limit=...; optional header Authorization: Bearer ${WEB_SEARCH_API_KEY}",
			res:  "Do not send secrets; keep queries minimal; respect legal and privacy requirements",
			tags: "internet,search",
			ent:  "builtin:web_search",
		},
		{
			name: "weather_query",
			ins:  "Fetch weather for a city/region. Providers: open-meteo (default), met.no, 7timer; optional wttr.in when AI_WEATHER_TRY_WTTR=true. Provide city in query/command; if missing, ask user for city.",
			res:  "Do not send secrets; prefer https; if no location provided, do not guess; ask for city/region; respect provider rate limits",
			tags: "weather,internet",
			ent:  "builtin:weather_query",
		},
		{
			name: "tool_install",
			ins:  "Install a tool/skill/mcp from a safe URL into /aistore, perform safety scan, and register into sqlite. Provide: name, url, kind, entrypoint, description.",
			res:  "Only allow https URLs from approved hosts; refuse suspicious scripts; keep tool small; no binaries",
			tags: "aistore,install,security",
			ent:  "builtin:tool_install",
		},
		{
			name: "sh",
			ins:  "Run shell command in workspace: bash -lc \"...\"",
			res:  "Use workspace for file operations; avoid destructive commands unless required",
			tags: "shell,workspace",
			ent:  "builtin:sh",
		},
		{
			name: "curl",
			ins:  "Run: curl <args>. Example: curl -L https://example.com -o file",
			res:  "Do not send secrets; prefer https; keep outputs small",
			tags: "http,download",
			ent:  "builtin:curl",
		},
		{
			name: "wget",
			ins:  "Run: wget <args>. Example: wget https://example.com/file -O file",
			res:  "Do not send secrets; prefer https; keep outputs small",
			tags: "http,download",
			ent:  "builtin:wget",
		},
		{
			name: "cat",
			ins:  "Run: cat <path>. Prefer relative paths under workspace",
			res:  "Read-only; do not read secrets outside workspace",
			tags: "read,file",
			ent:  "builtin:cat",
		},
		{
			name: "browser",
			ins:  "Fetch a URL and return HTML/text. Implemented using curl -L",
			res:  "Do not send secrets; prefer https; keep outputs small",
			tags: "browser,http",
			ent:  "builtin:browser",
		},
	}

	for _, d := range defs {
		if err := upsertByName(dbClient, "tools", "tool_name", d.name, []utils.FieldData{
			{Name: "tool_name", Value: d.name},
			{Name: "usage_instructions", Value: d.ins},
			{Name: "usage_restrictions", Value: d.res},
			{Name: "category", Value: "builtin"},
			{Name: "tags", Value: d.tags},
			{Name: "source", Value: "builtin"},
			{Name: "version", Value: "1"},
			{Name: "path", Value: ""},
			{Name: "entrypoint", Value: d.ent},
			{Name: "enabled", Value: 1},
			{Name: "created_at", Value: now},
			{Name: "updated_at", Value: now},
		}); err != nil {
			return err
		}
	}
	return nil
}

func LoadAistoreIntoDB(dbClient *utils.SQLiteClient, basePath string) error {
	if basePath == "" {
		basePath = "/aistore"
	}
	info, err := os.Stat(basePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.WalkDir(basePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(d.Name()) != "manifest.json" {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var mf AistoreManifest
		if err := json.Unmarshal(raw, &mf); err != nil {
			return nil
		}
		if mf.Name == "" {
			return nil
		}

		kind := strings.ToLower(mf.Type)
		if kind == "" {
			kind = inferKindFromPath(basePath, path)
		}
		if kind == "" {
			return nil
		}

		if mf.Path == "" {
			mf.Path = filepath.Dir(path)
		}
		if mf.Source == "" {
			mf.Source = "aistore"
		}
		return upsertManifest(dbClient, kind, mf)
	})
}

func inferKindFromPath(basePath string, manifestPath string) string {
	rel, err := filepath.Rel(basePath, manifestPath)
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 0 {
		return ""
	}
	switch strings.ToLower(parts[0]) {
	case "tools":
		return "tool"
	case "skills":
		return "skill"
	case "mcp", "agents":
		return "mcp"
	default:
		return ""
	}
}

func upsertManifest(dbClient *utils.SQLiteClient, kind string, mf AistoreManifest) error {
	now := time.Now()
	tags := strings.Join(mf.Tags, ",")
	enabled := 1
	if mf.Enabled != nil && !*mf.Enabled {
		enabled = 0
	}

	var expiresAt interface{}
	if mf.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, mf.ExpiresAt); err == nil {
			expiresAt = t
		} else {
			expiresAt = mf.ExpiresAt
		}
	} else {
		expiresAt = nil
	}

	switch kind {
	case "tool":
		return upsertByName(dbClient, "tools", "tool_name", mf.Name, []utils.FieldData{
			{Name: "tool_name", Value: mf.Name},
			{Name: "usage_instructions", Value: mf.UsageInstructions},
			{Name: "usage_restrictions", Value: mf.UsageRestrictions},
			{Name: "category", Value: mf.Category},
			{Name: "tags", Value: tags},
			{Name: "source", Value: mf.Source},
			{Name: "version", Value: mf.Version},
			{Name: "expires_at", Value: expiresAt},
			{Name: "path", Value: mf.Path},
			{Name: "entrypoint", Value: mf.Entrypoint},
			{Name: "enabled", Value: enabled},
			{Name: "created_at", Value: now},
			{Name: "updated_at", Value: now},
		})
	case "skill":
		restricted := mf.RestrictedScenarios
		if restricted == "" {
			restricted = mf.UsageRestrictions
		}
		return upsertByName(dbClient, "skills", "skill_name", mf.Name, []utils.FieldData{
			{Name: "skill_name", Value: mf.Name},
			{Name: "skill_description", Value: mf.Description},
			{Name: "restricted_scenarios", Value: restricted},
			{Name: "category", Value: mf.Category},
			{Name: "tags", Value: tags},
			{Name: "source", Value: mf.Source},
			{Name: "version", Value: mf.Version},
			{Name: "expires_at", Value: expiresAt},
			{Name: "path", Value: mf.Path},
			{Name: "entrypoint", Value: mf.Entrypoint},
			{Name: "enabled", Value: enabled},
			{Name: "created_at", Value: now},
			{Name: "updated_at", Value: now},
		})
	case "mcp":
		return upsertByName(dbClient, "agents", "agent_name", mf.Name, []utils.FieldData{
			{Name: "agent_name", Value: mf.Name},
			{Name: "agent_description", Value: mf.Description},
			{Name: "capabilities", Value: mf.Capabilities},
			{Name: "category", Value: mf.Category},
			{Name: "tags", Value: tags},
			{Name: "source", Value: mf.Source},
			{Name: "version", Value: mf.Version},
			{Name: "expires_at", Value: expiresAt},
			{Name: "path", Value: mf.Path},
			{Name: "entrypoint", Value: mf.Entrypoint},
			{Name: "enabled", Value: enabled},
			{Name: "created_at", Value: now},
			{Name: "updated_at", Value: now},
		})
	default:
		return nil
	}
}

func upsertByName(dbClient *utils.SQLiteClient, tableName string, nameColumn string, nameValue string, fields []utils.FieldData) error {
	existing, err := dbClient.QueryData(tableName, &utils.FieldID{ColumnName: nameColumn, Value: nameValue}, 1, nil)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		return dbClient.InsertData(tableName, fields)
	}
	idVal, ok := existing[0]["id"]
	if !ok {
		return nil
	}
	var id interface{}
	switch v := idVal.(type) {
	case int64:
		id = v
	case int:
		id = v
	case []byte:
		id = string(v)
	default:
		id = v
	}
	updateFields := fields
	var hasCreatedAt bool
	for _, f := range updateFields {
		if f.Name == "created_at" {
			hasCreatedAt = true
			break
		}
	}
	if hasCreatedAt {
		var filtered []utils.FieldData
		for _, f := range updateFields {
			if f.Name == "created_at" {
				continue
			}
			filtered = append(filtered, f)
		}
		updateFields = filtered
	}
	return dbClient.UpdateData(tableName, updateFields, utils.FieldID{ColumnName: "id", Value: id})
}
