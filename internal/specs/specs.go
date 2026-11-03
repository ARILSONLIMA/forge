package specs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DefaultSpecsDir is the standard path to specs directory.
const DefaultSpecsDir = ".forge/specs"

// DefaultRoadmapPath is the standard path to the system roadmap.
const DefaultRoadmapPath = ".forge/roadmap.md"

// DefaultGlossaryPath is the standard path to the DDD ubiquitous language glossary.
const DefaultGlossaryPath = ".forge/GLOSSARY.md"

// DefaultADRDir is the standard path to Architecture Decision Records.
const DefaultADRDir = ".forge/adr"

// SpecInfo represents metadata about a feature specification.
type SpecInfo struct {
	Number    int    `json:"number"`
	PaddedNum string `json:"padded_num"`
	Slug      string `json:"slug"`
	Filename  string `json:"filename"`
	RelPath   string `json:"rel_path"`
	FullPath  string `json:"full_path"`
	Title     string `json:"title"`
	Goal      string `json:"goal"`
	IsActive  bool   `json:"is_active"`
	Exists    bool   `json:"exists"`
}

// RoadmapItem represents an entry in .forge/roadmap.md.
type RoadmapItem struct {
	Number    int    `json:"number"`
	PaddedNum string `json:"padded_num"`
	Title     string `json:"title"`
	Goal      string `json:"goal"`
	Done      bool   `json:"done"`
}

// unaccent converts common accented Latin characters to basic ASCII.
func unaccent(r rune) rune {
	switch r {
	case 'á', 'à', 'ã', 'â', 'ä', 'å':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'õ', 'ô', 'ö':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	case 'ñ':
		return 'n'
	default:
		return r
	}
}

// Slugify converts a string into a clean URL-friendly slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var buf strings.Builder
	lastHyphen := false

	for _, r := range s {
		r = unaccent(r)
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			buf.WriteRune(r)
			lastHyphen = false
		case r == ' ' || r == '-' || r == '_' || r == '/':
			if !lastHyphen && buf.Len() > 0 {
				buf.WriteRune('-')
				lastHyphen = true
			}
		}
	}

	res := strings.Trim(buf.String(), "-")
	if res == "" {
		return "spec"
	}
	return res
}

