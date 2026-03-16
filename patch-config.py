#!/usr/bin/env python3
"""
config.json 패치 스크립트 — PP 원칙 적용
- GitLabSettings: Keycloak OIDC (환경변수에서 주입)
- LdapSettings: AD 연동 (환경변수에서 주입)
- EmailSettings: 이메일 로그인 비활성화 (SSO 전용)

환경변수 없으면 GitLabSettings.Enable만 true로 설정.
실제 값은 PolyON-Core provisioning 단계에서 API로 주입됨.
"""
import json, os, sys

config_path = sys.argv[1] if len(sys.argv) > 1 else '/mattermost/config/config.json'

with open(config_path) as f:
    cfg = json.load(f)

# GitLab OIDC (Keycloak) — 환경변수에서 주입 (없으면 Enable만 true)
cfg['GitLabSettings']['Enable'] = True
if os.environ.get('MM_GITLABSETTINGS_ID'):
    cfg['GitLabSettings']['Id'] = os.environ['MM_GITLABSETTINGS_ID']
    cfg['GitLabSettings']['Secret'] = os.environ.get('MM_GITLABSETTINGS_SECRET', '')
    cfg['GitLabSettings']['Scope'] = os.environ.get('MM_GITLABSETTINGS_SCOPE', 'openid profile email')
    cfg['GitLabSettings']['AuthEndpoint'] = os.environ.get('MM_GITLABSETTINGS_AUTHENDPOINT', '')
    cfg['GitLabSettings']['TokenEndpoint'] = os.environ.get('MM_GITLABSETTINGS_TOKENENDPOINT', '')
    cfg['GitLabSettings']['UserAPIEndpoint'] = os.environ.get('MM_GITLABSETTINGS_USERAPIENDPOINT', '')
    cfg['GitLabSettings']['ButtonText'] = os.environ.get('MM_GITLABSETTINGS_BUTTONTEXT', 'PolyON SSO (AD 계정)')
    cfg['GitLabSettings']['ButtonColor'] = os.environ.get('MM_GITLABSETTINGS_BUTTONCOLOR', '#0058CC')

# LDAP — 환경변수에서 주입 (없으면 스킵 — Core provisioning 단계에서 설정)
if os.environ.get('MM_LDAPSETTINGS_LDAPSERVER'):
    cfg['LdapSettings']['Enable'] = True
    cfg['LdapSettings']['EnableSync'] = True
    cfg['LdapSettings']['LdapServer'] = os.environ['MM_LDAPSETTINGS_LDAPSERVER']
    cfg['LdapSettings']['LdapPort'] = int(os.environ.get('MM_LDAPSETTINGS_LDAPPORT', '389'))
    cfg['LdapSettings']['BaseDN'] = os.environ.get('MM_LDAPSETTINGS_BASEDN', '')
    cfg['LdapSettings']['BindUsername'] = os.environ.get('MM_LDAPSETTINGS_BINDUSERNAME', '')
    cfg['LdapSettings']['BindPassword'] = os.environ.get('MM_LDAPSETTINGS_BINDPASSWORD', '')
    cfg['LdapSettings']['IdAttribute'] = os.environ.get('MM_LDAPSETTINGS_IDATTRIBUTE', 'sAMAccountName')
    cfg['LdapSettings']['UsernameAttribute'] = os.environ.get('MM_LDAPSETTINGS_USERNAMEATTRIBUTE', 'sAMAccountName')
    cfg['LdapSettings']['EmailAttribute'] = os.environ.get('MM_LDAPSETTINGS_EMAILATTRIBUTE', 'userPrincipalName')
    cfg['LdapSettings']['LoginIdAttribute'] = os.environ.get('MM_LDAPSETTINGS_LOGINIDATTRIBUTE', 'sAMAccountName')
    cfg['LdapSettings']['FirstNameAttribute'] = os.environ.get('MM_LDAPSETTINGS_FIRSTNAMEATTRIBUTE', 'givenName')
    cfg['LdapSettings']['LastNameAttribute'] = os.environ.get('MM_LDAPSETTINGS_LASTNAMEATTRIBUTE', 'sn')
    cfg['LdapSettings']['NicknameAttribute'] = os.environ.get('MM_LDAPSETTINGS_NICKNAMEATTRIBUTE', 'displayName')
    cfg['LdapSettings']['PositionAttribute'] = os.environ.get('MM_LDAPSETTINGS_POSITIONATTRIBUTE', 'title')
    cfg['LdapSettings']['UserFilter'] = os.environ.get('MM_LDAPSETTINGS_USERFILTER', '(&(objectClass=user)(!(objectClass=computer))(!(userAccountControl:1.2.840.113556.1.4.803:=2)))')
    cfg['LdapSettings']['LoginFieldName'] = os.environ.get('MM_LDAPSETTINGS_LOGINFIELDNAME', 'AD 계정')
    cfg['LdapSettings']['SkipCertificateVerification'] = True
    cfg['LdapSettings']['MaximumLoginAttempts'] = 10

# 이메일 로그인 비활성화 — PP 원칙 (Keycloak SSO 전용)
cfg['EmailSettings']['EnableSignInWithEmail'] = False
cfg['EmailSettings']['EnableSignInWithUsername'] = False

with open(config_path, 'w') as f:
    json.dump(cfg, f, indent=2)

print(f'Patched: {config_path}')
print(f'  GitLabSettings.Enable: {cfg["GitLabSettings"]["Enable"]}')
print(f'  EnableSignInWithEmail: {cfg["EmailSettings"]["EnableSignInWithEmail"]}')
