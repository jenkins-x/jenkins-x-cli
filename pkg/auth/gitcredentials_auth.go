package auth

import (
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// loadGitCredentialsAuth loads the git credentials from the `git/credentials` file
// in `$XDG_CONFIG_HOME/git/credentials` or in the `~/git/credentials` directory
func loadGitCredentialsAuth() (*AuthConfig, error) {
	fileName := util.GitCredentialsFile()
	return loadGitCredentialsAuthFile(fileName)
}

// loadGitCredentialsAuthFile loads the git credentials file
func loadGitCredentialsAuthFile(fileName string) (*AuthConfig, error) {
	exists, err := util.FileExists(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to check if git credentials file exists %s", fileName)
	}
	if !exists {
		return nil, nil
	}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load git credentials file %s", fileName)
	}

	config := &AuthConfig{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		u, err := url.Parse(line)
		if err != nil {
			log.Logger().Warnf("ignoring invalid line in git credentials file: %s error: %s", fileName, err.Error())
			continue
		}

		user := u.User
		username := user.Username()
		password, _ := user.Password()
		if username == "" {
			log.Logger().Warnf("ignoring missing user name in git credentials file: %s URL: %s", fileName, line)
			continue
		}
		if password == "" {
			log.Logger().Warnf("ignoring missing password in git credentials file: %s URL: %s", fileName, line)
			continue
		}
		u.User = nil
		serverURL := u.String()
		server := config.GetOrCreateServer(serverURL)
		server.CurrentUser = username
		server.Users = append(server.Users, &UserAuth{
			Username: username,
			Password: password,
			ApiToken: password,
		})
	}
	return config, nil
}
