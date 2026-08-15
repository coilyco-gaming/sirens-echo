# Grounding a channel name

The grounding check rejects a `#channel` the reply invented. Its allowlist was
built from supplied context alone, and that source is near enough to empty for
any channel a turn actually reached.

`restrict` cannot glob Discord snowflakes and the snowflakes share no prefix, so
the deploy guardfiles fix each channel into its own tool. The consequence is
that a channel's name lives in the tool name, `list_general-message`, and in the
tool result, and never in supplied context. A turn that read `#general` and said
so was refused for inventing it. Echo has the same shape with her `eco-`
channels, so this is structural rather than one lane's problem.

## Reading the tool name, not the tool result

The allowlist now also reads the completed tool names, the `-message` reads the
guardfiles produce. The two available sources are not equivalent and only one is
safe to widen a hallucination guard with.

A tool name is deploy-authored, reviewed, and fixed at image build, so it adds
no input anyone outside review controls. A tool result is community text, so a
member posting `#nonexistent` would enter it into the allowlist and teach the
check to accept a channel nobody has. Results stay out.

Outcome is deliberately not consulted. A failed call still proves the channel
exists, and a reply reporting that failure names it correctly.

## What the refusal costs

The check refuses a whole message, so one channel mention discards every other
paragraph. A roster summary naming `#general` in one line lost eleven correct
ones, two of them live outage findings that reached nobody. Sirens Deep runs a
single execution slot, so the 51 seconds it burned also timed out the member
queued behind it.

That is why under-firing is the right direction here, matching the polarity
argument in [the filing-claim rules](sirens-echo-grounding.md).

## See also

* [Grounding a filing claim](sirens-echo-grounding.md) - the other three rules.
* [Grounding corpus](sirens-echo-grounding-corpus.md) - the pinned replies.
