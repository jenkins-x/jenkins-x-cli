package vault

// secretPath generates a secret path from the secret path for storing in vault
// this just makes sure it gets stored under /secret
func secretPath(path string) string {
	return "secret/data/" + path
}

// secretMetaPath generates the secret metadata path form the secret path provided
func secretMetadataPath(path string) string {
	return "secret/metadata/" + path
}

// AdminSecretPath returns the admin secret path for a given admin secret
func AdminSecretPath(secret AdminSecret) string {
	return AdminSecretsPath + string(secret)
}

// GitOpsSecretsPath returns the path of an install secret
func GitOpsSecretPath(secret string) string {
	return GitOpsSecretsPath + secret
}

// AuthSecretPath returns the path of an auth secret
func AuthSecretPath(secret string) string {
	return AuthSecretsPath + secret
}
