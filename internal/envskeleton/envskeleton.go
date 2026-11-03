package envskeleton

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	envLineRegex    = regexp.MustCompile(`^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)
	placeholderRe   = regexp.MustCompile(`(?i)^(?:your[_-]|change[_-]?me|changeme|example|placeholder|xxx+|todo|replace[_-]?me|<.*>|\$\{.*\})$`)
)

// KeyStatus represents the masked state of an environment variable.
type KeyStatus struct {
	Key    string `json:"key"`
	Line   int    `json:"line"`
	Status string `json:"status"` // "SET", "EMPTY", "PLACEHOLDER"
}

// Report holds the comparative analysis between .env and .env.example.
type Report struct {
	EnvFile         string      `json:"env_file"`
	ExampleFile     string      `json:"example_file,omitempty"`
	Keys            []KeyStatus `json:"keys"`
	MissingKeys     []string    `json:"missing_keys"`
	PlaceholderKeys []string    `json:"placeholder_keys"`
	EmptyKeys       []string    `json:"empty_keys"`
	Healthy         bool        `json:"healthy"`
}

// Inspect safely parses env files without storing or leaking secret values.
func Inspect(rootDir string) (*Report, error) {
	if rootDir == "" {
		rootDir = "."
	}

	envFile := findFirstExisting(rootDir, []string{".env", ".env.local", ".env.development"})
	exampleFile := findFirstExisting(rootDir, []string{".env.example", ".env.sample", ".env.template"})

	if envFile == "" && exampleFile == "" {
		return &Report{Healthy: true}, nil
	}

	rep := &Report{
		EnvFile:     envFile,
		ExampleFile: exampleFile,
		Healthy:     true,
	}

	envKeysMap := make(map[string]KeyStatus)
	if envFile != "" {
		keys, err := parseKeysOnly(filepath.Join(rootDir, envFile))
		if err != nil {
			return nil, err
		}
		rep.Keys = keys
		for _, k := range keys {
			envKeysMap[k.Key] = k
			if k.Status == "PLACEHOLDER" {
				rep.PlaceholderKeys = append(rep.PlaceholderKeys, k.Key)
				rep.Healthy = false
			} else if k.Status == "EMPTY" {
				rep.EmptyKeys = append(rep.EmptyKeys, k.Key)
			}
		}
	}

	if exampleFile != "" {
		exampleKeys, err := parseKeysOnly(filepath.Join(rootDir, exampleFile))
		if err == nil {
			for _, ek := range exampleKeys {
				if _, exists := envKeysMap[ek.Key]; !exists {
					rep.MissingKeys = append(rep.MissingKeys, ek.Key)
					rep.Healthy = false
				}
			}
		}
	}

	sort.Strings(rep.MissingKeys)
	sort.Strings(rep.PlaceholderKeys)
	sort.Strings(rep.EmptyKeys)

	return rep, nil
}

func parseKeysOnly(fullPath string) ([]KeyStatus, error) {
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var keys []KeyStatus
	scanner := bufio.NewScanner(f)
	lineNum := 0
	seen := make(map[string]bool)

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		m := envLineRegex.FindStringSubmatch(line)
		if len(m) > 2 {
			k := m[1]
			val := strings.TrimSpace(m[2])

			// Strip inline comments if unquoted
			if !strings.HasPrefix(val, `"`) && !strings.HasPrefix(val, `'`) {
				if idx := strings.Index(val, " #"); idx != -1 {
					val = strings.TrimSpace(val[:idx])
				}
			} else if len(val) >= 2 {
				val = strings.Trim(val, `"'`)
			}

			status := "SET"
			if val == "" {
				status = "EMPTY"
			} else if placeholderRe.MatchString(val) {
				status = "PLACEHOLDER"
			}

			// Clear val memory immediately
			val = ""

			if !seen[k] {
				seen[k] = true
				keys = append(keys, KeyStatus{
					Key:    k,
					Line:   lineNum,
					Status: status,
				})
			}
		}
	}

	return keys, scanner.Err()
}

func findFirstExisting(rootDir string, names []string) string {
	for _, n := range names {
		p := filepath.Join(rootDir, n)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return n
		}
	}
	return ""
}

// FormatSummary formats report for terminal and doctor output.
func (r *Report) FormatSummary() string {
	if r.EnvFile == "" && r.ExampleFile == "" {
		return "ℹ️ Nenhum arquivo .env ou .env.example detectado.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🛡️ Secret Shield & Env Skeleton (%s):\n", r.EnvFile))
	sb.WriteString("---------------------------------------------------------\n")
	if len(r.MissingKeys) > 0 {
		sb.WriteString(fmt.Sprintf("  [!] Variáveis ausentes (presentes em %s): %s\n", r.ExampleFile, strings.Join(r.MissingKeys, ", ")))
	}
	if len(r.PlaceholderKeys) > 0 {
		sb.WriteString(fmt.Sprintf("  [!] Variáveis com placeholder (não configuradas): %s\n", strings.Join(r.PlaceholderKeys, ", ")))
	}
	if len(r.EmptyKeys) > 0 {
		sb.WriteString(fmt.Sprintf("  [-] Variáveis vazias: %s\n", strings.Join(r.EmptyKeys, ", ")))
	}
	if r.Healthy {
		sb.WriteString(fmt.Sprintf("  [✓] %d variáveis verificadas e íntegras (zero vazamento de valores).\n", len(r.Keys)))
	}
	return sb.String()
}
