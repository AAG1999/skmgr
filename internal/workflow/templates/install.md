Install the skill the user chose, using the source, path, and ref you were given. If you were given none, run `skmgr discover <source> --json` on the approved source and use the one skill it lists.

If skmgr.yml is missing, create it from the agents whose workflow files are already present:

```bash
skmgr init --targets cursor
```

If you cannot tell which agents those are, stop and tell the user to run skmgr init.

Show the dry-run plan, then run the install in the same turn:

```bash
skmgr add <source> --path <path> --ref <ref> --type skill --dry-run --json
skmgr add <source> --path <path> --ref <ref> --type skill
```

Stop and ask before the real install only when the plan:

- adds or changes anything other than the skill the user chose,
- targets an agent that skmgr.yml does not list, or
- uses global scope when the user did not ask for it.

Otherwise install with project scope and tell the user what was installed.
