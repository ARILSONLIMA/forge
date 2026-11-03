package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ARILSONLIMA/forge/internal/metrics"
	"github.com/ARILSONLIMA/forge/internal/state"
)

// ErrRawReadBlocked is returned when an uncompressed file read is attempted in restricted stages.
var ErrRawReadBlocked = errors.New("raw file read blocked by forge guard")

// ErrSecretReadBlocked is returned when a .env or secret file read is attempted.
var ErrSecretReadBlocked = errors.New("secret file read blocked by forge shield")

// DefaultMaxRawReadLines is the maximum number of lines allowed for cat/head/tail on source files.
const DefaultMaxRawReadLines = 100

// DefaultHeadroomMaxLines is the line threshold above which terminal output is truncated.
const DefaultHeadroomMaxLines = 50

// DefaultHeadCount is the number of initial lines preserved during output truncation.
const DefaultHeadCount = 20

// DefaultTailCount is the number of trailing lines preserved during output truncation.
const DefaultTailCount = 20

// SourceExtensions contains common code file extensions subject to raw read guards.
var SourceExtensions = map[string]bool{
	".go":    true,
	".ts":    true,
	".tsx":   true,
	".js":    true,
	".jsx":   true,
	".py":    true,
	".rs":    true,
	".c":     true,
	".cpp":   true,
	".h":     true,
	".hpp":   true,
	".java":  true,
	".kt":    true,
	".rb":    true,
	".php":   true,
	".cs":    true,
	".swift": true,
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// CheckRawRead inspects a command invocation and blocks raw file reads
// on large source code files during BUILD, REVIEW, and SIMPLIFY stages,
// and strictly blocks .env* reads across all stages (Secret Shield).
func CheckRawRead(st state.Stage, cmdArgs []string, rootDir string) error {
	if len(cmdArgs) == 0 {
		return nil
	}

	prog := filepath.Base(cmdArgs[0])
	switch prog {
	case "cat", "head", "tail", "more", "less", "type":
		// Direct file readers
	default:
		return nil
	}

	for _, arg := range cmdArgs[1:] {
		// Skip flags like -n, -f, --lines, etc.
		if strings.HasPrefix(arg, "-") {
			continue
		}

		targetPath := arg
		if !filepath.IsAbs(targetPath) && rootDir != "" {
			targetPath = filepath.Join(rootDir, targetPath)
		}

		baseName := strings.ToLower(filepath.Base(targetPath))

		// 1. Secret Shield: Block .env reads regardless of stage
		if strings.HasPrefix(baseName, ".env") {
			if rootDir != "" {
				_ = metrics.NewTracker(rootDir).RecordSaving("secret_shield", 500)
			}
			return fmt.Errorf("%w: [FORGE SECRET SHIELD] Leitura bruta de arquivo de ambiente (%s) bloqueada. Use 'forge env' ou 'forge doctor' para inspecionar variáveis sem expor credenciais.",
				ErrSecretReadBlocked, arg)
		}

		// 2. Active stage checks for source code files
		switch st {
		case state.StageBuild, state.StageReview, state.StageSimplify:
			// Active guard stages
		default:
			continue
		}

		ext := strings.ToLower(filepath.Ext(targetPath))
		if !SourceExtensions[ext] {
			continue
		}

		info, err := os.Stat(targetPath)
		if err != nil || info.IsDir() {
			continue
		}

		lines, err := countLines(targetPath)
		if err != nil {
			continue
		}

		if lines > DefaultMaxRawReadLines {
			if rootDir != "" {
				_ = metrics.NewTracker(rootDir).RecordSaving("raw_read_blocks", lines*4)
			}
			return fmt.Errorf("%w: [FORGE BLOCK] Leitura bruta bloqueada no estágio %s (%s tem %d linhas). Use as tools do Serena MCP (get_symbols_overview / find_symbol), 'forge map' ou 'rtk read -l aggressive %s'.",
				ErrRawReadBlocked, st, arg, lines, arg)
		}
	}

	return nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

// SanitizeOutput applies ANSI stripping, noise reduction and Headroom truncation.
// If line count > maxLines, preserves head + tail, saves full output to logFilePath,
// and returns truncated text with banner.
func SanitizeOutput(raw string, maxLines, headCount, tailCount int, logFilePath string, tracker *metrics.Tracker) string {
	if raw == "" {
		return ""
	}

	// 1. Strip ANSI escape sequences
	clean := ansiRegex.ReplaceAllString(raw, "")
	clean = strings.ReplaceAll(clean, "\r\n", "\n")
	clean = strings.ReplaceAll(clean, "\r", "\n")

	// 2. Filter noisy redundant CLI notices
	lines := strings.Split(clean, "\n")
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "npm notice") || strings.HasPrefix(trimmed, "gsd-tools: warning:") {
			continue
		}
		filtered = append(filtered, l)
	}

	if len(filtered) <= maxLines {
		return strings.Join(filtered, "\n")
	}

	// 3. Save full output to logFilePath
	if logFilePath != "" {
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0o755); err == nil {
			_ = os.WriteFile(logFilePath, []byte(clean), 0o644)
		}
	}

	if headCount+tailCount >= len(filtered) {
		return strings.Join(filtered, "\n")
	}

	head := filtered[:headCount]
	tail := filtered[len(filtered)-tailCount:]
	omitted := len(filtered) - headCount - tailCount

	if tracker != nil {
		_ = tracker.RecordSaving("headroom_truncator", omitted*12)
	}

	banner := []string{
		"",
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		fmt.Sprintf("⚠️ [FORGE HEADROOM] Saída truncada (%d linhas intermediárias omitidas)", omitted),
		fmt.Sprintf("   Log integral preservado em: %s", logFilePath),
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		"",
	}

	result := make([]string, 0, headCount+len(banner)+tailCount)
	result = append(result, head...)
	result = append(result, banner...)
	result = append(result, tail...)

	return strings.Join(result, "\n")
}

