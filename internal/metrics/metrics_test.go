package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetricsTracker(t *testing.T) {
	tmp := t.TempDir()
	tracker := NewTracker(tmp)

	data, err := tracker.Load()
	if err != nil {
		t.Fatalf("unexpected load err: %v", err)
	}
	if data.TotalTokens() != 0 {
		t.Errorf("expected 0 tokens initially, got %d", data.TotalTokens())
	}

	if err := tracker.RecordSaving("headroom_truncator", 500); err != nil {
		t.Fatalf("record saving error: %v", err)
	}
	if err := tracker.RecordSaving("headroom_truncator", 200); err != nil {
		t.Fatalf("record saving error: %v", err)
	}
	if err := tracker.RecordSaving("raw_read_blocks", 1200); err != nil {
		t.Fatalf("record saving error: %v", err)
	}

	updated, err := tracker.Load()
	if err != nil {
		t.Fatalf("load after record error: %v", err)
	}

	if updated.TotalTokens() != 1900 {
		t.Errorf("expected total 1900 tokens, got %d", updated.TotalTokens())
	}

	cat := updated.Categories["headroom_truncator"]
	if cat.Events != 2 || cat.TokensSaved != 700 {
		t.Errorf("unexpected category state: %+v", cat)
	}

	summary := updated.SummaryString()
	if !strings.Contains(summary, "Headroom Output Truncator") || !strings.Contains(summary, "1900") {
		t.Errorf("unexpected summary string: %s", summary)
	}

	// Verify file on disk
	if _, err := os.Stat(filepath.Join(tmp, ".forge", "logs", "token-savings.json")); err != nil {
		t.Errorf("token-savings.json file does not exist: %v", err)
	}
}
