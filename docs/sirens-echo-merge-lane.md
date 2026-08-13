# The merge lane

`ward agent director merge` refuses a pull request whose body carries no
same-repo closing reference. The reference is what makes a merge name the work
it finished.

## The accepted spellings

```
closes #324
closes coilyco-gaming/sirens-echo#324
```

`fixes` and `resolves` are equivalent. The keyword has to be followed
immediately by `#N` or `owner/repo#N`.

## A full issue URL does not satisfy it

This is the trap, because a URL is the house convention everywhere else. The
tracker guard rejects a bare hash-ref in an issue comment and requires the
canonical URL form, so the habit every agent builds is the one the merge verb
cannot read.

The two rules do not overlap. The guard governs issue comments, the merge verb
governs pull request bodies, and each wants the form the other refuses. Write
the URL in comments and the hash-ref in a pull request body.

## Partial delivery

If a pull request does not fully close its motivating issue, file the slice as
its own issue and close that. Weakening the reference to satisfy the verb turns
a merge into something that names nothing.

## Why the strictness stays

Relaxing the regex to accept URLs would remove the property that every merge
declares what it closed. That is worth more than the convenience, and this
repository already runs an every-commit-closes-an-issue rule that agrees with
it.

## See also

- [AGENTS.md](../AGENTS.md) - the lane declaration and the agent rules.
