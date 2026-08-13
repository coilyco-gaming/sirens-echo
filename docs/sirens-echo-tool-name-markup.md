# Tool-name markup

The second half of [tool-call markup](sirens-echo-tool-call-markup.md).
`toolNameMarkupFailures` matches the case's `required_tool`, qualified and bare, so
`<create_issue>` is a finding when the case declares `forgejo__create_issue`.

## Why key on the tool name

The delimiter set is a closed list of names taken from published formats. The model
does not use those: it builds the tag from **the tool's own name**. A tool name is a
value from configuration rather than a word from a vocabulary, which is why
`checkPrincipalEcho` survives translation while English word lists do not
([sirens-echo#253](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/253)).

## Measured

On the live English and French action probes, 7 markup replies and 5 clean:

| Checks | Markup caught | False positives |
| --- | --- | --- |
| delimiter set alone | 3 of 7 | 0 of 5 |
| delimiter set + name key | **6 of 7** | **0 of 5** |

The three it adds are bare `<create_issue>` tags carrying no `name=` attribute, so
the two checks are complementary rather than overlapping.

## It cannot reach the reply path

`ValidateNoToolCallMarkup` takes only a reply, so it has no case and therefore no
declared tool. **This shape was chosen for that reason.** Widening the shared
delimiter set makes production refuse more, and a refusal has no repair loop, so a
leak becomes a silence. Keying on a value the reply path does not have removes that
coupling by construction.

## Accepted misses, both pinned by test

**A case declaring no `required_tool` is not covered.** That includes
`no-invented-surface`, the case this defect was first found on. The general form
still wants the shared set widened, which still wants a repair loop first.

**An aliased tool in tag content escapes.** `<tool_uri> <tool>issue-create</tool>`
puts the tool name in the *content* under a name the roster never used, so no
name-keyed pattern reaches it. `TestToolNameMarkupMissesAnAliasedToolInTagContent`
fails when that changes.

## Opt-in

It runs only under `forbid_tool_call_markup`, alongside the delimiter set, and only
when the case declares a tool. So it adds no findings to any case that did not
already ask for this class of check.
