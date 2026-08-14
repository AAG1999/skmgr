# Agents, Skills & Rules

## Skills vs rules

- **Skills (`type: skill`)**: Directories of files (e.g. `SKILL.md`, scripts, examples). Symlinked as entire directories into the target agent folders.
- **Rules (`type: rule`)**: Instruction text. Stored as a directory in `.agents/rules/<name>/`; content is read from the first of `AGENTS.md`, `SKILL.md`, `rule.md`, or `README.md` found there.

Some agents rely on a single file (e.g. `CLAUDE.md`) rather than a directory of skills. For those targets, `skmgr` merges rule content into the target file, wrapped in `<!-- skmgr:start:rule-name -->` and `<!-- skmgr:end:rule-name -->` delimiters, so manually written project instructions stay alongside managed rules.

## Agent compatibility

Paths below are for project scope. Under `scope: global` the same layout is rooted at your home directory.

| Agent Target | Skills Directory | Rules Location | Rule Strategy |
|--------------|------------------|----------------|---------------|
| `cursor` | `.cursor/skills/` | `.cursor/rules/` | Symlink |
| `gemini` (project) | reads `.agents/skills/` natively — no symlink | `.agents/AGENTS.md` | Skipped (read natively) |
| `gemini` (global) | `~/.gemini/config/skills/` | `~/.gemini/config/AGENTS.md` | Merge |
| `claude-code` | `.claude/skills/` | `.claude/CLAUDE.md` | Merge |
| `copilot` | `.github/skills/` | `.github/copilot-instructions.md` | Merge |

Gemini already reads the canonical `.agents/` store at project scope, so `skmgr` creates no links for it there.

New agents can be added by defining an `AgentDef` in the registry. See [CONTRIBUTING.md](../../CONTRIBUTING.md).
