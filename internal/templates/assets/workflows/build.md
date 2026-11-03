---
description: "Implementa cirurgicamente a tarefa ativa do Beads via Serena e ast-grep"
---

1. Execute no terminal: `forge stage build`.
2. Identifique a tarefa em foco lendo `active_bead_id` em `.agent/rules/active-stage.md`.
3. Não leia arquivos inteiros com comandos de shell. Use estritamente:
   - `serena:find_symbol` para inspecionar a assinatura/corpo alvo.
   - `serena:replace_symbol_body` ou `serena:insert_after_symbol` para aplicar as mudanças.
   - `sg` (ast-grep) se a alteração for um padrão repetido em múltiplos nós.
4. Se encontrar problemas fora do escopo do ticket, cadastre via `bd create "<desc>" --deps discovered-from:<id>` e não implemente agora.
5. Responda em no máximo 2 frases e avance executando: `forge stage test`.
