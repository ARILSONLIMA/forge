---
name: build
description: Implementa cirurgicamente a tarefa ativa do Beads via Serena e ast-grep
---

# Forge: Estágio 3 - BUILD

1. Se um argumento for fornecido (ex.: `/build 1`), execute: `forge stage build <argumento>`.
2. Caso contrário, execute no terminal: `forge stage build`.
3. Identifique a tarefa em foco lendo `active_bead_id` em `.agent/rules/active-stage.md`.
4. Não leia arquivos inteiros com comandos de shell. Use estritamente:
   - `serena:find_symbol` para inspecionar a assinatura/corpo alvo.
   - `serena:replace_symbol_body` ou `serena:insert_after_symbol` para aplicar as mudanças.
   - `sg` (ast-grep) se a alteração for um padrão repetido em múltiplos nós.
5. Se encontrar problemas fora do escopo do ticket, cadastre via `bd create "<desc>" --deps discovered-from:<id>` e não implemente agora.
6. Responda em no máximo 2 frases e avance executando: `forge stage test`.
