package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Category represents a specific savings or efficiency dimension.
type Category struct {
	Events      int    `json:"events"`
	TokensSaved int    `json:"tokens_saved"`
	Label       string `json:"label"`
}

// Data holds the persistent metrics state.
type Data struct {
	UpdatedAt  string              `json:"updated_at"`
	Categories map[string]*Category `json:"categories"`
}

// Tracker records and aggregates token savings and efficiency stats.
type Tracker struct {
	rootDir  string
	filePath string
	mu       sync.Mutex
}

// NewTracker creates a Tracker for the given project root directory.
func NewTracker(rootDir string) *Tracker {
	return &Tracker{
		rootDir:  rootDir,
		filePath: filepath.Join(rootDir, ".forge", "logs", "token-savings.json"),
	}
}

func defaultCategories() map[string]*Category {
	return map[string]*Category{
		"rtk": {
			Events:      0,
			TokensSaved: 0,
			Label:       "RTK Output Compression",
		},
		"raw_read_blocks": {
			Events:      0,
			TokensSaved: 0,
			Label:       "I/O Guard Raw-Read Blocks",
		},
		"headroom_truncator": {
			Events:      0,
			TokensSaved: 0,
			Label:       "Headroom Output Truncator",
		},
		"session_briefing": {
			Events:      0,
			TokensSaved: 0,
			Label:       "Zero-Read Session Briefing",
		},
		"stage_swapping": {
			Events:      0,
			TokensSaved: 0,
			Label:       "Dynamic Stage Context Swapping (<250 tok)",
		},
		"secret_shield": {
			Events:      0,
			TokensSaved: 0,
			Label:       "Secret Shield & Env Guard",
		},
	}
}

// Load loads existing metrics or returns fresh structure.
func (t *Tracker) Load() (*Data, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := os.ReadFile(t.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Data{
				UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
				Categories: defaultCategories(),
			}, nil
		}
		return nil, err
	}

	var d Data
	if err := json.Unmarshal(data, &d); err != nil {
		return &Data{
			UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
			Categories: defaultCategories(),
		}, nil
	}

	// Ensure all standard categories exist
	if d.Categories == nil {
		d.Categories = defaultCategories()
	} else {
		for k, v := range defaultCategories() {
			if _, exists := d.Categories[k]; !exists {
				d.Categories[k] = v
			}
		}
	}

	return &d, nil
}

// RecordSaving increments the savings event and token count for category.
func (t *Tracker) RecordSaving(category string, tokens int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	d := &Data{
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
		Categories: defaultCategories(),
	}

	if raw, err := os.ReadFile(t.filePath); err == nil {
		_ = json.Unmarshal(raw, d)
		if d.Categories == nil {
			d.Categories = defaultCategories()
		}
	}

	cat, ok := d.Categories[category]
	if !ok {
		cat = &Category{Label: category}
		d.Categories[category] = cat
	}

	cat.Events++
	cat.TokensSaved += tokens
	d.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(filepath.Dir(t.filePath), 0o755); err != nil {
		return err
	}

	tmpFile := fmt.Sprintf("%s.%d.tmp", t.filePath, time.Now().UnixNano())
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpFile, b, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpFile, t.filePath)
}

// TotalTokens returns total tokens saved across all categories.
func (d *Data) TotalTokens() int {
	total := 0
	for _, c := range d.Categories {
		total += c.TokensSaved
	}
	return total
}

// SummaryString returns a human-readable summary of token economy.
func (d *Data) SummaryString() string {
	res := fmt.Sprintf("📊 Forge Token Economy (Total Saved: ~%d tokens)\n", d.TotalTokens())
	res += "---------------------------------------------------------\n"
	for _, k := range []string{"stage_swapping", "rtk", "headroom_truncator", "raw_read_blocks", "session_briefing", "secret_shield"} {
		if c, ok := d.Categories[k]; ok && (c.Events > 0 || c.TokensSaved > 0) {
			res += fmt.Sprintf("  • %-42s : %4d events | ~%6d tok saved\n", c.Label, c.Events, c.TokensSaved)
		}
	}
	return res
}
