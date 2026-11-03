package updater

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ARILSONLIMA/forge/internal/setup"
)

// DefaultRepository is the upstream GitHub repository for Forge.
const DefaultRepository = "ARILSONLIMA/forge"

// UpdateOptions contains options and flags for the update command.
type UpdateOptions struct {
	AutoApprove bool
	CheckOnly   bool
	Force       bool
	BinDir      string
	Repository  string
}

// CompareVersions compares two semver strings (e.g. "v0.0.2" and "0.0.3").
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	clean1 := strings.TrimPrefix(strings.TrimSpace(v1), "v")
	clean2 := strings.TrimPrefix(strings.TrimSpace(v2), "v")

	parts1 := strings.Split(clean1, ".")
	parts2 := strings.Split(clean2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}
	return 0
}

// IsSourceRepo checks if dir is the root of the Forge source repository.
func IsSourceRepo(dir string) bool {
	modPath := filepath.Join(dir, "go.mod")
	content, err := os.ReadFile(modPath)
	if err != nil {
		return false
	}
	if !strings.Contains(string(content), "module github.com/ARILSONLIMA/forge") {
		return false
	}

	mainPath := filepath.Join(dir, "cmd", "forge", "main.go")
	if _, err := os.Stat(mainPath); err != nil {
		return false
	}

	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	return err == nil && info.IsDir()
}

// FetchLatestGitHubTag queries GitHub API for the highest semver tag in the repository.
func FetchLatestGitHubTag(ctx context.Context, repo string) (string, error) {
	if repo == "" {
		repo = DefaultRepository
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/tags", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Forge-CLI-Updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "", errors.New("no tags found in repository")
	}

	bestTag := tags[0].Name
	for _, t := range tags[1:] {
		if CompareVersions(t.Name, bestTag) > 0 {
			bestTag = t.Name
		}
	}

	return bestTag, nil
}

// ReplaceExecutable replaces targetPath with sourcePath atomically using a temp file.
func ReplaceExecutable(sourcePath, targetPath string) error {
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", targetDir, err)
	}

	tmpFile, err := os.CreateTemp(targetDir, "forge-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", targetDir, err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	src, err := os.Open(sourcePath)
	if err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("opening source binary %s: %w", sourcePath, err)
	}
	defer src.Close()

	if _, err := io.Copy(tmpFile, src); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("copying binary data: %w", err)
	}

	if err := tmpFile.Chmod(0o755); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("replacing %s: %w", targetPath, err)
	}

	return nil
}

// RunUpdate handles checking and performing updates for Forge CLI.
func RunUpdate(ctx context.Context, currentVersion, rootDir string, opts UpdateOptions) error {
	if opts.Repository == "" {
		opts.Repository = DefaultRepository
	}
	if opts.BinDir == "" {
		opts.BinDir = setup.GetDefaultBinDir()
	}

	targetBin := filepath.Join(opts.BinDir, "forge")

	currExe, err := os.Executable()
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(currExe); err == nil {
			currExe = resolved
		}
	}

	if IsSourceRepo(rootDir) {
		return updateFromSource(ctx, currentVersion, rootDir, targetBin, currExe, opts)
	}

	return updateFromRemote(ctx, currentVersion, targetBin, currExe, opts)
}

