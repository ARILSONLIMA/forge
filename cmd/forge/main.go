package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ARILSONLIMA/forge/internal/briefing"
	"github.com/ARILSONLIMA/forge/internal/checkpoint"
	"github.com/ARILSONLIMA/forge/internal/config"
	"github.com/ARILSONLIMA/forge/internal/debt"
	"github.com/ARILSONLIMA/forge/internal/envskeleton"
	"github.com/ARILSONLIMA/forge/internal/gates"
	"github.com/ARILSONLIMA/forge/internal/hooks"
	"github.com/ARILSONLIMA/forge/internal/mcp"
	"github.com/ARILSONLIMA/forge/internal/metrics"
	"github.com/ARILSONLIMA/forge/internal/runner"
	"github.com/ARILSONLIMA/forge/internal/setup"
	"github.com/ARILSONLIMA/forge/internal/shrink"
	"github.com/ARILSONLIMA/forge/internal/specs"
	"github.com/ARILSONLIMA/forge/internal/state"
	"github.com/ARILSONLIMA/forge/internal/symbols"
	"github.com/ARILSONLIMA/forge/internal/templates"
	"github.com/ARILSONLIMA/forge/internal/testisolate"
	"github.com/ARILSONLIMA/forge/internal/updater"
)

// Version is the current version of the Forge CLI.
var Version = "0.0.6"

func init() {
	setup.CurrentVersion = Version
}

// isExemptFromToolCheck returns true for commands that do not require full ecosystem tools
// (e.g., diagnostic and installation commands that fix missing tools).
func isExemptFromToolCheck(cmd string) bool {
	switch strings.ToLower(cmd) {
	case "setup", "install-deps", "doctor", "version", "-v", "--version", "help", "-h", "--help", "update", "spec", "specs":
		return true
	default:
		return false
	}
}

