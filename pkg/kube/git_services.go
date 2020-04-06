package kube

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/pkg/errors"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/gits"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureGitServiceExistsForHost ensures that there is a GitService CRD for the given host and kind
func EnsureGitServiceExistsForHost(jxClient versioned.Interface, devNs string, kind string, name string, gitUrl string, out io.Writer) error {
	if kind == "" || (kind == "github" && gitUrl == gits.GitHubURL) || gitUrl == "" {
		return nil
	}

	gitServices := jxClient.JenkinsV1().GitServices(devNs)
	list, err := gitServices.List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list git services")
	}
	for _, gs := range list.Items {
		if gitUrlsEqual(gs.Spec.URL, gitUrl) {
			oldKind := gs.Spec.GitKind
			if oldKind != kind {
				fmt.Fprintf(out, "Updating GitService %s as the kind has changed from %s to %s\n", gs.Name, oldKind, kind)
				gs.Spec.GitKind = kind
				_, err = gitServices.PatchUpdate(&gs)
				if err != nil {
					return fmt.Errorf("Failed to update kind on GitService with name %s: %s", gs.Name, err)
				}
				return errors.Wrap(err, "failed to PatchUpdate")
			} else {
				log.Logger().Infof("already has GitService %s in namespace %s for URL %s", gs.Name, devNs, gitUrl)
				return nil
			}
		}
	}
	if name == "" {
		u, err := url.Parse(gitUrl)
		if err != nil {
			return errors.Wrapf(err, "no name supplied and could not parse URL %s", u)
		}
		name = u.Host
	}

	// not found so lets create a new GitService
	gitSvc := &v1.GitService{
		ObjectMeta: metav1.ObjectMeta{
			Name: naming.ToValidNameWithDots(name),
		},
		Spec: v1.GitServiceSpec{
			Name:    name,
			URL:     gitUrl,
			GitKind: kind,
		},
	}
	current, err := gitServices.Get(name, metav1.GetOptions{})
	if err != nil {
		_, err = gitServices.Create(gitSvc)
		if err != nil {
			return errors.Wrapf(err, "failed to create GitService with name %s", gitSvc.Name)
		}
		log.Logger().Infof("GitService %s created in namespace %s for URL %s", gitSvc.Name, devNs, gitUrl)
	} else if current != nil {
		if current.Spec.URL != gitSvc.Spec.URL || current.Spec.GitKind != gitSvc.Spec.GitKind {
			current.Spec.URL = gitSvc.Spec.URL
			current.Spec.GitKind = gitSvc.Spec.GitKind

			_, err = gitServices.PatchUpdate(current)
			if err != nil {
				return errors.Wrapf(err, "failed to PatchUpdate GitService with name %s", gitSvc.Name)
			}
			log.Logger().Infof("GitService %s updated in namespace %s for URL %s", gitSvc.Name, devNs, gitUrl)
		}
	}
	return nil
}

// GetGitServiceKind returns the kind of the given host if one can be found or ""
func GetGitServiceKind(jxClient versioned.Interface, kubeClient kubernetes.Interface, devNs string, clusterAuthConfig *auth.AuthConfig, gitServiceURL string) (string, error) {
	answer := gits.SaasGitKind(gitServiceURL)
	if answer != "" {
		return answer, nil
	}

	if clusterAuthConfig != nil {
		clusterServer := clusterAuthConfig.GetServer(gitServiceURL)
		if clusterServer != nil {
			return clusterServer.Kind, nil
		}
	}

	answer, err := GetServiceKindFromSecrets(kubeClient, devNs, gitServiceURL)
	if err == nil && answer != "" {
		return answer, nil
	}

	return getServiceKindFromGitServices(jxClient, devNs, gitServiceURL)
}

// GetServiceKindFromSecrets gets the kind of service from secrets
func GetServiceKindFromSecrets(kubeClient kubernetes.Interface, ns string, gitServiceURL string) (string, error) {
	secretList, err := kubeClient.CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to list the secrets")
	}

	// note sometimes the Git secret is just called 'jx-pipeline-git' if its created as part of
	// 'jx create cluster --git-provider-url' - so lets handle the missing - on the name
	secretNamePrefix := strings.TrimSuffix(SecretJenkinsPipelineGitCredentials, "-")

	for _, secret := range secretList.Items {
		if strings.HasPrefix(secret.GetName(), secretNamePrefix) {
			annotations := secret.GetAnnotations()
			url, ok := annotations[AnnotationURL]
			if !ok {
				continue
			}
			if gitUrlsEqual(url, gitServiceURL) {
				labels := secret.GetLabels()
				serviceKind, ok := labels[LabelServiceKind]
				if !ok {
					return "", fmt.Errorf("no service kind label found on secret '%s' for Git service '%s'",
						secret.GetName(), gitServiceURL)
				}
				if serviceKind == "" {
					kind := labels[LabelKind]
					if kind == "git" {
						serviceKind = gits.SaasGitKind(gitServiceURL)
						if serviceKind == "" {
							// lets default to github?
							serviceKind = gits.KindGitHub
						}
					}
				}
				return serviceKind, nil
			}
		}
	}
	return "", fmt.Errorf("no secret found with configuration for '%s' Git service", gitServiceURL)
}

func getServiceKindFromGitServices(jxClient versioned.Interface, ns string, gitServiceURL string) (string, error) {
	gitServices := jxClient.JenkinsV1().GitServices(ns)
	list, err := gitServices.List(metav1.ListOptions{})
	if err == nil {
		for _, gs := range list.Items {
			if gitUrlsEqual(gs.Spec.URL, gitServiceURL) {
				return gs.Spec.GitKind, nil
			}
		}
	}
	return "", fmt.Errorf("no Git service resource found with URL '%s' in namespace %s", gitServiceURL, ns)
}

func gitUrlsEqual(url1 string, url2 string) bool {
	return url1 == url2 || strings.TrimSuffix(url1, "/") == strings.TrimSuffix(url2, "/")
}
