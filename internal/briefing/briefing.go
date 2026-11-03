package briefing

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ARILSONLIMA/forge/internal/metrics"
	"github.com/ARILSONLIMA/forge/internal/state"
	"github.com/ARILSONLIMA/forge/internal/testisolate"
)

// Generate creates a compact, zero-read briefing string for the current session.
func Generate(rootDir string, recordMetric bool) (string, error) {
	st, err := state.Load(filepath.Join(rootDir, state.DefaultPath))
	if err != nil {
		return "[FORGE BRIEFING] Projeto não inicializado com Forge (execute 'forge init').", nil
	}

	// 1. Git branch and dirty count
	branch := "desconhecida"
	cmdBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmdBranch.Dir = rootDir
	if out, err := cmdBranch.Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}

	dirtyCount := 0
	cmdStatus := exec.Command("git", "status", "--porcelain")
	cmdStatus.Dir = rootDir
	if out, err := cmdStatus.Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				dirtyCount++
			}
		}
	}

	// 2. Bead & Spec
	beadStr := st.ActiveBeadID
	if beadStr == "" {
		beadStr = "nenhuma"
	}
	specStr := st.ActiveSpec
	if specStr == "" {
		specStr = "nenhuma"
	}

	// 3. Current gate status
	gatePassed := st.GatesPassed[string(st.CurrentStage)]
	gateStr := "PENDENTE"
	if gatePassed {
		gateStr = "APROVADO"
	}

	// 4. Test failure status
	testStr := "0 falhas pendentes"
	if rep, _ := testisolate.LoadReport(rootDir); rep != nil && len(rep.FailedTests) > 0 {
		testStr = fmt.Sprintf("⚠️ %d falha(s) pendente(s) [%s]", len(rep.FailedTests), strings.Join(rep.FailedTests, ", "))
	}

	brief := fmt.Sprintf("[FORGE BRIEFING] Estágio: %s | Tarefa: %s | Spec: %s | Portão: %s | Git: %s (%d mod) | Testes: %s",
		st.CurrentStage, beadStr, specStr, gateStr, branch, dirtyCount, testStr)

	if recordMetric {
		_ = metrics.NewTracker(rootDir).RecordSaving("session_briefing", 1500)
	}

	return brief, nil
}
