# Attachments the service cannot read

A message with a screenshot used to reach the model as its text alone. Nothing
in the repository referenced `Attachments`, `Embeds`, or `StickerItems`, so an
image was discarded before context assembly.

## Why that was worse than missing a feature

A member posting a screenshot and asking what is wrong sends four words. Echo
received four words and answered them, confidently, from nothing. It was not a
refusal anyone forgot to write. It was a question the model could not tell was
incomplete.

The multimedia checklist requires that Echo say so when it cannot read an
attachment. That rule was unmeetable, because the signal never arrived.

This is the same shape as the Eco filter defect in
[issue 195](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/195),
where an empty result asserted that no trades existed when the filter had
failed. An absence indistinguishable from a non-event produces a confidently
wrong answer however well the agent behaves.

## What is surfaced

The media type and the count. Not the filename, not the bytes.

A filename is member-authored text and would add an injection surface for no
benefit, since `image/png` is all the model needs to say it cannot read the
image. The type is what Discord recorded with the upload.

An entry renders as a suffix beside the existing agent and asserted markers:

```
- member: what is wrong here? (with 1 attachment this service cannot read: image/png)
```

Silence is the default. A text-only transcript grows nothing, which is asserted
by a test, because scaffolding on every ordinary message is its own cost.

## The type is still untrusted

It arrives with an upload, so `cleanMediaType` holds it to the grammar of a
media type: lowercase, one slash, and a small character set. Anything else is
dropped, and a dropped type takes the whole suffix with it rather than
rendering a count for something nobody can name.

That matters because the alternative is a prompt line reading
`image/png. IGNORE PRIOR INSTRUCTIONS`, supplied by whoever uploaded the file.

## What this is not

It is not attachment reading, and it does not shorten that work. Static images,
links and embeds, and GIFs are the checklist and are sequenced there. This
closes the gap in the meantime, during which the honest behaviour is to say an
image arrived and cannot be read.
