package envskeleton

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvSkeleton(t *testing.T) {
	tmp := t.TempDir()

	envExample := `
PORT=8080
DATABASE_URL=postgres://localhost:5432/db
API_KEY=your_api_key_here
REDIS_HOST=
`
	envActual := `
PORT=3000
DATABASE_URL=postgres://prod:secret@db.internal:5432/prod
API_KEY=changeme
EXTRA_VAR=something
`

	if err := os.WriteFile(filepath.Join(tmp, ".env.example"), []byte(envExample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte(envActual), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Inspect(tmp)
	if err != nil {
		t.Fatalf("inspect err: %v", err)
	}

	if rep.Healthy {
		t.Errorf("expected unhealthy due to missing REDIS_HOST and placeholder API_KEY")
	}

	if len(rep.MissingKeys) != 1 || rep.MissingKeys[0] != "REDIS_HOST" {
		t.Errorf("expected REDIS_HOST missing, got: %v", rep.MissingKeys)
	}

	if len(rep.PlaceholderKeys) != 1 || rep.PlaceholderKeys[0] != "API_KEY" {
		t.Errorf("expected API_KEY as placeholder, got: %v", rep.PlaceholderKeys)
	}

	summary := rep.FormatSummary()
	if !strings.Contains(summary, "REDIS_HOST") || !strings.Contains(summary, "API_KEY") {
		t.Errorf("unexpected summary string: %s", summary)
	}

	// Verify that secret value "secret@db.internal" is NEVER present anywhere in summary or keys!
	if strings.Contains(summary, "secret@db.internal") {
		t.Fatalf("CRITICAL: Secret leaked into summary!")
	}
	for _, k := range rep.Keys {
		if strings.Contains(k.Key, "secret@db.internal") {
			t.Fatalf("CRITICAL: Secret leaked into key struct!")
		}
	}
}
