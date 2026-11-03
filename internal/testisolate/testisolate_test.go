package testisolate

import (
	"strings"
	"testing"
)

func TestParseFailuresGo(t *testing.T) {
	output := `
=== RUN   TestLogin
--- PASS: TestLogin (0.00s)
=== RUN   TestRegister
--- FAIL: TestRegister (0.01s)
=== RUN   TestResetPassword
--- FAIL: TestResetPassword (0.02s)
FAIL
exit status 1
FAIL	github.com/example/auth	0.035s
`

	failures := ParseFailures(output)
	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d: %v", len(failures), failures)
	}
	if failures[0] != "TestRegister" || failures[1] != "TestResetPassword" {
		t.Fatalf("unexpected failures parsed: %v", failures)
	}

	cmd := BuildIsolatedCommand("go", failures)
	if !strings.Contains(cmd, "TestRegister|TestResetPassword") {
		t.Errorf("unexpected isolated command: %s", cmd)
	}
}

func TestParseFailuresPytest(t *testing.T) {
	output := `
tests/test_api.py .F.                                                    [ 75%]
FAILED tests/test_api.py::test_create_user - AssertionError: 400 != 201
FAILED tests/test_api.py::test_delete_user - AssertionError: not found
`
	failures := ParseFailures(output)
	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d: %v", len(failures), failures)
	}
	if failures[0] != "test_create_user" || failures[1] != "test_delete_user" {
		t.Fatalf("unexpected failures: %v", failures)
	}

	cmd := BuildIsolatedCommand("python", failures)
	if !strings.Contains(cmd, "test_create_user or test_delete_user") {
		t.Errorf("unexpected pytest command: %s", cmd)
	}
}

func TestSaveAndLoadReport(t *testing.T) {
	tmp := t.TempDir()
	out := `--- FAIL: TestAuthToken (0.01s)`

	rep, err := SaveReport(tmp, "go", "go test ./...", out)
	if err != nil {
		t.Fatalf("save report err: %v", err)
	}
	if rep == nil || len(rep.FailedTests) != 1 {
		t.Fatalf("expected saved report with 1 failure, got %+v", rep)
	}

	loaded, err := LoadReport(tmp)
	if err != nil {
		t.Fatalf("load report err: %v", err)
	}
	if loaded == nil || loaded.FailedTests[0] != "TestAuthToken" {
		t.Fatalf("expected loaded report to match: %+v", loaded)
	}

	ClearReport(tmp)
	cleared, _ := LoadReport(tmp)
	if cleared != nil {
		t.Errorf("expected report to be nil after clear, got %+v", cleared)
	}
}
