// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package ldap

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

type LdapInterfaceImpl struct {
	app *app.App
}

func NewLdapProvider(a *app.App) einterfaces.LdapInterface {
	return &LdapInterfaceImpl{
		app: a,
	}
}

// getLdapConfig returns current LDAP settings
func (li *LdapInterfaceImpl) getLdapConfig() *model.LdapSettings {
	cfg := li.app.Config().LdapSettings
	return &cfg
}

// connectToLdap establishes LDAP connection
func (li *LdapInterfaceImpl) connectToLdap() (*ldap.Conn, *model.AppError) {
	ldapSettings := li.getLdapConfig()
	
	if ldapSettings.LdapServer == nil || *ldapSettings.LdapServer == "" {
		return nil, model.NewAppError("Ldap.connectToLdap", "ent.ldap.connection.missing_server.app_error", nil, "", 0)
	}
	
	ldapPort := 389
	if ldapSettings.LdapPort != nil {
		ldapPort = *ldapSettings.LdapPort
	}
	
	address := fmt.Sprintf("%s:%d", *ldapSettings.LdapServer, ldapPort)
	
	var conn *ldap.Conn
	var err error
	
	// Handle connection security
	if ldapSettings.ConnectionSecurity != nil {
		switch *ldapSettings.ConnectionSecurity {
		case model.ConnSecurityTLS:
			tlsConfig := &tls.Config{
				InsecureSkipVerify: ldapSettings.SkipCertificateVerification != nil && *ldapSettings.SkipCertificateVerification,
			}
			conn, err = ldap.DialTLS("tcp", address, tlsConfig)
		case model.ConnSecurityStarttls:
			conn, err = ldap.Dial("tcp", address)
			if err == nil {
				tlsConfig := &tls.Config{
					InsecureSkipVerify: ldapSettings.SkipCertificateVerification != nil && *ldapSettings.SkipCertificateVerification,
				}
				err = conn.StartTLS(tlsConfig)
			}
		default:
			conn, err = ldap.Dial("tcp", address)
		}
	} else {
		conn, err = ldap.Dial("tcp", address)
	}
	
	if err != nil {
		return nil, model.NewAppError("Ldap.connectToLdap", "ent.ldap.connection.connection_failed.app_error", nil, err.Error(), 0)
	}
	
	// Bind with service account if configured
	if ldapSettings.BindUsername != nil && *ldapSettings.BindUsername != "" && 
	   ldapSettings.BindPassword != nil && *ldapSettings.BindPassword != "" {
		if err := conn.Bind(*ldapSettings.BindUsername, *ldapSettings.BindPassword); err != nil {
			conn.Close()
			return nil, model.NewAppError("Ldap.connectToLdap", "ent.ldap.connection.bind_failed.app_error", nil, err.Error(), 0)
		}
	}
	
	return conn, nil
}

