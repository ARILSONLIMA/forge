package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ARILSONLIMA/forge/internal/config"
	"github.com/ARILSONLIMA/forge/internal/setup"
	"github.com/ARILSONLIMA/forge/internal/state"
)

func TestCLIInitAndStatus(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Run init
	if err := runInit(tmpDir, nil); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Verify generated assets
	cfgFile := filepath.Join(tmpDir, config.DefaultPath)
	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("expected config file %s to exist", cfgFile)
	}

	stateFile := filepath.Join(tmpDir, state.DefaultPath)
	st, err := state.Load(stateFile)
	if err != nil {
		t.Fatalf("failed to load initial state: %v", err)
	}
	if st.CurrentStage != state.StageIdle {
		t.Fatalf("expected initial stage IDLE, got %s", st.CurrentStage)
	}

	activeStageFile := filepath.Join(tmpDir, ".agent", "rules", "active-stage.md")
	if _, err := os.Stat(activeStageFile); err != nil {
		t.Fatalf("expected active-stage.md to exist: %v", err)
	}

	// Verify all 7 workflows generated
	for _, w := range []string{"spec.md", "plan.md", "build.md", "test.md", "review.md", "simplify.md", "ship.md"} {
		wFile := filepath.Join(tmpDir, ".agent", "workflows", w)
		if _, err := os.Stat(wFile); err != nil {
			t.Fatalf("expected workflow %s to exist", w)
		}
	}

	// 2. Run status
	if err := runStatus(tmpDir); err != nil {
		t.Fatalf("runStatus failed: %v", err)
	}

	// 3. Run doctor
	if err := runDoctor(context.Background(), tmpDir, nil); err != nil {
		t.Fatalf("runDoctor failed: %v", err)
	}

	// 4. Run setup in non-interactive mode
	setup.CustomRCDirectory = tmpDir
	if err := runSetup(context.Background(), tmpDir, []string{"--bin-dir=" + filepath.Join(tmpDir, "bin"), "--yes"}); err != nil {
		t.Fatalf("runSetup failed: %v", err)
	}
	setup.CustomRCDirectory = ""

	// 5. Verify version
	if Version != "0.0.3" {
		t.Fatalf("expected version 0.0.3, got %s", Version)
	}

	// 6. Test update --help
	if err := runUpdate(context.Background(), tmpDir, []string{"--help"}); err != nil {
		t.Fatalf("runUpdate --help failed: %v", err)
	}
}

func TestCLIStageTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	if err := runInit(tmpDir, nil); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// 1. Transition IDLE -> SPEC (valid without gate)
	if err := runStage(ctx, tmpDir, []string{"spec"}); err != nil {
		t.Fatalf("failed to transition to spec: %v", err)
	}

	st, _ := state.Load(filepath.Join(tmpDir, state.DefaultPath))
	if st.CurrentStage != state.StageSpec {
		t.Fatalf("expected stage SPEC, got %s", st.CurrentStage)
	}

	// Active stage should now contain SPEC instructions
	ruleContent, _ := os.ReadFile(filepath.Join(tmpDir, ".agent", "rules", "active-stage.md"))
	if !strings.Contains(string(ruleContent), "SPEC") {
		t.Fatalf("expected active-stage.md to mention SPEC, got %s", string(ruleContent))
	}

	// 2. Transition SPEC -> PLAN without valid spec should be BLOCKED by Gate 1
	err := runStage(ctx, tmpDir, []string{"plan"})
	if err == nil {
		t.Fatal("expected Gate 1 to block transition to PLAN without valid spec, but got nil")
	}

	// 3. Create a valid spec file and verify Gate 1 allows advancing
	specsDir := filepath.Join(tmpDir, ".forge", "specs")
	var specContent strings.Builder
	specContent.WriteString("# Spec: Auth Service\n\n")
	specContent.WriteString("## Objetivo\nCriar serviço de autenticação jwt.\n\n")
	specContent.WriteString("## Contratos e Assinaturas\n- Authenticate(user, pass string) (string, error)\n\n")
	specContent.WriteString("## Casos de Borda\n- Senha inválida gera erro 403.\n\n")
	specContent.WriteString("## Fora de Escopo\n- Login social.\n\n")
	for i := 0; i < 10; i++ {
		specContent.WriteString("Detalhe técnico\n")
	}
	specFile := filepath.Join(specsDir, "auth.md")
	if err := os.WriteFile(specFile, []byte(specContent.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	st.ActiveSpec = ".forge/specs/auth.md"
	_ = state.Save(filepath.Join(tmpDir, state.DefaultPath), st)

	// Now stage plan should succeed
	if err := runStage(ctx, tmpDir, []string{"plan"}); err != nil {
		t.Fatalf("expected transition to PLAN to succeed, got %v", err)
	}

	st, _ = state.Load(filepath.Join(tmpDir, state.DefaultPath))
	if st.CurrentStage != state.StagePlan {
		t.Fatalf("expected stage PLAN, got %s", st.CurrentStage)
	}
}
