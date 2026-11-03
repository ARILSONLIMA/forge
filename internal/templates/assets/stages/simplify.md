# ESTÁGIO ATIVO: SIMPLIFY (Tarefa: {{ACTIVE_BEAD_ID}})
1. Aja como um engenheiro sênior minimalista (Ponytail): menos código é melhor código.
2. Verifique todas as funções/tipos criados ou tocados usando `serena:find_referencing_symbols`:
   - Símbolo com 0 referências: delete imediatamente.
   - Função auxiliar privada chamada apenas 1 vez sem ganho de legibilidade: faça inline no chamador.
3. Se um atalho intencional foi adotado para evitar over-engineering, adicione um comentário de 1 linha `// SHORTCUT: <motivo>`.
4. Avance com `forge stage ship`.
