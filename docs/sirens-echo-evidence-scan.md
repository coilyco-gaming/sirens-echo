# Counting a behaviour in evidence we already paid for

`ward exec evidence-scan` counts tool-call markup across every committed run
record. It answers how often without a bespoke evaluation case.

## Why not a case

The behaviour runs at roughly half a percent per attempt. A rate case at the
usual fifteen runs has an expected count of 0.08, so it reads zero and
establishes nothing. Worse, it would read as *fixed* once the reply path
refuses markup, when it was never measurable at that size.

The datasets already hold hundreds of attempts. Measuring the ones we have is
cheaper than commissioning a case that cannot see the thing.

## One definition, three readers

The scanner calls `ValidateNoToolCallMarkup`, which reads the same
`toolCallMarkupPatterns` the deployment gate and the reply path read. Two
copies of one definition drift in whichever direction nobody is watching.

## It gates nothing, deliberately

The verb exits zero whatever it finds. Evidence legitimately contains markup,
which is what makes it evidence, so a check that failed on it would refuse the
thing it exists to count. `ward exec test-skips` earns a non-zero exit because
a silent skip is never legitimate. This is the opposite case.

## Two kinds of record, reported separately

The rate datasets carry per-attempt records, so they have a reply count and a
share. The `eval-deep-run*` files are free-text transcripts with no record
boundaries, so they can report that markup is present and never how often per
attempt. Summing the two into one rate would invent a denominator, so the share
covers structured records only and the transcripts are listed on their own.

## Reading a dataset committed before the stdout split

Logs and the record shared stdout until the eval runner moved its logs to
stderr, so every dataset committed before that interleaves JSON log lines with
the document and will not unmarshal. Those files stay as they are, because they
are cited evidence and rewriting them would edit the record.

This scanner reads both forms. It seeks the record rather than starting at byte
zero, so nobody counting a behaviour across evidence has to strip log lines by
hand. Reading one of those files directly still does.

## Where a zero stops meaning absence

Two limits, both worth stating with any number this produces.

A dataset that parses to no replies is a parse that found nothing rather than a
run that produced nothing. The scanner exits non-zero when no structured
dataset parsed at all, because a confident zero over an empty read is the
quietest possible wrong answer.

And the pattern set covers the delimiter syntaxes that have been measured. A
zero for a model family whose syntax was never observed says nothing about that
family. See [tool-call markup](sirens-echo-tool-call-markup.md).
