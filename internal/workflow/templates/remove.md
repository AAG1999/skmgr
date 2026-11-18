Remove the skill the user named. If they named none and more than one is installed, ask which one. List them with:

```bash
skmgr list --json
```

Show the plan, then remove only after the user confirms:

```bash
skmgr remove <name> --dry-run --json
skmgr remove <name>
```
