---
name: spec
description: Inicia o ciclo de engenharia do Forge alinhando requisitos via interrogação por árvore de decisão, modelagem DDD (.forge/GLOSSARY.md) e contratos
---

# Forge: Estágio 1 - SPEC (Descoberta, Interrogação & Domínio DDD)

Este estágio realiza a descoberta profunda de requisitos, estressa decisões arquiteturais via interrogação por árvore de decisão (*grilling*), modela a Linguagem Ubíqua em `.forge/GLOSSARY.md` e sintetiza a especificação formal para o Gate 1.

## 1. Ativação e Detecção Híbrida

1. Se um argumento ou número for fornecido (ex.: `/spec 1`, `/spec 01`, `/spec auth`):
   - Execute no terminal: `forge spec <argumento>` (ou `forge stage spec <argumento>`).
2. Se nenhum argumento for fornecido:
   - Execute no terminal: `forge stage spec` e consulte `forge spec` para listar as especificações e o roadmap.
3. **Avaliação de Estado (Modo Híbrido):**
   - **Spec Completa e Estável** (>= 15 linhas, 4 seções preenchidas e termos definidos no `.forge/GLOSSARY.md`):
     - Apresente um resumo executivo de no máximo 3 linhas:
       - **Objetivo:** <1 linha>
       - **Contratos Principais:** <1 linha>
       - **Status do Domínio:** <termos no glossário / ADRs>
     - Pergunte: *"Deseja estressar/refinar as decisões com uma nova rodada de interrogação ou avançar para `/plan`?"*
   - **Spec Nova, Rascunho ou Incompleta** (esqueleto recém-criado, placeholders padrão ou < 15 linhas):
     - Inicie imediatamente o **Protocolo de Interrogação por Árvore de Decisão** e **Modelagem DDD** abaixo.

## 2. Regra de Ouro: Fatos vs Decisões

- **Fatos são trabalho exclusivo do agente, nunca do usuário.**
  - Antes de formular perguntas, inspecione assinaturas, tipos e pacotes com `serena:get_symbols_overview` e `serena:find_symbol`.
  - NUNCA pergunte ao usuário nada que possa ser inspecionado no código, repositório ou documentação.
- **Decisões pertencem ao usuário.**
  - O usuário decide objetivos de negócio, limites de escopo, prioridades e concessões arquiteturais.

## 3. Protocolo de Interrogação por Árvore de Decisão (Grilling)

1. Mapeie as incertezas como uma **Árvore de Decisão**: cada decisão raiz ramifica nas decisões dependentes.
2. Trabalhe em **Rodadas de Fronteira (Frontier)**:
   - A fronteira reúne todas as decisões cujos pré-requisitos já foram respondidos.
   - Perguntas que dependem de uma resposta aberta pertencem a uma rodada futura.
3. Formate cada rodada com perguntas numeradas e recomendações explícitas:
   ```markdown
   ❓ **Q1** - **<Título da Decisão>**: <Contexto, trade-offs e opções>

   ➡️ <Sua resposta recomendada>

   ---

   ❓ **Q2** - **<Título da Decisão>**: <Contexto, trade-offs e opções>

   ➡️ <Sua resposta recomendada>
   ```
4. **Redija cada pergunta de modo que responder "sim" aprove a sua recomendação.**
5. A cada resposta do usuário, recompute a fronteira da árvore.
6. **Critério de Parada:** A sessão termina quando a fronteira estiver vazia (todos os ramos percorridos, nada deixado subentendido) e o usuário confirmar o alinhamento.

## 4. Modelagem DDD e Linguagem Ubíqua (`.forge/GLOSSARY.md`)

1. **Desafie termos vagos ou ambíguos em tempo real:**
   - Se o usuário usar termos polissêmicos ou concorrentes, confronte imediatamente: *"Você disse 'conta', mas no domínio temos 'Tenant' e 'Perfil'. A qual conceito se refere?"*
2. **Atualização Inline em `.forge/GLOSSARY.md`:**
   - No momento exato em que um termo for acordado, registre-o no `.forge/GLOSSARY.md`.
   - Formato obrigatório:
     ```markdown
     **NomeDoTermo**:
     Definição concisa de 1 ou 2 frases sobre o que o termo É (não o que faz tecnicamente).
     _Avoid_: Sinônimo1, SinônimoVago
     ```
   - O glossário deve conter exclusivamente termos de domínio (proibido registrar detalhes mecânicos de código).
3. **ADRs em `.forge/adr/` (Apenas quando Justificado):**
   - Registre um ADR enxuto (`0001-<slug>.md`) apenas se a decisão for:
     1. Difícil de reverter;
     2. Surpreendente para leitores futuros;
     3. Fruto de um trade-off real entre alternativas.

## 5. Síntese da Spec e Validação do Gate 1

1. Escreva ou atualize `.forge/specs/01-<slug>.md` (estritamente entre **15 e 120 linhas**) contendo:
   - `## Objetivo` (máx. 3 linhas)
   - `## Contratos e Assinaturas` (utilizando os termos canônicos de `.forge/GLOSSARY.md`)
   - `## Casos de Borda` (cenários críticos de falha, limites e validações)
   - `## Fora de Escopo (YAGNI)` (o que NÃO será construído)
2. Certifique-se de que o `.forge/GLOSSARY.md` possui os termos populados.
3. Avance executando no terminal: `forge stage plan` (o Forge validará deterministicamente o Gate 1).
