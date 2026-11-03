package gates

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ARILSONLIMA/forge/internal/config"
	"github.com/ARILSONLIMA/forge/internal/runner"
	"github.com/ARILSONLIMA/forge/internal/specs"
	"github.com/ARILSONLIMA/forge/internal/state"
)

type mockBeadClient struct {
	readyFunc func(ctx context.Context) ([]runner.BeadTask, error)
	showFunc  func(ctx context.Context, id string) (*runner.BeadTask, error)
	closeFunc func(ctx context.Context, id string, reason string) error
}

func (m *mockBeadClient) Ready(ctx context.Context) ([]runner.BeadTask, error) {
	if m.readyFunc != nil {
		return m.readyFunc(ctx)
	}
	return []runner.BeadTask{}, nil
}

func (m *mockBeadClient) Show(ctx context.Context, id string) (*runner.BeadTask, error) {
	if m.showFunc != nil {
		return m.showFunc(ctx, id)
	}
	return &runner.BeadTask{ID: id, Status: "open"}, nil
}

func (m *mockBeadClient) Close(ctx context.Context, id string, reason string) error {
	if m.closeFunc != nil {
		return m.closeFunc(ctx, id, reason)
	}
	return nil
}

func TestGate1Validation(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, ".forge", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	st := state.NewInitialState()
	st.CurrentStage = state.StageSpec
	cfg := config.DetectProject(tmpDir)

	mgr := NewManager(tmpDir, &cfg, st, nil, nil)

	// Case 1: No spec file
	st.ActiveSpec = ".forge/specs/missing.md"
	_, err := mgr.ValidateGate1(context.Background())
	if err == nil {
		t.Fatal("expected error on missing spec, got nil")
	}

	// Case 2: Spec with fewer than 15 lines
	shortSpec := filepath.Join(specsDir, "short.md")
	_ = os.WriteFile(shortSpec, []byte("## Objetivo\nfoo\n"), 0o644)
	st.ActiveSpec = ".forge/specs/short.md"
	_, err = mgr.ValidateGate1(context.Background())
	if err == nil {
		t.Fatal("expected error on short spec (<15 lines), got nil")
	}

	// Case 3: Spec with >= 15 lines but missing sections
	var missingSecContent strings.Builder
	for i := 0; i < 20; i++ {
		missingSecContent.WriteString(fmt.Sprintf("line %d\n", i))
	}
	missingSecSpec := filepath.Join(specsDir, "missing-sec.md")
	_ = os.WriteFile(missingSecSpec, []byte(missingSecContent.String()), 0o644)
	st.ActiveSpec = ".forge/specs/missing-sec.md"
	_, err = mgr.ValidateGate1(context.Background())
	if err == nil {
		t.Fatal("expected error on missing sections, got nil")
	}

	// Case 4: Valid spec but missing glossary
	var validContent strings.Builder
	validContent.WriteString("# Spec: Token Rotation\n\n")
	validContent.WriteString("## Objetivo\nImplementar rotatividade de tokens jwt segura.\n\n")
	validContent.WriteString("## Contratos e Assinaturas\n- func RotateToken(t string) (string, error)\n\n")
	validContent.WriteString("## Casos de Borda\n- Token expirado deve retornar erro 401.\n\n")
	validContent.WriteString("## Fora de Escopo (YAGNI)\n- Suporte a OAuth2 externo.\n\n")
	for i := 0; i < 10; i++ {
		validContent.WriteString(fmt.Sprintf("Detalhe %d\n", i))
	}
	validSpec := filepath.Join(specsDir, "valid.md")
	_ = os.WriteFile(validSpec, []byte(validContent.String()), 0o644)
	st.ActiveSpec = ".forge/specs/valid.md"

	_, err = mgr.ValidateGate1(context.Background())
	if err == nil {
		t.Fatal("expected error on valid spec with missing glossary, got nil")
	}

	// Case 5: Valid spec with unpopulated skeleton glossary
	glossaryPath := filepath.Join(tmpDir, ".forge", "GLOSSARY.md")
	_, err = specs.WriteGlossarySkeleton(glossaryPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.ValidateGate1(context.Background())
	if err == nil {
		t.Fatal("expected error on skeleton glossary with no defined terms, got nil")
	}

	// Case 6: Valid spec with valid populated DDD glossary
	termContent := "\n**Token**: Credencial efêmera criptografada usada para autenticação.\n_Avoid_: Chave, Cookie\n"
	f, err := os.OpenFile(glossaryPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(termContent)
	_ = f.Close()

	res, err := mgr.ValidateGate1(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on valid spec and glossary: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected Gate 1 to pass, got %+v", res)
	}
	if !st.GatesPassed[string(state.StageSpec)] {
		t.Fatal("expected StageSpec gate marked as passed in state")
	}
}

func TestGate2Validation(t *testing.T) {
	st := state.NewInitialState()
	st.CurrentStage = state.StagePlan
	cfg := config.DetectProject(t.TempDir())

	// Case 1: Beads returns 0 tasks
	mockBeads := &mockBeadClient{
		readyFunc: func(ctx context.Context) ([]runner.BeadTask, error) {
			return []runner.BeadTask{}, nil
		},
	}
	mgr := NewManager(t.TempDir(), &cfg, st, nil, mockBeads)
	_, err := mgr.ValidateGate2(context.Background())
	if err == nil {
		t.Fatal("expected error when bd ready returns 0 tasks")
	}

	// Case 2: Beads returns ready tasks
	mockBeads.readyFunc = func(ctx context.Context) ([]runner.BeadTask, error) {
		return []runner.BeadTask{
			{ID: "bd-789", Title: "Build auth token table", Status: "ready"},
		}, nil
	}

	res, err := mgr.ValidateGate2(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on Gate 2: %v", err)
	}
	if !res.Passed || st.ActiveBeadID != "bd-789" {
		t.Fatalf("expected active_bead_id to be bd-789, got %s", st.ActiveBeadID)
	}
	if !st.GatesPassed[string(state.StagePlan)] {
		t.Fatal("expected StagePlan gate marked as passed")
	}
}

func TestGate3Validation(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.NewInitialState()
	st.CurrentStage = state.StageBuild
	cfg := config.Config{
		Commands: config.Commands{
			Build: "echo building...",
		},
	}

	mgr := NewManager(tmpDir, &cfg, st, nil, nil)

	// Case 1: Clean tree (no modified files)
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		return "", nil // empty git status
	}
	_, err := mgr.ValidateGate3(context.Background())
	if err == nil {
		t.Fatal("expected error on clean working tree for Gate 3, got nil")
	}

	// Case 2: Dirty tree with modified files
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		return " M internal/auth/service.go\n?? internal/auth/service_test.go", nil
	}
	res, err := mgr.ValidateGate3(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed || len(st.ModifiedSymbols) != 2 {
		t.Fatalf("expected 2 modified files in state, got %v", st.ModifiedSymbols)
	}
}

func TestGate4ValidationAndRollback(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.NewInitialState()
	st.CurrentStage = state.StageTest
	cfg := config.Config{
		Commands: config.Commands{
			Test: "false", // Simulate failing test
		},
	}

	mgr := NewManager(tmpDir, &cfg, st, nil, nil)
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		return " M internal/auth_test.go", nil
	}

	// Attempt 1: failure
	_, err := mgr.ValidateGate4(context.Background())
	if err == nil {
		t.Fatal("expected error on test failure")
	}
	if st.TestFailCount != 1 {
		t.Fatalf("expected TestFailCount == 1, got %d", st.TestFailCount)
	}

	// Attempt 2: second failure -> trigger rollback to BUILD
	_, err = mgr.ValidateGate4(context.Background())
	if err == nil {
		t.Fatal("expected error on second failure")
	}
	if st.TestFailCount != 2 {
		t.Fatalf("expected TestFailCount == 2, got %d", st.TestFailCount)
	}
	if st.CurrentStage != state.StageBuild {
		t.Fatalf("expected rollback to StageBuild, got %s", st.CurrentStage)
	}

	// Now simulate passing test
	cfg.Commands.Test = "echo tests pass"
	st.CurrentStage = state.StageTest
	res, err := mgr.ValidateGate4(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatal("expected gate 4 to pass")
	}
	if st.TestFailCount != 0 || st.LastTestFailureSummary != "" {
		t.Fatalf("expected fail count and summary reset, got count %d, summary %q", st.TestFailCount, st.LastTestFailureSummary)
	}
}

