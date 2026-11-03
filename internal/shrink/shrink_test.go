package shrink

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShrinkContent(t *testing.T) {
	input := `# 1. OBJETIVO DO RECURSO
Aqui está o resumo da especificação técnica:

Com certeza iremos criar o endpoint a fim de atender aos requisitos.
Note que o código deve ser eficiente.

` + "```go\nfunc Authenticate() bool {\n    return true\n}\n```" + `

Hope this helps! Qualquer dúvida estou à disposição.
`

	shrunk := ShrinkContent(input)

	// Verify headers and code block are intact verbatim
	if !strings.Contains(shrunk, "# 1. OBJETIVO DO RECURSO") {
		t.Errorf("heading dropped in shrunk output: %s", shrunk)
	}
	if !strings.Contains(shrunk, "func Authenticate() bool {") {
		t.Errorf("code block altered in shrunk output: %s", shrunk)
	}

	// Verify fluff sentences are stripped
	if strings.Contains(shrunk, "Aqui está o resumo") {
		t.Errorf("fluff intro not stripped")
	}
	if strings.Contains(shrunk, "Hope this helps") || strings.Contains(shrunk, "Qualquer dúvida estou à disposição") {
		t.Errorf("fluff outro not stripped")
	}

	// Verify ceremony removal and phrase replacement
	if strings.Contains(shrunk, "a fim de") {
		t.Errorf("expected 'a fim de' replaced by 'para'")
	}
	if !strings.Contains(shrunk, "para atender") {
		t.Errorf("expected 'para atender' in output: %s", shrunk)
	}
}

func TestShrinkFile(t *testing.T) {
	tmp := t.TempDir()
	specFile := filepath.Join(tmp, "spec.md")
	content := "# 1. Spec\n\nHere is the spec.\n\n`code_sample`\n\nHope this helps.\n"

	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ShrinkFile(tmp, "spec.md")
	if err != nil {
		t.Fatalf("shrink file error: %v", err)
	}

	if _, err := os.Stat(res.BackupPath); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	after, _ := os.ReadFile(specFile)
	if strings.Contains(string(after), "Here is the spec") || strings.Contains(string(after), "Hope this helps") {
		t.Errorf("file content still contains fluff: %s", string(after))
	}
}
