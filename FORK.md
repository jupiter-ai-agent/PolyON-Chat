# Fork 기록

## 베이스
- **Mattermost v11.5.1** (2026-03-07 클론)
- 소스: `github.com/mattermost/mattermost` tag `v11.5.1`
- Go: 1.24.13 (go.work는 1.25 요구)

## 수정 파일 목록

### 새로 생성 (4개)
```
server/polyon/imports.go              # //go:build !enterprise, LDAP import
server/polyon/ldap/init.go            # RegisterLdapInterface, RegisterLdapDiagnosticInterface
server/polyon/ldap/ldap.go            # LdapInterfaceImpl (DoLogin, GetUser, GetAllLdapUsers 등)
server/polyon/ldap/diagnostic.go      # LdapDiagnostic (RunTest, RunTestConnection 등)
```

### 수정 (10개)
```
server/cmd/mattermost/main.go         # + import _ "server/v8/polyon"
server/go.mod                          # + github.com/go-ldap/ldap/v3
server/go.sum                          # 의존성 해시
server/go.work                         # use (. ./public)

server/channels/api4/ldap.go           # LDAP API 라이선스 체크 제거
server/channels/api4/user.go           # switchToLdap 라이선스 체크 제거
server/channels/api4/group.go          # licensedAndConfiguredForGroupBySource LDAP 허용
server/channels/app/ldap.go            # LDAP sync/test 라이선스 체크 제거
server/channels/app/authentication.go  # LDAP 인증 라이선스 체크 제거
server/channels/app/login.go           # LDAP 로그인 라이선스 체크 제거
server/config/client.go                # LDAP/OIDC 설정 노출 라이선스 체크 제거
```

## 라이선스 체크 패턴

제거한 패턴 (원본):
```go
// 패턴 1: License() nil 체크
if c.App.Srv().License() == nil {
    c.Err = model.NewAppError(...)
    return
}

// 패턴 2: Feature 플래그 체크
if !*c.App.Srv().License().Features.LDAP {
    c.Err = model.NewAppError(...)
    return
}

// 패턴 3: 복합 조건
if c.App.Srv().License() == nil || !*c.App.Srv().License().Features.LDAPGroups {
    ...
}
```

변경 후:
```go
// PolyON: LDAP is always available (no license check)
// (체크 코드 삭제 또는 주석 처리)
```

## 리베이스 가이드

1. `upstream/` 을 새 버전으로 교체
2. `server/polyon/` 은 그대로 유지 (독립 패키지)
3. 아래 파일의 라이선스 체크만 재패치:
   - `channels/api4/ldap.go` — `requireLicense()` 제거
   - `channels/api4/user.go` — `switchToLdap` 라이선스 체크 제거
   - `channels/api4/group.go` — LDAP/LDAPGroups 항상 허용
   - `channels/app/ldap.go` — sync/test 라이선스 체크 제거
   - `channels/app/authentication.go` — 인증 라이선스 체크 제거
   - `channels/app/login.go` — 로그인 라이선스 체크 제거
   - `config/client.go` — 설정 노출 라이선스 체크 제거
4. `cmd/mattermost/main.go` — polyon import 재추가
5. `go.mod` — `go-ldap/v3` 의존성 확인
