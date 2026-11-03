package testisolate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	goFailRegex   = regexp.MustCompile(`(?m)^---\s+FAIL:\s+([A-Za-z0-9_]+)`)
	pyFailRegex   = regexp.MustCompile(`(?m)^FAILED\s+[^:]+::([A-Za-z0-9_]+)`)
	jestFailRegex = regexp.MustCompile(`(?m)^(?:\s*✕|\s*●)\s+(?:.*?›\s*)?([^\r\n]+)`)
	nodeFailRegex = regexp.MustCompile(`(?m)^(?:✖|not ok \d+\s*-?)\s+(.*?)(?:\s+\(\d+.*?\))?$`)
)

// FailureReport stores the isolated failed test state.
type FailureReport struct {
	Language        string    `json:"language"`
	FailedTests     []string  `json:"failed_tests"`
	OriginalCommand string    `json:"original_command"`
	IsolatedCommand string    `json:"isolated_command"`
	UpdatedAt       string    `json:"updated_at"`
}

// DefaultPath returns the path to .forge/failed-tests.json.
func DefaultPath(rootDir string) string {
	return filepath.Join(rootDir, ".forge", "failed-tests.json")
}

// ParseFailures extracts test names from raw test execution output.
func ParseFailures(output string) []string {
	if output == "" {
		return nil
	}

	seen := make(map[string]bool)
	var list []string

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			seen[name] = true
			list = append(list, name)
		}
	}

	for _, m := range goFailRegex.FindAllStringSubmatch(output, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range pyFailRegex.FindAllStringSubmatch(output, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range jestFailRegex.FindAllStringSubmatch(output, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range nodeFailRegex.FindAllStringSubmatch(output, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}

	return list
}

// BuildIsolatedCommand returns a command targeted only at the failed tests.
func BuildIsolatedCommand(language string, failed []string) string {
	if len(failed) == 0 {
		return ""
	}

	switch strings.ToLower(language) {
	case "go":
		pattern := strings.Join(failed, "|")
		return fmt.Sprintf("go test -run '^(%s)$' ./...", pattern)
	case "python":
		pattern := strings.Join(failed, " or ")
		return fmt.Sprintf("pytest -k '%s'", pattern)
	case "rust":
		return fmt.Sprintf("cargo test -- %s", failed[0])
	case "typescript", "javascript":
		pattern := strings.Join(failed, "|")
		return fmt.Sprintf("npm test -- -t '%s'", pattern)
	default:
		return fmt.Sprintf("go test -run '%s' ./...", failed[0])
	}
}

// SaveReport records failing tests to .forge/failed-tests.json.
func SaveReport(rootDir, language, originalCmd, output string) (*FailureReport, error) {
	failed := ParseFailures(output)
	if len(failed) == 0 {
		return nil, nil
	}

	rep := &FailureReport{
		Language:        language,
		FailedTests:     failed,
		OriginalCommand: originalCmd,
		IsolatedCommand: BuildIsolatedCommand(language, failed),
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	path := DefaultPath(rootDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return nil, err
	}

	tmp := fmt.Sprintf("%s.%d.tmp", path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, err
	}

	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}

	return rep, nil
}

// LoadReport retrieves the latest recorded failure report.
func LoadReport(rootDir string) (*FailureReport, error) {
	path := DefaultPath(rootDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var rep FailureReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, err
	}
	return &rep, nil
}

// ClearReport removes the failed tests file once tests are green.
func ClearReport(rootDir string) {
	_ = os.Remove(DefaultPath(rootDir))
}
