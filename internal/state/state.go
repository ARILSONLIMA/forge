package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Stage represents a lifecycle stage in the Forge deterministic state machine.
type Stage string

const (
	StageIdle     Stage = "IDLE"
	StageSpec     Stage = "SPEC"
	StagePlan     Stage = "PLAN"
	StageBuild    Stage = "BUILD"
	StageTest     Stage = "TEST"
	StageReview   Stage = "REVIEW"
	StageSimplify Stage = "SIMPLIFY"
	StageShip     Stage = "SHIP"
)

// DefaultPath is the standard path to state.json relative to the repository root.
const DefaultPath = ".forge/state.json"

// ErrInvalidTransition is returned when an illegal stage transition is attempted.
var ErrInvalidTransition = errors.New("invalid stage transition")

// ErrGateNotPassed is returned when attempting to advance without passing the current stage gate.
var ErrGateNotPassed = errors.New("required quality gate has not passed")

// State represents the persistent .forge/state.json structure.
type State struct {
	CurrentStage           Stage           `json:"current_stage"`
	ActiveSpec             string          `json:"active_spec"`
	ActiveBeadID           string          `json:"active_bead_id"`
	StageEnteredAt         string          `json:"stage_entered_at"`
	ModifiedSymbols        []string        `json:"modified_symbols"`
	GatesPassed            map[string]bool `json:"gates_passed"`
	TestFailCount          int             `json:"test_fail_count,omitempty"`
	LastTestFailureSummary string          `json:"last_test_failure_summary,omitempty"`
	ReviewDiffLines        int             `json:"review_diff_lines,omitempty"`
	IntegrityHash          string          `json:"integrity_hash,omitempty"`
}

// NewInitialState creates a clean state initialized at IDLE.
func NewInitialState() *State {
	s := &State{
		CurrentStage:    StageIdle,
		ActiveSpec:      "",
		ActiveBeadID:    "",
		StageEnteredAt:  time.Now().UTC().Format(time.RFC3339),
		ModifiedSymbols: []string{},
		GatesPassed:     defaultGatesMap(),
	}
	s.IntegrityHash = s.ComputeHash()
	return s
}

func defaultGatesMap() map[string]bool {
	return map[string]bool{
		string(StageSpec):     false,
		string(StagePlan):     false,
		string(StageBuild):    false,
		string(StageTest):     false,
		string(StageReview):   false,
		string(StageSimplify): false,
	}
}

// ParseStage normalizes and validates a stage name (case-insensitive).
func ParseStage(raw string) (Stage, error) {
	norm := Stage(strings.ToUpper(strings.TrimSpace(raw)))
	switch norm {
	case StageIdle, StageSpec, StagePlan, StageBuild, StageTest, StageReview, StageSimplify, StageShip:
		return norm, nil
	default:
		return "", fmt.Errorf("unknown stage %q (valid: idle, spec, plan, build, test, review, simplify, ship)", raw)
	}
}

// NextStage returns the default linear next stage in the pipeline.
func (s *State) NextStage() (Stage, error) {
	switch s.CurrentStage {
	case StageIdle:
		return StageSpec, nil
	case StageSpec:
		return StagePlan, nil
	case StagePlan:
		return StageBuild, nil
	case StageBuild:
		return StageTest, nil
	case StageTest:
		return StageReview, nil
	case StageReview:
		return StageSimplify, nil
	case StageSimplify:
		return StageShip, nil
	case StageShip:
		return StageIdle, nil
	default:
		return "", fmt.Errorf("no linear next stage from %s", s.CurrentStage)
	}
}

