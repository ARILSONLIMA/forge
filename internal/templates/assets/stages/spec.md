# ESTÁGIO ATIVO: SPEC
1. Inspecione contratos via `serena:get_symbols_overview` (fatos do código sem perguntar ao usuário).
2. Se spec incompleta ou incerteza: execute a Interrogação por Árvore de Decisão em rodadas (❓ Q... ➡️ <recomendação>) e atualize `.forge/GLOSSARY.md` inline com a Linguagem Ubíqua.
3. Se spec completa: apresente resumo de 3 linhas e confirme avanço.
4. Mantenha {{ACTIVE_SPEC}} (15-120 linhas) com:
- `## Objetivo` (máx. 3 linhas)
- `## Contratos e Assinaturas` (alinhados a .forge/GLOSSARY.md)
- `## Casos de Borda` (cenários críticos de falha)
- `## Fora de Escopo (YAGNI)` (o que NÃO será construído)
5. Valide Gate 1 (.forge/GLOSSARY.md + spec) e avance com `forge stage plan`.
