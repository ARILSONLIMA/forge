# 03 - Especificação dos 7 Estágios e Prompts Enxutos (.agent/workflows/)

Cada estágio possui um workflow acionável no Antigravity CLI (`/spec`, `/plan`, `/build`, `/test`, `/review`, `/simplify`, `/ship`) e um prompt de regra ativa ultracompacto (< 250 tokens).

## Estágio 1: `/spec` (Descoberta, Interrogação & Domínio DDD)

- **Inspiração**: Matt Pocock (`grill-me`, `grill-with-docs`, `domain-modeling`, `To-Spec`) + Serena Overview.
- **Objetivo**: Descoberta profunda de requisitos por árvore de decisão (*grilling* em rodadas), modelagem de Linguagem Ubíqua em `.forge/GLOSSARY.md` (e ADRs em `.forge/adr/`), mapeamento cirúrgico de contratos existentes sem ler corpos de funções e geração de `.forge/specs/<slug>.md`.
- **Ferramentas Permitidas**: `serena:get_symbols_overview`, `serena:find_symbol` (apenas assinaturas), escrita em `.forge/specs/`, escrita em `.forge/GLOSSARY.md` e `.forge/adr/`.
- **Ferramentas Proibidas**: Edição de código-fonte, criação de tarefas no Beads, leitura bruta de arquivos.
- **Template do `.agent/rules/active-stage.md` (Estágio SPEC)**:

```markdown
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
```

## Estágio 2: `/plan` (Fatiamento Vertical no Grafo Beads)

- **Inspiração**: Matt Pocock (`To-Tickets` Tracer Bullets) + Steve Yegge (`Beads`).

- **Objetivo**: Converter a Spec aprovada em tarefas atômicas encadeadas no grafo DAG do Beads (`bd`).

- **Ferramentas Permitidas**: Leitura de `.forge/specs/<slug>.md`, `serena:find_referencing_symbols`, CLI `bd` (`bd create`, `bd dep add`, `bd ready`).

- **Ferramentas Proibidas**: Edição de código-fonte, criação de arquivos `.md` de tarefas avulsos.

- **Template do `.agent/rules/active-stage.md` (Estágio PLAN)**:

  Markdown

  ```
  # ESTÁGIO ATIVO: PLAN
  1. Leia a spec ativa em `.forge/state.json` e mapeie os símbolos impactados com `serena:find_referencing_symbols`.
  2. Decomponha a implementação em fatias verticais (Tracer Bullets) onde cada tarefa caiba em uma única sessão.
  3. Registre cada tarefa no Beads usando `bd create "<título>" -d "<símbolos alvo e critério de aceite>" -p 1`.
  4. Vincule a ordem de execução com `bd dep add <id-filho> <id-pai>`.
  5. Confirme a fila com `bd ready` e avance com `forge stage build`.
  ```

## Estágio 3: `/build` (Implementação Cirúrgica por Símbolo)

- **Inspiração**: Serena LSP + `ast-grep` + modo conciso Caveman.

- **Objetivo**: Implementar exclusivamente a tarefa apontada por `active_bead_id` editando a AST/símbolos sem ler nem regenerar arquivos inteiros.

- **Ferramentas Permitidas**: `serena:find_symbol`, `serena:replace_symbol_body`, `serena:insert_after_symbol`, `serena:insert_before_symbol`, `sg` (`ast-grep`), `rtk read -l aggressive`.

- **Ferramentas Proibidas**: Reescrever arquivos longos inteiros, implementar tarefas futuras do Beads.

- **Template do `.agent/rules/active-stage.md` (Estágio BUILD)**:

  Markdown

  ```
  # ESTÁGIO ATIVO: BUILD (Tarefa: {{ACTIVE_BEAD_ID}})
  1. Foco exclusivo na tarefa `{{ACTIVE_BEAD_ID}}`. Se descobrir efeito colateral fora do escopo, registre com `bd create "<descrição>" --deps discovered-from:{{ACTIVE_BEAD_ID}}` e ignore-o agora.
  2. Edite código cirurgicamente usando `serena:replace_symbol_body`, `serena:insert_after_symbol` ou `sg` (ast-grep).
  3. Proibido criar wrappers especulativos, interfaces com uma única implementação ou código não solicitado (YAGNI).
  4. Respostas de texto devem ter no máximo 2 frases. Ao concluir a edição, execute `forge stage test`.
  ```

## Estágio 4: `/test` (Verificação Comprimida via RTK)

- **Inspiração**: Addy Osmani (TDD/Verification) + RTK Proxy.

