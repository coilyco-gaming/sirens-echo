# Reviewing a wider compile

The image bakes a bundle from the public catalogue it can reach. A layer holding
catalogues this build cannot read composes the same allowlist against all of
them, so the wider result is reviewable alongside the one the image ships.
Neither compile replaces the other.

## Running it

The image carries `sirens-echo-compose` and `scripts/stage-compose-sources.sh`,
so one container call does the whole pipeline with catalogues mounted:

```sh
docker run --rm \
  -v "$AOS":/catalogs/aos:ro \
  -v "$AOSK":/catalogs/aosk:ro \
  -v "$OUT":/out \
  -w /app \
  -e SIRENS_ECHO_COMPOSE_BIN=/usr/local/bin/sirens-echo-compose \
  --entrypoint bash "$IMAGE" \
  /app/scripts/stage-compose-sources.sh /out /catalogs/aos /catalogs/aosk
```

`--catalog` repeats, and the script forwards every catalogue it is given. The
expander reports which catalogue each admitted source came from.

## Why this rather than a second implementation

The wider compile runs this repository's expander over this repository's
`agent/compose/roles.kdl`. The allowlist, `DeniedComposedSkills`, the
pattern-matches-nothing rule, and `agent-compose verify` all still apply. The
other layer contributes catalogue checkouts and delivery, never policy.

## Two rules the wider set makes visible

A name owned by two catalogues is an error rather than a silent first-wins,
because which copy won would otherwise depend on argument order.

A pattern naming a denied source exactly is fatal: it asked for something
forbidden. A family glob that merely brushes one drops that member and prints
it. Without that split, adding a private catalogue would make every broad
pattern unusable, since `personal-preference-*` reaches the denied
`personal-preference-social` the moment the private catalogue is present.

## What it currently shows

Nothing new. Every source Deep's graph admits already lives in the public
catalogue, and the only private match is the denied one. The compile is worth
running anyway: it is the artifact that proves that statement, and it reports
the moment a graph or catalogue change makes it false.

## See also

See [composing the identity](sirens-echo-compose.md) and
[the rendered prompt](sirens-echo-prompt.md).
