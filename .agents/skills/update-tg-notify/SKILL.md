---
name: update-tg-notify
description: "Re-sync the tg-notify agent skill from the current tg-notify-cli project state. Use after project changes to flags, modes, retry policy, file type tables, size limits, or configuration, when the tg-notify skill may be stale."
title: Update tg-notify Skill
version: "20260830-1"
deps-skills: ["gen-skill"]
disable-model-invocation: false
user-invocable: true
allowed-tools: ["bash", "glob", "grep", "view", "ls", "edit", "write"]
kaizen:
  sources:
    tg-notify-skill: <SKILLS_ROOT>/tg-notify/SKILL.md (resolved from crush.json skills_paths)
    gen-skill: <SKILLS_ROOT>/gen-skill/SKILL.md (resolved from crush.json skills_paths)
  targets:
    tg-notify-skill: Monitor structure changes to the skill this updater maintains
    gen-skill: Monitor frontmatter and validation rules
---

Initialize todos for update-tg-notify operation:

```json
[
  {"content": "Read project sources of truth", "status": "in_progress", "active_form": "Reading project sources of truth"},
  {"content": "Diff sources against tg-notify skill files", "status": "pending", "active_form": "Diffing sources against skill files"},
  {"content": "Apply updates to stale entries", "status": "pending", "active_form": "Applying updates"},
  {"content": "Bump version stamp and baseline, validate with gen-skill", "status": "pending", "active_form": "Bumping stamps and validating"},
  {"content": "Report sync summary", "status": "pending", "active_form": "Reporting sync summary"}
]
```

# Update tg-notify Skill

Re-sync the tg-notify agent skill (TARGET) from the tg-notify-cli
project (REPO). The tg-notify skill documents the CLI surface of the
binary; this updater keeps every documented fact aligned with the
project source after changes.

## Constants

| Name | Value | Notes |
|------|-------|-------|
| SKILLS_ROOT | derived at runtime | the shared agent-skills folder, read as the first `skills_paths` entry from `~/.config/crush/crush.json` |
| TARGET | `<SKILLS_ROOT>/tg-notify/` | tg-notify skill root, resolved at runtime so no home or absolute path is stored |
| REPO | derived at runtime | the tg-notify-cli project root, three directories above this skill file (`.agents/skills/update-tg-notify/`); no absolute project path is stored |

Every project source below is addressed relative to REPO.

## Source-to-Target Mapping

| Project source (relative to REPO) | Truth it holds | Target skill file(s) (relative to TARGET) |
|----------------|----------------|----------------------|
| `AGENTS.md` | flag summary, env vars, gotchas, mode list | `SKILL.md` (gotchas, mode table), `references/gotchas.md`, `references/guide-config.md` |
| `cmd/tg-notify/main.go` (fs.*Var block) | exact flag names, shorthand, defaults, help text | every `references/modes/mode-*.md`, `references/guide-config.md` |
| `internal/telegram/filetype.go` | extension and MIME tables, size limits | `references/guide-filetypes.md` |
| `internal/telegram/` (retry and limit code, plus `cmd/tg-notify/main.go` backoff constants) | 429 and transient constants, self-hosted limit caveat | `references/guide-retry.md`, `references/guide-filetypes.md` |
| `CHANGELOG.md` (top entries) | what changed since last sync | all files, to locate the diff |
| `git describe --tags --always` | the release tag the skill tracks | the baseline version stamp in `SKILL.md` Tool Availability and Machine Profile |
| `cmd/tg-notify/` (new mode files) | new modes | `SKILL.md` mode table plus a new `references/modes/mode-*.md` |

## Workflow (BEGIN EXECUTION IMMEDIATELY)

1. Resolve SKILLS_ROOT from the first `skills_paths` entry in
   `~/.config/crush/crush.json`, then set TARGET to
   `<SKILLS_ROOT>/tg-notify/`. Read every project source of truth from
   the mapping table, from REPO. Read the flag block in
   `cmd/tg-notify/main.go`, the tables in
   `internal/telegram/filetype.go`, the retry constants in
   `cmd/tg-notify/main.go` (retryWithBackoff and backoff schedule),
   and the top entries of `CHANGELOG.md`.
2. Read every target file under TARGET. Diff each source of truth
   against its targets and collect the stale entries: missing or
   renamed flags, changed defaults, changed tables or constants, new
   modes, removed modes.
3. If nothing is stale, report up-to-date and stop.
4. Apply updates with small targeted edits. Add a new mode file and a
   new Mode Detection row when a new mode exists. Add new gotcha
   entries with the next sequential G number.
5. Refresh the baseline version stamp in TARGET `SKILL.md` (Tool
   Availability and Machine Profile) to the current release tag from
   `git describe --tags --always`. Bump the `version` frontmatter
   field of TARGET `SKILL.md` to the execution date, incrementing the
   suffix when the date is unchanged. Bump this skill's own `version`
   field the same way when this skill changed.
6. Load gen-skill and run validation mode against TARGET. Fix every
   finding. The provisioning WARN for the tg-notify skill (no
   bootstrap script, binary comes from PATH) is an accepted deviation.
7. Output a sync summary table: source, target, change.

## Constraints

- Never invent flags that are not present in `cmd/tg-notify/main.go`.
- Never store real tokens or chat IDs in any skill file.
- Keep TARGET `SKILL.md` under 5000 tokens; move overflow into guides
  or mode files.
- Keep TARGET free of absolute project paths; the tg-notify skill
  resolves the binary from PATH only.
- Use one term per concept; keep G numbering sequential.