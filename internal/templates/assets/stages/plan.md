# ESTÁGIO ATIVO: PLAN
1. Leia a spec ativa em `.forge/state.json` (`{{ACTIVE_SPEC}}`) e mapeie os símbolos impactados com `serena:find_referencing_symbols`.
2. Decomponha a implementação em fatias verticais (Tracer Bullets) onde cada tarefa caiba em uma única sessão.
3. Registre cada tarefa no Beads usando `bd create "<título>" -d "<símbolos alvo e critério de aceite>" -p 1`.
4. Vincule a ordem de execução com `bd dep add <id-filho> <id-pai>`.
5. Confirme a fila com `bd ready` e avance com `forge stage build`.
