package specs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Autenticação JWT e Sessão", "autenticacao-jwt-e-sessao"},
		{"01 - Core Domain / Models", "01-core-domain-models"},
		{"Hello, World! 123", "hello-world-123"},
		{"   Multiple    Spaces   ", "multiple-spaces"},
		{"", "spec"},
	}

	for _, c := range cases {
		got := Slugify(c.input)
		if got != c.expected {
			t.Errorf("Slugify(%q) = %q, expected %q", c.input, got, c.expected)
		}
	}
}

func TestParseRoadmap(t *testing.T) {
	tmpDir := t.TempDir()
	roadmapFile := filepath.Join(tmpDir, "roadmap.md")

	content := `# Project Roadmap

<!-- comments -->

- [ ] 01 - Auth Service: Implement login and JWT issuance
- [x] 02 - Database Schema: Run migrations
- [ ] 3 - API Endpoints: CRUD operations
`
	if err := os.WriteFile(roadmapFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := ParseRoadmap(roadmapFile)
	if err != nil {
		t.Fatalf("ParseRoadmap failed: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	if items[0].Number != 1 || items[0].PaddedNum != "01" || items[0].Title != "Auth Service" || items[0].Goal != "Implement login and JWT issuance" || items[0].Done {
		t.Errorf("unexpected item 0: %+v", items[0])
	}

	if items[1].Number != 2 || !items[1].Done {
		t.Errorf("unexpected item 1: %+v", items[1])
	}

	if items[2].Number != 3 || items[2].PaddedNum != "03" || items[2].Title != "API Endpoints" {
		t.Errorf("unexpected item 2: %+v", items[2])
	}
}

func TestListAndResolveSpec(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, ".forge", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	roadmapPath := filepath.Join(tmpDir, ".forge", "roadmap.md")
	roadmapContent := `# Roadmap
- [ ] 01 - Auth JWT: Login and JWT
- [ ] 02 - Billing Module: Stripe checkout
`
	if err := os.WriteFile(roadmapPath, []byte(roadmapContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create spec 01 on disk
	spec1Path := filepath.Join(specsDir, "01-auth-jwt.md")
	if err := CreateSpecSkeleton(spec1Path, "Auth JWT", "Provide JWT login"); err != nil {
		t.Fatal(err)
	}

	// 1. List specs
	specs, err := ListSpecs(tmpDir, ".forge/specs", ".forge/specs/01-auth-jwt.md")
	if err != nil {
		t.Fatalf("ListSpecs error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if !specs[0].IsActive || specs[0].Number != 1 || specs[0].PaddedNum != "01" {
		t.Errorf("unexpected spec list item: %+v", specs[0])
	}

	// 2. Resolve "1" (should find 01-auth-jwt.md on disk)
	res1, err := ResolveSpec(tmpDir, ".forge/specs", ".forge/roadmap.md", "1")
	if err != nil {
		t.Fatalf("ResolveSpec(1) failed: %v", err)
	}
	if !res1.Exists || res1.Filename != "01-auth-jwt.md" || res1.Number != 1 {
		t.Errorf("unexpected resolved spec for '1': %+v", res1)
	}

	// 3. Resolve "01" (should also find 01-auth-jwt.md)
	res01, err := ResolveSpec(tmpDir, ".forge/specs", ".forge/roadmap.md", "01")
	if err != nil {
		t.Fatalf("ResolveSpec(01) failed: %v", err)
	}
	if !res01.Exists || res01.Filename != "01-auth-jwt.md" {
		t.Errorf("unexpected resolved spec for '01': %+v", res01)
	}

	// 4. Resolve "2" (not on disk, but in roadmap!)
	res2, err := ResolveSpec(tmpDir, ".forge/specs", ".forge/roadmap.md", "2")
	if err != nil {
		t.Fatalf("ResolveSpec(2) failed: %v", err)
	}
	if res2.Exists {
		t.Errorf("expected spec 2 not to exist yet on disk")
	}
	if res2.Number != 2 || res2.PaddedNum != "02" || !strings.Contains(res2.Filename, "02-billing-module") {
		t.Errorf("unexpected resolved roadmap spec for '2': %+v", res2)
	}

	// 5. Resolve "3" (neither on disk nor in roadmap - should provide clean skeleton target)
	res3, err := ResolveSpec(tmpDir, ".forge/specs", ".forge/roadmap.md", "3")
	if err != nil {
		t.Fatalf("ResolveSpec(3) failed: %v", err)
	}
	if res3.Number != 3 || res3.PaddedNum != "03" || res3.Exists {
		t.Errorf("unexpected resolved spec for '3': %+v", res3)
	}

	// 6. Test CreateSpecSkeleton
	spec2Path := filepath.Join(specsDir, res2.Filename)
	if err := CreateSpecSkeleton(spec2Path, res2.Title, res2.Goal); err != nil {
		t.Fatalf("CreateSpecSkeleton failed: %v", err)
	}

	data, err := os.ReadFile(spec2Path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 15 || len(lines) > 120 {
		t.Errorf("skeleton lines outside Gate 1 bounds: %d", len(lines))
	}
	for _, sec := range []string{"## Objetivo", "## Contratos e Assinaturas", "## Casos de Borda", "## Fora de Escopo"} {
		if !strings.Contains(string(data), sec) {
			t.Errorf("missing mandatory section %q in skeleton", sec)
		}
	}
}

func TestGlossaryAndDraft(t *testing.T) {
	tmpDir := t.TempDir()
	glossaryPath := filepath.Join(tmpDir, ".forge", "GLOSSARY.md")

	// 1. Missing glossary
	if err := ValidateGlossary(tmpDir, ".forge/GLOSSARY.md"); err == nil {
		t.Fatal("expected error on missing glossary")
	}

	// 2. Write skeleton
	created, err := WriteGlossarySkeleton(glossaryPath)
	if err != nil || !created {
		t.Fatalf("expected skeleton created, got created=%v, err=%v", created, err)
	}

	// Skeleton has only template comment, so terms count is 0 -> should fail validation
	if err := ValidateGlossary(tmpDir, ".forge/GLOSSARY.md"); err == nil {
		t.Fatal("expected error when glossary has no defined terms")
	}

	// 3. Add valid domain term
	termContent := "\n**Conta**: Entidade que representa o registro financeiro de um usuário.\n_Avoid_: Perfil\n"
	f, err := os.OpenFile(glossaryPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(termContent)
	_ = f.Close()

	if err := ValidateGlossary(tmpDir, ".forge/GLOSSARY.md"); err != nil {
		t.Fatalf("expected valid glossary to pass, got: %v", err)
	}

	// 4. Test IsSpecDraft
	specDraftPath := filepath.Join(tmpDir, "draft.md")
	if err := CreateSpecSkeleton(specDraftPath, "Draft Spec", ""); err != nil {
		t.Fatal(err)
	}
	isDraft, reason := IsSpecDraft(specDraftPath)
	if !isDraft {
		t.Fatalf("expected newly created skeleton to be detected as draft, got false: %s", reason)
	}

	// Complete spec
	completeSpecPath := filepath.Join(tmpDir, "complete.md")
	completeContent := `# Especificação: Auth

## Objetivo
Implementar fluxo OAuth2 completo com Google e GitHub.

## Contratos e Assinaturas
- func HandleOAuthCallback(w http.ResponseWriter, r *http.Request)
- type TokenPayload struct { AccessToken string }

## Casos de Borda
- Token expirado deve invalidar a sessão imediatamente.
- Falha na API do provedor deve emitir alerta estruturado.

## Fora de Escopo (YAGNI)
- Integração com provedores legados SAML.
- Migração de bancos externos.

Linhas adicionais para garantir que o tamanho seja suficiente.
Mais detalhes de engenharia e contexto do projeto.
`
	if err := os.WriteFile(completeSpecPath, []byte(completeContent), 0o644); err != nil {
		t.Fatal(err)
	}
	isDraft, reason = IsSpecDraft(completeSpecPath)
	if isDraft {
		t.Fatalf("expected completed spec not to be draft, got draft: %s", reason)
	}
}
