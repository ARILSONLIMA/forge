---
name: plan
description: Decompõe a especificação aprovada em um grafo DAG de tarefas no Beads
---

# Forge: Estágio 2 - PLAN

1. Se um argumento ou número for fornecido (ex.: `/plan 1`):
   - Execute no terminal: `forge stage plan <argumento>` (ou `forge spec <argumento>`).
2. Caso contrário, execute no terminal: `forge stage plan`.
3. Leia a spec ativa indicada em `.forge/state.json` (ou `.agent/rules/active-stage.md`).
4. Use `serena:find_referencing_symbols` para medir o alcance das alterações.
5. Divida o trabalho em fatias verticais (Tracer Bullets) independentes.
6. Crie as tarefas no Beads via terminal:
   - `bd create "<título>" -d "<critérios e símbolos>" -p 1`
   - Se houver dependência entre elas: `bd dep add <id-filho> <id-pai>`
7. Execute `bd ready` para listar o primeiro nó desbloqueado.
8. Avance para a codificação executando: `forge stage build`.
