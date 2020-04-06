package config

// BasicAuth keeps the credentials for basic authentication
type BasicAuth struct {
	// Username stores the basic authentication user name
	Username string `json:"username"`
	// Password stores the basic authentication password
	Password string `json:"password"`
}
