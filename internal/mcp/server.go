package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ARILSONLIMA/forge/internal/briefing"
	"github.com/ARILSONLIMA/forge/internal/checkpoint"
	"github.com/ARILSONLIMA/forge/internal/debt"
	"github.com/ARILSONLIMA/forge/internal/envskeleton"
	"github.com/ARILSONLIMA/forge/internal/gates"
	"github.com/ARILSONLIMA/forge/internal/shrink"
	"github.com/ARILSONLIMA/forge/internal/state"
	"github.com/ARILSONLIMA/forge/internal/symbols"
	"github.com/ARILSONLIMA/forge/internal/testisolate"
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool represents a tool definition in MCP.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// Server handles JSON-RPC MCP requests over stdio.
type Server struct {
	RootDir string
	In      io.Reader
	Out     io.Writer
}

// NewServer creates a new MCP Server for rootDir.
func NewServer(rootDir string, in io.Reader, out io.Writer) *Server {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return &Server{
		RootDir: rootDir,
		In:      in,
		Out:     out,
	}
}

// Serve starts the server loop until EOF.
func (s *Server) Serve() error {
	scanner := bufio.NewScanner(s.In)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		s.handleRequest(&req)
	}
	return scanner.Err()
}

func (s *Server) handleRequest(req *Request) {
	switch req.Method {
	case "initialize":
		s.sendResult(req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]string{
				"name":    "forge-mcp",
				"version": "0.1.0",
			},
		})

	case "notifications/initialized":
		// No-op notification

	case "tools/list":
		s.sendResult(req.ID, map[string]interface{}{
			"tools": s.toolDefinitions(),
		})

	case "tools/call":
		var callParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			s.sendError(req.ID, -32602, "Invalid params")
			return
		}
		resText, err := s.executeTool(callParams.Name, callParams.Arguments)
		if err != nil {
			s.sendResult(req.ID, map[string]interface{}{
				"content": []map[string]string{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			})
			return
		}
		s.sendResult(req.ID, map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": resText},
			},
		})

	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *Server) executeTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "forge_status":
		b, err := briefing.Generate(s.RootDir, false)
		if err != nil {
			return "", err
		}
		st, _ := state.Load(filepath.Join(s.RootDir, state.DefaultPath))
		if st == nil {
			return b, nil
		}
		return fmt.Sprintf("%s\nIntegridade: %s\nGates: %+v", b, st.IntegrityHash, st.GatesPassed), nil

	case "forge_gate":
		st, err := state.Load(filepath.Join(s.RootDir, state.DefaultPath))
		if err != nil {
			return "", err
		}
		mgr := gates.NewManager(s.RootDir, nil, st, nil, nil)
		res, err := mgr.ValidateCurrentGate(context.Background())
		if err != nil {
			return fmt.Sprintf("Gate %d REPROVADO: %v", res.GateNumber, err), nil
		}
		return fmt.Sprintf("Gate %d APROVADO: %s", res.GateNumber, res.Details), nil

	case "forge_map":
		return symbols.GenerateCodebaseMap(symbols.MapOptions{
			RootDir:     s.RootDir,
			BudgetChars: 2000,
		})

	case "forge_checkpoint":
		label, _ := args["label"].(string)
		if label == "" {
			label = "manual-checkpoint"
		}
		chk, err := checkpoint.NewManager(s.RootDir).Create(label)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Checkpoint criado: %s (HEAD: %s)", chk.ID, chk.CommitHash), nil

	case "forge_rollback":
		id, _ := args["id"].(string)
		target, err := checkpoint.NewManager(s.RootDir).Rollback(id)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Rollback concluído com sucesso para: %s (%s)", target.ID, target.CommitHash), nil

	case "forge_debt":
		items, err := debt.Scan(s.RootDir, nil)
		if err != nil {
			return "", err
		}
		return debt.FormatSummary(items), nil

	case "forge_test_isolate":
		rep, err := testisolate.LoadReport(s.RootDir)
		if err != nil {
			return "", err
		}
		if rep == nil || len(rep.FailedTests) == 0 {
			return "Zero testes falhos registrados.", nil
		}
		return fmt.Sprintf("Testes falhos: %s\nComando isolado sugerido: %s", strings.Join(rep.FailedTests, ", "), rep.IsolatedCommand), nil

	case "forge_env":
		rep, err := envskeleton.Inspect(s.RootDir)
		if err != nil {
			return "", err
		}
		return rep.FormatSummary(), nil

	case "forge_shrink":
		file, _ := args["file"].(string)
		if file == "" {
			return "", fmt.Errorf("argumento 'file' é obrigatório")
		}
		res, err := shrink.ShrinkFile(s.RootDir, file)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Arquivo compactado com sucesso! Redução: %d -> %d linhas. Backup em: %s", res.OriginalLines, res.ShrunkLines, res.BackupPath), nil

	default:
		return "", fmt.Errorf("ferramenta desconhecida %s", name)
	}
}

func (s *Server) toolDefinitions() []Tool {
	return []Tool{
		{
			Name:        "forge_status",
			Description: "Obtém o briefing condensado, estágio atual e gates aprovados do Forge.",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "forge_gate",
			Description: "Valida deterministicamente o Quality Gate da fase ativa atual.",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "forge_map",
			Description: "Retorna o mapa condensado de símbolos públicos exportados do projeto (<450 tokens).",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "forge_checkpoint",
			Description: "Cria um micro-checkpoint leve no Git antes de operações perigosas.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"label": map[string]interface{}{"type": "string", "description": "Rótulo do checkpoint"},
				},
			},
		},
		{
			Name:        "forge_rollback",
			Description: "Restaura cirurgicamente a working tree para um checkpoint anterior.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string", "description": "ID ou tag do checkpoint"},
				},
			},
		},
		{
			Name:        "forge_debt",
			Description: "Varre e audita atalhos (// SHORTCUT:) e débitos técnicos no código.",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "forge_test_isolate",
			Description: "Inspeciona testes falhos pendentes e gera comando de reexecução cirúrgica.",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "forge_env",
			Description: "Analisa esqueleto de ambiente (.env vs .env.example) sem vazar segredos.",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "forge_shrink",
			Description: "Compacta arquivo markdown ou especificação removendo cerimônias de texto.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file": map[string]interface{}{"type": "string", "description": "Caminho do arquivo markdown"},
				},
				"required": []string{"file"},
			},
		},
	}
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	b, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(s.Out, "%s\n", string(b))
}

func (s *Server) sendError(id interface{}, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	b, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(s.Out, "%s\n", string(b))
}
