# Canonical phrases

`agent/phrases.yaml` holds the phrases the model invokes by key instead of
composing. The model writes `{{phrase:no-tool}}` and the harness renders the
tracked text in the blockquote-code form the harness already uses.

## Why a key rather than a prompt rule

A prompt rule asking for terse boundary responses is a *behaviour*. Behaviours
have to be re-verified on every model, and sycophancy erodes exactly this kind
of instruction: a member who pushes back gets a longer, softer, more
apologetic answer than the rule asked for.

A key is a lookup. It renders the same text on every model, and a model that
gets the key wrong fails loudly rather than drifting quietly. That is the same
reasoning behind the content classifier and the post-hoc claim check: put the
guarantee in the harness, not in prose the model can be argued out of.

## What the registry enforces

A phrase must survive the notice alphabet unchanged. If rendering would alter
it, the registry is refused at load rather than at reply time — a phrase that
says one thing in git and another in the channel is worse than no registry.

Keys are lowercase and hyphenated so they never need quoting and never look
like prose. Duplicates are refused, because a shadowed phrase is one nobody
can find by reading the file.

## An unknown key fails

A reply invoking a key that does not exist is an error, not a rendered marker.
`{{phrase:typo}}` reaching a member is worse than a failed turn: the failure is
recoverable through the repair loop, and the leaked marker is not recoverable
at all once it is on screen.

## Render lands before the prompt does

The reply path resolves an invocation, and nothing tells the model the syntax
exists. That ordering is deliberate. `RenderPhrases` is the only thing that
substitutes the marker, so adding the prompt half first would put the literal
text in a member's channel the first time a model used it. Render first makes
that impossible and changes nothing until the prompt half lands.

A reply carrying no invocation is returned untouched, which is every reply
today. An invocation with no registry configured fails the turn rather than
reaching a member, because a marker is not a phrase.

An invocation must be the whole reply. Surrounding whitespace is not other
text. See sirens-echo#588.

## Changing a phrase

The key stays and the text moves under it. A key is a contract with every reply
that already uses it, so renaming one is a breaking change and rewording one is
not.

Adding a phrase is a pull request, which is the point: what members read on a
boundary becomes reviewable rather than emergent.

## What this does not do

It does not force the model to use a key. That is a separate question from
whether the keys exist and render correctly, and it is where the prompt still
has a job: telling the model which keys are available and when they fit.

## See also

- [notices](sirens-echo-notices.md) - the form these render in.
- [the content gate](sirens-echo-content-gate.md) - the same
  harness-over-prompt reasoning applied to refusals.
