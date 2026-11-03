package runner

import (
	"context"
	"strings"
	"testing"
)

func TestBeadsReadyJSONSlice(t *testing.T) {
	mockJSON := `[
		{"id": "bd-1001", "title": "Implement auth handler", "status": "ready", "priority": 1},
		{"id": "bd-1002", "title": "Add test cases", "status": "ready", "priority": 2}
	]`

	client := &CLIBeadClient{
		BDPath: "/mock/bd",
		Executor: func(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "ready" {
				return []byte(mockJSON), nil
			}
			return nil, nil
		},
	}

	tasks, err := client.Ready(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "bd-1001" || tasks[0].Title != "Implement auth handler" {
		t.Errorf("unexpected task 0: %+v", tasks[0])
	}
	if tasks[1].ID != "bd-1002" {
		t.Errorf("unexpected task 1: %+v", tasks[1])
	}
}

func TestBeadsReadyJSONObjectWrapper(t *testing.T) {
	mockJSON := `{
		"tasks": [
			{"id": "bd-2001", "title": "Wire gates", "status": "ready"}
		]
	}`

	client := &CLIBeadClient{
		BDPath: "/mock/bd",
		Executor: func(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
			return []byte(mockJSON), nil
		},
	}

	tasks, err := client.Ready(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "bd-2001" {
		t.Fatalf("expected 1 task bd-2001, got %+v", tasks)
	}
}

func TestBeadsReadyTextFallback(t *testing.T) {
	mockText := `
bd-3001 Setup database schema
bd-3002 Configure migrations
`

	client := &CLIBeadClient{
		BDPath: "/mock/bd",
		Executor: func(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
			// First call with --json fails, triggers text fallback
			if len(args) >= 2 && args[1] == "--json" {
				return nil, context.DeadlineExceeded
			}
			return []byte(mockText), nil
		},
	}

	tasks, err := client.Ready(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "bd-3001" || !strings.Contains(tasks[0].Title, "Setup database schema") {
		t.Errorf("unexpected task 0: %+v", tasks[0])
	}
}

func TestBeadsShow(t *testing.T) {
	mockJSON := `{"id": "bd-1001", "status": "closed", "title": "Implement auth handler"}`

	client := &CLIBeadClient{
		BDPath: "/mock/bd",
		Executor: func(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
			return []byte(mockJSON), nil
		},
	}

	task, err := client.Show(context.Background(), "bd-1001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID != "bd-1001" || task.Status != "closed" {
		t.Fatalf("unexpected task: %+v", task)
	}
}

func TestBeadsClose(t *testing.T) {
	var capturedArgs []string
	client := &CLIBeadClient{
		BDPath: "/mock/bd",
		Executor: func(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
			capturedArgs = args
			return []byte("Task bd-1001 closed."), nil
		},
	}

	err := client.Close(context.Background(), "bd-1001", "completed all tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedArgs) < 4 || capturedArgs[0] != "close" || capturedArgs[1] != "bd-1001" || capturedArgs[3] != "completed all tests" {
		t.Fatalf("unexpected captured args: %v", capturedArgs)
	}
}
