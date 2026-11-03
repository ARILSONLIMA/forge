# ESTÁGIO ATIVO: SPEC
1. Inspecione os módulos afetados APENAS via `serena:get_symbols_overview` (não leia corpos de funções).
2. Se houver ambiguidade de negócio, consulte `.forge/roadmap.md` ou faça no máximo 1 pergunta direta.
3. Mantenha a especificação ativa em {{ACTIVE_SPEC}} (formato `01-<slug>.md`) contendo estritamente:
- `## Objetivo` (máx. 3 linhas)
- `## Contratos e Assinaturas` (tipos, interfaces, rotas ou tabelas afetadas)
- `## Casos de Borda` (cenários críticos de falha)
- `## Fora de Escopo (YAGNI)` (o que NÃO será construído)
4. Finalize executando `forge stage plan`.
