//go:build !enterprise

// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.  
// See LICENSE.txt for license information.

package polyon

import (
	// Import PolyON LDAP implementation for non-enterprise builds
	_ "github.com/mattermost/mattermost/server/v8/polyon/ldap"
)