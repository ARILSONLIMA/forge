package gates

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ARILSONLIMA/forge/internal/config"
	"github.com/ARILSONLIMA/forge/internal/runner"
	"github.com/ARILSONLIMA/forge/internal/state"
)

// GateResult holds the outcome of a quality gate validation.
type GateResult struct {
	GateNumber int         `json:"gate_number"`
	Name       string      `json:"name"`
	From       state.Stage `json:"from"`
	To         state.Stage `json:"to"`
	Passed     bool        `json:"passed"`
	Error      string      `json:"error,omitempty"`
	Details    string      `json:"details,omitempty"`
}

// GitExecutor runs git commands in a directory.
type GitExecutor func(ctx context.Context, dir string, args ...string) (string, error)

// DefaultGitExecutor uses the system git binary.
func DefaultGitExecutor(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Manager coordinates validation of the 7 Forge deterministic Quality Gates.
type Manager struct {
	RootDir       string
	Config        *config.Config
	State         *state.State
	Runner        *runner.Runner
	Beads         runner.BeadClient
	GitExec       GitExecutor
	AuditLogPath  string
	SGPath        string // ast-grep / sg path
}

// NewManager creates a Gate Manager initialized with defaults.
func NewManager(rootDir string, cfg *config.Config, st *state.State, r *runner.Runner, beads runner.BeadClient) *Manager {
	if cfg == nil {
		detected := config.DetectProject(rootDir)
		cfg = &detected
	}
	if r == nil {
		r = runner.NewRunner(rootDir)
	}
	if beads == nil {
		beads = runner.NewBeadClient(rootDir)
	}

	sgPath, _ := exec.LookPath("sg")
	if sgPath == "" {
		sgPath, _ = exec.LookPath("ast-grep")
	}

	return &Manager{
		RootDir:      rootDir,
		Config:       cfg,
		State:        st,
		Runner:       r,
		Beads:        beads,
		GitExec:      DefaultGitExecutor,
		AuditLogPath: filepath.Join(rootDir, ".forge", "logs", "gate-audit.log"),
		SGPath:       sgPath,
	}
}

// ValidateGate validates a specific gate number (1 through 7).
func (m *Manager) ValidateGate(ctx context.Context, gateNum int) (*GateResult, error) {
	var res *GateResult
	var err error

	switch gateNum {
	case 1:
		res, err = m.ValidateGate1(ctx)
	case 2:
		res, err = m.ValidateGate2(ctx)
	case 3:
		res, err = m.ValidateGate3(ctx)
	case 4:
		res, err = m.ValidateGate4(ctx)
	case 5:
		res, err = m.ValidateGate5(ctx)
	case 6:
		res, err = m.ValidateGate6(ctx)
	case 7:
		res, err = m.ValidateGate7(ctx)
	default:
		return nil, fmt.Errorf("invalid gate number %d (must be 1-7)", gateNum)
	}

	if logErr := m.AppendAuditLog(res); logErr != nil {
		// Log error is non-fatal for gate result, but we record it
		_ = logErr
	}

	return res, err
}

// ValidateCurrentGate validates the gate corresponding to the state's current stage.
func (m *Manager) ValidateCurrentGate(ctx context.Context) (*GateResult, error) {
	switch m.State.CurrentStage {
	case state.StageSpec:
		return m.ValidateGate(ctx, 1)
	case state.StagePlan:
		return m.ValidateGate(ctx, 2)
	case state.StageBuild:
		return m.ValidateGate(ctx, 3)
	case state.StageTest:
		return m.ValidateGate(ctx, 4)
	case state.StageReview:
		return m.ValidateGate(ctx, 5)
	case state.StageSimplify:
		return m.ValidateGate(ctx, 6)
	case state.StageShip:
		return m.ValidateGate(ctx, 7)
	case state.StageIdle:
		return &GateResult{
			GateNumber: 0,
			Name:       "IDLE",
			From:       state.StageIdle,
			To:         state.StageSpec,
			Passed:     true,
			Details:    "Idle stage has no blocking gate",
		}, nil
	default:
		return nil, fmt.Errorf("unknown stage %s", m.State.CurrentStage)
	}
}

// ValidateGate1: SPEC -> PLAN
// Verifies markdown spec contains the 4 mandatory sections and is between 15 and 120 lines.
func (m *Manager) ValidateGate1(ctx context.Context) (*GateResult, error) {
	res := &GateResult{
		GateNumber: 1,
		Name:       "Gate 1 (Spec Validation)",
		From:       state.StageSpec,
		To:         state.StagePlan,
	}

	specPath := m.State.ActiveSpec
	if specPath == "" {
		// Discover if there is a single spec in .forge/specs/
		specsDir := filepath.Join(m.RootDir, m.Config.Paths.SpecsDir)
		entries, _ := os.ReadDir(specsDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				specPath = filepath.Join(m.Config.Paths.SpecsDir, e.Name())
				m.State.ActiveSpec = specPath
				break
			}
		}
	}

	if specPath == "" {
		res.Error = "active_spec is not defined and no spec found in .forge/specs/"
		return res, errors.New(res.Error)
	}

	fullPath := specPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(m.RootDir, specPath)
	}

	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		res.Error = fmt.Sprintf("failed to read spec file %s: %v", specPath, err)
		return res, errors.New(res.Error)
	}

	lines := strings.Split(string(contentBytes), "\n")
	lineCount := len(lines)
	if lineCount < 15 || lineCount > 120 {
		res.Error = fmt.Sprintf("spec line count %d is outside required range [15, 120] lines", lineCount)
		return res, errors.New(res.Error)
	}

	// Required 4 sections
	reqSections := []string{
		"## Objetivo",
		"## Contratos e Assinaturas",
		"## Casos de Borda",
		"## Fora de Escopo",
	}

	contentLower := strings.ToLower(string(contentBytes))
	var missing []string
	for _, sec := range reqSections {
		if !strings.Contains(contentLower, strings.ToLower(sec)) {
			missing = append(missing, sec)
		}
	}

	if len(missing) > 0 {
		res.Error = fmt.Sprintf("spec is missing mandatory section(s): %s", strings.Join(missing, ", "))
		return res, errors.New(res.Error)
	}

	res.Passed = true
	res.Details = fmt.Sprintf("Spec %s validated (%d lines, all 4 sections present)", specPath, lineCount)
	m.State.MarkGatePassed(state.StageSpec)
	return res, nil
}

