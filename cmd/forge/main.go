package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ARILSONLIMA/forge/internal/config"
	"github.com/ARILSONLIMA/forge/internal/gates"
	"github.com/ARILSONLIMA/forge/internal/runner"
	"github.com/ARILSONLIMA/forge/internal/setup"
	"github.com/ARILSONLIMA/forge/internal/state"
	"github.com/ARILSONLIMA/forge/internal/templates"
)

// Version is the current version of the Forge CLI.
var Version = "0.0.2"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	rootDir, err := findProjectRoot()
	if err != nil {
		rootDir = "."
	}

	ctx := context.Background()

	switch cmd {
	case "setup", "install-deps":
		err = runSetup(ctx, rootDir, args)
	case "init":
		err = runInit(rootDir, args)
	case "status":
		err = runStatus(rootDir)
	case "stage":
		err = runStage(ctx, rootDir, args)
	case "gate":
		err = runGate(ctx, rootDir, args)
	case "exec":
		err = runExec(ctx, rootDir, args)
	case "doctor":
		err = runDoctor(ctx, rootDir, args)
	case "version", "-v", "--version":
		fmt.Printf("forge version %s\n", Version)
		return
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q. Run 'forge help' for usage.\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Forge CLI (%s) - Deterministic AI Engineering Harness

Usage:
  forge <command> [arguments]

Commands:
  setup [--yes]     Install and configure ecosystem dependencies (rtk, bd, sg, uvx)
  init              Initialize Forge in current project (.forge/, .agent/, .mcp.json)
  status            Show current stage, active task, gates passed and integrity
  stage <name>      Transition to target stage (validates exit gate deterministically)
  gate [number]     Validate current quality gate without advancing stage
  exec [--] <cmd>   Execute shell command with RTK proxying and raw-read guards
  doctor [--install] Check environment dependencies (rtk, bd, sg, uvx, git) and config
  version           Display Forge CLI version
  help              Display this help message
`, Version)
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return ".", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".forge")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return os.Getwd()
}

func runInit(root string, args []string) error {
	fmt.Println("🔨 Initializing Forge harness...")

	// 1. Config
	cfgPath := filepath.Join(root, config.DefaultPath)
	var cfg config.Config
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg = config.DetectProject(root)
		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("✓ Created %s (Language: %s)\n", config.DefaultPath, cfg.Language)
	} else {
		loaded, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("reading existing config: %w", err)
		}
		cfg = loaded
		fmt.Printf("✓ Using existing %s\n", config.DefaultPath)
	}

	// 2. Directories
	for _, dir := range []string{
		filepath.Join(root, ".forge", "specs"),
		filepath.Join(root, ".forge", "rules"),
		filepath.Join(root, ".forge", "logs"),
		filepath.Join(root, ".agent", "rules"),
		filepath.Join(root, ".agent", "workflows"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	// 3. State
	stPath := filepath.Join(root, state.DefaultPath)
	var st *state.State
	if _, err := os.Stat(stPath); os.IsNotExist(err) {
		st = state.NewInitialState()
		if err := state.Save(stPath, st); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
		fmt.Printf("✓ Created %s (Current Stage: %s)\n", state.DefaultPath, st.CurrentStage)
	} else {
		loaded, err := state.Load(stPath)
		if err != nil {
			return fmt.Errorf("reading state: %w", err)
		}
		st = loaded
	}

	// 4. Workflows (.agent/workflows/*.md)
	if err := templates.WriteWorkflows(root); err != nil {
		return fmt.Errorf("writing workflows: %w", err)
	}
	fmt.Println("✓ Generated 7 Antigravity workflows in .agent/workflows/")

	// 5. ast-grep rules (.forge/rules/*.yml)
	if err := templates.WriteRules(root, cfg.Paths.RulesDir, cfg.Language); err != nil {
		return fmt.Errorf("writing ast-grep rules: %w", err)
	}
	fmt.Printf("✓ Installed ast-grep rules in %s\n", cfg.Paths.RulesDir)

	// 6. MCP Serena (.mcp.json)
	createdMCP, err := templates.WriteMCP(root)
	if err != nil {
		return fmt.Errorf("writing .mcp.json: %w", err)
	}
	if createdMCP {
		fmt.Println("✓ Generated .mcp.json (Serena MCP Server)")
	} else {
		fmt.Println("✓ Preserved existing .mcp.json")
	}

	// 7. Active stage rule
	content, err := templates.SwapStage(root, st)
	if err != nil {
		return fmt.Errorf("swapping initial stage rule: %w", err)
	}
	fmt.Printf("✓ Injected dynamic active-stage.md (~%d tokens)\n", templates.EstimateTokens(content))

	fmt.Println("\n🚀 Forge initialized successfully! Ready for '/spec' or 'forge stage spec'.")
	return nil
}

func runStatus(root string) error {
	stPath := filepath.Join(root, state.DefaultPath)
	st, err := state.Load(root)
	if err != nil {
		// Try relative to DefaultPath
		st, err = state.Load(stPath)
		if err != nil {
			return fmt.Errorf("cannot load state (run 'forge init' first): %w", err)
		}
	}

	cfgPath := filepath.Join(root, config.DefaultPath)
	cfg, _ := config.Load(cfgPath)

	hashStatus := "VALID"
	if !st.VerifyHash() {
		hashStatus = "TAMPERED (Hash mismatch!)"
	}

	elapsed := ""
	if t, err := time.Parse(time.RFC3339, st.StageEnteredAt); err == nil {
		dur := time.Since(t).Round(time.Second)
		elapsed = fmt.Sprintf(" (%s ago)", dur)
	}

	fmt.Printf("=== Forge Pipeline Status ===\n")
	fmt.Printf("Current Stage:    %s%s\n", st.CurrentStage, elapsed)
	if st.ActiveSpec != "" {
		fmt.Printf("Active Spec:      %s\n", st.ActiveSpec)
	} else {
		fmt.Printf("Active Spec:      (none)\n")
	}
	if st.ActiveBeadID != "" {
		fmt.Printf("Active Bead Task: %s\n", st.ActiveBeadID)
	} else {
		fmt.Printf("Active Bead Task: (none)\n")
	}
	fmt.Printf("Language:         %s\n", cfg.Language)
	fmt.Printf("Integrity Hash:   %s\n\n", hashStatus)

	fmt.Println("Quality Gates:")
	stageOrder := []state.Stage{
		state.StageSpec, state.StagePlan, state.StageBuild,
		state.StageTest, state.StageReview, state.StageSimplify,
	}
	for _, s := range stageOrder {
		status := " [ ] PENDING"
		if st.GatesPassed != nil && st.GatesPassed[string(s)] {
			status = " [✓] PASSED"
		}
		pointer := "  "
		if st.CurrentStage == s {
			pointer = "-> "
		}
		fmt.Printf("%s%-10s %s\n", pointer, s, status)
	}

	if st.TestFailCount > 0 {
		fmt.Printf("\n⚠️ Test Fail Count: %d\n", st.TestFailCount)
		if st.LastTestFailureSummary != "" {
			fmt.Printf("Last Failure:\n%s\n", st.LastTestFailureSummary)
		}
	}
	if st.ReviewDiffLines > 0 {
		fmt.Printf("Review Diff Lines Recorded: %d\n", st.ReviewDiffLines)
	}

	return nil
}

func runStage(ctx context.Context, root string, args []string) error {
	if len(args) == 0 {
		return errors.New("missing stage argument. Usage: forge stage <spec|plan|build|test|review|simplify|ship|idle>")
	}

	target, err := state.ParseStage(args[0])
	if err != nil {
		return err
	}

	st, err := state.Load(root)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if st.CurrentStage == target {
		fmt.Printf("Already in stage %s.\n", target)
		return nil
	}

	cfg, _ := config.Load(filepath.Join(root, config.DefaultPath))
	mgr := gates.NewManager(root, &cfg, st, runner.NewRunner(root), runner.NewBeadClient(root))

	// If moving forward in pipeline, execute the gate deterministically
	isRollback := (st.CurrentStage == state.StageTest && target == state.StageBuild) ||
		(st.CurrentStage == state.StageReview && target == state.StageBuild) ||
		(st.CurrentStage == state.StageSimplify && target == state.StageBuild)

	if !isRollback && st.CurrentStage != state.StageIdle {
		fmt.Printf("🔍 Running deterministic quality gate for %s...\n", st.CurrentStage)
		res, err := mgr.ValidateCurrentGate(ctx)
		if err != nil {
			fmt.Printf("❌ GATE REJECTED: %s\n", err)
			return fmt.Errorf("transition blocked by quality gate: %w", err)
		}
		fmt.Printf("✓ %s passed: %s\n", res.Name, res.Details)
	}

	// Apply transition
	if err := st.Transition(target); err != nil {
		return fmt.Errorf("transition failed: %w", err)
	}

	// Dynamic context swapping
	ruleContent, err := templates.SwapStage(root, st)
	if err != nil {
		return fmt.Errorf("failed to swap active stage rule: %w", err)
	}

	// Save updated state
	if err := state.Save(filepath.Join(root, state.DefaultPath), st); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	tokens := templates.EstimateTokens(ruleContent)
	fmt.Printf("✨ Swapped to stage %s. Active rule updated (%d tokens).\n", target, tokens)
	return nil
}

func runGate(ctx context.Context, root string, args []string) error {
	st, err := state.Load(root)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	cfg, _ := config.Load(filepath.Join(root, config.DefaultPath))
	mgr := gates.NewManager(root, &cfg, st, runner.NewRunner(root), runner.NewBeadClient(root))

	var res *gates.GateResult
	if len(args) > 0 {
		var gateNum int
		_, err := fmt.Sscanf(args[0], "%d", &gateNum)
		if err != nil || gateNum < 1 || gateNum > 7 {
			return fmt.Errorf("invalid gate number %q (must be 1-7)", args[0])
		}
		res, err = mgr.ValidateGate(ctx, gateNum)
	} else {
		res, err = mgr.ValidateCurrentGate(ctx)
	}

	if err != nil {
		fmt.Printf("❌ Gate failed: %v\n", err)
		return err
	}

	// Persist state updates (such as gate flags, active bead, etc.)
	_ = state.Save(filepath.Join(root, state.DefaultPath), st)

	fmt.Printf("✓ %s PASSED\nDetails: %s\n", res.Name, res.Details)
	return nil
}

func runExec(ctx context.Context, root string, args []string) error {
	if len(args) == 0 {
		return errors.New("missing command. Usage: forge exec [--] <command...>")
	}

	// Strip leading "--" if present
	if args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return errors.New("empty command after '--'")
	}

	st, err := state.Load(root)
	currentStage := state.StageIdle
	if err == nil && st != nil {
		currentStage = st.CurrentStage
	}

	r := runner.NewRunner(root)
	exitCode, err := r.Stream(ctx, currentStage, args, os.Stdout, os.Stderr)
	if err != nil {
		if errors.Is(err, runner.ErrRawReadBlocked) {
			os.Exit(1)
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return err
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

func runDoctor(ctx context.Context, root string, args []string) error {
	for _, a := range args {
		if a == "--install" || a == "--fix" {
			return runSetup(ctx, root, args)
		}
	}
	statuses := setup.CheckDependencies(ctx)
	fmt.Print(setup.FormatDoctorReport(statuses, root))
	return nil
}

func runSetup(ctx context.Context, root string, args []string) error {
	autoApprove := false
	binDir := setup.GetDefaultBinDir()

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-h" || a == "--help" {
			fmt.Println(`Usage: forge setup [options]

Install and configure all Forge dependencies and environment.

Options:
  -y, --yes       Automatically install all missing tools and run forge init without prompting
  --bin-dir=<dir> Target directory to install binaries (default: ~/.local/bin)
  -h, --help      Display this help message`)
			return nil
		} else if a == "-y" || a == "--yes" || a == "--all" || a == "--fix" || a == "--install" {
			autoApprove = true
		} else if strings.HasPrefix(a, "--bin-dir=") {
			binDir = strings.TrimPrefix(a, "--bin-dir=")
		} else if a == "--bin-dir" && i+1 < len(args) {
			binDir = args[i+1]
			i++
		}
	}

	shouldInit, err := setup.RunSetup(ctx, root, autoApprove, binDir)
	if err != nil {
		return err
	}

	if shouldInit {
		if err := runInit(root, nil); err != nil {
			return fmt.Errorf("failed to initialize project: %w", err)
		}
	}

	return nil
}
