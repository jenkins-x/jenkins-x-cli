package auth

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// labelKind inidicates the kind of auth such as git
	labelKind = "jenkins.io/kind"
	// labelServiceKind inidicates the  kind of service such as github
	labelServiceKind = "jenkins.io/service-kind"
	// labelCreatedBy indicates the service that created this resource
	labelCreatedBy = "jenkins.io/created-by"
	// labelCredentialsType the kind of jenkins credential for a secret
	labelCredentialsType = "jenkins.io/credentials-type"
	// labelGithubAppOwner the label to indicate the owner of a repository for github app token secrets
	labelGithubAppOwner = "jenkins.io/githubapp-owner"
	// valueCreatedByJX for resources created by the Jenkins X CLI
	valueCreatedByJX = "jx"
	// valueCredentialTypeUsernamePassword for user password credential secrets
	valueCredentialTypeUsernamePassword = "usernamePassword"
	// annotationURL indicates a service/server's URL
	annotationURL = "jenkins.io/url"
	// annotationName indicates a service/server's textual name (can be mixed case, contain spaces unlike Kubernetes resources)
	annotationName = "jenkins.io/name"
	// annotationCredentialsDescription the description text for a Credential on a Secret
	annotationCredentialsDescription = "jenkins.io/credentials-description"
	// secretDataUsername the username in a Secret/Credentials
	usernameKey = "username"
	// secretDataPassword the password in a Secret/Credentials
	passwordKey = "password"
	// secretPrefix prefix for pipeline secrets
	secretPrefix = "jx-pipeline"
)

// LoadConfig loads the config from kuberntes secrets
func (k *KubeAuthConfigHandler) LoadConfig() (*AuthConfig, error) {
	secrets, err := k.secrets()
	if err != nil {
		return nil, errors.Wrap(err, "retrieving config from k8s secrets")
	}
	if secrets == nil {
		return nil, fmt.Errorf("no secrets found for server kind %q and service kind %q", k.kind, k.serviceKind)
	}
	config := &AuthConfig{}
	for _, secret := range secrets.Items {
		labels := secret.Labels
		annotations := secret.Annotations
		if labels != nil && annotations != nil {
			url := annotations[annotationURL]
			name := annotations[annotationName]
			serviceKind := labels[labelServiceKind]
			if url != "" {
				user, err := k.userFromSecret(secret)
				if err != nil {
					continue
				}
				user.GithubAppOwner = labels[labelGithubAppOwner]
				var server *AuthServer

				// for github app mode lets share the same server and have multiple users
				if user.GithubAppOwner != "" {
					for _, s := range config.Servers {
						if s.URL == url {
							server = s
							break
						}
					}
				}
				if server != nil {
					server.Users = append(server.Users, &user)
				} else {
					server = &AuthServer{
						URL:  url,
						Name: name,
						Kind: serviceKind,
						Users: []*UserAuth{
							&user,
						},
					}
					if user.GithubAppOwner == "" {
						server.CurrentUser = user.Username
					}
					if config.Servers == nil {
						config.Servers = []*AuthServer{}
					}
					config.Servers = append(config.Servers, server)
				}
				config.CurrentServer = server.URL
				config.PipeLineServer = server.URL
				if user.GithubAppOwner == "" {
					config.PipeLineUsername = user.Username
					config.DefaultUsername = user.Username
				}
			}
		}
	}
	return config, nil
}

