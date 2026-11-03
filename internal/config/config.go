package config

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Commands holds the project native build, test, and lint commands.
type Commands struct {
	Build string `json:"build"`
	Test  string `json:"test"`
	Lint  string `json:"lint"`
}

// GateThresholds holds deterministic gate limits.
type GateThresholds struct {
	MaxDiffLinesPerTicket     int  `json:"max_diff_lines_per_ticket"`
	ForbidUnreferencedExports bool `json:"forbid_unreferenced_exports"`
	AstGrepStrict             bool `json:"ast_grep_strict"`
	MaxFunctionLines          int  `json:"max_function_lines"`
	MaxNestingDepth           int  `json:"max_nesting_depth"`
}

// Paths holds configurable Forge directories.
type Paths struct {
	SpecsDir string `json:"specs_dir"`
	RulesDir string `json:"rules_dir"`
}

// Config represents the .forge/config.yaml schema.
type Config struct {
	Version        string         `json:"version"`
	Language       string         `json:"language"`
	StrictTools    bool           `json:"strict_tools"`
	Commands       Commands       `json:"commands"`
	GateThresholds GateThresholds `json:"gate_thresholds"`
	Paths          Paths          `json:"paths"`
}

// DefaultPath is the standard location of the config file relative to project root.
const DefaultPath = ".forge/config.yaml"

// DetectProject inspects the given root directory and returns a Config
// populated with detected language defaults.
func DetectProject(rootDir string) Config {
	cfg := Config{
		Version:     "1",
		Language:    "go",
		StrictTools: true,
		Commands: Commands{
			Build: "go build ./...",
			Test:  "go test ./...",
			Lint:  "go vet ./...",
		},
		GateThresholds: GateThresholds{
			MaxDiffLinesPerTicket:     300,
			ForbidUnreferencedExports: true,
			AstGrepStrict:             true,
			MaxFunctionLines:          50,
			MaxNestingDepth:           4,
		},
		Paths: Paths{
			SpecsDir: ".forge/specs",
			RulesDir: ".forge/rules",
		},
	}

	switch {
	case fileExists(filepath.Join(rootDir, "go.mod")):
		cfg.Language = "go"
		cfg.Commands.Build = "go build ./..."
		cfg.Commands.Test = "go test ./..."
		if _, err := exec.LookPath("golangci-lint"); err == nil {
			cfg.Commands.Lint = "golangci-lint run"
		} else {
			cfg.Commands.Lint = "go vet ./..."
		}
	case fileExists(filepath.Join(rootDir, "package.json")):
		cfg.Language = "typescript"
		cfg.Commands.Build = "npm run build --if-present"
		cfg.Commands.Test = "npm test"
		cfg.Commands.Lint = "npm run lint --if-present"
	case fileExists(filepath.Join(rootDir, "pyproject.toml")) || fileExists(filepath.Join(rootDir, "requirements.txt")):
		cfg.Language = "python"
		cfg.Commands.Build = "python -m py_compile $(git ls-files '*.py')"
		cfg.Commands.Test = "pytest"
		cfg.Commands.Lint = "ruff check ."
	case fileExists(filepath.Join(rootDir, "Cargo.toml")):
		cfg.Language = "rust"
		cfg.Commands.Build = "cargo check"
		cfg.Commands.Test = "cargo test"
		cfg.Commands.Lint = "cargo clippy -- -D warnings"
	}

	return cfg
}

// MarshalYAML renders Config into the canonical .forge/config.yaml format.
func (c Config) MarshalYAML() string {
	return fmt.Sprintf(`# .forge/config.yaml
version: %q
language: %q # Opções: go, typescript, python, rust
strict_tools: %t

commands:
  build: %q
  test: %q
  lint: %q

gate_thresholds:
  max_diff_lines_per_ticket: %d
  forbid_unreferenced_exports: %t
  ast_grep_strict: %t
  max_function_lines: %d
  max_nesting_depth: %d

paths:
  specs_dir: %q
  rules_dir: %q
`,
		c.Version,
		c.Language,
		c.StrictTools,
		c.Commands.Build,
		c.Commands.Test,
		c.Commands.Lint,
		c.GateThresholds.MaxDiffLinesPerTicket,
		c.GateThresholds.ForbidUnreferencedExports,
		c.GateThresholds.AstGrepStrict,
		c.GateThresholds.MaxFunctionLines,
		c.GateThresholds.MaxNestingDepth,
		c.Paths.SpecsDir,
		c.Paths.RulesDir,
	)
}

// Save writes the configuration atomically to path.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(cfg.MarshalYAML()), 0o644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp config: %w", err)
	}
	return nil
}

// Load reads and parses .forge/config.yaml from path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return ParseYAML(string(data))
}

// ParseYAML parses the .forge/config.yaml structure without external dependencies.
func ParseYAML(content string) (Config, error) {
	cfg := DetectProject(".")
	scanner := bufio.NewScanner(strings.NewReader(content))
	section := ""

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := stripYAMLComment(rawLine)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		isIndented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := unquoteYAML(strings.TrimSpace(parts[1]))

		if !isIndented {
			if val == "" {
				section = key
				continue
			}
			section = ""
			switch key {
			case "version":
				cfg.Version = val
			case "language":
				cfg.Language = val
			case "strict_tools":
				if b, err := strconv.ParseBool(val); err == nil {
					cfg.StrictTools = b
				}
			case "build_cmd":
				cfg.Commands.Build = val
			case "test_cmd":
				cfg.Commands.Test = val
			case "lint_cmd":
				cfg.Commands.Lint = val
			}
			continue
		}

		switch section {
		case "commands":
			switch key {
			case "build":
				cfg.Commands.Build = val
			case "test":
				cfg.Commands.Test = val
			case "lint":
				cfg.Commands.Lint = val
			}
		case "gate_thresholds":
			switch key {
			case "max_diff_lines_per_ticket":
				if n, err := strconv.Atoi(val); err == nil {
					cfg.GateThresholds.MaxDiffLinesPerTicket = n
				}
			case "forbid_unreferenced_exports":
				if b, err := strconv.ParseBool(val); err == nil {
					cfg.GateThresholds.ForbidUnreferencedExports = b
				}
			case "ast_grep_strict":
				if b, err := strconv.ParseBool(val); err == nil {
					cfg.GateThresholds.AstGrepStrict = b
				}
			case "max_function_lines":
				if n, err := strconv.Atoi(val); err == nil {
					cfg.GateThresholds.MaxFunctionLines = n
				}
			case "max_nesting_depth":
				if n, err := strconv.Atoi(val); err == nil {
					cfg.GateThresholds.MaxNestingDepth = n
				}
			}
		case "paths":
			switch key {
			case "specs_dir":
				cfg.Paths.SpecsDir = val
			case "rules_dir":
				cfg.Paths.RulesDir = val
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func stripYAMLComment(s string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && (i == 0 || s[i-1] != '\\') {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return s[:i]
			}
		}
	}
	return s
}

func unquoteYAML(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			if u, err := strconv.Unquote(s); err == nil {
				return u
			}
			return s[1 : len(s)-1]
		}
	}
	return s
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
