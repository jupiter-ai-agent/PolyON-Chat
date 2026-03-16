#!/usr/bin/env python3
"""
config.json 패치 스크립트 — PP 원칙 적용
- GitLabSettings.Enable: true (Keycloak SSO 항상 활성화)
- EmailSettings: 이메일 로그인 비활성화 (SSO 전용)
"""
import json, os, sys

config_path = sys.argv[1] if len(sys.argv) > 1 else '/mattermost/config/config.json'

with open(config_path) as f:
    cfg = json.load(f)

# GitLab OIDC (Keycloak) — PP 제1원칙
cfg['GitLabSettings']['Enable'] = True

# 이메일 로그인 비활성화 — PP 원칙 (Keycloak SSO 전용)
cfg['EmailSettings']['EnableSignInWithEmail'] = False
cfg['EmailSettings']['EnableSignInWithUsername'] = False

with open(config_path, 'w') as f:
    json.dump(cfg, f, indent=2)

print(f'Patched: {config_path}')
print(f'  GitLabSettings.Enable: {cfg["GitLabSettings"]["Enable"]}')
print(f'  EnableSignInWithEmail: {cfg["EmailSettings"]["EnableSignInWithEmail"]}')