// filterSkipToolCheckArg checks and removes --skip-tool-check from argument list.
func filterSkipToolCheckArg(args []string) (bool, []string) {
	skip := false
	var filtered []string
	for _, a := range args {
		if a == "--skip-tool-check" {
			skip = true
		} else {
			filtered = append(filtered, a)
		}
	}
	return skip, filtered
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	skipCheck, filteredArgs := filterSkipToolCheckArg(args)
	args = filteredArgs

	rootDir, err := findProjectRoot()
	if err != nil {
		rootDir = "."
	}

	ctx := context.Background()

	// Pre-flight Hard Dependency Gate: Block execution if required ecosystem tools are missing or broken
	if !isExemptFromToolCheck(cmd) && !skipCheck && os.Getenv("FORGE_SKIP_TOOL_CHECK") == "" {
		enforceTools := true
		if cfg, cfgErr := config.Load(filepath.Join(rootDir, config.DefaultPath)); cfgErr == nil {
			enforceTools = cfg.StrictTools
		}
		if enforceTools {
			if err := setup.ValidateRequiredDependencies(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
	}

	switch cmd {
	case "setup", "install-deps":
		err = runSetup(ctx, rootDir, args)
	case "init":
		err = runInit(rootDir, args)
	case "status":
		err = runStatus(rootDir, args...)
	case "briefing":
		err = runBriefing(rootDir)
	case "map":
		err = runMap(rootDir)
	case "checkpoint":
		err = runCheckpoint(rootDir, args)
	case "rollback":
		err = runRollback(rootDir, args)
	case "test":
		err = runTest(ctx, rootDir, args)
	case "debt":
		err = runDebt(rootDir)
	case "env":
		err = runEnv(rootDir)
	case "shrink":
		err = runShrink(rootDir, args)
	case "stats":
		err = runStats(rootDir)
	case "hook":
		err = runHook(rootDir, args)
	case "mcp":
		err = runMCP(rootDir)
	case "spec", "specs":
		err = runSpec(rootDir, args)
	case "stage":
		err = runStage(ctx, rootDir, args)
	case "gate":
		err = runGate(ctx, rootDir, args)
	case "exec":
		err = runExec(ctx, rootDir, args)
	case "doctor":
		err = runDoctor(ctx, rootDir, args)
	case "update":
		err = runUpdate(ctx, rootDir, args)
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
  status [--brief]  Show current stage, active task, gates passed and integrity
  briefing          Display ultra-compact Zero-Read session briefing (<120 tokens)
  spec [list|<id>]  List specifications or activate by number/slug (e.g. 'forge spec 1')
  stage <name>      Transition to target stage (validates exit gate deterministically)
  gate [number]     Validate current quality gate without advancing stage
  map               Generate fast public symbol map of the codebase (<450 tokens)
  test [--failed]   Run tests via RTK, or rerun only failing tests with --failed
  checkpoint [lbl]  Create lightweight git snapshot checkpoint (or 'list')
  rollback [id]     Restore working tree to specified checkpoint
  debt              Audit technical debt and shortcuts (// SHORTCUT:, // TODO:)
  env               Inspect .env vs .env.example safely with zero secret leakage
  shrink <file>     Compress spec/markdown removing fluff while keeping code verbatim
  stats             Display Token Economy metrics and token savings breakdown
  hook <action>     Execute Antigravity lifecycle hooks (pretool, sessionstart)
  mcp               Start native embedded MCP JSON-RPC server over stdio
  exec [--] <cmd>   Execute shell command with RTK proxying and raw-read guards
  doctor [--install] Check environment dependencies (rtk, bd, sg, uvx, git) and config
  update [--check]  Check for updates and upgrade Forge CLI to latest version
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
		filepath.Join(root, ".forge", "backups"),
		filepath.Join(root, ".agents", "skills"),
		filepath.Join(root, ".agent", "rules"),
		filepath.Join(root, ".agent", "workflows"),
		filepath.Join(root, ".agent", "skills"),
		filepath.Join(root, ".agent", "hooks"),
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

	// 4. Antigravity Skills (.agents/skills/*) & Workflows (.agent/workflows/*.md)
	if err := templates.WriteSkills(root); err != nil {
		return fmt.Errorf("writing skills: %w", err)
	}
	if err := templates.WriteWorkflows(root); err != nil {
		return fmt.Errorf("writing workflows: %w", err)
	}
	fmt.Println("✓ Generated Antigravity skills in .agents/skills/ (/spec, /plan, /build, /test, /review, /simplify, /ship, /forge)")

	// 5. Antigravity Lifecycle Hooks (.agent/hooks/*.sh)
	if err := templates.WriteHooks(root, "forge"); err != nil {
		return fmt.Errorf("writing hooks: %w", err)
	}
	fmt.Println("✓ Installed Antigravity lifecycle hooks in .agent/hooks/")

	// 6. ast-grep rules (.forge/rules/*.yml)
	if err := templates.WriteRules(root, cfg.Paths.RulesDir, cfg.Language); err != nil {
		return fmt.Errorf("writing ast-grep rules: %w", err)
	}
	fmt.Printf("✓ Installed ast-grep rules in %s\n", cfg.Paths.RulesDir)

	// 7. MCP Configuration (.mcp.json)
	createdMCP, err := templates.WriteMCP(root)
	if err != nil {
		return fmt.Errorf("writing .mcp.json: %w", err)
	}
	if createdMCP {
		fmt.Println("✓ Generated .mcp.json (Forge & Serena MCP Servers)")
	} else {
		fmt.Println("✓ Preserved existing .mcp.json")
	}

	// 8. Project Roadmap (.forge/roadmap.md)
	roadmapPath := filepath.Join(root, specs.DefaultRoadmapPath)
	if created, err := specs.WriteRoadmapSkeleton(roadmapPath); err == nil && created {
		fmt.Println("✓ Generated project roadmap template in .forge/roadmap.md")
	}

	// 9. Active stage rule
	content, err := templates.SwapStage(root, st)
	if err != nil {
		return fmt.Errorf("swapping initial stage rule: %w", err)
	}
	_ = metrics.NewTracker(root).RecordSaving("stage_swapping", 4800)
	fmt.Printf("✓ Injected dynamic active-stage.md (~%d tokens)\n", templates.EstimateTokens(content))

	fmt.Println("\n🚀 Forge initialized successfully! Ready for '/spec' or 'forge stage spec'.")
	return nil
}

func runStatus(root string, args ...string) error {
	if len(args) > 0 {
		switch args[0] {
		case "--brief", "-b":
			return runBriefing(root)
		case "--stats", "-s":
			return runStats(root)
		}
	}

	stPath := filepath.Join(root, state.DefaultPath)
	st, err := state.Load(root)
	if err != nil {
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

	fmt.Printf("=== Forge Pipeline Status (v%s) ===\n", strings.TrimPrefix(Version, "v"))
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
	specsDir := cfg.Paths.SpecsDir
	if specsDir == "" {
		specsDir = specs.DefaultSpecsDir
	}
	if spList, _ := specs.ListSpecs(root, specsDir, st.ActiveSpec); len(spList) > 0 {
		fmt.Printf("Specifications:   %d registered in %s\n", len(spList), specsDir)
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
		if st.GatesPassed[string(s)] {
			status = " [✓] PASSED"
		}
		fmt.Printf("  Gate %-8s %s\n", s, status)
	}

	if st.LastTestFailureSummary != "" {
		fmt.Printf("\n⚠️ Last Test Failure Summary:\n%s\n", st.LastTestFailureSummary)
	}

	return nil
}

func runBriefing(root string) error {
	b, err := briefing.Generate(root, true)
	if err != nil {
		return err
	}
	fmt.Println(b)
	return nil
}

func runMap(root string) error {
	out, err := symbols.GenerateCodebaseMap(symbols.MapOptions{
		RootDir:     root,
		BudgetChars: 2000,
	})
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func runCheckpoint(root string, args []string) error {
	mgr := checkpoint.NewManager(root)
	if len(args) > 0 && args[0] == "list" {
		list, err := mgr.List()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("Nenhum checkpoint registrado.")
			return nil
		}
		fmt.Printf("Checkpoints registrados (%d):\n", len(list))
		for _, c := range list {
			fmt.Printf("  • %-32s [%s] (Commit: %s) @ %s\n", c.ID, c.Label, c.CommitHash, c.Timestamp)
		}
		return nil
	}

	label := "manual"
	if len(args) > 0 {
		label = args[0]
	}
	chk, err := mgr.Create(label)
	if err != nil {
		return err
	}
	fmt.Printf("✓ Checkpoint criado: %s [%s] (HEAD: %s)\n", chk.ID, chk.Label, chk.CommitHash)
	return nil
}

func runRollback(root string, args []string) error {
	id := ""
	if len(args) > 0 {
		id = args[0]
	}
	chk, err := checkpoint.NewManager(root).Rollback(id)
	if err != nil {
		return err
	}
	fmt.Printf("✓ Rollback executado com sucesso para checkpoint: %s [%s] (Commit: %s)\n", chk.ID, chk.Label, chk.CommitHash)
	return nil
}

func runTest(ctx context.Context, root string, args []string) error {
	isFailedOnly := false
	for _, a := range args {
		if a == "--failed" || a == "-f" {
			isFailedOnly = true
			break
		}
	}

	cfg, err := config.Load(filepath.Join(root, config.DefaultPath))
	if err != nil {
		cfg = config.DetectProject(root)
	}

	testCmd := strings.TrimSpace(cfg.Commands.Test)
	if testCmd == "" {
		testCmd = "go test ./..."
	}

	if isFailedOnly {
		rep, err := testisolate.LoadReport(root)
		if err != nil {
			return err
		}
		if rep == nil || len(rep.FailedTests) == 0 {
			fmt.Println("✓ Nenhum teste falho registrado. Executando comando padrão...")
		} else {
			testCmd = rep.IsolatedCommand
			fmt.Printf("🎯 Isolando %d teste(s) com falha:\n   Comando: %s\n\n", len(rep.FailedTests), testCmd)
		}
	}

	r := runner.NewRunner(root)
	cmdParts := strings.Fields(testCmd)
	exitCode, err := r.Stream(ctx, state.StageTest, cmdParts, os.Stdout, os.Stderr)
	if err != nil && exitCode != 0 {
		os.Exit(exitCode)
	}
	return err
}

func runDebt(root string) error {
	items, err := debt.Scan(root, nil)
	if err != nil {
		return err
	}
	fmt.Print(debt.FormatSummary(items))
	return nil
}

func runEnv(root string) error {
	rep, err := envskeleton.Inspect(root)
	if err != nil {
		return err
	}
	fmt.Print(rep.FormatSummary())
	return nil
}

func runShrink(root string, args []string) error {
	if len(args) == 0 {
		return errors.New("caminho do arquivo ausente. Uso: forge shrink <caminho-do-arquivo>")
	}
	res, err := shrink.ShrinkFile(root, args[0])
	if err != nil {
		return err
	}
	fmt.Printf("✓ Arquivo compactado com sucesso!\n")
	fmt.Printf("  • Linhas:  %d -> %d (%d linhas poupadas)\n", res.OriginalLines, res.ShrunkLines, res.OriginalLines-res.ShrunkLines)
	fmt.Printf("  • Bytes:   %d -> %d\n", res.OriginalBytes, res.ShrunkBytes)
	fmt.Printf("  • Backup:  %s\n", res.BackupPath)
	return nil
}

func runStats(root string) error {
	tr := metrics.NewTracker(root)
	data, err := tr.Load()
	if err != nil {
		return err
	}
	fmt.Print(data.SummaryString())
	return nil
}

func runHook(root string, args []string) error {
	if len(args) == 0 {
		return errors.New("ação de hook ausente. Uso: forge hook <pretool|sessionstart>")
	}
	switch args[0] {
	case "pretool":
		return hooks.HandlePreTool(root, os.Stdin)
	case "sessionstart":
		fmt.Println(hooks.HandleSessionStart(root))
		return nil
	default:
		return fmt.Errorf("ação de hook desconhecida: %q", args[0])
	}
}

func runMCP(root string) error {
	srv := mcp.NewServer(root, os.Stdin, os.Stdout)
	return srv.Serve()
}

func runSpec(root string, args []string) error {
	stPath := filepath.Join(root, state.DefaultPath)
	st, err := state.Load(root)
	if err != nil {
		st, err = state.Load(stPath)
		if err != nil {
			return fmt.Errorf("cannot load state: %w", err)
		}
	}

	cfgPath := filepath.Join(root, config.DefaultPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.DetectProject(root)
	}

	specsDir := cfg.Paths.SpecsDir
	if specsDir == "" {
		specsDir = specs.DefaultSpecsDir
	}
	roadmapPath := filepath.Join(root, specs.DefaultRoadmapPath)

	if len(args) == 0 || args[0] == "list" {
		specList, err := specs.ListSpecs(root, specsDir, st.ActiveSpec)
		if err != nil {
			return err
		}

		roadmapItems, _ := specs.ParseRoadmap(roadmapPath)

		fmt.Println("=== Specifications in " + specsDir + " ===")
		if len(specList) == 0 && len(roadmapItems) == 0 {
			fmt.Println("No specifications or roadmap items found. Use 'forge spec 1' or edit .forge/roadmap.md.")
			return nil
		}

		if len(specList) > 0 {
			for _, sp := range specList {
				activeMarker := "   "
				if sp.IsActive {
					activeMarker = "-> "
				}
				numStr := fmt.Sprintf("[%02d]", sp.Number)
				if sp.Number == 0 {
					numStr = " [--]"
				}
				fmt.Printf("%s%s %s (%s)\n", activeMarker, numStr, sp.Title, sp.RelPath)
				if sp.Goal != "" {
					fmt.Printf("      Goal: %s\n", sp.Goal)
				}
			}
		}

		if len(roadmapItems) > 0 {
			fmt.Println("\n=== Roadmap Items (.forge/roadmap.md) ===")
			for _, item := range roadmapItems {
				status := "[ ]"
				if item.Done {
					status = "[x]"
				}
				specExists := false
				for _, sp := range specList {
					if sp.Number == item.Number {
						specExists = true
						break
					}
				}
				specMarker := ""
				if specExists {
					specMarker = " (spec file exists)"
				} else {
					specMarker = fmt.Sprintf(" (pending /spec %d)", item.Number)
				}
				fmt.Printf("  %s [%02d] %s%s\n", status, item.Number, item.Title, specMarker)
			}
		}
		return nil
	}

	query := args[0]
	resSpec, err := specs.ResolveSpec(root, specsDir, roadmapPath, query)
	if err != nil {
		return err
	}

	created := false
	if !resSpec.Exists {
		if err := specs.CreateSpecSkeleton(resSpec.FullPath, resSpec.Title, resSpec.Goal); err != nil {
			return fmt.Errorf("creating spec skeleton: %w", err)
		}
		resSpec.Exists = true
		created = true
	}

	st.ActiveSpec = resSpec.RelPath
	if err := state.Save(stPath, st); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	content, err := templates.SwapStage(root, st)
	if err != nil {
		return fmt.Errorf("swapping stage: %w", err)
	}
	tokens := templates.EstimateTokens(content)

	if created {
		fmt.Printf("✓ Created specification skeleton at %s\n", resSpec.RelPath)
	}
	fmt.Printf("✓ Active specification set to: [%s] %s (%s)\n", resSpec.PaddedNum, resSpec.Title, resSpec.RelPath)
	if resSpec.Goal != "" {
		fmt.Printf("  Goal: %s\n", resSpec.Goal)
	}
	fmt.Printf("✓ Injected dynamic %s (~%d tokens)\n", templates.ActiveStagePath, tokens)
	return nil
}

func runStage(ctx context.Context, root string, args []string) error {
	if len(args) < 1 {
		return errors.New("missing stage name. Usage: forge stage <name> [spec-id]")
	}

	targetStage, err := state.ParseStage(args[0])
	if err != nil {
		return err
	}

	stPath := filepath.Join(root, state.DefaultPath)
	st, err := state.Load(root)
	if err != nil {
		st, err = state.Load(stPath)
		if err != nil {
			return fmt.Errorf("cannot load state: %w", err)
		}
	}

	cfgPath := filepath.Join(root, config.DefaultPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		detected := config.DetectProject(root)
		cfg = detected
	}

	specsDir := cfg.Paths.SpecsDir
	if specsDir == "" {
		specsDir = specs.DefaultSpecsDir
	}
	roadmapPath := filepath.Join(root, specs.DefaultRoadmapPath)

	specChanged := false
	if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
		specArg := strings.TrimSpace(args[1])
		resSpec, resErr := specs.ResolveSpec(root, specsDir, roadmapPath, specArg)
		if resErr == nil && resSpec != nil {
			if !resSpec.Exists {
				_ = specs.CreateSpecSkeleton(resSpec.FullPath, resSpec.Title, resSpec.Goal)
				resSpec.Exists = true
				fmt.Printf("✓ Created spec skeleton from roadmap: %s\n", resSpec.RelPath)
			}
			if st.ActiveSpec != resSpec.RelPath {
				st.ActiveSpec = resSpec.RelPath
				specChanged = true
			}
			fmt.Printf("✓ Active spec: [%s] %s (%s)\n", resSpec.PaddedNum, resSpec.Title, resSpec.RelPath)
		}
	}

	if st.CurrentStage == targetStage {
		if specChanged {
			if err := state.Save(stPath, st); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}
			content, _ := templates.SwapStage(root, st)
			tokens := templates.EstimateTokens(content)
			fmt.Printf("✓ Refreshed active spec for %s (~%d tokens)\n", targetStage, tokens)
			return nil
		}
		fmt.Printf("Already at stage %s\n", targetStage)
		return nil
	}

	mgr := gates.NewManager(root, &cfg, st, nil, nil)

	// Validate exit gate of current stage
	var gateRes *gates.GateResult
	if st.CurrentStage != state.StageIdle {
		gateRes, err = mgr.ValidateCurrentGate(ctx)
		if err != nil {
			_ = state.Save(stPath, st)
			return fmt.Errorf("cannot transition from %s to %s (quality gate failed): %w", st.CurrentStage, targetStage, err)
		}
	}

	// Perform state transition
	if err := st.Transition(targetStage); err != nil {
		return fmt.Errorf("transition denied: %w", err)
	}

	// Persist state atomically
	if err := state.Save(stPath, st); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Swap active stage rule
	content, err := templates.SwapStage(root, st)
	if err != nil {
		return fmt.Errorf("swapping stage rule: %w", err)
	}
	_ = metrics.NewTracker(root).RecordSaving("stage_swapping", 4800)

	tokens := templates.EstimateTokens(content)
	fmt.Printf("✓ Transitioned to %s\n", targetStage)
	if gateRes != nil {
		fmt.Printf("✓ %s passed: %s\n", gateRes.Name, gateRes.Details)
	}
	fmt.Printf("✓ Injected dynamic %s (~%d tokens)\n", templates.ActiveStagePath, tokens)
	return nil
}

func runGate(ctx context.Context, root string, args []string) error {
	stPath := filepath.Join(root, state.DefaultPath)
	st, err := state.Load(root)
	if err != nil {
		st, err = state.Load(stPath)
		if err != nil {
			return fmt.Errorf("cannot load state: %w", err)
		}
	}

	cfgPath := filepath.Join(root, config.DefaultPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		detected := config.DetectProject(root)
		cfg = detected
	}

	mgr := gates.NewManager(root, &cfg, st, nil, nil)
	var res *gates.GateResult

	if len(args) > 0 {
		gateNum := 0
		switch strings.ToUpper(args[0]) {
		case "1", "SPEC":
			gateNum = 1
		case "2", "PLAN":
			gateNum = 2
		case "3", "BUILD":
			gateNum = 3
		case "4", "TEST":
			gateNum = 4
		case "5", "REVIEW":
			gateNum = 5
		case "6", "SIMPLIFY":
			gateNum = 6
		case "7", "SHIP":
			gateNum = 7
		default:
			return fmt.Errorf("unknown gate identifier %q (use 1-7 or stage name)", args[0])
		}
		res, err = mgr.ValidateGate(ctx, gateNum)
	} else {
		res, err = mgr.ValidateCurrentGate(ctx)
	}

	if err != nil {
		fmt.Printf("❌ Gate failed: %v\n", err)
		return err
	}

	_ = state.Save(filepath.Join(root, state.DefaultPath), st)

	fmt.Printf("✓ %s PASSED\nDetails: %s\n", res.Name, res.Details)
	return nil
}

func runExec(ctx context.Context, root string, args []string) error {
	if len(args) == 0 {
		return errors.New("missing command. Usage: forge exec [--] <command...>")
	}

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
		if errors.Is(err, runner.ErrRawReadBlocked) || errors.Is(err, runner.ErrSecretReadBlocked) {
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
	fmt.Print(setup.FormatDoctorReport(statuses, root, Version))
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

func runUpdate(ctx context.Context, root string, args []string) error {
	opts := updater.UpdateOptions{
		Repository: updater.DefaultRepository,
		BinDir:     setup.GetDefaultBinDir(),
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Println(`Usage: forge update [options]

Check for updates and upgrade Forge CLI to the latest version.

Options:
  -y, --yes          Automatically apply update without interactive prompts
  --check            Check for updates without downloading or applying
  --force            Force recompilation and reinstallation even if up to date
  --bin-dir=<dir>    Target directory for the binary (default: ~/.local/bin)
  -h, --help         Display this help message`)
			return nil
		case a == "-y" || a == "--yes":
			opts.AutoApprove = true
		case a == "--check":
			opts.CheckOnly = true
		case a == "--force":
			opts.Force = true
		case strings.HasPrefix(a, "--bin-dir="):
			opts.BinDir = strings.TrimPrefix(a, "--bin-dir=")
		case a == "--bin-dir" && i+1 < len(args):
			opts.BinDir = args[i+1]
			i++
		}
	}

	return updater.RunUpdate(ctx, Version, root, opts)
}
