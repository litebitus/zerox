# Changelog

## Unreleased

- Templates
  - Read at any ref git resolves: a commit, a tag or a branch.
- Configuration
  - The schema defines every JSON Schema keyword an input's schema holds, and refuses any other key.
  - The inputs are read as JSON Schema draft 2020-12, and an `$schema` they state names that draft.

## v0.0.2 — 2026-11-26

- Commands
  - `--force`, a flag of every command, does what a command refuses to do by default, after a warning and a confirmation: `generate` and `update` take over each file zerox did not write.
  - `--yes`, a flag of every command, answers yes to every prompt a command asks.
- Configuration
  - A target's `zerox.yml` opens with a modeline naming its schema and a line saying zerox generated it, not to be edited.
- Client packages
  - The Go client answers values that break the inputs' schema as `InputProblems`, each `InputProblem` at the path of the input it is about.

## v0.0.1 — 2026-11-25

- Commands
  - `zerox generate`: renders a template set into a new target, from the values given and the defaults.
  - `zerox update`: renders a target again from its `zerox.yml`, with its values edited or from newer templates.
  - `zerox completion`: writes the `bash`, `fish`, `powershell` or `zsh` completion script.
- Templates
  - Read from git at a commit, a release or a floating major, never the working tree.
  - Go's `text/template`, with the delimiters the templates' `zerox.yml` states.
  - A file updated out of band is left alone, as its fingerprint shows.
- Configuration
  - `schemas/zerox-v1.0.schema.json`, attached to each release with `zerox-v1.schema.json` as its newest 1.x.
  - The inputs are a JSON Schema with a `tags` keyword, and a common config merged under each set's `zerox.yml`.
  - `yaml-dsl.yml`, the config files as a YAML DSL.
- Client packages
  - The Go client `github.com/litebitus/zerox/client/go` reads the config files, lists the inputs with their tags and checks values.
- Install
  - `brew install litebitus/tap/zerox`, a Homebrew cask each release pushes to `litebitus/homebrew-tap`, with the completion scripts.