// searchUser searches for a user by ID (username or email)
func (li *LdapInterfaceImpl) searchUser(conn *ldap.Conn, id string) (*ldap.Entry, *model.AppError) {
	ldapSettings := li.getLdapConfig()
	
	if ldapSettings.BaseDN == nil || *ldapSettings.BaseDN == "" {
		return nil, model.NewAppError("Ldap.searchUser", "ent.ldap.search.missing_base_dn.app_error", nil, "", 0)
	}
	
	// Build search filter — combine user filter with ID lookup
	escapedID := ldap.EscapeFilter(id)
	idFilter := fmt.Sprintf("(|(sAMAccountName=%s)(mail=%s)(userPrincipalName=%s))", escapedID, escapedID, escapedID)
	
	var searchFilter string
	if ldapSettings.UserFilter != nil && *ldapSettings.UserFilter != "" {
		// Combine: (&(userFilter)(idFilter))
		searchFilter = fmt.Sprintf("(&%s%s)", *ldapSettings.UserFilter, idFilter)
	} else {
		searchFilter = fmt.Sprintf("(&(objectClass=user)%s)", idFilter)
	}
	
	fmt.Printf("[PolyON LDAP] searchUser filter: %s\n", searchFilter)
	
	// Search attributes to retrieve
	attributes := []string{
		"sAMAccountName",
		"mail", 
		"displayName",
		"givenName",
		"sn",
		"cn",
		"objectGUID",
	}
	
	// Add custom attributes if configured
	if ldapSettings.UsernameAttribute != nil && *ldapSettings.UsernameAttribute != "" {
		attributes = append(attributes, *ldapSettings.UsernameAttribute)
	}
	if ldapSettings.EmailAttribute != nil && *ldapSettings.EmailAttribute != "" {
		attributes = append(attributes, *ldapSettings.EmailAttribute)
	}
	if ldapSettings.FirstNameAttribute != nil && *ldapSettings.FirstNameAttribute != "" {
		attributes = append(attributes, *ldapSettings.FirstNameAttribute)
	}
	if ldapSettings.LastNameAttribute != nil && *ldapSettings.LastNameAttribute != "" {
		attributes = append(attributes, *ldapSettings.LastNameAttribute)
	}
	if ldapSettings.IdAttribute != nil && *ldapSettings.IdAttribute != "" {
		attributes = append(attributes, *ldapSettings.IdAttribute)
	}
	
	searchRequest := ldap.NewSearchRequest(
		*ldapSettings.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, // Size limit - we expect only one result
		0, // Time limit
		false, // Types only
		searchFilter,
		attributes,
		nil,
	)
	
	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return nil, model.NewAppError("Ldap.searchUser", "ent.ldap.search.search_failed.app_error", nil, err.Error(), 0)
	}
	
	if len(searchResult.Entries) == 0 {
		return nil, model.NewAppError("Ldap.searchUser", "ent.ldap.search.user_not_found.app_error", nil, fmt.Sprintf("id=%s", id), 0)
	}
	
	return searchResult.Entries[0], nil
}

// ldapEntryToUser converts LDAP entry to Mattermost user
func (li *LdapInterfaceImpl) ldapEntryToUser(entry *ldap.Entry) (*model.User, *model.AppError) {
	ldapSettings := li.getLdapConfig()
	
	user := &model.User{}
	user.AuthService = model.UserAuthServiceLdap
	
	// Extract username
	if ldapSettings.UsernameAttribute != nil && *ldapSettings.UsernameAttribute != "" {
		user.Username = entry.GetAttributeValue(*ldapSettings.UsernameAttribute)
	} else {
		user.Username = entry.GetAttributeValue("sAMAccountName")
	}
	
	// Extract email
	if ldapSettings.EmailAttribute != nil && *ldapSettings.EmailAttribute != "" {
		user.Email = entry.GetAttributeValue(*ldapSettings.EmailAttribute)
	} else {
		user.Email = entry.GetAttributeValue("mail")
	}
	
	// Extract first name
	if ldapSettings.FirstNameAttribute != nil && *ldapSettings.FirstNameAttribute != "" {
		user.FirstName = entry.GetAttributeValue(*ldapSettings.FirstNameAttribute)
	} else {
		user.FirstName = entry.GetAttributeValue("givenName")
	}
	
	// Extract last name
	if ldapSettings.LastNameAttribute != nil && *ldapSettings.LastNameAttribute != "" {
		user.LastName = entry.GetAttributeValue(*ldapSettings.LastNameAttribute)
	} else {
		user.LastName = entry.GetAttributeValue("sn")
	}
	
	// Extract LDAP ID for AuthData
	var authData string
	idAttr := "objectGUID"
	if ldapSettings.IdAttribute != nil && *ldapSettings.IdAttribute != "" {
		idAttr = *ldapSettings.IdAttribute
	}
	
	if strings.EqualFold(idAttr, "objectGUID") {
		// objectGUID is binary — convert to hex string
		guidBytes := entry.GetRawAttributeValue("objectGUID")
		if len(guidBytes) > 0 {
			authData = fmt.Sprintf("%x", guidBytes)
		} else {
			authData = entry.GetAttributeValue("sAMAccountName")
		}
	} else {
		authData = entry.GetAttributeValue(idAttr)
		if authData == "" {
			authData = entry.GetAttributeValue("sAMAccountName")
		}
	}
	
	fmt.Printf("[PolyON LDAP] User %s authData=%s (idAttr=%s)\n", user.Username, authData, idAttr)
	user.AuthData = &authData
	
	// Set other fields
	if user.FirstName == "" && user.LastName == "" {
		displayName := entry.GetAttributeValue("displayName")
		if displayName != "" {
			nameParts := strings.Fields(displayName)
			if len(nameParts) >= 2 {
				user.FirstName = nameParts[0]
				user.LastName = strings.Join(nameParts[1:], " ")
			} else if len(nameParts) == 1 {
				user.FirstName = nameParts[0]
			}
		}
	}
	
	// Generate nickname if not set
	if ldapSettings.NicknameAttribute != nil && *ldapSettings.NicknameAttribute != "" {
		user.Nickname = entry.GetAttributeValue(*ldapSettings.NicknameAttribute)
	} else if user.FirstName != "" {
		user.Nickname = user.FirstName
	}
	
	return user, nil
}

