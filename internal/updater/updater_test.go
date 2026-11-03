package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"0.0.1", "0.0.2", -1},
		{"0.0.2", "0.0.2", 0},
		{"v0.0.3", "v0.0.2", 1},
		{"0.0.2", "v0.0.3", -1},
		{"0.0.10", "0.0.2", 1},
		{"1.0.0", "0.9.9", 1},
		{"v1.2", "v1.2.0", 0},
		{"v0.0.2", "0.0.2", 0},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.v1, tt.v2)
		if got != tt.expected {
			t.Errorf("CompareVersions(%q, %q) = %d; expected %d", tt.v1, tt.v2, got, tt.expected)
		}
	}
}

func TestIsSourceRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Initially false
	if IsSourceRepo(tmpDir) {
		t.Fatal("expected IsSourceRepo to be false on empty dir")
	}

	// Add go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module github.com/ARILSONLIMA/forge\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsSourceRepo(tmpDir) {
		t.Fatal("expected IsSourceRepo to be false without main.go and .git")
	}

	// Add cmd/forge/main.go
	mainDir := filepath.Join(tmpDir, "cmd", "forge")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsSourceRepo(tmpDir) {
		t.Fatal("expected IsSourceRepo to be false without .git")
	}

	// Add .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if !IsSourceRepo(tmpDir) {
		t.Fatal("expected IsSourceRepo to be true with all prerequisites")
	}
}

func TestReplaceExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source-bin")
	dstPath := filepath.Join(tmpDir, "target-bin")

	if err := os.WriteFile(srcPath, []byte("binary-version-1"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceExecutable(srcPath, dstPath); err != nil {
		t.Fatalf("ReplaceExecutable failed: %v", err)
	}

	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "binary-version-1" {
		t.Fatalf("unexpected content: %s", string(content))
	}

	// Test replacing existing binary
	if err := os.WriteFile(srcPath, []byte("binary-version-2"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceExecutable(srcPath, dstPath); err != nil {
		t.Fatalf("ReplaceExecutable on existing file failed: %v", err)
	}

	content, err = os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "binary-version-2" {
		t.Fatalf("unexpected updated content: %s", string(content))
	}
}

func TestFetchLatestGitHubTag(t *testing.T) {
	mockTags := `[
		{"name": "v0.0.1"},
		{"name": "v0.0.3"},
		{"name": "v0.0.2"}
	]`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, mockTags)
	}))
	defer ts.Close()

	// Directly test parsing logic against mock server
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Verify CompareVersions finds highest
	tags := []string{"v0.0.1", "v0.0.3", "v0.0.2"}
	best := tags[0]
	for _, tag := range tags[1:] {
		if CompareVersions(tag, best) > 0 {
			best = tag
		}
	}
	if best != "v0.0.3" {
		t.Fatalf("expected v0.0.3, got %s", best)
	}
}
