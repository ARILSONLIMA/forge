---
description: "Inicia o ciclo de engenharia alinhando contratos e criando a especificação"
---

1. Execute no terminal: `forge stage spec`.
2. Verifique o estado atual lendo `.agent/rules/active-stage.md`.
3. Use a ferramenta `serena:get_symbols_overview` nos pacotes/módulos alvo para mapear tipos e assinaturas existentes sem ler implementações inteiras.
4. Se houver dúvidas de regra de negócio, faça no máximo 3 perguntas diretas ao usuário.
5. Escreva a especificação em `.forge/specs/<nome-da-feature>.md` contendo:
   - Objetivo (máx. 3 linhas)
   - Contratos e Assinaturas
   - Casos de Borda
   - Fora de Escopo (YAGNI)
6. Avance executando no terminal: `forge stage plan`.