// ValidateGate2: PLAN -> BUILD
// Verifies bd ready --json returns at least 1 open task, and selects the first active bead id.
func (m *Manager) ValidateGate2(ctx context.Context) (*GateResult, error) {
	res := &GateResult{
		GateNumber: 2,
		Name:       "Gate 2 (Beads Task Readiness)",
		From:       state.StagePlan,
		To:         state.StageBuild,
	}

	tasks, err := m.Beads.Ready(ctx)
	if err != nil {
		res.Error = fmt.Sprintf("failed to check ready tasks via Beads: %v", err)
		return res, errors.New(res.Error)
	}

	if len(tasks) == 0 {
		res.Error = "no unblocked tasks ready in Beads DAG (bd ready returned 0 tasks)"
		return res, errors.New(res.Error)
	}

	activeTask := tasks[0]
	m.State.ActiveBeadID = activeTask.ID
	res.Passed = true
	res.Details = fmt.Sprintf("Ready task selected: %s (%s). Total queue: %d", activeTask.ID, activeTask.Title, len(tasks))
	m.State.MarkGatePassed(state.StagePlan)
	return res, nil
}

// ValidateGate3: BUILD -> TEST
// Verifies code compiles cleanly and working tree has modified files.
func (m *Manager) ValidateGate3(ctx context.Context) (*GateResult, error) {
	res := &GateResult{
		GateNumber: 3,
		Name:       "Gate 3 (Build & Compilation)",
		From:       state.StageBuild,
		To:         state.StageTest,
	}

	buildCmd := strings.TrimSpace(m.Config.Commands.Build)
	if buildCmd != "" {
		args := strings.Fields(buildCmd)
		execRes, err := m.Runner.Run(ctx, state.StageBuild, args)
		if err != nil || execRes.ExitCode != 0 {
			res.Error = fmt.Sprintf("build command %q failed (exit code %d): %s %s", buildCmd, execRes.ExitCode, execRes.Stdout, execRes.Stderr)
			return res, errors.New(res.Error)
		}
	}

	// Check git modified files
	statusOut, err := m.GitExec(ctx, m.RootDir, "status", "--porcelain")
	if err != nil {
		res.Error = fmt.Sprintf("failed to inspect git status: %v: %s", err, statusOut)
		return res, errors.New(res.Error)
	}

	var modifiedFiles []string
	scanner := bufio.NewScanner(strings.NewReader(statusOut))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) >= 3 {
			modifiedFiles = append(modifiedFiles, strings.TrimSpace(line[2:]))
		}
	}

	if len(modifiedFiles) == 0 {
		res.Error = "compilation succeeded but git working tree has no modified files"
		return res, errors.New(res.Error)
	}

	m.State.ModifiedSymbols = modifiedFiles
	res.Passed = true
	res.Details = fmt.Sprintf("Build clean. Modified files: %d (%s)", len(modifiedFiles), strings.Join(modifiedFiles, ", "))
	m.State.MarkGatePassed(state.StageBuild)
	return res, nil
}

