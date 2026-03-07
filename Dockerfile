# ── Stage 1: Build PolyON Chat server (Go) ──
FROM golang:1.25-alpine AS server-builder
RUN apk add --no-cache git
WORKDIR /src
COPY upstream/server/ server/
WORKDIR /src/server
RUN go build -o /polyon-chat ./cmd/mattermost/

# ── Stage 2: Extract webapp from official amd64 image ──
FROM --platform=linux/amd64 mattermost/mattermost-team-edition:11.5.1 AS webapp-source

# ── Stage 3: Runtime ──
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata mailcap tini
RUN adduser -D -u 2000 -h /mattermost mattermost

WORKDIR /mattermost
COPY --from=server-builder /polyon-chat bin/mattermost
COPY --from=webapp-source /mattermost/client/ client/
COPY --from=webapp-source /mattermost/config/config.json config/config.json
COPY --from=webapp-source /mattermost/i18n/ i18n/
COPY --from=webapp-source /mattermost/templates/ templates/
COPY --from=webapp-source /mattermost/fonts/ fonts/

# PP module manifest
COPY module.yaml /polyon-module/module.yaml

RUN mkdir -p data logs plugins client/plugins \
    && chown -R mattermost:mattermost /mattermost
USER mattermost

EXPOSE 8065
ENTRYPOINT ["tini", "--"]
CMD ["bin/mattermost", "server"]