func TestGate5Validation(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.NewInitialState()
	st.CurrentStage = state.StageReview
	cfg := config.Config{
		Commands: config.Commands{
			Lint: "echo lint ok",
		},
		GateThresholds: config.GateThresholds{
			MaxDiffLinesPerTicket: 100,
		},
		Paths: config.Paths{
			RulesDir: ".forge/rules",
		},
	}

	mgr := NewManager(tmpDir, &cfg, st, nil, nil)

	// Mock git diff with 150 lines (> 100 max)
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "diff" {
			return " 2 files changed, 120 insertions(+), 35 deletions(-)", nil
		}
		return "", nil
	}

	_, err := mgr.ValidateGate5(context.Background())
	if err == nil {
		t.Fatal("expected diff threshold violation error, got nil")
	}

	// Mock diff under threshold
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "diff" {
			return " 2 files changed, 40 insertions(+), 10 deletions(-)", nil
		}
		return "", nil
	}

	res, err := mgr.ValidateGate5(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected Gate 5 to pass, got %+v", res)
	}
}

func TestGate6Validation(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.NewInitialState()
	st.CurrentStage = state.StageSimplify
	st.ReviewDiffLines = 50 // Baseline from Review stage

	cfg := config.Config{
		Commands: config.Commands{
			Test: "echo all good",
		},
	}

	mgr := NewManager(tmpDir, &cfg, st, nil, nil)

	// Case 1: Diff increased to 80 during simplify (regression in YAGNI pruning)
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		return " 2 files changed, 60 insertions(+), 20 deletions(-)", nil
	}
	_, err := mgr.ValidateGate6(context.Background())
	if err == nil {
		t.Fatal("expected error when diff grew during simplify")
	}

	// Case 2: Diff stayed equal or decreased (e.g. 40)
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		return " 2 files changed, 30 insertions(+), 10 deletions(-)", nil
	}
	res, err := mgr.ValidateGate6(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected Gate 6 to pass")
	}
}

