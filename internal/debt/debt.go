package debt

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	debtRegex = regexp.MustCompile(`(?i)(?://|#|/\*)\s*(SHORTCUT|TODO|HACK|FIXME)(?:\(([^)]+)\))?:\s*(.*)`)

	skipDirRegex = regexp.MustCompile(`(^|/)(node_modules|\.git|dist|build|out|vendor|venv|\.venv|__pycache__|target|\.agent|\.forge|coverage)(/|$)`)
)

// Item represents a detected technical debt marker.
type Item struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Tag     string `json:"tag"`
	Context string `json:"context,omitempty"`
	Message string `json:"message"`
}

// Scan scans rootDir or the specified files for technical debt markers.
func Scan(rootDir string, files []string) ([]Item, error) {
	if rootDir == "" {
		rootDir = "."
	}

	if len(files) == 0 {
		files = listTrackedFiles(rootDir)
	}

	sort.Strings(files)
	var items []Item

	for _, relPath := range files {
		fullPath := filepath.Join(rootDir, relPath)
		fileItems, err := scanFile(rootDir, relPath, fullPath)
		if err != nil {
			continue
		}
		items = append(items, fileItems...)
	}

	return items, nil
}

func scanFile(rootDir, relPath, fullPath string) ([]Item, error) {
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var items []Item
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		matches := debtRegex.FindStringSubmatch(line)
		if len(matches) > 3 {
			tag := strings.ToUpper(matches[1])
			ctx := matches[2]
			msg := strings.TrimSpace(matches[3])
			msg = strings.TrimSuffix(msg, "*/")
			msg = strings.TrimSpace(msg)

			items = append(items, Item{
				File:    relPath,
				Line:    lineNum,
				Tag:     tag,
				Context: ctx,
				Message: msg,
			})
		}
	}

	return items, scanner.Err()
}

func listTrackedFiles(rootDir string) []string {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		var result []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" || skipDirRegex.MatchString(l) {
				continue
			}
			result = append(result, l)
		}
		if len(result) > 0 {
			return result
		}
	}

	// Fallback
	var result []string
	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && skipDirRegex.MatchString(path) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(rootDir, path)
		if !skipDirRegex.MatchString(rel) {
			result = append(result, rel)
		}
		return nil
	})
	return result
}

// FormatSummary formats debt items for human-readable output.
func FormatSummary(items []Item) string {
	if len(items) == 0 {
		return "✓ Nenhum débito técnico ou atalho registrado no código.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Débitos Técnicos e Atalhos (%d itens encontrados):\n", len(items)))
	sb.WriteString("---------------------------------------------------------\n")
	for _, it := range items {
		tagStr := it.Tag
		if it.Context != "" {
			tagStr = fmt.Sprintf("%s(%s)", it.Tag, it.Context)
		}
		sb.WriteString(fmt.Sprintf("  • %s:%d [%s] %s\n", it.File, it.Line, tagStr, it.Message))
	}
	return sb.String()
}
