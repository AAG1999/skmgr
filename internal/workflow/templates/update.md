Update the skill the user named, or every installed skill when they named none. Show the dry-run plan, then run the update in the same turn:

```bash
skmgr update --dry-run --json
skmgr update <name> --dry-run --json
```

```bash
skmgr update
skmgr update <name>
```

Stop and ask before the real update only when the plan changes a skill the user did not name.

If the user asked to leave the declared range, use `--latest`. Show that plan and wait for confirmation before writing, because it can move a skill past its declared ref:

```bash
skmgr update --latest --dry-run --json
skmgr update --latest
```

This refreshes refs and skmgr.lock. It does not refresh workflow files. Tell the user to run skmgr init again for that.
