// +build unit

package amazon_test

import (
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/cloud/amazon/testutils"

	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"

	"github.com/stretchr/testify/assert"
)

func TestCreateNewSessionWithDefaultRegion(t *testing.T) {
	// TODO Refactor for encapsulation
	oldHome, err := testutils.SwitchAWSHome()
	defer testutils.RestoreHome(oldHome)
	os.Setenv("AWS_REGION", "")
	os.Setenv("AWS_DEFAULT_REGION", "")
	os.Setenv("AWS_PROFILE", "")
	sess, err := session.NewAwsSessionWithoutOptions()
	assert.Nil(t, err)
	assert.Equal(t, "us-west-2", *sess.Config.Region)
}

func TestCreateNewSessionWithRegionFromAwsRegion(t *testing.T) {
	// TODO Refactor for encapsulation
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_DEFAULT_REGION", "")
	os.Setenv("AWS_PROFILE", "")
	sess, err := session.NewAwsSessionWithoutOptions()
	assert.Nil(t, err)
	assert.Equal(t, "us-east-1", *sess.Config.Region)
}
