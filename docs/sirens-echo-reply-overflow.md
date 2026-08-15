# A reply too large for one message

Large content has three paths through the harness and two of them already
refuse to destroy the remainder. An upload lands in the scratchpad and the turn
is told the path and the true size. A tool result over its cap is saved whole
and the model is told where it went. Outbound was the exception: the reply was
cut at the send budget and everything past the cut was gone.

## What happens now

A reply that does not fit is sent as a message plus the whole reply as
`reply.txt`. The message carries what fits and one line naming the file and the
byte count, so the size is visible without opening it. That line is the same
promise the other two paths already make to the model, made to the member.

The file name is fixed. Nothing member-supplied or model-supplied reaches an
attachment name, for the reason a server-supplied tool name never reaches the
filesystem as a path.

## The whole reply, not the tail

The file is the complete reply and the message is a prefix of it. Attaching
only the remainder would make a reader stitch two pieces together and would
disagree with the tool-result path, which saves the whole result rather than
the part that did not fit.

## No scratchpad is involved

The bytes are the reply the turn just composed, held in memory for the length
of the send. Nothing is written to a partition, so nothing here regresses when
the deployment mounts no scratchpad, and no quota is spent on an outbound
message.

## The cut was never a disclosure control

Every reply check runs on the untruncated text. Grounding, the response check,
the neutral rule, and the identifier refusal all pass or fail on the whole
reply before the send budget touches it. Sending the rest as a file therefore
adds nothing that was not already approved, and truncation was never the thing
keeping anything back.

## Failing soft

Three cases send the cut message exactly as before, and none of them reaches
the member as an error:

- the transport cannot carry a file, which is every non-Discord turn
- the reply is larger than the attachment bound, which is a defect rather than
  an answer
- the send budget has no room to say the file exists, which would otherwise
  send a message that is only the notice

Losing the remainder is much better than losing the answer, which is the same
trade the tool-result spill makes.

## What the attachment does not change

The message still routes exactly as it did. A turn that ran long enough already
replies in a thread, and the file rides on whatever message that resolved to
rather than picking a target of its own.

Discord does not render mentions inside an attachment, so the file carries the
answer as composed while the message carries the ids the harness resolved. An
attachment cannot ping anyone.

## What is not covered

The job-notice path truncates at the same budget and looks like the same
defect. It is not one: a job ends on one of three fixed phrases, so that cut
can never fire. Progress lines are the same shape for the same reason.

## See also

* [Reply assembly](sirens-echo-reply-assembly.md) - what shares the budget.
* [Tool results](sirens-echo-tool-results.md) - the inbound half of this trade.
* [Attachments](sirens-echo-attachments.md) - a large body arriving.
