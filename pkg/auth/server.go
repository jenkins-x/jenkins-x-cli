package auth

import (
	"fmt"
	"sort"

	"github.com/jenkins-x/jx/pkg/util"
)

func (s *AuthServer) Label() string {
	if s.Name != "" {
		return s.Name
	}
	return s.URL
}

func (s *AuthServer) Description() string {
	if s.Name != "" {
		return s.Name + " at " + s.URL
	}
	return s.URL
}

func (s *AuthServer) DeleteUser(username string) error {
	idx := -1
	for i, user := range s.Users {
		if user.Username == username {
			idx = i
			break
		}
	}
	if idx < 0 {
		if len(s.Users) == 0 {
			return fmt.Errorf("Cannot remote user %s as there are no users for this server", username)
		}
		return util.InvalidArg(username, s.GetUsernames())
	}
	s.Users = append(s.Users[0:idx], s.Users[idx+1:]...)
	return nil
}

func (s *AuthServer) GetUsernames() []string {
	answer := []string{}
	for _, user := range s.Users {
		name := user.Username
		if name != "" {
			answer = append(answer, name)
		}
	}
	sort.Strings(answer)
	return answer
}

//HasUserAuths checks if a server has any user auth configured
func (s *AuthServer) HasUserAuths() bool {
	return len(s.Users) > 0
}

// CurrentAuth returns the current user auth, otherwise the first one
func (s *AuthServer) CurrentAuth() *UserAuth {
	for _, user := range s.Users {
		if user.Username == s.CurrentUser {
			return user
		}
	}
	if len(s.Users) > 0 {
		return s.Users[0]
	}
	return nil
}

func (s *AuthServer) GetUserAuth(username string) *UserAuth {
	if s == nil {
		return nil
	}
	for _, user := range s.Users {
		if username == user.Username {
			return user
		}
	}
	return nil
}
