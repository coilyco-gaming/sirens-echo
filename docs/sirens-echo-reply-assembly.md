# Reply assembly

Some of a reply is written by the service rather than the model. The tool
disclosure footer and the issue reference block are both appended after the
reply checks have run, because the harness wrote them and the checks exist to
police what the model wrote.

One step appends all of them, inside one transport budget.

## Why one step

Two independent appends budget against each other. The first is appended, the
second is appended inside the remaining room, and the transport cuts the tail
blindly. The tail is where both live, so the second silently truncates the
first. The failure is invisible. A reader sees a reply that looks complete, and it
lands on the turn that did the most for the member, because a turn that filed
an issue and called tools is the turn carrying both suffixes. Neither suffix
can fix it, because neither can see the total and the total is what decides
what yields. See sirens-echo#413.

## The answer is what yields

Both suffixes are service-authored, short, and bounded. The answer is none of
those, so the answer is shortened to make room rather than a suffix being
dropped for it. A transport with a ceiling declares it, and one with no
ceiling, like the HTTP turn, declares none and nothing is shortened.

## Convergence, not reservation

Suffix length is not monotone in the answer, so the natural implementation is
wrong:

```
suffixes := render(answer)
answer    = truncate(answer, limit - len(suffixes))
reply     = answer + suffixes
```

It assumes rendering against a shorter answer cannot produce a longer suffix.
Both directions break that:

- The reference block resolves short forms found in the reply. Shortening can
  remove the `#233` a link was resolving, leaving a link to something the
  member cannot see.
- The reference block skips a URL the reply already contains. Shortening can
  remove that URL, so the block is no longer suppressed and the suffix grows.

So the step re-measures. Each pass renders the suffixes unbounded against the
current answer, measures the overflow, and removes at least that much from the
answer, so references always resolve against the answer that will be sent.

Termination is by construction, since every pass removes at least the overflow
it measured and the answer is finite. An explicit pass bound guards a future
suffix that grows faster than the answer shrinks, and a test reaches it.

## When the suffixes alone do not fit

The append order is the preference order, because a cut reaches the tail first.
A reference is a link a member can act on and the footer is a record of what
ran, so the reference is appended first and the footer yields.

When the answer has yielded everything it has and the suffixes still do not fit,
the least preferred suffix is dropped whole rather than cut into a
half-rendered receipt. Below that, a reference block that cannot fit whole is
dropped entire, because a truncated URL is worse than no URL, and the receipt
that does fit remains.

A third suffix is one entry in the order. Adding it does not re-decide either
trade, which is the property the two-append shape could not offer.

## Verify at the send

Assert on the assembled string, not on a budget function. A budget can be
correct while the send is not, which is what happened to the first repair here.

## See also

See [tool call disclosure](sirens-echo-tool-disclosure.md), [knowledge gaps and
corrections](sirens-echo-issues.md), and [thread
prefill](sirens-echo-thread-prefill.md) for the suffixes this assembles.
