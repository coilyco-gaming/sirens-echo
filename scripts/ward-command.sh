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
