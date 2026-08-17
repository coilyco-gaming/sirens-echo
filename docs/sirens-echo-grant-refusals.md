# Reporting a refusal

A grant this deployment does not hold is permanent. Retrying cannot satisfy it,
and every surface that reports one has to say so rather than leave the caller
guessing which kind of failure they hit.

## The classifier

`GrantTable.Permits` returns a `GrantDenial`, and `IsGrantDenial` is what tells
it from anything else an error path carries. Both submit surfaces call it.

## What each surface answers

**HTTP** answers `403` with `sirens_echo.jobs.not_permitted` and the phrase
`job kind is not permitted`.

Before, it fell to the `default` arm and answered `400` with
`sirens_echo.jobs.rejected` and `job could not be accepted` - the same answer an
unknown kind or a malformed body gets. A caller could not tell a request it
should fix from an authority it does not have, and `400` says the first one.

**Discord** says the caller is not permitted to start that job kind, rather than
the generic notice that reads as an invitation to try again.

`503` for a full queue is unchanged and remains the one job refusal that is this
service's fault rather than the caller's.

## What it does not change

**No retry loop was removed, because there was never one.** `Submit` already
recorded a denial before this: it writes the job, moves it to `failed` with the
outcome `not permitted`, and emits `job.denied`. sirens-echo#825 read the bare
`return err` in `permits` as a retry, and the retry it names is the caller's,
invited by a status code that said the request was wrong.

The grant model, the table, the validation, and the record a denial leaves are
all untouched. This is only how the refusal is reported.

## Why 403 leaks nothing

A principal learns only about its own grant, which it may already ask for:
`GrantedKinds` exists to answer exactly that without a refusal. The reason
string stays out of the response body, so `403` carries the fact and not the
table.

Contrast the not-found and not-owner pair, which deliberately share `404` so an
id cannot be probed for. That collapse protects other principals' records. This
one would protect nothing.

## See also

- [per-requester authority](sirens-echo-grants.md) - the grant model itself.
- [access](sirens-echo-access.md) - who may reach the agent, decided first.
