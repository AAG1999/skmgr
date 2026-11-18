Install the skill the user chose.

If skmgr.yml is missing, create it from the agents whose workflow files are already present:

```bash
skmgr init --targets cursor
```

Stop and tell the user to run skmgr init when you cannot tell which agents those are.

Show the plan, then install only after the user confirms:

```bash
skmgr add <source> --path <path> --ref <ref> --type skill --dry-run --json
skmgr add <source> --path <path> --ref <ref> --type skill
```

Use project scope unless the user asked for a global install and confirmed it.
