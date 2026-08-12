FROM forgejo.coilysiren.me/coilyco-flight-deck/agentic-os:release AS build

USER root
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY agent ./agent
COPY .agents/skills/sirens-echo-community ./.agents/skills/sirens-echo-community
COPY .agents/skills/sirens-echo-knowledge ./.agents/skills/sirens-echo-knowledge
COPY .agents/skills/coilyco-general ./.agents/skills/coilyco-general
COPY docs ./docs
RUN CGO_ENABLED=0 go build -trimpath -o /out/sirens-echo ./cmd/sirens-echo \
    && CGO_ENABLED=0 go build -trimpath -o /out/sirens-echo-policy-check ./cmd/sirens-echo-policy-check \
    && CGO_ENABLED=0 go build -trimpath -o /out/sirens-echo-compose ./cmd/sirens-echo-compose \
    && /out/sirens-echo-policy-check

# The release image ships agent-compose but not the composed catalogue, so this
# stage fetches it. Pinning the ref keeps the bundle deterministic and makes a
# catalogue change a reviewed bump. See docs/sirens-echo-compose.md.
FROM forgejo.coilysiren.me/coilyco-flight-deck/agentic-os:release AS compose
ARG AOS_CATALOG_REF=main
USER root
WORKDIR /src
RUN git clone --depth 1 --branch "${AOS_CATALOG_REF}" \
    https://forgejo.coilysiren.me/coilyco-flight-deck/agentic-os.git /tmp/aos-catalog
COPY agent/compose ./agent/compose
COPY scripts/stage-compose-sources.sh ./scripts/
# The expander comes from the build stage, so this stage needs no Go toolchain
# work and the binary is the one the suite already exercised.
COPY --from=build /out/sirens-echo-compose /usr/local/bin/sirens-echo-compose
RUN SIRENS_ECHO_COMPOSE_BIN=/usr/local/bin/sirens-echo-compose \
    bash scripts/stage-compose-sources.sh /out/bundles /tmp/aos-catalog

FROM forgejo.coilysiren.me/coilyco-flight-deck/agentic-os:release

USER root
WORKDIR /app
COPY --from=build --chown=1000:1000 /out/sirens-echo /usr/local/bin/sirens-echo
# Shipped so another layer can expand the same allowlist with catalogues this
# build cannot see. See docs/sirens-echo-compose.md.
COPY --from=build --chown=1000:1000 /out/sirens-echo-compose /usr/local/bin/sirens-echo-compose
COPY --chown=1000:1000 scripts/stage-compose-sources.sh /app/scripts/stage-compose-sources.sh
COPY --chown=1000:1000 agent /app/agent
COPY --chown=1000:1000 .agents/skills/sirens-echo-community /app/.agents/skills/sirens-echo-community
COPY --chown=1000:1000 .agents/skills/sirens-echo-knowledge /app/.agents/skills/sirens-echo-knowledge
COPY --chown=1000:1000 .agents/skills/coilyco-general /app/.agents/skills/coilyco-general
COPY --from=compose --chown=1000:1000 /out/bundles /app/agent/bundles
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/sirens-echo"]
