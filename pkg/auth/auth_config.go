package auth

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/util"
	survey "gopkg.in/AlecAivazis/survey.v1"
)

func (c *AuthConfig) FindUserAuths(serverURL string) []*UserAuth {
	for _, server := range c.Servers {
		if urlsEqual(server.URL, serverURL) {
			return server.Users
		}
	}
	return []*UserAuth{}
}

func (c *AuthConfig) GetOrCreateUserAuth(url string, username string) *UserAuth {
	user := c.FindUserAuth(url, username)
	if user != nil {
		return user
	}
	server := c.GetOrCreateServer(url)
	if server.Users == nil {
		server.Users = []*UserAuth{}
	}
	user = &UserAuth{
		Username: username,
	}
	server.Users = append(server.Users, user)
	for _, user := range server.Users {
		if user.Username == username {
			return user
		}
	}
	return nil
}

// FindUserAuth finds the auth for the given user name
// if no username is specified and there is only one auth then return that else nil
func (c *AuthConfig) FindUserAuth(serverURL string, username string) *UserAuth {
	auths := c.FindUserAuths(serverURL)
	if username == "" {
		if len(auths) == 1 {
			return auths[0]
		} else {
			return nil
		}
	}
	for _, auth := range auths {
		if auth.Username == username {
			return auth
		}
	}
	return nil
}

func (c *AuthConfig) IndexOfServerName(name string) int {
	for i, server := range c.Servers {
		if server.Name == name {
			return i
		}
	}
	return -1
}

func (c *AuthConfig) SetUserAuth(url string, auth *UserAuth) {
	username := auth.Username
	for i, server := range c.Servers {
		if urlsEqual(server.URL, url) {
			for j, a := range server.Users {
				if a.Username == auth.Username {
					c.Servers[i].Users[j] = auth
					c.Servers[i].CurrentUser = username
					c.DefaultUsername = username
					c.CurrentServer = url
					return
				}
			}
			c.Servers[i].Users = append(c.Servers[i].Users, auth)
			c.Servers[i].CurrentUser = username
			c.DefaultUsername = username
			c.CurrentServer = url
			return
		}
	}
	c.Servers = append(c.Servers, &AuthServer{
		URL:         url,
		Users:       []*UserAuth{auth},
		CurrentUser: username,
	})
	c.DefaultUsername = username
	c.CurrentServer = url

}

func urlsEqual(url1, url2 string) bool {
	return url1 == url2 || strings.TrimSuffix(url1, "/") == strings.TrimSuffix(url2, "/")
}

// GetServerByName returns the server for the given URL or null if its not found
func (c *AuthConfig) GetServer(url string) *AuthServer {
	for _, s := range c.Servers {
		if urlsEqual(s.URL, url) {
			return s
		}
	}
	return nil
}

