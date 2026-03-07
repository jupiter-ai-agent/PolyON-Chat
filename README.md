# PolyON Chat

Mattermost v11.5.1 기반 커스텀 채팅 서버. PolyON 플랫폼의 채팅 모듈.

## 왜 포크했는가

Mattermost Team Edition은 LDAP/OIDC 기능에 Enterprise 라이선스 체크가 걸려 있다.
PolyON은 AD DC(Active Directory) 기반 통합 인증이 핵심인데, 라이선스 없이는:

- `POST /api/v4/ldap/sync` → **501 Not Implemented**
- LDAP 로그인 → 라이선스 체크 → 차단
- OpenID Connect → 라이선스 체크 → 차단

**Stalwart Mail처럼 설정만 넣으면 AD DC 인증이 즉시 동작해야 한다** — 이것이 PolyON의 철학.

## 무엇을 바꿨는가

### 1. LDAP 라이선스 체크 제거

API, App, Config 레이어에서 LDAP 관련 라이선스 체크를 제거했다.

| 레이어 | 파일 | 변경 내용 |
|--------|------|-----------|
| API | `channels/api4/ldap.go` | `requireLicense` 체크 제거 |
| API | `channels/api4/user.go` | LDAP 전환 시 라이선스 체크 제거 |
| API | `channels/api4/group.go` | `licensedAndConfiguredForGroupBySource` LDAP 항상 허용 |
| App | `channels/app/ldap.go` | LDAP sync/test 라이선스 체크 제거 |
| App | `channels/app/authentication.go` | LDAP 인증 라이선스 체크 제거 |
| App | `channels/app/login.go` | LDAP 로그인 라이선스 체크 제거 |
| Config | `config/client.go` | LDAP/OIDC 설정 노출 라이선스 체크 제거 |

### 2. PolyON LDAP 구현체 (`server/polyon/ldap/`)

Mattermost Enterprise의 LDAP 구현은 비공개 저장소(`mattermost/enterprise`)에 있다.
Team Edition 빌드에는 포함되지 않으므로, `go-ldap/v3` 기반으로 직접 구현했다.

| 파일 | 역할 |
|------|------|
| `polyon/ldap/init.go` | `RegisterLdapInterface` — 서버 시작 시 LDAP 인터페이스 등록 |
| `polyon/ldap/ldap.go` | `LdapInterfaceImpl` — DoLogin, GetUser, GetAllLdapUsers, SwitchToLdap 등 |
| `polyon/ldap/diagnostic.go` | `LdapDiagnostic` — RunTest, RunTestConnection, RunTestDiagnostics |
| `polyon/imports.go` | `//go:build !enterprise` — Enterprise 빌드가 아닐 때 PolyON LDAP 활성화 |

**AD DC 호환 설계:**
- `sAMAccountName` 기반 ID 매핑 (objectGUID 바이너리 문제 회피)
- `mail`, `userPrincipalName` 복합 검색 필터
- Bind 인증 (AD DC 표준)
- 자동 유저 생성/업데이트

### 3. 진입점 수정

`server/cmd/mattermost/main.go`에 `_ "server/v8/polyon"` import 추가.
서버 시작 시 `init()`으로 LDAP 인터페이스가 자동 등록된다.

## 빌드

```bash
docker build --platform linux/arm64 -t jupitertriangles/polyon-chat:v2.0.3 .
```

**빌드 구조 (멀티스테이지):**
1. `golang:1.25-alpine` — Go 서버 소스 빌드
2. `mattermost/mattermost-team-edition:11.5.1` (amd64) — webapp/i18n/templates 추출
3. `alpine:3.21` — 런타임

> 공식 Mattermost 이미지는 amd64만 제공. 서버는 소스에서 arm64로 빌드하고, webapp(JS)은 아키텍처 무관이므로 공식 이미지에서 추출.

## LDAP 설정

서버 기동 후 Config API로 LDAP 설정 투입:

```bash
curl -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "LdapSettings": {
      "Enable": true,
      "EnableSync": true,
      "LdapServer": "polyon-dc.polyon.svc.cluster.local",
      "LdapPort": 389,
      "BaseDN": "DC=CMARS,DC=COM",
      "BindUsername": "CN=Administrator,CN=Users,DC=CMARS,DC=COM",
      "BindPassword": "...",
      "IdAttribute": "sAMAccountName",
      "LoginIdAttribute": "sAMAccountName",
      "EmailAttribute": "mail",
      "UsernameAttribute": "sAMAccountName",
      "FirstNameAttribute": "givenName",
      "LastNameAttribute": "sn",
      "NicknameAttribute": "displayName",
      "UserFilter": "(&(objectClass=user)(!(objectClass=computer))(!(userAccountControl:1.2.840.113556.1.4.803:=2)))"
    }
  }' \
  http://localhost:8065/api/v4/config
```

설정 투입 후 AD 유저는 **즉시 LDAP 로그인 가능** — 자동 계정 생성됨.

## 업스트림 리베이스 방침

최소 패치 원칙을 따른다:

- LDAP/OIDC **라이선스 체크만** 제거 (기능 코드 변경 없음)
- `polyon/` 디렉토리는 완전 독립 (업스트림 코드와 충돌 없음)
- `//go:build !enterprise` 태그로 Enterprise 코드와 공존 가능

업스트림 업데이트 시: `upstream/` 갱신 → 라이선스 체크 파일 12개만 재패치.

## 관련 프로젝트

- [PolyON](https://gitlab.triangles.co.kr/cmars/polyon) — 플랫폼 본체 (Core, Console, Operator)
- Mattermost v11.5.1 — [github.com/mattermost/mattermost](https://github.com/mattermost/mattermost)
