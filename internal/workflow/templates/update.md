Update every installed skill, or only the names the user gave.

Show the plan first:

```bash
skmgr update --dry-run --json
skmgr update <name> --dry-run --json
```

After the user confirms, run the same command without --dry-run:

```bash
skmgr update
skmgr update <name>
```

Pass --latest only when the user asks to leave the declared range, and show that plan before writing:

```bash
skmgr update --latest --dry-run --json
skmgr update --latest
```

This refreshes refs and skmgr.lock. It does not refresh workflow files. Tell the user to run skmgr init again for that.
