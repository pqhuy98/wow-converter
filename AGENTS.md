# Project instructions

The authoritative guidance is in the existing Cursor files linked below.
Their frontmatter is reproduced verbatim, including each full description.
Before work, read and follow every rule with `alwaysApply: true`. Read other
rules when their description or `globs` matches the task. Read a relevant skill's
`SKILL.md` in full before using its workflow; respect `disable-model-invocation`.
Resolve skill references from the skill's directory. All links below are relative
to this repository root.

When updating guidance, edit the authoritative Cursor file and keep its metadata
in this index an exact match. Do not shorten or paraphrase descriptions.

## Cursor rules

- [.cursor/rules/development.mdc](.cursor/rules/development.mdc)

  ```yaml
  description: AI agent development rules for wow-converter
  alwaysApply: true
  ```

- [.cursor/rules/snapshot-tests.mdc](.cursor/rules/snapshot-tests.mdc)

  ```yaml
  description: Visual snapshot catalog, runner, and review.html workflow
  globs: tests/snapshot-tests/**
  alwaysApply: false
  ```

- [.cursor/rules/golang.mdc](.cursor/rules/golang.mdc)

  ```yaml
  description: Go ownership, deterministic selection, metadata isolation, and cancellation invariants. Read when editing or debugging Go code.
  globs: golang/**/*.go
  alwaysApply: false
  ```

- [.cursor/rules/ponytail.mdc](.cursor/rules/ponytail.mdc)

  ```yaml
  description: Ponytail, lazy senior dev mode. Always pick the simplest solution that works.
  alwaysApply: true
  ```

## Cursor skills

- [.cursor/skills/debug-wowhead-mismatch/SKILL.md](.cursor/skills/debug-wowhead-mismatch/SKILL.md)

  ```yaml
  name: debug-wowhead-mismatch
  description: Diagnose and fix a reported visual mismatch between a Wowhead model and its wow-converter export.
  ```

- [.cursor/skills/shot-export-wow-converter/SKILL.md](.cursor/skills/shot-export-wow-converter/SKILL.md)

  ```yaml
  name: shot-export-wow-converter
  description: Capture an exported WC3 model, including paired Wowhead comparisons, when visual verification is requested or needed.
  ```

- [.cursor/skills/shot-export-wowhead/SKILL.md](.cursor/skills/shot-export-wowhead/SKILL.md)

  ```yaml
  name: shot-export-wowhead
  description: Capture a Wowhead model-viewer canvas when a source screenshot or visual inspection is requested or needed.
  ```

## Directory guidance

- Work under `deploy/vps/`: also read and follow [deploy/vps/AGENTS.md](deploy/vps/AGENTS.md).

## Other Cursor configuration

- For development environment setup, consult
  [Cursor environment](.cursor/environment.json) for install and terminal commands.
- Snapshot review writes `tests/snapshot-tests/model/review.html` (wowhead |
  expected | actual). Rebuild with `bun tests/snapshot-tests/model/write-review.ts`.
  Full workflow is in [.cursor/rules/snapshot-tests.mdc](.cursor/rules/snapshot-tests.mdc).
