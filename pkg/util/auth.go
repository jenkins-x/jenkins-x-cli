package util

import (
	"crypto/sha1" // #nosec
	"encoding/base64"
	"strings"
)

// BasicAuth encodes the provided user name and password as basic auth credentials
func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// HashPassword hashes the given password with SHA1
func HashPassword(password string) string {
	s := sha1.New() // #nosec
	s.Write([]byte(password))
	passwordSum := s.Sum(nil)
	return base64.StdEncoding.EncodeToString(passwordSum)
}

// RemoveScheme removes the scheme from a URL
func RemoveScheme(u string) string {
	idx := strings.Index(u, "://")
	if idx >= 0 {
		return u[idx+3:]
	}
	return u
}
