package setup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllDependencies(t *testing.T) {
	deps := AllDependencies()
	if len(deps) < 6 {
		t.Fatalf("expected at least 6 dependencies, got %d", len(deps))
	}

	foundBD := false
	foundRTK := false
	foundSG := false

	for _, d := range deps {
		if d.Name == "bd (Beads DAG)" {
			foundBD = true
			if len(d.Binaries) == 0 || d.Binaries[0] != "bd" {
				t.Errorf("bd binary name expected 'bd', got %v", d.Binaries)
			}
		}
		if d.Name == "rtk" {
			foundRTK = true
		}
		if d.Name == "sg / ast-grep" {
			foundSG = true
		}
	}

	if !foundBD {
		t.Error("bd dependency missing from AllDependencies")
	}
	if !foundRTK {
		t.Error("rtk dependency missing from AllDependencies")
	}
	if !foundSG {
		t.Error("sg dependency missing from AllDependencies")
	}
}

func TestCheckDependencies(t *testing.T) {
	ctx := context.Background()
	statuses := CheckDependencies(ctx)
	if len(statuses) == 0 {
		t.Fatal("expected at least 1 dependency status")
	}

	for _, s := range statuses {
		if s.Installed && s.Path == "" {
			t.Errorf("dependency %s is installed but has empty path", s.Dep.Name)
		}
	}
}

func TestFormatDoctorReport(t *testing.T) {
	tmpDir := t.TempDir()
	statuses := []DependencyStatus{
		{
			Dep: Dependency{
				Name:        "test-tool",
				Required:    true,
				InstallTip:  "apt install test",
				Description: "testing tool",
			},
			Installed: false,
		},
		{
			Dep: Dependency{
				Name:        "git",
				Required:    true,
				Description: "version control",
			},
			Installed: true,
			Path:      "/usr/bin/git",
			Version:   "2.45.0",
		},
	}

	report := FormatDoctorReport(statuses, tmpDir)
	if report == "" {
		t.Fatal("report should not be empty")
	}
	if !strings.Contains(report, "test-tool") {
		t.Error("report should contain 'test-tool'")
	}
	if !strings.Contains(report, "[!] MISSING") {
		t.Error("report should contain '[!] MISSING'")
	}
	if !strings.Contains(report, "[✓] OK") {
		t.Error("report should contain '[✓] OK'")
	}
}

func TestEnsureBinInPath(t *testing.T) {
	tmpHome := t.TempDir()
	CustomRCDirectory = tmpHome
	defer func() { CustomRCDirectory = "" }()

	binDir := filepath.Join(tmpHome, "bin")

	// Create dummy .bashrc
	_ = os.WriteFile(filepath.Join(tmpHome, ".bashrc"), []byte("# test rc\n"), 0o644)

	added, files, err := EnsureBinInPath(binDir)
	if err != nil {
		t.Fatalf("EnsureBinInPath returned error: %v", err)
	}
	if !added {
		t.Error("expected binDir to be marked as added")
	}
	if len(files) == 0 {
		t.Error("expected at least 1 updated file")
	}
}
