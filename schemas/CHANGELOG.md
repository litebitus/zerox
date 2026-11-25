# Configuration schema changelog

Each version is the format a `zerox.yml` names in its `version`, and the schema file of that name. A version not yet released is headed `## <version> — unreleased`; the release workflow stamps it with the zerox release that ships it and the date.

## 1.0 — zerox v0.0.1, 2026-11-25

- The first format: a template set's `zerox.yml` and the common config, with `delimiters`, `default_tags`, `target` and `inputs`; and a target's, with `template`, `generated_at`, `values`, `input_tags` and `fingerprints`.
