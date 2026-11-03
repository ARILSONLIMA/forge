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
	"strings"

	"github.com/ARILSONLIMA/forge/internal/state"
)

// ErrRawReadBlocked is returned when an uncompressed file read is attempted in restricted stages.
var ErrRawReadBlocked = errors.New("raw file read blocked by forge guard")

// DefaultMaxRawReadLines is the maximum number of lines allowed for cat/head/tail on source files.
const DefaultMaxRawReadLines = 100

// SourceExtensions contains common code file extensions subject to raw read guards.
var SourceExtensions = map[string]bool{
	".go":   true,
	".ts":   true,
	".tsx":  true,
	".js":   true,
	".jsx":  true,
	".py":   true,
	".rs":   true,
	".c":    true,
	".cpp":  true,
	".h":    true,
	".hpp":  true,
	".java": true,
	".kt":   true,
	".rb":   true,
	".php":  true,
	".cs":   true,
	".swift": true,
}

// CheckRawRead inspects a command invocation and blocks raw file reads
// on large source code files during BUILD, REVIEW, and SIMPLIFY stages.
func CheckRawRead(st state.Stage, cmdArgs []string, rootDir string) error {
	switch st {
	case state.StageBuild, state.StageReview, state.StageSimplify:
		// Active guard stages
	default:
		return nil
	}

	if len(cmdArgs) == 0 {
		return nil
	}

	prog := filepath.Base(cmdArgs[0])
	switch prog {
	case "cat", "head", "tail", "more", "less":
		// These commands read raw files directly.
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
			return fmt.Errorf("%w: [FORGE BLOCK] Leitura bruta bloqueada no estágio %s (%s tem %d linhas). Use as tools do Serena MCP (get_symbols_overview / find_symbol) ou 'rtk read -l aggressive %s'.",
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
}

// NewRunner creates a new Runner, auto-discovering RTK if present in PATH.
func NewRunner(rootDir string) *Runner {
	rtkPath, _ := exec.LookPath("rtk")
	return &Runner{
		RootDir: rootDir,
		RTKPath: rtkPath,
	}
}

// Run executes a command, capturing stdout and stderr.
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

	return &ExecResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
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
