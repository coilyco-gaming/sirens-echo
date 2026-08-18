# Canonical phrases

`agent/phrases.yaml` holds the phrases the model invokes by key instead of composing. The model writes
`{{phrase:no-tool}}` and the harness renders the tracked text in the blockquote-code form.

**A prompt rule asking for terse boundary responses is a behaviour**, and behaviours have to be
re-verified on every model, with sycophancy eroding exactly this kind of instruction: a member who
pushes back gets a longer, softer, more apologetic answer than the rule asked for. **A key is a
lookup**: it renders the same text on every model, and a model that gets the key wrong fails loudly
rather than drifting quietly. Same reasoning as the content classifier and the post-hoc claim check -
**put the guarantee in the harness, not in prose the model can be argued out of**.

**A phrase must survive the notice alphabet unchanged**, and the registry is refused at load rather than
at reply time if rendering would alter it, because **a phrase that says one thing in git and another in
the channel is worse than no registry**. Keys are lowercase and hyphenated so they never need quoting
and never look like prose, and duplicates are refused because a shadowed phrase is one nobody can find.

**An unknown key fails the turn rather than rendering a marker.** `{{phrase:typo}}` reaching a member is
worse than a failed turn: the failure is recoverable through the repair loop, **and the leaked marker is
not recoverable at all once it is on screen**. An invocation with no registry configured fails for the
same reason, and **an invocation must be the whole reply**, surrounding whitespace not being other text
(#588).

**Render lands before the prompt does.** The reply path resolves an invocation and nothing tells the
model the syntax exists, deliberately: `RenderPhrases` is the only thing that substitutes the marker, so
**adding the prompt half first would put the literal text in a member's channel the first time a model
used it**. When a registry is configured the prompt lists the **keys** and states the terminal rule, not
the texts, because **a model given the text composes with it, which is the behaviour a registry exists
to replace**. The policy is appended to the built prompt rather than written into `BuildSystemPrompt`,
so the snapshot generator and the policy check render what they rendered before.

**The key stays and the text moves under it**, so renaming one is a breaking change and rewording one is
not. Adding a phrase is a pull request, which is the point: **what members read on a boundary becomes
reviewable rather than emergent.** It does not force the model to use a key, which is a separate
question and where the prompt still has a job.

## A reaction instead of a reply

`{{react:agree}}` marks the member's own message and posts nothing. The keys are `acknowledge`,
`agree`, and `disagree`, compiled in and **keyed, not spelled as an emoji**, so no literal rune reaches
a member past the checks. The terminal and unknown-key rules hold, and **a mark is an answer**, earning
silence with no tool call.

Three cases send the glyph as text: a turn owing a receipt a mark cannot carry, a transport that cannot
mark, and a mark Discord refuses. None loses the answer.

## Counting and scoring a phrase

`sirens_echo.phrase.invocations` carries a `phrase.key` label, and an invocation also emits
`response.phrase.invoked` and sets `response.phrase` on the turn span. **The key is safe as a metric
label because the registry authors it**: every other closed-set rule in the telemetry contract exists to
keep a member value out of cardinality, **and a phrase key is the opposite of member-supplied**. What it
answers is which boundaries members actually probe, and how often. The issue recommended a tool call so
an invocation would appear as its own span; the registry that shipped implements the sentinel, **a real
divergence that costs the span** but not the anti-spoofing argument, which the terminal rule closes.

`expect_phrase` asserts which key the reply invoked, failing when the reply invoked nothing, a different
key, more than one, or the right key beside other text. **This is the replacement for frozen keyword
lists**: a keyword list can be fitted to outputs after seeing them, **and a key is exact with nothing to
fit**. The check reads the raw reply, before rendering, because rendering replaces the key with its
text. **Nothing sets `SIRENS_ECHO_PHRASES` yet**, so every phrase path is inert.

## Object emoji in a neutral reply

The neutral profile refused every emoji. It now admits an emoji that **names a thing** and still refuses
one that carries tone, for legibility: an eye finds `Wood 🪵` in a wall of text faster than `Wood`
(#203, emphatic that this is **not for whimsy**). Refused: **emotive** (faces, people, body parts,
hands, hearts), which make a reply read as a person; **celebration** (fireworks, confetti, sparkles,
`💯`), since a party popper is an object to Unicode and tone to a reader **and the reader wins**; and
**indicators** (status dots, geometric shapes, verdict marks, arrows), because `🟢` is legible and is
not an object. Admitted: everything else, at most **three** in one reply.

**A denylist rather than an allowlist**, because an allowlist of object ranges would refuse any object
emoji nobody thought of, and a style refusal costs a repair attempt and then the turn, **and wrongly
refusing a correct reply is the more expensive mistake**. The model is steered toward objects by the
object table rather than by the check: **the check is a floor against tone, not the taste.**

## What a reply in another language is still checked for

**Every reply validator is either a list of English words or a match on a value.** A translation changes
nothing about the second kind and defeats the first entirely, so a non-English reply ships with part of
its guarantees **and no sign of which part** (issue 253). Whether the service should answer in another
language is issue 298: this is the price list for it.

**Surviving a translation**: `ValidateNoToolCallMarkup` matches delimiters rather than words;
`checkHandleEcho` and `checkUserIDEcho` match by value, the latter across digits and encodings;
`ValidateGrounding`'s invented-channel half matches a `#token`; and `ValidateNeutralStyle`'s emoji and
exclamation halves read characters. **That is most of the security-relevant set**, because a principal
disclosure, a leaked ID, and unparsed tool-call markup were never detected by reading English.

**Lapsing**: every `ValidateGrounding` action claim, so an invented filing is not caught;
`ValidateSelfAttributedClaim`; `ValidateIdentityClaim`, claiming to be a person or denying being an
agent; `ValidateNeutralStyle` on first person, social openings, and personality framing; and
`checkUserIDEcho`'s spelled-digit evasion, which reads English number words. **The grounding row is the
serious one**, an ungrounded claim of a completed write being the defect behind issues 206, 209, and
211, **and the whole defence is a list of English verbs**. The neutral-style rows mean the profile's
promise of impersonal output **holds in English and nowhere else**.

**The fix is not more word lists.** A list per language rots the moment a language is added and nobody
remembers to extend it, **and the rot is silent, because the check passes**. Detection is worse: a
misdetected reply runs a check it should never have run, and a grounding failure routes straight to
`failTurn` with no repair, **so a false positive costs a member their answer**. The reachable direction
is what the surviving column shows - **a check anchored on a value rather than a word needs no language
work at all**. `validatorlanguage_test.go` records a reach for every reply validator and fails when a
new one is added without that decision, the surviving rows **proved by running each validator against a
French reply carrying the defect** rather than asserted, and the lapsing rows pinned the same way with
English controls.