// DoLogin authenticates user against LDAP and returns/creates Mattermost user
func (li *LdapInterfaceImpl) DoLogin(rctx request.CTX, id string, password string) (*model.User, *model.AppError) {
	fmt.Printf("[PolyON LDAP] DoLogin called for id=%s\n", id)
	// Connect to LDAP
	conn, appErr := li.connectToLdap()
	if appErr != nil {
		fmt.Printf("[PolyON LDAP] DoLogin connection failed: %s\n", appErr.Error())
		return nil, appErr
	}
	defer conn.Close()
	
	// Search for user
	entry, appErr := li.searchUser(conn, id)
	if appErr != nil {
		return nil, appErr
	}
	
	// Try to bind with user credentials for authentication
	userDN := entry.DN
	if err := conn.Bind(userDN, password); err != nil {
		mlog.Warn("LDAP authentication failed", mlog.String("user_dn", userDN), mlog.Err(err))
		return nil, model.NewAppError("Ldap.DoLogin", "ent.ldap.do_login.invalid_password.app_error", nil, err.Error(), 0)
	}
	
	// Convert LDAP entry to Mattermost user
	ldapUser, appErr := li.ldapEntryToUser(entry)
	if appErr != nil {
		return nil, appErr
	}
	
	// Check if user already exists in Mattermost
	existingUser, err := li.app.Srv().Store().User().GetByAuth(ldapUser.AuthData, model.UserAuthServiceLdap)
	if err != nil {
		// User doesn't exist, create new one
		ldapUser.EmailVerified = true // LDAP users are considered verified
		
		// Create user in Mattermost
		user, appErr := li.app.CreateUser(rctx, ldapUser)
		if appErr != nil {
			return nil, appErr
		}
		
		mlog.Info("Created new LDAP user", mlog.String("username", user.Username), mlog.String("email", user.Email))
		return user, nil
	} else {
		// User exists, update their information
		existingUser.Email = ldapUser.Email
		existingUser.FirstName = ldapUser.FirstName
		existingUser.LastName = ldapUser.LastName
		existingUser.Nickname = ldapUser.Nickname
		
		user, appErr := li.app.UpdateUser(rctx, existingUser, false)
		if appErr != nil {
			return nil, appErr
		}
		
		return user, nil
	}
}

