# 05 - Ferramentas Integradas e Ecossistema (Runtime e Metodologia)

A arquitetura da ferramenta divide as ferramentas do ecossistema em dois grupos: os **binários e servidores MCP de execução direta** (o motor determinístico do Forge CLI) e os **repositórios conceituais de regras** (de onde extraímos a lógica dos 7 estágios).

## 1. Ferramentas Essenciais do Motor (Runtime e Binários)

Estas são as ferramentas de linha de comando e servidores MCP que o `forge` invoca diretamente via subprocesso ou expõe ao Antigravity CLI (`agy`):

- **RTK (Rust Token Killer)**
  - **Repositório:** [rtk-ai/rtk](https://github.com/rtk-ai/rtk)
  - **Papel:** Proxy CLI que intercepta stdout/stderr de comandos de terminal (`git diff`, linters, suítes de teste), cortando de 80% a 90% da saída redundante antes que o texto chegue ao LLM.
  - **Instalação:** `cargo install rtk` ou download da release pré-compilada.

- **Serena MCP**
  - **Repositório:** [oraios/serena](https://github.com/oraios/serena)
  - **Papel:** Servidor MCP que conecta o agente diretamente aos Language Servers locais (`gopls`, `tsserver`, `pyright`, etc.), permitindo navegação semântica (`find_symbol`, `replace_symbol_body`, referências) sem ler arquivos inteiros.
  - **Execução via MCP:** `uvx --from git+https://github.com/oraios/serena.git serena` (gerenciado dentro do `.mcp.json`).

- **Beads (`bd`)**
  - **Repositório:** [steveyegge/beads](https://github.com/steveyegge/beads)
  - **Papel:** Rastreamento determinístico de tarefas persistido em grafo DAG no próprio Git. Mantém a memória de progresso entre sessões sem poluir o contexto com logs de chat antigos.
  - **Instalação:** Binário via GitHub Releases ou compilação Go (`go install github.com/steveyegge/beads/cmd/bd@latest`).

- **ast-grep (`sg`)**
  - **Repositório:** [ast-grep/ast-grep](https://github.com/ast-grep/ast-grep)
  - **Papel:** Busca e mutação sintática na AST do código para auditorias automatizadas nos Quality Gates e refatorações cirúrgicas em lote nos estágios `Build` e `Simplify`.
  - **Instalação:** `cargo install ast-grep --locked` ou `npm install -g @ast-grep/cli`.

---

## 2. Repositórios de Regras e Metodologia (Base dos Prompts dos 7 Estágios)

Estes repositórios fornecem a estrutura conceitual destilada e embutida nos templates de `.agent/workflows/` e `.agent/rules/active-stage.md`:

- **Agent Skills (Addy Osmani)**
  - **Repositório:** [addyosmani/agent-skills](https://github.com/addyosmani/agent-skills)
  - **Contribuição no Forge:** A disciplina de desenvolvimento com portões de verificação, padrões TDD e tabelas de anti-racionalização que regem os estágios `/test` e `/review`.

- **Matt Pocock Skills**
  - **Repositório:** [mattpocock/skills](https://github.com/mattpocock/skills)
  - **Contribuição no Forge:** Os fluxos de *Wayfinder*, *To-Spec* e *To-Tickets*, adaptados para guiar o levantamento de contratos no `/spec` e a quebra em fatias verticais (*tracer bullets*) no `/plan`.

- **Ponytail**
  - **Repositório:** [dietrichgebert/ponytail](https://github.com/dietrichgebert/ponytail)
  - **Contribuição no Forge:** A mentalidade sênior minimalista e as regras YAGNI usadas no estágio `/simplify` para podar abstrações precoces e remover código sem chamadores reais.

---

## 3. Ferramentas da Análise Inicial (Opcionais ou Substituídas)

Projetos analisados na concepção inicial que servem como referência ou ferramentas auxiliares sob demanda:

- **Caveman** ([juliusbrussee/caveman](https://github.com/juliusbrussee/caveman))
  - *Status no Forge:* A compressão de I/O é tratada pelo binário de alta performance do **RTK**, mantendo o estilo de resposta concisa no estágio `/build` sem poluir tokens de prompt com regras de fala.

- **Graphify** ([Graphify-Labs/graphify](https://github.com/Graphify-Labs/graphify))
  - *Status no Forge:* Substituído no pipeline principal de código pelo **Serena MCP** (LSP tem precisão de compilador para tipos e referências). Permanece útil para visualização macro de arquitetura.

- **MemPalace** ([MemPalace/mempalace](https://github.com/MemPalace/mempalace))
  - *Status no Forge:* Substituído pelo **Beads (`bd`)**. Em vez de armazenar transcrições completas de chats que degradam a janela do LLM, o Forge mantém o estado estritamente estruturado em tarefas atômicas no Git.
