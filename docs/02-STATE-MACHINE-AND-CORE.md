# 02 - State Machine, Dynamic Context & I/O Guard

## 1. Estrutura de Diretórios Gerenciada pelo Forge

Ao rodar `forge init` em qualquer repositório, a seguinte estrutura deve ser criada e mantida:

```text
.forge/
├── config.yaml              # Comandos nativos do projeto (test_cmd, lint_cmd, build_cmd, language)
├── state.json               # Estado atual da máquina de estados e hash de integridade
├── specs/                   # Especificações enxutas geradas no estágio SPEC
│   └── <feature-slug>.md
├── rules/                   # Regras estruturais do ast-grep para auditoria e poda YAGNI
│   ├── dead-exports.yml
│   └── complexity-guard.yml
└── logs/
    └── gate-audit.log       # Histórico determinístico de aprovações e reprovações de gates

.agent/
├── rules/
│   └── active-stage.md      # Arquivo dinâmico (< 250 tokens) sobrescrito a cada mudança de estágio
└── workflows/               # Comandos acionáveis no Antigravity CLI (agy)
    ├── spec.md
    ├── plan.md
    ├── build.md
    ├── test.md
    ├── review.md
    ├── simplify.md
    └── ship.md

.mcp.json                    # Configuração do servidor MCP Serena apontando para a raiz do projeto
```

## 2. Esquema de Estado (`.forge/state.json`)

O arquivo `.forge/state.json` controla rigorosamente onde o agente está e o que ele pode fazer:

JSON

```
{
  "current_stage": "BUILD",
  "active_spec": ".forge/specs/auth-token-rotation.md",
  "active_bead_id": "bd-a1b2",
  "stage_entered_at": "2026-10-30T15:00:00Z",
  "modified_symbols": [
    "internal/auth/service.go::RotateToken",
    "internal/auth/repository.go::RevokeOldToken"
  ],
  "gates_passed": {
    "SPEC": true,
    "PLAN": true,
    "BUILD": false,
    "TEST": false,
    "REVIEW": false,
    "SIMPLIFY": false
  }
}
```

## 3. Grafo de Transições da Máquina de Estados

As transições permitidas são estritamente lineares com rotas de recuo automático em caso de falha:

1. `IDLE` -> `SPEC`: Inicia uma nova feature ou correção arquitetural.

2. `SPEC` -> `PLAN`: Exige Gate 1 aprovado (documento de spec válido com seções obrigatórias).

3. `PLAN` -> `BUILD`: Exige Gate 2 aprovado (tarefas criadas no Beads DAG e primeiro nó `bd ready` selecionado).

4. `BUILD` -> `TEST`: Exige Gate 3 aprovado (código compila e LSP não reporta erros de tipo/sintaxe).

5. `TEST` -> `REVIEW`: Exige Gate 4 aprovado (`rtk <test_cmd>` retorna código 0).

   - *Rota de falha*: Se `TEST` falhar, o estado recua automaticamente para `BUILD`.

6. `REVIEW` -> `SIMPLIFY`: Exige Gate 5 aprovado (todos os contratos da Spec atendidos e zero avisos críticos no diff).

   - *Rota de falha*: Se `REVIEW` detectar desvio de contrato, recua para `BUILD`.

7. `SIMPLIFY` -> `SHIP`: Exige Gate 6 aprovado (auditoria YAGNI concluída, zero símbolos órfãos introduzidos e testes passando via `rtk`).

8. `SHIP` -> `BUILD` (se houver próximo nó em `bd ready`) ou `IDLE` (se todas as tarefas da Spec foram fechadas).

## 4. Mecanismo de Injeção Dinâmica de Contexto (Swapping)

Para evitar que o LLM sofra degradação de atenção, o `forge` NUNCA expõe as regras dos 7 estágios ao mesmo tempo.Quando `forge stage <novo_estagio>` é executado com sucesso:

1. O binário valida o Gate do estágio atual.

2. Atualiza o campo `current_stage` em `.forge/state.json`.

3. Sobrescreve atomicamente o arquivo `.agent/rules/active-stage.md` com o template exclusivo daquele estágio (contendo no máximo 250 tokens).

4. Imprime no terminal apenas o resumo enxuto do novo estágio e os metadados da tarefa ativa (`active_bead_id`).

## 5. Guarda de I/O e Proxy `forge exec`

O subcomando `forge exec -- <comando>` intercepta comandos executados pelo agente:

- Se o comando for `git diff`, `git status`, `go test`, `npm test`, `pytest`, `cargo test` ou linters, o `forge` reescreve a chamada internamente para `rtk <comando>`, garantindo até 90% de economia de tokens.

- Se o estágio atual for `BUILD`, `REVIEW` ou `SIMPLIFY` e o comando tentar executar `cat`, `head` ou `tail` em arquivos de código-fonte com mais de 100 linhas, o `forge exec` bloqueia a execução com erro educativo:`[FORGE BLOCK] Leitura bruta bloqueada no estágio BUILD. Use as tools do Serena MCP (get_symbols_overview / find_symbol) ou 'rtk read -l aggressive <arquivo>'.`
