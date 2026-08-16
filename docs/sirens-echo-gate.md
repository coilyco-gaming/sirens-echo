# The pre-push gate

`just gate` runs everything CI runs, in CI's order:

```
build  ->  policy-check  ->  vet  ->  test  ->  test-skips  ->  pre-commit
```

It exists because five separate habits is four too many. `vet`, `test`, and
`build` all pass on a tree that CI rejects, so an engineer who runs the obvious
verbs still learns about a violation from a red `main`.

## Why pre-commit runs last

Its hooks are fixers. `trailing-whitespace`, `end-of-file-fixer`, `go-mod-tidy`
and `gofmt` rewrite files, so running it before the other steps would check a
tree that no longer exists by the time they finish.

## What it does not do

It does not stand in for the commit hook. `git commit` does not invoke
`pre-commit` unless someone has run `pre-commit install`, and this repository is
worked from task-scoped temporary clones that start without hooks.

`just setup` installs them, and `scripts/task.sh` installs a
missing hook before running any verb it carries, so the daily loop of `vet`,
`test`, and `tidy` each leave the commit gate armed behind them on a fresh
clone. Recipes that name their tool directly in the justfile, the snapshot
and regeneration checks among them, never reach that code. Whether a `pre-push`
hook should close the remainder is open on issue 305.

It also does not run the live evaluation cadence, which needs Agent Proxy and a
model. Those are `eval-echo`, `eval-deep`, `board-deep` and the rate pack, and
they gate nothing.

## Editing the gate itself

The gate runs through `scripts/task.sh`, so editing it means running
`bash scripts/task.sh gate` directly rather than through `just`. Editing the
gate itself means running `bash scripts/task.sh gate` directly to check
the change. Ordinary work is unaffected, since a dirty Go file is not a dirty
verb definition.

## A file git has never seen

Several hooks enumerate git's own file list rather than the paths pre-commit
hands them, so `--all-files` means every *tracked* file. A file that has never
been added is invisible to them, which made the gate green on a tree the commit
then rejected. The new file is the most likely thing in a change to carry a
fresh violation, so this was the case the gate most needed to cover and the one
it silently did not.

The gate therefore marks every untracked, non-ignored file with
`git add --intent-to-add` before the hooks run. Intent only: no content is
staged. The marks are removed however the run exits, including the failure exit,
so a red gate leaves the index exactly as it was found.

Measured at no cost: the hook pass takes the same time either way, because the
work is per-file and the file count barely moves.

## See also

- [features and release tooling](features-release-tooling.md) - what CI runs.
- [justfile](../justfile) - every recipe.
