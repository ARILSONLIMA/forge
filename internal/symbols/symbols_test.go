package symbols

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodebaseMap(t *testing.T) {
	tmp := t.TempDir()

	// 1. Create a Go file
	goCode := `package main

type ServerConfig struct{}

func StartServer() {}
func unexportedHelper() {}
`
	if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte(goCode), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Create a TypeScript file
	tsCode := `export interface User { id: string; }
export function loginUser() {}
function internalHelper() {}
`
	if err := os.WriteFile(filepath.Join(tmp, "auth.ts"), []byte(tsCode), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Create a Python file
	pyCode := `class AuthService:
    pass

def authenticate():
    pass

def _private_func():
    pass
`
	if err := os.WriteFile(filepath.Join(tmp, "service.py"), []byte(pyCode), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GenerateCodebaseMap(MapOptions{
		RootDir:     tmp,
		BudgetChars: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(res, "ServerConfig") || !strings.Contains(res, "StartServer") {
		t.Errorf("expected Go symbols in output: %s", res)
	}
	if strings.Contains(res, "unexportedHelper") {
		t.Errorf("expected unexported Go function to be excluded: %s", res)
	}
	if !strings.Contains(res, "User") || !strings.Contains(res, "loginUser") {
		t.Errorf("expected TS symbols in output: %s", res)
	}
	if strings.Contains(res, "internalHelper") {
		t.Errorf("expected internal TS helper to be excluded: %s", res)
	}
	if !strings.Contains(res, "AuthService") || !strings.Contains(res, "authenticate") {
		t.Errorf("expected Python symbols in output: %s", res)
	}
	if strings.Contains(res, "_private_func") {
		t.Errorf("expected private Python function to be excluded: %s", res)
	}
}