// GetServerByName returns the server for the given name or null if its not found
func (c *AuthConfig) GetServerByName(name string) *AuthServer {
	for _, s := range c.Servers {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// GetServerByKind returns the server for the given kind or null if its not found
func (c *AuthConfig) GetServerByKind(kind string) *AuthServer {
	for _, s := range c.Servers {
		if s.Kind == kind && s.URL == c.CurrentServer {
			return s
		}
	}
	return nil
}

//DeleteServer deletes the server for the given URL and updates the current server
//if is the same with the deleted server
func (c *AuthConfig) DeleteServer(url string) {
	for i, s := range c.Servers {
		if urlsEqual(s.URL, url) {
			c.Servers = append(c.Servers[:i], c.Servers[i+1:]...)
		}
	}
	if urlsEqual(c.CurrentServer, url) && len(c.Servers) > 0 {
		c.CurrentServer = c.Servers[0].URL
	} else {
		c.CurrentServer = ""
	}
}

func (c *AuthConfig) CurrentUser(server *AuthServer, inCluster bool) *UserAuth {
	if server == nil {
		return nil
	}
	if urlsEqual(c.PipeLineServer, server.URL) && inCluster {
		return server.GetUserAuth(c.PipeLineUsername)
	}
	return server.CurrentAuth()
}

// CurrentAuthServer returns the current AuthServer configured in the configuration
func (c *AuthConfig) CurrentAuthServer() *AuthServer {
	return c.GetServer(c.CurrentServer)
}

func (c *AuthConfig) GetOrCreateServer(url string) *AuthServer {
	name := ""
	kind := ""
	return c.GetOrCreateServerName(url, name, kind)
}

func (c *AuthConfig) GetOrCreateServerName(url string, name string, kind string) *AuthServer {
	s := c.GetServer(url)
	if s == nil {
		if name == "" {
			// lets default the name to the server URL
			name = urlHostName(url)
		}
		if c.Servers == nil {
			c.Servers = []*AuthServer{}
		}
		s = &AuthServer{
			URL:   url,
			Users: []*UserAuth{},
			Name:  name,
			Kind:  kind,
		}
		c.Servers = append(c.Servers, s)
	}
	if s.Kind == "" {
		s.Kind = kind
	}
	return s
}

func (c *AuthConfig) AddServer(server *AuthServer) {
	s := c.GetServer(server.URL)
	if s == nil {
		if c.Servers == nil {
			c.Servers = []*AuthServer{}
		}
		c.Servers = append(c.Servers, server)
	} else {
		s.Users = append(s.Users, server.Users...)
	}
}

func urlHostName(rawUrl string) string {
	u, err := url.Parse(rawUrl)
	if err == nil {
		return u.Host
	}
	idx := strings.Index(rawUrl, "://")
	if idx > 0 {
		rawUrl = rawUrl[idx+3:]
	}
	return strings.TrimSuffix(rawUrl, "/")
}

func (c *AuthConfig) PickServer(message string, batchMode bool, handles util.IOFileHandles) (*AuthServer, error) {
	if c.Servers == nil || len(c.Servers) == 0 {
		return nil, fmt.Errorf("No servers available!")
	}
	if len(c.Servers) == 1 {
		return c.Servers[0], nil
	}
	urls := []string{}
	for _, s := range c.Servers {
		urls = append(urls, s.URL)
	}
	url := ""
	if len(urls) > 1 {
		if batchMode {
			url = c.CurrentServer
		} else {
			prompt := &survey.Select{
				Message: message,
				Options: urls,
			}
			surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
			err := survey.AskOne(prompt, &url, survey.Required, surveyOpts)
			if err != nil {
				return nil, err
			}
		}
	}
	for _, s := range c.Servers {
		if urlsEqual(s.URL, url) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("Could not find server for URL %s", url)
}

// PickServerAuth Pick the servers auth
func (c *AuthConfig) PickServerUserAuth(server *AuthServer, message string, batchMode bool, org string, handles util.IOFileHandles) (*UserAuth, error) {
	url := server.URL
	userAuths := c.FindUserAuths(url)
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	if len(userAuths) == 1 {

		auth := userAuths[0]
		if batchMode {
			return auth, nil
		}
		confirm := &survey.Confirm{
			Message: fmt.Sprintf("Do you wish to use %s as the %s", auth.Username, message),
			Default: true,
		}
		flag := false
		err := survey.AskOne(confirm, &flag, nil, surveyOpts)
		if err != nil {
			return auth, err
		}
		if flag {
			return auth, nil
		}

		// lets create a new user name
		prompt := &survey.Input{
			Message: message,
		}
		username := ""
		err = survey.AskOne(prompt, &username, nil, surveyOpts)
		if err != nil {
			return auth, err
		}
		return c.GetOrCreateUserAuth(url, username), nil
	}
	if len(userAuths) > 1 {

		// If in batchmode select the user auth based on the org passed, or default to the first auth.
		if batchMode {
			for i, x := range userAuths {
				if x.Username == org {
					return userAuths[i], nil
				}
			}
			return userAuths[0], nil
		}

		usernames := []string{}
		m := map[string]*UserAuth{}
		for _, ua := range userAuths {
			name := ua.Username
			usernames = append(usernames, name)
			m[name] = ua
		}
		username := ""
		prompt := &survey.Select{
			Message: message,
			Options: usernames,
		}
		err := survey.AskOne(prompt, &username, survey.Required, surveyOpts)
		if err != nil {
			return &UserAuth{}, err
		}
		answer := m[username]
		if answer == nil {
			return &UserAuth{}, fmt.Errorf("no username chosen")
		}
		return answer, nil
	}
	return &UserAuth{}, nil
}

// PrintUserFn prints the use name
type PrintUserFn func(username string) error

// EditUserAuth Lets the user input/edit the user auth
func (c *AuthConfig) EditUserAuth(serverLabel string, auth *UserAuth, defaultUserName string, editUser, batchMode bool, fn PrintUserFn, handles util.IOFileHandles) error {
	// default the user name if its empty
	defaultUsername := c.DefaultUsername
	if defaultUsername == "" {
		defaultUsername = defaultUserName
	}
	if auth.Username == "" {
		auth.Username = defaultUsername
	}

	if batchMode {
		if auth.Username == "" {
			return fmt.Errorf("running in batch mode and no default Git username found")
		}
		if auth.ApiToken == "" {
			return fmt.Errorf("running in batch mode and no default API token found")
		}
		return nil
	}
	var err error

	if editUser || auth.Username == "" {
		auth.Username, err = util.PickValue(serverLabel+" username:", auth.Username, true, "", handles)
		if err != nil {
			return err
		}
	}
	if auth.ApiToken == "" {
		if fn != nil {
			err := fn(auth.Username)
			if err != nil {
				return err
			}
		}
		auth.ApiToken, err = util.PickPassword("API Token:", "", handles)
	}
	return err
}

// GetServerNames returns the name of the server currently in the configuration
func (c *AuthConfig) GetServerNames() []string {
	answer := []string{}
	for _, server := range c.Servers {
		name := server.Name
		if name != "" {
			answer = append(answer, name)
		}
	}
	sort.Strings(answer)
	return answer
}

// GetServerURLs returns the server URLs currently in the configuration
func (c *AuthConfig) GetServerURLs() []string {
	answer := []string{}
	for _, server := range c.Servers {
		u := server.URL
		if u != "" {
			answer = append(answer, u)
		}
	}
	sort.Strings(answer)
	return answer
}

// PickOrCreateServer picks the server to use defaulting to the current server
func (c *AuthConfig) PickOrCreateServer(fallbackServerURL string, serverURL string, message string, batchMode bool, handles util.IOFileHandles) (*AuthServer, error) {
	servers := c.Servers
	if len(servers) == 0 {
		if serverURL != "" {
			return c.GetOrCreateServer(serverURL), nil
		}
		return c.GetOrCreateServer(fallbackServerURL), nil
	}
	// lets let the user pick which server to use defaulting to the current server
	names := []string{}
	teamServerMissing := true
	for _, s := range servers {
		u := s.URL
		if u != "" {
			names = append(names, u)
		}
		if u == serverURL {
			teamServerMissing = false
		}
	}
	if teamServerMissing && serverURL != "" {
		names = append(names, serverURL)
	}
	if len(names) == 1 {
		return c.GetOrCreateServer(names[0]), nil
	}
	defaultValue := serverURL
	if defaultValue == "" {
		defaultValue = c.CurrentServer
	}
	if defaultValue == "" {
		defaultValue = names[0]
	}
	if batchMode {
		return c.GetOrCreateServer(defaultValue), nil
	}
	name, err := util.PickRequiredNameWithDefault(names, message, defaultValue, "", handles)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("no server URL chosen")
	}
	return c.GetOrCreateServer(name), nil
}

// UpdatePipelineServer updates the pipeline server in the configuration
func (c *AuthConfig) UpdatePipelineServer(server *AuthServer, user *UserAuth) {
	c.PipeLineServer = server.URL
	c.PipeLineUsername = user.Username
}

// GetPipelineAuth returns the current pipline server and user authentication
func (c *AuthConfig) GetPipelineAuth() (*AuthServer, *UserAuth) {
	server := c.GetServer(c.PipeLineServer)
	user := server.GetUserAuth(c.PipeLineUsername)
	return server, user
}

// Merge merges another auth config such as if loading git/credentials
func (c *AuthConfig) Merge(o *AuthConfig) {
	if o == nil {
		return
	}
	for _, s := range o.Servers {
		cs := c.GetOrCreateServer(s.URL)
		for _, u := range s.Users {
			cu := cs.GetUserAuth(u.Username)
			if cu == nil {
				cs.Users = append(cs.Users, u)
			} else {
				if u.Password != "" {
					cu.Password = u.Password
				}
				if u.ApiToken != "" {
					cu.ApiToken = u.ApiToken
				}
			}
		}
	}
}
