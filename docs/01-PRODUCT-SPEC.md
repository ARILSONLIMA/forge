# 01 - Product Specification: Forge CLI (Deterministic AI Engineering Harness)

## 1. Visão Geral e Objetivo

O **Forge CLI (`forge`)** é um orquestrador determinístico de engenharia assistida por IA construído para operar em conjunto com o **Google Antigravity CLI (`agy`)**. Seu objetivo é executar o ciclo de vida completo de desenvolvimento em 7 estágios estritos (`Spec -> Plan -> Build -> Test -> Review -> Simplify -> Ship`), combinando:

1. **Qualidade máxima de código**: Edição semântica via LSP e portões de qualidade bloqueantes.
2. **Degradação zero de contexto**: Sessões efêmeras por tarefa e injeção dinâmica de prompts por estágio (máximo de 250 tokens de instruções ativas por vez).
3. **Economia extrema de tokens**: Compressão de saída de terminal via proxy binário e proibição de leitura integral de arquivos longos.

## 2. Stack Tecnológica do Orquestrador

- **Linguagem**: Go 1.23+ (binário único estático, zero dependências de runtime, inicialização < 5ms).
- **Empacotamento de Templates**: Uso de `embed.FS` nativo do Go para embutir os arquivos de workflow do Antigravity (`.agent/workflows/*.md`), regras do `ast-grep` e configuração MCP.
- **Persistência de Estado Local**: Arquivo `.forge/state.json` sincronizado com o grafo de tarefas do Beads (`bd`).

## 3. Ecossistema de Ferramentas Integradas (Dependências Externas)

O `forge` atua como maestro das seguintes ferramentas de linha de comando e servidores MCP:

| Ferramenta          | Binário / Comando           | Papel na Arquitetura                                              | Estágios de Atuação                   |
|:------------------- |:--------------------------- |:----------------------------------------------------------------- |:------------------------------------- |
| **Antigravity CLI** | `agy`                       | Agente executor principal e interface de slash commands           | Todos (`Spec` a `Ship`)               |
| **RTK**             | `rtk`                       | Proxy binário em Rust que comprime stdout/stderr em até 90%       | Todos (Camada de I/O)                 |
| **Serena MCP**      | `uvx --from git+... serena` | Servidor MCP conectado ao LSP (`gopls`, `tsserver`, `pyright`)    | `Spec`, `Build`, `Review`, `Simplify` |
| **Beads**           | `bd`                        | Grafo DAG de tarefas persistido no Git para memória entre sessões | `Plan`, `Build`, `Ship`               |
| **ast-grep**        | `sg`                        | Busca e refatoração estrutural baseada em AST                     | `Build`, `Simplify`                   |

## 4. Princípios Arquiteturais Inegociáveis

1. **Proibição de Fusão Monolítica**: Os prompts dos 7 estágios NUNCA devem ser carregados simultaneamente. O `forge` reescreve o arquivo `.agent/rules/active-stage.md` a cada transição de fase, contendo exclusivamente as regras do estágio atual.
2. **Uma Tarefa por Janela de Contexto**: Cada ciclo `Build -> Test -> Review -> Simplify -> Ship` opera sobre um único nó do Beads (`bd-xxxx`). Ao executar `Ship`, a sessão de contexto deve ser encerrada.
3. **Navegação Semântica Obrigatória**: Durante `Build`, `Review` e `Simplify`, o agente é proibido de ler arquivos inteiros de código-fonte (> 100 linhas) usando `cat` ou leitura bruta. Toda inspeção deve ocorrer via `serena` (`get_symbols_overview`, `find_symbol`, `find_referencing_symbols`) ou `rtk read -l aggressive`.
4. **Transições Governadas por Código de Saída**: O agente não decide subjetivamente se um estágio terminou. A transição só ocorre quando o comando `forge next` executa o Quality Gate determinístico da fase atual e recebe `exit code 0`.

## 5. Interface de Linha de Comando (`forge`)

O binário `forge` deve expor os seguintes subcomandos:

- `forge init`: Verifica a instalação de `rtk`, `bd`, `sg` e `uvx`, inicializa o `.forge/state.json`, configura o `.mcp.json` para o Serena e gera os 7 workflows em `.agent/workflows/`.
- `forge status`: Exibe o estágio atual, a tarefa ativa do Beads (`bd-xxxx`), os portões já validados e o consumo estimado da sessão.
- `forge stage <nome>`: Tenta transicionar para o estágio informado (`spec`, `plan`, `build`, `test`, `review`, `simplify`, `ship`), executando antes o gate de saída do estágio anterior.
- `forge gate`: Executa manualmente as validações determinísticas do estágio atual sem avançar de fase.
- `forge exec -- <cmd>`: Wrapper que executa comandos de shell forçando o uso do proxy `rtk` e bloqueando comandos proibidos no estágio atual.
- `forgedoctor`: Diagnostica a saúde das integrações (LSP ativo no Serena, repositório Beads íntegro, binários no PATH).
