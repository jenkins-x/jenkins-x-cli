package vault

import "fmt"

// AwsServiceAccountSecretName builds the secret name where the AWS service account is stored
func AwsServiceAccountSecretName(vaultName string) string {
	return fmt.Sprintf("%s-%s", vaultName, "aws-cred")
}
