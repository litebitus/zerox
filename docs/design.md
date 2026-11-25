# zerox

zerox renders a template set into a target folder with the values its command line gives, as the set's `zerox.yml` declares them. The target's own `zerox.yml` records every value applied and the templates read, so the target can be rendered again later, with its values edited or from newer templates.

zerox knows nothing of what its templates hold. A template holds as little logic as it can; what differs between targets derives from their values.

## Concepts

| Term | Meaning |
|---|---|
| template set | a folder under `zerox/` at a repository's root: its templates and its `zerox.yml` |
| common config | `zerox/zerox.yml`, merged under every template set's `zerox.yml` |
| input | a value the templates read, declared in the inputs' schema |
| tag | a word an input is annotated with; zerox resolves an input's tags, and what a tag means is its reader's to interpret |
| target | the folder a template set is rendered into |
| lineage | the template set and the commit, release or floating major its templates were read at, as `mock-set@v1` |
| fingerprint | the SHA-256 of a file's content as zerox last wrote it |

## Template sets

- A template set is `zerox/<set>/`, its name a folder name. Its `zerox.yml` is at the top and is not rendered; every other file under it is a template, rendered to the same relative path under the target.
- The templates and both config files are read from git at a ref, never from the working tree, so a lineage names exactly what was read.
- A template set is released by tagging its repository `zerox-templates/vX.Y.Z`, and the floating major `zerox-templates/vX` moves to the newest release of its major, as a GitHub Action's tags do. The tags version every set in the repository together.

## Configuration

Both config files are named `zerox.yml` and checked against zerox's JSON Schema; a target's states `template`, and a template set's and the common config never do.

### The templates' `zerox.yml`

| Field | Holds |
|---|---|
| `version` | the format the file is written in: a minor such as `"1.0"`, or a major such as `"1"` |
| `delimiters` | `left` and `right`, the text a template action starts and ends with |
| `default_tags` | the tags of an input that states none |
| `target` | the target folder's path from the repository's root, a template rendered with the values |
| `inputs` | the values' JSON Schema, draft 2020-12, an object; `tags` is a keyword of it beside JSON Schema's own |

- The common config holds what no one set owns, such as a version every set pins. It is merged under each set's `zerox.yml`: maps merge key by key, and the set's value wins anywhere else.
- After the merge, `delimiters`, `default_tags`, `target` and `inputs` are each stated.
- An input is each property of `inputs`, and each property of an input that is an object, at any depth. Its name is a letter or `_` followed by letters, digits and `_`, so a template reads it as `.name`.
- An input's `tags` is a list of words; an input without one takes `default_tags`.

```yaml
version: "1"
delimiters: {left: "[[", right: "]]"}
default_tags: [operator]
target: mocks/[[ .mock_id ]]
inputs:
  type: object
  additionalProperties: false
  required: [mock_id]
  properties:
    mock_id:
      description: The mock's id.
      type: string
      tags: [requester]
    size:
      description: The mock's size.
      type: integer
      default: 10
```

### The target's `zerox.yml`

zerox writes it at the top of the target; the target's files never include it.

| Field | Holds |
|---|---|
| `version` | the format the file is written in |
| `template` | the lineage |
| `generated_at` | when the target last changed, RFC 3339 in UTC |
| `values` | every value applied, an instance of the inputs' schema |
| `input_tags` | each input's resolved tags, keyed by its path |
| `fingerprints` | each file zerox wrote, keyed by its path from the target, and its fingerprint |

An input's path is its names from the top, joined by `.`, as `limits.size`.

```yaml
version: "1"
template: mock-set@4e436dc
generated_at: "2026-11-25T08:00:00Z"
values:
  mock_id: mock1
  size: 10
input_tags:
  mock_id: [requester]
  size: [operator]
fingerprints:
  mock.yml: 2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae
```

## Values

- `--value <path>=<value>` sets one value, its value read as YAML, so it takes any shape: `--value limits='{size: 10}'`. A later value for the same path wins.
- An input left out takes its schema's `default`. An object input left out whose properties have defaults takes those, at any depth.
- The values are checked against the inputs' schema before anything is rendered. Every problem is reported at once.

## Rendering

