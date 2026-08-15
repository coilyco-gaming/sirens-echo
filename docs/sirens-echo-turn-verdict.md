# The turn event is not the send event

Two events carry `discord_failure` and only one of them is about Discord.
Reading the wrong one is what made an 18 percent delivery failure rate out of a
population that had barely any delivery failures in it.

## What went wrong

`discord.turn.failed` fires when the turn returned any error at all, and it
classified that error the same way a failed send is classified. A model stage
failure is neither a context error nor a rejection Discord answered, so it fell
through to the catch-all and reported `no_response`, the value that means the
gateway never answered.

In one 24 hour window, thirteen of fourteen classified rows were turns that
failed at a stage. None had reached a send. That is why every sample on the
series read `no_response` and not one ever read `rest_error`: the catch-all was
collecting errors that never touched Discord.

## What it does now

The turn event classifies a Discord verdict only where one exists. The reply
send is marked as it returns, and everything else reports:

```
discord_failure: not_attempted
```

so the two populations split in a group-by instead of merging. A run of
`not_attempted` says the turns died before delivery and points at the model
stage. A run of `no_response` now genuinely means the gateway went quiet.

## Which event to read

* `discord.reply.failed` fires when a send actually failed. Count delivery
  failures here, and nowhere else.
* `turn.reply.undelivered` fires when the apology for a failed send also failed.
  A member who got neither the answer nor the notice is in this one.
* `discord.turn.failed` fires when the turn failed for any reason. Its
  `discord_failure` is a delivery verdict only when it is not `not_attempted`.

## Reading a window that straddles the change

`not_attempted` is newer than the pods that predate it, so a row carrying no
`discord_failure` at all is an old image rather than a turn that classified to
nothing. That is the shape [indistinguishable
values](sirens-echo-indistinguishable-values.md) catalogues, and this was one
more instance of it.

## See also

* [Delivery failures](sirens-echo-delivery-failures.md) - what a failed send
  records and why.
* [Observability](sirens-echo-observability.md) - reading these events live.
