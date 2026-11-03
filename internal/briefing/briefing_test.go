package briefing

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ARILSONLIMA/forge/internal/state"
)

func TestBriefingGenerate(t *testing.T) {
	tmp := t.TempDir()

	// Initialize state
	st := state.NewInitialState()
	st.CurrentStage = state.StageBuild
	st.ActiveBeadID = "bd-test"
	st.ActiveSpec = ".forge/specs/auth.md"
	if err := state.Save(filepath.Join(tmp, state.DefaultPath), st); err != nil {
		t.Fatal(err)
	}

	brief, err := Generate(tmp, true)
	if err != nil {
		t.Fatalf("generate briefing err: %v", err)
	}

	if !strings.Contains(brief, "Estágio: BUILD") || !strings.Contains(brief, "bd-test") {
		t.Fatalf("unexpected brief: %s", brief)
	}
}
