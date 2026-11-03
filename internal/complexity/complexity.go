package complexity

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Violation represents a code smell violation.
type Violation struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
	Type     string `json:"type"` // "LONG_METHOD" or "DEEP_NESTING"
	Value    int    `json:"value"`
	Limit    int    `json:"limit"`
}

// Options configures the complexity guard thresholds.
type Options struct {
	MaxFunctionLines int
	MaxNestingDepth  int
}

var (
	fnStartRegex = regexp.MustCompile(`^(?:func\s+(?:\([^)]*\)\s*)?([A-Za-z0-9_]+)|(?:export\s+)?(?:async\s+)?function\s+([A-Za-z0-9_$]+)|(?:const|let|var)\s+([A-Za-z0-9_$]+)\s*=\s*(?:async\s*)?\([^)]*\)\s*=>|(?:async\s+)?def\s+([A-Za-z0-9_]+))`)
)

// CheckFile inspects a single source file for Fowler complexity smells.
func CheckFile(filePath string, relPath string, opts Options) ([]Violation, error) {
	if opts.MaxFunctionLines <= 0 && opts.MaxNestingDepth <= 0 {
		return nil, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var violations []Violation
	scanner := bufio.NewScanner(f)

	type funcContext struct {
		name       string
		startLine  int
		baseIndent int
		braceDepth int
		maxNesting int
	}

	var currentFn *funcContext
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		rawLine := scanner.Text()
		trimmed := strings.TrimSpace(rawLine)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Detect function start
		if m := fnStartRegex.FindStringSubmatch(trimmed); len(m) > 1 {
			fnName := ""
			for i := 1; i < len(m); i++ {
				if m[i] != "" {
					fnName = m[i]
					break
				}
			}

			// If already in a function, check previous
			if currentFn != nil {
				fnLines := lineNum - currentFn.startLine
				if opts.MaxFunctionLines > 0 && fnLines > opts.MaxFunctionLines {
					violations = append(violations, Violation{
						File:     relPath,
						Line:     currentFn.startLine,
						Function: currentFn.name,
						Type:     "LONG_METHOD",
						Value:    fnLines,
						Limit:    opts.MaxFunctionLines,
					})
				}
				if opts.MaxNestingDepth > 0 && currentFn.maxNesting > opts.MaxNestingDepth {
					violations = append(violations, Violation{
						File:     relPath,
						Line:     currentFn.startLine,
						Function: currentFn.name,
						Type:     "DEEP_NESTING",
						Value:    currentFn.maxNesting,
						Limit:    opts.MaxNestingDepth,
					})
				}
			}

			indent := len(rawLine) - len(strings.TrimLeft(rawLine, " \t"))
			currentFn = &funcContext{
				name:       fnName,
				startLine:  lineNum,
				baseIndent: indent,
				braceDepth: strings.Count(trimmed, "{") - strings.Count(trimmed, "}"),
				maxNesting: 1,
			}
			continue
		}

		if currentFn != nil {
			// Update brace depth
			currentFn.braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

			// Check block nesting keywords: if, for, switch, while
			if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "for ") ||
				strings.HasPrefix(trimmed, "switch ") || strings.HasPrefix(trimmed, "while ") {
				depth := currentFn.braceDepth
				if depth > currentFn.maxNesting {
					currentFn.maxNesting = depth
				}
			}

			// End of function if braceDepth returns to 0 or negative
			if currentFn.braceDepth <= 0 && strings.Contains(trimmed, "}") {
				fnLines := lineNum - currentFn.startLine + 1
				if opts.MaxFunctionLines > 0 && fnLines > opts.MaxFunctionLines {
					violations = append(violations, Violation{
						File:     relPath,
						Line:     currentFn.startLine,
						Function: currentFn.name,
						Type:     "LONG_METHOD",
						Value:    fnLines,
						Limit:    opts.MaxFunctionLines,
					})
				}
				if opts.MaxNestingDepth > 0 && currentFn.maxNesting > opts.MaxNestingDepth {
					violations = append(violations, Violation{
						File:     relPath,
						Line:     currentFn.startLine,
						Function: currentFn.name,
						Type:     "DEEP_NESTING",
						Value:    currentFn.maxNesting,
						Limit:    opts.MaxNestingDepth,
					})
				}
				currentFn = nil
			}
		}
	}

	if currentFn != nil {
		fnLines := lineNum - currentFn.startLine + 1
		if opts.MaxFunctionLines > 0 && fnLines > opts.MaxFunctionLines {
			violations = append(violations, Violation{
				File:     relPath,
				Line:     currentFn.startLine,
				Function: currentFn.name,
				Type:     "LONG_METHOD",
				Value:    fnLines,
				Limit:    opts.MaxFunctionLines,
			})
		}
		if opts.MaxNestingDepth > 0 && currentFn.maxNesting > opts.MaxNestingDepth {
			violations = append(violations, Violation{
				File:     relPath,
				Line:     currentFn.startLine,
				Function: currentFn.name,
				Type:     "DEEP_NESTING",
				Value:    currentFn.maxNesting,
				Limit:    opts.MaxNestingDepth,
			})
		}
	}

	return violations, scanner.Err()
}

// CheckFiles inspects multiple files in rootDir.
func CheckFiles(rootDir string, files []string, opts Options) ([]Violation, error) {
	var all []Violation
	for _, rel := range files {
		ext := strings.ToLower(filepath.Ext(rel))
		if ext != ".go" && ext != ".ts" && ext != ".js" && ext != ".py" {
			continue
		}
		full := filepath.Join(rootDir, rel)
		v, err := CheckFile(full, rel, opts)
		if err != nil {
			continue
		}
		all = append(all, v...)
	}
	return all, nil
}

// FormatViolations renders violations into human-readable report.
func FormatViolations(violations []Violation) string {
	if len(violations) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚠️ Violações de Complexidade Estrutural (%d ocorrências):\n", len(violations)))
	for _, v := range violations {
		if v.Type == "LONG_METHOD" {
			sb.WriteString(fmt.Sprintf("  • %s:%d função '%s' tem %d linhas (limite: %d)\n", v.File, v.Line, v.Function, v.Value, v.Limit))
		} else if v.Type == "DEEP_NESTING" {
			sb.WriteString(fmt.Sprintf("  • %s:%d função '%s' possui aninhamento de profundidade %d (limite: %d)\n", v.File, v.Line, v.Function, v.Value, v.Limit))
		}
	}
	return sb.String()
}
