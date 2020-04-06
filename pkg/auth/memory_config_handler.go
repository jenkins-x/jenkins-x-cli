package auth

// NewMemoryAuthConfigService creates a new memory based auth service
func NewMemoryAuthConfigService() ConfigService {
	handler := newMemoryAuthHandler()
	return NewAuthConfigService(handler)
}

func newMemoryAuthHandler() ConfigHandler {
	return &MemoryAuthConfigHandler{}
}

// LoadConfig returns the current config from memory
func (m *MemoryAuthConfigHandler) LoadConfig() (*AuthConfig, error) {
	return &m.config, nil
}

// SaveConfig updates the config in memory
func (m *MemoryAuthConfigHandler) SaveConfig(config *AuthConfig) error {
	m.config = *config
	return nil
}
