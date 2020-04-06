// +build unit

package features_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/features"
	"github.com/stretchr/testify/assert"
)

func TestCheckTektonEnabledOss(t *testing.T) {

	//Given
	features.SetFeatureFlagToken("oss")
	features.Init()

	//When
	err := features.CheckTektonEnabled()

	//Then
	assert.Nil(t, err)
}

func TestCheckTektonDisabledByDefaultWithToken(t *testing.T) {

	//Given
	features.SetFeatureFlagToken("test-token")
	features.Init()

	//When
	err := features.CheckTektonEnabled()

	//Then
	assert.NotNil(t, err)
}

func TestCheckStaticJenkinsEnabledOss(t *testing.T) {

	//Given
	features.SetFeatureFlagToken("oss")
	features.Init()

	//When
	err := features.CheckStaticJenkins()

	//Then
	assert.Nil(t, err)
}

func TestCheckStaticJenkinsDisabledByDefaultWithToken(t *testing.T) {

	//Given
	features.SetFeatureFlagToken("test-token")
	features.Init()

	//When
	err := features.CheckStaticJenkins()

	//Then
	assert.NotNil(t, err)
}
