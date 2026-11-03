package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProject(t *testing.T) {
	tests := []struct {
		name         string
		markerFile   string
		wantLanguage string
		wantTestCmd  string
	}{
		{
			name:         "go project",
			markerFile:   "go.mod",
			wantLanguage: "go",
			wantTestCmd:  "go test ./...",
		},
		{
			name:         "typescript project",
			markerFile:   "package.json",
			wantLanguage: "typescript",
			wantTestCmd:  "npm test",
		},
		{
			name:         "python project",
			markerFile:   "pyproject.toml",
			wantLanguage: "python",
			wantTestCmd:  "pytest",
		},
		{
			name:         "rust project",
			markerFile:   "Cargo.toml",
			wantLanguage: "rust",
			wantTestCmd:  "cargo test",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.markerFile), []byte(""), 0o644); err != nil {
				t.Fatalf("write marker: %v", err)
			}
			cfg := DetectProject(dir)
			if cfg.Language != tc.wantLanguage {
				t.Fatalf("got language %q, want %q", cfg.Language, tc.wantLanguage)
			}
			if cfg.Commands.Test != tc.wantTestCmd {
				t.Fatalf("got test cmd %q, want %q", cfg.Commands.Test, tc.wantTestCmd)
			}
		})
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".forge", "config.yaml")

	want := Config{
		Version:  "1",
		Language: "rust",
		Commands: Commands{
			Build: "cargo check",
			Test:  "cargo test",
			Lint:  "cargo clippy -- -D warnings",
		},
		GateThresholds: GateThresholds{
			MaxDiffLinesPerTicket:     250,
			ForbidUnreferencedExports: true,
			AstGrepStrict:             false,
		},
		Paths: Paths{
			SpecsDir: ".forge/specs",
			RulesDir: ".forge/rules",
		},
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	// Test strict_tools override to true
	want.StrictTools = true
	if err := Save(path, want); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	gotStrict, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !gotStrict.StrictTools {
		t.Fatalf("expected StrictTools to be true, got %t", gotStrict.StrictTools)
	}
}
