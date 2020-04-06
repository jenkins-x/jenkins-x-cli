package testkube

import (
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateTestPipelineGitSecret creates a test git pipeline credential secret
func CreateTestPipelineGitSecret(gitServiceKind string, name string, gitUrl string, username string, password string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.ToValidName(name),
			Namespace: "jx",
			Annotations: map[string]string{
				kube.AnnotationURL:  gitUrl,
				kube.AnnotationName: name,
			},
			Labels: map[string]string{
				kube.LabelKind:            kube.ValueKindGit,
				kube.LabelCredentialsType: kube.ValueCredentialTypeUsernamePassword,
				kube.LabelServiceKind:     gitServiceKind,
			},
		},
		Data: map[string][]byte{
			kube.SecretDataUsername: []byte(username),
			kube.SecretDataPassword: []byte(password),
		},
	}
}

// CreateFakeGitSecret creates a Secret for connecting to the fake git provider
func CreateFakeGitSecret() *corev1.Secret {
	secret := CreateTestPipelineGitSecret(gits.KindGitFake, "jx-pipeline-git-fake", gits.FakeGitURL, "fakeuser", "fakepwd")
	return &secret
}
