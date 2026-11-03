````
# 07 - Workflows Executáveis do Antigravity CLI (.agent/workflows/)

Estes são os arquivos que o `forge init` cria em `.agent/workflows/` para que o Antigravity CLI (`agy`) registre os slash commands no terminal. Cada workflow orquestra a chamada de transição do `forge`, lê o contexto enxuto e delega as ações.

## 1. `.agent/workflows/spec.md`
```markdown
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
````

## 2. `.agent/workflows/plan.md`

Markdown

```
---
description: "Decompõe a especificação aprovada em um grafo DAG de tarefas no Beads"---

1. Execute no terminal: `forge stage plan`.
2. Leia a spec indicada em `.forge/state.json`.
3. Use `serena:find_referencing_symbols` para medir o alcance das alterações.
4. Divida o trabalho em fatias verticais (Tracer Bullets) independentes.
5. Crie as tarefas no Beads via terminal:
   - `bd create "<título>" -d "<critérios e símbolos>" -p 1`
   - Se houver dependência entre elas: `bd dep add <id-filho> <id-pai>`
6. Execute `bd ready` para listar o primeiro nó desbloqueado.
7. Avance para a codificação executando: `forge stage build`.
```

## 3. `.agent/workflows/build.md`

Markdown

```
---
description: "Implementa cirurgicamente a tarefa ativa do Beads via Serena e ast-grep"---

1. Execute no terminal: `forge stage build`.
2. Identifique a tarefa em foco lendo `active_bead_id` em `.agent/rules/active-stage.md`.
3. Não leia arquivos inteiros com comandos de shell. Use estritamente:
   - `serena:find_symbol` para inspecionar a assinatura/corpo alvo.
   - `serena:replace_symbol_body` ou `serena:insert_after_symbol` para aplicar as mudanças.
   - `sg` (ast-grep) se a alteração for um padrão repetido em múltiplos nós.
4. Se encontrar problemas fora do escopo do ticket, cadastre via `bd create "<desc>" --deps discovered-from:<id>` e não implemente agora.
5. Responda em no máximo 2 frases e avance executando: `forge stage test`.
```

## 4. `.agent/workflows/test.md`

Markdown

```
---
description: "Valida a implementação com suíte de testes filtrada pelo RTK"---

1. Execute no terminal: `forge stage test`.
2. Escreva ou atualize os testes unitários/integração cobrindo os casos de borda da Spec.
3. Execute a suíte através do wrapper: `forge exec -- test`.
4. Se houver falha, inspecione o log resumido devolvido pelo RTK, ajuste o código via Serena e reexecute.
5. Com todos os testes passando (exit code 0), avance executando: `forge stage review`.
```

## 5. `.agent/workflows/review.md`

Markdown

```
---
description: "Audita o diff semântico contra a Spec e valida integridade de contratos"---

1. Execute no terminal: `forge stage review`.
2. Execute `forge exec -- git diff` para inspecionar as alterações de forma comprimida.
3. Use `serena:find_referencing_symbols` para garantir que nenhum chamador externo quebrou.
4. Confronte o diff com a spec ativa em `.forge/specs/`. Se houver desvio de requisito ou violação de contrato, retorne com `forge stage build`.
5. Se o diff estiver correto, avance executando: `forge stage simplify`.
```

## 6. `.agent/workflows/simplify.md`

Markdown

```
---
description: "Aplica poda YAGNI (Ponytail) removendo código morto e abstrações inúteis"---

1. Execute no terminal: `forge stage simplify`.
2. Execute `sg scan --rule .forge/rules/complexity-guard.yml` para identificar nós excessivamente aninhados.
3. Verifique se os símbolos exportados novos possuem referências reais via `serena:find_referencing_symbols`. Se tiverem 0 referências externas, delete ou torne privados.
4. Se houver abstrações usadas apenas uma vez sem motivo arquitetural evidente, faça o inline no chamador.
5. Avance para a entrega executando: `forge stage ship`.
```

## 7. `.agent/workflows/ship.md`

Markdown

```
---
description: "Executa os gates determinísticos finais, fecha o nó no Beads e limpa a sessão"---

1. Execute no terminal: `forge stage ship`.
2. Execute o validador completo: `forge gate`.
3. Se todos os portões retornarem aprovação, feche a tarefa ativa no Beads:
   `bd close <active_bead_id> --reason "<resumo técnico de 1 linha>"`
4. Faça o commit das alterações no Git vinculando o ID da tarefa.
5. Finalize informando ao usuário o fechamento da tarefa e recomende o reset de contexto antes do próximo ciclo.
```
