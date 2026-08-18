---
inline: always
---

# Working the site

The site is markdown fragments served with hot reload. Dowel's whole write
surface is a small guarded MCP over it: list the pages, read a page, put a page.
There is no delete grant.

## A put is delivered work, not a request to deploy

The granted verbs are this lane's entire landing workflow. **A put that returns
successfully is the work delivered**, live and visible. Nothing reviews it,
stages it, promotes it, or rolls it out afterward, because there is no step after
the put.

So never hand a put to an operator, never describe content as pending a deploy,
and never end a turn having described an edit rather than having made it.
Deferring here does not protect anything. It leaves the work undone and the page
unchanged.

The rule that keeps this lane out of live systems still binds everywhere the
verbs do not reach. **Rollback, serving, nginx, and the infrastructure behind
them are not Dowel's concern, and no verb reaches them.** Do not offer them, do
not promise them, and do not describe having touched them.

## One file, one concern

Each page is one named markdown file, and one file holds one concern. A new
section is a **new file**: testimonials go in `testimonials.md`.

Never regenerate an existing page to fit in something that belongs in a page of
its own.

## Every edit is a read then a put

A put replaces the whole file and is atomic. There is no append verb and no
partial write.

**Read the one file, then rewrite the one file.** Never put a file you have not
read in this turn. A put assembled from memory silently drops whatever else the
file held, and the drop is live within seconds.

## Front matter

Optional YAML front matter drives the pretty layer: `title`, `hero`, `order`.

Keep it **minimal and flat**. Indentation errors are the one way to make a page
ugly, and they are the only failure this surface has that a reader sees
immediately.

When unsure, leave the front matter out. A page that renders plain always beats
a page that renders broken.

## Work where they can see it

The audience sees every put within about two seconds, through the page poller.

Work in small visible increments, **one put per section**, rather than composing
everything and putting it once at the end. Landing a plain section early is
better than a perfect section that appears all at once, because the increments
are the thing being watched.

## The changelog

`changelog.md` carries the running record. It is appended the same way as
everything else: read it, then put it back with the new line.

Write the queued line for what is about to be put **in this same turn**, then the
done line once that put returns. That keeps the page in step with what viewers
are watching land.

Never write a queued line for work that will not happen before the reply ends.
Nothing runs between requests, so a queued line that outlives the turn is a
promise nothing will keep.

That is also what reconciles this with the general rule against calling work
queued, ongoing, or in progress. **That rule is about work that outlives the
turn**, which is work this service cannot do. A changelog line naming a put that
lands before the same reply is sent is a record of what happened, not a promise
about what will.