func updateFromSource(ctx context.Context, currentVersion, rootDir, targetBin, currExe string, opts UpdateOptions) error {
	fmt.Println("🔍 Repositório de desenvolvimento do Forge detectado.")
	fmt.Printf("Versão atual: %s | Pasta: %s\n\n", currentVersion, rootDir)

	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("o utilitário 'git' não foi encontrado no PATH")
	}

	// 1. Check for uncommitted local changes
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = rootDir
	statusOut, _ := statusCmd.Output()
	if len(strings.TrimSpace(string(statusOut))) > 0 {
		fmt.Println("⚠️  Aviso: Existem modificações locais não commitadas no repositório:")
		lines := strings.Split(strings.TrimSpace(string(statusOut)), "\n")
		for i, l := range lines {
			if i >= 5 {
				fmt.Printf("   ... e mais %d arquivo(s)\n", len(lines)-5)
				break
			}
			fmt.Printf("   %s\n", l)
		}
		fmt.Println("Recomendamos salvar ou commitar suas alterações antes de atualizar.")
		fmt.Println()
	}

	// 2. Fetch updates
	fmt.Println("📡 Verificando atualizações no GitHub (git fetch origin)...")
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin")
	fetchCmd.Dir = rootDir
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("falha ao consultar atualizações remotas: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// 3. Detect current branch
	branchCmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = rootDir
	branchOut, _ := branchCmd.Output()
	branch := strings.TrimSpace(string(branchOut))
	if branch == "" || branch == "HEAD" {
		branch = "main"
	}

	upstream := "origin/" + branch

	// 4. Compare commits
	revLocalCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	revLocalCmd.Dir = rootDir
	localHead, _ := revLocalCmd.Output()

	revRemoteCmd := exec.CommandContext(ctx, "git", "rev-parse", upstream)
	revRemoteCmd.Dir = rootDir
	remoteHead, err := revRemoteCmd.Output()
	if err != nil {
		upstream = "origin/main"
		revRemoteCmd = exec.CommandContext(ctx, "git", "rev-parse", upstream)
		revRemoteCmd.Dir = rootDir
		remoteHead, _ = revRemoteCmd.Output()
	}

	locSHA := strings.TrimSpace(string(localHead))
	remSHA := strings.TrimSpace(string(remoteHead))

	hasNewCommits := (locSHA != remSHA && remSHA != "")
	var logLines []string
	if hasNewCommits {
		logCmd := exec.CommandContext(ctx, "git", "log", fmt.Sprintf("HEAD..%s", upstream), "--oneline", "-n", "10")
		logCmd.Dir = rootDir
		logOut, _ := logCmd.Output()
		for _, l := range strings.Split(strings.TrimSpace(string(logOut)), "\n") {
			if strings.TrimSpace(l) != "" {
				logLines = append(logLines, l)
			}
		}
	}

	if !hasNewCommits && !opts.Force {
		shortSHA := locSHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		fmt.Printf("✓ Você já está com a versão mais recente do Forge (%s, commit %s).\n", currentVersion, shortSHA)
		return nil
	}

	if hasNewCommits {
		fmt.Printf("✨ Novas atualizações encontradas em %s (%d novo(s) commit(s)):\n", upstream, len(logLines))
		for _, line := range logLines {
			fmt.Printf("  • %s\n", line)
		}
		fmt.Println()
	} else if opts.Force {
		fmt.Println("⚡ Flag --force ativada: recompilando Forge mesmo sem novos commits.")
	}

	if opts.CheckOnly {
		fmt.Println("Modo --check: Nenhuma alteração foi realizada.")
		return nil
	}

	// 5. Ask confirmation if not auto-approved
	if !opts.AutoApprove {
		fmt.Print("Deseja atualizar e recompilar o Forge agora? [Y/n]: ")
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "" && ans != "y" && ans != "yes" && ans != "s" && ans != "sim" {
			fmt.Println("Atualização cancelada.")
			return nil
		}
	}

	// 6. Pull updates
	if hasNewCommits {
		fmt.Println("\n⬇️  Puxando novidades do repositório (git pull)...")
		pullCmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
		pullCmd.Dir = rootDir
		if out, err := pullCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("falha ao executar git pull: %s (%w)", strings.TrimSpace(string(out)), err)
		}
	}

	// 7. Compile binary
	fmt.Println("🔨 Compilando o novo executável do Forge...")
	localBinary := filepath.Join(rootDir, "forge")
	buildCmd := exec.CommandContext(ctx, "go", "build", "-ldflags", "-s -w", "-o", localBinary, "./cmd/forge")
	buildCmd.Dir = rootDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("falha ao compilar com go build: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	fmt.Printf("✓ Binário compilado em: %s\n", localBinary)

	// 8. Update global install directory
	if targetBin != localBinary {
		if err := ReplaceExecutable(localBinary, targetBin); err != nil {
			fmt.Printf("⚠️  Aviso ao copiar para %s: %v\n", targetBin, err)
		} else {
			fmt.Printf("✓ Executável global atualizado em: %s\n", targetBin)
		}
	}

	// 9. If current running binary is elsewhere, update it too
	if currExe != "" && currExe != localBinary && currExe != targetBin {
		if err := ReplaceExecutable(localBinary, currExe); err == nil {
			fmt.Printf("✓ Executável em execução atualizado em: %s\n", currExe)
		}
	}

	fmt.Println("\n🎉 Forge atualizado com sucesso!")
	return nil
}

