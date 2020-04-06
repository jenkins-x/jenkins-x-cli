// +build unit

package deletecmd

import (
	"fmt"
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	clients_test "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/petergtz/pegomock"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDeleteApplicationInJenkins(t *testing.T) {
	pegomock.RegisterMockTestingT(t)
	t.Parallel()

	testOrgNameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	testRepoNameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	testOrgName := testOrgNameUUID.String()
	testRepoName := testRepoNameUUID.String()

	mockFactory := clients_test.NewMockFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(mockFactory)

	testhelpers.ConfigureTestOptionsWithResources(&commonOpts,
		[]runtime.Object{},
		[]runtime.Object{
			kube.NewPermanentEnvironment("EnvWhereApplicationIsDeployed"),
		},
		gits.NewGitLocal(),
		nil,
		helm_test.NewMockHelmer(),
		resources_test.NewMockInstaller(),
	)
	testhelpers.MockFactoryWithKubeClients(mockFactory, &commonOpts)
	kubeClient, _, _ := mockFactory.CreateKubeClient()

	jenkinsClient := clients_test.NewMockJenkinsClient()
	pegomock.When(mockFactory.CreateJenkinsClient(kubeClient, "jx", commonOpts.GetIOFileHandles())).ThenReturn(pegomock.ReturnValue(jenkinsClient), pegomock.ReturnValue(nil))
	job := gojenkins.Job{
		Name:     testRepoName,
		FullName: fmt.Sprintf("%s/%s", testOrgName, testRepoName),
		Class:    "org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject",
	}
	jobs := []gojenkins.Job{
		job,
	}
	pegomock.When(jenkinsClient.GetJobs()).ThenReturn(pegomock.ReturnValue(jobs), pegomock.ReturnValue(nil))
	pegomock.When(jenkinsClient.GetJob(pegomock.EqString(testRepoName))).ThenReturn(pegomock.ReturnValue(job), pegomock.ReturnValue(nil))

	o := &DeleteApplicationOptions{
		CommonOptions: &commonOpts,
	}
	o.Args = []string{testRepoName}

	err = o.Run()
	assert.NoError(t, err)

	jenkinsClient.VerifyWasCalledOnce().DeleteJob(job)
}

func TestRemoveEnvrionmentFromJobs(t *testing.T) {
	t.Parallel()

	fakeEnv := &v1.Environment{
		Spec: v1.EnvironmentSpec{
			Source: v1.EnvironmentRepository{
				URL: "http://github.com/owner/environment-repo-dev.git",
			},
		},
	}

	tests := []struct {
		name string
		jobs []string
		envs map[string]*v1.Environment
		want []string
	}{
		{
			"when there's no matching environment",
			[]string{"owner/job-repo", "owner/environment-foobar"},
			map[string]*v1.Environment{"dev": fakeEnv},
			[]string{"owner/job-repo", "owner/environment-foobar"},
		},
		{
			"when there's an environment",
			[]string{"owner/job-repo", "owner/environment-repo-dev"},
			map[string]*v1.Environment{"dev": fakeEnv},
			[]string{"owner/job-repo"},
		},
	}

	for _, test := range tests {
		got := removeEnvironments(test.jobs, test.envs)
		assert.Equal(t, test.want, got, test.name)
	}
}
