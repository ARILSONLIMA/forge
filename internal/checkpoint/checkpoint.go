package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Checkpoint represents a git micro-snapshot point in time.
type Checkpoint struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Timestamp  string `json:"timestamp"`
	CommitHash string `json:"commit_hash"`
	GitStatus  string `json:"git_status"`
}

var tagSanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// DefaultStorePath returns the path to .forge/checkpoints.json.
func DefaultStorePath(rootDir string) string {
	return filepath.Join(rootDir, ".forge", "checkpoints.json")
}

// Manager manages git checkpoints.
type Manager struct {
	RootDir   string
	StorePath string
}

// NewManager creates a Checkpoint Manager for rootDir.
func NewManager(rootDir string) *Manager {
	return &Manager{
		RootDir:   rootDir,
		StorePath: DefaultStorePath(rootDir),
	}
}

// Create creates a lightweight git tag and records the checkpoint.
func (m *Manager) Create(label string) (*Checkpoint, error) {
	if label == "" {
		label = "snapshot"
	}
	cleanLabel := tagSanitizeRegex.ReplaceAllString(label, "-")
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	tag := fmt.Sprintf("forge-chk-%s-%d", cleanLabel, ts)

	// 1. Get HEAD commit
	cmdHead := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmdHead.Dir = m.RootDir
	headOut, err := cmdHead.Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	head := strings.TrimSpace(string(headOut))

	// 2. Get git status
	cmdStatus := exec.Command("git", "status", "--porcelain")
	cmdStatus.Dir = m.RootDir
	statusOut, _ := cmdStatus.Output()
	status := strings.TrimSpace(string(statusOut))

	// 3. Create git tag
	cmdTag := exec.Command("git", "tag", tag, "HEAD")
	cmdTag.Dir = m.RootDir
	if err := cmdTag.Run(); err != nil {
		return nil, fmt.Errorf("create git tag %s: %w", tag, err)
	}

	chk := Checkpoint{
		ID:         tag,
		Label:      label,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		CommitHash: head,
		GitStatus:  status,
	}

	// 4. Save to store
	if err := m.save(chk); err != nil {
		return nil, err
	}

	return &chk, nil
}

// List returns all recorded checkpoints (most recent first).
func (m *Manager) List() ([]Checkpoint, error) {
	data, err := os.ReadFile(m.StorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var list []Checkpoint
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return list, nil
}

// Rollback restores working tree and HEAD to the specified checkpoint ID or latest checkpoint if ID is empty.
func (m *Manager) Rollback(id string) (*Checkpoint, error) {
	list, err := m.List()
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no checkpoints found to rollback")
	}

	var target *Checkpoint
	if id == "" || id == "latest" {
		target = &list[len(list)-1]
	} else {
		for i := range list {
			if list[i].ID == id || list[i].Label == id {
				target = &list[i]
				break
			}
		}
	}

	if target == nil {
		return nil, fmt.Errorf("checkpoint %q not found", id)
	}

	// Git checkout / reset
	cmdReset := exec.Command("git", "reset", "--hard", target.ID)
	cmdReset.Dir = m.RootDir
	if out, err := cmdReset.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git reset to %s: %w (%s)", target.ID, err, strings.TrimSpace(string(out)))
	}

	return target, nil
}

func (m *Manager) save(chk Checkpoint) error {
	list, _ := m.List()
	list = append(list, chk)

	if err := os.MkdirAll(filepath.Dir(m.StorePath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	tmp := fmt.Sprintf("%s.%d.tmp", m.StorePath, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.StorePath)
}