// SaveConfig saves the config into kuberntes secret
func (k *KubeAuthConfigHandler) SaveConfig(config *AuthConfig) error {
	for _, server := range config.Servers {
		name := k.secretName(server)
		labels := k.labels(server)
		annotations := k.annotations(server)
		secret, err := k.client.CoreV1().Secrets(k.namespace).Get(name, metav1.GetOptions{})
		create := false
		if err != nil {
			create = true
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Labels:      labels,
					Annotations: annotations,
				},
				Data: map[string][]byte{},
			}
		} else {
			secret.Labels = util.MergeMaps(secret.Labels, labels)
			secret.Annotations = util.MergeMaps(secret.Annotations, annotations)
		}
		user := server.CurrentAuth()
		if user == nil {
			return fmt.Errorf("current user for %q server is empty", server.URL)
		}
		if user.Username == "" {
			return errors.New("empty username")
		}
		if user.ApiToken == "" && user.Password == "" {
			return errors.New("empty credentials")
		}
		secret.Data[usernameKey] = []byte(user.Username)
		if user.ApiToken != "" {
			secret.Data[passwordKey] = []byte(user.ApiToken)
		} else {
			secret.Data[passwordKey] = []byte(user.Password)
		}
		if user.GithubAppOwner != "" {
			labels := map[string]string{
				labelGithubAppOwner: user.GithubAppOwner,
			}
			secret.Labels = util.MergeMaps(secret.Labels, labels)
		}
		if create {
			if _, err := k.client.CoreV1().Secrets(k.namespace).Create(secret); err != nil {
				return errors.Wrapf(err, "creating secret %q", name)
			}
		} else {
			if _, err := k.client.CoreV1().Secrets(k.namespace).Update(secret); err != nil {
				return errors.Wrapf(err, "updating secret %q", name)
			}
		}
	}
	return nil
}

// secretName builds the secret name
func (k *KubeAuthConfigHandler) secretName(server *AuthServer) string {
	secretName := secretPrefix
	kind := strings.ToLower(k.kind)
	if kind != "" {
		secretName += "-" + kind
	}
	serviceKind := strings.ToLower(server.Kind)
	if serviceKind != "" {
		secretName += "-" + serviceKind
	}
	name := strings.ToLower(server.Name)
	if name != "" {
		secretName += "-" + name
	}
	return secretName
}

func (k *KubeAuthConfigHandler) labels(server *AuthServer) map[string]string {
	return map[string]string{
		labelCredentialsType: valueCredentialTypeUsernamePassword,
		labelCreatedBy:       valueCreatedByJX,
		labelKind:            k.kind,
		labelServiceKind:     server.Kind,
	}
}

func (k *KubeAuthConfigHandler) annotations(server *AuthServer) map[string]string {
	return map[string]string{
		annotationCredentialsDescription: fmt.Sprintf("Configuration and credentials for server %s", server.URL),
		annotationURL:                    server.URL,
		annotationName:                   server.Name,
	}
}

func (k *KubeAuthConfigHandler) secrets() (*corev1.SecretList, error) {
	selector := labelKind + "=" + k.kind
	if k.serviceKind != "" {
		selector = labelServiceKind + "=" + k.serviceKind
	}
	opts := metav1.ListOptions{
		LabelSelector: selector,
	}
	return k.client.CoreV1().Secrets(k.namespace).List(opts)
}

func (k *KubeAuthConfigHandler) userFromSecret(secret corev1.Secret) (UserAuth, error) {
	data := secret.Data
	if data == nil {
		return UserAuth{}, fmt.Errorf("no user auth credentials found in secret '%s'", secret.Name)
	}
	username, ok := data[usernameKey]
	if !ok || len(username) == 0 {
		return UserAuth{}, fmt.Errorf("no user name found in secret '%s'", secret.Name)
	}
	password, ok := data[passwordKey]
	if !ok || len(password) == 0 {
		return UserAuth{}, fmt.Errorf("no password found in secret '%s'", secret.Name)
	}

	return UserAuth{
		Username: string(username),
		ApiToken: string(password),
	}, nil
}

// NewKubeAuthConfigHandler creates a handler which loads/stores the auth config from/into Kubernetes secrets
func NewKubeAuthConfigHandler(client kubernetes.Interface, namespace string, kind string, serviceKind string) KubeAuthConfigHandler {
	return KubeAuthConfigHandler{
		client:      client,
		namespace:   namespace,
		kind:        kind,
		serviceKind: serviceKind,
	}
}

// NewKubeAuthConfigService creates a config services that loads/stores the auth config from a Kubernetes secret
func NewKubeAuthConfigService(client kubernetes.Interface, namespace string, kind string, serviceKind string) ConfigService {
	handler := NewKubeAuthConfigHandler(client, namespace, kind, serviceKind)
	return NewAuthConfigService(&handler)
}
