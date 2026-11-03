package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ARILSONLIMA/forge/internal/state"
)

func TestAllStagesUnderTokenBudget(t *testing.T) {
	for _, st := range append([]state.Stage{state.StageIdle}, Stages...) {
		s := &state.State{CurrentStage: st, ActiveBeadID: "bd-a1b2", ActiveSpec: ".forge/specs/auth-token-rotation.md"}
		out, err := RenderStage(s, "")
		if err != nil {
			t.Fatalf("%s: %v", st, err)
		}
		if n := EstimateTokens(out); n >= MaxStageTokens {
			t.Errorf("%s: ~%d tokens", st, n)
		}
		if strings.Contains(out, "{{") {
			t.Errorf("%s: unreplaced placeholder", st)
		}
	}
}

func TestSwapStageOnlyContainsCurrentStage(t *testing.T) {
	root := t.TempDir()
	s := &state.State{CurrentStage: state.StageBuild, ActiveBeadID: "bd-a1b2"}
	if _, err := SwapStage(root, s); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ActiveStagePath))
	got := string(data)
	if !strings.Contains(got, "BUILD (Tarefa: bd-a1b2)") || strings.Contains(got, "ESTÁGIO ATIVO: TEST") {
		t.Fatalf("unexpected content:\n%s", got)
	}
}

func TestSwapStageInjectsFailure(t *testing.T) {
	s := &state.State{CurrentStage: state.StageBuild, ActiveBeadID: "bd-1", LastTestFailureSummary: "FAIL TestX"}
	out, _ := RenderStage(s, s.LastTestFailureSummary)
	if !strings.HasPrefix(out, "## ÚLTIMA FALHA") {
		t.Fatalf("failure not at top:\n%s", out)
	}
}

func TestWriteAll(t *testing.T) {
	root := t.TempDir()
	if err := WriteWorkflows(root); err != nil {
		t.Fatal(err)
	}
	if err := WriteRules(root, ".forge/rules", "python"); err != nil {
		t.Fatal(err)
	}
	created, err := WriteMCP(root)
	if err != nil || !created {
		t.Fatalf("mcp: %v %v", created, err)
	}
	if created, _ = WriteMCP(root); created {
		t.Fatal("mcp must not be overwritten")
	}
	data, _ := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if !json.Valid(data) {
		t.Fatal("invalid .mcp.json")
	}
	for _, p := range []string{".agent/workflows/ship.md", ".forge/rules/complexity-guard.yml"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Fatal(err)
		}
	}
}