- A template is Go's `text/template`, with the delimiters the merged config states, framing a value and a directive alike: `[[ .mock_id ]]`, `[[ if .enabled ]]`, `[[ range .names ]]`, and `[[- ` and ` -]]` stripping the whitespace around an action. Text outside the delimiters, another tool's placeholder among it, is written as it stands.
- Its functions are `text/template`'s own.
- A value a template reads that the values lack is an error.
- The target's path is rendered first; every file is rendered before any is written, and a template that fails writes nothing.

## Commands

| Command | Does |
|---|---|
| `zerox generate <set>` | renders a template set into a new target, from the `--value`s given and the defaults |
| `zerox update <target>` | renders a target again from its `zerox.yml`, with the `--value`s given over its values |

| Flag | Holds |
|---|---|
| `--value <path>=<value>` | a value; repeatable |
| `--ref <ref>` | the ref the templates are read at: a commit, or a release or floating major, as `v1.2.3` or `v1`, which reads the tag `zerox-templates/<ref>`; `generate` reads `HEAD` without it, `update` its lineage's ref |

- `zerox --version`, or `-v`, prints zerox's version.
- zerox runs in a git repository's working tree, from any folder in it, and reads and writes paths from its root.
- A lineage records the ref as it was given; `HEAD` is recorded as its commit, abbreviated as `git rev-parse --short` abbreviates it.
- `generate` into a target that holds a `zerox.yml` renders it again when the values match those it records, so a retried request is a no-op where it is done. Different values are refused: that is an `update`.
- Each file written, unchanged, removed and left alone is reported on a line of its own, with why it was left alone or removed.
- The target's `zerox.yml` is written last, and only when a file was written or removed or what it records changed.

## Fingerprints

- A file whose content differs from its recorded fingerprint was updated out of band; zerox leaves it as it stands, and reports it, from then on.
- A file whose template is gone from the newer templates is removed when its content matches its fingerprint.
- A file present that zerox has no fingerprint of is left alone, and reported, unless its content is what zerox would write, which zerox then records; so a generate cut short is finished by its retry.

## Client packages

A client package reads zerox's config files, for a reader such as a UI that edits a target's values. Each language's is a package of its own in `client/<language>/`, released with zerox at its version; the Go client is the module `github.com/litebitus/zerox/client/go`. A client package:

- reads a template set's `zerox.yml` and the common config, from any `fs.FS`, so a reader passes the files at the ref a lineage names;
- reads a target's `zerox.yml`;
- lists the inputs, each with its path, schema, default and resolved tags;
- checks a set of values against the inputs' schema.

## DSL documents

A template set written in a YAML DSL is checked as it is rendered: each file the DSL's `yaml-dsl.yml` claims is validated against the schema its DSL names, by the [YAML DSL standard](https://github.com/litebitus/yaml-dsl-spec), and a file that fails writes nothing.

## Versions

The configuration format and zerox carry separate versions. The format's version is the schema's, `schemas/zerox-v<major>.<minor>.schema.json`, and zerox's version is the release.

| Change | Format version | zerox version |
|---|---|---|
| a breaking format change | a new major | a new major |
| an additive format change, such as a new field | a new minor | a new minor |
| zerox's code only | none | a new patch |

- A minor's schema is a file of its own, frozen once released.
- A config file states a major; it is read as that major's newest minor.
- Each release attaches every minor's schema, and each major's as its newest minor, so a modeline naming `zerox-v1.schema.json` follows the newest 1.x.
- `schemas/CHANGELOG.md` records what each minor adds and the release that first shipped it.

`yaml-dsl.yml` defines the config files as a YAML DSL, for the [YAML DSL editor](https://github.com/litebitus/vscode-yaml-dsl-editor).

## Distribution

- Binaries for Linux and macOS, `amd64` and `arm64`, attached to each release, built by [go-publisher](https://github.com/litebitus/go-publisher).
- A Homebrew cask, `zerox` in `litebitus/homebrew-tap`, pushed by each release; a pre-release, such as a snapshot, leaves the tap alone.
- The Go client is tagged `client/go/vX.Y.Z` beside each release's `vX.Y.Z`, its tagged commit holding the schemas it embeds.

## TODO

- DSL documents: validation follows schema resolution joining the YAML DSL standard's core.
