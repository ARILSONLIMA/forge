package hooks

import (
	"bytes"
	"strings"
	"testing"
)

func TestHandlePreTool(t *testing.T) {
	tmp := t.TempDir()

	// 1. Block .env read
	inputEnv := `{"tool_name": "read_file", "arguments": {"Path": ".env"}}`
	err := HandlePreTool(tmp, strings.NewReader(inputEnv))
	if err == nil || !strings.Contains(err.Error(), "[FORGE SECRET SHIELD]") {
		t.Fatalf("expected secret shield block on .env, got: %v", err)
	}

	// 2. Block large code slice (>100 lines)
	inputLarge := `{"tool_name": "read_file", "arguments": {"Path": "main.go", "StartLine": 1, "EndLine": 150}}`
	err = HandlePreTool(tmp, strings.NewReader(inputLarge))
	if err == nil || !strings.Contains(err.Error(), "[FORGE BLOCK]") {
		t.Fatalf("expected 100-line block on code read, got: %v", err)
	}

	// 3. Allow small code slice (<100 lines)
	inputSmall := `{"tool_name": "read_file", "arguments": {"Path": "main.go", "StartLine": 1, "EndLine": 50}}`
	err = HandlePreTool(tmp, strings.NewReader(inputSmall))
	if err != nil {
		t.Fatalf("unexpected block on small slice: %v", err)
	}

	// 4. Session start
	brief := HandleSessionStart(tmp)
	if !strings.Contains(brief, "[FORGE BRIEFING]") {
		t.Fatalf("unexpected session start output: %s", brief)
	}
}

func TestGenerateHookScripts(t *testing.T) {
	tmp := t.TempDir()
	if err := GenerateHookScripts(tmp, "forge"); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	_ = buf
}
