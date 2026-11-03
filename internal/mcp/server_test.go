package mcp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ARILSONLIMA/forge/internal/state"
)

func TestMCPServer(t *testing.T) {
	tmp := t.TempDir()

	// Initialize state
	st := state.NewInitialState()
	st.CurrentStage = state.StageSpec
	_ = state.Save(filepath.Join(tmp, state.DefaultPath), st)

	// Build JSON-RPC messages
	var in bytes.Buffer
	var out bytes.Buffer

	// 1. Initialize
	in.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	// 2. tools/list
	in.WriteString(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	// 3. tools/call forge_status
	in.WriteString(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"forge_status","arguments":{}}}` + "\n")

	srv := NewServer(tmp, &in, &out)
	if err := srv.Serve(); err != nil {
		t.Fatalf("serve error: %v", err)
	}

	responses := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d: %v", len(responses), responses)
	}

	// Verify initialize response
	var initResp Response
	if err := json.Unmarshal([]byte(responses[0]), &initResp); err != nil {
		t.Fatalf("parse init resp: %v", err)
	}
	if initResp.ID != float64(1) {
		t.Errorf("expected id 1, got %v", initResp.ID)
	}

	// Verify tools list response
	var listResp Response
	if err := json.Unmarshal([]byte(responses[1]), &listResp); err != nil {
		t.Fatalf("parse tools/list resp: %v", err)
	}

	// Verify tools call response
	var callResp Response
	if err := json.Unmarshal([]byte(responses[2]), &callResp); err != nil {
		t.Fatalf("parse tools/call resp: %v", err)
	}
	if callResp.Error != nil {
		t.Errorf("call returned error: %+v", callResp.Error)
	}
}