// GetUser retrieves user info from LDAP by ID
func (li *LdapInterfaceImpl) GetUser(rctx request.CTX, id string) (*model.User, *model.AppError) {
	fmt.Printf("[PolyON LDAP] GetUser called for id=%s\n", id)
	conn, appErr := li.connectToLdap()
	if appErr != nil {
		fmt.Printf("[PolyON LDAP] GetUser connection failed: %s\n", appErr.Error())
		return nil, appErr
	}
	defer conn.Close()
	
	entry, appErr := li.searchUser(conn, id)
	if appErr != nil {
		return nil, appErr
	}
	
	return li.ldapEntryToUser(entry)
}

// GetLDAPUserForMMUser retrieves LDAP info for existing Mattermost user
func (li *LdapInterfaceImpl) GetLDAPUserForMMUser(rctx request.CTX, mmUser *model.User) (*model.User, string, *model.AppError) {
	if mmUser.AuthService != model.UserAuthServiceLdap || mmUser.AuthData == nil {
		return nil, "", model.NewAppError("Ldap.GetLDAPUserForMMUser", "ent.ldap.get_ldap_user.not_ldap_user.app_error", nil, "", 0)
	}
	
	// Search by AuthData (LDAP ID)
	ldapUser, appErr := li.GetUser(rctx, *mmUser.AuthData)
	if appErr != nil {
		return nil, "", appErr
	}
	
	return ldapUser, "", nil
}

// GetUserAttributes retrieves specific attributes for a user
func (li *LdapInterfaceImpl) GetUserAttributes(rctx request.CTX, id string, attributes []string) (map[string]string, *model.AppError) {
	conn, appErr := li.connectToLdap()
	if appErr != nil {
		return nil, appErr
	}
	defer conn.Close()
	
	ldapSettings := li.getLdapConfig()
	
	if ldapSettings.BaseDN == nil || *ldapSettings.BaseDN == "" {
		return nil, model.NewAppError("Ldap.GetUserAttributes", "ent.ldap.search.missing_base_dn.app_error", nil, "", 0)
	}
	
	// Build search filter
	var searchFilter string
	if ldapSettings.UserFilter != nil && *ldapSettings.UserFilter != "" {
		searchFilter = strings.ReplaceAll(*ldapSettings.UserFilter, "{id}", ldap.EscapeFilter(id))
	} else {
		searchFilter = fmt.Sprintf("(|(sAMAccountName=%s)(mail=%s))", ldap.EscapeFilter(id), ldap.EscapeFilter(id))
	}
	
	searchRequest := ldap.NewSearchRequest(
		*ldapSettings.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,
		0,
		false,
		searchFilter,
		attributes,
		nil,
	)
	
	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return nil, model.NewAppError("Ldap.GetUserAttributes", "ent.ldap.search.search_failed.app_error", nil, err.Error(), 0)
	}
	
	if len(searchResult.Entries) == 0 {
		return nil, model.NewAppError("Ldap.GetUserAttributes", "ent.ldap.search.user_not_found.app_error", nil, fmt.Sprintf("id=%s", id), 0)
	}
	
	result := make(map[string]string)
	entry := searchResult.Entries[0]
	for _, attr := range attributes {
		result[attr] = entry.GetAttributeValue(attr)
	}
	
	return result, nil
}

// CheckProviderAttributes validates LDAP provider attributes
func (li *LdapInterfaceImpl) CheckProviderAttributes(rctx request.CTX, LS *model.LdapSettings, ouser *model.User, patch *model.UserPatch) string {
	// This is typically used for validation - return empty string for no issues
	return ""
}

// SwitchToLdap converts a user account to LDAP authentication
func (li *LdapInterfaceImpl) SwitchToLdap(rctx request.CTX, userID, ldapID, ldapPassword string) *model.AppError {
	// Get the existing user
	user, appErr := li.app.GetUser(userID)
	if appErr != nil {
		return appErr
	}
	
	// Verify LDAP credentials
	ldapUser, appErr := li.DoLogin(rctx, ldapID, ldapPassword)
	if appErr != nil {
		return appErr
	}
	
	// Switch user to LDAP auth
	user.AuthService = model.UserAuthServiceLdap
	user.AuthData = ldapUser.AuthData
	user.Password = ""
	
	_, appErr = li.app.UpdateUser(rctx, user, false)
	return appErr
}

