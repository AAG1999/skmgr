Remove the skill the user named. If they named none and more than one is installed, list them with `skmgr list --json` and ask which one.

Show the dry-run plan, then remove in the same turn:

```bash
skmgr remove <name> --dry-run --json
skmgr remove <name>
```

Stop and ask before the real remove only when the plan removes anything besides the named skill.
