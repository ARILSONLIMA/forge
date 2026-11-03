package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// BeadTask represents a task in the Beads DAG.
type BeadTask struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority,omitempty"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

// BeadClient defines methods to interact with the Beads issue tracker / task DAG.
type BeadClient interface {
	Ready(ctx context.Context) ([]BeadTask, error)
	Show(ctx context.Context, id string) (*BeadTask, error)
	Close(ctx context.Context, id string, reason string) error
}

// CmdExecutor executes an external CLI command.
type CmdExecutor func(ctx context.Context, dir string, name string, args ...string) ([]byte, error)

// DefaultCmdExecutor is the standard os/exec implementation.
func DefaultCmdExecutor(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}

// CLIBeadClient implements BeadClient using the `bd` binary.
type CLIBeadClient struct {
	Dir      string
	BDPath   string
	Executor CmdExecutor
}

// NewBeadClient returns a CLIBeadClient, finding `bd` in PATH.
func NewBeadClient(dir string) *CLIBeadClient {
	bdPath, _ := exec.LookPath("bd")
	return &CLIBeadClient{
		Dir:      dir,
		BDPath:   bdPath,
		Executor: DefaultCmdExecutor,
	}
}

// ErrBeadsNotInstalled is returned when `bd` binary cannot be found.
var ErrBeadsNotInstalled = errors.New("beads CLI (bd) not found in PATH")

func (c *CLIBeadClient) checkInstalled() error {
	if c.BDPath == "" {
		p, err := exec.LookPath("bd")
		if err != nil {
			return ErrBeadsNotInstalled
		}
		c.BDPath = p
	}
	return nil
}

// Ready lists unblocked, open tasks ready for work from `bd ready --json`.
func (c *CLIBeadClient) Ready(ctx context.Context) ([]BeadTask, error) {
	if err := c.checkInstalled(); err != nil {
		return nil, err
	}

	execFn := c.Executor
	if execFn == nil {
		execFn = DefaultCmdExecutor
	}

	out, err := execFn(ctx, c.Dir, c.BDPath, "ready", "--json")
	if err != nil {
		// Attempt fallback without --json if --json returned error
		outText, errText := execFn(ctx, c.Dir, c.BDPath, "ready")
		if errText != nil {
			return nil, fmt.Errorf("bd ready failed: %w: %s", err, string(out))
		}
		return parseBeadsTextOutput(string(outText)), nil
	}

	return parseBeadsReadyJSON(out)
}

func parseBeadsReadyJSON(data []byte) ([]BeadTask, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "[]" {
		return []BeadTask{}, nil
	}

	// Case 1: direct slice of BeadTask
	var tasks []BeadTask
	if err := json.Unmarshal([]byte(trimmed), &tasks); err == nil {
		return tasks, nil
	}

	// Case 2: wrapped in an object like {"tasks": [...]} or {"ready": [...]}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil {
		for _, key := range []string{"tasks", "ready", "items", "data"} {
			if raw, ok := wrapper[key]; ok {
				var subTasks []BeadTask
				if err := json.Unmarshal(raw, &subTasks); err == nil {
					return subTasks, nil
				}
			}
		}
	}

	// Case 3: fallback to text parsing if JSON shape differs
	parsed := parseBeadsTextOutput(trimmed)
	if len(parsed) > 0 {
		return parsed, nil
	}

	return nil, fmt.Errorf("unable to parse bd ready output: %s", trimmed)
}

func parseBeadsTextOutput(text string) []BeadTask {
	lines := strings.Split(text, "\n")
	var tasks []BeadTask
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) > 0 && strings.HasPrefix(parts[0], "bd-") {
			id := parts[0]
			title := strings.TrimSpace(strings.TrimPrefix(l, id))
			tasks = append(tasks, BeadTask{
				ID:     id,
				Title:  title,
				Status: "ready",
			})
		}
	}
	return tasks
}

// Show inspects a specific bead task.
func (c *CLIBeadClient) Show(ctx context.Context, id string) (*BeadTask, error) {
	if err := c.checkInstalled(); err != nil {
		return nil, err
	}

	execFn := c.Executor
	if execFn == nil {
		execFn = DefaultCmdExecutor
	}

	out, err := execFn(ctx, c.Dir, c.BDPath, "show", id, "--json")
	if err != nil {
		// Fallback to text mode
		outText, errText := execFn(ctx, c.Dir, c.BDPath, "show", id)
		if errText != nil {
			return nil, fmt.Errorf("bd show %s failed: %w: %s", id, err, string(out))
		}
		return parseBeadShowText(id, string(outText)), nil
	}

	var task BeadTask
	if err := json.Unmarshal(out, &task); err == nil && task.ID != "" {
		return &task, nil
	}

	return parseBeadShowText(id, string(out)), nil
}

func parseBeadShowText(id, text string) *BeadTask {
	task := &BeadTask{
		ID:     id,
		Status: "open",
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "closed") || strings.Contains(lower, "status: closed") || strings.Contains(lower, "[closed]") {
		task.Status = "closed"
	}
	lines := strings.Split(text, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(strings.ToLower(l), "title:") {
			task.Title = strings.TrimSpace(strings.TrimPrefix(l, "Title:"))
		}
	}
	return task
}

// Close marks a task closed in the Beads DAG.
func (c *CLIBeadClient) Close(ctx context.Context, id string, reason string) error {
	if err := c.checkInstalled(); err != nil {
		return err
	}

	execFn := c.Executor
	if execFn == nil {
		execFn = DefaultCmdExecutor
	}

	args := []string{"close", id}
	if reason != "" {
		args = append(args, "--reason", reason)
	}

	out, err := execFn(ctx, c.Dir, c.BDPath, args...)
	if err != nil {
		return fmt.Errorf("bd close %s failed: %w: %s", id, err, string(out))
	}
	return nil
}
