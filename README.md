# zerox

Renders a template set into a target folder with the values its command line gives, and records in the target's `zerox.yml` every value applied and the templates read, so the target can be rendered again with its values edited or from newer templates.

## Features

- A template set is a folder under `zerox/` at a repository's root; every file in it is a template, rendered to the same path under the target.
- Templates are Go's `text/template`, with delimiters each repository states, so another tool's placeholders pass through as written.
- The inputs are a JSON Schema: each input's description, type, allowed values and default, and its tags, a word list zerox resolves and a reader such as a UI interprets.
- A common config is merged under every set's `zerox.yml`, for what no one set owns.
- Templates are read from git at a commit, a release `zerox-templates/vX.Y.Z` or a floating major `zerox-templates/vX`, and the target records that lineage.
- A file updated out of band is left alone from then on, as its fingerprint shows; a retried generate is a no-op.
- Every problem in a config file or the values is reported at once, at its line and column.
- A Go client package reads the config files, lists the inputs with their tags and checks values.

The design, the config files and every rule are in [docs/design.md](docs/design.md).

## Install

```sh
brew install litebitus/tap/zerox
```

The cask installs completion for bash, zsh and fish. For zsh, in `~/.zshrc`:

```sh
fpath=(/opt/homebrew/share/zsh/site-functions $fpath)
autoload -Uz compinit && compinit
```

## Usage

```sh
zerox generate mock-set --value mock_id=m1 --value limits='{size: 10}'
zerox update mocks/m1 --value limits.size=20
zerox update mocks/m1 --ref v2
```

| Command | Does |
|---|---|
| `zerox generate <set>` | renders a template set into a new target |
| `zerox update <target>` | renders a target again from its `zerox.yml` |
| `zerox completion <shell>` | writes the shell's completion script: `bash`, `fish`, `powershell` or `zsh` |

The Go client package:

```sh
go get github.com/litebitus/zerox/client/go
```

## Published schemas

Every release attaches the config files' JSON Schemas as assets, for a modeline or `schema_search_paths` to name:

| Schema | Asset | In the latest release |
|---|---|---|
| a minor | `zerox-v<major>.<minor>.schema.json` | [`zerox-v1.0.schema.json`](https://github.com/litebitus/zerox/releases/latest/download/zerox-v1.0.schema.json) |
| a major, as its newest minor | `zerox-v<major>.schema.json` | [`zerox-v1.schema.json`](https://github.com/litebitus/zerox/releases/latest/download/zerox-v1.schema.json) |

[`yaml-dsl.yml`](yaml-dsl.yml) defines the config files as a YAML DSL: copied into a repo's `yaml-dsl.yml`, it lets the [YAML DSL editor](https://github.com/litebitus/vscode-yaml-dsl-editor) check a `zerox.yml` as it is written.

## Development

Go builds and tests the repo.

| Target | Does |
|---|---|
| `make build` | runs the tests, then builds `zerox` into `bin/` |
| `make test` | `go vet`, a `gofmt` check, the Go tests, and a check that the tests cover at least 90% of each Go module's code |
| `make completion` | `make build`, then installs its zsh completion into `~/.zsh/completions` (`COMPLETION_DIR` to override), for `zerox` and `./bin/zerox` |
| `make completion-clean` | removes it, so a brew-installed zerox's completion takes over |

The command tests run `zerox` in git repositories made for each test, from templates committed and released there.

### Releases

| Workflow | Does |
|---|---|
| Release | runs the tests, stamps the changelogs, tags `vX.Y.Z` and the Go client's `client/go/vX.Y.Z`, and publishes the binaries and schemas with [go-publisher](https://github.com/litebitus/go-publisher), and the cask to `litebitus/homebrew-tap` with the `HOMEBREW_TAP_TOKEN` secret |
| Snapshot | publishes a pre-release of the current commit, `vX.Y.Z-snapshot.<time>`, and its Go client tag |
