package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ARILSONLIMA/forge/internal/briefing"
	"github.com/ARILSONLIMA/forge/internal/runner"
	"github.com/ARILSONLIMA/forge/internal/state"
)

// PreToolInput represents JSON payload passed to PreToolUse hook.
type PreToolInput struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// HandlePreTool inspects a tool call and returns an error if blocked.
func HandlePreTool(rootDir string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var input PreToolInput
	if err := json.Unmarshal(data, &input); err != nil {
		// Not JSON payload, pass through
		return nil
	}

	st, _ := state.Load(filepath.Join(rootDir, state.DefaultPath))
	currentStage := state.StageBuild
	if st != nil {
		currentStage = st.CurrentStage
	}

	tool := strings.ToLower(input.ToolName)

	switch tool {
	case "read_file", "view_file":
		pathVal, _ := input.Arguments["Path"].(string)
		if pathVal == "" {
			pathVal, _ = input.Arguments["path"].(string)
		}
		pathVal = cleanPath(pathVal)

		// 1. Secret shield
		baseName := strings.ToLower(filepath.Base(pathVal))
		if strings.HasPrefix(baseName, ".env") {
			return fmt.Errorf("[FORGE SECRET SHIELD] Leitura de arquivo .env bloqueada. Use 'forge env' ou 'forge doctor'.")
		}

		// 2. Large code read guard
		ext := strings.ToLower(filepath.Ext(pathVal))
		if runner.SourceExtensions[ext] {
			startLine := getInt(input.Arguments, "StartLine", "start_line")
			endLine := getInt(input.Arguments, "EndLine", "end_line")

			if endLine > 0 && startLine > 0 {
				if endLine-startLine > 100 {
					return fmt.Errorf("[FORGE BLOCK] Leitura de %d linhas em %s excede o teto cirúrgico de 100 linhas. Use fatias menores ou 'forge map'.", endLine-startLine, pathVal)
				}
			} else if endLine == 0 && startLine == 0 {
				// No lines specified, check file size
				fullPath := pathVal
				if !filepath.IsAbs(fullPath) && rootDir != "" {
					fullPath = filepath.Join(rootDir, fullPath)
				}
				if fi, err := os.Stat(fullPath); err == nil && !fi.IsDir() {
					// Approximate lines (> 4000 bytes)
					if fi.Size() > 4000 {
						return fmt.Errorf("[FORGE BLOCK] Leitura bruta sem StartLine/EndLine em arquivo de código grande (%s). Especifique linhas ou use Serena LSP / 'forge map'.", pathVal)
					}
				}
			}
		}

	case "run_command":
		cmdLine, _ := input.Arguments["CommandLine"].(string)
		if cmdLine == "" {
			cmdLine, _ = input.Arguments["command"].(string)
		}
		parts := strings.Fields(cmdLine)
		if err := runner.CheckRawRead(currentStage, parts, rootDir); err != nil {
			return err
		}
	}

	return nil
}

// HandleSessionStart outputs the briefing for session startup.
func HandleSessionStart(rootDir string) string {
	b, _ := briefing.Generate(rootDir, true)
	return b
}

func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "file://")
	return p
}

func getInt(m map[string]interface{}, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch num := v.(type) {
			case float64:
				return int(num)
			case int:
				return num
			}
		}
	}
	return 0
}

// GenerateHookScripts writes hook script wrappers to .agent/hooks/
func GenerateHookScripts(rootDir, forgeBinPath string) error {
	hooksDir := filepath.Join(rootDir, ".agent", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return err
	}

	if forgeBinPath == "" {
		forgeBinPath = "forge"
	}

	pretoolContent := fmt.Sprintf(`#!/usr/bin/env bash
# Forge PreToolUse Hook
%s hook pretool
`, forgeBinPath)

	sessionStartContent := fmt.Sprintf(`#!/usr/bin/env bash
# Forge SessionStart Hook
%s hook sessionstart
`, forgeBinPath)

	if err := os.WriteFile(filepath.Join(hooksDir, "forge-pretool.sh"), []byte(pretoolContent), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "forge-sessionstart.sh"), []byte(sessionStartContent), 0o755); err != nil {
		return err
	}

	return nil
}
