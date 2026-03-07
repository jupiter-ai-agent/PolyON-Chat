// Copyright (c) 2026 Triangle.s - PolyON Platform

package ldap

import (
	"fmt"

	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

func init() {
	fmt.Println("[PolyON] Registering LDAP interface")
	app.RegisterLdapInterface(func(a *app.App) einterfaces.LdapInterface {
		fmt.Println("[PolyON] LDAP interface created")
		return NewLdapProvider(a)
	})

	platform.RegisterLdapDiagnosticInterface(func(ps *platform.PlatformService) einterfaces.LdapDiagnosticInterface {
		fmt.Println("[PolyON] LDAP Diagnostic interface created")
		return &LdapDiagnostic{
			config: ps.Config,
		}
	})
}