- **Objetivo**: Garantir que os testes dos símbolos modificados existam, testem riscos reais e passem com zero falhas.

- **Ferramentas Permitidas**: `forge exec -- <test_cmd>` (via `rtk`), edição de arquivos `*_test.*` ou `*.spec.*` via Serena.

- **Template do `.agent/rules/active-stage.md` (Estágio TEST)**:

  Markdown

  ```
  # ESTÁGIO ATIVO: TEST (Tarefa: {{ACTIVE_BEAD_ID}})
  1. Escreva ou atualize testes cobrindo apenas o comportamento novo e os Casos de Borda da Spec. Proibido testar getters/setters triviais.
  2. Execute a suíte exclusivamente via `forge exec -- <comando de teste>` (filtrado pelo RTK).
  3. Se houver falha, analise o resumo do RTK, corrija o símbolo via Serena e reexecute.
  4. Com 100% dos testes passando, execute `forge stage review`.
  ```

## Estágio 5: `/review` (Auditoria Semântica e Aderência à Spec)

- **Inspiração**: Addy Osmani (Code Review & Anti-Rationalization) + Serena LSP.

- **Objetivo**: Confrontar o `rtk git diff` e os diagnósticos do LSP com os contratos definidos na Spec inicial.

- **Ferramentas Permitidas**: `rtk git diff`, `serena:find_referencing_symbols`, leitura de `.forge/specs/<slug>.md`.

- **Template do `.agent/rules/active-stage.md` (Estágio REVIEW)**:

  Markdown

  ```
  # ESTÁGIO ATIVO: REVIEW (Tarefa: {{ACTIVE_BEAD_ID}})
  1. Inspecione as alterações com `rtk git diff` e compare cada mudança com `.forge/specs/<slug>.md`.
  2. Verifique via `serena:find_referencing_symbols` se algum chamador externo foi quebrado pelas mudanças de assinatura.
  3. Se encontrar bug, quebra de contrato ou falha de segurança, retorne ao código com `forge stage build`.
  4. Se o diff estiver íntegro e fiel à Spec, execute `forge stage simplify`.
  ```

## Estágio 6: `/simplify` (Poda YAGNI e Remoção de Código Morto)

- **Inspiração**: Ponytail (`Ponytail Audit` / Lazy Senior Dev) + `ast-grep` + Serena.

- **Objetivo**: Cortar complexidade acidental, remover símbolos sem referência e inline de abstrações usadas apenas uma vez.

- **Ferramentas Permitidas**: `serena:find_referencing_symbols`, `sg scan`, `serena:replace_symbol_body`.

- **Template do `.agent/rules/active-stage.md` (Estágio SIMPLIFY)**:

  Markdown

  ```
  # ESTÁGIO ATIVO: SIMPLIFY (Tarefa: {{ACTIVE_BEAD_ID}})
  1. Aja como um engenheiro sênior minimalista (Ponytail): menos código é melhor código.
  2. Verifique todas as funções/tipos criados ou tocados usando `serena:find_referencing_symbols`:
     - Símbolo com 0 referências: delete imediatamente.
     - Função auxiliar privada chamada apenas 1 vez sem ganho de legibilidade: faça inline no chamador.
  3. Se um atalho intencional foi adotado para evitar over-engineering, adicione um comentário de 1 linha `// SHORTCUT: <motivo>`.
  4. Avance com `forge stage ship`.
  ```

## Estágio 7: `/ship` (Trava Final, Fechamento no Grafo e Reset de Sessão)

- **Inspiração**: Beads (`bd close`) + Quality Gates Determinísticos.

- **Objetivo**: Validar todos os portões, fechar o nó no grafo do Beads registrando a decisão técnica em 1 linha, realizar o commit e limpar a janela de contexto.

- **Ferramentas Permitidas**: `forge gate`, `bd close`, `rtk git status`, `git commit`.

- **Template do `.agent/rules/active-stage.md` (Estágio SHIP)**:

  Markdown

  ```
  # ESTÁGIO ATIVO: SHIP (Tarefa: {{ACTIVE_BEAD_ID}})
  1. Execute `forge gate` para rodar a bateria final determinística (Build + Lint + Testes via RTK + Checagem AST).
  2. Feche a tarefa no Beads com `bd close {{ACTIVE_BEAD_ID}} --reason "<resumo técnico de 1 linha da solução>"`.
  3. Faça o commit das alterações vinculando o ID `{{ACTIVE_BEAD_ID}}`.
  4. Encerre a sessão imediatamente e instrua o usuário a limpar a janela de contexto (`/clear`) antes do próximo `/build`.
  ```
