package state

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestInvalidTransitionsBlocked(t *testing.T) {
	s := NewInitialState()

	// Cannot jump from IDLE directly to BUILD
	if err := s.Transition(StageBuild); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition from IDLE -> BUILD, got %v", err)
	}

	// Move to SPEC
	if err := s.Transition(StageSpec); err != nil {
		t.Fatalf("unexpected error IDLE -> SPEC: %v", err)
	}

	// Cannot jump from SPEC directly to BUILD
	if err := s.Transition(StageBuild); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition from SPEC -> BUILD, got %v", err)
	}

	// Cannot advance SPEC -> PLAN without Gate 1 passed
	if err := s.Transition(StagePlan); !errors.Is(err, ErrGateNotPassed) {
		t.Fatalf("expected ErrGateNotPassed from SPEC -> PLAN without gate, got %v", err)
	}
}

func TestFullLinearCycleAndRollbacks(t *testing.T) {
	s := NewInitialState()

	// 1. IDLE -> SPEC
	if err := s.Transition(StageSpec); err != nil {
		t.Fatalf("IDLE -> SPEC: %v", err)
	}
	// Idempotent transition SPEC -> SPEC must succeed
	if err := s.Transition(StageSpec); err != nil {
		t.Fatalf("idempotent SPEC -> SPEC failed: %v", err)
	}

	// 2. SPEC -> PLAN
	s.MarkGatePassed(StageSpec)
	if err := s.Transition(StagePlan); err != nil {
		t.Fatalf("SPEC -> PLAN: %v", err)
	}

	// 3. PLAN -> BUILD
	s.MarkGatePassed(StagePlan)
	if err := s.Transition(StageBuild); err != nil {
		t.Fatalf("PLAN -> BUILD: %v", err)
	}

	// 4. BUILD -> TEST
	s.MarkGatePassed(StageBuild)
	if err := s.Transition(StageTest); err != nil {
		t.Fatalf("BUILD -> TEST: %v", err)
	}

	// Rollback route TEST -> BUILD on test failure
	if err := s.Transition(StageBuild); err != nil {
		t.Fatalf("rollback TEST -> BUILD: %v", err)
	}
	if s.GatesPassed[string(StageBuild)] {
		t.Fatalf("expected BUILD gate to be reset after rollback")
	}

	// Re-advance BUILD -> TEST -> REVIEW
	s.MarkGatePassed(StageBuild)
	if err := s.Transition(StageTest); err != nil {
		t.Fatalf("BUILD -> TEST (2nd): %v", err)
	}
	s.MarkGatePassed(StageTest)
	if err := s.Transition(StageReview); err != nil {
		t.Fatalf("TEST -> REVIEW: %v", err)
	}

	// Rollback route REVIEW -> BUILD on contract deviation
	if err := s.Transition(StageBuild); err != nil {
		t.Fatalf("rollback REVIEW -> BUILD: %v", err)
	}

	// Advance through SIMPLIFY -> SHIP -> IDLE
	s.MarkGatePassed(StageBuild)
	_ = s.Transition(StageTest)
	s.MarkGatePassed(StageTest)
	_ = s.Transition(StageReview)
	s.MarkGatePassed(StageReview)
	if err := s.Transition(StageSimplify); err != nil {
		t.Fatalf("REVIEW -> SIMPLIFY: %v", err)
	}
	s.MarkGatePassed(StageSimplify)
	if err := s.Transition(StageShip); err != nil {
		t.Fatalf("SIMPLIFY -> SHIP: %v", err)
	}
	if err := s.Transition(StageIdle); err != nil {
		t.Fatalf("SHIP -> IDLE: %v", err)
	}
}

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".forge", "state.json")

	s := NewInitialState()
	_ = s.Transition(StageSpec)
	s.ActiveSpec = ".forge/specs/auth-token-rotation.md"
	s.ActiveBeadID = "bd-a1b2"
	s.ModifiedSymbols = []string{"internal/auth/service.go::RotateToken"}

	if err := Save(path, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.CurrentStage != StageSpec {
		t.Fatalf("got stage %s, want %s", loaded.CurrentStage, StageSpec)
	}
	if loaded.ActiveSpec != s.ActiveSpec || loaded.ActiveBeadID != s.ActiveBeadID {
		t.Fatalf("loaded metadata mismatch: %+v", loaded)
	}
	if loaded.IntegrityHash == "" {
		t.Fatalf("expected non-empty IntegrityHash")
	}
}