// CanTransition checks whether transitioning from s.CurrentStage to target is valid
// and whether the required quality gate has been marked as passed.
func (s *State) CanTransition(target Stage) error {
	// Idempotent transition to the same stage is always allowed.
	if s.CurrentStage == target {
		return nil
	}

	switch s.CurrentStage {
	case StageIdle:
		if target == StageSpec {
			return nil
		}
	case StageSpec:
		if target == StagePlan {
			return s.requireGate(StageSpec)
		}
	case StagePlan:
		if target == StageBuild {
			return s.requireGate(StagePlan)
		}
	case StageBuild:
		if target == StageTest {
			return s.requireGate(StageBuild)
		}
	case StageTest:
		if target == StageReview {
			return s.requireGate(StageTest)
		}
		if target == StageBuild {
			// Rollback route on test failure.
			return nil
		}
	case StageReview:
		if target == StageSimplify {
			return s.requireGate(StageReview)
		}
		if target == StageBuild {
			// Rollback route on contract deviation.
			return nil
		}
	case StageSimplify:
		if target == StageShip {
			return s.requireGate(StageSimplify)
		}
		if target == StageBuild {
			// Rollback route on regression during YAGNI pruning.
			return nil
		}
	case StageShip:
		if target == StageBuild || target == StageIdle || target == StageSpec {
			return nil
		}
	}

	return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidTransition, s.CurrentStage, target)
}

func (s *State) requireGate(stage Stage) error {
	if s.GatesPassed == nil || !s.GatesPassed[string(stage)] {
		return fmt.Errorf("%w: gate for stage %s must pass before advancing", ErrGateNotPassed, stage)
	}
	return nil
}

// MarkGatePassed records that the quality gate for a stage has passed.
func (s *State) MarkGatePassed(stage Stage) {
	if s.GatesPassed == nil {
		s.GatesPassed = defaultGatesMap()
	}
	s.GatesPassed[string(stage)] = true
}

// Transition validates and applies a transition to target stage.
// If current_stage == target, it acts idempotently without resetting timestamps.
func (s *State) Transition(target Stage) error {
	if s.CurrentStage == target {
		return nil
	}
	if err := s.CanTransition(target); err != nil {
		return err
	}

	prev := s.CurrentStage
	s.CurrentStage = target
	s.StageEnteredAt = time.Now().UTC().Format(time.RFC3339)

	// When rolling back to BUILD or starting a new cycle from SHIP -> BUILD/IDLE,
	// reset downstream cycle gates so they must be re-validated.
	if target == StageBuild && (prev == StageTest || prev == StageReview || prev == StageSimplify || prev == StageShip) {
		s.GatesPassed[string(StageBuild)] = false
		s.GatesPassed[string(StageTest)] = false
		s.GatesPassed[string(StageReview)] = false
		s.GatesPassed[string(StageSimplify)] = false
		if prev == StageShip {
			s.TestFailCount = 0
			s.LastTestFailureSummary = ""
			s.ReviewDiffLines = 0
		}
	}

	if target == StageIdle || target == StageSpec {
		s.GatesPassed = defaultGatesMap()
		s.TestFailCount = 0
		s.LastTestFailureSummary = ""
		s.ReviewDiffLines = 0
		if target == StageIdle {
			s.ActiveBeadID = ""
		}
	}

	s.IntegrityHash = s.ComputeHash()
	return nil
}

// ComputeHash calculates a deterministic SHA-256 integrity hash over state fields.
func (s *State) ComputeHash() string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%v",
		s.CurrentStage,
		s.ActiveSpec,
		s.ActiveBeadID,
		s.StageEnteredAt,
		s.GatesPassed,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}

// VerifyHash checks whether the stored IntegrityHash matches the computed hash.
func (s *State) VerifyHash() bool {
	if s.IntegrityHash == "" {
		return false
	}
	return s.IntegrityHash == s.ComputeHash()
}

// Save atomically writes the state JSON to disk.
func Save(path string, s *State) error {
	if s.ModifiedSymbols == nil {
		s.ModifiedSymbols = []string{}
	}
	if s.GatesPassed == nil {
		s.GatesPassed = defaultGatesMap()
	}
	s.IntegrityHash = s.ComputeHash()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp state: %w", err)
	}
	return nil
}

// Load reads .forge/state.json from disk. Path can be the state file or project root.
func Load(path string) (*State, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, DefaultPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state json: %w", err)
	}
	if s.ModifiedSymbols == nil {
		s.ModifiedSymbols = []string{}
	}
	if s.GatesPassed == nil {
		s.GatesPassed = defaultGatesMap()
	}
	return &s, nil
}
