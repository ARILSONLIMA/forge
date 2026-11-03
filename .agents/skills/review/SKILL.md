---
name: review
description: Audita o diff semântico contra a Spec e valida integridade de contratos
---

# Forge: Estágio 5 - REVIEW

1. Execute no terminal: `forge stage review`.
2. Execute `forge exec -- git diff` para inspecionar as alterações de forma comprimida.
3. Use `serena:find_referencing_symbols` para garantir que nenhum chamador externo quebrou.
4. Confronte o diff com a spec ativa em `.forge/specs/`. Se houver desvio de requisito ou violação de contrato, retorne com `forge stage build`.
5. Se o diff estiver correto, avance executando: `forge stage simplify`.
