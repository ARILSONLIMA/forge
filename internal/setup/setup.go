package setup

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ARILSONLIMA/forge/internal/config"
	"github.com/ARILSONLIMA/forge/internal/state"
)

// Dependency describes an ecosystem binary required or recommended by Forge.
type Dependency struct {
	Name        string   `json:"name"`
	Binaries    []string `json:"binaries"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	InstallTip  string   `json:"install_tip"`
}

// DependencyStatus contains the check result for a single dependency.
type DependencyStatus struct {
	Dep        Dependency `json:"dependency"`
	Installed  bool       `json:"installed"`
	BinaryUsed string     `json:"binary_used,omitempty"`
	Path       string     `json:"path,omitempty"`
	Version    string     `json:"version,omitempty"`
}

// AllDependencies returns the list of official tools required or recommended by Forge.
func AllDependencies() []Dependency {
	return []Dependency{
		{
			Name:        "git",
			Binaries:    []string{"git"},
			Required:    true,
			Description: "Version control system",
			InstallTip:  "install git via apt, dnf, pacman or brew",
		},
		{
			Name:        "go",
			Binaries:    []string{"go"},
			Required:    false,
			Description: "Go compiler and runtime",
			InstallTip:  "install go from https://go.dev/dl",
		},
		{
			Name:        "rtk",
			Binaries:    []string{"rtk"},
			Required:    true,
			Description: "Rust Token Killer proxy (80-90% terminal token compression)",
			InstallTip:  "run 'forge setup' or install from https://github.com/rtk-ai/rtk",
		},
		{
			Name:        "agy (Antigravity CLI)",
			Binaries:    []string{"agy"},
			Required:    true,
			Description: "Google Antigravity CLI agent harness",
			InstallTip:  "install Antigravity CLI (agy)",
		},
		{
			Name:        "sg / ast-grep",
			Binaries:    []string{"ast-grep", "sg"},
			Required:    true,
			Description: "AST structural search, linting and surgical refactoring",
			InstallTip:  "run 'forge setup' or 'npm i -g @ast-grep/cli' or 'cargo install ast-grep'",
		},
		{
			Name:        "bd (Beads DAG)",
			Binaries:    []string{"bd", "beads"},
			Required:    true,
			Description: "Beads DAG task tracker for persistent agent memory",
			InstallTip:  "run 'forge setup' or 'curl -fsSL https://raw.githubusercontent.com/gastownhall/beads/main/scripts/install.sh | bash'",
		},
		{
			Name:        "uvx / uv (Serena MCP)",
			Binaries:    []string{"uvx", "uv"},
			Required:    true,
			Description: "Python tool runner to launch Serena LSP MCP server",
			InstallTip:  "run 'forge setup' or 'curl -LsSf https://astral.sh/uv/install.sh | sh'",
		},
	}
}

// CheckDependencies checks all dependencies on the current system.
func CheckDependencies(ctx context.Context) []DependencyStatus {
	deps := AllDependencies()
	statuses := make([]DependencyStatus, len(deps))

	for i, dep := range deps {
		statuses[i] = CheckSingleDependency(ctx, dep)
	}
	return statuses
}

// CheckSingleDependency inspects a single dependency.
func CheckSingleDependency(ctx context.Context, dep Dependency) DependencyStatus {
	status := DependencyStatus{Dep: dep}

	for _, bin := range dep.Binaries {
		path, err := exec.LookPath(bin)
		if err == nil && path != "" {
			status.Installed = true
			status.BinaryUsed = bin
			status.Path = path
			status.Version = QueryVersion(ctx, bin)
			break
		}
	}

	return status
}

// QueryVersion attempts to extract a clean version string from a binary.
func QueryVersion(ctx context.Context, binaryName string) string {
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var args []string
	switch binaryName {
	case "go":
		args = []string{"version"}
	case "bd", "beads":
		args = []string{"version"}
	default:
		args = []string{"--version"}
	}

	cmd := exec.CommandContext(timeoutCtx, binaryName, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return ""
	}

	// First line only
	lines := strings.Split(raw, "\n")
	firstLine := strings.TrimSpace(lines[0])

	// Clean common prefixes
	switch binaryName {
	case "git":
		return strings.TrimPrefix(firstLine, "git version ")
	case "go":
		parts := strings.Fields(firstLine)
		if len(parts) >= 3 {
			return parts[2]
		}
	case "rtk":
		return strings.TrimPrefix(firstLine, "rtk ")
	case "ast-grep", "sg":
		return strings.TrimPrefix(firstLine, "ast-grep ")
	case "bd", "beads":
		// Format: "bd version 1.3.2 (...)" -> extract "1.3.2"
		parts := strings.Fields(firstLine)
		if len(parts) >= 3 && parts[1] == "version" {
			return parts[2]
		}
	case "uv", "uvx":
		// Format: "uv 0.13.7 (...)" -> extract "0.13.7"
		parts := strings.Fields(firstLine)
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	return firstLine
}

// FormatDoctorReport produces the diagnostic report as a string.
func FormatDoctorReport(statuses []DependencyStatus, rootDir string) string {
	var b strings.Builder

	b.WriteString("🏥 Running Forge Doctor Diagnostics...\n\n")
	b.WriteString(fmt.Sprintf("%-26s %-12s %-10s %s\n", "TOOL / DEPENDENCY", "STATUS", "VERSION", "LOCATION / TIP"))
	b.WriteString(fmt.Sprintf("%-26s %-12s %-10s %s\n", "-----------------", "------", "-------", "--------------"))

	allOk := true
	for _, s := range statuses {
		statusStr := " [✓] OK"
		verStr := s.Version
		if verStr == "" {
			verStr = "-"
		}
		locStr := s.Path

		if !s.Installed {
			verStr = "-"
			if s.Dep.Required {
				statusStr = " [!] MISSING"
				allOk = false
			} else {
				statusStr = " [-] OPTIONAL"
			}
			locStr = s.Dep.InstallTip
		}
		b.WriteString(fmt.Sprintf("%-26s %-12s %-10s %s\n", s.Dep.Name, statusStr, verStr, locStr))
	}

	b.WriteString("\nConfiguration & State Health:\n")
	cfgPath := filepath.Join(rootDir, config.DefaultPath)
	if _, err := os.Stat(cfgPath); err == nil {
		b.WriteString(fmt.Sprintf(" [✓] %s exists\n", config.DefaultPath))
	} else {
		b.WriteString(fmt.Sprintf(" [!] %s missing (run 'forge init')\n", config.DefaultPath))
	}

	stPath := filepath.Join(rootDir, state.DefaultPath)
	st, err := state.Load(stPath)
	if err == nil {
		if st.VerifyHash() {
			b.WriteString(fmt.Sprintf(" [✓] %s valid (Stage: %s, Hash: OK)\n", state.DefaultPath, st.CurrentStage))
		} else {
			b.WriteString(fmt.Sprintf(" [!] %s integrity hash mismatch!\n", state.DefaultPath))
		}
	} else {
		b.WriteString(fmt.Sprintf(" [!] %s missing (run 'forge init')\n", state.DefaultPath))
	}

	mcpPath := filepath.Join(rootDir, ".mcp.json")
	if _, err := os.Stat(mcpPath); err == nil {
		b.WriteString(fmt.Sprintf(" [✓] %s exists\n", ".mcp.json"))
	} else {
		b.WriteString(fmt.Sprintf(" [-] %s missing (run 'forge init')\n", ".mcp.json"))
	}

	if !allOk {
		b.WriteString("\n💡 Tip: Run 'forge setup' or './install.sh' to install missing dependencies automatically.\n")
	}

	return b.String()
}

// GetDefaultBinDir returns ~/.local/bin or /usr/local/bin.
func GetDefaultBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/usr/local/bin"
	}
	return filepath.Join(home, ".local", "bin")
}

// CustomRCDirectory, when non-empty, is used instead of user home directory for RC files (useful for tests).
var CustomRCDirectory string

// EnsureBinInPath checks if binDir is in PATH and adds it to shell RC files if missing.
func EnsureBinInPath(binDir string) (bool, []string, error) {
	currentPath := os.Getenv("PATH")
	entries := filepath.SplitList(currentPath)
	for _, entry := range entries {
		if filepath.Clean(entry) == filepath.Clean(binDir) {
			return false, nil, nil // Already in PATH
		}
	}

	baseDir := CustomRCDirectory
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, nil, err
		}
		baseDir = home
	}

	rcFiles := []string{
		filepath.Join(baseDir, ".bashrc"),
		filepath.Join(baseDir, ".zshrc"),
		filepath.Join(baseDir, ".profile"),
	}

	exportLine := fmt.Sprintf("\n# Forge and ecosystem tools\nexport PATH=\"%s:$PATH\"\n", binDir)
	var updatedFiles []string

	for _, rc := range rcFiles {
		if _, err := os.Stat(rc); err == nil {
			content, err := os.ReadFile(rc)
			if err == nil && !strings.Contains(string(content), binDir) {
				f, err := os.OpenFile(rc, os.O_APPEND|os.O_WRONLY, 0o644)
				if err == nil {
					_, _ = f.WriteString(exportLine)
					_ = f.Close()
					updatedFiles = append(updatedFiles, rc)
				}
			}
		}
	}

	return true, updatedFiles, nil
}

// InstallDependency downloads and installs a missing dependency.
func InstallDependency(ctx context.Context, depName string, binDir string) error {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("creating bin dir %s: %w", binDir, err)
	}

	switch strings.ToLower(depName) {
	case "rtk":
		return installRTK(ctx, binDir)
	case "bd", "beads":
		return installBeads(ctx, binDir)
	case "uv", "uvx":
		return installUV(ctx)
	case "ast-grep", "sg":
		return installAstGrep(ctx, binDir)
	default:
		return fmt.Errorf("automated installation for %q not supported. Please install manually.", depName)
	}
}

func installRTK(ctx context.Context, binDir string) error {
	var archiveName string
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "arm64" {
			archiveName = "rtk-aarch64-unknown-linux-gnu.tar.gz"
		} else {
			archiveName = "rtk-x86_64-unknown-linux-musl.tar.gz"
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			archiveName = "rtk-aarch64-apple-darwin.tar.gz"
		} else {
			archiveName = "rtk-x86_64-apple-darwin.tar.gz"
		}
	default:
		return fmt.Errorf("unsupported OS %s for automated RTK installation", runtime.GOOS)
	}

	url := fmt.Sprintf("https://github.com/rtk-ai/rtk/releases/latest/download/%s", archiveName)
	fmt.Printf("⬇️  Downloading RTK from %s...\n", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download RTK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading RTK returned HTTP %d", resp.StatusCode)
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("uncompressing gzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		if header.Typeflag == tar.TypeReg && (header.Name == "rtk" || filepath.Base(header.Name) == "rtk") {
			destPath := filepath.Join(binDir, "rtk")
			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return fmt.Errorf("creating destination file %s: %w", destPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close()
				return err
			}
			_ = outFile.Close()
			fmt.Printf("✓ Installed RTK to %s\n", destPath)
			return nil
		}
	}

	return errors.New("rtk binary not found inside archive")
}

func installBeads(ctx context.Context, binDir string) error {
	fmt.Println("⬇️  Running official Beads installer...")
	cmd := exec.CommandContext(ctx, "bash", "-c", "curl -fsSL https://raw.githubusercontent.com/gastownhall/beads/main/scripts/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("beads install script failed: %w", err)
	}
	return nil
}

func installUV(ctx context.Context) error {
	fmt.Println("⬇️  Running official astral.sh uv installer...")
	cmd := exec.CommandContext(ctx, "sh", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uv install script failed: %w", err)
	}
	return nil
}

func installAstGrep(ctx context.Context, binDir string) error {
	// Try npm first if available
	if npmPath, err := exec.LookPath("npm"); err == nil && npmPath != "" {
		fmt.Println("⬇️  Installing @ast-grep/cli via npm...")
		cmd := exec.CommandContext(ctx, "npm", "install", "-g", "@ast-grep/cli")
		if out, err := cmd.CombinedOutput(); err == nil {
			fmt.Println("✓ Installed @ast-grep/cli via npm")
			return nil
		} else {
			fmt.Printf("npm install failed (%v): %s. Falling back to direct binary download...\n", err, string(out))
		}
	}

	// Direct zip download fallback
	var zipName string
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "arm64" {
			zipName = "app-aarch64-unknown-linux-gnu.zip"
		} else {
			zipName = "app-x86_64-unknown-linux-gnu.zip"
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			zipName = "app-aarch64-apple-darwin.zip"
		} else {
			zipName = "app-x86_64-apple-darwin.zip"
		}
	default:
		return fmt.Errorf("unsupported OS %s for automated ast-grep installation", runtime.GOOS)
	}

	url := fmt.Sprintf("https://github.com/ast-grep/ast-grep/releases/latest/download/%s", zipName)
	fmt.Printf("⬇️  Downloading ast-grep from %s...\n", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading ast-grep: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading ast-grep returned HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}

	for _, file := range zipReader.File {
		if file.Name == "ast-grep" || filepath.Base(file.Name) == "ast-grep" {
			rc, err := file.Open()
			if err != nil {
				return err
			}
			destPath := filepath.Join(binDir, "ast-grep")
			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				_ = rc.Close()
				return err
			}
			_, err = io.Copy(outFile, rc)
			_ = rc.Close()
			_ = outFile.Close()
			if err != nil {
				return err
			}

			// Symlink 'sg' -> 'ast-grep'
			sgLink := filepath.Join(binDir, "sg")
			_ = os.Remove(sgLink)
			_ = os.Symlink(destPath, sgLink)

			fmt.Printf("✓ Installed ast-grep and sg to %s\n", binDir)
			return nil
		}
	}

	return errors.New("ast-grep binary not found in zip archive")
}

// RunSetup executes an interactive or automated environment setup and dependency installer.
// It returns true if the user requested 'forge init' to be executed.
func RunSetup(ctx context.Context, rootDir string, autoApprove bool, binDir string) (bool, error) {
	if binDir == "" {
		binDir = GetDefaultBinDir()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(" 🔨 FORGE ECOSYSTEM SETUP & CONFIGURATOR")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Target binary directory: %s\n\n", binDir)

	// 1. Ensure PATH configuration
	added, updatedFiles, err := EnsureBinInPath(binDir)
	if err != nil {
		fmt.Printf("⚠️  Warning checking PATH: %v\n", err)
	} else if added && len(updatedFiles) > 0 {
		fmt.Printf("✓ Added %s to PATH in: %s\n", binDir, strings.Join(updatedFiles, ", "))
		fmt.Printf("  (Tip: Run 'source %s' or open a new terminal for PATH to take effect)\n\n", updatedFiles[0])
	} else {
		fmt.Printf("✓ Target directory %s is configured in PATH.\n\n", binDir)
	}

	// 2. Check dependencies
	statuses := CheckDependencies(ctx)
	var missing []DependencyStatus
	for _, s := range statuses {
		if !s.Installed && s.Dep.Name != "git" && s.Dep.Name != "go" && s.Dep.Name != "agy (Antigravity CLI)" {
			// Tools that can be automatically installed (rtk, bd, uv, ast-grep)
			missing = append(missing, s)
		}
	}

	if len(missing) == 0 {
		fmt.Println("✨ All automated ecosystem tools are already installed!")
	} else {
		fmt.Printf("The following tools are missing and can be installed automatically:\n")
		for _, m := range missing {
			fmt.Printf("  • %-20s - %s\n", m.Dep.Name, m.Dep.Description)
		}
		fmt.Println()

		shouldInstall := autoApprove
		if !autoApprove {
			fmt.Print("Do you want to install all missing tools now? [Y/n]: ")
			reader := bufio.NewReader(os.Stdin)
			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(strings.ToLower(ans))
			shouldInstall = (ans == "" || ans == "y" || ans == "yes" || ans == "s" || ans == "sim")
		}

		if shouldInstall {
			for _, m := range missing {
				fmt.Printf("\n⚙️  Installing %s...\n", m.Dep.Name)
				primaryBin := m.Dep.Binaries[0]
				if err := InstallDependency(ctx, primaryBin, binDir); err != nil {
					fmt.Printf("❌ Failed to install %s: %v\n", m.Dep.Name, err)
				} else {
					fmt.Printf("✓ %s installed successfully!\n", m.Dep.Name)
				}
			}
		} else {
			fmt.Println("Skipping automatic installation of tools.")
		}
	}

	// 3. Check project configuration
	shouldInit := false
	cfgPath := filepath.Join(rootDir, config.DefaultPath)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		fmt.Println()
		if autoApprove {
			shouldInit = true
		} else {
			fmt.Print("Forge is not yet initialized in this directory. Run 'forge init' now? [Y/n]: ")
			reader := bufio.NewReader(os.Stdin)
			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(strings.ToLower(ans))
			shouldInit = (ans == "" || ans == "y" || ans == "yes" || ans == "s" || ans == "sim")
		}
	}

	// 4. Print updated diagnostics
	fmt.Println()
	updatedStatuses := CheckDependencies(ctx)
	fmt.Print(FormatDoctorReport(updatedStatuses, rootDir))

	return shouldInit, nil
}
