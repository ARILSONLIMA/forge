# 06 - Schemas de Configuração, MCP e Regras ast-grep (embed.FS)

Este documento contém o conteúdo integral dos arquivos estáticos que o binário `forge` deve embutir via `embed.FS` no pacote `internal/templates` e gerar durante a execução do comando `forge init`.

## 1. Schema do `.forge/config.yaml`

O `forge init` deve detectar a linguagem raiz do repositório e gerar o `.forge/config.yaml` adequado:

```yaml
# .forge/config.yaml
version: "1"
language: "go" # Opções: go, typescript, python, rust

commands:
  build: "go build ./..."
  test: "go test -v ./..."
  lint: "golangci-lint run"

gate_thresholds:
  max_diff_lines_per_ticket: 300
  forbid_unreferenced_exports: true
  ast_grep_strict: true

paths:
  specs_dir: ".forge/specs"
  rules_dir: ".forge/rules"
```

### Perfis de Detecção Automática (Fallback Defaults):

- **Go** (se `go.mod` existir):

  - `build`: `go build ./...`

  - `test`: `go test ./...`

  - `lint`: `golangci-lint run` ou `go vet ./...`

- **TypeScript / Node** (se `package.json` existir):

  - `build`: `npm run build --if-present`

  - `test`: `npm test`

  - `lint`: `npm run lint --if-present`

- **Python** (se `pyproject.toml` ou `requirements.txt` existir):

  - `build`: `python -m py_compile $(git ls-files '*.py')`

  - `test`: `pytest`

  - `lint`: `ruff check .`

- **Rust** (se `Cargo.toml` existir):

  - `build`: `cargo check`

  - `test`: `cargo test`

  - `lint`: `cargo clippy -- -D warnings`

## 2. Configuração do Servidor MCP Serena (`.mcp.json`)

Gerado na raiz do repositório pelo `forge init` para disponibilizar os LSPs oficiais ao Antigravity:

JSON

```
{
  "mcpServers": {
    "serena": {
      "command": "uvx",
      "args": [
        "--from",
        "git+[https://github.com/oraios/serena.git](https://github.com/oraios/serena.git)",
        "serena",
        "start",
        "--enable-lsp"
      ],
      "env": {
        "SERENA_LOG_LEVEL": "WARN"
      }
    }
  }
}
```

## 3. Regras Nativas do `ast-grep` (`.forge/rules/`)

Regras que o `forge` injeta no disco para serem executadas via `sg scan --rule <arquivo>` durante os Gates 5 (`REVIEW`) e 6 (`SIMPLIFY`):

### Arquivo: `.forge/rules/no-empty-catch.yml`

YAML

```
id: no-empty-catch
message: "Blocos de erro vazios ou silenciosos são proibidos pelos portões do Forge."
severity: error
languages: [go, typescript, javascript, python]
rule:
  any:
    # Go: erro descartado com _ sem tratamento
    - pattern: '$VAL, _ := $FUNC($$$ARGS)' # TypeScript/JS: catch sem corpo de tratamento - pattern: \vert{} try { $$$BODY
        } catch ($ERR) { } # Python: except pass sem log ou re-raise - pattern: \vert{} try: $$$BODY
        except:
            pass
```

### Arquivo: `.forge/rules/no-debug-artifacts.yml`

YAML

```
id: no-debug-artifacts
message: "Artefatos de debug temporários ou TODOs sem vínculo com Beads são proibidos antes do Ship."
severity: error
languages: [go, typescript, javascript, python]
rule:
  any:
    - pattern: 'fmt.Println($$$ARGS)' - pattern: 'console.log($$$ARGS)' - pattern: 'print($$$ARGS)'
    - pattern: 'debugger'
```

### Arquivo: `.forge/rules/complexity-guard.yml`

YAML

```
id: complexity-guard
message: "Função com excesso de aninhamento condicional. Simplifique com early return no estágio SIMPLIFY."
severity: warning
languages: [go, typescript, javascript, python]
rule:
  pattern: |    if ($C1) {      if ($C2) {        if ($C3) {$$$BODY        }      }    }
```
