#!/usr/bin/env bash
set -euo pipefail

case "${1:-}" in
  build)
    mkdir -p bin
    go build -o bin/sirens-echo ./cmd/sirens-echo
    go build -o bin/sirens-echo-policy-check ./cmd/sirens-echo-policy-check
    go build -o bin/sirens-echo-eval ./cmd/sirens-echo-eval
    ;;
  compose-bundles)
    # Local runs need a catalogue checkout; the image build clones a pinned ref.
    catalog=${AOS_CATALOG:-$HOME/projects/coilyco-flight-deck/agentic-os}
    if [ ! -d "$catalog/.agents/composed" ]; then
      echo "compose-bundles: set AOS_CATALOG to an agentic-os checkout" >&2
      exit 1
    fi
    bash scripts/stage-compose-sources.sh agent/bundles "$catalog"
    ;;
  policy-check)
    go run ./cmd/sirens-echo-policy-check
    ;;
  test-skips)
    # A skip and a pass share an exit code and the word ok, so a guard can stop
    # running for months. See docs/sirens-echo-test-skips.md.
    allow=.ward/test-skips.allow
    fired=$(go test -v ./... 2>&1 |
      sed -n 's/^ *--- SKIP: \([A-Za-z0-9_]*\).*/\1/p' | sort -u)
    expected=$(sed -e 's/#.*//' -e 's/[[:space:]]//g' "$allow" |
      grep -v '^$' | sort -u)
    unexpected=$(comm -23 <(printf '%s\n' "$fired" | grep -v '^$') \
      <(printf '%s\n' "$expected" | grep -v '^$'))
    stale=$(comm -13 <(printf '%s\n' "$fired" | grep -v '^$') \
      <(printf '%s\n' "$expected" | grep -v '^$'))
    status=0
    if [ -n "$unexpected" ]; then
      echo "test-skips: these tests skipped and are not reviewed:" >&2
      printf '  %s\n' $unexpected >&2
      echo "Fix the test, or add it to $allow with the reason." >&2
      status=1
    fi
    # A stale entry is the same defect pointed the other way: it reads as a
    # known exception and nobody deletes it.
    if [ -n "$stale" ]; then
      echo "test-skips: these are allowlisted but no longer skip:" >&2
      printf '  %s\n' $stale >&2
      echo "Delete them from $allow." >&2
      status=1
    fi
    [ "$status" -eq 0 ] && echo "test-skips: reviewed skip set matches"
    exit "$status"
    ;;
  run-echo)
    go run ./cmd/sirens-echo
    ;;
  eval-echo)
    go run ./cmd/sirens-echo-eval
    ;;
  eval-deep)
    SIRENS_ECHO_DEFINITION=agent/sirens-deep.yaml \
      SIRENS_ECHO_EVALUATION_PACK=agent/evaluation-deep.yaml \
      go run ./cmd/sirens-echo-eval
    ;;
  board-deep)
    # Emits an annotation dataset on stdout and reports no verdict. Redirect it
    # to evaluations/ before grading, because the dataset is the evidence.
    SIRENS_ECHO_DEFINITION=agent/sirens-deep.yaml \
      SIRENS_ECHO_EVALUATION_PACK=agent/board-deep.yaml \
      go run ./cmd/sirens-echo-eval
    ;;
  rate-echo)
    # Emits a measurement dataset on stdout. Redirect it to evaluations/ before
    # reading, because every reply in it is the evidence a failure was real.
    SIRENS_ECHO_EVALUATION_PACK=agent/rate-echo.yaml \
      go run ./cmd/sirens-echo-eval
    ;;
  rate-deep)
    # Emits a measurement dataset on stdout. Redirect it to evaluations/ before
    # reading, because every reply in it is the evidence a failure was real.
    SIRENS_ECHO_DEFINITION=agent/sirens-deep.yaml \
      SIRENS_ECHO_EVALUATION_PACK=agent/rate-deep.yaml \
      go run ./cmd/sirens-echo-eval
    ;;
  rate-fixture-deep)
    # The data-borne injection pack. SIRENS_ECHO_TOOL_FIXTURE is exclusive with
    # the MCP roster, so this runs separately from rate-deep.
    SIRENS_ECHO_DEFINITION=agent/sirens-deep.yaml \
      SIRENS_ECHO_EVALUATION_PACK=agent/rate-fixture-deep.yaml \
      SIRENS_ECHO_TOOL_FIXTURE=agent/tool-fixture-injection.yaml \
      go run ./cmd/sirens-echo-eval
    ;;
  format)
    find cmd internal -type f -name '*.go' -exec gofmt -w {} +
    ;;
  *)
    echo "unknown Ward action: ${1:-}" >&2
    exit 2
    ;;
esac
