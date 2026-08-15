# How long a thread title may be

Fifty characters, and an over-long one is asked again rather than cut.

## The bound

`threadTitleRunes` is 50. Discord's own cap is 100 and `threadNameRunes` still
holds it, but 100 does not display whole in the thread list or in the narrow
surfaces that truncate hardest. The tighter bound is the readable one.

Kai decided 50 over the looser ~100 the original report suggested. See
sirens-echo#753.

## Regenerate, do not truncate

A trimmed title loses its subject, which is the one thing a title exists to
carry. This reads worse in a channel list than a shorter title that names the
topic:

```
Comparing current market prices for wood, stone, an...
```

So an over-length title goes back to the model, and the second request states
the limit. `threadTitleRetryPrompt` builds that sentence from
`threadTitleRunes`, so the number in the prose cannot drift from the number in
the check.

Truncation with an ellipsis, truncation at a clause boundary, and keeping the
looser 100 were all considered and rejected.

## Exactly one regeneration

If the second answer is also over, the title is hard-trimmed and the trim is
recorded as `thread.title.trimmed`. Never a loop: a title generator must not be
able to spend a turn's budget on itself.

The trim is a plain cut, not `truncateRunes`. That helper spends a rune on an
ellipsis saying it truncated, which is the thing this bound exists to avoid.

Recording it matters as much as the trim. A generator that keeps overrunning is
a prompt problem, and a silent fallback would make it invisible.

## The bound holds at creation, not only in the generator

`threadCreationName` is where a thread's name is decided, and it bounds
whatever it is handed. The generated title is one source. The other is the name
derived from the member's own message, which never went through the generator
and was previously free to reach Discord's 100.

A 100-character derived name is the same unreadable row in a thread list that
this issue was filed about, so the creation path binds both.

## What this does not touch

Existing threads. This binds creation only, and nothing renames a thread that
already exists.

A failed titling call still returns empty and the caller keeps the derived
name. Failure is not over-length, so it triggers no regeneration.

## See also

- [threads](sirens-echo-threads.md) - when a thread happens at all.
- [tuning numbers](sirens-echo-tuning.md) - where the constants live.
