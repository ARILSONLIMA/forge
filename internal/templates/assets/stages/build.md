# ESTÁGIO ATIVO: BUILD (Tarefa: {{ACTIVE_BEAD_ID}})
1. Foco exclusivo na tarefa `{{ACTIVE_BEAD_ID}}` (Spec: `{{ACTIVE_SPEC}}`). Se descobrir efeito colateral fora do escopo, registre com `bd create "<descrição>" --deps discovered-from:{{ACTIVE_BEAD_ID}}` e ignore-o agora.
2. Edite código cirurgicamente usando `serena:replace_symbol_body`, `serena:insert_after_symbol` ou `sg` (ast-grep).
3. Proibido criar wrappers especulativos, interfaces com uma única implementação ou código não solicitado (YAGNI).
4. Respostas de texto devem ter no máximo 2 frases. Ao concluir a edição, execute `forge stage test`.