func updateFromRemote(ctx context.Context, currentVersion, targetBin, currExe string, opts UpdateOptions) error {
	fmt.Printf("🔍 Verificando versão remota do Forge (%s)...\n", opts.Repository)
	fmt.Printf("Versão instalada: %s\n\n", currentVersion)

	latestTag, err := FetchLatestGitHubTag(ctx, opts.Repository)
	if err != nil {
		return fmt.Errorf("não foi possível consultar versão mais recente: %w", err)
	}

	cmp := CompareVersions(currentVersion, latestTag)
	if cmp >= 0 && !opts.Force {
		fmt.Printf("✓ Você já está com a versão mais recente do Forge (%s).\n", currentVersion)
		return nil
	}

	fmt.Printf("🚀 Nova versão disponível: %s (versão atual: %s)\n\n", latestTag, currentVersion)

	if opts.CheckOnly {
		fmt.Println("Modo --check: Nenhuma alteração foi realizada.")
		return nil
	}

	// Prompt confirmation
	if !opts.AutoApprove {
		fmt.Printf("Deseja atualizar para %s agora? [Y/n]: ", latestTag)
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "" && ans != "y" && ans != "yes" && ans != "s" && ans != "sim" {
			fmt.Println("Atualização cancelada.")
			return nil
		}
	}

	// If go is available, use go install
	if _, err := exec.LookPath("go"); err == nil {
		fmt.Printf("⚙️  Instalando versão %s via 'go install'...\n", latestTag)
		installTarget := fmt.Sprintf("github.com/%s/cmd/forge@%s", opts.Repository, latestTag)
		cmd := exec.CommandContext(ctx, "go", "install", installTarget)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("falha ao executar go install: %s (%w)", strings.TrimSpace(string(out)), err)
		}

		home, _ := os.UserHomeDir()
		goBinPath := filepath.Join(home, "go", "bin", "forge")
		if gopath := os.Getenv("GOPATH"); gopath != "" {
			goBinPath = filepath.Join(gopath, "bin", "forge")
		}

		if _, err := os.Stat(goBinPath); err == nil {
			dest := targetBin
			if currExe != "" {
				dest = currExe
			}
			if dest != goBinPath {
				if err := ReplaceExecutable(goBinPath, dest); err != nil {
					fmt.Printf("⚠️  Aviso ao copiar para %s: %v\n", dest, err)
				} else {
					fmt.Printf("✓ Binário atualizado em: %s\n", dest)
				}
			}
		}
	} else {
		// Fallback: run install.sh via curl and bash
		fmt.Println("⚙️  Baixando e instalando nova versão via script oficial...")
		scriptURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/install.sh", opts.Repository, latestTag)
		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("curl -fsSL %s | bash -s -- --no-deps -y", scriptURL))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("falha ao rodar instalador remoto: %w", err)
		}
	}

	fmt.Printf("\n🎉 Forge atualizado com sucesso para %s!\n", latestTag)
	return nil
}