// ValidateGate4: TEST -> REVIEW
// Verifies test suite returns exit code 0 and at least 1 test file is touched/validated.
// Enforces auto-rollback if test fails consecutively.
func (m *Manager) ValidateGate4(ctx context.Context) (*GateResult, error) {
	res := &GateResult{
		GateNumber: 4,
		Name:       "Gate 4 (Test Suite Verification)",
		From:       state.StageTest,
		To:         state.StageReview,
	}

	testCmd := strings.TrimSpace(m.Config.Commands.Test)
	if testCmd == "" {
		testCmd = "go test ./..."
	}

	args := strings.Fields(testCmd)
	execRes, err := m.Runner.Run(ctx, state.StageTest, args)
	if err != nil || execRes.ExitCode != 0 {
		m.State.TestFailCount++
		combinedOutput := execRes.Stdout + "\n" + execRes.Stderr
		summary := extractLastLines(combinedOutput, 15)
		m.State.LastTestFailureSummary = summary

		if m.State.TestFailCount >= 2 {
			_ = m.State.Transition(state.StageBuild)
			res.Error = fmt.Sprintf("test suite failed twice consecutively. Auto-rollback to BUILD initiated:\n%s", summary)
		} else {
			res.Error = fmt.Sprintf("test suite failed (attempt %d):\n%s", m.State.TestFailCount, summary)
		}
		return res, errors.New(res.Error)
	}

	// Verify test file touched
	statusOut, _ := m.GitExec(ctx, m.RootDir, "status", "--porcelain")
	diffOut, _ := m.GitExec(ctx, m.RootDir, "diff", "--name-only")
	combinedFiles := statusOut + "\n" + diffOut
	if !hasTestFile(combinedFiles) {
		res.Error = "test suite passed but no test file (*_test.*, *.spec.*) was touched or created in git status/diff"
		return res, errors.New(res.Error)
	}

	// Success: reset failures and record current diff lines for Gate 6 comparison
	m.State.TestFailCount = 0
	m.State.LastTestFailureSummary = ""
	diffLines := m.countDiffLines(ctx)
	m.State.ReviewDiffLines = diffLines

	res.Passed = true
	res.Details = fmt.Sprintf("All tests passed. Review diff lines recorded: %d", diffLines)
	m.State.MarkGatePassed(state.StageTest)
	return res, nil
}

func hasTestFile(text string) bool {
	lines := strings.Split(text, "\n")
	for _, l := range lines {
		l = strings.ToLower(strings.TrimSpace(l))
		if strings.Contains(l, "_test.") || strings.Contains(l, ".test.") ||
			strings.Contains(l, ".spec.") || strings.Contains(l, "test_") {
			return true
		}
	}
	return false
}

