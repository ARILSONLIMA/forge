package symbols

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DefaultBudgetChars is the maximum characters allowed for the symbol map (~450 tokens).
const DefaultBudgetChars = 2000

var (
	goFuncRegex = regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Z]\w*)`)
	goTypeRegex = regexp.MustCompile(`^type\s+([A-Z]\w*)`)

	tsExportFn   = regexp.MustCompile(`^export\s+(?:default\s+)?(?:async\s+)?(?:function\*?|class)\s+([A-Za-z_$][\w$]*)`)
	tsExportDecl = regexp.MustCompile(`^export\s+(?:const|let|var|interface|type|enum)\s+([A-Za-z_$][\w$]*)`)

	pyDefRegex   = regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z]\w*)`)
	pyClassRegex = regexp.MustCompile(`^class\s+([A-Za-z]\w*)`)

	rsPubRegex = regexp.MustCompile(`^pub\s+(?:async\s+)?(?:fn|struct|enum|trait|type)\s+(\w+)`)

	skipDirRegex  = regexp.MustCompile(`(^|/)(node_modules|\.git|dist|build|out|vendor|venv|\.venv|__pycache__|target|\.agent|\.forge|coverage)(/|$)`)
	skipFileRegex = regexp.MustCompile(`(^|/)(test_[^/]*|[^/]*(_test|\.test|\.spec|\.gen|\.min|\.d)\.[a-z]+)$`)
)

// MapOptions configures codebase map generation.
type MapOptions struct {
	RootDir     string
	BudgetChars int
}

// GenerateCodebaseMap scans source code files in rootDir and returns a condensed map of public symbols.
func GenerateCodebaseMap(opts MapOptions) (string, error) {
	if opts.BudgetChars <= 0 {
		opts.BudgetChars = DefaultBudgetChars
	}
	if opts.RootDir == "" {
		opts.RootDir = "."
	}

	files := listFiles(opts.RootDir)
	sort.Strings(files)

	var sb strings.Builder
	sb.WriteString("=== FORGE CODEBASE MAP (Public Symbols) ===\n")

	for _, file := range files {
		symbols := extractSymbols(opts.RootDir, file)
		if len(symbols) == 0 {
			continue
		}

		line := fmt.Sprintf("%s: %s\n", file, strings.Join(symbols, ", "))
		if sb.Len()+len(line) > opts.BudgetChars {
			sb.WriteString("... [truncated to preserve context budget]\n")
			break
		}
		sb.WriteString(line)
	}

	return sb.String(), nil
}

func listFiles(rootDir string) []string {
	// 1. Try git ls-files first
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		var result []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" || skipDirRegex.MatchString(l) || skipFileRegex.MatchString(l) {
				continue
			}
			if isSupportedCodeFile(l) {
				result = append(result, l)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	// 2. Fallback to WalkDir
	var result []string
	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(rootDir, path)
		if d.IsDir() {
			if skipDirRegex.MatchString(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if skipFileRegex.MatchString(rel) {
			return nil
		}
		if isSupportedCodeFile(rel) {
			result = append(result, rel)
		}
		return nil
	})
	return result
}

func isSupportedCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs":
		return true
	default:
		return false
	}
}

func extractSymbols(rootDir, relPath string) []string {
	fullPath := filepath.Join(rootDir, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(relPath))
	scanner := bufio.NewScanner(bytes.NewReader(data))
	seen := make(map[string]bool)
	var symbols []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}

		var found string
		switch ext {
		case ".go":
			if m := goFuncRegex.FindStringSubmatch(line); len(m) > 1 {
				found = m[1]
			} else if m := goTypeRegex.FindStringSubmatch(line); len(m) > 1 {
				found = m[1]
			}
		case ".ts", ".tsx", ".js", ".jsx":
			if m := tsExportFn.FindStringSubmatch(line); len(m) > 1 {
				found = m[1]
			} else if m := tsExportDecl.FindStringSubmatch(line); len(m) > 1 {
				found = m[1]
			}
		case ".py":
			if m := pyClassRegex.FindStringSubmatch(line); len(m) > 1 {
				found = m[1]
			} else if m := pyDefRegex.FindStringSubmatch(line); len(m) > 1 {
				if !strings.HasPrefix(m[1], "_") {
					found = m[1]
				}
			}
		case ".rs":
			if m := rsPubRegex.FindStringSubmatch(line); len(m) > 1 {
				found = m[1]
			}
		}

		if found != "" && !seen[found] {
			seen[found] = true
			symbols = append(symbols, found)
		}
	}

	return symbols
}
