# The worklog element

A long turn shows what it is doing, one row per tool call, resolving in place:

```
Working on it
> ✅ `eco.get_market`
> 📭 `eco.get_stores`
> ❌ `forgejo.list_issue`
> 🔨 `eco.find_trade`
4 tools, 14 seconds elapsed
```

Direction A of the sirens-echo#111 design. You can see progress rather than
motion: resolved rows read as working, where a lone spinner reads as possibly
hung. #137 and #190 were both a live service looking dead.

## Two surfaces, one of them a fallback

An embed needs `EMBED_LINKS`, missing from every install link this estate
published, so the permission decides the surface: granted gives the embed,
absent gives the [stacked notice lines](sirens-echo-progress.md) unchanged.

The permission is read rather than assumed. A channel that reads as unknown is
treated as granted, because a direct message always reads that way: a wrong yes
costs one refused call, a wrong no strands the richer surface for a whole turn.

**A refusal degrades, it never fails.** Discord answering `50013` routes the
turn to the notice lines and latches, so a channel without the grant pays one
rejected call rather than one per beat. Posting nothing is the silence this
element exists to remove.

## The glyphs are the reaction vocabulary

🔨 invoked, ✅ ok, 📭 empty, ❌ failed. Shared with the reactions and the
disclosure footer rather than invented here, so one state model has three
renderings. An unresolved row on a stopped element keeps 🔨, carrying its
approved meaning exactly: invoked, outcome never learned. Claiming failure
there would be a claim the harness cannot support.

## The embed is a container, not an exemption

Every row is a harness notice, the same shape a member reads outside an embed.
That was Kai's decision, and it is why the alphabet gained the underscore: the
tool name is the payload, and `list_issue` sanitized to `list issue` is a name
nobody can look up. Markdown cannot act on an underscore inside a code span,
and the backtick is still stripped.

Rows cap at six with a count standing in for the rest. Arguments are never
shown: names are roster-derived, arguments carry member text.

## Terminal states

**Delivered answer** - deleted. The answer carries the disclosure footer, which
already names the tools, and two lists of them is what sirens-echo#385 avoided.

**Anything else** - resolves to `Did not finish` and stays. An element that
merely vanishes mid-narration is the #137 silence in a costume.

**A block is not tellable from any other stop.** One wording, no reason
carried. Nothing in the view takes a category, so the property is structural
rather than a convention someone has to remember. A progress surface naming the
classifier undoes sirens-echo#226 in one line.

## What this does not do

**The element does not become the answer.** The design has the embed cleared
and the same message filled with the reply. That routes delivery through the
progress message, bypassing the overflow-attachment path (sirens-echo#791) and
the thread routing, so it is its own change. Deleting on success keeps the rule
it served: progress and the footer never coexist.

**The threshold is unchanged at 5 seconds.** The design table says ~2.5s, but
`turnProgressAfter` is an operator knob with the beat and the thread threshold
derived from it, so halving it halves both. That is tuning, not this build.

## See also

- [reply progress](sirens-echo-progress.md) - the fallback surface.
- [notices](sirens-echo-notices.md) - the text contract inside the embed.