// ParseRoadmap parses .forge/roadmap.md into structured items.
func ParseRoadmap(roadmapPath string) ([]RoadmapItem, error) {
	file, err := os.Open(roadmapPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var items []RoadmapItem
	scanner := bufio.NewScanner(file)

	// Regex patterns to match:
	// - [ ] 01 - Title: Goal
	// - [x] 1 - Title
	// 1. Title: Goal
	reCheckbox := regexp.MustCompile(`^[\s*-]*\[([ xX])\]\s*(\d+)[\s.:-]+([^:\n]+)(?::\s*(.+))?$`)
	reNumbered := regexp.MustCompile(`^\s*(\d+)[\s.:-]+([^:\n]+)(?::\s*(.+))?$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "<!--") {
			continue
		}

		if m := reCheckbox.FindStringSubmatch(line); m != nil {
			num, _ := strconv.Atoi(m[2])
			done := strings.ToLower(m[1]) == "x"
			title := strings.TrimSpace(m[3])
			goal := ""
			if len(m) > 4 {
				goal = strings.TrimSpace(m[4])
			}
			items = append(items, RoadmapItem{
				Number:    num,
				PaddedNum: fmt.Sprintf("%02d", num),
				Title:     title,
				Goal:      goal,
				Done:      done,
			})
			continue
		}

		if m := reNumbered.FindStringSubmatch(line); m != nil {
			num, _ := strconv.Atoi(m[1])
			title := strings.TrimSpace(m[2])
			goal := ""
			if len(m) > 3 {
				goal = strings.TrimSpace(m[3])
			}
			items = append(items, RoadmapItem{
				Number:    num,
				PaddedNum: fmt.Sprintf("%02d", num),
				Title:     title,
				Goal:      goal,
				Done:      false,
			})
		}
	}

	return items, scanner.Err()
}

// ListSpecs reads all markdown files in specsDir and parses their metadata.
func ListSpecs(rootDir, specsDir, activeSpec string) ([]SpecInfo, error) {
	fullSpecsDir := specsDir
	if !filepath.IsAbs(fullSpecsDir) {
		fullSpecsDir = filepath.Join(rootDir, specsDir)
	}

	entries, err := os.ReadDir(fullSpecsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	normActive := strings.TrimPrefix(filepath.Clean(activeSpec), "./")
	reNumbered := regexp.MustCompile(`^(\d+)[-_](.+)\.md$`)

	var specs []SpecInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		filename := e.Name()
		fullPath := filepath.Join(fullSpecsDir, filename)
		relPath := filepath.Join(specsDir, filename)
		normRel := strings.TrimPrefix(filepath.Clean(relPath), "./")

		num := 0
		padded := ""
		slug := strings.TrimSuffix(filename, ".md")

		if m := reNumbered.FindStringSubmatch(filename); m != nil {
			num, _ = strconv.Atoi(m[1])
			padded = fmt.Sprintf("%02d", num)
			slug = m[2]
		}

		title, goal := extractTitleAndGoal(fullPath)
		if title == "" {
			title = strings.ReplaceAll(slug, "-", " ")
			title = strings.Title(title)
		}

		isActive := (normActive != "" && (normActive == normRel || normActive == filename || strings.HasSuffix(normActive, "/"+filename)))

		specs = append(specs, SpecInfo{
			Number:    num,
			PaddedNum: padded,
			Slug:      slug,
			Filename:  filename,
			RelPath:   relPath,
			FullPath:  fullPath,
			Title:     title,
			Goal:      goal,
			IsActive:  isActive,
			Exists:    true,
		})
	}

	sort.Slice(specs, func(i, j int) bool {
		if specs[i].Number != specs[j].Number {
			return specs[i].Number < specs[j].Number
		}
		return specs[i].Filename < specs[j].Filename
	})

	return specs, nil
}

// extractTitleAndGoal extracts the first heading and the goal section from a spec file.
func extractTitleAndGoal(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}

	var title, goal string
	lines := strings.Split(string(data), "\n")
	inGoal := false
	var goalLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if title == "" && strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimPrefix(trimmed, "# ")
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "objetivo") || strings.Contains(lower, "goal") {
				inGoal = true
				continue
			} else if inGoal {
				break
			}
		}

		if inGoal {
			if strings.HasPrefix(trimmed, "## ") {
				break
			}
			if trimmed != "" {
				goalLines = append(goalLines, trimmed)
			}
		}
	}

	goal = strings.Join(goalLines, " ")
	return title, goal
}

// ResolveSpec resolves a spec query string (e.g. "1", "01", "auth", "01-auth-jwt.md")
// against existing specs and .forge/roadmap.md.
func ResolveSpec(rootDir, specsDir, roadmapPath, query string) (*SpecInfo, error) {
	query = strings.TrimSpace(query)
	specs, err := ListSpecs(rootDir, specsDir, "")
	if err != nil {
		return nil, fmt.Errorf("listing specs: %w", err)
	}

	// 1. If query is numeric (e.g. "1", "01")
	if num, err := strconv.Atoi(query); err == nil && num > 0 {
		padded := fmt.Sprintf("%02d", num)
		// Check existing specs by number
		for _, s := range specs {
			if s.Number == num || s.PaddedNum == padded {
				return &s, nil
			}
		}

		// Not found in existing specs. Check roadmap.md
		fullRoadmap := roadmapPath
		if !filepath.IsAbs(fullRoadmap) {
			fullRoadmap = filepath.Join(rootDir, roadmapPath)
		}

		if roadmapItems, err := ParseRoadmap(fullRoadmap); err == nil && len(roadmapItems) > 0 {
			for _, item := range roadmapItems {
				if item.Number == num {
					slug := Slugify(item.Title)
					filename := fmt.Sprintf("%02d-%s.md", num, slug)
					relPath := filepath.Join(specsDir, filename)
					return &SpecInfo{
						Number:    num,
						PaddedNum: padded,
						Slug:      slug,
						Filename:  filename,
						RelPath:   relPath,
						FullPath:  filepath.Join(rootDir, relPath),
						Title:     item.Title,
						Goal:      item.Goal,
						Exists:    false,
					}, nil
				}
			}
		}

		// Not in roadmap either; provide a clean new spec target
		slug := fmt.Sprintf("spec-%02d", num)
		filename := fmt.Sprintf("%02d-%s.md", num, slug)
		relPath := filepath.Join(specsDir, filename)
		return &SpecInfo{
			Number:    num,
			PaddedNum: padded,
			Slug:      slug,
			Filename:  filename,
			RelPath:   relPath,
			FullPath:  filepath.Join(rootDir, relPath),
			Title:     fmt.Sprintf("Spec %02d", num),
			Goal:      "",
			Exists:    false,
		}, nil
	}

	// 2. Query is slug or filename match
	qLower := strings.ToLower(query)
	for _, s := range specs {
		if strings.ToLower(s.Filename) == qLower ||
			strings.ToLower(s.Slug) == qLower ||
			strings.TrimSuffix(strings.ToLower(s.Filename), ".md") == qLower {
			return &s, nil
		}
	}

	// Partial search in existing specs
	for _, s := range specs {
		if strings.Contains(strings.ToLower(s.Slug), qLower) || strings.Contains(strings.ToLower(s.Title), qLower) {
			return &s, nil
		}
	}

	// 3. Fallback to roadmap search by title
	fullRoadmap := roadmapPath
	if !filepath.IsAbs(fullRoadmap) {
		fullRoadmap = filepath.Join(rootDir, roadmapPath)
	}
	if roadmapItems, err := ParseRoadmap(fullRoadmap); err == nil {
		for _, item := range roadmapItems {
			if strings.Contains(strings.ToLower(item.Title), qLower) {
				slug := Slugify(item.Title)
				filename := fmt.Sprintf("%02d-%s.md", item.Number, slug)
				relPath := filepath.Join(specsDir, filename)
				return &SpecInfo{
					Number:    item.Number,
					PaddedNum: item.PaddedNum,
					Slug:      slug,
					Filename:  filename,
					RelPath:   relPath,
					FullPath:  filepath.Join(rootDir, relPath),
					Title:     item.Title,
					Goal:      item.Goal,
					Exists:    false,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("spec %q not found in %s or %s", query, specsDir, roadmapPath)
}

// CreateSpecSkeleton creates a valid markdown spec file with the 4 mandatory sections.
func CreateSpecSkeleton(fullPath, title, goal string) error {
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}

	if goal == "" {
		goal = "Implementação e alinhamento dos contratos da especificação."
	}

	content := fmt.Sprintf(`# Especificação: %s

## Objetivo
%s

## Contratos e Assinaturas
- Tipos, interfaces ou endpoints centrais (alinhados a .forge/GLOSSARY.md):
  - Assinaturas de métodos e estruturas de dados de entrada/saída
  - Validações de integridade de esquema

## Casos de Borda
- Cenários com entradas nulas, vazias ou fora dos limites
- Tratamento explícito de timeout, cancelamento ou indisponibilidade
- Comportamento sob falha de validação e códigos de erro semânticos

## Fora de Escopo (YAGNI)
- Otimizações prematuras ou abstrações de uso único
- Funcionalidades secundárias não descritas no objetivo
- Alterações em módulos não relacionados
`, title, goal)

	return os.WriteFile(fullPath, []byte(content), 0o644)
}

// WriteRoadmapSkeleton generates a default .forge/roadmap.md template if not present.
func WriteRoadmapSkeleton(fullPath string) (bool, error) {
	if _, err := os.Stat(fullPath); err == nil {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return false, err
	}

	template := `# Project Roadmap

<!--
Defina as fases/funcionalidades do sistema numeradas sequencialmente.
O Forge associa automaticamente comandos como '/spec 1' ao item '01'.
-->

- [ ] 01 - Foundation & Core Contracts: Definição dos tipos de domínio e contratos base
- [ ] 02 - Primary Feature Flow: Implementação do fluxo principal de negócio
- [ ] 03 - API & Integrations: Endpoints de serviço e integrações externas
`

	return true, os.WriteFile(fullPath, []byte(template), 0o644)
}

// WriteGlossarySkeleton generates a default .forge/GLOSSARY.md template if not present.
func WriteGlossarySkeleton(fullPath string) (bool, error) {
	if _, err := os.Stat(fullPath); err == nil {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return false, err
	}

	template := `# Glossário de Domínio (DDD)

Linguagem Ubíqua e termos canônicos do sistema. Mantenha definições concisas (1-2 frases) focadas no que o conceito É.

## Termos

<!--
Exemplo de formato para novos termos:
**NomeDoTermo**:
Definição concisa de uma ou duas frases sobre o conceito único no domínio.
_Avoid_: SinônimoAmbíguo, OutroNome
-->
`
	return true, os.WriteFile(fullPath, []byte(template), 0o644)
}

// ValidateGlossary verifies the existence and consistency of the DDD glossary.
// It checks that the file exists, has a valid title, and contains at least one defined domain term.
func ValidateGlossary(rootDir, glossaryPath string) error {
	fullPath := glossaryPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(rootDir, glossaryPath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("glossary file %q does not exist. O alinhamento DDD e interrogação são obrigatórios no estágio SPEC", glossaryPath)
		}
		return fmt.Errorf("reading glossary file %q: %w", glossaryPath, err)
	}

	content := strings.TrimSpace(string(data))
	if len(content) == 0 {
		return fmt.Errorf("glossary file %q is empty", glossaryPath)
	}

	lower := strings.ToLower(content)
	if !strings.Contains(lower, "# glossár") && !strings.Contains(lower, "# glossar") {
		return fmt.Errorf("glossary %q is missing a '# Glossário' or '# Glossary' header", glossaryPath)
	}

	// Scan lines ignoring comment blocks <!-- ... -->
	lines := strings.Split(content, "\n")
	inComment := false
	reTerm := regexp.MustCompile(`^\s*\*\*([^*]+)\*\*:\s*`)
	termCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<!--") {
			inComment = true
		}
		if inComment {
			if strings.Contains(trimmed, "-->") {
				inComment = false
			}
			continue
		}
		if m := reTerm.FindStringSubmatch(trimmed); m != nil {
			term := strings.TrimSpace(m[1])
			if term != "" {
				termCount++
			}
		}
	}

	if termCount == 0 {
		return fmt.Errorf("glossary %q contains no defined domain terms (expected format: **Termo**: Definição)", glossaryPath)
	}

	return nil
}

// IsSpecDraft checks if a spec file is a draft, placeholder, or incomplete.
func IsSpecDraft(fullPath string) (bool, string) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return true, "spec file does not exist"
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) < 15 {
		return true, fmt.Sprintf("spec has only %d lines (minimum 15)", len(lines))
	}
	if strings.Contains(content, "Implementação e alinhamento dos contratos da especificação.") {
		return true, "spec contains default skeleton goal placeholder"
	}
	if strings.Contains(content, "Assinaturas de métodos e estruturas de dados de entrada/saída") &&
		strings.Contains(content, "Validações de integridade de esquema") {
		return true, "spec contains default skeleton contracts placeholder"
	}
	return false, "complete"
}
