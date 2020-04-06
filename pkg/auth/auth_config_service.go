package auth

// Config gets the AuthConfig from the service
func (s *AuthConfigService) Config() *AuthConfig {
	if s.config == nil {
		s.config = &AuthConfig{}
	}
	return s.config
}

// SetConfig sets the AuthConfig object
func (s *AuthConfigService) SetConfig(c *AuthConfig) {
	s.config = c
}

// SaveUserAuth saves the given user auth for the server url
func (s *AuthConfigService) SaveUserAuth(url string, userAuth *UserAuth) error {
	config := s.config
	config.SetUserAuth(url, userAuth)
	user := userAuth.Username
	if user != "" {
		config.DefaultUsername = user
	}

	// Set Pipeline user once only.
	if config.PipeLineUsername == "" {
		config.PipeLineUsername = user
		config.PipeLineServer = url
	}

	config.CurrentServer = url
	return s.handler.SaveConfig(s.config)
}

// DeleteServer removes the given server from the configuration
func (s *AuthConfigService) DeleteServer(url string) error {
	s.config.DeleteServer(url)
	return s.handler.SaveConfig(s.config)
}

// LoadConfig loads the configuration from the users JX config directory
func (s *AuthConfigService) LoadConfig() (*AuthConfig, error) {
	var err error
	s.config, err = s.handler.LoadConfig()
	return s.config, err
}

// SaveConfig saves the configuration to disk
func (s *AuthConfigService) SaveConfig() error {
	return s.handler.SaveConfig(s.Config())
}

// NewAuthConfigService generates a AuthConfigService with a custom saver. This should not be used directly
func NewAuthConfigService(handler ConfigHandler) *AuthConfigService {
	return &AuthConfigService{handler: handler}
}
