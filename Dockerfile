FROM forgejo.coilysiren.me/coilyco-flight-deck/agentic-os:release AS build

USER root
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY agent ./agent
COPY agents ./agents
COPY .agents/skills ./.agents/skills
COPY docs ./docs
ARG SIRENS_ECHO_REVISION=
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community.buildRevision=${SIRENS_ECHO_REVISION}" \
    -o /out/sirens-echo ./cmd/sirens-echo \
    && CGO_ENABLED=0 go build -trimpath -o /out/sirens-echo-policy-check ./cmd/sirens-echo-policy-check \
    && CGO_ENABLED=0 go build -trimpath -o /out/sirens-echo-compose ./cmd/sirens-echo-compose \
    && CGO_ENABLED=0 go build -trimpath -o /out/sirens-echo-prompt ./cmd/sirens-echo-prompt \
    && CGO_ENABLED=0 go build -trimpath -o /out/sirens-echo-access-check ./cmd/sirens-echo-access-check \
    && /out/sirens-echo-policy-check

# The release image ships agent-compose but not the composed catalogue, so this
# stage fetches it. The ref floats on main by design, so a rebuild takes the
# catalogue as it stands; override it to reproduce an older bundle.
# See docs/sirens-echo-compose.md.
FROM forgejo.coilysiren.me/coilyco-flight-deck/agentic-os:release AS compose
ARG AOS_CATALOG_REF=main
# The clone below caches on this instruction's text, which never changes, so the
# floating ref froze. This is the resolved commit, and it is what keys the layer.
ARG AOS_CATALOG_HEAD
USER root
WORKDIR /src
RUN set -eu; \
    if [ -z "${AOS_CATALOG_HEAD:-}" ]; then \
      echo "AOS_CATALOG_HEAD is required; resolve it with scripts/lib/catalog-head.sh" >&2; \
      exit 1; \
    fi; \
    git clone --depth 1 --branch "${AOS_CATALOG_REF}" \
      https://forgejo.coilysiren.me/coilyco-flight-deck/agentic-os.git /tmp/aos-catalog; \
    cloned=$(git -C /tmp/aos-catalog rev-parse HEAD); \
    if [ "${cloned}" != "${AOS_CATALOG_HEAD}" ]; then \
      echo "catalogue ${AOS_CATALOG_REF} cloned ${cloned}, caller resolved ${AOS_CATALOG_HEAD}" >&2; \
      exit 1; \
    fi
COPY agent ./agent
COPY agents ./agents
COPY .agents/skills ./.agents/skills
COPY scripts/stage-compose-sources.sh ./scripts/
# The expander comes from the build stage, so this stage needs no Go toolchain
# work and the binary is the one the suite already exercised.
COPY --from=build /out/sirens-echo-compose /usr/local/bin/sirens-echo-compose
COPY --from=build /out/sirens-echo-prompt /usr/local/bin/sirens-echo-prompt
RUN SIRENS_ECHO_COMPOSE_BIN=/usr/local/bin/sirens-echo-compose \
    bash scripts/stage-compose-sources.sh /out/bundles /tmp/aos-catalog
# Every baked role must render a valid composed prompt and select exactly what
# its tracked record says. See docs/sirens-echo-compose.md.
RUN sirens-echo-prompt --bundles /out/bundles --check

FROM forgejo.coilysiren.me/coilyco-flight-deck/agentic-os:release

USER root
WORKDIR /app
COPY --from=build --chown=1000:1000 /out/sirens-echo /usr/local/bin/sirens-echo
# Shipped so another layer can expand the same allowlist with catalogues this
# build cannot see. See docs/sirens-echo-compose.md.
COPY --from=build --chown=1000:1000 /out/sirens-echo-compose /usr/local/bin/sirens-echo-compose
# Deploy's CI invokes this against the ConfigMap before applying it, so it has
# to reach the released image and not only the build stage. See #628.
COPY --from=build --chown=1000:1000 /out/sirens-echo-access-check /usr/local/bin/sirens-echo-access-check
COPY --chown=1000:1000 scripts/stage-compose-sources.sh /app/scripts/stage-compose-sources.sh
COPY --chown=1000:1000 agent /app/agent
COPY --chown=1000:1000 agents /app/agents
COPY --chown=1000:1000 .agents/skills /app/.agents/skills
# A wildcard here copies each root's contents rather than the root, flattening
# the tree, and a definition naming a root then crashes at startup. deploy#666.
RUN set -eu; \
    for root in /app/.agents/skills/*/; do \
      [ -f "${root}SKILL.md" ] || { echo "skill root ${root} lost its SKILL.md" >&2; exit 1; }; \
    done; \
    [ ! -e /app/.agents/skills/SKILL.md ] || \
      { echo "a SKILL.md landed at the skills root, so the tree was flattened" >&2; exit 1; }
COPY --from=compose --chown=1000:1000 /out/bundles /app/agent/bundles
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/sirens-echo"]
