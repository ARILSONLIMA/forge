---
name: spec
description: Inicia o ciclo de engenharia do Forge alinhando contratos e criando a especificação
---

# Forge: Estágio 1 - SPEC

1. Se um argumento ou número for fornecido (ex.: `/spec 1`, `/spec 01`, `/spec auth`):
   - Execute no terminal: `forge spec <argumento>` (ou `forge stage spec <argumento>`).
   - Se a especificação já existir no disco:
     - O Forge ativará o arquivo correspondente em `.forge/specs/01-*.md`.
     - Leia apenas o cabeçalho e o objetivo da spec ativa indicada em `.agent/rules/active-stage.md`.
     - Apresente um resumo executivo de no máximo 3 linhas (Objetivo, Contratos principais e Status) e pergunte se deseja avançar para `/plan` ou ajustar algo.
   - Se a especificação ainda NÃO existir:
     - O Forge consultará o `.forge/roadmap.md` e criará o esqueleto inicial com as 4 seções obrigatórias.
     - Se nem no Roadmap houver registro, pergunte em 1 linha o objetivo principal da spec e registre no roadmap.
2. Se nenhum argumento for fornecido:
   - Execute no terminal: `forge stage spec`.
   - Consulte `forge spec` para listar as especificações e o roadmap disponíveis.
3. Use a ferramenta `serena:get_symbols_overview` nos pacotes alvo para mapear tipos sem ler implementações inteiras.
4. Escreva ou refine a especificação em `.forge/specs/01-<slug>.md` (sempre prefixado com 2 dígitos: `01-`, `02-`, etc.) contendo:
   - Objetivo (máx. 3 linhas)
   - Contratos e Assinaturas
   - Casos de Borda
   - Fora de Escopo (YAGNI)
5. Avance executando no terminal: `forge stage plan`.
