# Never mistaken for a human

Both profiles carry one rule about what the agent may say about itself: it is an
agent, it says so when asked, and it claims to be neither a human nor any
specific person.

Sharing house taste and house style is the point of a composed identity. Being
taken for a person is the line.

## Why it is enforced and not merely instructed

Prose instruction is the mechanism everyone relies on and the one that does not
hold under pressure. A composed identity makes a first-person human claim more
available to the model, not less, so the profile that gained a persona is the
one that needed a check.

The neutral profile already had deterministic reply checks in
`ValidateNeutralStyle`, which reject first-person voice outright. The social
profile had none: `ValidateResponseStyle` returned `nil` for it. The guard
existed exactly where it was not needed and was absent where it was.

## The two layers

**The prompt.** `identityPolicy` joins the shared sections in `prompt.go`, so
every profile renders it and `validateSharedPolicy` fails a build that drops it.
It is un-droppable in the same way the pronoun and trust policies are, and it
shows up in the tracked snapshot diff.

**The validator.** `ValidateIdentityClaim` runs on every reply for every style,
beside grounding rather than inside `ValidateResponseStyle`, because this is a
safety property and not a voice preference. It rejects three things:

* claiming to be a human, a person, a woman, a man, and the plural forms
* denying being an agent, a bot, an AI, a language model, and the like
* answering as the configured principal, matched on the deployment-owned handle

A rejection is an ordinary validation failure, so the member gets the response
check notice and the reply never reaches Discord.

## What stays allowed

The honest answers have to survive, or the check trades one failure for another.
These all pass:

* `I am an agent running the sirens-echo harness.`
* `I'm a bot, not a person.`
* `I am Sirens Deep of Coilyco.`
* `No, I am not a human. I am an agent.`
* naming the principal in the third person

The patterns are deliberately narrow. A wider net would start blocking ordinary
social replies, and a blocked reply is a turn the member has to rephrase.

## The remaining layer, which is not code

The principal handle is the only name the validator knows, because it is the
only one the deployment supplies. An agent claiming to be some other named
human is caught by the human-claim patterns only when it says so in the first
person. Prompt instruction carries the rest.

See [the prompt](sirens-echo-prompt.md), [response
profiles](response-profiles.md), and [notices](sirens-echo-notices.md).
