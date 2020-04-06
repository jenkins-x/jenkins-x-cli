package vault

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	awsServiceAccountSecretKey = "credentials"
)

// StoreAWSCredentialsIntoSecret stores AWS credentials into a secret
func StoreAWSCredentialsIntoSecret(client kubernetes.Interface, awsAccessKeyID, awsSecretAccessKey, vaultName, namespace string) (string, error) {
	credentialsFileContent := []byte(fmt.Sprintf(`[default]
aws_access_key_id=%s
aws_secret_access_key=%s
`, awsAccessKeyID, awsSecretAccessKey))

	secretName := AwsServiceAccountSecretName(vaultName)
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: map[string][]byte{
			awsServiceAccountSecretKey: credentialsFileContent,
		},
	}

	secrets := client.CoreV1().Secrets(namespace)
	_, err := secrets.Get(secretName, metav1.GetOptions{})
	if err != nil {
		_, err = secrets.Create(secret)
	} else {
		_, err = secrets.Update(secret)
	}
	return secretName, nil
}
