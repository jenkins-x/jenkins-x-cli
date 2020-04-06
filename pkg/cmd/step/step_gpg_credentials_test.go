// +build unit

package step_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/step"

	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStepGPGCredentials(t *testing.T) {
	t.Parallel()
	tempDir, err := ioutil.TempDir("", "test-step-gpg")
	assert.NoError(t, err)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{},
		Data: map[string][]byte{
			"pubring.gpg":     []byte("Pubring"),
			"sec-jenkins.gpg": []byte("sec jenkins"),
			"secring.gpg":     []byte("secring"),
			"trustdb.gpg":     []byte("trustdb"),
		},
	}

	options := &step.StepGpgCredentialsOptions{
		OutputDir: tempDir,
	}

	err = options.GenerateGpgFiles(secret)
	assert.NoError(t, err)

	tests.AssertFileExists(t, filepath.Join(tempDir, "pubring.gpg"))
	tests.AssertFileExists(t, filepath.Join(tempDir, "sec-jenkins.gpg"))
	tests.AssertFileExists(t, filepath.Join(tempDir, "secring.gpg"))
	tests.AssertFileExists(t, filepath.Join(tempDir, "trustdb.gpg"))
}
