# What counts as a summon

Three things address this service in a guild: an explicit mention, a reply to
one of its own messages, and an edit that adds a mention to a message that did
not have one. A direct message is addressed to it by definition.

## Replies

A reply summons when the referenced message was authored by this service. The
Gateway payload usually carries the referenced message already, so the common
case costs no API call. When the payload carries only a reference id, the
runtime looks the message up and compares the author, and that lookup passes
through the admission gate so a busy channel cannot turn replies into a stream
of API calls.

A reply aimed at another member is not a summon, and an unresolved reference is
never guessed either way.

## Edits

Discord delivers `MESSAGE_UPDATE` under the guild messages intent the runtime
already requests, so no additional intent or permission is involved. An edit
runs the same eligibility, exchange, access, admission, and summon path a new
message runs. Three gates come first.

**A member edit, not a link preview.** Discord emits the same event when it
resolves an embed, with no member involved. Only a member edit sets
`edited_timestamp`, so an update without one is ignored. Without this gate a
message that already named the service would summon again every time Discord
finished unfurling a link in it.

**Guild only.** An update payload is partial. A missing guild id would otherwise
read as a direct message, which summons without a mention and answers to a
different policy, so an update without a guild id is ignored. A direct message
is already addressed to the service, which leaves little for an edit to add.

**An explicit mention.** A partial payload is not a sound basis for re-deriving
a reply reference, so an edit summons on a mention alone.

## Answered once

The duplicate gate keys on message id, and an edit keeps the id of the message
it edited. A message this service already answered therefore stays answered
once, however many times it is edited afterwards. Only a message it had no
reason to answer before can become a summon, which is what "newly mentioned"
means here.

## See also

See [contexts](sirens-echo-contexts.md), [the access policy](sirens-echo-access.md),
[admission control](sirens-echo-admission.md), and
[counterparts](sirens-echo-counterparts.md).
