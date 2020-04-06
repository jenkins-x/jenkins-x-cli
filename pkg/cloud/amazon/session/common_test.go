// +build unit

package session_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/cloud/amazon/testutils"

	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"

	"github.com/stretchr/testify/assert"
)

// Region tests
func TestResolvingDefaultRegion(t *testing.T) {
	oldHome, err := testutils.SwitchAWSHome()
	defer testutils.RestoreHome(oldHome)
	assert.Nil(t, err)
	testutils.ConfigureEnv("", "", "")
	sess, err := session.NewAwsSessionWithoutOptions()
	assert.Nil(t, err)
	assert.Equal(t, session.DefaultRegion, *sess.Config.Region)
}

func TestResolvingRegionFromAwsRegionEnv(t *testing.T) {
	oldHome, err := testutils.SwitchAWSHome()
	defer testutils.RestoreHome(oldHome)
	assert.Nil(t, err)
	testutils.ConfigureEnv("us-east-1", "", "")
	session, err := session.NewAwsSessionWithoutOptions()
	assert.Nil(t, err)
	assert.Equal(t, "us-east-1", *session.Config.Region)
}

func TestResolvingRegionFromAwsDefaultRegionEnv(t *testing.T) {
	oldHome, err := testutils.SwitchAWSHome()
	defer testutils.RestoreHome(oldHome)
	assert.Nil(t, err)
	testutils.ConfigureEnv("", "us-east-1", "")
	session, err := session.NewAwsSessionWithoutOptions()
	assert.Nil(t, err)
	assert.Equal(t, "us-east-1", *session.Config.Region)
}

func TestReturnOption(t *testing.T) {
	oldHome, err := testutils.SwitchAWSHome()
	defer testutils.RestoreHome(oldHome)
	assert.Nil(t, err)
	testutils.ConfigureEnv("", "", "")
	session, err := session.NewAwsSession("", "someRegion")
	assert.Nil(t, err)
	assert.Equal(t, "someRegion", *session.Config.Region)
}

func TestReadingRegionFromConfigProfile(t *testing.T) {
	oldHome, err := testutils.SwitchAWSHome()
	defer testutils.RestoreHome(oldHome)
	assert.Nil(t, err)
	testutils.ConfigureEnv("", "", "")
	session, err := session.NewAwsSession("foo", "")
	assert.Nil(t, err)
	assert.Equal(t, "bar", *session.Config.Region)
}

func TestReadingRegionFromEnvProfile(t *testing.T) {
	oldHome, err := testutils.SwitchAWSHome()
	defer testutils.RestoreHome(oldHome)
	assert.Nil(t, err)
	testutils.ConfigureEnv("", "", "baz")
	session, err := session.NewAwsSessionWithoutOptions()
	assert.Nil(t, err)
	assert.Equal(t, "qux", *session.Config.Region)
}

func TestParseContext(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		context string
		cluster string
		region  string
	}{
		"full cluster name from eksctl": {
			context: "cluster-name-jx.us-east-1.eksctl.io",
			cluster: "cluster-name-jx",
			region:  "us-east-1",
		},
		"full cluster name no eksctl": {
			context: "cluster-name-jx.us-east-1",
			cluster: "cluster-name-jx",
			region:  "us-east-1",
		},
		"full cluster name other region": {
			context: "cluster-name-jx.eu-north-4",
			cluster: "cluster-name-jx",
			region:  "eu-north-4",
		},
		"full cluster name eks arn": {
			context: "arn:aws:eks:us-east-1:111111111111:cluster/cluster-name-jx",
			cluster: "cluster-name-jx",
			region:  "us-east-1",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cluster, region, err := session.ParseContext(tc.context)
			assert.NoErrorf(t, err, "there shouldn't be an error parsing context %s", tc.context)
			assert.Equal(t, tc.cluster, cluster)
			assert.Equal(t, tc.region, region)
		})
	}
}
