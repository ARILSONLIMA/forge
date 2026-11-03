---
description: "Decompõe a especificação aprovada em um grafo DAG de tarefas no Beads"
---

1. Execute no terminal: `forge stage plan`.
2. Leia a spec indicada em `.forge/state.json`.
3. Use `serena:find_referencing_symbols` para medir o alcance das alterações.
4. Divida o trabalho em fatias verticais (Tracer Bullets) independentes.
5. Crie as tarefas no Beads via terminal:
   - `bd create "<título>" -d "<critérios e símbolos>" -p 1`
   - Se houver dependência entre elas: `bd dep add <id-filho> <id-pai>`
6. Execute `bd ready` para listar o primeiro nó desbloqueado.
7. Avance para a codificação executando: `forge stage build`.