// RTKSupportedCommands defines command bases that RTK specializes in compressing.
var RTKSupportedCommands = map[string]bool{
	"git":           true,
	"go":            true,
	"cargo":         true,
	"npm":           true,
	"pnpm":          true,
	"pytest":        true,
	"ruff":          true,
	"golangci-lint": true,
	"read":          true,
	"diff":          true,
	"test":          true,
	"lint":          true,
}

// RewriteForRTK inspects command arguments and rewrites them to pass through `rtk`
// if `rtk` is available on the system PATH and the command benefits from it.
// If rtk is not installed or the command is already prefixed with rtk, it returns args unchanged.
func RewriteForRTK(args []string, rtkPath string) []string {
	if len(args) == 0 || rtkPath == "" {
		return args
	}

	prog := filepath.Base(args[0])
	if prog == "rtk" {
		return args
	}

	// Direct match for commands with native RTK proxies
	if RTKSupportedCommands[prog] {
		return append([]string{rtkPath}, args...)
	}

	return args
}

// ExecResult contains the output and exit code of a command execution.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner provides command execution with RTK proxying and raw-read guards.
type Runner struct {
	RootDir string
	RTKPath string
	Tracker *metrics.Tracker
}

// NewRunner creates a new Runner, auto-discovering RTK if present in PATH.
func NewRunner(rootDir string) *Runner {
	rtkPath, _ := exec.LookPath("rtk")
	var tr *metrics.Tracker
	if rootDir != "" {
		tr = metrics.NewTracker(rootDir)
	}
	return &Runner{
		RootDir: rootDir,
		RTKPath: rtkPath,
		Tracker: tr,
	}
}

// Run executes a command, capturing stdout and stderr with sanitization.
func (r *Runner) Run(ctx context.Context, st state.Stage, cmdArgs []string) (*ExecResult, error) {
	if err := CheckRawRead(st, cmdArgs, r.RootDir); err != nil {
		return &ExecResult{
			Stderr:   err.Error(),
			ExitCode: 1,
		}, err
	}

	finalArgs := RewriteForRTK(cmdArgs, r.RTKPath)
	if len(finalArgs) == 0 {
		return nil, errors.New("empty command")
	}

	cmd := exec.CommandContext(ctx, finalArgs[0], finalArgs[1:]...)
	if r.RootDir != "" {
		cmd.Dir = r.RootDir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	logFile := filepath.Join(r.RootDir, ".forge", "logs", "last-command.log")
	stdoutSanitized := SanitizeOutput(stdoutBuf.String(), DefaultHeadroomMaxLines, DefaultHeadCount, DefaultTailCount, logFile, r.Tracker)
	stderrSanitized := SanitizeOutput(stderrBuf.String(), DefaultHeadroomMaxLines, DefaultHeadCount, DefaultTailCount, logFile, r.Tracker)

	return &ExecResult{
		Stdout:   stdoutSanitized,
		Stderr:   stderrSanitized,
		ExitCode: exitCode,
	}, err
}

// Stream executes a command while piping stdout and stderr to the provided writers in real time.
func (r *Runner) Stream(ctx context.Context, st state.Stage, cmdArgs []string, stdout, stderr io.Writer) (int, error) {
	if err := CheckRawRead(st, cmdArgs, r.RootDir); err != nil {
		if stderr != nil {
			_, _ = fmt.Fprintln(stderr, err.Error())
		}
		return 1, err
	}

	finalArgs := RewriteForRTK(cmdArgs, r.RTKPath)
	if len(finalArgs) == 0 {
		return 1, errors.New("empty command")
	}

	cmd := exec.CommandContext(ctx, finalArgs[0], finalArgs[1:]...)
	if r.RootDir != "" {
		cmd.Dir = r.RootDir
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), err
		}
		return 1, err
	}
	return 0, nil
}
