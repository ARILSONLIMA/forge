---
name: forge
description: Orquestrador determinístico Forge - gerencia estágios, gates, status e comandos do ciclo de vida
---

# Forge CLI - Deterministic AI Engineering Harness

Orquestrador determinístico de ciclo de vida de software para agentes de IA (Google Antigravity CLI / `agy`).

## Comandos Principais

- `forge status`: Visualizar estágio atual, bead ativo, gates e integridade
- `forge briefing`: Obter resumo de contexto ultra-enxuto (<120 tokens)
- `forge gate`: Executar todos os portões de qualidade determinísticos
- `forge stage <nome>`: Transicionar para um estágio (`spec`, `plan`, `build`, `test`, `review`, `simplify`, `ship`)
- `forge doctor`: Diagnosticar saúde das ferramentas e ambiente
- `forge stats`: Ver telemetria de economia de tokens
- `forge checkpoint`: Criar micro-checkpoint Git
- `forge rollback`: Desfazer alterações com segurança

## 7 Estágios de Engenharia (Slash Commands)

- `/spec`: Inicia alinhando contratos e criando a especificação
- `/plan`: Decompõe a spec em tarefas no Beads (`bd`)
- `/build`: Implementação cirúrgica com edição semântica (LSP/Serena)
- `/test`: Testes automatizados com filtro RTK
- `/review`: Auditoria de diff e verificação AST
- `/simplify`: Poda YAGNI e remoção de código morto
- `/ship`: Gates determinísticos finais e encerramento
