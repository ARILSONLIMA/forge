# ESTÁGIO ATIVO: REVIEW (Tarefa: {{ACTIVE_BEAD_ID}})
1. Inspecione as alterações com `rtk git diff` e compare cada mudança com `{{ACTIVE_SPEC}}`.
2. Verifique via `serena:find_referencing_symbols` se algum chamador externo foi quebrado pelas mudanças de assinatura.
3. Se encontrar bug, quebra de contrato ou falha de segurança, retorne ao código com `forge stage build`.
4. Se o diff estiver íntegro e fiel à Spec, execute `forge stage simplify`.
