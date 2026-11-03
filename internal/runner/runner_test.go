package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ARILSONLIMA/forge/internal/state"
)

func TestCheckRawRead(t *testing.T) {
	tmpDir := t.TempDir()

	// Create small code file (50 lines)
	smallFile := filepath.Join(tmpDir, "small.go")
	var smallContent strings.Builder
	for i := 1; i <= 50; i++ {
		smallContent.WriteString("// line\n")
	}
	if err := os.WriteFile(smallFile, []byte(smallContent.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create large code file (150 lines)
	largeFile := filepath.Join(tmpDir, "large.go")
	var largeContent strings.Builder
	for i := 1; i <= 150; i++ {
		largeContent.WriteString("// line\n")
	}
	if err := os.WriteFile(largeFile, []byte(largeContent.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create large markdown file (150 lines)
	largeMD := filepath.Join(tmpDir, "large.md")
	if err := os.WriteFile(largeMD, []byte(largeContent.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// Case 1: IDLE stage (guard inactive)
	err := CheckRawRead(state.StageIdle, []string{"cat", largeFile}, tmpDir)
	if err != nil {
		t.Fatalf("expected nil in IDLE, got %v", err)
	}

	// Case 2: BUILD stage with small code file -> allowed
	err = CheckRawRead(state.StageBuild, []string{"cat", smallFile}, tmpDir)
	if err != nil {
		t.Fatalf("expected nil for small code file, got %v", err)
	}

	// Case 3: BUILD stage with large non-code file -> allowed
	err = CheckRawRead(state.StageBuild, []string{"cat", largeMD}, tmpDir)
	if err != nil {
		t.Fatalf("expected nil for non-code file, got %v", err)
	}

	// Case 4: BUILD stage with large code file -> BLOCKED
	err = CheckRawRead(state.StageBuild, []string{"cat", largeFile}, tmpDir)
	if err == nil {
		t.Fatal("expected error blocking large code file in BUILD, got nil")
	}
	if !strings.Contains(err.Error(), "[FORGE BLOCK]") {
		t.Fatalf("expected '[FORGE BLOCK]' in error message, got %v", err)
	}

	// Case 5: REVIEW stage with head on large code file -> BLOCKED
	err = CheckRawRead(state.StageReview, []string{"head", "-n", "20", largeFile}, tmpDir)
	if err == nil {
		t.Fatal("expected error blocking head on large code file in REVIEW, got nil")
	}

	// Case 6: SIMPLIFY stage with tail on large code file -> BLOCKED
	err = CheckRawRead(state.StageSimplify, []string{"tail", largeFile}, tmpDir)
	if err == nil {
		t.Fatal("expected error blocking tail on large code file in SIMPLIFY, got nil")
	}

	// Case 7: BUILD stage with non-read command (e.g. go vet) -> allowed
	err = CheckRawRead(state.StageBuild, []string{"go", "vet", largeFile}, tmpDir)
	if err != nil {
		t.Fatalf("expected nil for non-read command, got %v", err)
	}
}

func TestRewriteForRTK(t *testing.T) {
	rtkBin := "/usr/local/bin/rtk"

	// 1. Without RTK available
	res := RewriteForRTK([]string{"git", "diff"}, "")
	if len(res) != 2 || res[0] != "git" {
		t.Fatalf("expected unmodified args without rtk, got %v", res)
	}

	// 2. Supported command: git diff -> rtk git diff
	res = RewriteForRTK([]string{"git", "diff"}, rtkBin)
	if len(res) != 3 || res[0] != rtkBin || res[1] != "git" || res[2] != "diff" {
		t.Fatalf("expected rtk git diff, got %v", res)
	}

	// 3. Supported command: go test ./... -> rtk go test ./...
	res = RewriteForRTK([]string{"go", "test", "./..."}, rtkBin)
	if len(res) != 4 || res[0] != rtkBin || res[1] != "go" {
		t.Fatalf("expected rtk go test ./..., got %v", res)
	}

	// 4. Command already prefixed with rtk -> unchanged
	res = RewriteForRTK([]string{"rtk", "git", "status"}, rtkBin)
	if len(res) != 3 || res[0] != "rtk" {
		t.Fatalf("expected unchanged rtk invocation, got %v", res)
	}

	// 5. Unsupported generic command -> unchanged
	res = RewriteForRTK([]string{"echo", "hello"}, rtkBin)
	if len(res) != 2 || res[0] != "echo" {
		t.Fatalf("expected unchanged generic command, got %v", res)
	}
}

func TestRunnerRun(t *testing.T) {
	r := &Runner{
		RootDir: t.TempDir(),
		RTKPath: "", // Test native fallback
	}

	res, err := r.Run(context.Background(), state.StageBuild, []string{"echo", "forge runner test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Stdout, "forge runner test") {
		t.Fatalf("expected stdout to contain test string, got %q", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", res.ExitCode)
	}
}