func extractLastLines(text string, count int) string {
	raw := strings.Split(strings.TrimSpace(text), "\n")
	if len(raw) <= count {
		return strings.Join(raw, "\n")
	}
	return strings.Join(raw[len(raw)-count:], "\n")
}

func (m *Manager) countDiffLines(ctx context.Context) int {
	out, err := m.GitExec(ctx, m.RootDir, "diff", "--shortstat")
	if err != nil || out == "" {
		return 0
	}
	// Example shortstat: " 3 files changed, 45 insertions(+), 12 deletions(-)"
	parts := strings.Split(out, ",")
	total := 0
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, "insertion") || strings.Contains(p, "deletion") {
			fields := strings.Fields(p)
			if len(fields) > 0 {
				if n, err := strconv.Atoi(fields[0]); err == nil {
					total += n
				}
			}
		}
	}
	return total
}

// ValidateGate5: REVIEW -> SIMPLIFY
// Verifies lint passes, ast-grep finds zero critical violations, and diff <= max threshold.
func (m *Manager) ValidateGate5(ctx context.Context) (*GateResult, error) {
	res := &GateResult{
		GateNumber: 5,
		Name:       "Gate 5 (Lint, AST-Grep & Diff Threshold)",
		From:       state.StageReview,
		To:         state.StageSimplify,
	}

	lintCmd := strings.TrimSpace(m.Config.Commands.Lint)
	if lintCmd != "" {
		args := strings.Fields(lintCmd)
		execRes, err := m.Runner.Run(ctx, state.StageReview, args)
		if err != nil || execRes.ExitCode != 0 {
			res.Error = fmt.Sprintf("lint command %q failed (exit code %d): %s %s", lintCmd, execRes.ExitCode, execRes.Stdout, execRes.Stderr)
			return res, errors.New(res.Error)
		}
	}

	// AST-Grep scan if sg / ast-grep is present
	rulesDir := filepath.Join(m.RootDir, m.Config.Paths.RulesDir)
	if m.SGPath != "" {
		if entries, err := os.ReadDir(rulesDir); err == nil && len(entries) > 0 {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml") {
					ruleFile := filepath.Join(rulesDir, e.Name())
					sgArgs := []string{m.SGPath, "scan", "--rule", ruleFile}
					sgRes, _ := m.Runner.Run(ctx, state.StageReview, sgArgs)
					if sgRes.ExitCode != 0 || strings.Contains(sgRes.Stdout, "error:") || strings.Contains(sgRes.Stderr, "error:") {
						if m.Config.GateThresholds.AstGrepStrict {
							res.Error = fmt.Sprintf("ast-grep detected violation in %s: %s %s", e.Name(), sgRes.Stdout, sgRes.Stderr)
							return res, errors.New(res.Error)
						}
					}
				}
			}
		}
	}

	// Diff lines threshold
	currentDiff := m.countDiffLines(ctx)
	maxDiff := m.Config.GateThresholds.MaxDiffLinesPerTicket
	if maxDiff > 0 && currentDiff > maxDiff {
		res.Error = fmt.Sprintf("diff size (%d lines) exceeds max_diff_lines_per_ticket threshold (%d)", currentDiff, maxDiff)
		return res, errors.New(res.Error)
	}

	res.Passed = true
	res.Details = fmt.Sprintf("Lint clean, ast-grep passed, diff lines: %d (max: %d)", currentDiff, maxDiff)
	m.State.MarkGatePassed(state.StageReview)
	return res, nil
}

