# ── Stage 1: Build PolyON Chat server (Go) ──
FROM golang:1.25-alpine AS server-builder
RUN apk add --no-cache git
WORKDIR /src
COPY upstream/server/ server/
WORKDIR /src/server
RUN go build -o /polyon-chat ./cmd/mattermost/

# ── Stage 2: Official image — config/i18n/templates/fonts 추출용 ──
FROM --platform=linux/amd64 mattermost/mattermost-team-edition:11.5.1 AS official-source

# ── Stage 3: Build webapp from source ──
# login.tsx (auto-redirect to Keycloak), login_gitlab_icon.tsx (PolyON icon) 반영
FROM node:20-alpine AS webapp-builder
RUN apk add --no-cache git python3 make g++
WORKDIR /webapp
COPY upstream/webapp/ .
# npm ci: postinstall이 patch-package + platform subpackages 자동 빌드
RUN npm ci
# channels webapp 빌드 (output: channels/dist/)
RUN npm run build --workspace=channels

# ── Stage 4: Runtime ──
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata mailcap tini python3
RUN adduser -D -u 2000 -h /mattermost mattermost

WORKDIR /mattermost
COPY --from=server-builder /polyon-chat bin/mattermost
COPY --from=webapp-builder /webapp/channels/dist/ client/
# config/i18n/templates/fonts는 공식 이미지에서 가져옴 (서버 전용 리소스)
COPY --from=official-source /mattermost/config/config.json config/config.json
COPY --from=official-source /mattermost/i18n/ i18n/
COPY --from=official-source /mattermost/templates/ templates/
COPY --from=official-source /mattermost/fonts/ fonts/

# PP 원칙 적용: GitLab OIDC 활성화, 이메일 로그인 비활성화
COPY patch-config.py /tmp/patch-config.py
RUN python3 /tmp/patch-config.py config/config.json

# PP module manifest
COPY module.yaml /polyon-module/module.yaml

RUN mkdir -p data logs plugins client/plugins \
    && chown -R mattermost:mattermost /mattermost
USER mattermost

EXPOSE 8065
ENTRYPOINT ["tini", "--"]
CMD ["bin/mattermost", "server"]
