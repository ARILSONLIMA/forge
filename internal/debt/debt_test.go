package debt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebtScanner(t *testing.T) {
	tmp := t.TempDir()

	code := `package main

// SHORTCUT: In-memory store used instead of Redis for MVP
func SaveUser() {}

// TODO(bd-04): Implement email verification
func VerifyEmail() {}

// HACK: Temporary sleep to avoid rate limiting
func CallAPI() {}
`
	filePath := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(filePath, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := Scan(tmp, []string{"main.go"})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 debt items, got %d", len(items))
	}

	if items[0].Tag != "SHORTCUT" || !strings.Contains(items[0].Message, "In-memory store") {
		t.Errorf("unexpected first item: %+v", items[0])
	}
	if items[1].Tag != "TODO" || items[1].Context != "bd-04" {
		t.Errorf("unexpected second item: %+v", items[1])
	}
	if items[2].Tag != "HACK" {
		t.Errorf("unexpected third item: %+v", items[2])
	}

	summary := FormatSummary(items)
	if !strings.Contains(summary, "SHORTCUT") || !strings.Contains(summary, "TODO(bd-04)") {
		t.Errorf("unexpected summary: %s", summary)
	}
}
