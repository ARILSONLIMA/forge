// Package templates embeds Forge workflows, stage rules, ast-grep rules and MCP
// config, and performs dynamic context swapping of .agent/rules/active-stage.md.
package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ARILSONLIMA/forge/internal/state"
)

//go:embed all:assets
var assets embed.FS

// MaxStageTokens is the budget for the active stage rule file.
const MaxStageTokens = 250

// ActiveStagePath is the dynamic rule file, relative to the project root.
const ActiveStagePath = ".agent/rules/active-stage.md"

// Stages lists the 7 workflow stages in order.
var Stages = []state.Stage{
	state.StageSpec, state.StagePlan, state.StageBuild, state.StageTest,
	state.StageReview, state.StageSimplify, state.StageShip,
}

// EstimateTokens approximates token count (~4 chars per token).
func EstimateTokens(s string) int {
	return (len([]rune(s)) + 3) / 4
}

// RenderStage returns the rule text for a stage with placeholders replaced.
// failureSummary, when non-empty, is prepended (auto-rollback context injection).
func RenderStage(st *state.State, failureSummary string) (string, error) {
	data, err := assets.ReadFile("assets/stages/" + strings.ToLower(string(st.CurrentStage)) + ".md")
	if err != nil {
		return "", fmt.Errorf("stage template %s: %w", st.CurrentStage, err)
	}
	spec := st.ActiveSpec
	if spec == "" {
		spec = ".forge/specs/<slug>.md"
	}
	bead := st.ActiveBeadID
	if bead == "" {
		bead = "bd-?"
	}
	out := strings.NewReplacer("{{ACTIVE_BEAD_ID}}", bead, "{{ACTIVE_SPEC}}", spec).Replace(string(data))
	if failureSummary != "" {
		out = "## ÚLTIMA FALHA DE TESTE\n" + strings.TrimSpace(failureSummary) + "\n\n" + out
	}
	return out, nil
}

// SwapStage atomically overwrites .agent/rules/active-stage.md with the
// template for the state's current stage. Returns the written content.
func SwapStage(root string, st *state.State) (string, error) {
	content, err := RenderStage(st, st.LastTestFailureSummary)
	if err != nil {
		return "", err
	}
	if st.LastTestFailureSummary == "" {
		if n := EstimateTokens(content); n > MaxStageTokens {
			return "", fmt.Errorf("stage %s rule has ~%d tokens (max %d)", st.CurrentStage, n, MaxStageTokens)
		}
	}
	if err := writeAtomic(filepath.Join(root, ActiveStagePath), []byte(content)); err != nil {
		return "", err
	}
	return content, nil
}

// WriteWorkflows writes the 7 slash-command workflows to .agent/workflows/.
func WriteWorkflows(root string) error {
	for _, st := range Stages {
		name := strings.ToLower(string(st)) + ".md"
		data, err := assets.ReadFile("assets/workflows/" + name)
		if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(root, ".agent/workflows", name), data); err != nil {
			return err
		}
	}
	return nil
}

// WriteRules writes the ast-grep rules for a language into rulesDir
// (absolute, or relative to root). Unknown languages fall back to go.
func WriteRules(root, rulesDir, language string) error {
	dir := "assets/rules/" + language
	if _, err := fs.Stat(assets, dir); err != nil {
		dir = "assets/rules/go"
	}
	if !filepath.IsAbs(rulesDir) {
		rulesDir = filepath.Join(root, rulesDir)
	}
	entries, err := fs.ReadDir(assets, dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		data, err := assets.ReadFile(dir + "/" + e.Name())
		if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(rulesDir, e.Name()), data); err != nil {
			return err
		}
	}
	return nil
}

// WriteMCP writes .mcp.json (Serena + Forge). An existing file is left untouched
// and false is returned, so user MCP servers are never clobbered.
func WriteMCP(root string) (bool, error) {
	path := filepath.Join(root, ".mcp.json")
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	data, err := assets.ReadFile("assets/mcp/mcp.json")
	if err != nil {
		return false, err
	}
	return true, writeAtomic(path, data)
}

// WriteHooks writes default Antigravity lifecycle hooks to .agent/hooks/.
func WriteHooks(root, forgeBin string) error {
	if forgeBin == "" {
		forgeBin = "forge"
	}
	pretool := fmt.Sprintf("#!/usr/bin/env bash\n# Forge PreToolUse Hook\n%s hook pretool\n", forgeBin)
	sessionstart := fmt.Sprintf("#!/usr/bin/env bash\n# Forge SessionStart Hook\n%s hook sessionstart\n", forgeBin)

	if err := writeAtomic(filepath.Join(root, ".agent/hooks/forge-pretool.sh"), []byte(pretool)); err != nil {
		return err
	}
	_ = os.Chmod(filepath.Join(root, ".agent/hooks/forge-pretool.sh"), 0o755)

	if err := writeAtomic(filepath.Join(root, ".agent/hooks/forge-sessionstart.sh"), []byte(sessionstart)); err != nil {
		return err
	}
	_ = os.Chmod(filepath.Join(root, ".agent/hooks/forge-sessionstart.sh"), 0o755)

	return nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