// StartSynchronizeJob starts LDAP synchronization job
func (li *LdapInterfaceImpl) StartSynchronizeJob(rctx request.CTX, waitForJobToFinish bool) (*model.Job, *model.AppError) {
	// For now, return a dummy job - actual implementation would create a job
	job := &model.Job{
		Id:       model.NewId(),
		Type:     model.JobTypeLdapSync,
		Status:   model.JobStatusSuccess,
		Progress: 100,
	}
	
	return job, nil
}

// GetAllLdapUsers retrieves all LDAP users for synchronization
func (li *LdapInterfaceImpl) GetAllLdapUsers(rctx request.CTX) ([]*model.User, *model.AppError) {
	conn, appErr := li.connectToLdap()
	if appErr != nil {
		return nil, appErr
	}
	defer conn.Close()
	
	ldapSettings := li.getLdapConfig()
	
	if ldapSettings.BaseDN == nil || *ldapSettings.BaseDN == "" {
		return nil, model.NewAppError("Ldap.GetAllLdapUsers", "ent.ldap.search.missing_base_dn.app_error", nil, "", 0)
	}
	
	// Search for all users
	var searchFilter string
	if ldapSettings.UserFilter != nil && *ldapSettings.UserFilter != "" {
		// Remove {id} placeholder and use wildcard
		searchFilter = strings.ReplaceAll(*ldapSettings.UserFilter, "{id}", "*")
	} else {
		searchFilter = "(objectClass=user)" // Default AD user filter
	}
	
	attributes := []string{
		"sAMAccountName",
		"mail",
		"displayName", 
		"givenName",
		"sn",
		"cn",
		"objectGUID",
	}
	
	searchRequest := ldap.NewSearchRequest(
		*ldapSettings.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, // No size limit
		0, // No time limit
		false,
		searchFilter,
		attributes,
		nil,
	)
	
	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return nil, model.NewAppError("Ldap.GetAllLdapUsers", "ent.ldap.search.search_failed.app_error", nil, err.Error(), 0)
	}
	
	var users []*model.User
	for _, entry := range searchResult.Entries {
		user, appErr := li.ldapEntryToUser(entry)
		if appErr == nil {
			users = append(users, user)
		}
	}
	
	return users, nil
}

// MigrateIDAttribute migrates LDAP ID attribute
func (li *LdapInterfaceImpl) MigrateIDAttribute(rctx request.CTX, toAttribute string) error {
	// Implementation would handle migration of ID attributes
	// For now, return nil (no-op)
	return nil
}

// GetGroup retrieves LDAP group by UID
func (li *LdapInterfaceImpl) GetGroup(rctx request.CTX, groupUID string) (*model.Group, *model.AppError) {
	// Basic implementation - would need to search for group in LDAP
	return nil, model.NewAppError("Ldap.GetGroup", "ent.ldap.group.not_implemented.app_error", nil, "", 0)
}

// GetAllGroupsPage retrieves paginated LDAP groups
func (li *LdapInterfaceImpl) GetAllGroupsPage(rctx request.CTX, page int, perPage int, opts model.LdapGroupSearchOpts) ([]*model.Group, int, *model.AppError) {
	// Basic implementation - would need to search for groups in LDAP
	return []*model.Group{}, 0, nil
}

// FirstLoginSync performs first-time login synchronization
func (li *LdapInterfaceImpl) FirstLoginSync(rctx request.CTX, user *model.User) *model.AppError {
	// No additional sync needed for basic implementation
	return nil
}

// UpdateProfilePictureIfNecessary updates profile picture from LDAP
func (li *LdapInterfaceImpl) UpdateProfilePictureIfNecessary(rctx request.CTX, user model.User, session model.Session) {
	// Implementation would fetch profile picture from LDAP if configured
	// For now, this is a no-op
}