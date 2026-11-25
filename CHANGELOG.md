# Changelog

## Unreleased

- Commands
  - `zerox generate`: renders a template set into a new target, from the values given and the defaults.
  - `zerox update`: renders a target again from its `zerox.yml`, with its values edited or from newer templates.
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
  - `brew install litebitus/tap/zerox`, a Homebrew cask each release pushes to `litebitus/homebrew-tap`.
