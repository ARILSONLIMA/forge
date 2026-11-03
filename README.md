# Forge CLI (`forge`)

> **Deterministic AI Engineering Harness** — Orquestrador determinístico de ciclo de vida de software para agentes de IA (Google Antigravity CLI / `agy`).

O **Forge** combina qualidade máxima de código, degradação zero de contexto e economia extrema de tokens, governando o desenvolvimento em **7 estágios estritos** através de portões de qualidade binários (*Deterministic Quality Gates*).

---

## 🚀 Pilares do Forge

1. **Portões de Qualidade Determinísticos (Quality Gates)**: Transições entre fases não dependem de "opinião" do LLM; são validadas por verificações binárias em Go (parsers de especificação, `bd ready`, compilação, suíte de testes, linters e regras AST no `ast-grep`).
2. **Context Swapping Dinâmico (< 250 tokens)**: Em vez de inflar o contexto com conversas ou regras longas, o Forge injeta apenas as regras do estágio atual em `.agent/rules/active-stage.md`, mantendo a janela do LLM limpa e hiperfocada.
3. **Economia Extrema de Tokens**: Comandos de terminal rodam através do proxy **RTK** (redução de 80% a 90% de output redundante), e o guarda de I/O bloqueia a leitura bruta de arquivos longos, forçando o uso semântico via LSP (**Serena MCP**).
4. **Memória de Tarefas em Grafo DAG**: Progresso e dependências são gerenciados pelo **Beads (`bd`)** no Git, sem logs efêmeros de chat.

---

## 🔄 Os 7 Estágios de Engenharia

| # | Estágio | Slash Command | Foco Principal | Validação do Gate de Saída |
|---|---|---|---|---|
| **1** | **SPEC** | `/spec` | Alinhamento de contratos e casos de borda | Markdown em `.forge/specs/` com 4 seções obrigatórias (15–120 linhas) |
| **2** | **PLAN** | `/plan` | Quebra em fatias verticais (*tracer bullets*) | Pelo menos 1 tarefa aberta e desbloqueada no Beads (`bd ready`) |
| **3** | **BUILD** | `/build` | Implementação com edição semântica (LSP) | Compilação limpa (`rtk <build>`) e arquivos modificados no Git |
| **4** | **TEST** | `/test` | Testes automatizados e disciplina TDD | Suíte de testes aprovada (`rtk <test>`) e testes tocados/validados |
| **5** | **REVIEW** | `/review` | Linter e auditoria estática de AST | Zero erros de lint (`rtk <lint>`) e zero violações críticas no `ast-grep` |
| **6** | **SIMPLIFY** | `/simplify` | Poda YAGNI e remoção de código morto | Testes continuam passando e diff final $\le$ diff anterior |
| **7** | **SHIP** | `/ship` | Fechamento da tarefa e entrega | Tarefa fechada no Beads (`bd close`) e working tree do Git limpa |

---

## 📦 Instalação

### Pré-requisitos
- **Go 1.23+**
- Ferramentas recomendadas do ecossistema:
  - `git`
  - [rtk](https://github.com/rtk-ai/rtk) (`cargo install rtk`)
  - [beads](https://github.com/steveyegge/beads) (`go install github.com/steveyegge/beads/cmd/bd@latest`)
  - [ast-grep](https://github.com/ast-grep/ast-grep) (`cargo install ast-grep --locked`)
  - `uvx` (para levantar o Serena MCP automaticamente)

### Compilando e Instalando o Forge
```bash
# Instalação direta via Go (versão mais recente)
go install github.com/ARILSONLIMA/forge/cmd/forge@latest

# Ou compilando a partir do código fonte clonado
go build -o forge ./cmd/forge
```

Verifique seu ambiente a qualquer momento com:
```bash
forge doctor
```

---

## 🛠️ Comandos Principais

```bash
# Inicializar o Forge em um projeto (detecta Go, TypeScript, Python ou Rust)
forge init

# Ver o status do ciclo atual (estágio ativo, bead, gates aprovados e integridade)
forge status

# Executar a transição para um novo estágio (avalia o Quality Gate correspondente)
forge stage spec
forge stage plan
forge stage build
forge stage test
forge stage review
forge stage simplify
forge stage ship

# Testar o Quality Gate atual sem avançar o estágio
forge gate

# Executar comandos protegidos pelo RTK e I/O Guard
forge exec go test ./...
forge exec git status
```

---

## 📚 Documentação Técnica de Arquitetura

As especificações detalhadas do design do Forge estão organizadas na pasta [`docs/`](file:///home/arilson/PROJETOS/forge/docs/):

- **[01 - Product Specification](file:///home/arilson/PROJETOS/forge/docs/01-PRODUCT-SPEC.md)**: Visão macro, arquitetura e stack tecnológica.
- **[02 - State Machine & Core](file:///home/arilson/PROJETOS/forge/docs/02-STATE-MACHINE-AND-CORE.md)**: Estrutura do `.forge/`, integridade SHA-256 e injeção dinâmica de prompts.
- **[03 - Seven Stages Workflows](file:///home/arilson/PROJETOS/forge/docs/03-SEVEN-STAGES-WORKFLOWS.md)**: Definição conceitual detalhada e regras ultracompactas de cada estágio.
- **[04 - Deterministic Gates](file:///home/arilson/PROJETOS/forge/docs/04-DETERMINISTIC-GATES.md)**: Critérios binários de aprovação dos Gates 1 a 7 e regras de auto-recuo.
- **[05 - Ecosystem & Tools](file:///home/arilson/PROJETOS/forge/docs/05-ECOSYSTEM-TOOLS.md)**: Papel do RTK, Serena MCP, Beads (`bd`) e ast-grep (`sg`).
- **[06 - Config Templates & Rules](file:///home/arilson/PROJETOS/forge/docs/06-CONFIG-TEMPLATES-AND-RULES.md)**: Schemas de configuração, embutimento via `embed.FS` e regras de AST.
- **[07 - Antigravity Workflows](file:///home/arilson/PROJETOS/forge/docs/07-ANTIGRAVITY-WORKFLOWS.md)**: Workflows de slash commands integrados ao Antigravity CLI.
