package shrink

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	fluffSentences = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(?:I will now|Let's|Now we can|Here is|Below is|Next step is|O próximo passo é|Aqui está|Abaixo segue|Neste documento iremos).{0,60}:?\s*$`),
		regexp.MustCompile(`(?i)^(?:Hope this helps|Let me know if you need anything else|Qualquer dúvida estou à disposição|Espero ter ajudado|Fico no aguardo).{0,60}\.?\s*$`),
	}

	ceremonyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:sure(?:ly)?|certainly|of course|happy to help|please note that|it should be noted that|as mentioned (?:earlier|previously)|as you can see|basically|actually|essentially)[!,.:;]?\s*`),
		regexp.MustCompile(`(?i)\b(?:com certeza|claro que sim|estou feliz em ajudar|note que|vale ressaltar que|como mencionado anteriormente|basicamente|na verdade)[!,.:;]?\s*`),
	}

	phraseReplacements = []struct {
		pattern *regexp.Regexp
		repl    string
	}{
		{regexp.MustCompile(`(?i)\bin order to\b`), "to"},
		{regexp.MustCompile(`(?i)\ba fim de\b`), "para"},
		{regexp.MustCompile(`(?i)\bcom o intuito de\b`), "para"},
		{regexp.MustCompile(`(?i)\bmake sure to\b`), "ensure"},
		{regexp.MustCompile(`(?i)\bcertifique-se de que\b`), "garanta que"},
	}

	codeBlockRegex = regexp.MustCompile("(?s)(```[^\n]*\n.*?```|`[^`\r\n]+`)")
)

// ShrinkResult details what was compressed.
type ShrinkResult struct {
	OriginalLines int
	ShrunkLines   int
	OriginalBytes int
	ShrunkBytes   int
	BackupPath    string
}

// ShrinkContent compresses markdown string while preserving code blocks verbatim.
func ShrinkContent(content string) string {
	// 1. Extract code blocks into placeholders
	type placeholder struct {
		key  string
		code string
	}
	var placeholders []placeholder
	idx := 0

	textWithoutCode := codeBlockRegex.ReplaceAllStringFunc(content, func(m string) string {
		k := fmt.Sprintf("__FORGE_VERBATIM_%d__", idx)
		placeholders = append(placeholders, placeholder{key: k, code: m})
		idx++
		return k
	})

	// 2. Process line by line
	lines := strings.Split(textWithoutCode, "\n")
	var processed []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Preserve headings and tables verbatim
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "|") {
			processed = append(processed, line)
			continue
		}

		// Filter full fluff sentences
		isFluff := false
		for _, re := range fluffSentences {
			if re.MatchString(trimmed) {
				isFluff = true
				break
			}
		}
		if isFluff {
			continue
		}

		// Apply phrase replacements and ceremony stripping
		compressed := line
		for _, pr := range phraseReplacements {
			compressed = pr.pattern.ReplaceAllString(compressed, pr.repl)
		}
		for _, cp := range ceremonyPatterns {
			compressed = cp.ReplaceAllString(compressed, "")
		}

		processed = append(processed, compressed)
	}

	joined := strings.Join(processed, "\n")

	// 3. Restore code blocks verbatim
	for _, p := range placeholders {
		joined = strings.ReplaceAll(joined, p.key, p.code)
	}

	// 4. Collapse repetitive empty lines
	reMultipleNewlines := regexp.MustCompile(`\n{3,}`)
	joined = reMultipleNewlines.ReplaceAllString(joined, "\n\n")

	return strings.TrimSpace(joined) + "\n"
}

// ShrinkFile compresses a markdown file on disk with safe backup.
func ShrinkFile(rootDir, targetPath string) (*ShrinkResult, error) {
	fullPath := targetPath
	if !filepath.IsAbs(fullPath) && rootDir != "" {
		fullPath = filepath.Join(rootDir, fullPath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	originalStr := string(data)
	shrunkStr := ShrinkContent(originalStr)

	// Backup
	backupDir := filepath.Join(rootDir, ".forge", "backups")
	_ = os.MkdirAll(backupDir, 0o755)
	base := filepath.Base(fullPath)
	ts := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s.%s.bak", base, ts))
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
	}

	// Write shrunk file
	if err := os.WriteFile(fullPath, []byte(shrunkStr), 0o644); err != nil {
		return nil, fmt.Errorf("write shrunk file: %w", err)
	}

	origLines := len(strings.Split(originalStr, "\n"))
	shrunkLines := len(strings.Split(shrunkStr, "\n"))

	return &ShrinkResult{
		OriginalLines: origLines,
		ShrunkLines:   shrunkLines,
		OriginalBytes: len(originalStr),
		ShrunkBytes:   len(shrunkStr),
		BackupPath:    backupPath,
	}, nil
}
