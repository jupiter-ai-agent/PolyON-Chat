// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package oauthgitlab

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

type GitLabProvider struct {
}

type GitLabUser struct {
	Id                int64  `json:"id"`
	Username          string `json:"username"`
	Login             string `json:"login"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	// PolyON: Keycloak OIDC claims (userinfo endpoint)
	Sub               string `json:"sub"`               // Keycloak subject (unique ID)
	PreferredUsername string `json:"preferred_username"` // Keycloak username
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
}

func init() {
	provider := &GitLabProvider{}
	einterfaces.RegisterOAuthProvider(model.UserAuthServiceGitlab, provider)
}

func userFromGitLabUser(logger mlog.LoggerIFace, glu *GitLabUser) *model.User {
	user := &model.User{}

	// PolyON: Keycloak preferred_username 우선, 없으면 GitLab username/login
	username := glu.PreferredUsername
	if username == "" {
		username = glu.Username
	}
	if username == "" {
		username = glu.Login
	}
	user.Username = model.CleanUsername(logger, username)

	// PolyON: Keycloak given_name/family_name 우선
	if glu.GivenName != "" || glu.FamilyName != "" {
		user.FirstName = glu.GivenName
		user.LastName = glu.FamilyName
	} else {
		splitName := strings.Split(glu.Name, " ")
		if len(splitName) == 2 {
			user.FirstName = splitName[0]
			user.LastName = splitName[1]
		} else if len(splitName) >= 2 {
			user.FirstName = splitName[0]
			user.LastName = strings.Join(splitName[1:], " ")
		} else {
			user.FirstName = glu.Name
		}
	}

	user.Email = strings.ToLower(glu.Email)
	userId := glu.getAuthData()
	user.AuthData = &userId
	user.AuthService = model.UserAuthServiceGitlab

	return user
}

func gitLabUserFromJSON(data io.Reader) (*GitLabUser, error) {
	decoder := json.NewDecoder(data)
	var glu GitLabUser
	err := decoder.Decode(&glu)
	if err != nil {
		return nil, err
	}
	return &glu, nil
}

func (glu *GitLabUser) IsValid() error {
	// PolyON: Keycloak OIDC는 sub 필드를 사용 (Id=0 허용)
	if glu.Id == 0 && glu.Sub == "" {
		return errors.New("user id can't be 0 and sub is empty")
	}

	if glu.Email == "" {
		return errors.New("user e-mail should not be empty")
	}

	return nil
}

func (glu *GitLabUser) getAuthData() string {
	// PolyON: Keycloak은 sub, GitLab은 Id 사용
	if glu.Sub != "" {
		return glu.Sub
	}
	return strconv.FormatInt(glu.Id, 10)
}

func (gp *GitLabProvider) GetUserFromJSON(rctx request.CTX, data io.Reader, tokenUser *model.User) (*model.User, error) {
	glu, err := gitLabUserFromJSON(data)
	if err != nil {
		return nil, err
	}
	if err = glu.IsValid(); err != nil {
		return nil, err
	}

	return userFromGitLabUser(rctx.Logger(), glu), nil
}

func (gp *GitLabProvider) GetSSOSettings(_ request.CTX, config *model.Config, service string) (*model.SSOSettings, error) {
	return &config.GitLabSettings, nil
}

func (gp *GitLabProvider) GetUserFromIdToken(_ request.CTX, idToken string) (*model.User, error) {
	return nil, nil
}

func (gp *GitLabProvider) IsSameUser(_ request.CTX, dbUser, oauthUser *model.User) bool {
	return dbUser.AuthData == oauthUser.AuthData
}