// ValidateGate6: SIMPLIFY -> SHIP
// Re-executes test suite after YAGNI pruning and ensures final diff <= Review diff.
func (m *Manager) ValidateGate6(ctx context.Context) (*GateResult, error) {
	res := &GateResult{
		GateNumber: 6,
		Name:       "Gate 6 (YAGNI Pruning & Regression Check)",
		From:       state.StageSimplify,
		To:         state.StageShip,
	}

	testCmd := strings.TrimSpace(m.Config.Commands.Test)
	if testCmd == "" {
		testCmd = "go test ./..."
	}

	args := strings.Fields(testCmd)
	execRes, err := m.Runner.Run(ctx, state.StageSimplify, args)
	if err != nil || execRes.ExitCode != 0 {
		res.Error = fmt.Sprintf("regression detected after simplification: test suite broke:\n%s\n%s", execRes.Stdout, execRes.Stderr)
		return res, errors.New(res.Error)
	}

	currentDiff := m.countDiffLines(ctx)
	// If review diff was recorded, final diff should not have grown substantially
	if m.State.ReviewDiffLines > 0 && currentDiff > m.State.ReviewDiffLines {
		res.Error = fmt.Sprintf("diff grew during SIMPLIFY stage (was %d in Review, now %d). Simplification must reduce or keep diff equal.",
			m.State.ReviewDiffLines, currentDiff)
		return res, errors.New(res.Error)
	}

	res.Passed = true
	res.Details = fmt.Sprintf("Regression tests passed. Final diff lines: %d (Review baseline: %d)", currentDiff, m.State.ReviewDiffLines)
	m.State.MarkGatePassed(state.StageSimplify)
	return res, nil
}

// ValidateGate7: SHIP -> IDLE / BUILD
// Verifies active task is closed in Beads and Git working tree is completely clean.
func (m *Manager) ValidateGate7(ctx context.Context) (*GateResult, error) {
	res := &GateResult{
		GateNumber: 7,
		Name:       "Gate 7 (Ship & Beads Close Verification)",
		From:       state.StageShip,
		To:         state.StageIdle, // or StageBuild if more tasks remain
	}

	if m.State.ActiveBeadID != "" {
		task, err := m.Beads.Show(ctx, m.State.ActiveBeadID)
		if err != nil {
			res.Error = fmt.Sprintf("failed to inspect active task %s in Beads: %v", m.State.ActiveBeadID, err)
			return res, errors.New(res.Error)
		}
		if task.Status != "closed" {
			res.Error = fmt.Sprintf("active bead %s is not marked 'closed' (current status: %q). Please run 'bd close %s'.", m.State.ActiveBeadID, task.Status, m.State.ActiveBeadID)
			return res, errors.New(res.Error)
		}
	}

	// Working tree must be completely clean (no uncommitted changes)
	statusOut, err := m.GitExec(ctx, m.RootDir, "status", "--porcelain")
	if err != nil {
		res.Error = fmt.Sprintf("failed to check git status: %v: %s", err, statusOut)
		return res, errors.New(res.Error)
	}
	if strings.TrimSpace(statusOut) != "" {
		res.Error = fmt.Sprintf("working tree has uncommitted changes. Please commit changes before shipping:\n%s", statusOut)
		return res, errors.New(res.Error)
	}

	// Determine next destination: if more ready beads exist, next stage is BUILD; otherwise IDLE
	readyTasks, _ := m.Beads.Ready(ctx)
	if len(readyTasks) > 0 {
		res.To = state.StageBuild
		res.Details = fmt.Sprintf("Active task %s closed and clean commit verified. Next ready task: %s (%s). Advancing to BUILD.",
			m.State.ActiveBeadID, readyTasks[0].ID, readyTasks[0].Title)
	} else {
		res.To = state.StageIdle
		res.Details = fmt.Sprintf("Active task %s closed and clean commit verified. No more ready tasks in Beads DAG. Returning to IDLE.",
			m.State.ActiveBeadID)
	}

	res.Passed = true
	return res, nil
}

// AppendAuditLog records a gate execution result in .forge/logs/gate-audit.log.
func (m *Manager) AppendAuditLog(res *GateResult) error {
	if res == nil || m.AuditLogPath == "" {
		return nil
	}

	dir := filepath.Dir(m.AuditLogPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(m.AuditLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	status := "PASS"
	msg := res.Details
	if !res.Passed {
		status = "FAIL"
		msg = res.Error
	}

	entry := fmt.Sprintf("[%s] GATE %d (%s -> %s): %s - %s\n",
		time.Now().UTC().Format(time.RFC3339),
		res.GateNumber,
		res.From,
		res.To,
		status,
		strings.TrimSpace(msg),
	)

	_, err = f.WriteString(entry)
	return err
}
