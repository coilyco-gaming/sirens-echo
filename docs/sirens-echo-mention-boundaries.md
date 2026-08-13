# What a mention will not do

The rules that decide a name is not a person. What a mention *is* lives in
[mentions](sirens-echo-mentions.md).

## Too small, or part of something bigger

**A name shorter than three characters never resolves.** It matches too much
ordinary prose to be safe.

**A name inside a longer word is not that person.** Matching is bounded, so
`alphabet` does not reach `alpha`.

**A longer name wins over a shorter one it contains**, so a display name that
happens to start with another person's name resolves as itself.

## Prose only

**A person is named in prose, and that is what bounds this.** A name resolves
only where prose puts one: after whitespace, an opening bracket, an emphasis
mark, or the start of the reply.

That one rule is what keeps a name from matching inside a link, a code span,
Discord markup, or a dotted identifier. Each of those arrived as its own defect
and its own patch within one afternoon, and enumerating them was losing. The
sections below remain as defence in depth, and their corpora are this rule's
regression suite. See sirens-echo#494.

A link, a code span and Discord markup are carried through byte-identical,
because their contents are an address, a command or an id rather than a person.
A member called `eco`, `wiki` or `main` cannot corrupt a URL or a tool name by
being in the room. Bold, italic and quoted text is prose, so a name inside one
is still that person.

The non-prose runs are named rather than prose being named, because the miss
costs differ. An unlisted markup kind gets rewritten and someone reports it. An
unlisted prose kind silently stops reaching people, and nobody reports that.

## Dotted identifiers

**A name inside a dotted identifier is not that person either.** A host written
without a scheme is not a link by that shared definition, so the spans do not
cover it and a second rule does. A name immediately preceded by a dot is a
label. So is a name whose following run of label characters arrives at a dot
that begins another label. A trailing dot before a space or the end of the
reply is the punctuation of a sentence, so a name that ends one still resolves.
See sirens-echo#481 and sirens-echo#515.

The rule walks forward rather than reading the adjacent character, because the
**first** label of a host has nothing before it but a space, which is what
prose looks like. In `eco-app.coilysiren.me` the name `coilysiren` sits after a
dot and `eco` does not, so reading adjacency alone made the same host safe for
one member and not for another.

A hyphen joins labels, so the walk crosses one. A hyphen that arrives at no dot
is an ordinary word, and a member named `eco` is still reached in
`eco-friendly builds`. That is the case that keeps this from becoming "a name
before a hyphen never resolves".

Resolution reads every occurrence in a span rather than the first, because the
first can be a hostname label while the person is named later in the sentence.

## Once, and never nested

**Someone named four times is reached once.** A reply that mentions a person
repeatedly should notify them, not ping them per sentence. The remaining
occurrences stay as the name and still read correctly.

**An existing mention is left alone**, so nothing nests.

## See also

- [mentions](sirens-echo-mentions.md) - the roster and the delivery decision.
