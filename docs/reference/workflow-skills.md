# Workflow skills

After `skmgr` is installed, day-to-day work happens in chat. `skmgr init` writes the skills and slash commands. Restart the IDE so it loads them.

The only project file you maintain is `skmgr.yml`. `skmgr.lock` is written by the CLI. The generated `SKILL.md` and command files are the agent interface; they are not a plan, a task list, or an archive.

## Skills

| Skill | What it does |
|-------|----------------|
| `/skmgr-find-skill` | Asks what you need, searches the web, and runs `skmgr discover` only on a URL you approve. If you approved it to install and it has one skill, it hands off to the install skill without asking again. |
| `/skmgr-install-skill` | Creates `skmgr.yml` with `skmgr init --targets` when it is missing, shows the `skmgr add --dry-run` plan, and installs in the same turn. It stops only if the plan differs from what you asked for. |
| `/skmgr-update-skill` | Shows the `skmgr update --dry-run` plan and updates in the same turn, for every skill or only the names you give. It asks first only for `--latest`, which leaves the declared range, or when the plan changes a skill you did not name. It does not refresh workflow files. |
| `/skmgr-remove-skill` | Shows the `skmgr remove --dry-run` plan and removes the named skill in the same turn. It stops only if the plan removes anything else. |

A request counts as approval for the action it names, so the skills do not ask a second time. Global scope is used only when you ask for it.

## Slash commands

The slash command carries the same instructions under a shorter name, so the picker does not list one action twice.

| Agent | Find | Install | Update | Remove |
|-------|------|---------|--------|--------|
| Cursor | `/skm-find` | `/skm-install` | `/skm-update` | `/skm-remove` |
| Copilot | `/skm-find` | `/skm-install` | `/skm-update` | `/skm-remove` |
| Claude Code | `/skm:find` | `/skm:install` | `/skm:update` | `/skm:remove` |
| Gemini | `/skm:find` | `/skm:install` | `/skm:update` | `/skm:remove` |

Cursor commands live in `.cursor/commands/skm-find.md`. Claude Code uses `.claude/commands/skm/find.md`. Gemini uses `.gemini/commands/skm/find.toml`. Copilot uses `.github/prompts/skm-find.prompt.md`. Skills use the same pattern: `.cursor/skills/skmgr-find-skill/SKILL.md`, `.claude/skills/`, `.github/skills/`, and `.agents/skills/` for Gemini.

## Refreshing the files

Run `skmgr init` again. Unedited generated files are rewritten. A file you changed is kept unless you pass `--force`. Files that do not carry the skmgr marker are never overwritten. `skmgr update` is still the command that moves skill refs.
