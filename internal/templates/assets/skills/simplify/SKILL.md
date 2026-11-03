---
name: simplify
description: Aplica poda YAGNI removendo código morto e abstrações inúteis
---

# Forge: Estágio 6 - SIMPLIFY

1. Execute no terminal: `forge stage simplify`.
2. Execute `sg scan --rule .forge/rules/complexity-guard.yml` para identificar nós excessivamente aninhados.
3. Verifique se os símbolos exportados novos possuem referências reais via `serena:find_referencing_symbols`. Se tiverem 0 referências externas, delete ou torne privados.
4. Se houver abstrações usadas apenas uma vez sem motivo arquitetural evidente, faça o inline no chamador.
5. Avance para a entrega executando: `forge stage ship`.
