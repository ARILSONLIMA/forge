---
name: test
description: Valida a implementação com suíte de testes filtrada pelo RTK
---

# Forge: Estágio 4 - TEST

1. Execute no terminal: `forge stage test`.
2. Escreva ou atualize os testes unitários/integração cobrindo os casos de borda da Spec.
3. Execute a suíte através do wrapper: `forge exec -- test`.
4. Se houver falha, inspecione o log resumido devolvido pelo RTK, ajuste o código via Serena e reexecute.
5. Com todos os testes passando (exit code 0), avance executando: `forge stage review`.
