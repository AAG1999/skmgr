# Workflow skills

After `skmgr` is installed, day-to-day work happens in chat. `skmgr init` writes the skills and slash commands. Restart the IDE so it loads them.

The only project file you maintain is `skmgr.yml`. `skmgr.lock` is written by the CLI. The generated `SKILL.md` and command files are the agent interface; they are not a plan, a task list, or an archive.

## Skills

| Skill | What it does |
|-------|----------------|
| `/skmgr-find-skill` | Asks what you need, searches the web, and runs `skmgr discover` only after you approve a URL. It does not install. |
| `/skmgr-install-skill` | Creates `skmgr.yml` with `skmgr init --targets` when it is missing, shows `skmgr add --dry-run`, and installs after you confirm. |
| `/skmgr-update-skill` | Runs `skmgr update` for every skill, or only the names you give. `--latest` is used only when you ask to leave the declared range. It does not refresh workflow files. |
| `/skmgr-remove-skill` | Shows `skmgr remove --dry-run` and removes the skill after you confirm. |

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