func TestGate7Validation(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.NewInitialState()
	st.CurrentStage = state.StageShip
	st.ActiveBeadID = "bd-999"

	cfg := config.DetectProject(tmpDir)
	mockBeads := &mockBeadClient{
		showFunc: func(ctx context.Context, id string) (*runner.BeadTask, error) {
			return &runner.BeadTask{ID: id, Status: "open"}, nil // Not closed yet
		},
	}

	mgr := NewManager(tmpDir, &cfg, st, nil, mockBeads)
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		return "", nil // clean
	}

	// Case 1: Bead still open -> fail
	_, err := mgr.ValidateGate7(context.Background())
	if err == nil {
		t.Fatal("expected error when bead is not closed")
	}

	// Case 2: Bead closed, but git is dirty -> fail
	mockBeads.showFunc = func(ctx context.Context, id string) (*runner.BeadTask, error) {
		return &runner.BeadTask{ID: id, Status: "closed"}, nil
	}
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		return " M uncommitted_file.go", nil
	}
	_, err = mgr.ValidateGate7(context.Background())
	if err == nil {
		t.Fatal("expected error on uncommitted changes in Gate 7")
	}

	// Case 3: Bead closed & git clean, no more tasks -> pass to IDLE
	mgr.GitExec = func(ctx context.Context, dir string, args ...string) (string, error) {
		return "", nil
	}
	mockBeads.readyFunc = func(ctx context.Context) ([]runner.BeadTask, error) {
		return []runner.BeadTask{}, nil
	}
	res, err := mgr.ValidateGate7(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.To != state.StageIdle {
		t.Fatalf("expected destination IDLE, got %s", res.To)
	}

	// Case 4: More ready tasks in DAG -> pass to BUILD
	mockBeads.readyFunc = func(ctx context.Context) ([]runner.BeadTask, error) {
		return []runner.BeadTask{
			{ID: "bd-1000", Title: "Next tracer bullet", Status: "ready"},
		}, nil
	}
	res, err = mgr.ValidateGate7(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.To != state.StageBuild {
		t.Fatalf("expected destination BUILD, got %s", res.To)
	}
}

func TestAuditLog(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.NewInitialState()
	cfg := config.DetectProject(tmpDir)

	mgr := NewManager(tmpDir, &cfg, st, nil, nil)
	logFile := filepath.Join(tmpDir, ".forge", "logs", "gate-audit.log")
	mgr.AuditLogPath = logFile

	res := &GateResult{
		GateNumber: 1,
		From:       state.StageSpec,
		To:         state.StagePlan,
		Passed:     true,
		Details:    "Spec valid",
	}

	if err := mgr.AppendAuditLog(res); err != nil {
		t.Fatalf("failed to append audit log: %v", err)
	}

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	if !strings.Contains(string(content), "GATE 1 (SPEC -> PLAN): PASS - Spec valid") {
		t.Fatalf("unexpected audit log content: %s", string(content))
	}
}
