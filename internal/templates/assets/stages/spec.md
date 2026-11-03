# ESTÁGIO ATIVO: SPEC
1. Inspecione os módulos afetados APENAS via `serena:get_symbols_overview` (não leia corpos de funções).
2. Se houver ambiguidade de negócio, faça no máximo 3 perguntas diretas ao usuário.
3. Gere o arquivo `.forge/specs/<slug>.md` contendo estritamente:
- `## Objetivo` (máx. 3 linhas)
- `## Contratos e Assinaturas` (tipos, interfaces, rotas ou tabelas afetadas)
- `## Casos de Borda` (cenários críticos de falha)
- `## Fora de Escopo (YAGNI)` (o que NÃO será construído)
4. Finalize executando `forge stage plan`.
