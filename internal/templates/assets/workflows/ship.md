---
name: ship
description: "Executa os gates determinísticos finais, fecha o nó no Beads e limpa a sessão"
---

1. Execute no terminal: `forge stage ship`.
2. Execute o validador completo: `forge gate`.
3. Se todos os portões retornarem aprovação, feche a tarefa ativa no Beads:
   `bd close <active_bead_id> --reason "<resumo técnico de 1 linha>"`
4. Faça o commit das alterações no Git vinculando o ID da tarefa.
5. Finalize informando ao usuário o fechamento da tarefa e recomende o reset de contexto antes do próximo ciclo.
