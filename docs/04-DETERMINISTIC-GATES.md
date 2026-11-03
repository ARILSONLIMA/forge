# 04 - Especificação dos Quality Gates Determinísticos

O comando `forge stage <próximo>` ou `forge gate` executa verificações binárias em Go antes de permitir qualquer avanço na máquina de estados. Se um Gate retornar erro, a transição é abortada e o erro exato (comprimido) é devolvido ao terminal.

## Tabela de Portões por Transição

| Gate       | Transição            | Validação Executada pelo Binário `forge`                     | Critério de Aprovação                                                              |
|:---------- |:-------------------- |:------------------------------------------------------------ |:---------------------------------------------------------------------------------- |
| **Gate 1** | `SPEC -> PLAN`       | Parser Markdown valida `.forge/specs/<slug>.md`              | Presença das 4 seções obrigatórias e tamanho entre 15 e 120 linhas                 |
| **Gate 2** | `PLAN -> BUILD`      | Executa `bd ready --json`                                    | Pelo menos 1 tarefa aberta e desbloqueada no grafo DAG                             |
| **Gate 3** | `BUILD -> TEST`      | Executa `rtk <build_cmd>` e checa árvore suja no Git         | Compilação limpa (exit code 0) e arquivos modificados presentes                    |
| **Gate 4** | `TEST -> REVIEW`     | Executa `rtk <test_cmd>`                                     | Exit code 0 na suíte de testes e pelo menos 1 arquivo de teste tocado ou validado  |
| **Gate 5** | `REVIEW -> SIMPLIFY` | Executa `rtk <lint_cmd>` + `sg scan` com regras de segurança | Zero erros de linter e zero violações nas regras críticas do `ast-grep`            |
| **Gate 6** | `SIMPLIFY -> SHIP`   | Reexecuta `rtk <test_cmd>` + checagem de complexidade AST    | Testes continuam passando após a poda YAGNI e diff final <= diff do estágio Review |
| **Gate 7** | `SHIP -> IDLE/BUILD` | Verifica status do `active_bead_id` no Beads (`bd show`)     | Tarefa marcada como `closed` no Beads e working tree do Git limpa                  |

## Regras Embutidas do `ast-grep` (`.forge/rules/`)

O `forge init` deve gerar regras base para o `ast-grep` (`sg`) auditar o código automaticamente nos Gates 5 e 6:

1. **`no-empty-catch.yml`**: Bloqueia blocos de captura de erro vazios ou erros ignorados silenciosamente.
2. **`no-debug-artifacts.yml`**: Bloqueia `console.log`, `fmt.Println` de debug soltos ou `TODO` sem vínculo com um ID do Beads (`// TODO(bd-xxxx): ...`).
3. **`complexity-guard.yml`**: Sinaliza funções com mais de 4 níveis de aninhamento condicional para forçar simplificação via *early return* no estágio `SIMPLIFY`.

## Comportamento de Auto-Recuo (Rollback de Estado)

- Se o **Gate 4 (`TEST -> REVIEW`)** falhar duas vezes consecutivas, o `forge` mantém o estado em `BUILD` e injeta no topo de `.agent/rules/active-stage.md` as últimas 15 linhas filtradas pelo `rtk` contendo a falha do teste.
- Se o **Gate 6 (`SIMPLIFY -> SHIP`)** quebrar algum teste durante a remoção de código YAGNI, o `forge` alerta qual símbolo removido causou a regressão para restauração imediata.
