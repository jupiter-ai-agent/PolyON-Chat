// Copyright (c) 2026 Triangle.s - PolyON Platform
// LDAP Diagnostic implementation for AD DC integration

package ldap

import (
	"crypto/tls"
	"fmt"

	ldapv3 "github.com/go-ldap/ldap/v3"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func dialLDAP(settings *model.LdapSettings) (*ldapv3.Conn, error) {
	server := *settings.LdapServer
	port := 389
	if settings.LdapPort != nil {
		port = *settings.LdapPort
	}
	addr := fmt.Sprintf("%s:%d", server, port)

	if settings.ConnectionSecurity != nil && *settings.ConnectionSecurity == model.ConnSecurityTLS {
		tlsCfg := &tls.Config{InsecureSkipVerify: settings.SkipCertificateVerification != nil && *settings.SkipCertificateVerification}
		return ldapv3.DialTLS("tcp", addr, tlsCfg)
	}
	return ldapv3.Dial("tcp", addr)
}

func bindLDAP(conn *ldapv3.Conn, settings *model.LdapSettings) error {
	if settings.BindUsername != nil && *settings.BindUsername != "" {
		return conn.Bind(*settings.BindUsername, *settings.BindPassword)
	}
	return nil
}

type LdapDiagnostic struct {
	config func() *model.Config
}

func (ld *LdapDiagnostic) RunTest(rctx request.CTX) *model.AppError {
	settings := ld.config().LdapSettings
	return ld.RunTestConnection(rctx, settings)
}

func (ld *LdapDiagnostic) RunTestConnection(rctx request.CTX, settings model.LdapSettings) *model.AppError {
	conn, err := dialLDAP(&settings)
	if err != nil {
		return model.NewAppError("LdapDiagnostic.RunTestConnection", "ldap.test_connection.failed",
			nil, err.Error(), 500)
	}
	defer conn.Close()

	if err := bindLDAP(conn, &settings); err != nil {
		return model.NewAppError("LdapDiagnostic.RunTestConnection", "ldap.test_bind.failed",
			nil, err.Error(), 500)
	}

	return nil
}

func (ld *LdapDiagnostic) RunTestDiagnostics(rctx request.CTX, testType model.LdapDiagnosticTestType, settings model.LdapSettings) ([]model.LdapDiagnosticResult, *model.AppError) {
	conn, err := dialLDAP(&settings)
	if err != nil {
		return nil, model.NewAppError("LdapDiagnostic.RunTestDiagnostics", "ldap.test_connection.failed",
			nil, err.Error(), 500)
	}
	defer conn.Close()

	if err := bindLDAP(conn, &settings); err != nil {
		return nil, model.NewAppError("LdapDiagnostic.RunTestDiagnostics", "ldap.test_bind.failed",
			nil, err.Error(), 500)
	}

	var results []model.LdapDiagnosticResult

	switch testType {
	case model.LdapDiagnosticTestTypeFilters:
		results = ld.testFilters(conn, &settings)
	case model.LdapDiagnosticTestTypeAttributes:
		results = ld.testAttributes(conn, &settings)
	default:
		results = []model.LdapDiagnosticResult{{TestName: string(testType), Error: "unsupported test type"}}
	}

	return results, nil
}

func (ld *LdapDiagnostic) GetVendorNameAndVendorVersion(rctx request.CTX) (string, string, error) {
	return "Samba AD DC", "4.x", nil
}

func (ld *LdapDiagnostic) testFilters(conn *ldapv3.Conn, settings *model.LdapSettings) []model.LdapDiagnosticResult {
	filter := *settings.UserFilter
	if filter == "" {
		filter = "(&(objectClass=user)(!(objectClass=computer)))"
	}

	sr, err := conn.Search(ldapv3.NewSearchRequest(
		*settings.BaseDN, ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 100, 10,
		false, filter, []string{"dn"}, nil,
	))

	result := model.LdapDiagnosticResult{
		TestName:  "user_filter",
		TestValue: filter,
	}

	if err != nil {
		result.Error = err.Error()
	} else {
		result.TotalCount = len(sr.Entries)
		result.Message = fmt.Sprintf("Found %d users", len(sr.Entries))
	}

	return []model.LdapDiagnosticResult{result}
}

func (ld *LdapDiagnostic) testAttributes(conn *ldapv3.Conn, settings *model.LdapSettings) []model.LdapDiagnosticResult {
	filter := *settings.UserFilter
	if filter == "" {
		filter = "(&(objectClass=user)(!(objectClass=computer)))"
	}

	attrs := []string{
		*settings.IdAttribute,
		*settings.UsernameAttribute,
		*settings.EmailAttribute,
		*settings.FirstNameAttribute,
		*settings.LastNameAttribute,
	}

	sr, err := conn.Search(ldapv3.NewSearchRequest(
		*settings.BaseDN, ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 5, 10,
		false, filter, attrs, nil,
	))

	result := model.LdapDiagnosticResult{
		TestName: "attributes",
	}

	if err != nil {
		result.Error = err.Error()
	} else {
		result.TotalCount = len(sr.Entries)
		for _, entry := range sr.Entries {
			sample := model.LdapSampleEntry{
				DN:       entry.DN,
				Username: entry.GetAttributeValue(*settings.UsernameAttribute),
				Email:    entry.GetAttributeValue(*settings.EmailAttribute),
			}
			result.SampleResults = append(result.SampleResults, sample)
		}
	}

	return []model.LdapDiagnosticResult{result}
}
