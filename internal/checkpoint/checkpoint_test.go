package checkpoint

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
	}

	run("init")
	run("config", "user.name", "Forge Test")
	run("config", "user.email", "forge@test.local")

	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial commit")
}

func TestCheckpointManager(t *testing.T) {
	tmp := t.TempDir()
	initGitRepo(t, tmp)

	mgr := NewManager(tmp)

	// 1. Create first checkpoint
	chk1, err := mgr.Create("green-test")
	if err != nil {
		t.Fatalf("create checkpoint err: %v", err)
	}
	if chk1.Label != "green-test" || chk1.CommitHash == "" {
		t.Errorf("unexpected checkpoint data: %+v", chk1)
	}

	// 2. Modify file and commit
	filePath := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(filePath, []byte("second version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmdCommit := exec.Command("git", "commit", "-am", "second commit")
	cmdCommit.Dir = tmp
	if err := cmdCommit.Run(); err != nil {
		t.Fatal(err)
	}

	// 3. Create second checkpoint
	chk2, err := mgr.Create("pre-simplify")
	if err != nil {
		t.Fatalf("create second checkpoint err: %v", err)
	}
	if chk2.Label != "pre-simplify" {
		t.Errorf("unexpected label: %s", chk2.Label)
	}

	// 4. List checkpoints
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("list err: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 checkpoints, got %d", len(list))
	}

	// 5. Corrupt file
	if err := os.WriteFile(filePath, []byte("broken corrupt code\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 6. Rollback to first checkpoint
	target, err := mgr.Rollback(chk1.ID)
	if err != nil {
		t.Fatalf("rollback error: %v", err)
	}
	if target.ID != chk1.ID {
		t.Errorf("expected rolled back to %s, got %s", chk1.ID, target.ID)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "initial\n" {
		t.Errorf("expected content to be restored to 'initial\n', got %q", string(content))
	}
}
