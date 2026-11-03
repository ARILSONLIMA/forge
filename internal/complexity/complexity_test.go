package complexity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComplexityGuard(t *testing.T) {
	tmp := t.TempDir()

	// Create a file with a long method and deep nesting
	var code strings.Builder
	code.WriteString("package main\n\nfunc ProcessData() {\n")
	code.WriteString("    if true {\n")
	code.WriteString("        for i := 0; i < 10; i++ {\n")
	code.WriteString("            if i > 2 {\n")
	code.WriteString("                if i > 5 {\n")
	code.WriteString("                    // deep nesting\n")
	code.WriteString("                }\n")
	code.WriteString("            }\n")
	code.WriteString("        }\n")
	code.WriteString("    }\n")
	for i := 0; i < 60; i++ {
		code.WriteString("    println(\"extra line\")\n")
	}
	code.WriteString("}\n")

	filePath := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(filePath, []byte(code.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	violations, err := CheckFile(filePath, "main.go", Options{
		MaxFunctionLines: 50,
		MaxNestingDepth:  3,
	})
	if err != nil {
		t.Fatalf("check file error: %v", err)
	}

	if len(violations) < 2 {
		t.Fatalf("expected at least 2 violations (long method + deep nesting), got %d: %+v", len(violations), violations)
	}

	hasLong := false
	hasDeep := false
	for _, v := range violations {
		if v.Type == "LONG_METHOD" {
			hasLong = true
		}
		if v.Type == "DEEP_NESTING" {
			hasDeep = true
		}
	}

	if !hasLong || !hasDeep {
		t.Errorf("expected both LONG_METHOD and DEEP_NESTING, got %+v", violations)
	}

	rep := FormatViolations(violations)
	if !strings.Contains(rep, "ProcessData") {
		t.Errorf("unexpected report output: %s", rep)
	}
}
